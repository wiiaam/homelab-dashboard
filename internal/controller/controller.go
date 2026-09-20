package controller

import (
	"context"
	"log"

	"github.com/wiiaam/homelab-dashboard/internal/models"
	"github.com/wiiaam/homelab-dashboard/internal/store"
)

// ChangeKind describes how a discovered service changed.
type ChangeKind int

const (
	ChangeAdded ChangeKind = iota
	ChangeUpdated
	ChangeRemoved
)

// Change is a single mutation from a Source. For ChangeAdded/ChangeUpdated
// the Service carries the full desired state; for ChangeRemoved only Key is
// meaningful.
type Change struct {
	Kind    ChangeKind
	Service models.Service
}

// Source yields the current desired state and a stream of mutations. v0
// ships FixtureSource (seed + scripted changes); v1 adds an informer-driven
// source over Ingress/HTTPRoute watches without touching this interface.
type Source interface {
	Snapshot() []models.Service
	Changes() <-chan Change
}

// Controller reconciles a Source into a Store. Reconcile logic is identical
// regardless of which Source or Store implementation backs it.
type Controller struct {
	store store.Store
	src   Source
}

func New(st store.Store, src Source) *Controller {
	return &Controller{store: st, src: src}
}

func (c *Controller) Run(ctx context.Context) error {
	for _, s := range c.src.Snapshot() {
		c.store.SetService(s)
	}
	log.Printf("controller: reconciled initial snapshot, %d service(s) active", len(c.store.ListServices()))

	for {
		select {
		case ch, ok := <-c.src.Changes():
			if !ok {
				return nil
			}
			switch ch.Kind {
			case ChangeRemoved:
				c.store.DeleteService(ch.Service.Key)
				log.Printf("controller: %s removed, %d service(s) active", ch.Service.Key, len(c.store.ListServices()))
			default:
				c.store.SetService(ch.Service)
				verb := "added"
				if ch.Kind == ChangeUpdated {
					verb = "updated"
				}
				log.Printf("controller: %s %s, %d service(s) active", ch.Service.Key, verb, len(c.store.ListServices()))
			}
		case <-ctx.Done():
			return nil
		}
	}
}
