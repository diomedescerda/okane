package httpapi

import (
	"encoding/json"
	"errors"
	"io"
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
	case errors.Is(err, finance.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "conflict", "resource already exists")
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

// nullableString distinguishes a missing JSON key from an explicit null value.
// With a plain **string both cases arrive as an outer nil, so patch requests
// could never clear a field.
type nullableString struct {
	Present bool
	Value   *string
}

func (n *nullableString) UnmarshalJSON(data []byte) error {
	n.Present = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n.Value = &s
	return nil
}

type accountRequest struct {
	Name            string        `json:"name"`
	Type            string        `json:"type"`
	Currency        string        `json:"currency"`
	OpeningBalance  *money.Amount `json:"opening_balance"`
	InstitutionName string        `json:"institution_name"`
	LinkID          *string       `json:"link_id"`
}

type accountPatchRequest struct {
	Name            *string        `json:"name"`
	Type            *string        `json:"type"`
	Subtype         *string        `json:"subtype"`
	Currency        *string        `json:"currency"`
	OpeningBalance  *money.Amount  `json:"opening_balance"`
	InstitutionName *string        `json:"institution_name"`
	IsActive        *bool          `json:"is_active"`
	LinkID          nullableString `json:"link_id"`
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

	var linkID *uuid.UUID
	if req.LinkID != nil {
		id, err := uuid.Parse(*req.LinkID)
		if err != nil {
			h.fail(w, r, finance.Invalid("link_id must be a valid UUID"))
			return
		}
		linkID = &id
	}

	account, err := h.svc.CreateAccount(r.Context(), user.ID, finance.NewAccount{
		Name:            req.Name,
		Type:            req.Type,
		Currency:        req.Currency,
		OpeningBalance:  opening,
		InstitutionName: req.InstitutionName,
		LinkID:          linkID,
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

	var linkID **uuid.UUID
	if req.LinkID.Present {
		if req.LinkID.Value == nil {
			var detach *uuid.UUID
			linkID = &detach
		} else {
			id, err := uuid.Parse(strings.TrimSpace(*req.LinkID.Value))
			if err != nil {
				h.fail(w, r, finance.Invalid("link_id must be a valid UUID"))
				return
			}
			attach := &id
			linkID = &attach
		}
	}

	account, err := h.svc.UpdateAccount(r.Context(), user.ID, accountID, finance.AccountPatch{
		Name:            req.Name,
		Type:            req.Type,
		Subtype:         req.Subtype,
		Currency:        req.Currency,
		OpeningBalance:  req.OpeningBalance,
		InstitutionName: req.InstitutionName,
		IsActive:        req.IsActive,
		LinkID:          linkID,
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
	AccountID   *string        `json:"account_id"`
	CategoryID  nullableString `json:"category_id"`
	Description *string        `json:"description"`
	Merchant    nullableString `json:"merchant"`
	Amount      *money.Amount  `json:"amount"`
	Direction   *string        `json:"direction"`
	OccurredAt  *time.Time     `json:"occurred_at"`
	Status      *string        `json:"status"`
	Notes       nullableString `json:"notes"`
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
		Amount:      req.Amount,
		OccurredAt:  req.OccurredAt,
	}
	if req.Merchant.Present {
		patch.Merchant = &req.Merchant.Value
	}
	if req.Notes.Present {
		patch.Notes = &req.Notes.Value
	}
	if req.AccountID != nil {
		accountID, err := uuid.Parse(strings.TrimSpace(*req.AccountID))
		if err != nil {
			h.fail(w, r, finance.Invalid("account_id must be a valid UUID"))
			return
		}
		patch.AccountID = &accountID
	}
	if req.CategoryID.Present {
		if req.CategoryID.Value == nil {
			var nilID *uuid.UUID
			patch.CategoryID = &nilID
		} else {
			categoryID, err := uuid.Parse(strings.TrimSpace(*req.CategoryID.Value))
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

// --- Institutions ---

type institutionRequest struct {
	DisplayName  string `json:"display_name"`
	Type         string `json:"type"`
	Website      string `json:"website"`
	PrimaryColor string `json:"primary_color"`
	Icon         string `json:"icon"`
}

func (h *financeHandler) ListInstitutions(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())

	filter := finance.InstitutionFilter{
		Country: strings.TrimSpace(r.URL.Query().Get("country")),
		Search:  strings.TrimSpace(r.URL.Query().Get("search")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("type")); raw != "" {
		t := domain.InstitutionType(raw)
		filter.Type = &t
	}

	institutions, err := h.svc.ListInstitutions(r.Context(), user.ID, filter)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"institutions": institutions})
}

func (h *financeHandler) CreateInstitution(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())

	var req institutionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	institution, err := h.svc.CreateInstitution(r.Context(), user.ID, finance.NewInstitution{
		DisplayName:  req.DisplayName,
		Type:         req.Type,
		Website:      req.Website,
		PrimaryColor: req.PrimaryColor,
		Icon:         req.Icon,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, institution)
}

func (h *financeHandler) DeleteInstitution(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	institutionID, err := urlUUID(r, "institutionID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteInstitution(r.Context(), user.ID, institutionID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Connections ---

func (h *financeHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	links, err := h.svc.ListLinks(r.Context(), user.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

type linkRequest struct {
	InstitutionID string `json:"institution_id"`
}

func (h *financeHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())

	var req linkRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	institutionID, err := uuid.Parse(strings.TrimSpace(req.InstitutionID))
	if err != nil {
		h.fail(w, r, finance.Invalid("institution_id must be a valid UUID"))
		return
	}

	link, err := h.svc.CreateLink(r.Context(), user.ID, institutionID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (h *financeHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	linkID, err := urlUUID(r, "linkID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteLink(r.Context(), user.ID, linkID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ImportStatement previews or commits a CSV/OFX bank statement for an account.
// The multipart form must contain a "file" part; pass commit=true to persist.
func (h *financeHandler) ImportStatement(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	accountID, err := urlUUID(r, "accountID")
	if err != nil {
		h.fail(w, r, err)
		return
	}

	if err := r.ParseMultipartForm(maxImportBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "expected a multipart form with a file field")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", `missing "file" field`)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImportBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "unreadable_file", "could not read the uploaded file")
		return
	}

	commit := r.FormValue("commit") == "true" || r.FormValue("commit") == "1"
	if commit {
		result, err := h.svc.CommitImport(r.Context(), user.ID, accountID, header.Filename, data)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"result": result})
		return
	}

	preview, err := h.svc.PreviewImport(r.Context(), user.ID, accountID, header.Filename, data)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preview": preview})
}

const maxImportBytes = 10 << 20
