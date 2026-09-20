package server

import (
	"strings"
	"testing"

	"github.com/wiiaam/homelab-dashboard/internal/models"
	"github.com/wiiaam/homelab-dashboard/internal/store"
)

func TestFragmentForMetrics(t *testing.T) {
	st := store.NewMemoryStore()
	s := models.Service{Key: models.ServiceKey("default", "grafana"), NS: "default", Name: "grafana"}
	st.SetService(s)

	srv := New(st)
	name, frag := srv.fragmentFor(models.Event{
		Type:    models.EventMetrics,
		Key:     models.MetricsKey("default", "grafana"),
		Metrics: &models.Metrics{CPUMillicores: 250, MemBytes: 768 << 20},
	})

	if name != "metrics" {
		t.Fatalf("event name = %q, want metrics", name)
	}
	if !strings.Contains(frag, `id="metrics-default-grafana"`) {
		t.Fatalf("fragment %q missing metrics span id", frag)
	}
	if !strings.Contains(frag, "250m CPU") {
		t.Fatalf("fragment %q missing cpu", frag)
	}
}

func TestFragmentForRouteAddTargetsPrecedingTile(t *testing.T) {
	st := store.NewMemoryStore()
	st.SetService(models.Service{Key: models.ServiceKey("default", "apollo"), NS: "default", Name: "apollo", Order: 1})
	st.SetService(models.Service{Key: models.ServiceKey("default", "hades"), NS: "default", Name: "hades", Order: 3})

	added := models.Service{Key: models.ServiceKey("default", "athena"), NS: "default", Name: "athena", Order: 2}
	st.SetService(added)

	srv := New(st)
	name, frag := srv.fragmentFor(models.Event{Type: models.EventAdded, Key: added.Key, Service: &added})

	if name != "route" {
		t.Fatalf("event name = %q, want route", name)
	}
	if !strings.Contains(frag, `hx-swap-oob="afterend:#service-default-apollo"`) {
		t.Fatalf("fragment %q has wrong OOB target", frag)
	}
	if !strings.Contains(frag, `<div id="service-default-athena"`) {
		t.Fatalf("fragment %q does not carry the tile box id", frag)
	}
}

func TestFragmentForRouteRemove(t *testing.T) {
	srv := New(store.NewMemoryStore())
	s := models.Service{Key: models.ServiceKey("tools", "searxng"), NS: "tools", Name: "searxng"}
	name, frag := srv.fragmentFor(models.Event{Type: models.EventRemoved, Key: s.Key, Service: &s})

	if name != "route" {
		t.Fatalf("event name = %q, want route", name)
	}
	if !strings.Contains(frag, `hx-swap-oob="delete"`) || !strings.Contains(frag, `id="service-tools-searxng"`) {
		t.Fatalf("fragment %q wrong for remove", frag)
	}
}

func TestFragmentForMetricsDropsInactiveService(t *testing.T) {
	// A metrics event for a service that is not in the store (e.g. raced out
	// after removal) must produce no fragment, so the browser never swaps
	// into a phantom #metrics-* node after the tile was deleted.
	st := store.NewMemoryStore()
	srv := New(st)

	name, frag := srv.fragmentFor(models.Event{
		Type:    models.EventMetrics,
		Key:     models.MetricsKey("tools", "searxng"),
		Metrics: &models.Metrics{CPUMillicores: 100, MemBytes: 1 << 30},
	})
	if name != "" || frag != "" {
		t.Fatalf("expected dropped event for inactive service, got name=%q frag=%q", name, frag)
	}

	st.SetService(models.Service{Key: models.ServiceKey("tools", "searxng"), NS: "tools", Name: "searxng"})
	name, frag = srv.fragmentFor(models.Event{
		Type:    models.EventMetrics,
		Key:     models.MetricsKey("tools", "searxng"),
		Metrics: &models.Metrics{CPUMillicores: 100, MemBytes: 1 << 30},
	})
	if name != "metrics" || frag == "" {
		t.Fatalf("expected forwarded event for active service, got name=%q frag=%q", name, frag)
	}
}
