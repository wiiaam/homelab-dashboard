package store

import (
	"context"

	"github.com/wiiaam/homelab-dashboard/internal/models"
)

// Store abstracts the dashboard's data layer. It is implemented by
// MemoryStore in v0 and RedisStore in v1; controller, render, and SSE
// logic are written once against this interface.
type Store interface {
	GetService(key string) (models.Service, bool)
	ListServices() []models.Service
	SetService(s models.Service)
	DeleteService(key string)

	SetMetrics(key string, m models.Metrics)
	GetMetrics(key string) (models.Metrics, bool)

	// Subscribe returns a stream of change events. The channel is closed
	// when ctx is cancelled.
	Subscribe(ctx context.Context) <-chan models.Event
}
