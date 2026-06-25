// Command cofiswarm-rag-worker drains the RAG auto-index FHS queue on a poll loop,
// serves a /healthz endpoint, and optionally announces presence on the observer bus.
// Go port of scripts/run-worker.py (+ auto_index.py / observer.py).
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/keepdevops/cofiswarm-observer-sdk/pkg/servicecomponent"
	"github.com/keepdevops/cofiswarm-rag-worker/internal/bus"
	"github.com/keepdevops/cofiswarm-rag-worker/internal/queue"
)

func main() {
	addr := flag.String("listen", envOr("RAG_WORKER_PORT", ":8018"), "health listen address")
	poll := flag.Duration("poll", pollEnv(), "queue poll interval")
	flag.Parse()

	// Optional: announce presence on the observer bus alongside the worker (default-off).
	// COFISWARM_NATS_URL=nats://host:4222 enables it.
	var comp *servicecomponent.Component
	if url := os.Getenv("COFISWARM_NATS_URL"); url != "" {
		nc, cErr := servicecomponent.Connect(url, "cofiswarm-rag-worker")
		if cErr != nil {
			log.Printf("bus connect %s: %v (running without presence)", url, cErr)
		} else {
			defer nc.Close()
			comp = servicecomponent.New(nc, "rag-worker", "rag-worker", bus.Routes())
			if sErr := comp.Start(); sErr != nil {
				log.Printf("bus start: %v (running without presence)", sErr)
				comp = nil
			} else {
				log.Printf("rag-worker announcing presence via %s", url)
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/health", health)
	httpSrv := &http.Server{Addr: *addr, Handler: mux}
	go func() {
		log.Printf("rag-worker listening %s", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("rag-worker: server error: %v", err)
		}
	}()

	// Drain loop: process the queue every poll interval until shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go drainLoop(ctx, *poll)

	<-ctx.Done()
	log.Printf("rag-worker: shutting down")
	if comp != nil {
		comp.Shutdown() // goodbye -> offline
	}
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Printf("rag-worker: graceful shutdown: %v", err)
	}
}

func drainLoop(ctx context.Context, poll time.Duration) {
	drain := func() {
		if n, err := queue.DrainOnce(log.Printf); err != nil {
			log.Printf("drain failed: %v", err)
		} else if n > 0 {
			log.Printf("drained %d job(s)", n)
		}
	}
	drain() // first pass immediately
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			drain()
		}
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// envOr returns the env value, normalizing a bare port (e.g. "8018") to ":8018".
func envOr(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if v[0] != ':' {
		if _, err := strconv.Atoi(v); err == nil {
			return ":" + v
		}
	}
	return v
}

func pollEnv() time.Duration {
	if v := os.Getenv("RAG_WORKER_POLL_S"); v != "" {
		if secs, err := strconv.ParseFloat(v, 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second))
		}
	}
	return 5 * time.Second
}
