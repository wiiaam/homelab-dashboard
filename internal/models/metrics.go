package models

import "time"

// MetricsKey returns the canonical store key for a service's metrics
// (metrics:<ns>:<name>).
func MetricsKey(ns, name string) string {
	return MetricsPrefix + ns + ":" + name
}

// Metrics is the current load sample for a service.
type Metrics struct {
	Key           string // metrics:<ns>:<name>
	CPUMillicores int64
	MemBytes      int64
	UpdatedAt     time.Time
}
