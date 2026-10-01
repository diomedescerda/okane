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

// TestFinanceFlow exercises accounts, transactions, filters and reports against
// a real database. It is skipped unless OKANE_DATABASE_URL is set.
func TestFinanceFlow(t *testing.T) {
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
	email := fmt.Sprintf("finance-test-%d@example.com", time.Now().UnixNano())
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES ($1, 'x', 'Test') RETURNING id`, email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	// Categories seeded by migration 2.
	var groceries, salary uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'groceries' AND user_id IS NULL`).Scan(&groceries); err != nil {
		t.Fatalf("select groceries category: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'salary' AND user_id IS NULL`).Scan(&salary); err != nil {
		t.Fatalf("select salary category: %v", err)
	}

	// --- Accounts ---

	account, err := svc.CreateAccount(ctx, userID, finance.NewAccount{
		Name:           "Test checking",
		Type:           "checking",
		Currency:       "COP",
		OpeningBalance: money.New("100.00"),
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	assertAmount(t, account.BalanceCurrent, "100")
	assertAmount(t, account.OpeningBalance, "100")

	if _, err := svc.CreateAccount(ctx, userID, finance.NewAccount{Name: "  ", Type: "checking"}); err == nil {
		t.Fatal("expected validation error for empty account name")
	}
	if _, err := svc.CreateAccount(ctx, userID, finance.NewAccount{Name: "Bad", Type: "crypto"}); err == nil {
		t.Fatal("expected validation error for invalid account type")
	}

	// --- Transactions ---

	now := time.Now().UTC().Truncate(time.Second)

	salaryTx, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID:   account.ID,
		CategoryID:  &salary,
		Description: "Salary",
		Amount:      money.New("1000.50"),
		Direction:   domain.DirectionIn,
		OccurredAt:  &now,
	})
	if err != nil {
		t.Fatalf("create salary transaction: %v", err)
	}
	assertAmount(t, salaryTx.Amount, "1000.5")
	if salaryTx.Direction != domain.DirectionIn || salaryTx.CategorySource != domain.CategorySourceManual {
		t.Fatalf("unexpected salary transaction: direction=%q source=%q", salaryTx.Direction, salaryTx.CategorySource)
	}
	if salaryTx.CategoryName == nil || *salaryTx.CategoryName != "Salary" {
		t.Fatalf("expected joined category name, got %v", salaryTx.CategoryName)
	}

	groceriesTx, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID:   account.ID,
		CategoryID:  &groceries,
		Description: "Supermarket run",
		Amount:      money.New("100.25"),
		Direction:   domain.DirectionOut,
		OccurredAt:  &now,
	})
	if err != nil {
		t.Fatalf("create groceries transaction: %v", err)
	}
	assertAmount(t, groceriesTx.Amount, "-100.25")

	coffeeTx, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID:   account.ID,
		Description: "Coffee",
		Amount:      money.New("50.25"),
		Direction:   domain.DirectionOut,
		OccurredAt:  &now,
	})
	if err != nil {
		t.Fatalf("create coffee transaction: %v", err)
	}

	if _, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID: account.ID, Description: "Zero", Amount: money.Zero(), Direction: domain.DirectionOut, OccurredAt: &now,
	}); err == nil {
		t.Fatal("expected validation error for zero amount")
	}
	if _, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID: uuid.New(), Description: "Foreign account", Amount: money.New("1"), Direction: domain.DirectionOut, OccurredAt: &now,
	}); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for foreign account, got %v", err)
	}

	// Categorize the coffee after the fact (patch with an explicit category).
	coffeeCategory := groceries
	coffeeCategoryPtr := &coffeeCategory
	updated, err := svc.UpdateTransaction(ctx, userID, coffeeTx.ID, finance.TransactionPatch{
		CategoryID: &coffeeCategoryPtr,
	})
	if err != nil {
		t.Fatalf("categorize coffee: %v", err)
	}
	if updated.CategoryID == nil || *updated.CategoryID != groceries || updated.CategorySource != domain.CategorySourceManual {
		t.Fatalf("expected coffee categorized as groceries, got %v (%q)", updated.CategoryID, updated.CategorySource)
	}

	// Changing only the amount keeps the direction sign.
	newAmount := money.New("120.00")
	updated, err = svc.UpdateTransaction(ctx, userID, groceriesTx.ID, finance.TransactionPatch{Amount: &newAmount})
	if err != nil {
		t.Fatalf("update amount: %v", err)
	}
	assertAmount(t, updated.Amount, "-120")
	if updated.Direction != domain.DirectionOut {
		t.Fatalf("expected direction out after amount-only update, got %q", updated.Direction)
	}

	// --- Filters ---

	out := domain.DirectionOut
	txs, total, err := svc.ListTransactions(ctx, finance.TransactionFilter{UserID: userID, Direction: &out})
	if err != nil {
		t.Fatalf("list outflow transactions: %v", err)
	}
	if total != 2 || len(txs) != 2 {
		t.Fatalf("expected 2 outflow transactions, got total=%d len=%d", total, len(txs))
	}

	txs, total, err = svc.ListTransactions(ctx, finance.TransactionFilter{UserID: userID, Search: "sala"})
	if err != nil {
		t.Fatalf("search transactions: %v", err)
	}
	if total != 1 || len(txs) != 1 || txs[0].ID != salaryTx.ID {
		t.Fatalf("expected search to find the salary transaction, got total=%d", total)
	}

	txs, total, err = svc.ListTransactions(ctx, finance.TransactionFilter{UserID: userID, Limit: 1})
	if err != nil {
		t.Fatalf("paginate transactions: %v", err)
	}
	if total != 3 || len(txs) != 1 {
		t.Fatalf("expected total=3 with one page item, got total=%d len=%d", total, len(txs))
	}

	// --- Reports ---

	from := now.Add(-time.Hour)
	to := now.Add(time.Hour)
	summary, err := svc.Summary(ctx, userID, &from, &to)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	assertAmount(t, summary.Income, "1000.5")
	assertAmount(t, summary.Expenses, "170.25")
	assertAmount(t, summary.Net, "830.25")
	assertAmount(t, summary.NetWorth, "930.25")
	if len(summary.ByCategory) != 2 {
		t.Fatalf("expected 2 category totals, got %d: %+v", len(summary.ByCategory), summary.ByCategory)
	}
	byName := map[string]finance.CategoryTotal{}
	for _, ct := range summary.ByCategory {
		byName[ct.Name] = ct
	}
	assertAmount(t, byName["Groceries"].Total, "-170.25")
	if byName["Groceries"].Count != 2 {
		t.Fatalf("expected 2 groceries transactions, got %d", byName["Groceries"].Count)
	}
	assertAmount(t, byName["Salary"].Total, "1000.5")

	months, err := svc.MonthlyTotals(ctx, userID, 3)
	if err != nil {
		t.Fatalf("monthly totals: %v", err)
	}
	if len(months) != 3 {
		t.Fatalf("expected 3 months, got %d", len(months))
	}
	var monthIncome, monthExpenses money.Amount
	for _, m := range months {
		monthIncome = monthIncome.Add(m.Income)
		monthExpenses = monthExpenses.Add(m.Expenses)
	}
	assertAmount(t, monthIncome, "1000.5")
	assertAmount(t, monthExpenses, "170.25")

	accounts, err := svc.ListAccounts(ctx, userID)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	assertAmount(t, accounts[0].BalanceCurrent, "930.25")
	if accounts[0].TransactionCount != 3 {
		t.Fatalf("expected 3 transactions on the account, got %d", accounts[0].TransactionCount)
	}

	// --- Deletes ---

	if err := svc.DeleteTransaction(ctx, userID, coffeeTx.ID); err != nil {
		t.Fatalf("delete transaction: %v", err)
	}
	if err := svc.DeleteTransaction(ctx, userID, coffeeTx.ID); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}

	if err := svc.DeleteAccount(ctx, userID, account.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if _, _, err := svc.ListTransactions(ctx, finance.TransactionFilter{UserID: userID}); err != nil {
		t.Fatalf("list transactions after account delete: %v", err)
	}
	_, total, err = svc.ListTransactions(ctx, finance.TransactionFilter{UserID: userID})
	if err != nil {
		t.Fatalf("list transactions after account delete: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected transactions to be deleted with the account, got %d", total)
	}
	if err := svc.DeleteAccount(ctx, userID, account.ID); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second account delete, got %v", err)
	}
}

func assertAmount(t *testing.T, got money.Amount, want string) {
	t.Helper()
	if !got.Decimal.Equal(money.New(want).Decimal) {
		t.Fatalf("amount = %s, want %s", got, want)
	}
}

// TestInstitutionsAndLinks exercises Colombian institutions, custom
// institutions and bank connections against a real database.
func TestInstitutionsAndLinks(t *testing.T) {
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
	email := fmt.Sprintf("finance-inst-%d@example.com", time.Now().UnixNano())
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES ($1, 'x', 'Test') RETURNING id`, email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	// --- Seeded institutions ---

	all, err := svc.ListInstitutions(ctx, userID, finance.InstitutionFilter{})
	if err != nil {
		t.Fatalf("list institutions: %v", err)
	}
	if len(all) < 19 {
		t.Fatalf("expected at least 19 seeded institutions, got %d", len(all))
	}

	var bancolombia, nequi *finance.InstitutionView
	for i := range all {
		switch all[i].Slug {
		case "bancolombia":
			bancolombia = &all[i]
		case "nequi":
			nequi = &all[i]
		}
	}
	if bancolombia == nil || nequi == nil {
		t.Fatal("expected seeded Bancolombia and Nequi institutions")
	}
	if bancolombia.LogoURL == "" || bancolombia.PrimaryColor == "" {
		t.Fatalf("expected Bancolombia logo and color, got %+v", bancolombia)
	}
	if bancolombia.Type != domain.InstitutionBank || nequi.Type != domain.InstitutionWallet {
		t.Fatalf("unexpected institution types: %q, %q", bancolombia.Type, nequi.Type)
	}

	walletType := domain.InstitutionWallet
	wallets, err := svc.ListInstitutions(ctx, userID, finance.InstitutionFilter{Type: &walletType})
	if err != nil {
		t.Fatalf("list wallet institutions: %v", err)
	}
	if len(wallets) < 4 {
		t.Fatalf("expected at least 4 wallets, got %d", len(wallets))
	}

	found, err := svc.ListInstitutions(ctx, userID, finance.InstitutionFilter{Search: "nequi"})
	if err != nil {
		t.Fatalf("search institutions: %v", err)
	}
	if len(found) != 1 || found[0].Slug != "nequi" {
		t.Fatalf("expected the search to find only Nequi, got %d results", len(found))
	}

	// --- Custom institutions ---

	if _, err := svc.CreateInstitution(ctx, userID, finance.NewInstitution{DisplayName: "  "}); err == nil {
		t.Fatal("expected validation error for empty institution name")
	}
	if _, err := svc.CreateInstitution(ctx, userID, finance.NewInstitution{DisplayName: "My bank", Type: "crypto"}); err == nil {
		t.Fatal("expected validation error for invalid institution type")
	}
	if _, err := svc.CreateInstitution(ctx, userID, finance.NewInstitution{DisplayName: "My bank", PrimaryColor: "red"}); err == nil {
		t.Fatal("expected validation error for invalid color")
	}

	custom, err := svc.CreateInstitution(ctx, userID, finance.NewInstitution{
		DisplayName:  "Cooperative Bank",
		Type:         "bank",
		Website:      "https://coop.example.com",
		PrimaryColor: "#1A2B3C",
		Icon:         "🏦",
	})
	if err != nil {
		t.Fatalf("create institution: %v", err)
	}
	if custom.IsSystem || custom.UserID == nil || *custom.UserID != userID {
		t.Fatalf("unexpected custom institution: %+v", custom)
	}
	if custom.LinkCount != 0 {
		t.Fatalf("expected zero links, got %d", custom.LinkCount)
	}

	if err := svc.DeleteInstitution(ctx, userID, bancolombia.ID); err == nil {
		t.Fatal("expected built-in institutions to be undeletable")
	}

	// --- Connections ---

	link, err := svc.CreateLink(ctx, userID, bancolombia.ID)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if link.InstitutionName != "Bancolombia" || link.AccountCount != 0 {
		t.Fatalf("unexpected link: %+v", link)
	}
	if _, err := svc.CreateLink(ctx, userID, bancolombia.ID); err == nil {
		t.Fatal("expected duplicate connection to be rejected")
	}
	if _, err := svc.CreateLink(ctx, userID, uuid.New()); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown institution, got %v", err)
	}

	// An account attached to the connection inherits the bank branding.
	account, err := svc.CreateAccount(ctx, userID, finance.NewAccount{
		Name:           "Savings",
		Type:           "savings",
		OpeningBalance: money.New("100"),
		LinkID:         &link.ID,
	})
	if err != nil {
		t.Fatalf("create linked account: %v", err)
	}
	if account.LinkID == nil || *account.LinkID != link.ID {
		t.Fatalf("expected account attached to %s, got %v", link.ID, account.LinkID)
	}
	if account.InstitutionName != "Bancolombia" {
		t.Fatalf("expected institution name from the link, got %q", account.InstitutionName)
	}
	if account.InstitutionLogoURL == "" || account.InstitutionType != "bank" {
		t.Fatalf("expected joined institution fields, got %+v", account)
	}

	// Detach and reattach through the patch semantics.
	var detach *uuid.UUID
	detached, err := svc.UpdateAccount(ctx, userID, account.ID, finance.AccountPatch{LinkID: &detach})
	if err != nil {
		t.Fatalf("detach account: %v", err)
	}
	if detached.LinkID != nil {
		t.Fatalf("expected account detached, got link %v", detached.LinkID)
	}
	attach := link.ID
	attachPtr := &attach
	reattached, err := svc.UpdateAccount(ctx, userID, account.ID, finance.AccountPatch{LinkID: &attachPtr})
	if err != nil {
		t.Fatalf("reattach account: %v", err)
	}
	if reattached.LinkID == nil || *reattached.LinkID != link.ID {
		t.Fatalf("expected account reattached to %s, got %v", link.ID, reattached.LinkID)
	}

	// Deleting a connection detaches its accounts without losing data.
	if err := svc.DeleteLink(ctx, userID, link.ID); err != nil {
		t.Fatalf("delete link: %v", err)
	}
	accounts, err := svc.ListAccounts(ctx, userID)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account after detach, got %d", len(accounts))
	}
	if accounts[0].LinkID != nil {
		t.Fatalf("expected detached account, got link %v", accounts[0].LinkID)
	}
	if accounts[0].Provider != "manual" {
		t.Fatalf("expected manual provider, got %q", accounts[0].Provider)
	}
	assertAmount(t, accounts[0].BalanceCurrent, "100")
	if _, err := svc.GetLink(ctx, userID, link.ID); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected link to be deleted, got %v", err)
	}

	// Deleting a custom institution removes its connections and detaches accounts.
	customLink, err := svc.CreateLink(ctx, userID, custom.ID)
	if err != nil {
		t.Fatalf("create custom link: %v", err)
	}
	customAttach := customLink.ID
	customAttachPtr := &customAttach
	if _, err := svc.UpdateAccount(ctx, userID, account.ID, finance.AccountPatch{LinkID: &customAttachPtr}); err != nil {
		t.Fatalf("attach to custom institution: %v", err)
	}
	if err := svc.DeleteInstitution(ctx, userID, custom.ID); err != nil {
		t.Fatalf("delete custom institution: %v", err)
	}
	if _, err := svc.GetLink(ctx, userID, customLink.ID); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected custom link deleted, got %v", err)
	}
	if _, err := svc.GetInstitution(ctx, userID, custom.ID); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("expected custom institution deleted, got %v", err)
	}
	accounts, err = svc.ListAccounts(ctx, userID)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if accounts[0].LinkID != nil {
		t.Fatalf("expected detached account after institution delete, got link %v", accounts[0].LinkID)
	}

	if err := svc.DeleteAccount(ctx, userID, account.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}
}
