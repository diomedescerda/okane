package finance_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/ai"
	"github.com/kenma/okane/internal/db"
	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/finance"
	"github.com/kenma/okane/internal/jobs"
	"github.com/kenma/okane/internal/money"
)

// fakeCategorizer records prompts and returns scripted assignments.
type fakeCategorizer struct {
	prompts     int
	lastPrompt  string
	assignments []ai.Assignment
}

func (f *fakeCategorizer) Categorize(ctx context.Context, userPrompt string) ([]ai.Assignment, error) {
	f.prompts++
	f.lastPrompt = userPrompt
	return f.assignments, nil
}

func (f *fakeCategorizer) reset() {
	f.prompts = 0
	f.lastPrompt = ""
}

func mustMoney(t *testing.T, s string) money.Amount {
	t.Helper()
	amount, err := money.Parse(s)
	if err != nil {
		t.Fatalf("parse money %q: %v", s, err)
	}
	return amount
}

// TestAICategorization exercises the AI reclassification seam against a real
// database with a scripted fake categorizer. It is skipped unless
// OKANE_DATABASE_URL is set.
func TestAICategorization(t *testing.T) {
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
	email := fmt.Sprintf("ai-test-%d@example.com", time.Now().UnixNano())
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

	account, err := svc.CreateAccount(ctx, userID, finance.NewAccount{
		Name: "AI checking",
		Type: "checking",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// System categories for assertions.
	var groceries, restaurants uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'groceries' AND user_id IS NULL`).Scan(&groceries); err != nil {
		t.Fatalf("select groceries category: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = 'restaurants' AND user_id IS NULL`).Scan(&restaurants); err != nil {
		t.Fatalf("select restaurants category: %v", err)
	}

	mustUncat := func(description string) finance.TransactionView {
		t.Helper()
		view, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
			AccountID:   account.ID,
			Description: description,
			Amount:      mustMoney(t, "45000"),
			Direction:   domain.DirectionOut,
			Currency:    "COP",
			Status:      domain.StatusPosted,
		})
		if err != nil {
			t.Fatalf("create transaction %q: %v", description, err)
		}
		return view
	}

	first := mustUncat("Carulla supermercado")
	second := mustUncat("Restaurante Wok")

	// Without a categorizer or hook the run refuses.
	if _, err := svc.ReclassifyWithAI(ctx, userID, account.ID); !errors.Is(err, finance.ErrAINotConfigured) {
		t.Fatalf("ReclassifyWithAI without client = %v, want ErrAINotConfigured", err)
	}
	if _, err := svc.EnqueueReclassification(ctx, userID, account.ID); !errors.Is(err, finance.ErrAINotConfigured) {
		t.Fatalf("EnqueueReclassification without hook = %v, want ErrAINotConfigured", err)
	}

	// Scripted model: assigns groceries to both transactions via valid ids.
	fake := &fakeCategorizer{}
	svc.SetCategorizer(fake)
	fake.assignments = []ai.Assignment{
		{TransactionID: first.ID.String(), CategoryID: groceries.String(), Slug: "groceries"},
		{TransactionID: second.ID.String(), CategoryID: groceries.String(), Slug: "groceries"},
	}
	count, err := svc.ReclassifyWithAI(ctx, userID, account.ID)
	if err != nil {
		t.Fatalf("ReclassifyWithAI: %v", err)
	}
	if count != 2 {
		t.Fatalf("reclassified = %d, want 2", count)
	}
	if fake.prompts == 0 {
		t.Fatal("categorizer never asked")
	}
	if !strings.Contains(fake.lastPrompt, groceries.String()) || !strings.Contains(fake.lastPrompt, first.ID.String()) {
		t.Errorf("prompt missing candidates or transactions:\n%s", fake.lastPrompt)
	}
	for name, txn := range map[string]finance.TransactionView{"first": first, "second": second} {
		view, err := svc.GetTransaction(ctx, userID, txn.ID)
		if err != nil {
			t.Fatalf("get %s: %v", name, err)
		}
		if view.CategoryID == nil || *view.CategoryID != groceries {
			t.Errorf("%s category = %v, want groceries %s", name, view.CategoryID, groceries)
		}
		if view.CategorySource != domain.CategorySourceAI {
			t.Errorf("%s category_source = %q, want %q", name, view.CategorySource, domain.CategorySourceAI)
		}
	}

	// Run again: nothing uncategorized left, so no AI calls at all.
	fake.reset()
	count, err = svc.ReclassifyWithAI(ctx, userID, account.ID)
	if err != nil {
		t.Fatalf("ReclassifyWithAI after full run: %v", err)
	}
	if count != 0 || fake.prompts != 0 {
		t.Fatalf("reclassified = %d with %d prompts, want 0/0", count, fake.prompts)
	}

	// User picks survive: a manual transaction is never touched, and an
	// unknown category id falls back to the echoed slug.
	third := mustUncat("Otra compra")
	fourth, err := svc.CreateTransaction(ctx, userID, finance.NewTransaction{
		AccountID:   account.ID,
		Description: "Pago usuario",
		Amount:      mustMoney(t, "45000"),
		Direction:   domain.DirectionOut,
		Currency:    "COP",
		Status:      domain.StatusPosted,
		CategoryID:  &restaurants,
	})
	if err != nil {
		t.Fatalf("create manual transaction: %v", err)
	}
	unknown := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	fake.reset()
	fake.assignments = []ai.Assignment{
		{TransactionID: third.ID.String(), CategoryID: unknown.String(), Slug: "groceries"},
		{TransactionID: fourth.ID.String(), CategoryID: groceries.String(), Slug: "groceries"},
	}
	count, err = svc.ReclassifyWithAI(ctx, userID, account.ID)
	if err != nil {
		t.Fatalf("ReclassifyWithAI with manual row: %v", err)
	}
	if count != 1 {
		t.Fatalf("reclassified = %d, want 1", count)
	}
	thirdView, err := svc.GetTransaction(ctx, userID, third.ID)
	if err != nil {
		t.Fatalf("get third: %v", err)
	}
	if thirdView.CategoryID == nil || *thirdView.CategoryID != groceries || thirdView.CategorySource != domain.CategorySourceAI {
		t.Errorf("third = %+v, want groceries/ai via slug fallback", thirdView)
	}
	fourthView, err := svc.GetTransaction(ctx, userID, fourth.ID)
	if err != nil {
		t.Fatalf("get fourth: %v", err)
	}
	if fourthView.CategoryID == nil || *fourthView.CategoryID != restaurants || fourthView.CategorySource != domain.CategorySourceManual {
		t.Errorf("fourth = %+v, want manual restaurants preserved", fourthView)
	}

	// Wiring the hook enqueues real jobs.
	jobStore := jobs.NewStore(pool)
	svc.SetImportHook(func(ctx context.Context, userID, accountID uuid.UUID) (int64, error) {
		job, err := jobStore.Enqueue(ctx, finance.JobReclassify, finance.ReclassifyPayload{
			UserID:    userID,
			AccountID: accountID,
		})
		if err != nil {
			return 0, err
		}
		return job.ID, nil
	})
	jobID, err := svc.EnqueueReclassification(ctx, userID, account.ID)
	if err != nil {
		t.Fatalf("EnqueueReclassification: %v", err)
	}
	if jobID <= 0 {
		t.Fatalf("job id = %d, want > 0", jobID)
	}
	if _, err := jobStore.Get(ctx, jobID); err != nil {
		t.Fatalf("get enqueued job: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, jobID); err != nil {
		t.Errorf("cleanup job: %v", err)
	}
}
