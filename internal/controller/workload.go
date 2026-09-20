package controller

import (
	"context"
	"log"
	"math/rand/v2"
	"time"

	"github.com/wiiaam/homelab-dashboard/internal/models"
	"github.com/wiiaam/homelab-dashboard/internal/store"
)

// SimulatedMetrics drives the metrics Store in v0: on each tick it reads the
// current services and writes random CPU/mem samples, so the metrics SSE
// path can be observed updating live without metrics-server. In v1 this is
// replaced by a poller over metrics-server / Prometheus.
type SimulatedMetrics struct {
	store store.Store
	every time.Duration
}

func NewSimulatedMetrics(st store.Store, every time.Duration) *SimulatedMetrics {
	return &SimulatedMetrics{store: st, every: every}
}

func (m *SimulatedMetrics) Run(ctx context.Context) error {
	t := time.NewTicker(m.every)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-t.C:
			// Only poll services currently in the store: whatever the store
			// lists is the active set. A service removed mid-tick is guarded
			// again right before writing.
			active := m.store.ListServices()
			keys := make([]string, 0, len(active))
			for _, s := range active {
				keys = append(keys, s.Key)
			}
			log.Printf("metrics: sampling %d active service(s): %v", len(keys), keys)

			for _, s := range active {
				if _, ok := m.store.GetService(s.Key); !ok {
					continue // disappeared between list and write
				}
				m.store.SetMetrics(s.Key, models.Metrics{
					Key:           models.MetricsKey(s.NS, s.Name),
					CPUMillicores: rand.Int64N(400),
					MemBytes:      (128 + rand.Int64N(896)) << 20,
					UpdatedAt:     now,
				})
			}
		}
	}
}
