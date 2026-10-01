package finance_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/db"
	"github.com/kenma/okane/internal/finance"
	"github.com/kenma/okane/internal/money"
)

// TestImportFlow previews and imports a CSV statement twice to prove that the
// dedupe hash makes re-imports idempotent. It is skipped unless
// OKANE_DATABASE_URL is set.
func TestImportFlow(t *testing.T) {
	dsn := os.Getenv("OKANE_DATABASE_URL")
	if dsn == "" {
		t.Skip("OKANE_DATABASE_URL not set; skipping database integration test")
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	svc := finance.NewService(finance.NewStore(pool), time.UTC)

	var userID uuid.UUID
	email := fmt.Sprintf("import-test-%d@example.com", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES ($1, 'x', 'Import') RETURNING id`, email,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	account, err := svc.CreateAccount(ctx, userID, finance.NewAccount{
		Name: "Import checking",
		Type: "checking",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	statement := "Fecha;Descripción;Valor\n" +
		"01/09/2026;Supermercado Éxito;-350500,50\n" +
		"02/09/2026;Pago de nómina;2500000,00\n"

	preview, err := svc.PreviewImport(ctx, userID, account.ID, "extracto.csv", []byte(statement))
	if err != nil {
		t.Fatalf("preview import: %v", err)
	}
	if preview.Format != "csv" || preview.Total != 2 || preview.Existing != 0 || len(preview.Rows) != 2 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if preview.Rows[0].Date != "2026-09-01" || preview.Rows[0].Amount != "-350500.5" || preview.Rows[0].Direction != "out" {
		t.Fatalf("unexpected first preview row: %+v", preview.Rows[0])
	}
	if preview.Rows[0].Duplicate {
		t.Fatalf("expected first import to have no duplicates: %+v", preview.Rows[0])
	}

	first, err := svc.CommitImport(ctx, userID, account.ID, "extracto.csv", []byte(statement))
	if err != nil {
		t.Fatalf("commit import: %v", err)
	}
	if first.Total != 2 || first.Imported != 2 || first.Duplicate != 0 {
		t.Fatalf("unexpected first import result: %+v", first)
	}

	// Re-importing the same statement must not create duplicates.
	preview, err = svc.PreviewImport(ctx, userID, account.ID, "extracto.csv", []byte(statement))
	if err != nil {
		t.Fatalf("preview second import: %v", err)
	}
	if preview.Existing != 2 {
		t.Fatalf("expected 2 existing rows on second preview, got %d", preview.Existing)
	}
	for _, row := range preview.Rows {
		if !row.Duplicate {
			t.Fatalf("expected all rows flagged as duplicates: %+v", row)
		}
	}

	second, err := svc.CommitImport(ctx, userID, account.ID, "extracto.csv", []byte(statement))
	if err != nil {
		t.Fatalf("commit second import: %v", err)
	}
	if second.Imported != 0 || second.Duplicate != 2 {
		t.Fatalf("unexpected second import result: %+v", second)
	}

	accounts, err := svc.ListAccounts(ctx, userID)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	assertAmount(t, accounts[0].BalanceCurrent, "2149499.5")
	if accounts[0].TransactionCount != 2 {
		t.Fatalf("expected 2 transactions after import, got %d", accounts[0].TransactionCount)
	}

	_, total, err := svc.ListTransactions(ctx, finance.TransactionFilter{UserID: userID, AccountID: &account.ID})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 imported transactions, got %d", total)
	}

	var imported money.Amount
	if err := pool.QueryRow(ctx,
		`SELECT amount FROM transactions WHERE account_id = $1 ORDER BY occurred_at LIMIT 1`, account.ID,
	).Scan(&imported); err != nil {
		t.Fatalf("select imported transaction: %v", err)
	}
	assertAmount(t, imported, "-350500.5")
}
