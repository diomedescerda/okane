// Package domain contains the core data model shared across the application.
package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/money"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	UserAgent  string    `json:"user_agent"`
	IP         string    `json:"ip"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
}

type LinkStatus string

const (
	LinkHealthy  LinkStatus = "healthy"
	LinkOutdated LinkStatus = "outdated"
	LinkError    LinkStatus = "error"
	LinkInvalid  LinkStatus = "invalid"
	LinkPending  LinkStatus = "pending"
)

// Link represents a connection to a bank institution (via an aggregator or
// maintained manually).
type Link struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	Provider        string     `json:"provider"`
	ProviderLinkID  *string    `json:"provider_link_id,omitempty"`
	InstitutionID   *uuid.UUID `json:"institution_id,omitempty"`
	Institution     string     `json:"institution"`
	InstitutionName string     `json:"institution_name"`
	AccessMode      string     `json:"access_mode"`
	Status          LinkStatus `json:"status"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	LastSyncError   string     `json:"last_sync_error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type InstitutionType string

const (
	InstitutionBank   InstitutionType = "bank"
	InstitutionWallet InstitutionType = "wallet"
	InstitutionOther  InstitutionType = "other"
)

// Institution is a bank or wallet that accounts and connections are grouped
// under. System institutions (UserID nil) ship with the app; users can add
// their own with a custom icon and color.
type Institution struct {
	ID                    uuid.UUID       `json:"id"`
	UserID                *uuid.UUID      `json:"user_id,omitempty"`
	Provider              string          `json:"provider"`
	ProviderInstitutionID *string         `json:"provider_institution_id,omitempty"`
	Slug                  string          `json:"slug"`
	Name                  string          `json:"name"`
	DisplayName           string          `json:"display_name"`
	CountryCode           string          `json:"country_code"`
	Type                  InstitutionType `json:"type"`
	Website               string          `json:"website,omitempty"`
	PrimaryColor          string          `json:"primary_color,omitempty"`
	LogoURL               string          `json:"logo_url,omitempty"`
	Icon                  string          `json:"icon,omitempty"`
	SortOrder             int             `json:"sort_order"`
	IsSystem              bool            `json:"is_system"`
	IsActive              bool            `json:"is_active"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

type AccountType string

const (
	AccountChecking   AccountType = "checking"
	AccountSavings    AccountType = "savings"
	AccountCreditCard AccountType = "credit_card"
	AccountLoan       AccountType = "loan"
	AccountInvestment AccountType = "investment"
	AccountOther      AccountType = "other"
)

type Account struct {
	ID               uuid.UUID     `json:"id"`
	UserID           uuid.UUID     `json:"user_id"`
	LinkID           *uuid.UUID    `json:"link_id,omitempty"`
	Provider         string        `json:"provider"`
	ProviderAccount  *string       `json:"provider_account_id,omitempty"`
	Name             string        `json:"name"`
	Type             AccountType   `json:"type"`
	Subtype          string        `json:"subtype,omitempty"`
	Currency         string        `json:"currency"`
	BalanceCurrent   money.Amount  `json:"balance_current"`
	BalanceAvailable *money.Amount `json:"balance_available,omitempty"`
	CreditLimit      *money.Amount `json:"credit_limit,omitempty"`
	InstitutionName  string        `json:"institution_name"`
	IsActive         bool          `json:"is_active"`
	LastSyncedAt     *time.Time    `json:"last_synced_at,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type CategoryKind string

const (
	CategoryExpense  CategoryKind = "expense"
	CategoryIncome   CategoryKind = "income"
	CategoryTransfer CategoryKind = "transfer"
)

type Category struct {
	ID        uuid.UUID    `json:"id"`
	UserID    *uuid.UUID   `json:"user_id,omitempty"`
	ParentID  *uuid.UUID   `json:"parent_id,omitempty"`
	Slug      string       `json:"slug"`
	Name      string       `json:"name"`
	Kind      CategoryKind `json:"kind"`
	Icon      string       `json:"icon,omitempty"`
	Color     string       `json:"color,omitempty"`
	SortOrder int          `json:"sort_order"`
	IsSystem  bool         `json:"is_system"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

type TransactionStatus string

const (
	StatusPosted  TransactionStatus = "posted"
	StatusPending TransactionStatus = "pending"
)

type CategorySource string

const (
	CategorySourceNone     CategorySource = ""
	CategorySourceRule     CategorySource = "rule"
	CategorySourceAI       CategorySource = "ai"
	CategorySourceManual   CategorySource = "manual"
	CategorySourceProvider CategorySource = "provider"
)

type Transaction struct {
	ID                    uuid.UUID         `json:"id"`
	UserID                uuid.UUID         `json:"user_id"`
	AccountID             uuid.UUID         `json:"account_id"`
	CategoryID            *uuid.UUID        `json:"category_id,omitempty"`
	Provider              string            `json:"provider"`
	ProviderTransactionID *string           `json:"provider_transaction_id,omitempty"`
	Description           string            `json:"description"`
	Merchant              *string           `json:"merchant,omitempty"`
	Amount                money.Amount      `json:"amount"`
	Direction             Direction         `json:"direction"`
	Currency              string            `json:"currency"`
	Status                TransactionStatus `json:"status"`
	CategorySource        CategorySource    `json:"category_source,omitempty"`
	OccurredAt            time.Time         `json:"occurred_at"`
	ValueDate             *time.Time        `json:"value_date,omitempty"`
	Reference             *string           `json:"reference,omitempty"`
	Notes                 *string           `json:"notes,omitempty"`
	Tags                  []string          `json:"tags"`
	Source                string            `json:"source"`
	DedupeHash            *string           `json:"-"`
	Raw                   []byte            `json:"-"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

type MatchField string

const (
	MatchFieldDescription MatchField = "description"
	MatchFieldMerchant    MatchField = "merchant"
	MatchFieldAmount      MatchField = "amount"
	MatchFieldReference   MatchField = "reference"
)

type MatchType string

const (
	MatchContains MatchType = "contains"
	MatchEquals   MatchType = "equals"
	MatchRegex    MatchType = "regex"
	MatchGT       MatchType = "gt"
	MatchLT       MatchType = "lt"
)

type Rule struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	Priority   int        `json:"priority"`
	CategoryID uuid.UUID  `json:"category_id"`
	Field      MatchField `json:"field"`
	MatchType  MatchType  `json:"match_type"`
	Pattern    string     `json:"pattern"`
	Enabled    bool       `json:"enabled"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type BudgetPeriod string

const (
	BudgetMonthly BudgetPeriod = "monthly"
	BudgetWeekly  BudgetPeriod = "weekly"
	BudgetYearly  BudgetPeriod = "yearly"
)

type Budget struct {
	ID         uuid.UUID    `json:"id"`
	UserID     uuid.UUID    `json:"user_id"`
	CategoryID *uuid.UUID   `json:"category_id,omitempty"`
	Period     BudgetPeriod `json:"period"`
	Amount     money.Amount `json:"amount"`
	Currency   string       `json:"currency"`
	StartsOn   time.Time    `json:"starts_on"`
	Enabled    bool         `json:"enabled"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

type AIReportKind string

const (
	ReportMonthlyDigest AIReportKind = "monthly_digest"
	ReportAnomalies     AIReportKind = "anomalies"
	ReportQuestion      AIReportKind = "qa"
	ReportBudgetAdvice  AIReportKind = "budget_advice"
)

type AIReport struct {
	ID          uuid.UUID    `json:"id"`
	UserID      uuid.UUID    `json:"user_id"`
	Kind        AIReportKind `json:"kind"`
	Status      string       `json:"status"`
	PeriodStart *time.Time   `json:"period_start,omitempty"`
	PeriodEnd   *time.Time   `json:"period_end,omitempty"`
	Model       string       `json:"model,omitempty"`
	Title       string       `json:"title,omitempty"`
	Content     string       `json:"content,omitempty"`
	Data        []byte       `json:"-"`
	Error       string       `json:"error,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}
