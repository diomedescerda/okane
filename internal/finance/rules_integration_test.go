package finance_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/db"
	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/finance"
	"github.com/kenma/okane/internal/money"
)

// TestRulesFlow exercises rule CRUD, automatic categorization and the
// uncategorized filter against a real database. It is skipped unless
// OKANE_DATABASE_URL is set.
func TestRulesFlow(t *testing.T) {
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
	email := fmt.Sprintf("rules-test-%d@example.com", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES ($1, 'x', 'Test') RETURNING id`, email,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	var groceries, transport uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'groceries' AND user_id IS NULL`).Scan(&groceries); err != nil {
		t.Fatalf("select groceries category: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'transport' AND user_id IS NULL`).Scan(&transport); err != nil {
		t.Fatalf("select transport category: %v", err)
	}

	// --- CRUD ---

	low := 25
	groceriesRule, err := svc.CreateRule(ctx, userID, finance.NewRule{
		CategoryID: groceries,
		Priority:   low,
		Field:      string(domain.MatchFieldDescription),
		MatchType:  string(domain.MatchContains),
		Pattern:    "supermercado",
	})
	if err != nil {
		t.Fatalf("create groceries rule: %v", err)
	}
	if groceriesRule.Priority != low || !groceriesRule.Enabled || groceriesRule.CategoryID != groceries {
		t.Fatalf("unexpected groceries rule: %+v", groceriesRule)
	}

	amountRule, err := svc.CreateRule(ctx, userID, finance.NewRule{
		CategoryID: transport,
		Priority:   50,
		Field:      string(domain.MatchFieldAmount),
		MatchType:  string(domain.MatchGT),
		Pattern:    "70000",
	})
	if err != nil {
		t.Fatalf("create amount rule: %v", err)
	}

	// Validation errors.
	if _, err := svc.CreateRule(ctx, userID, finance.NewRule{
		CategoryID: groceries, Field: string(domain.MatchFieldDescription),
		MatchType: string(domain.MatchGT), Pattern: "x",
	}); err == nil {
		t.Fatal("expected validation error for gt on description")
	}
	if _, err := svc.CreateRule(ctx, userID, finance.NewRule{
		CategoryID: groceries, Field: string(domain.MatchFieldAmount),
		MatchType: string(domain.MatchEquals), Pattern: "12.5.5",
	}); err == nil {
		t.Fatal("expected validation error for malformed amount pattern")
	}
	if _, err := svc.CreateRule(ctx, userID, finance.NewRule{
		CategoryID: uuid.New(), Field: string(domain.MatchFieldDescription),
		MatchType: string(domain.MatchContains), Pattern: "x",
	}); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown category, got %v", err)
	}

	rules, err := svc.ListRules(ctx, userID)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 2 || rules[0].ID != groceriesRule.ID || rules[1].ID != amountRule.ID {
		t.Fatalf("expected rules ordered by priority, got %+v", rules)
	}

	// Partial update: disable the groceries rule and re-enable it later.
	disabled := false
	patched, err := svc.UpdateRule(ctx, userID, groceriesRule.ID, finance.RulePatch{Enabled: &disabled})
	if err != nil {
		t.Fatalf("update rule: %v", err)
	}
	if patched.Enabled || patched.Pattern != "supermercado" {
		t.Fatalf("unexpected rule after patch: %+v", patched)
	}

	accountID := mustAccount(t, svc, userID)

	// The disabled rule must not match anymore.
	now := time.Now().UTC().Truncate(time.Second)
	tx, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID:   mustAccount(t, svc, userID),
		Description: "Supermercado Éxito",
		Amount:      money.New("1000"),
		Direction:   domain.DirectionOut,
		OccurredAt:  &now,
	})
	if err != nil {
		t.Fatalf("create transaction with disabled rule: %v", err)
	}
	if tx.CategoryID != nil {
		t.Fatalf("expected disabled rule to leave transaction uncategorized, got %v", tx.CategoryID)
	}

	// Re-enable via patch; the same pattern must now categorize on create.
	enabled := true
	if _, err := svc.UpdateRule(ctx, userID, groceriesRule.ID, finance.RulePatch{Enabled: &enabled}); err != nil {
		t.Fatalf("re-enable rule: %v", err)
	}

	// --- Automatic categorization from the rule engine ---

	matched, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID:   accountID,
		Description: "Supermercado Exito",
		Amount:      money.New("95000"),
		Direction:   domain.DirectionOut,
		OccurredAt:  &now,
	})
	if err != nil {
		t.Fatalf("create auto-categorized transaction: %v", err)
	}
	if matched.CategoryID == nil || *matched.CategoryID != groceries || matched.CategorySource != domain.CategorySourceRule {
		t.Fatalf("expected rule categorization as groceries, got %v (%q)", matched.CategoryID, matched.CategorySource)
	}

	// An explicit user pick wins over the rule engine.
	pinned, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID:   accountID,
		CategoryID:  &transport,
		Description: "Supermercado con parada en el tranporte",
		Amount:      money.New("80000"),
		Direction:   domain.DirectionOut,
		OccurredAt:  &now,
	})
	if err != nil {
		t.Fatalf("create pinned transaction: %v", err)
	}
	if pinned.CategorySource != domain.CategorySourceManual || *pinned.CategoryID != transport {
		t.Fatalf("expected manual category to win, got %v (%q)", pinned.CategoryID, pinned.CategorySource)
	}

	// --- Uncategorized filter ---

	txs, total, err := svc.ListTransactions(ctx, finance.TransactionFilter{UserID: userID, Uncategorized: true})
	if err != nil {
		t.Fatalf("list uncategorized transactions: %v", err)
	}
	if total != 1 || len(txs) != 1 || txs[0].ID != tx.ID {
		t.Fatalf("expected only the leftover transaction, got total=%d", total)
	}

	// UpdateTransaction re-applies rules on description change.
	newDescription := "Supermercado olímpico"
	recategorized, err := svc.UpdateTransaction(ctx, userID, tx.ID, finance.TransactionPatch{Description: &newDescription})
	if err != nil {
		t.Fatalf("update description: %v", err)
	}
	if recategorized.CategoryID == nil || *recategorized.CategoryID != groceries || recategorized.CategorySource != domain.CategorySourceRule {
		t.Fatalf("expected rule re-application, got %v (%q)", recategorized.CategoryID, recategorized.CategorySource)
	}

	// Re-categorizing manually must not be undone by later description edits.
	manual := &transport
	updated, err := svc.UpdateTransaction(ctx, userID, tx.ID, finance.TransactionPatch{CategoryID: &manual})
	if err != nil {
		t.Fatalf("manual categorize: %v", err)
	}
	if updated.CategorySource != domain.CategorySourceManual {
		t.Fatalf("expected manual source after explicit pick, got %q", updated.CategorySource)
	}
	anotherDescription := "Supermercado de nuevo"
	if _, err := svc.UpdateTransaction(ctx, userID, tx.ID, finance.TransactionPatch{Description: &anotherDescription}); err != nil {
		t.Fatalf("update description after manual pick: %v", err)
	}
	after, err := svc.GetTransaction(ctx, userID, tx.ID)
	if err != nil {
		t.Fatalf("get transaction: %v", err)
	}
	if after.CategoryID == nil || *after.CategoryID != transport || after.CategorySource != domain.CategorySourceManual {
		t.Fatalf("expected manual pick preserved after edit, got %v (%q)", after.CategoryID, after.CategorySource)
	}

	// --- Rules listing + delete ---

	rules, err = svc.ListRules(ctx, userID)
	if err != nil {
		t.Fatalf("list rules again: %v", err)
	}
	if rules[0].CategoryID != groceries {
		t.Fatalf("expected joined groceries category, got %v", rules[0].CategoryID)
	}

	if err := svc.DeleteRule(ctx, userID, amountRule.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	if err := svc.DeleteRule(ctx, userID, amountRule.ID); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
	rules, err = svc.ListRules(ctx, userID)
	if err != nil {
		t.Fatalf("list rules after delete: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule after delete, got %d", len(rules))
	}
}

// TestImportCategorization proves that import preview and commit apply the
// rule engine, flagging suggested categories and stamping category_source. It
// is skipped unless OKANE_DATABASE_URL is set.
func TestImportCategorization(t *testing.T) {
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
	email := fmt.Sprintf("import-rules-%d@example.com", time.Now().UnixNano())
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

	var groceries, salary uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'groceries' AND user_id IS NULL`).Scan(&groceries); err != nil {
		t.Fatalf("select groceries category: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'salary' AND user_id IS NULL`).Scan(&salary); err != nil {
		t.Fatalf("select salary category: %v", err)
	}

	account, err := svc.CreateAccount(ctx, userID, finance.NewAccount{
		Name: "Rules import checking",
		Type: "checking",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	marketRule, err := svc.CreateRule(ctx, userID, finance.NewRule{
		CategoryID: groceries,
		Field:      string(domain.MatchFieldDescription),
		MatchType:  string(domain.MatchContains),
		Pattern:    "d1",
	})
	if err != nil {
		t.Fatalf("create market rule: %v", err)
	}
	salaryRule, err := svc.CreateRule(ctx, userID, finance.NewRule{
		CategoryID: salary,
		Field:      string(domain.MatchFieldDescription),
		MatchType:  string(domain.MatchContains),
		Pattern:    "nomina",
	})
	if err != nil {
		t.Fatalf("create salary rule: %v", err)
	}
	_ = marketRule
	_ = salaryRule

	statement := "Fecha;Descripción;Valor\n" +
		"01/09/2026;Supermercado D1;-350500,50\n" +
		"02/09/2026;Pago de nomina;2500000,00\n"

	preview, err := svc.PreviewImport(ctx, userID, account.ID, "extracto.csv", []byte(statement))
	if err != nil {
		t.Fatalf("preview import: %v", err)
	}
	if preview.Rows[0].CategoryName == nil || *preview.Rows[0].CategoryName != "Groceries" ||
		preview.Rows[0].CategorySource != string(domain.CategorySourceRule) {
		t.Fatalf("expected groceries suggestion on first row: %+v", preview.Rows[0])
	}
	if preview.Rows[1].CategoryName == nil || *preview.Rows[1].CategoryName != "Salary" {
		t.Fatalf("expected salary suggestion on second row: %+v", preview.Rows[1])
	}

	first, err := svc.CommitImport(ctx, userID, account.ID, "extracto.csv", []byte(statement))
	if err != nil {
		t.Fatalf("commit import: %v", err)
	}
	if first.Imported != 2 {
		t.Fatalf("unexpected first import result: %+v", first)
	}

	txs, _, err := svc.ListTransactions(ctx, finance.TransactionFilter{UserID: userID, AccountID: &account.ID})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	byDescription := map[string]finance.TransactionView{}
	for _, t2 := range txs {
		byDescription[t2.Description] = t2
	}
	d1 := byDescription["Supermercado D1"]
	if d1.CategoryID == nil || *d1.CategoryID != groceries || d1.CategorySource != domain.CategorySourceRule {
		t.Fatalf("expected imported D1 transaction categorized by rule, got %v (%q)", d1.CategoryID, d1.CategorySource)
	}
	nomina := byDescription["Pago de nomina"]
	if nomina.CategoryID == nil || *nomina.CategoryID != salary || nomina.CategorySource != domain.CategorySourceRule {
		t.Fatalf("expected imported nomina transaction categorized by rule, got %v (%q)", nomina.CategoryID, nomina.CategorySource)
	}

	// Second preview must flag duplicates without losing the suggestions.
	preview, err = svc.PreviewImport(ctx, userID, account.ID, "extracto.csv", []byte(statement))
	if err != nil {
		t.Fatalf("preview second import: %v", err)
	}
	if preview.Existing != 2 {
		t.Fatalf("expected 2 existing rows on second preview, got %d", preview.Existing)
	}
	for _, row := range preview.Rows {
		if !row.Duplicate || row.CategorySource != string(domain.CategorySourceRule) {
			t.Fatalf("expected duplicate rows to keep the rule suggestion: %+v", row)
		}
	}
}

// mustAccount creates a fresh checking account for the user and returns its id.
func mustAccount(t *testing.T, svc *finance.Service, userID uuid.UUID) uuid.UUID {
	t.Helper()
	account, err := svc.CreateAccount(context.Background(), userID, finance.NewAccount{
		Name: "Rules checking",
		Type: "checking",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account.ID
}
