package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/finance"
	"github.com/kenma/okane/internal/money"
)

type financeHandler struct {
	svc *finance.Service
	log *slog.Logger
}

// fail maps service errors to HTTP responses.
func (h *financeHandler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve finance.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "validation", ve.Message)
	case errors.Is(err, finance.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	default:
		h.log.ErrorContext(r.Context(), "finance_request_failed",
			"error", err, "method", r.Method, "path", r.URL.Path)
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}

func urlUUID(r *http.Request, key string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		return uuid.Nil, finance.Invalid("invalid %s", key)
	}
	return id, nil
}

func queryUUID(r *http.Request, key string) (*uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, finance.Invalid("%s must be a valid UUID", key)
	}
	return &id, nil
}

func queryTime(r *http.Request, key string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, finance.Invalid("%s must be an RFC3339 timestamp", key)
	}
	return &t, nil
}

func queryInt(r *http.Request, key string, def int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, finance.Invalid("%s must be an integer", key)
	}
	return n, nil
}

// --- Accounts ---

type accountRequest struct {
	Name            string        `json:"name"`
	Type            string        `json:"type"`
	Currency        string        `json:"currency"`
	OpeningBalance  *money.Amount `json:"opening_balance"`
	InstitutionName string        `json:"institution_name"`
}

type accountPatchRequest struct {
	Name            *string       `json:"name"`
	Type            *string       `json:"type"`
	Subtype         *string       `json:"subtype"`
	Currency        *string       `json:"currency"`
	OpeningBalance  *money.Amount `json:"opening_balance"`
	InstitutionName *string       `json:"institution_name"`
	IsActive        *bool         `json:"is_active"`
}

func (h *financeHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	accounts, err := h.svc.ListAccounts(r.Context(), user.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (h *financeHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())

	var req accountRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	opening := money.Zero()
	if req.OpeningBalance != nil {
		opening = *req.OpeningBalance
	}

	account, err := h.svc.CreateAccount(r.Context(), user.ID, finance.NewAccount{
		Name:            req.Name,
		Type:            req.Type,
		Currency:        req.Currency,
		OpeningBalance:  opening,
		InstitutionName: req.InstitutionName,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (h *financeHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	accountID, err := urlUUID(r, "accountID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	account, err := h.svc.GetAccount(r.Context(), user.ID, accountID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (h *financeHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	accountID, err := urlUUID(r, "accountID")
	if err != nil {
		h.fail(w, r, err)
		return
	}

	var req accountPatchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	account, err := h.svc.UpdateAccount(r.Context(), user.ID, accountID, finance.AccountPatch{
		Name:            req.Name,
		Type:            req.Type,
		Subtype:         req.Subtype,
		Currency:        req.Currency,
		OpeningBalance:  req.OpeningBalance,
		InstitutionName: req.InstitutionName,
		IsActive:        req.IsActive,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (h *financeHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	accountID, err := urlUUID(r, "accountID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteAccount(r.Context(), user.ID, accountID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Categories ---

func (h *financeHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	categories, err := h.svc.ListCategories(r.Context(), user.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

// --- Transactions ---

type transactionRequest struct {
	AccountID   string       `json:"account_id"`
	CategoryID  *string      `json:"category_id"`
	Description string       `json:"description"`
	Merchant    *string      `json:"merchant"`
	Amount      money.Amount `json:"amount"`
	Direction   string       `json:"direction"`
	Currency    string       `json:"currency"`
	Status      string       `json:"status"`
	OccurredAt  *time.Time   `json:"occurred_at"`
	Notes       *string      `json:"notes"`
}

type transactionPatchRequest struct {
	AccountID   *string       `json:"account_id"`
	CategoryID  **string      `json:"category_id"`
	Description *string       `json:"description"`
	Merchant    **string      `json:"merchant"`
	Amount      *money.Amount `json:"amount"`
	Direction   *string       `json:"direction"`
	OccurredAt  *time.Time    `json:"occurred_at"`
	Status      *string       `json:"status"`
	Notes       **string      `json:"notes"`
}

func (h *financeHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())

	filter := finance.TransactionFilter{
		UserID: user.ID,
		Search: strings.TrimSpace(r.URL.Query().Get("search")),
	}
	var err error
	if filter.AccountID, err = queryUUID(r, "account_id"); err != nil {
		h.fail(w, r, err)
		return
	}
	if filter.CategoryID, err = queryUUID(r, "category_id"); err != nil {
		h.fail(w, r, err)
		return
	}
	if filter.From, err = queryTime(r, "from"); err != nil {
		h.fail(w, r, err)
		return
	}
	if filter.To, err = queryTime(r, "to"); err != nil {
		h.fail(w, r, err)
		return
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("direction")); raw != "" {
		d := domain.Direction(raw)
		if d != domain.DirectionIn && d != domain.DirectionOut {
			h.fail(w, r, finance.Invalid(`direction must be "in" or "out"`))
			return
		}
		filter.Direction = &d
	}
	if filter.Limit, err = queryInt(r, "limit", 50); err != nil {
		h.fail(w, r, err)
		return
	}
	if filter.Offset, err = queryInt(r, "offset", 0); err != nil {
		h.fail(w, r, err)
		return
	}

	transactions, total, err := h.svc.ListTransactions(r.Context(), filter)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"transactions": transactions,
		"total":        total,
		"limit":        filter.Limit,
		"offset":       filter.Offset,
	})
}

func (h *financeHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	transactionID, err := urlUUID(r, "transactionID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	transaction, err := h.svc.GetTransaction(r.Context(), user.ID, transactionID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, transaction)
}

func (h *financeHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())

	var req transactionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	accountID, err := uuid.Parse(strings.TrimSpace(req.AccountID))
	if err != nil {
		h.fail(w, r, finance.Invalid("account_id must be a valid UUID"))
		return
	}
	categoryID, err := parseOptionalUUID(req.CategoryID, "category_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}

	transaction, err := h.svc.CreateTransaction(r.Context(), user.ID, finance.NewTransaction{
		AccountID:   accountID,
		CategoryID:  categoryID,
		Description: req.Description,
		Merchant:    req.Merchant,
		Amount:      req.Amount,
		Direction:   domain.Direction(req.Direction),
		Currency:    req.Currency,
		Status:      domain.TransactionStatus(req.Status),
		OccurredAt:  req.OccurredAt,
		Notes:       req.Notes,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, transaction)
}

func (h *financeHandler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	transactionID, err := urlUUID(r, "transactionID")
	if err != nil {
		h.fail(w, r, err)
		return
	}

	var req transactionPatchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	patch := finance.TransactionPatch{
		Description: req.Description,
		Merchant:    req.Merchant,
		Amount:      req.Amount,
		OccurredAt:  req.OccurredAt,
		Notes:       req.Notes,
	}
	if req.AccountID != nil {
		accountID, err := uuid.Parse(strings.TrimSpace(*req.AccountID))
		if err != nil {
			h.fail(w, r, finance.Invalid("account_id must be a valid UUID"))
			return
		}
		patch.AccountID = &accountID
	}
	if req.CategoryID != nil {
		if *req.CategoryID == nil {
			var nilID *uuid.UUID
			patch.CategoryID = &nilID
		} else {
			categoryID, err := uuid.Parse(strings.TrimSpace(**req.CategoryID))
			if err != nil {
				h.fail(w, r, finance.Invalid("category_id must be a valid UUID"))
				return
			}
			categoryPtr := &categoryID
			patch.CategoryID = &categoryPtr
		}
	}
	if req.Direction != nil {
		d := domain.Direction(*req.Direction)
		patch.Direction = &d
	}
	if req.Status != nil {
		s := domain.TransactionStatus(*req.Status)
		patch.Status = &s
	}

	transaction, err := h.svc.UpdateTransaction(r.Context(), user.ID, transactionID, patch)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, transaction)
}

func (h *financeHandler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	transactionID, err := urlUUID(r, "transactionID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteTransaction(r.Context(), user.ID, transactionID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseOptionalUUID(raw *string, field string) (*uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return nil, finance.Invalid("%s must be a valid UUID", field)
	}
	return &id, nil
}

// --- Reports ---

func (h *financeHandler) Summary(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())

	from, err := queryTime(r, "from")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	to, err := queryTime(r, "to")
	if err != nil {
		h.fail(w, r, err)
		return
	}

	summary, err := h.svc.Summary(r.Context(), user.ID, from, to)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *financeHandler) MonthlyTotals(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())

	months, err := queryInt(r, "months", 6)
	if err != nil {
		h.fail(w, r, err)
		return
	}

	totals, err := h.svc.MonthlyTotals(r.Context(), user.ID, months)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"months": totals})
}
