package controller

import (
	"math/rand/v2"
	"time"

	"github.com/wiiaam/homelab-dashboard/internal/models"
)

// FixtureSource is the v0 Source: a seeded snapshot plus a flap simulation
// that removes a random service for a few seconds and re-adds it, so the
// SSE/OOB-swap add/remove mechanics can be exercised end-to-end without a
// cluster.
type FixtureSource struct {
	svcs    []models.Service
	changes chan Change
}

// NewFixtureSource seeds a standard homelab service set and starts the
// flap script.
func NewFixtureSource() *FixtureSource {
	f := &FixtureSource{
		svcs:    fixtureServices(),
		changes: make(chan Change, 8),
	}
	go f.script()
	return f
}

func (f *FixtureSource) Snapshot() []models.Service {
	return f.svcs
}

func (f *FixtureSource) Changes() <-chan Change {
	return f.changes
}

// script flaps a single fixed service: 10s of normal operation, then that
// service disappears for 5s and is re-added, cycling forever. Picking one
// service up front (rather than a new random one each cycle) keeps the demo
// clearly about one tile flapping instead of services visibly swapping.
func (f *FixtureSource) script() {
	const (
		normalTime = 10 * time.Second
		flapGone   = 5 * time.Second
	)

	s := f.svcs[rand.IntN(len(f.svcs))]

	time.Sleep(normalTime)
	for {
		f.changes <- Change{Kind: ChangeRemoved, Service: s}
		time.Sleep(flapGone)
		f.changes <- Change{Kind: ChangeAdded, Service: s}
		time.Sleep(normalTime)
	}
}

func fixtureServices() []models.Service {
	return []models.Service{
		svc("default", "prometheus", "http://prometheus.monitoring.svc:9090", "📈", 1, ""),
		svc("default", "grafana", "http://grafana.monitoring.svc:3000", "📊", 1, ""),
		svc("home", "homeassistant", "http://homeassistant.home.svc:8123", "🏠", 2, ""),
		svc("network", "opnsense", "http://opnsense.lan", "🛡️", 3, ""),
		svc("network", "unifi", "http://unifi.lan", "📶", 4, ""),
		svc("media", "jellyfin", "http://jellyfin.media.svc:8096", "🎬", 5, ""),
		svc("tools", "filebrowser", "http://filebrowser.tools.svc", "📁", 6, ""),
		svc("tools", "searxng", "http://searxng.tools.svc:8080", "🔎", 7, ""),
	}
}

func svc(ns, name, url, icon string, order int, group string) models.Service {
	return models.Service{
		Key:   models.ServiceKey(ns, name),
		NS:    ns,
		Name:  name,
		URL:   url,
		Icon:  icon,
		Order: order,
		Group: group,
	}
}
