package models

const (
	ServicePrefix = "service:"
	MetricsPrefix = "metrics:"
)

// EventType identifies what kind of change an Event describes.
type EventType string

const (
	EventAdded   EventType = "added"
	EventUpdated EventType = "updated"
	EventRemoved EventType = "removed"
	EventMetrics EventType = "metrics"
)

// Event is a change notification emitted by the Store. Exactly one of
// Service or Metrics is set, depending on Type.
type Event struct {
	Type    EventType
	Key     string
	Service *Service
	Metrics *Metrics
}
