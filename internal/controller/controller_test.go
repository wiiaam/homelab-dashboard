package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/wiiaam/homelab-dashboard/internal/controller"
	"github.com/wiiaam/homelab-dashboard/internal/models"
	"github.com/wiiaam/homelab-dashboard/internal/store"
)

func TestControllerReconcilesFixtureSnapshot(t *testing.T) {
	st := store.NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := st.Subscribe(ctx)
	go controller.New(st, controller.NewFixtureSource()).Run(ctx)

	// The snapshot reconcile must emit an "added" event for each seeded
	// service before the flap script (3s grace) can remove any.
	seen := make(map[string]bool)
	deadline := time.After(2 * time.Second)
	for len(seen) < 8 {
		select {
		case ev := <-ch:
			if ev.Type == models.EventAdded && ev.Service != nil {
				seen[ev.Service.Key] = true
			}
		case <-deadline:
			t.Fatalf("snapshot not fully reconciled, saw %d/8 services", len(seen))
		}
	}
}
