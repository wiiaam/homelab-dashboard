package store

import (
	"context"
	"testing"
	"time"

	"github.com/wiiaam/homelab-dashboard/internal/models"
)

func testService() models.Service {
	return models.Service{
		Key:   models.ServiceKey("default", "grafana"),
		NS:    "default",
		Name:  "grafana",
		URL:   "http://grafana.local",
		Icon:  "📊",
		Order: 1,
	}
}

func TestMemoryStoreServiceLifecycle(t *testing.T) {
	st := NewMemoryStore()
	svc := testService()

	if _, ok := st.GetService(svc.Key); ok {
		t.Fatal("service should not exist before SetService")
	}

	st.SetService(svc)
	got, ok := st.GetService(svc.Key)
	if !ok {
		t.Fatal("service should exist after SetService")
	}
	if got != svc {
		t.Fatalf("got %v, want %v", got, svc)
	}
	if got := st.ListServices(); len(got) != 1 || got[0] != svc {
		t.Fatalf("ListServices = %v, want [%v]", got, svc)
	}

	st.DeleteService(svc.Key)
	if _, ok := st.GetService(svc.Key); ok {
		t.Fatal("service should be deleted")
	}
	if got := st.ListServices(); len(got) != 0 {
		t.Fatalf("ListServices = %v, want empty", got)
	}
}

func TestMemoryStoreSubscribe(t *testing.T) {
	st := NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := st.Subscribe(ctx)
	svc := testService()

	st.SetService(svc)
	select {
	case ev := <-ch:
		if ev.Type != models.EventAdded || ev.Service == nil {
			t.Fatalf("got %+v, want added event with service", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for added event")
	}

	st.DeleteService(svc.Key)
	select {
	case ev := <-ch:
		if ev.Type != models.EventRemoved {
			t.Fatalf("got type %q, want removed", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for removed event")
	}

	st.SetMetrics(svc.Key, models.Metrics{CPUMillicores: 100, MemBytes: 1 << 30})
	select {
	case ev := <-ch:
		if ev.Type != models.EventMetrics || ev.Metrics == nil {
			t.Fatalf("got %+v, want metrics event", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for metrics event")
	}
}

func TestMemoryStoreSubscribeClosedOnCancel(t *testing.T) {
	st := NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	ch := st.Subscribe(ctx)

	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should be closed after cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after cancel")
	}
}

func TestMemoryStoreSubscriberDoesNotBlock(t *testing.T) {
	st := NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st.Subscribe(ctx)

	// Stop reading; SetMetrics should still return immediately because the
	// fan-out is non-blocking.
	for i := 0; i < 512; i++ {
		st.SetMetrics("burn", models.Metrics{CPUMillicores: int64(i)})
	}
}
