package finance

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/money"
)

// Service implements account, category, transaction and report operations with
// validation on top of the store.
type Service struct {
	store *Store
	loc   *time.Location

	// categorizer and importHook are optional seams wired by the command
	// layer (see ai.go).
	categorizer Categorizer
	importHook  ReclassifyHook
}

func NewService(store *Store, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{store: store, loc: loc}
}

// NewAccount is the input for CreateAccount.
type NewAccount struct {
	Name            string
	Type            string
	Currency        string
	OpeningBalance  money.Amount
	InstitutionName string
	// LinkID attaches the account to a bank connection when set.
	LinkID *uuid.UUID
}

// AccountPatch is a partial account update; nil fields are left unchanged.
// LinkID uses pointer-to-pointer: nil leaves the connection unchanged, a
// pointer to nil detaches the account, a pointer to a UUID attaches it.
type AccountPatch struct {
	Name            *string
	Type            *string
	Subtype         *string
	Currency        *string
	OpeningBalance  *money.Amount
	InstitutionName *string
	IsActive        *bool
	LinkID          **uuid.UUID
}

// NewTransaction is the input for CreateTransaction. Amount is the positive
// magnitude; Direction decides the sign.
type NewTransaction struct {
	AccountID   uuid.UUID
	CategoryID  *uuid.UUID
	Description string
	Merchant    *string
	Amount      money.Amount
	Direction   domain.Direction
	Currency    string
	Status      domain.TransactionStatus
	OccurredAt  *time.Time
	Notes       *string
}

// TransactionPatch is a partial transaction update. CategoryID, Merchant and
// Notes use pointer-to-pointer: nil means "leave unchanged", a pointer to nil
// clears the value.
type TransactionPatch struct {
	AccountID   *uuid.UUID
	CategoryID  **uuid.UUID
	Description *string
	Merchant    **string
	Amount      *money.Amount
	Direction   *domain.Direction
	OccurredAt  *time.Time
	Status      *domain.TransactionStatus
	Notes       **string
}

func (s *Service) ListAccounts(ctx context.Context, userID uuid.UUID) ([]AccountView, error) {
	return s.store.ListAccounts(ctx, userID)
}

func (s *Service) GetAccount(ctx context.Context, userID, accountID uuid.UUID) (AccountView, error) {
	return s.store.GetAccount(ctx, userID, accountID)
}

func (s *Service) CreateAccount(ctx context.Context, userID uuid.UUID, in NewAccount) (AccountView, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return AccountView{}, Invalid("name is required")
	}
	if utf8.RuneCountInString(name) > 80 {
		return AccountView{}, Invalid("name must be at most 80 characters")
	}

	accType := domain.AccountType(strings.TrimSpace(in.Type))
	if accType == "" {
		accType = domain.AccountChecking
	}
	if !validAccountType(accType) {
		return AccountView{}, Invalid("invalid account type %q", in.Type)
	}

	currency, err := normalizeCurrency(in.Currency)
	if err != nil {
		return AccountView{}, err
	}
	if currency == "" {
		currency = "COP"
	}

	params := CreateAccountParams{
		UserID:          userID,
		Name:            name,
		Type:            accType,
		Currency:        currency,
		OpeningBalance:  in.OpeningBalance,
		InstitutionName: strings.TrimSpace(in.InstitutionName),
	}
	if in.LinkID != nil {
		link, err := s.store.GetLink(ctx, userID, *in.LinkID)
		if err != nil {
			return AccountView{}, err
		}
		params.LinkID = in.LinkID
		if params.InstitutionName == "" {
			params.InstitutionName = link.InstitutionName
		}
	}
	return s.store.CreateAccount(ctx, params)
}

func (s *Service) UpdateAccount(ctx context.Context, userID, accountID uuid.UUID, in AccountPatch) (AccountView, error) {
	current, err := s.store.GetAccount(ctx, userID, accountID)
	if err != nil {
		return AccountView{}, err
	}
	if current.Provider != "manual" && (in.OpeningBalance != nil || in.Currency != nil) {
		return AccountView{}, Invalid("balance and currency of a linked account are managed by the bank connection")
	}

	params := UpdateAccountParams{UserID: userID, AccountID: accountID}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return AccountView{}, Invalid("name cannot be empty")
		}
		if utf8.RuneCountInString(name) > 80 {
			return AccountView{}, Invalid("name must be at most 80 characters")
		}
		params.Name = &name
	}
	if in.Type != nil {
		t := domain.AccountType(strings.TrimSpace(*in.Type))
		if !validAccountType(t) {
			return AccountView{}, Invalid("invalid account type %q", *in.Type)
		}
		params.Type = &t
	}
	if in.Subtype != nil {
		subtype := strings.TrimSpace(*in.Subtype)
		params.Subtype = &subtype
	}
	if in.Currency != nil {
		currency, err := normalizeCurrency(*in.Currency)
		if err != nil {
			return AccountView{}, err
		}
		if currency == "" {
			return AccountView{}, Invalid("currency cannot be empty")
		}
		params.Currency = &currency
	}
	if in.OpeningBalance != nil {
		if err := validateAmount(*in.OpeningBalance, false); err != nil {
			return AccountView{}, err
		}
		params.OpeningBalance = in.OpeningBalance
	}
	if in.InstitutionName != nil {
		institution := strings.TrimSpace(*in.InstitutionName)
		params.InstitutionName = &institution
	}
	params.IsActive = in.IsActive

	if in.LinkID != nil {
		if *in.LinkID == nil {
			// Detach: keep the account and its transactions, drop the connection.
			params.SetLink = true
			params.LinkID = nil
			institution := ""
			params.InstitutionName = &institution
		} else {
			link, err := s.store.GetLink(ctx, userID, **in.LinkID)
			if err != nil {
				return AccountView{}, err
			}
			params.SetLink = true
			params.LinkID = *in.LinkID
			institution := link.InstitutionName
			params.InstitutionName = &institution
		}
	}

	return s.store.UpdateAccount(ctx, params)
}

func (s *Service) DeleteAccount(ctx context.Context, userID, accountID uuid.UUID) error {
	return s.store.DeleteAccount(ctx, userID, accountID)
}

func (s *Service) ListCategories(ctx context.Context, userID uuid.UUID) ([]domain.Category, error) {
	return s.store.ListCategories(ctx, userID)
}

func (s *Service) ListTransactions(ctx context.Context, f TransactionFilter) ([]TransactionView, int64, error) {
	return s.store.ListTransactions(ctx, f)
}

func (s *Service) GetTransaction(ctx context.Context, userID, transactionID uuid.UUID) (TransactionView, error) {
	return s.store.GetTransaction(ctx, userID, transactionID)
}

func (s *Service) CreateTransaction(ctx context.Context, userID uuid.UUID, in NewTransaction) (TransactionView, error) {
	account, err := s.store.GetAccount(ctx, userID, in.AccountID)
	if err != nil {
		return TransactionView{}, err
	}

	description := strings.TrimSpace(in.Description)
	if description == "" {
		return TransactionView{}, Invalid("description is required")
	}
	if utf8.RuneCountInString(description) > 200 {
		return TransactionView{}, Invalid("description must be at most 200 characters")
	}
	if err := validateAmount(in.Amount, true); err != nil {
		return TransactionView{}, err
	}
	if !validDirection(in.Direction) {
		return TransactionView{}, Invalid(`direction must be "in" or "out"`)
	}
	if in.CategoryID != nil {
		if err := s.requireCategory(ctx, userID, *in.CategoryID); err != nil {
			return TransactionView{}, err
		}
	}
	status := in.Status
	if status == "" {
		status = domain.StatusPosted
	}
	if !validStatus(status) {
		return TransactionView{}, Invalid("invalid status %q", in.Status)
	}
	currency, err := normalizeCurrency(in.Currency)
	if err != nil {
		return TransactionView{}, err
	}
	if currency == "" {
		currency = account.Currency
	}
	occurredAt := time.Now()
	if in.OccurredAt != nil {
		occurredAt = *in.OccurredAt
	}

	amount := in.Amount
	if in.Direction == domain.DirectionOut {
		amount = amount.Neg()
	}
	merchant := cleanOptional(in.Merchant)

	categoryID := in.CategoryID
	source := domain.CategorySourceNone
	if categoryID != nil {
		source = domain.CategorySourceManual
	} else if cat, err := s.ruleCategory(ctx, userID, RuleRow{
		Description: description,
		Merchant:    merchant,
		Amount:      amount,
	}); err != nil {
		return TransactionView{}, err
	} else if cat != nil {
		categoryID = cat
		source = domain.CategorySourceRule
	}

	return s.store.CreateTransaction(ctx, CreateTransactionParams{
		UserID:         userID,
		AccountID:      in.AccountID,
		CategoryID:     categoryID,
		Description:    description,
		Merchant:       merchant,
		Amount:         amount,
		Direction:      in.Direction,
		Currency:       currency,
		Status:         status,
		CategorySource: source,
		OccurredAt:     occurredAt,
		Notes:          cleanOptional(in.Notes),
	})
}

func (s *Service) UpdateTransaction(ctx context.Context, userID, transactionID uuid.UUID, in TransactionPatch) (TransactionView, error) {
	existing, err := s.store.GetTransaction(ctx, userID, transactionID)
	if err != nil {
		return TransactionView{}, err
	}

	params := UpdateTransactionParams{UserID: userID, TransactionID: transactionID}
	descriptionChanged := false
	merchantSet := false

	if in.AccountID != nil {
		if _, err := s.store.GetAccount(ctx, userID, *in.AccountID); err != nil {
			return TransactionView{}, err
		}
		params.AccountID = in.AccountID
	}

	// Keep amount and direction consistent: recompute the signed amount when
	// either side changes.
	magnitude := existing.Amount.Abs()
	direction := existing.Direction
	signedChanged := false
	if in.Amount != nil {
		if err := validateAmount(*in.Amount, true); err != nil {
			return TransactionView{}, err
		}
		magnitude = *in.Amount
		signedChanged = true
	}
	if in.Direction != nil {
		if !validDirection(*in.Direction) {
			return TransactionView{}, Invalid(`direction must be "in" or "out"`)
		}
		direction = *in.Direction
		signedChanged = true
	}
	if signedChanged {
		signed := magnitude
		if direction == domain.DirectionOut {
			signed = signed.Neg()
		}
		params.Amount = &signed
		params.Direction = &direction
	}

	if in.CategoryID != nil {
		categoryID := *in.CategoryID
		if categoryID != nil {
			if err := s.requireCategory(ctx, userID, *categoryID); err != nil {
				return TransactionView{}, err
			}
		}
		source := domain.CategorySourceManual
		if categoryID == nil {
			source = domain.CategorySourceNone
		}
		params.SetCategory = true
		params.CategoryID = categoryID
		params.CategorySource = &source
	}

	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		if description == "" {
			return TransactionView{}, Invalid("description cannot be empty")
		}
		if utf8.RuneCountInString(description) > 200 {
			return TransactionView{}, Invalid("description must be at most 200 characters")
		}
		params.Description = &description
		descriptionChanged = true
	}

	if merchant, set := cleanOptionalPresent(in.Merchant); set {
		params.SetMerchant = true
		params.Merchant = merchant
		merchantSet = true
	}
	if notes, set := cleanOptionalPresent(in.Notes); set {
		params.SetNotes = true
		params.Notes = notes
	}

	if in.OccurredAt != nil {
		params.OccurredAt = in.OccurredAt
	}
	if in.Status != nil {
		if !validStatus(*in.Status) {
			return TransactionView{}, Invalid("invalid status %q", *in.Status)
		}
		params.Status = in.Status
	}

	// Re-apply the rules when the identifying fields change and the category
	// was not explicitly picked in this request. Rules only re-assign; they
	// never clear a category (users clear categories on purpose).
	if in.CategoryID == nil && existing.CategorySource != domain.CategorySourceManual &&
		(descriptionChanged || merchantSet || signedChanged) {
		row := RuleRow{Description: existing.Description, Merchant: existing.Merchant, Amount: existing.Amount}
		if params.Description != nil {
			row.Description = *params.Description
		}
		if params.SetMerchant {
			row.Merchant = params.Merchant
		}
		if params.Amount != nil {
			row.Amount = *params.Amount
		}
		cat, err := s.ruleCategory(ctx, userID, row)
		if err != nil {
			return TransactionView{}, err
		}
		if cat != nil {
			source := domain.CategorySourceRule
			params.SetCategory = true
			params.CategoryID = cat
			params.CategorySource = &source
		}
	}

	return s.store.UpdateTransaction(ctx, params)
}

func (s *Service) DeleteTransaction(ctx context.Context, userID, transactionID uuid.UUID) error {
	return s.store.DeleteTransaction(ctx, userID, transactionID)
}

// Summary returns period totals, category breakdown and net worth. from/to
// default to the current month (in the configured timezone) up to now.
func (s *Service) Summary(ctx context.Context, userID uuid.UUID, from, to *time.Time) (Summary, error) {
	now := time.Now().In(s.loc)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.loc)
	end := start.AddDate(0, 1, 0)
	f, t := start, end
	if from != nil {
		f = from.In(s.loc)
	}
	if to != nil {
		t = to.In(s.loc)
	}
	if !f.Before(t) {
		return Summary{}, Invalid("from must be before to")
	}

	income, expenses, netWorth, err := s.store.SummaryTotals(ctx, userID, f, t)
	if err != nil {
		return Summary{}, err
	}
	byCategory, err := s.store.CategoryTotals(ctx, userID, f, t)
	if err != nil {
		return Summary{}, err
	}

	return Summary{
		From:       f,
		To:         t,
		Income:     income,
		Expenses:   expenses,
		Net:        income.Sub(expenses),
		NetWorth:   netWorth,
		ByCategory: byCategory,
	}, nil
}

// MonthlyTotals returns the last n months of income/expenses, oldest first,
// including months without transactions.
func (s *Service) MonthlyTotals(ctx context.Context, userID uuid.UUID, months int) ([]MonthTotal, error) {
	if months <= 0 {
		months = 6
	}
	if months > 24 {
		months = 24
	}

	now := time.Now().In(s.loc)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.loc).AddDate(0, -(months - 1), 0)

	rows, err := s.store.MonthlyTotals(ctx, userID, start, s.loc.String())
	if err != nil {
		return nil, err
	}
	byMonth := make(map[string]MonthTotal, len(rows))
	for _, r := range rows {
		byMonth[r.Month] = r
	}

	out := make([]MonthTotal, 0, months)
	for i := 0; i < months; i++ {
		month := start.AddDate(0, i, 0).Format("2006-01")
		if r, ok := byMonth[month]; ok {
			out = append(out, r)
			continue
		}
		out = append(out, MonthTotal{
			Month:    month,
			Income:   money.Zero(),
			Expenses: money.Zero(),
			Net:      money.Zero(),
		})
	}
	return out, nil
}

func (s *Service) requireCategory(ctx context.Context, userID, categoryID uuid.UUID) error {
	ok, err := s.store.CategoryValid(ctx, userID, categoryID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func validAccountType(t domain.AccountType) bool {
	switch t {
	case domain.AccountChecking, domain.AccountSavings, domain.AccountCreditCard,
		domain.AccountLoan, domain.AccountInvestment, domain.AccountOther:
		return true
	}
	return false
}

func validDirection(d domain.Direction) bool {
	return d == domain.DirectionIn || d == domain.DirectionOut
}

func validStatus(s domain.TransactionStatus) bool {
	return s == domain.StatusPosted || s == domain.StatusPending
}

// validateAmount requires a positive magnitude and at most 4 decimal places
// (the storage precision). allowNegative is used for opening balances.
func validateAmount(a money.Amount, requirePositive bool) error {
	if requirePositive && !a.Decimal.IsPositive() {
		return Invalid("amount must be greater than zero")
	}
	if a.Decimal.Exponent() < -4 {
		return Invalid("amount supports at most 4 decimal places")
	}
	return nil
}

func normalizeCurrency(c string) (string, error) {
	c = strings.ToUpper(strings.TrimSpace(c))
	if c == "" {
		return "", nil
	}
	if len(c) != 3 {
		return "", Invalid("currency must be a 3-letter code")
	}
	for _, r := range c {
		if !unicode.IsLetter(r) || r > unicode.MaxASCII {
			return "", Invalid("currency must be a 3-letter code")
		}
	}
	return c, nil
}

// cleanOptional trims a string pointer, turning empty strings into nil.
func cleanOptional(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// cleanOptionalPresent handles pointer-to-pointer patch fields. set reports
// whether the field was present in the request; value is nil to clear.
func cleanOptionalPresent(s **string) (value *string, set bool) {
	if s == nil {
		return nil, false
	}
	if *s == nil {
		return nil, true
	}
	return cleanOptional(*s), true
}

// --- Institutions ---

// NewInstitution is the input for adding a custom bank or wallet.
type NewInstitution struct {
	DisplayName  string
	Type         string
	Website      string
	PrimaryColor string
	Icon         string
}

// ListInstitutions returns the system catalog plus the user's custom rows.
func (s *Service) ListInstitutions(ctx context.Context, userID uuid.UUID, f InstitutionFilter) ([]InstitutionView, error) {
	f.UserID = userID
	if f.Type != nil && !validInstitutionType(*f.Type) {
		return nil, Invalid("invalid institution type %q", *f.Type)
	}
	return s.store.ListInstitutions(ctx, f)
}

// CreateInstitution adds a custom bank or wallet for the user.
func (s *Service) CreateInstitution(ctx context.Context, userID uuid.UUID, in NewInstitution) (InstitutionView, error) {
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		return InstitutionView{}, Invalid("institution name is required")
	}
	if utf8.RuneCountInString(displayName) > 60 {
		return InstitutionView{}, Invalid("institution name must be 60 characters or fewer")
	}
	instType := domain.InstitutionType(strings.TrimSpace(in.Type))
	if instType == "" {
		instType = domain.InstitutionBank
	}
	if !validInstitutionType(instType) {
		return InstitutionView{}, Invalid("invalid institution type %q", in.Type)
	}
	website := strings.TrimSpace(in.Website)
	if website != "" && !strings.HasPrefix(website, "http://") && !strings.HasPrefix(website, "https://") {
		return InstitutionView{}, Invalid("website must start with http:// or https://")
	}
	color := strings.TrimSpace(in.PrimaryColor)
	if color != "" && !validHexColor(color) {
		return InstitutionView{}, Invalid("primary color must be a hex value like #1A2B3C")
	}
	icon := strings.TrimSpace(in.Icon)
	if utf8.RuneCountInString(icon) > 8 {
		return InstitutionView{}, Invalid("icon must be a short emoji")
	}
	return s.store.CreateInstitution(ctx, CreateInstitutionParams{
		UserID:       userID,
		Name:         strings.ToLower(displayName),
		DisplayName:  displayName,
		Type:         instType,
		Website:      website,
		PrimaryColor: color,
		Icon:         icon,
	})
}

// DeleteInstitution removes a custom institution and detaches every account
// connected through it. System institutions cannot be deleted.
func (s *Service) DeleteInstitution(ctx context.Context, userID, institutionID uuid.UUID) error {
	inst, err := s.store.GetInstitution(ctx, userID, institutionID)
	if err != nil {
		return err
	}
	if inst.IsSystem {
		return Invalid("built-in institutions cannot be deleted")
	}
	return s.store.DeleteInstitution(ctx, userID, institutionID)
}

// GetInstitution returns a single institution visible to the user.
func (s *Service) GetInstitution(ctx context.Context, userID, institutionID uuid.UUID) (InstitutionView, error) {
	return s.store.GetInstitution(ctx, userID, institutionID)
}

// --- Connections ---

// ListLinks returns the user's bank connections.
func (s *Service) ListLinks(ctx context.Context, userID uuid.UUID) ([]LinkView, error) {
	return s.store.ListLinks(ctx, userID)
}

// GetLink returns a single connection owned by the user.
func (s *Service) GetLink(ctx context.Context, userID, linkID uuid.UUID) (LinkView, error) {
	return s.store.GetLink(ctx, userID, linkID)
}

// CreateLink connects the user to an institution. Accounts created afterwards
// can be attached to the connection.
func (s *Service) CreateLink(ctx context.Context, userID, institutionID uuid.UUID) (LinkView, error) {
	if _, err := s.store.GetInstitution(ctx, userID, institutionID); err != nil {
		return LinkView{}, err
	}
	exists, err := s.store.LinkExists(ctx, userID, institutionID)
	if err != nil {
		return LinkView{}, err
	}
	if exists {
		return LinkView{}, Invalid("this institution is already connected")
	}
	return s.store.CreateLink(ctx, CreateLinkParams{UserID: userID, InstitutionID: institutionID})
}

// DeleteLink detaches the connection's accounts and removes the connection.
// Accounts and their transactions are kept.
func (s *Service) DeleteLink(ctx context.Context, userID, linkID uuid.UUID) error {
	return s.store.DeleteLink(ctx, userID, linkID)
}

func validInstitutionType(t domain.InstitutionType) bool {
	switch t {
	case domain.InstitutionBank, domain.InstitutionWallet, domain.InstitutionOther:
		return true
	default:
		return false
	}
}

func validHexColor(c string) bool {
	if len(c) != 4 && len(c) != 7 {
		return false
	}
	if c[0] != '#' {
		return false
	}
	for _, r := range c[1:] {
		if !unicode.IsDigit(r) && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// PreviewImport parses a statement file and reports what would be imported.
func (s *Service) PreviewImport(ctx context.Context, userID, accountID uuid.UUID, fileName string, data []byte) (ImportPreview, error) {
	if _, err := s.store.GetAccount(ctx, userID, accountID); err != nil {
		return ImportPreview{}, err
	}
	loc := s.loc
	if loc == nil {
		loc = time.UTC
	}
	format, rows, skipped, err := parseStatement(fileName, data, loc)
	if err != nil {
		return ImportPreview{}, err
	}
	if len(rows) == 0 {
		return ImportPreview{}, Invalid("no transactions found in the file")
	}

	hashes := hashRows(accountID, rows)
	existing, err := s.store.CountDedupeHashes(ctx, accountID, hashes)
	if err != nil {
		return ImportPreview{}, err
	}
	repeats := make(map[string]int, len(hashes))
	for _, hash := range hashes {
		repeats[hash]++
	}

	suggestions, err := s.ruleSuggestionsFor(ctx, userID)
	if err != nil {
		return ImportPreview{}, err
	}

	preview := ImportPreview{Format: format, FileName: fileName, Total: len(rows), Skipped: skipped}
	for i, row := range rows {
		hash := hashes[i]
		view := ImportRowView{
			Date:        row.OccurredAt.Format("2006-01-02"),
			Description: row.Description,
			Merchant:    importText(row.Merchant),
			Reference:   importText(row.Reference),
			ExternalID:  importText(row.ExternalID),
			Amount:      row.Amount.String(),
			Direction:   string(row.Direction),
			Duplicate:   existing[hash],
			Repeated:    repeats[hash] > 1,
		}
		if category := suggestions.suggest(importRuleRow(row)); category != nil {
			id := category.ID.String()
			name := category.Name
			view.CategoryID = &id
			view.CategoryName = &name
			view.CategorySource = string(domain.CategorySourceRule)
		}
		if view.Duplicate {
			preview.Existing++
		}
		if view.Repeated {
			preview.Repeated++
		}
		if preview.DateFrom == "" || view.Date < preview.DateFrom {
			preview.DateFrom = view.Date
		}
		if view.Date > preview.DateTo {
			preview.DateTo = view.Date
		}
		if len(preview.Rows) < previewRowLimit {
			preview.Rows = append(preview.Rows, view)
		} else {
			preview.Truncated = true
		}
	}
	return preview, nil
}

// CommitImport parses a statement and stores the new rows. Rows already present
// (same account and dedupe hash) are skipped, so importing the same file twice
// is safe.
func (s *Service) CommitImport(ctx context.Context, userID, accountID uuid.UUID, fileName string, data []byte) (ImportResult, error) {
	account, err := s.store.GetAccount(ctx, userID, accountID)
	if err != nil {
		return ImportResult{}, err
	}
	loc := s.loc
	if loc == nil {
		loc = time.UTC
	}
	_, rows, skipped, err := parseStatement(fileName, data, loc)
	if err != nil {
		return ImportResult{}, err
	}
	if len(rows) == 0 {
		return ImportResult{}, Invalid("no transactions found in the file")
	}

	hashes := hashRows(accountID, rows)
	suggestions, err := s.ruleSuggestionsFor(ctx, userID)
	if err != nil {
		return ImportResult{}, err
	}
	transactions := make([]ImportedTransaction, 0, len(rows))
	for i, row := range rows {
		transaction := ImportedTransaction{
			Description: row.Description,
			Merchant:    row.Merchant,
			Reference:   row.Reference,
			Amount:      row.Amount,
			Direction:   row.Direction,
			OccurredAt:  row.OccurredAt,
			DedupeHash:  hashes[i],
		}
		if category := suggestions.suggest(importRuleRow(row)); category != nil {
			id := category.ID
			transaction.CategoryID = &id
			transaction.CategorySource = domain.CategorySourceRule
		}
		transactions = append(transactions, transaction)
	}
	imported, err := s.store.InsertImportedTransactions(ctx, ImportInsertParams{
		UserID:       userID,
		AccountID:    accountID,
		Currency:     account.Currency,
		Transactions: transactions,
	})
	if err != nil {
		return ImportResult{}, err
	}
	s.maybeEnqueueReclassification(ctx, userID, accountID)
	return ImportResult{
		Total:     len(rows),
		Imported:  int(imported),
		Duplicate: len(rows) - int(imported),
		Skipped:   skipped,
	}, nil
}

func importText(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
