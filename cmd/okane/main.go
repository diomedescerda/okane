// Command okane is the Okane API server.
//
// Usage:
//
//	okane serve             start the HTTP API (applies migrations in dev)
//	okane migrate [up|down|status|version]
//	okane seed              create the demo@okane.dev dataset (idempotent)
//	okane version
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embed the timezone database for distroless images

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenma/okane/internal/ai"
	"github.com/kenma/okane/internal/auth"
	"github.com/kenma/okane/internal/config"
	"github.com/kenma/okane/internal/db"
	"github.com/kenma/okane/internal/finance"
	"github.com/kenma/okane/internal/httpapi"
	"github.com/kenma/okane/internal/jobs"
)

const version = "0.1.0-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "okane:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: okane <serve|migrate|seed|version>")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch os.Args[1] {
	case "serve":
		// Migrations are idempotent and protected by a Postgres advisory lock,
		// so applying them on startup is safe with multiple replicas.
		if err := db.Migrate(ctx, pool, "up"); err != nil {
			return fmt.Errorf("apply migrations: %w", err)
		}
		return serve(ctx, cfg, pool, log)

	case "migrate":
		direction := "up"
		if len(os.Args) > 2 {
			direction = os.Args[2]
		}
		return db.Migrate(ctx, pool, direction)

	case "seed":
		// Demo data hotels behind migrations for the system categories it
		// references.
		if err := db.Migrate(ctx, pool, "up"); err != nil {
			return fmt.Errorf("apply migrations: %w", err)
		}
		return seed(ctx, cfg, pool)

	case "version":
		fmt.Println("okane", version)
		return nil

	default:
		return fmt.Errorf("unknown command %q (want serve, migrate, seed or version)", os.Args[1])
	}
}

func serve(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, log *slog.Logger) error {
	loc, err := cfg.Location()
	if err != nil {
		return err
	}

	authSvc := auth.NewService(pool, cfg.SessionTTL)
	financeSvc := finance.NewService(finance.NewStore(pool), loc)
	jobStore := jobs.NewStore(pool)
	wireAI(cfg, financeSvc, jobStore)
	handler := httpapi.NewRouter(cfg, pool, authSvc, financeSvc, log)

	// The background worker blocks on ctx, so shutdown stops it before the
	// server drain; draining jobs are re-claimed after the lease expires.
	manager := jobs.NewManager(jobStore, jobs.Config{}, map[string]jobs.Handler{
		finance.JobReclassify: reclassifyHandler(financeSvc),
	})
	go manager.Run(ctx, log)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server_started", "addr", cfg.Addr, "env", cfg.Env, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("server_stopping")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("server_stopped")
	return nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}

// wireAI connects the AI model client and the import-time background hook when
// an API key is configured; without one the app behaves rules-only.
func wireAI(cfg *config.Config, financeSvc *finance.Service, jobStore *jobs.Store) {
	if cfg.AIAPIKey == "" {
		return
	}
	financeSvc.SetCategorizer(ai.New(ai.Config{
		BaseURL: cfg.AIBaseURL,
		APIKey:  cfg.AIAPIKey,
		Model:   cfg.AIModel,
	}))
	financeSvc.SetImportHook(func(ctx context.Context, userID, accountID uuid.UUID) (int64, error) {
		job, err := jobStore.Enqueue(ctx, finance.JobReclassify, finance.ReclassifyPayload{
			UserID:    userID,
			AccountID: accountID,
		})
		if err != nil {
			return 0, err
		}
		return job.ID, nil
	})
}

// reclassifyHandler adapts the jobs payload to the finance service; this is
// the worker side of the import and reclassify flow.
func reclassifyHandler(financeSvc *finance.Service) jobs.Handler {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p finance.ReclassifyPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("decode reclassify payload: %w", err)
		}
		count, err := financeSvc.ReclassifyWithAI(ctx, p.UserID, p.AccountID)
		if err != nil {
			return err
		}
		slog.InfoContext(ctx, "reclassification_finished", "account_id", p.AccountID, "reclassified", count)
		return nil
	}
}
