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

// ErrAlreadyExists is returned when a unique constraint would be violated.
var ErrAlreadyExists = errors.New("already exists")

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

	// Institution presentation fields joined through the account's connection.
	InstitutionLogoURL string `json:"institution_logo_url,omitempty"`
	InstitutionColor   string `json:"institution_color,omitempty"`
	InstitutionIcon    string `json:"institution_icon,omitempty"`
	InstitutionType    string `json:"institution_type,omitempty"`
}

// InstitutionView is a bank or wallet with the number of the user's
// connections that point at it.
type InstitutionView struct {
	domain.Institution
	LinkCount int64 `json:"link_count"`
}

// LinkView is a bank connection with denormalized institution presentation
// fields and the number of accounts attached to it.
type LinkView struct {
	domain.Link
	InstitutionLogoURL string `json:"institution_logo_url,omitempty"`
	InstitutionColor   string `json:"institution_primary_color,omitempty"`
	InstitutionIcon    string `json:"institution_icon,omitempty"`
	InstitutionType    string `json:"institution_type,omitempty"`
	AccountCount       int64  `json:"account_count"`
}

// InstitutionFilter selects institutions visible to a user.
type InstitutionFilter struct {
	UserID  uuid.UUID
	Country string
	Type    *domain.InstitutionType
	Search  string
}

// CreateInstitutionParams is the store input for a custom institution.
type CreateInstitutionParams struct {
	UserID       uuid.UUID
	Name         string
	DisplayName  string
	Type         domain.InstitutionType
	Website      string
	PrimaryColor string
	Icon         string
}

// CreateLinkParams is the store input for a connection to an institution.
type CreateLinkParams struct {
	UserID        uuid.UUID
	InstitutionID uuid.UUID
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
	LinkID          *uuid.UUID
	Name            string
	Type            domain.AccountType
	Subtype         string
	Currency        string
	OpeningBalance  money.Amount
	InstitutionName string
}

// UpdateAccountParams is a partial update; nil fields are left unchanged.
// SetLink distinguishes "detach the account" (true with a nil LinkID) from
// "leave the connection unchanged" (false).
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
	SetLink         bool
	LinkID          *uuid.UUID
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

// ImportedTransaction is a parsed statement row ready to be stored. Amount is
// signed, negative for outflows.
type ImportedTransaction struct {
	Description string
	Merchant    *string
	Reference   *string
	Amount      money.Amount
	Direction   domain.Direction
	OccurredAt  time.Time
	DedupeHash  string
}

// ImportInsertParams inserts a batch of imported transactions for one account.
type ImportInsertParams struct {
	UserID       uuid.UUID
	AccountID    uuid.UUID
	Currency     string
	Transactions []ImportedTransaction
}
