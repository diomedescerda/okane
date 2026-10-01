// Package finance implements accounts, categories, transactions and reports.
package finance

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/money"
)

// ErrNotFound is returned when a resource does not exist or belongs to a
// different user.
var ErrNotFound = errors.New("not found")

// ValidationError describes invalid input and maps to an HTTP 400 response.
type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

// Invalid builds a ValidationError.
func Invalid(format string, args ...any) error {
	return ValidationError{Message: fmt.Sprintf(format, args...)}
}

// AccountView is an account together with its computed current balance. For
// manually maintained accounts the current balance is the opening balance plus
// the sum of all transactions; for linked accounts the stored balance reported
// by the aggregator is used.
type AccountView struct {
	domain.Account
	OpeningBalance   money.Amount `json:"opening_balance"`
	TransactionCount int64        `json:"transaction_count"`
}

// TransactionView is a transaction with denormalized account and category
// fields for display.
type TransactionView struct {
	domain.Transaction
	AccountName   string  `json:"account_name"`
	CategoryName  *string `json:"category_name,omitempty"`
	CategoryIcon  *string `json:"category_icon,omitempty"`
	CategoryColor *string `json:"category_color,omitempty"`
	CategorySlug  *string `json:"category_slug,omitempty"`
}

// TransactionFilter selects and paginates a transaction list.
type TransactionFilter struct {
	UserID     uuid.UUID
	AccountID  *uuid.UUID
	CategoryID *uuid.UUID
	Direction  *domain.Direction
	Search     string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

// CategoryTotal aggregates one category (or uncategorized spending) over a
// period. Total is the signed net sum of matching transactions.
type CategoryTotal struct {
	CategoryID *uuid.UUID          `json:"category_id,omitempty"`
	Name       string              `json:"name"`
	Icon       string              `json:"icon,omitempty"`
	Color      string              `json:"color,omitempty"`
	Kind       domain.CategoryKind `json:"kind,omitempty"`
	Direction  domain.Direction    `json:"direction"`
	Total      money.Amount        `json:"total"`
	Count      int64               `json:"count"`
}

// Summary is the dashboard report for one period.
type Summary struct {
	From       time.Time       `json:"from"`
	To         time.Time       `json:"to"`
	Income     money.Amount    `json:"income"`
	Expenses   money.Amount    `json:"expenses"`
	Net        money.Amount    `json:"net"`
	NetWorth   money.Amount    `json:"net_worth"`
	ByCategory []CategoryTotal `json:"by_category"`
}

// MonthTotal is one month of the cash-flow series (month is "YYYY-MM").
type MonthTotal struct {
	Month    string       `json:"month"`
	Income   money.Amount `json:"income"`
	Expenses money.Amount `json:"expenses"`
	Net      money.Amount `json:"net"`
}

// CreateAccountParams is the store input for creating an account.
type CreateAccountParams struct {
	UserID          uuid.UUID
	Name            string
	Type            domain.AccountType
	Subtype         string
	Currency        string
	OpeningBalance  money.Amount
	InstitutionName string
}

// UpdateAccountParams is a partial update; nil fields are left unchanged.
type UpdateAccountParams struct {
	UserID          uuid.UUID
	AccountID       uuid.UUID
	Name            *string
	Type            *domain.AccountType
	Subtype         *string
	Currency        *string
	OpeningBalance  *money.Amount
	InstitutionName *string
	IsActive        *bool
}

// CreateTransactionParams is the store input for creating a transaction.
// Amount is the signed amount: negative is an outflow.
type CreateTransactionParams struct {
	UserID         uuid.UUID
	AccountID      uuid.UUID
	CategoryID     *uuid.UUID
	Description    string
	Merchant       *string
	Amount         money.Amount
	Direction      domain.Direction
	Currency       string
	Status         domain.TransactionStatus
	CategorySource domain.CategorySource
	OccurredAt     time.Time
	Notes          *string
}

// UpdateTransactionParams is a partial update; nil fields are left unchanged.
// SetCategory, SetMerchant and SetNotes distinguish "clear the value" (true
// with a nil value) from "leave unchanged" (false).
type UpdateTransactionParams struct {
	UserID         uuid.UUID
	TransactionID  uuid.UUID
	AccountID      *uuid.UUID
	CategoryID     *uuid.UUID
	SetCategory    bool
	CategorySource *domain.CategorySource
	Description    *string
	Merchant       *string
	SetMerchant    bool
	Amount         *money.Amount
	Direction      *domain.Direction
	OccurredAt     *time.Time
	Status         *domain.TransactionStatus
	Notes          *string
	SetNotes       bool
}
