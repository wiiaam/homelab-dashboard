package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/wiiaam/homelab-dashboard/internal/models"
	"github.com/wiiaam/homelab-dashboard/web/components"
)

func svc(name string, order int) models.Service {
	return models.Service{
		Key:   models.ServiceKey("default", name),
		NS:    "default",
		Name:  name,
		URL:   "http://" + name + ".local",
		Icon:  "🔷",
		Order: order,
	}
}

func TestInsertTarget(t *testing.T) {
	svcs := []models.Service{svc("apollo", 1), svc("zeus", 2), svc("hades", 4)}

	// Insert into first position.
	first := components.InsertTarget(append([]models.Service{}, svcs...), svc("ares", 0))
	if want := "afterbegin:#service-list"; first != want {
		t.Fatalf("first insert target = %q, want %q", first, want)
	}

	// Insert in the middle (order 3 → after zeus, before hades).
	middle := components.InsertTarget(append([]models.Service{}, svcs...), svc("athena", 3))
	if want := "afterend:#service-default-zeus"; middle != want {
		t.Fatalf("middle insert target = %q, want %q", middle, want)
	}

	// Append to the end.
	last := components.InsertTarget(append([]models.Service{}, svcs...), svc("poseidon", 9))
	if want := "afterend:#service-default-hades"; last != want {
		t.Fatalf("last insert target = %q, want %q", last, want)
	}

	// Empty list.
	empty := components.InsertTarget(nil, svc("ares", 0))
	if want := "afterbegin:#service-list"; empty != want {
		t.Fatalf("empty insert target = %q, want %q", empty, want)
	}
}

func TestInsertTargetSortsByOrderThenName(t *testing.T) {
	svcs := []models.Service{svc("zeta", 1), svc("alpha", 1)}
	target := components.InsertTarget(append([]models.Service{}, svcs...), svc("beta", 1))
	// beta shares order 1; alphabetical tie-break places it after alpha,
	// before zeta.
	if want := "afterend:#service-default-alpha"; target != want {
		t.Fatalf("target = %q, want %q", target, want)
	}
}

func TestTileID(t *testing.T) {
	s := models.Service{NS: "home", Name: "my-app_v1"}
	if got := components.TileID(s); got != "service-home-my-app_v1" {
		t.Fatalf("TileID = %q", got)
	}
}

func TestMetricsFragmentContainsSwap(t *testing.T) {
	frag := components.MetricsFragment(components.TileView{
		Service: svc("grafana", 1),
		Metrics: &models.Metrics{CPUMillicores: 123, MemBytes: 512 << 20},
	})

	got := renderT(t, frag)
	for _, want := range []string{`id="metrics-default-grafana"`, `hx-swap-oob="true"`, "123m CPU", "512 MiB"} {
		if !strings.Contains(got, want) {
			t.Fatalf("fragment %q missing %q", got, want)
		}
	}
}

func renderT(t *testing.T, c templ.Component) string {
	t.Helper()
	var sb strings.Builder
	if err := c.Render(context.Background(), &sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}
