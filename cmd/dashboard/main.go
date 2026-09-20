package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wiiaam/homelab-dashboard/internal/controller"
	"github.com/wiiaam/homelab-dashboard/internal/server"
	"github.com/wiiaam/homelab-dashboard/internal/store"
)

// v0: single binary sharing one MemoryStore between the controller
// (fixture-driven source), the metrics simulator, and the HTTP server.
// v1 swaps in RedisStore + informers with no code changes to render/SSE.
func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	st := store.NewMemoryStore()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go controller.New(st, controller.NewFixtureSource()).Run(ctx)
	go controller.NewSimulatedMetrics(st, 2*time.Second).Run(ctx)

	httpSrv := &http.Server{
		Addr:    *addr,
		Handler: server.New(st).Handler(),
	}

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpSrv.Shutdown(shutCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
		close(done)
	}()

	log.Printf("dashboard listening on %s", *addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
	<-done
}
