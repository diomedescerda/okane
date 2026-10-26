// Package jobs implements a small Postgres-backed work queue. Jobs are rows
// in the jobs table; workers claim them atomically with SELECT ... FOR UPDATE
// SKIP LOCKED, run a registered handler and record the outcome. Failed jobs
// are retried with exponential backoff until max_attempts, then marked dead.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Lease is how long a claimed job may stay running before another worker is
// allowed to reclaim it. It must be comfortably larger than the per-job
// handler timeout below.
const lease = 5 * time.Minute

// jobTimeout bounds a single handler execution.
const defaultJobTimeout = 3 * time.Minute

// Handler processes one job of a given kind. The payload is the JSON stored
// with the job.
type Handler func(ctx context.Context, payload json.RawMessage) error

// Config tunes the manager.
type Config struct {
	// Concurrency is the number of workers consuming the queue. Zero means 2.
	Concurrency int
	// Poll is how long an idle worker waits before polling the queue again.
	// Zero means 1s.
	Poll time.Duration
	// JobTimeout bounds a single handler execution. Zero means 3min.
	JobTimeout time.Duration
}

// Statuses written to the jobs.status column.
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
	StatusDead    = "dead"
)

// Job is one row of the jobs table.
type Job struct {
	ID          int64           `json:"id"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Priority    int             `json:"priority"`
	RunAt       time.Time       `json:"run_at"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	LastError   string          `json:"last_error"`
	CreatedAt   time.Time       `json:"created_at"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
}

// Store owns the jobs table.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a store over the given pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Enqueue schedules a job of the given kind to run as soon as possible. The
// payload is encoded as JSON.
func (s *Store) Enqueue(ctx context.Context, kind string, payload any) (Job, error) {
	runAt := time.Time{} // zero -> "now()" in SQL
	return s.enqueue(ctx, kind, payload, runAt)
}

// enqueue schedules a job; a zero runAt means immediate availability.
func (s *Store) enqueue(ctx context.Context, kind string, payload any, runAt time.Time) (Job, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Job{}, fmt.Errorf("jobs: encode payload: %w", err)
	}
	const sql = `INSERT INTO jobs (kind, payload, run_at)
		VALUES ($1, $2, CASE WHEN $3::timestamptz IS NULL THEN now() ELSE $3::timestamptz END)
		RETURNING id, kind, payload, status, priority, run_at, attempts, max_attempts, last_error, created_at`
	var job Job
	if err := s.pool.QueryRow(ctx, sql, kind, data, nilIfZero(runAt)).Scan(
		&job.ID, &job.Kind, &job.Payload, &job.Status, &job.Priority, &job.RunAt,
		&job.Attempts, &job.MaxAttempts, &job.LastError, &job.CreatedAt,
	); err != nil {
		return Job{}, fmt.Errorf("jobs: enqueue %q: %w", kind, err)
	}
	return job, nil
}

// claim atomically hands the next runnable job to workerID. It first reclaims
// stale running jobs whose lease expired, then picks the pending job with the
// lowest priority value (ties broken by run_at, then id). A nil Job means the
// queue is empty.
func (s *Store) claim(ctx context.Context, workerID string, lease time.Duration) (*Job, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("jobs: begin claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const reclaim = `UPDATE jobs SET status = 'pending', locked_at = NULL, locked_by = NULL
		WHERE status = 'running' AND locked_at < now() - $1::interval`
	if _, err := tx.Exec(ctx, reclaim, lease); err != nil {
		return nil, fmt.Errorf("jobs: reclaim stale: %w", err)
	}

	const sql = `UPDATE jobs SET status = 'running', attempts = attempts + 1,
			locked_at = now(), locked_by = $1
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = 'pending' AND run_at <= now()
			ORDER BY priority ASC, run_at ASC, id ASC
			FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING id, kind, payload, status, priority, run_at, attempts, max_attempts, last_error, created_at`
	var job Job
	err = tx.QueryRow(ctx, sql, workerID).Scan(
		&job.ID, &job.Kind, &job.Payload, &job.Status, &job.Priority, &job.RunAt,
		&job.Attempts, &job.MaxAttempts, &job.LastError, &job.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("jobs: claim: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("jobs: commit claim: %w", err)
	}
	return &job, nil
}

// complete marks a successfully handled job as done.
func (s *Store) complete(ctx context.Context, id int64) error {
	const sql = `UPDATE jobs SET status = 'done', finished_at = now(), locked_at = NULL, locked_by = NULL WHERE id = $1`
	return exec(ctx, s.pool, sql, id)
}

// release puts a failed job back in the queue with exponential backoff
// (10s, 20s, 40s ... capped at 10 minutes) or, when attempts are exhausted,
// marks it dead.
func (s *Store) release(ctx context.Context, id int64, jobErr error) error {
	msg := truncate(jobErr.Error(), 2000)
	const sql = `UPDATE jobs SET
			status = CASE WHEN attempts >= max_attempts THEN 'dead' ELSE 'pending' END,
			run_at = CASE WHEN attempts >= max_attempts THEN run_at
				ELSE now() + make_interval(secs => least(600, 10 * pow(2, attempts - 1)::double precision)) END,
			last_error = $2, locked_at = NULL, locked_by = NULL,
			finished_at = CASE WHEN attempts >= max_attempts THEN now() ELSE finished_at END
		WHERE id = $1`
	return exec(ctx, s.pool, sql, id, msg)
}

// dead marks a job as dead without honouring attempts, e.g. for jobs with no
// registered handler.
func (s *Store) dead(ctx context.Context, id int64, reason string) error {
	const sql = `UPDATE jobs SET status = 'dead', last_error = $2, finished_at = now(),
		locked_at = NULL, locked_by = NULL WHERE id = $1`
	return exec(ctx, s.pool, sql, id, truncate(reason, 2000))
}

// Get loads one job by id.
func (s *Store) Get(ctx context.Context, id int64) (Job, error) {
	const sql = `SELECT id, kind, payload, status, priority, run_at, attempts, max_attempts, last_error, created_at, finished_at
		FROM jobs WHERE id = $1`
	var job Job
	if err := s.pool.QueryRow(ctx, sql, id).Scan(
		&job.ID, &job.Kind, &job.Payload, &job.Status, &job.Priority, &job.RunAt,
		&job.Attempts, &job.MaxAttempts, &job.LastError, &job.CreatedAt, &job.FinishedAt,
	); err != nil {
		return Job{}, fmt.Errorf("jobs: get %d: %w", id, err)
	}
	return job, nil
}

// Manager runs the workers.
type Manager struct {
	store       *Store
	handlers    map[string]Handler
	concurrency int
	poll        time.Duration
	timeout     time.Duration
	workerID    string
}

// NewManager wires a job manager. Handlers must be registered before Run; a
// wildcard registration is not supported.
func NewManager(store *Store, cfg Config, handlers map[string]Handler) *Manager {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 2
	}
	if cfg.Poll <= 0 {
		cfg.Poll = time.Second
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = defaultJobTimeout
	}
	if handlers == nil {
		handlers = map[string]Handler{}
	}
	return &Manager{
		store:       store,
		handlers:    handlers,
		concurrency: cfg.Concurrency,
		poll:        cfg.Poll,
		timeout:     cfg.JobTimeout,
		workerID:    workerID(),
	}
}

// Register adds a handler for kind.
func (m *Manager) Register(kind string, h Handler) { m.handlers[kind] = h }

// Run blocks until ctx is cancelled or every worker errors fatally, processing
// jobs sequentially per worker.
func (m *Manager) Run(ctx context.Context, log *slog.Logger) error {
	var wg sync.WaitGroup
	for i := 0; i < m.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				ran, err := m.RunOnce(ctx)
				if err != nil {
					log.Error("jobs_worker", "error", err, "worker", m.workerID)
				}
				if !ran {
					select {
					case <-ctx.Done():
						return
					case <-time.After(m.poll):
					}
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

// RunOnce claims and processes at most one job. It returns false when the
// queue had nothing runnable.
func (m *Manager) RunOnce(ctx context.Context) (bool, error) {
	job, err := m.store.claim(ctx, m.workerID, lease)
	if err != nil {
		return false, fmt.Errorf("claim job: %w", err)
	}
	if job == nil {
		return false, nil
	}

	handler, ok := m.handlers[job.Kind]
	if !ok {
		err := m.store.dead(ctx, job.ID, fmt.Sprintf("no handler registered for kind %q", job.Kind))
		if err != nil {
			return true, err
		}
		return true, nil
	}

	runCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	err = safeHandle(runCtx, handler, job.Payload)
	switch {
	case err == nil:
		err = m.store.complete(ctx, job.ID)
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		// The job timed out but the manager is still alive: release with backoff.
		err = m.store.release(ctx, job.ID, fmt.Errorf("timed out after %s: %w", m.timeout, runCtx.Err()))
	default:
		err = m.store.release(ctx, job.ID, err)
	}
	if err != nil {
		return true, fmt.Errorf("finish job %d (%s): %w", job.ID, job.Kind, err)
	}
	return true, nil
}

// safeHandle runs the handler with panic recovery.
func safeHandle(ctx context.Context, h Handler, payload json.RawMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()
	return h(ctx, payload)
}

// workerID identifies this process in locked_by.
func workerID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%s:%d:%d", host, os.Getpid(), time.Now().UnixNano())
}

func nilIfZero(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func exec(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) error {
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("jobs: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
