package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kenma/okane/internal/db"
	"github.com/kenma/okane/internal/jobs"
)

// TestJobQueueLifecycle exercises enqueue, claim, success, failure retry,
// exhaustion and stale-lease reclaim against a real database. It is skipped
// unless OKANE_DATABASE_URL is set.
func TestJobQueueLifecycle(t *testing.T) {
	dsn := os.Getenv("OKANE_DATABASE_URL")
	if dsn == "" {
		t.Skip("OKANE_DATABASE_URL not set; skipping database integration test")
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	// Registered before the jobs cleanup so it runs LAST (cleanups run LIFO).
	t.Cleanup(pool.Close)

	// Sweep leftovers from earlier failed runs.
	if _, err := pool.Exec(ctx, `DELETE FROM jobs WHERE kind LIKE 'jobs_test-%'`); err != nil {
		t.Fatalf("sweep leftover jobs: %v", err)
	}

	kind := fmt.Sprintf("jobs_test-echo-%d", time.Now().UnixNano())
	failKind := fmt.Sprintf("jobs_test-fail-%d", time.Now().UnixNano())
	staleKind := fmt.Sprintf("jobs_test-stale-%d", time.Now().UnixNano())

	store := jobs.NewStore(pool)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM jobs WHERE kind IN ($1, $2, $3)`, kind, failKind, staleKind); err != nil {
			t.Errorf("cleanup jobs: %v", err)
		}
	})

	var calls atomic.Int32
	payload := struct{ Greeting string }{Greeting: "hola"}

	// --- Success path ---

	job, err := store.Enqueue(ctx, kind, payload)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if job.Status != jobs.StatusPending || job.ID == 0 {
		t.Fatalf("unexpected enqueued job: %+v", job)
	}

	manager := jobs.NewManager(store, jobs.Config{Concurrency: 1, Poll: time.Millisecond}, map[string]jobs.Handler{
		kind: func(ctx context.Context, p json.RawMessage) error {
			calls.Add(1)
			var got struct{ Greeting string }
			if err := json.Unmarshal(p, &got); err != nil {
				return fmt.Errorf("decode payload: %w", err)
			}
			if got != payload {
				return fmt.Errorf("payload mismatch: got %+v want %+v", got, payload)
			}
			return nil
		},
		failKind: func(ctx context.Context, p json.RawMessage) error {
			return errors.New("boom")
		},
		staleKind: func(ctx context.Context, p json.RawMessage) error {
			calls.Add(1)
			return nil
		},
	})

	if ran, err := manager.RunOnce(ctx); err != nil || !ran {
		t.Fatalf("expected to run one job (ran=%v err=%v)", ran, err)
	}
	got, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if calls.Load() != 1 || got.Status != jobs.StatusDone || got.FinishedAt == nil {
		t.Fatalf("expected done job after run, got %+v (calls=%d)", got, calls.Load())
	}

	// An empty queue reports nothing to do.
	if ran, err := manager.RunOnce(ctx); err != nil || ran {
		t.Fatalf("expected idle run once (ran=%v err=%v)", ran, err)
	}

	// --- Unknown kind is dead-lettered immediately ---

	orphanKind := fmt.Sprintf("jobs_test-orphan-%d", time.Now().UnixNano())
	orphan, err := store.Enqueue(ctx, orphanKind, payload)
	if err != nil {
		t.Fatalf("enqueue orphan: %v", err)
	}
	if _, err := manager.RunOnce(ctx); err != nil {
		t.Fatalf("run orphan: %v", err)
	}
	got, err = store.Get(ctx, orphan.ID)
	if err != nil {
		t.Fatalf("get orphan: %v", err)
	}
	if got.Status != jobs.StatusDead {
		t.Fatalf("expected dead status for unhandled kind, got %+v", got)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM jobs WHERE kind = $1`, orphanKind)
	})

	// --- Retry with backoff, then dead ---

	failing, err := store.Enqueue(ctx, failKind, payload)
	if err != nil {
		t.Fatalf("enqueue failing: %v", err)
	}
	if _, err := manager.RunOnce(ctx); err != nil {
		t.Fatalf("run failing job: %v", err)
	}
	got, err = store.Get(ctx, failing.ID)
	if err != nil {
		t.Fatalf("get failing job: %v", err)
	}
	if got.Status != jobs.StatusPending || got.Attempts != 1 || got.LastError != "boom" {
		t.Fatalf("expected pending retry with error, got %+v", got)
	}
	if !got.RunAt.After(time.Now().Add(5 * time.Second)) {
		t.Fatalf("expected backoff delay in the future, got run_at=%s", got.RunAt)
	}

	// Exhaust attempts and verify the dead state.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET max_attempts = attempts, run_at = now() WHERE id = $1`, failing.ID); err != nil {
		t.Fatalf("force max_attempts: %v", err)
	}
	if _, err := manager.RunOnce(ctx); err != nil {
		t.Fatalf("run exhausted job: %v", err)
	}
	got, err = store.Get(ctx, failing.ID)
	if err != nil {
		t.Fatalf("get exhausted job: %v", err)
	}
	if got.Status != jobs.StatusDead || got.FinishedAt == nil {
		t.Fatalf("expected dead after exhausting attempts, got %+v", got)
	}

	// --- Stale lease reclaim ---

	stale, err := store.Enqueue(ctx, staleKind, payload)
	if err != nil {
		t.Fatalf("enqueue stale: %v", err)
	}
	// Pretend a dead worker claimed it a while ago.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'running', locked_at = now() - interval '10 minutes', locked_by = 'ghost' WHERE id = $1`, stale.ID); err != nil {
		t.Fatalf("simulate stale job: %v", err)
	}
	if _, err := manager.RunOnce(ctx); err != nil {
		t.Fatalf("run stale job: %v", err)
	}
	got, err = store.Get(ctx, stale.ID)
	if err != nil {
		t.Fatalf("get stale job: %v", err)
	}
	if got.Status != jobs.StatusDone || calls.Load() < 1 {
		t.Fatalf("expected stale job reclaimed and completed, got %+v (calls=%d)", got, calls.Load())
	}

	// --- Job timeout releases with backoff ---

	slowKind := fmt.Sprintf("jobs_test-slow-%d", time.Now().UnixNano())
	slow, err := store.Enqueue(ctx, slowKind, payload)
	if err != nil {
		t.Fatalf("enqueue slow: %v", err)
	}
	slowManager := jobs.NewManager(store, jobs.Config{Concurrency: 1, JobTimeout: 200 * time.Millisecond}, map[string]jobs.Handler{
		slowKind: func(ctx context.Context, p json.RawMessage) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second): // longer than the test job timeout
				return nil
			}
		},
	})
	if _, err := slowManager.RunOnce(ctx); err != nil {
		t.Fatalf("run slow job: %v", err)
	}
	got, err = store.Get(ctx, slow.ID)
	if err != nil {
		t.Fatalf("get slow job: %v", err)
	}
	if got.Status != jobs.StatusPending || got.LastError == "" {
		t.Fatalf("expected timed-out job pending with error, got %+v", got)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM jobs WHERE kind = $1`, slowKind)
	})
}
