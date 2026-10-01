package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/money"
)

// Store provides database access for accounts, categories, transactions and
// reports. Every query is scoped to a user.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const accountSelect = `
	SELECT
		a.id, a.user_id, a.link_id, a.provider, a.provider_account_id, a.name, a.type, a.subtype,
		a.currency,
		CASE WHEN a.provider = 'manual'
		     THEN a.balance_current + COALESCE((SELECT sum(t.amount) FROM transactions t WHERE t.account_id = a.id), 0)
		     ELSE a.balance_current
		END AS current_balance,
		a.balance_current, a.balance_available, a.credit_limit, a.institution_name, a.is_active,
		a.last_synced_at, a.created_at, a.updated_at,
		(SELECT count(*) FROM transactions t WHERE t.account_id = a.id) AS transaction_count,
		COALESCE(i.logo_url, ''), COALESCE(i.primary_color, ''), COALESCE(i.icon, ''), COALESCE(i.type, '')
	FROM accounts a
	LEFT JOIN links l ON l.id = a.link_id
	LEFT JOIN institutions i ON i.id = l.institution_id`

func scanAccountView(row pgx.Row) (AccountView, error) {
	var (
		v                      AccountView
		current                money.Amount
		available, creditLimit *money.Amount
	)
	err := row.Scan(
		&v.ID, &v.UserID, &v.LinkID, &v.Provider, &v.ProviderAccount, &v.Name, &v.Type, &v.Subtype,
		&v.Currency, &current, &v.OpeningBalance, &available, &creditLimit, &v.InstitutionName,
		&v.IsActive, &v.LastSyncedAt, &v.CreatedAt, &v.UpdatedAt, &v.TransactionCount,
		&v.InstitutionLogoURL, &v.InstitutionColor, &v.InstitutionIcon, &v.InstitutionType,
	)
	if err != nil {
		return AccountView{}, err
	}
	v.BalanceCurrent = current
	v.BalanceAvailable = available
	v.CreditLimit = creditLimit
	return v, nil
}

func (s *Store) ListAccounts(ctx context.Context, userID uuid.UUID) ([]AccountView, error) {
	rows, err := s.pool.Query(ctx, accountSelect+`
		WHERE a.user_id = $1
		ORDER BY a.is_active DESC, a.created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	views := make([]AccountView, 0)
	for rows.Next() {
		v, err := scanAccountView(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

func (s *Store) GetAccount(ctx context.Context, userID, accountID uuid.UUID) (AccountView, error) {
	v, err := scanAccountView(s.pool.QueryRow(ctx, accountSelect+`
		WHERE a.user_id = $1 AND a.id = $2`, userID, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountView{}, ErrNotFound
	}
	if err != nil {
		return AccountView{}, fmt.Errorf("get account: %w", err)
	}
	return v, nil
}

func (s *Store) CreateAccount(ctx context.Context, p CreateAccountParams) (AccountView, error) {
	const q = `
		INSERT INTO accounts (user_id, link_id, provider, name, type, subtype, currency, balance_current, institution_name)
		VALUES ($1, $2, 'manual', $3, $4, $5, $6, $7, $8)
		RETURNING id`

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, q,
		p.UserID, p.LinkID, p.Name, p.Type, p.Subtype, p.Currency, p.OpeningBalance, p.InstitutionName,
	).Scan(&id)
	if err != nil {
		return AccountView{}, fmt.Errorf("insert account: %w", err)
	}
	return s.GetAccount(ctx, p.UserID, id)
}

func (s *Store) UpdateAccount(ctx context.Context, p UpdateAccountParams) (AccountView, error) {
	const q = `
		UPDATE accounts SET
			name = COALESCE($3, name),
			type = COALESCE($4, type),
			subtype = COALESCE($5, subtype),
			currency = COALESCE($6, currency),
			balance_current = COALESCE($7, balance_current),
			institution_name = COALESCE($8, institution_name),
			is_active = COALESCE($9, is_active),
			link_id = CASE WHEN $10 THEN $11::uuid ELSE link_id END
		WHERE user_id = $1 AND id = $2`

	tag, err := s.pool.Exec(ctx, q,
		p.UserID, p.AccountID, p.Name, p.Type, p.Subtype, p.Currency,
		p.OpeningBalance, p.InstitutionName, p.IsActive, p.SetLink, p.LinkID,
	)
	if err != nil {
		return AccountView{}, fmt.Errorf("update account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return AccountView{}, ErrNotFound
	}
	return s.GetAccount(ctx, p.UserID, p.AccountID)
}

func (s *Store) DeleteAccount(ctx context.Context, userID, accountID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM accounts WHERE user_id = $1 AND id = $2`, userID, accountID)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListCategories(ctx context.Context, userID uuid.UUID) ([]domain.Category, error) {
	const q = `
		SELECT id, user_id, parent_id, slug, name, kind, icon, color, sort_order, is_system, created_at, updated_at
		FROM categories
		WHERE user_id IS NULL OR user_id = $1
		ORDER BY sort_order, name`

	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	categories := make([]domain.Category, 0)
	for rows.Next() {
		var c domain.Category
		err := rows.Scan(
			&c.ID, &c.UserID, &c.ParentID, &c.Slug, &c.Name, &c.Kind, &c.Icon, &c.Color,
			&c.SortOrder, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// CategoryValid reports whether the category exists and is visible to the user
// (a system category or one they own).
func (s *Store) CategoryValid(ctx context.Context, userID, categoryID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM categories WHERE id = $1 AND (user_id IS NULL OR user_id = $2))`

	var ok bool
	if err := s.pool.QueryRow(ctx, q, categoryID, userID).Scan(&ok); err != nil {
		return false, fmt.Errorf("check category: %w", err)
	}
	return ok, nil
}

const transactionSelect = `
	SELECT
		t.id, t.user_id, t.account_id, t.category_id, t.provider, t.provider_transaction_id,
		t.description, t.merchant, t.amount, t.direction, t.currency, t.status, t.category_source,
		t.occurred_at, t.value_date, t.reference, t.notes, t.tags, t.source, t.dedupe_hash, t.raw,
		t.created_at, t.updated_at,
		a.name,
		c.name, c.icon, c.color, c.slug
	FROM transactions t
	JOIN accounts a ON a.id = t.account_id
	LEFT JOIN categories c ON c.id = t.category_id`

func scanTransactionView(row pgx.Row) (TransactionView, error) {
	var v TransactionView
	err := row.Scan(
		&v.ID, &v.UserID, &v.AccountID, &v.CategoryID, &v.Provider, &v.ProviderTransactionID,
		&v.Description, &v.Merchant, &v.Amount, &v.Direction, &v.Currency, &v.Status, &v.CategorySource,
		&v.OccurredAt, &v.ValueDate, &v.Reference, &v.Notes, &v.Tags, &v.Source, &v.DedupeHash, &v.Raw,
		&v.CreatedAt, &v.UpdatedAt,
		&v.AccountName,
		&v.CategoryName, &v.CategoryIcon, &v.CategoryColor, &v.CategorySlug,
	)
	if err != nil {
		return TransactionView{}, err
	}
	return v, nil
}

func (s *Store) GetTransaction(ctx context.Context, userID, transactionID uuid.UUID) (TransactionView, error) {
	v, err := scanTransactionView(s.pool.QueryRow(ctx, transactionSelect+`
		WHERE t.user_id = $1 AND t.id = $2`, userID, transactionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return TransactionView{}, ErrNotFound
	}
	if err != nil {
		return TransactionView{}, fmt.Errorf("get transaction: %w", err)
	}
	return v, nil
}

func (s *Store) CreateTransaction(ctx context.Context, p CreateTransactionParams) (TransactionView, error) {
	const q = `
		INSERT INTO transactions
			(user_id, account_id, category_id, provider, description, merchant, amount, direction,
			 currency, status, category_source, occurred_at, notes)
		VALUES ($1, $2, $3, 'manual', $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, q,
		p.UserID, p.AccountID, p.CategoryID, p.Description, p.Merchant, p.Amount, p.Direction,
		p.Currency, p.Status, p.CategorySource, p.OccurredAt, p.Notes,
	).Scan(&id)
	if err != nil {
		return TransactionView{}, fmt.Errorf("insert transaction: %w", err)
	}
	return s.GetTransaction(ctx, p.UserID, id)
}

func (s *Store) UpdateTransaction(ctx context.Context, p UpdateTransactionParams) (TransactionView, error) {
	sets := make([]string, 0, 10)
	args := []any{p.UserID, p.TransactionID}
	add := func(clause string, val any) {
		args = append(args, val)
		sets = append(sets, fmt.Sprintf(clause, len(args)))
	}

	if p.AccountID != nil {
		add("account_id = $%d", *p.AccountID)
	}
	if p.SetCategory {
		add("category_id = $%d", p.CategoryID)
		add("category_source = $%d", p.CategorySource)
	}
	if p.Description != nil {
		add("description = $%d", *p.Description)
	}
	if p.SetMerchant {
		add("merchant = $%d", p.Merchant)
	}
	if p.Amount != nil {
		add("amount = $%d", *p.Amount)
	}
	if p.Direction != nil {
		add("direction = $%d", *p.Direction)
	}
	if p.OccurredAt != nil {
		add("occurred_at = $%d", *p.OccurredAt)
	}
	if p.Status != nil {
		add("status = $%d", *p.Status)
	}
	if p.SetNotes {
		add("notes = $%d", p.Notes)
	}

	if len(sets) == 0 {
		return s.GetTransaction(ctx, p.UserID, p.TransactionID)
	}

	q := `UPDATE transactions SET ` + strings.Join(sets, ", ") + ` WHERE user_id = $1 AND id = $2`
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return TransactionView{}, fmt.Errorf("update transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return TransactionView{}, ErrNotFound
	}
	return s.GetTransaction(ctx, p.UserID, p.TransactionID)
}

func (s *Store) DeleteTransaction(ctx context.Context, userID, transactionID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM transactions WHERE user_id = $1 AND id = $2`, userID, transactionID)
	if err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListTransactions(ctx context.Context, f TransactionFilter) ([]TransactionView, int64, error) {
	where := []string{"t.user_id = $1"}
	args := []any{f.UserID}
	add := func(cond string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}

	if f.AccountID != nil {
		add("t.account_id = $%d", *f.AccountID)
	}
	if f.CategoryID != nil {
		add("t.category_id = $%d", *f.CategoryID)
	}
	if f.Direction != nil {
		add("t.direction = $%d", *f.Direction)
	}
	if f.From != nil {
		add("t.occurred_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("t.occurred_at < $%d", *f.To)
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		args = append(args, "%"+search+"%")
		n := len(args)
		where = append(where, fmt.Sprintf(
			"(t.description ILIKE $%d OR COALESCE(t.merchant, '') ILIKE $%d)", n, n))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM transactions t WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)
	q := transactionSelect + " WHERE " + whereSQL +
		fmt.Sprintf(" ORDER BY t.occurred_at DESC, t.created_at DESC, t.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	views := make([]TransactionView, 0)
	for rows.Next() {
		v, err := scanTransactionView(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		views = append(views, v)
	}
	return views, total, rows.Err()
}

// SummaryTotals returns period income and expenses (both positive) and the
// current net worth across active accounts.
func (s *Store) SummaryTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) (income, expenses, netWorth money.Amount, err error) {
	const totalsQ = `
		SELECT
			COALESCE(sum(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(sum(CASE WHEN amount < 0 THEN -amount ELSE 0 END), 0)
		FROM transactions
		WHERE user_id = $1 AND occurred_at >= $2 AND occurred_at < $3`

	if err := s.pool.QueryRow(ctx, totalsQ, userID, from, to).Scan(&income, &expenses); err != nil {
		return money.Zero(), money.Zero(), money.Zero(), fmt.Errorf("summary totals: %w", err)
	}

	const worthQ = `
		SELECT COALESCE(sum(
			CASE WHEN a.provider = 'manual'
			     THEN a.balance_current + COALESCE(x.total, 0)
			     ELSE a.balance_current
			END
		), 0)
		FROM accounts a
		LEFT JOIN (
			SELECT account_id, sum(amount) AS total FROM transactions GROUP BY account_id
		) x ON x.account_id = a.id
		WHERE a.user_id = $1 AND a.is_active`

	if err := s.pool.QueryRow(ctx, worthQ, userID).Scan(&netWorth); err != nil {
		return money.Zero(), money.Zero(), money.Zero(), fmt.Errorf("net worth: %w", err)
	}
	return income, expenses, netWorth, nil
}

// CategoryTotals groups transactions by category (or uncategorized) for a
// period. Totals are signed nets.
func (s *Store) CategoryTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]CategoryTotal, error) {
	const q = `
		SELECT
			c.id, COALESCE(c.name, 'Uncategorized'), COALESCE(c.icon, ''), COALESCE(c.color, ''),
			COALESCE(c.kind, ''), t.direction, sum(t.amount), count(*)
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.user_id = $1 AND t.occurred_at >= $2 AND t.occurred_at < $3
		GROUP BY c.id, c.name, c.icon, c.color, c.kind, t.direction
		ORDER BY sum(abs(t.amount)) DESC`

	rows, err := s.pool.Query(ctx, q, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("category totals: %w", err)
	}
	defer rows.Close()

	totals := make([]CategoryTotal, 0)
	for rows.Next() {
		var t CategoryTotal
		err := rows.Scan(
			&t.CategoryID, &t.Name, &t.Icon, &t.Color, &t.Kind, &t.Direction, &t.Total, &t.Count,
		)
		if err != nil {
			return nil, fmt.Errorf("scan category total: %w", err)
		}
		totals = append(totals, t)
	}
	return totals, rows.Err()
}

// MonthlyTotals returns per-month income and expenses from the given start,
// grouped in the user's timezone so month boundaries match local dates.
func (s *Store) MonthlyTotals(ctx context.Context, userID uuid.UUID, from time.Time, tz string) ([]MonthTotal, error) {
	const q = `
		SELECT
			to_char(date_trunc('month', occurred_at AT TIME ZONE $3), 'YYYY-MM'),
			COALESCE(sum(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(sum(CASE WHEN amount < 0 THEN -amount ELSE 0 END), 0)
		FROM transactions
		WHERE user_id = $1 AND occurred_at >= $2
		GROUP BY 1
		ORDER BY 1`

	rows, err := s.pool.Query(ctx, q, userID, from, tz)
	if err != nil {
		return nil, fmt.Errorf("monthly totals: %w", err)
	}
	defer rows.Close()

	totals := make([]MonthTotal, 0)
	for rows.Next() {
		var t MonthTotal
		if err := rows.Scan(&t.Month, &t.Income, &t.Expenses); err != nil {
			return nil, fmt.Errorf("scan monthly total: %w", err)
		}
		t.Net = t.Income.Sub(t.Expenses)
		totals = append(totals, t)
	}
	return totals, rows.Err()
}

// --- Institutions ---

const institutionSelect = `
	SELECT
		i.id, i.user_id, i.provider, i.provider_institution_id, i.slug, i.name, i.display_name,
		i.country_code, i.type, i.website, i.primary_color, i.logo_url, i.icon, i.sort_order,
		i.is_system, i.is_active, i.created_at, i.updated_at,
		(SELECT count(*) FROM links l WHERE l.user_id = $1 AND l.institution_id = i.id) AS link_count
	FROM institutions i`

func scanInstitutionView(row pgx.Row) (InstitutionView, error) {
	var v InstitutionView
	err := row.Scan(
		&v.ID, &v.UserID, &v.Provider, &v.ProviderInstitutionID, &v.Slug, &v.Name, &v.DisplayName,
		&v.CountryCode, &v.Type, &v.Website, &v.PrimaryColor, &v.LogoURL, &v.Icon, &v.SortOrder,
		&v.IsSystem, &v.IsActive, &v.CreatedAt, &v.UpdatedAt, &v.LinkCount,
	)
	if err != nil {
		return InstitutionView{}, err
	}
	return v, nil
}

// ListInstitutions returns system institutions plus the user's own.
func (s *Store) ListInstitutions(ctx context.Context, f InstitutionFilter) ([]InstitutionView, error) {
	where := []string{"i.is_active", "(i.user_id IS NULL OR i.user_id = $1)"}
	args := []any{f.UserID}
	add := func(cond string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Country != "" {
		add("i.country_code = $%d", strings.ToUpper(f.Country))
	}
	if f.Type != nil {
		add("i.type = $%d", *f.Type)
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		add("i.display_name ILIKE $%d", "%"+search+"%")
	}

	q := institutionSelect + " WHERE " + strings.Join(where, " AND ") + " ORDER BY i.sort_order, i.display_name"
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list institutions: %w", err)
	}
	defer rows.Close()

	institutions := make([]InstitutionView, 0)
	for rows.Next() {
		v, err := scanInstitutionView(rows)
		if err != nil {
			return nil, fmt.Errorf("scan institution: %w", err)
		}
		institutions = append(institutions, v)
	}
	return institutions, rows.Err()
}

// GetInstitution returns one institution visible to the user.
func (s *Store) GetInstitution(ctx context.Context, userID, institutionID uuid.UUID) (InstitutionView, error) {
	v, err := scanInstitutionView(s.pool.QueryRow(ctx, institutionSelect+`
		WHERE i.id = $2 AND (i.user_id IS NULL OR i.user_id = $1)`, userID, institutionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return InstitutionView{}, ErrNotFound
	}
	if err != nil {
		return InstitutionView{}, fmt.Errorf("get institution: %w", err)
	}
	return v, nil
}

// CreateInstitution inserts a custom institution owned by the user.
func (s *Store) CreateInstitution(ctx context.Context, p CreateInstitutionParams) (InstitutionView, error) {
	const q = `
		INSERT INTO institutions (user_id, name, display_name, type, website, primary_color, icon)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, q,
		p.UserID, p.Name, p.DisplayName, p.Type, p.Website, p.PrimaryColor, p.Icon,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return InstitutionView{}, ErrAlreadyExists
		}
		return InstitutionView{}, fmt.Errorf("insert institution: %w", err)
	}
	return s.GetInstitution(ctx, p.UserID, id)
}

// DeleteInstitution removes a custom institution, detaching the accounts of
// the user's connections to it first. System institutions cannot be deleted.
func (s *Store) DeleteInstitution(ctx context.Context, userID, institutionID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete institution: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var owned bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM institutions WHERE id = $1 AND user_id = $2)`,
		institutionID, userID).Scan(&owned)
	if err != nil {
		return fmt.Errorf("check institution owner: %w", err)
	}
	if !owned {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE accounts SET link_id = NULL
		WHERE user_id = $1 AND link_id IN (
			SELECT id FROM links WHERE user_id = $1 AND institution_id = $2
		)`, userID, institutionID); err != nil {
		return fmt.Errorf("detach institution accounts: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM links WHERE user_id = $1 AND institution_id = $2`, userID, institutionID); err != nil {
		return fmt.Errorf("delete institution links: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM institutions WHERE id = $1 AND user_id = $2`, institutionID, userID); err != nil {
		return fmt.Errorf("delete institution: %w", err)
	}
	return tx.Commit(ctx)
}

// --- Links (bank connections) ---

const linkSelect = `
	SELECT
		l.id, l.user_id, l.provider, l.provider_link_id, l.institution_id, l.institution, l.institution_name,
		l.access_mode, l.status, l.last_sync_at, l.last_sync_error, l.created_at, l.updated_at,
		COALESCE(i.logo_url, ''), COALESCE(i.primary_color, ''), COALESCE(i.icon, ''), COALESCE(i.type, ''),
		(SELECT count(*) FROM accounts a WHERE a.link_id = l.id) AS account_count
	FROM links l
	LEFT JOIN institutions i ON i.id = l.institution_id`

func scanLinkView(row pgx.Row) (LinkView, error) {
	var v LinkView
	err := row.Scan(
		&v.ID, &v.UserID, &v.Provider, &v.ProviderLinkID, &v.InstitutionID, &v.Institution, &v.InstitutionName,
		&v.AccessMode, &v.Status, &v.LastSyncAt, &v.LastSyncError, &v.CreatedAt, &v.UpdatedAt,
		&v.InstitutionLogoURL, &v.InstitutionColor, &v.InstitutionIcon, &v.InstitutionType, &v.AccountCount,
	)
	if err != nil {
		return LinkView{}, err
	}
	return v, nil
}

// ListLinks returns the user's bank connections.
func (s *Store) ListLinks(ctx context.Context, userID uuid.UUID) ([]LinkView, error) {
	rows, err := s.pool.Query(ctx, linkSelect+` WHERE l.user_id = $1 ORDER BY l.created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()

	links := make([]LinkView, 0)
	for rows.Next() {
		v, err := scanLinkView(rows)
		if err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, v)
	}
	return links, rows.Err()
}

// GetLink returns one bank connection owned by the user.
func (s *Store) GetLink(ctx context.Context, userID, linkID uuid.UUID) (LinkView, error) {
	v, err := scanLinkView(s.pool.QueryRow(ctx, linkSelect+` WHERE l.user_id = $1 AND l.id = $2`, userID, linkID))
	if errors.Is(err, pgx.ErrNoRows) {
		return LinkView{}, ErrNotFound
	}
	if err != nil {
		return LinkView{}, fmt.Errorf("get link: %w", err)
	}
	return v, nil
}

// LinkExists reports whether the user already has a connection to the institution.
func (s *Store) LinkExists(ctx context.Context, userID, institutionID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM links WHERE user_id = $1 AND institution_id = $2)`,
		userID, institutionID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check link: %w", err)
	}
	return exists, nil
}

// CreateLink connects the user to a visible institution.
func (s *Store) CreateLink(ctx context.Context, p CreateLinkParams) (LinkView, error) {
	const q = `
		INSERT INTO links (user_id, provider, institution_id, institution, institution_name, access_mode, status)
		SELECT $1, 'manual', i.id, i.slug, i.display_name, 'manual', 'healthy'
		FROM institutions i
		WHERE i.id = $2 AND i.is_active AND (i.user_id IS NULL OR i.user_id = $1)
		RETURNING id`

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, q, p.UserID, p.InstitutionID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return LinkView{}, ErrNotFound
	}
	if err != nil {
		return LinkView{}, fmt.Errorf("insert link: %w", err)
	}
	return s.GetLink(ctx, p.UserID, id)
}

// DeleteLink removes a bank connection, detaching its accounts but keeping
// their transactions.
func (s *Store) DeleteLink(ctx context.Context, userID, linkID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete link: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET link_id = NULL WHERE user_id = $1 AND link_id = $2`, userID, linkID); err != nil {
		return fmt.Errorf("detach link accounts: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM links WHERE user_id = $1 AND id = $2`, userID, linkID)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
