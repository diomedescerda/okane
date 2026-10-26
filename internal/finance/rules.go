// Package finance: the rule matching engine and rule management.
//
// Rules let users describe transactions (description, merchant, reference or
// amount) and automatically assign a category. They are applied when a
// transaction is created, when its identifying fields change, and during
// statement imports — always filling only uncategorized transactions or
// ones already assigned by automation, never a user's explicit pick.
package finance

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/money"
)

// RuleRow is the matcher's view of a transaction: signed amount, with the
// optional text fields raw (no trimming or case folding).
type RuleRow struct {
	Description string
	Merchant    *string
	Reference   *string
	Amount      money.Amount
}

// RuleEngine holds enabled rules ordered for matching plus their precompiled
// regular expressions, so a batch of transactions matches without recompiling.
type RuleEngine struct {
	rules []domain.Rule
	res   map[uuid.UUID]*regexp.Regexp
}

// NewRuleEngine prepares enabled rules (ordered by priority) for matching.
// Rules with an invalid regular expression cannot be stored by the API, but if
// they somehow exist they are skipped rather than breaking matching.
func NewRuleEngine(rules []domain.Rule) *RuleEngine {
	res := make(map[uuid.UUID]*regexp.Regexp)
	for _, rule := range rules {
		if rule.MatchType != domain.MatchRegex {
			continue
		}
		re, err := regexp.Compile(ruleCasePrefix + rule.Pattern)
		if err != nil {
			continue
		}
		res[rule.ID] = re
	}
	return &RuleEngine{rules: rules, res: res}
}

// Match returns the first rule matching the row, or nil. Priority ties are
// broken by the store's ordering (creation time).
func (e *RuleEngine) Match(row RuleRow) *domain.Rule {
	for i := range e.rules {
		if ruleMatches(&e.rules[i], row, e.res[e.rules[i].ID]) {
			return &e.rules[i]
		}
	}
	return nil
}

// MatchRule is a convenience one-shot match for a single row.
func MatchRule(rules []domain.Rule, row RuleRow) *domain.Rule {
	return NewRuleEngine(rules).Match(row)
}

const ruleCasePrefix = "(?i)"

func ruleMatches(rule *domain.Rule, row RuleRow, re *regexp.Regexp) bool {
	switch rule.Field {
	case domain.MatchFieldDescription, domain.MatchFieldMerchant, domain.MatchFieldReference:
		var value string
		switch rule.Field {
		case domain.MatchFieldDescription:
			value = row.Description
		case domain.MatchFieldMerchant:
			if row.Merchant == nil {
				return false
			}
			value = *row.Merchant
		case domain.MatchFieldReference:
			if row.Reference == nil {
				return false
			}
			value = *row.Reference
		}
		return ruleMatchesText(rule.MatchType, rule.Pattern, value, re)
	case domain.MatchFieldAmount:
		// Compare magnitudes so a rule describes "more than X" regardless of
		// the transaction direction.
		magnitude := row.Amount.Abs().Decimal
		threshold := money.New(rule.Pattern).Decimal
		switch rule.MatchType {
		case domain.MatchEquals:
			return magnitude.Equal(threshold)
		case domain.MatchGT:
			return magnitude.GreaterThan(threshold)
		case domain.MatchLT:
			return magnitude.LessThan(threshold)
		}
	}
	return false
}

func ruleMatchesText(match domain.MatchType, pattern, value string, re *regexp.Regexp) bool {
	switch match {
	case domain.MatchContains:
		return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
	case domain.MatchEquals:
		return strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(pattern))
	case domain.MatchRegex:
		if re == nil {
			return false
		}
		return re.MatchString(value)
	}
	return false
}

// validateRule checks that field, match type and pattern are coherent.
func validateRule(field domain.MatchField, match domain.MatchType, pattern string) error {
	switch field {
	case domain.MatchFieldDescription, domain.MatchFieldMerchant, domain.MatchFieldReference:
		switch match {
		case domain.MatchContains, domain.MatchEquals, domain.MatchRegex:
			if strings.TrimSpace(pattern) == "" {
				return Invalid("pattern is required")
			}
			if utf8.RuneCountInString(pattern) > 200 {
				return Invalid("pattern must be at most 200 characters")
			}
			if match == domain.MatchRegex {
				if _, err := regexp.Compile(ruleCasePrefix + pattern); err != nil {
					return Invalid("pattern is not a valid regular expression: %v", err)
				}
			}
		default:
			return Invalid("match type %q is not valid for the %s field", match, field)
		}
	case domain.MatchFieldAmount:
		switch match {
		case domain.MatchEquals, domain.MatchGT, domain.MatchLT:
		default:
			return Invalid("match type %q is not valid for the amount field", match)
		}
		threshold, err := moneyFromPattern(pattern)
		if err != nil {
			return err
		}
		if threshold.Decimal.IsNegative() {
			return Invalid("amount pattern must be zero or positive")
		}
	default:
		return Invalid("invalid rule field %q", field)
	}
	return nil
}

// moneyFromPattern parses an amount pattern with the same precision rules as
// transaction amounts.
func moneyFromPattern(pattern string) (money.Amount, error) {
	if strings.TrimSpace(pattern) == "" {
		return money.Amount{}, Invalid("pattern is required")
	}
	if utf8.RuneCountInString(pattern) > 16 {
		return money.Amount{}, Invalid("amount pattern must be at most 16 characters")
	}
	a, err := money.Parse(strings.TrimSpace(pattern))
	if err != nil {
		return money.Amount{}, Invalid("amount pattern must be a number like 150000")
	}
	if a.Decimal.Exponent() < -4 {
		return money.Amount{}, Invalid("amount pattern supports at most 4 decimal places")
	}
	return a, nil
}

// NewRule is the input for CreateRule.
type NewRule struct {
	CategoryID uuid.UUID
	Priority   int
	Field      string
	MatchType  string
	Pattern    string
	// Enabled defaults to true; nil leaves the default in place.
	Enabled *bool
}

// RulePatch is a partial rule update; nil fields are left unchanged.
type RulePatch struct {
	CategoryID *uuid.UUID
	Priority   *int
	Field      *string
	MatchType  *string
	Pattern    *string
	Enabled    *bool
}

// ListRules returns the user's rules ordered by priority.
func (s *Service) ListRules(ctx context.Context, userID uuid.UUID) ([]domain.Rule, error) {
	return s.store.ListRules(ctx, userID)
}

// CreateRule validates and stores a new classification rule.
func (s *Service) CreateRule(ctx context.Context, userID uuid.UUID, in NewRule) (domain.Rule, error) {
	field := domain.MatchField(in.Field)
	match := domain.MatchType(in.MatchType)
	pattern := strings.TrimSpace(in.Pattern)
	if err := validateRule(field, match, pattern); err != nil {
		return domain.Rule{}, err
	}
	priority := in.Priority
	if priority == 0 {
		priority = 100
	}
	if priority < 1 || priority > 100000 {
		return domain.Rule{}, Invalid("priority must be between 1 and 100000")
	}
	if err := s.requireCategory(ctx, userID, in.CategoryID); err != nil {
		return domain.Rule{}, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return s.store.CreateRule(ctx, CreateRuleParams{
		UserID:     userID,
		CategoryID: in.CategoryID,
		Priority:   priority,
		Field:      field,
		MatchType:  match,
		Pattern:    pattern,
		Enabled:    enabled,
	})
}

// UpdateRule applies a partial patch; the combined field/match/pattern must
// stay valid.
func (s *Service) UpdateRule(ctx context.Context, userID, ruleID uuid.UUID, in RulePatch) (domain.Rule, error) {
	existing, err := s.getRule(ctx, userID, ruleID)
	if err != nil {
		return domain.Rule{}, err
	}

	field, match, pattern := existing.Field, existing.MatchType, existing.Pattern
	if in.Field != nil {
		field = domain.MatchField(strings.TrimSpace(*in.Field))
	}
	if in.MatchType != nil {
		match = domain.MatchType(strings.TrimSpace(*in.MatchType))
	}
	if in.Pattern != nil {
		pattern = strings.TrimSpace(*in.Pattern)
	}
	if in.Field != nil || in.MatchType != nil || in.Pattern != nil {
		if err := validateRule(field, match, pattern); err != nil {
			return domain.Rule{}, err
		}
	}
	if in.Priority != nil {
		if *in.Priority < 1 || *in.Priority > 100000 {
			return domain.Rule{}, Invalid("priority must be between 1 and 100000")
		}
	}
	if in.CategoryID != nil {
		if err := s.requireCategory(ctx, userID, *in.CategoryID); err != nil {
			return domain.Rule{}, err
		}
	}

	params := UpdateRuleParams{UserID: userID, RuleID: ruleID}
	params.Priority = in.Priority
	params.CategoryID = in.CategoryID
	params.Enabled = in.Enabled
	if in.Pattern != nil {
		params.Pattern = &pattern
	}
	if in.Field != nil {
		params.Field = &field
	}
	if in.MatchType != nil {
		params.MatchType = &match
	}
	return s.store.UpdateRule(ctx, params)
}

// DeleteRule removes a rule; transactions it categorized keep their category.
func (s *Service) DeleteRule(ctx context.Context, userID, ruleID uuid.UUID) error {
	return s.store.DeleteRule(ctx, userID, ruleID)
}

// ruleCategory returns the category picked by the user's first matching
// enabled rule, or nil when no rule matches.
func (s *Service) ruleCategory(ctx context.Context, userID uuid.UUID, row RuleRow) (*uuid.UUID, error) {
	rules, err := s.store.ListEnabledRules(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	m := MatchRule(rules, row)
	if m == nil {
		return nil, nil
	}
	id := m.CategoryID
	return &id, nil
}

// ruleSuggestions bundles the matcher with a category lookup so imported rows
// can show the suggested category. nil signals "no enabled rules".
type ruleSuggestions struct {
	engine     *RuleEngine
	categories map[uuid.UUID]domain.Category
}

// ruleSuggestionsFor loads the user's enabled rules once for a batch of
// candidate rows.
func (s *Service) ruleSuggestionsFor(ctx context.Context, userID uuid.UUID) (*ruleSuggestions, error) {
	rules, err := s.store.ListEnabledRules(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	categories, err := s.store.ListCategories(ctx, userID)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]domain.Category, len(categories))
	for _, c := range categories {
		byID[c.ID] = c
	}
	return &ruleSuggestions{engine: NewRuleEngine(rules), categories: byID}, nil
}

// suggest matches one row and returns its category (falling back to a bare id
// if the lookup is somehow missing), or nil when no rule matches.
func (rs *ruleSuggestions) suggest(row RuleRow) *domain.Category {
	if rs == nil {
		return nil
	}
	rule := rs.engine.Match(row)
	if rule == nil {
		return nil
	}
	if category, ok := rs.categories[rule.CategoryID]; ok {
		return &category
	}
	return &domain.Category{ID: rule.CategoryID}
}

// importRuleRow adapts a parsed statement row for matching.
func importRuleRow(row ImportRow) RuleRow {
	return RuleRow{
		Description: row.Description,
		Merchant:    row.Merchant,
		Reference:   row.Reference,
		Amount:      row.Amount,
	}
}

func (s *Service) getRule(ctx context.Context, userID, ruleID uuid.UUID) (domain.Rule, error) {
	return s.store.GetRule(ctx, userID, ruleID)
}

// EnabledRules is used by tests and callers that need the matcher input.
func (s *Service) EnabledRules(ctx context.Context, userID uuid.UUID) ([]domain.Rule, error) {
	return s.store.ListEnabledRules(ctx, userID)
}
