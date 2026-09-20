package store

import (
	"context"
	"sync"

	"github.com/wiiaam/homelab-dashboard/internal/models"
)

// MemoryStore is the v0 in-process Store implementation: mutex-protected
// maps plus a non-blocking fan-out of change events to subscribers.
type MemoryStore struct {
	mu       sync.RWMutex
	services map[string]models.Service
	metrics  map[string]models.Metrics
	order    []string

	subMu   sync.Mutex
	nextSub int
	subs    map[int]chan models.Event
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		services: make(map[string]models.Service),
		metrics:  make(map[string]models.Metrics),
		subs:     make(map[int]chan models.Event),
	}
}

func (m *MemoryStore) GetService(key string) (models.Service, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.services[key]
	return s, ok
}

func (m *MemoryStore) ListServices() []models.Service {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]models.Service, 0, len(m.order))
	for _, key := range m.order {
		out = append(out, m.services[key])
	}
	return out
}

func (m *MemoryStore) SetService(s models.Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, existed := m.services[s.Key]
	if !existed {
		m.order = append(m.order, s.Key)
	}
	m.services[s.Key] = s

	typ := models.EventUpdated
	if !existed {
		typ = models.EventAdded
	}
	m.broadcast(models.Event{Type: typ, Key: s.Key, Service: &s})
}

func (m *MemoryStore) DeleteService(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.services[key]
	if ok {
		delete(m.services, key)
		for i, k := range m.order {
			if k == key {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
		delete(m.metrics, key)
		m.broadcast(models.Event{Type: models.EventRemoved, Key: key, Service: &s})
	}
}

func (m *MemoryStore) SetMetrics(key string, met models.Metrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	met.Key = key
	m.metrics[key] = met
	m.broadcast(models.Event{Type: models.EventMetrics, Key: key, Metrics: &met})
}

func (m *MemoryStore) GetMetrics(key string) (models.Metrics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	met, ok := m.metrics[key]
	return met, ok
}

func (m *MemoryStore) Subscribe(ctx context.Context) <-chan models.Event {
	ch := make(chan models.Event, 256)

	m.subMu.Lock()
	id := m.nextSub
	m.nextSub++
	m.subs[id] = ch
	m.subMu.Unlock()

	go func() {
		<-ctx.Done()
		m.subMu.Lock()
		delete(m.subs, id)
		close(ch)
		m.subMu.Unlock()
	}()

	return ch
}

// broadcast fans an event out to subscribers, dropping to a slow consumer
// rather than stalling the writer (fine here: metrics are high-frequency,
// and a full page reload reconciles any missed route event in v0).
func (m *MemoryStore) broadcast(ev models.Event) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	for _, ch := range m.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}
