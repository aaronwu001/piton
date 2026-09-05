// Command piton is the orchestrator.
//
// The boot order below is fixed by SPEC.md rather than chosen:
//
//  1. Read the one YAML configuration file (SPEC.md 4.4).
//  2. Open storage, and fail fast if it is unreachable — SPEC.md 13.1 case 5:
//     "non-zero exit, and an error message that names storage as the cause".
//  3. Apply migrations to completion (SPEC.md 18.1).
//  4. Register this process and start the heartbeat and the sweep (SPEC.md 8.6,
//     8.7). SPEC.md 8.6: "startup recovery is not a separate code path — it is
//     the first sweep."
//  5. Only then bind the listener, so that a 200 from GET /healthz proves steps
//     2 and 3 finished — which is what demos/alpha's environment relies on,
//     having no migration service of its own.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aaronwu001/piton/internal/config"
	"github.com/aaronwu001/piton/internal/engine"
	"github.com/aaronwu001/piton/internal/httpapi"
	"github.com/aaronwu001/piton/internal/storage"
	"github.com/aaronwu001/piton/internal/storage/postgres"
)

func main() {
	configPath := flag.String("config", "/etc/piton/piton.yaml",
		"path to the YAML configuration file (SPEC.md 4.4)")
	flag.Parse()

	// SPEC.md 17.3: "error text is kept in the database, not only in logs.
	// Standard output stays minimal." What is logged here is what has no row to
	// live in — boot, shutdown, and failures of the machinery itself.
	logger := log.New(os.Stdout, "piton: ", log.LstdFlags|log.LUTC)

	if err := run(*configPath, logger); err != nil {
		logger.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run(configPath string, logger *log.Logger) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	store, err := openStore(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	// SPEC.md 13.1 case 5: storage unreachable at startup is a fail-fast, and
	// the message must name storage as the cause. postgres.Open does not
	// connect, so this is the first moment the answer is known.
	pingCtx, cancelPing := context.WithTimeout(context.Background(), 30*time.Second)
	err = waitForStorage(pingCtx, store, logger)
	cancelPing()
	if err != nil {
		return err
	}

	if err := store.Migrate(context.Background()); err != nil {
		return err
	}
	logger.Printf("migrations applied; storage backend %q", cfg.Storage.Backend)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	eng := engine.New(store, cfg, logger)
	if err := eng.Start(ctx); err != nil {
		return err
	}
	defer eng.Stop()

	api := httpapi.New(store, eng, logger)
	server := &http.Server{
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// The listener is opened explicitly rather than through ListenAndServe so
	// that a port already in use is reported before anything claims to be
	// serving, and so that GET /healthz cannot answer 200 one instant early.
	listener, err := net.Listen("tcp", cfg.HTTP.ListenAddress)
	if err != nil {
		return fmt.Errorf("piton: cannot listen on %s: %w", cfg.HTTP.ListenAddress, err)
	}
	logger.Printf("serving on %s", listener.Addr())

	serveErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		logger.Printf("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("http shutdown: %v", err)
	}
	return <-serveErr
}

// openStore is where SPEC.md 4.4's "the storage backend is a value in that
// file" becomes a branch. Postgres is today's only implementation; an unknown
// backend is a configuration error, never a silent fallback.
func openStore(cfg *config.Config) (storage.Store, error) {
	switch cfg.Storage.Backend {
	case "postgres":
		return postgres.Open(cfg.Storage.DSN)
	default:
		return nil, fmt.Errorf(
			"piton: storage.backend %q has no implementation in this build (SPEC.md 4.4, SPEC.md 7)",
			cfg.Storage.Backend)
	}
}

// waitForStorage retries the first connection for as long as its context
// allows.
//
// SPEC.md 13.1 case 5 requires a fail-fast when storage is unreachable at
// startup, and this is still that: the exit is non-zero and the message names
// storage. The retry window exists because a container starting beside its
// database is not the failure case case 5 describes — the compose file already
// waits for the database to report healthy, and a few seconds of tolerance
// keeps a healthy start from being reported as a broken one.
func waitForStorage(ctx context.Context, store storage.Store, logger *log.Logger) error {
	var last error
	for {
		last = store.Ping(ctx)
		if last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("piton: storage is unreachable at startup: %w", last)
		case <-time.After(time.Second):
			logger.Printf("waiting for storage: %v", last)
		}
	}
}
