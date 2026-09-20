// Package components holds templ UI components shared by the full page
// render and the SSE/OOB-swap fragments.
package components

import (
	"fmt"
	"strings"

	"github.com/wiiaam/homelab-dashboard/internal/models"
)

// TileView is the render model for a service tile: the service plus its
// latest metrics sample, which may be absent (nil) before the first poll.
type TileView struct {
	Service models.Service
	Metrics *models.Metrics
}

// TileID is the DOM id of a service tile.
func TileID(s models.Service) string {
	return "service-" + dashed(s.NS) + "-" + dashed(s.Name)
}

// MetricsID is the DOM id of a tile's metrics span.
func MetricsID(s models.Service) string {
	return "metrics-" + dashed(s.NS) + "-" + dashed(s.Name)
}

// InsertTarget computes the hx-swap-oob value that places a newly added
// service in its correct sorted position: afterbegin of the grid list when
// it sorts first, otherwise afterend of the preceding tile.
func InsertTarget(services []models.Service, toInsert models.Service) string {
	models.SortServices(services)
	for i := range services {
		if orderLess(services[i], toInsert) {
			continue
		}
		if i == 0 {
			return "afterbegin:#service-list"
		}
		return "afterend:#" + TileID(services[i-1])
	}
	if len(services) > 0 {
		return "afterend:#" + TileID(services[len(services)-1])
	}
	return "afterbegin:#service-list"
}

// orderLess mirrors models.SortServices' ordering: order ascending, then
// name.
func orderLess(a, b models.Service) bool {
	if a.Order != b.Order {
		return a.Order < b.Order
	}
	return a.Name < b.Name
}

func metricsText(v TileView) string {
	if v.Metrics == nil {
		return "no data yet"
	}
	return fmt.Sprintf("%dm CPU · %d MiB", v.Metrics.CPUMillicores, v.Metrics.MemBytes>>20)
}

func dashed(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
