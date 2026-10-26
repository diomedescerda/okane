package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/ai"
	"github.com/kenma/okane/internal/domain"
)

// ErrAINotConfigured is returned when AI categorization is requested but no
// model client or import hook is wired up.
var ErrAINotConfigured = errors.New("ai categorization is not configured")

// Categorizer assigns categories to transactions with an external model; the
// prompt is built by the finance service so the contract stays in one place.
// It is satisfied by *ai.Client.
type Categorizer interface {
	Categorize(ctx context.Context, userPrompt string) ([]ai.Assignment, error)
}

// ReclassifyHook enqueues background categorization work; the jobs package is
// wired in by the command layer, so the service only holds this seam.
type ReclassifyHook func(ctx context.Context, userID, accountID uuid.UUID) (int64, error)

// JobReclassify is the jobs kind that triggers AI categorization for the
// transactions of one account.
const JobReclassify = "reclassify_account"

// ReclassifyPayload is the JSON payload stored with a reclassify job.
type ReclassifyPayload struct {
	UserID    uuid.UUID `json:"user_id"`
	AccountID uuid.UUID `json:"account_id"`
}

// SetImportHook wires the background job enqueuer; called after every import
// that leaves uncategorized transactions behind.
func (s *Service) SetImportHook(hook ReclassifyHook) { s.importHook = hook }

// SetCategorizer wires the AI client; nil (the default) leaves categorization
// to the rule engine only.
func (s *Service) SetCategorizer(c Categorizer) { s.categorizer = c }

// EnqueueReclassification queues a background job so the worker can ask the AI
// model to categorize the account's uncategorized transactions.
func (s *Service) EnqueueReclassification(ctx context.Context, userID, accountID uuid.UUID) (int64, error) {
	if s.importHook == nil {
		return 0, ErrAINotConfigured
	}
	return s.importHook(ctx, userID, accountID)
}

// Bounds for one reclassification run: AI calls stay small so a bad batch
// cannot poison the whole account.
const (
	aiBatchSize  = 30
	aiMaxBatches = 25
)

// ReclassifyWithAI asks the model to categorize the account's uncategorized
// transactions and returns how many rows it updated. It never touches rows
// with a category, so user choices and rule results are safe.
func (s *Service) ReclassifyWithAI(ctx context.Context, userID, accountID uuid.UUID) (int, error) {
	if s.categorizer == nil {
		return 0, ErrAINotConfigured
	}

	categories, err := s.store.ListCategories(ctx, userID)
	if err != nil {
		return 0, err
	}
	candidates := aiCandidates(categories)
	if len(candidates) == 0 {
		return 0, nil
	}
	byID := make(map[uuid.UUID]aiCategory, len(candidates))
	bySlug := make(map[string]aiCategory, len(candidates))
	for _, c := range candidates {
		byID[c.id] = c
		bySlug[c.slug] = c
	}

	total := 0
	for range aiMaxBatches {
		rows, err := s.store.ListUncategorizedTransactions(ctx, userID, &accountID, aiBatchSize)
		if err != nil {
			return total, err
		}
		if len(rows) == 0 {
			break
		}

		prompt := buildCategorizePrompt(candidates, rows)
		assignments, err := s.categorizer.Categorize(ctx, prompt)
		if err != nil {
			return total, err
		}

		inBatch := make(map[uuid.UUID]struct{}, len(rows))
		for _, row := range rows {
			inBatch[row.ID] = struct{}{}
		}
		assigning := make(map[uuid.UUID]uuid.UUID)
		for _, a := range assignments {
			categoryID, ok := resolveAICategory(a, byID, bySlug)
			if !ok {
				continue
			}
			transactionID, err := uuid.Parse(strings.TrimSpace(a.TransactionID))
			if err != nil {
				continue
			}
			if _, known := inBatch[transactionID]; !known {
				continue
			}
			assigning[transactionID] = categoryID
		}

		applied, err := s.store.ApplyAICategories(ctx, userID, assigning)
		if err != nil {
			return total, err
		}
		total += int(applied)
		if applied == 0 {
			// The model could not categorize this batch; retrying the same
			// rows would loop forever.
			break
		}
	}
	return total, nil
}

// aiCategory is one candidate category offered to the model.
type aiCategory struct {
	id    uuid.UUID
	slug  string
	label string
}

// aiCandidates keeps expense and income categories; transfers never apply to
// a transaction row. Labels include the parent name for two-level categories.
func aiCandidates(all []domain.Category) []aiCategory {
	names := make(map[uuid.UUID]string, len(all))
	for _, c := range all {
		names[c.ID] = c.Name
	}

	candidates := make([]aiCategory, 0, len(all))
	for _, c := range all {
		if c.Kind != domain.CategoryExpense && c.Kind != domain.CategoryIncome {
			continue
		}
		label := c.Name
		if c.ParentID != nil {
			if parent, ok := names[*c.ParentID]; ok {
				label = parent + " > " + c.Name
			}
		}
		candidates = append(candidates, aiCategory{id: c.ID, slug: c.Slug, label: label})
	}
	return candidates
}

// resolveAICategory trusts the category id first and falls back to the echoed
// slug; anything unknown is skipped.
func resolveAICategory(a ai.Assignment, byID map[uuid.UUID]aiCategory, bySlug map[string]aiCategory) (uuid.UUID, bool) {
	if id, err := uuid.Parse(strings.TrimSpace(a.CategoryID)); err == nil {
		if c, ok := byID[id]; ok {
			return c.id, true
		}
	}
	if c, ok := bySlug[strings.TrimSpace(a.Slug)]; ok {
		return c.id, true
	}
	return uuid.Nil, false
}

// buildCategorizePrompt renders the candidates and transactions for the model;
// the field-by-field format keeps parsing forgiving.
func buildCategorizePrompt(candidates []aiCategory, rows []LeanTransaction) string {
	var b strings.Builder
	b.WriteString("Categories (id | slug | name):\n")
	for _, c := range candidates {
		fmt.Fprintf(&b, "- %s | %s | %s\n", c.id, c.slug, c.label)
	}
	b.WriteString("\nTransactions (id | date | direction | amount | merchant | description):\n")
	for _, row := range rows {
		merchant := ""
		if row.Merchant != nil {
			merchant = strings.TrimSpace(*row.Merchant)
		}
		fmt.Fprintf(&b, "- %s | %s | %s | %s | %s | %s\n",
			row.ID, row.OccurredAt.Format(time.DateOnly), string(row.Direction),
			row.Amount.String(), merchant, row.Description)
	}
	b.WriteString("\nAssign one category_id to each transaction you recognize.\n")
	return b.String()
}

// maybeEnqueueReclassification fires the import hook after a successful
// import; enqueue failures are logged, never fail the import itself.
func (s *Service) maybeEnqueueReclassification(ctx context.Context, userID, accountID uuid.UUID) {
	if s.importHook == nil {
		return
	}
	if _, err := s.importHook(ctx, userID, accountID); err != nil {
		slog.WarnContext(ctx, "enqueue_reclassification_failed", "error", err, "account_id", accountID)
	}
}
