package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/a-h/templ"

	"github.com/wiiaam/homelab-dashboard/internal/models"
	"github.com/wiiaam/homelab-dashboard/internal/store"
	"github.com/wiiaam/homelab-dashboard/web/components"
)

// Server is the stateless HTTP frontend: full page render on each request,
// SSE pushed updates via the store's Subscribe stream.
type Server struct {
	store store.Store
	mux   *http.ServeMux
}

func New(st store.Store) *Server {
	s := &Server{store: st, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /events", s.handleEvents)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	services := s.store.ListServices()
	models.SortServices(services)

	views := make([]components.TileView, 0, len(services))
	for _, svc := range services {
		views = append(views, tileView(s.store, svc))
	}

	if err := components.Page(views).Render(r.Context(), w); err != nil {
		log.Printf("render page: %v", err)
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	for ev := range s.store.Subscribe(r.Context()) {
		name, frag := s.fragmentFor(ev)
		if frag == "" {
			continue
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, frag); err != nil {
			return
		}
		flusher.Flush()
	}
}

// fragmentFor renders the SSE event name and OOB-swap HTML fragment for a
// store event. Returns ("", "") for events that have no browser effect.
func (s *Server) fragmentFor(ev models.Event) (string, string) {
	switch ev.Type {
	case models.EventMetrics:
		ns, name := models.SplitKey(ev.Key)
		svcKey := models.ServiceKey(ns, name)
		if _, ok := s.store.GetService(svcKey); !ok {
			// The service was removed but a poll tick still raced out a
			// sample. The tile is gone from the grid — drop the event rather
			// than swap into a phantom #metrics-* node.
			return "", ""
		}
		return "metrics", render(components.MetricsFragment(
			components.TileView{
				Service: models.Service{NS: ns, Name: name, Key: svcKey},
				Metrics: ev.Metrics,
			},
		))

	case models.EventRemoved:
		return "route", render(components.RouteRemoveFragment(*ev.Service))

	case models.EventAdded:
		target := components.InsertTarget(s.store.ListServices(), *ev.Service)
		return "route", render(components.TileInsert(tileView(s.store, *ev.Service), target))

	case models.EventUpdated:
		return "route", render(components.Tile(tileView(s.store, *ev.Service), "true"))
	}
	return "", ""
}

func tileView(st store.Store, svc models.Service) components.TileView {
	v := components.TileView{Service: svc}
	if met, ok := st.GetMetrics(svc.Key); ok {
		v.Metrics = &met
	}
	return v
}

func render(c templ.Component) string {
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		log.Printf("render fragment: %v", err)
		return ""
	}
	return buf.String()
}
