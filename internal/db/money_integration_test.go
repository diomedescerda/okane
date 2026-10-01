package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kenma/okane/internal/money"
)

// TestMoneyRoundTrip verifies that money.Amount encodes to and decodes from a
// Postgres NUMERIC column with full precision. It runs only when
// OKANE_DATABASE_URL is set (see `make db-up`).
func TestMoneyRoundTrip(t *testing.T) {
	dbURL := os.Getenv("OKANE_DATABASE_URL")
	if dbURL == "" {
		t.Skip("OKANE_DATABASE_URL not set; skipping database integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire conn: %v", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `CREATE TEMP TABLE money_check (amount numeric(20,4), nullable numeric(20,4)) ON COMMIT PRESERVE ROWS`); err != nil {
		t.Fatalf("create temp table: %v", err)
	}

	cases := []money.Amount{
		money.New("-1234.5678"),
		money.New("0.01"),
		money.New("9999999999.9999"),
		money.Zero(),
	}
	for _, want := range cases {
		var got money.Amount
		var gotNullable money.Amount
		err := conn.QueryRow(ctx,
			`INSERT INTO money_check (amount, nullable) VALUES ($1, NULL) RETURNING amount, COALESCE(nullable, 0)`,
			want,
		).Scan(&got, &gotNullable)
		if err != nil {
			t.Fatalf("round trip %s: %v", want, err)
		}
		if !got.Equal(want.Decimal) {
			t.Fatalf("amount round trip: got %q, want %q", got.Decimal.String(), want.Decimal.String())
		}
		if !gotNullable.IsZero() {
			t.Fatalf("NULL round trip: got %s, want 0", gotNullable)
		}
	}
}
