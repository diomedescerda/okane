package finance

import (
	"testing"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/money"
)

func rule(id int64, priority int, category uuid.UUID, field domain.MatchField, match domain.MatchType, pattern string) domain.Rule {
	return domain.Rule{
		ID:         uuid.New(),
		Priority:   priority,
		CategoryID: category,
		Field:      field,
		MatchType:  match,
		Pattern:    pattern,
		Enabled:    true,
	}
}

func sampleRow() RuleRow {
	merchant := "Café San Alberto"
	reference := "REF-993"
	return RuleRow{
		Description: "Compra en Exito norte",
		Merchant:    &merchant,
		Reference:   &reference,
		Amount:      money.New("-85000"),
	}
}

func TestRuleContainsCaseInsensitive(t *testing.T) {
	category := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	r := rule(1, 100, category, domain.MatchFieldDescription, domain.MatchContains, "EXITO")
	got := MatchRule([]domain.Rule{r}, sampleRow())
	if got == nil {
		t.Fatal("expected a match for EXITO")
	}
	if got.CategoryID != category {
		t.Fatalf("category = %s, want %s", got.CategoryID, category)
	}
}

func TestRuleContainsNoPartialWordTrouble(t *testing.T) {
	r := rule(1, 100, uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		domain.MatchFieldDescription, domain.MatchContains, "cxito")
	if m := MatchRule([]domain.Rule{r}, sampleRow()); m != nil {
		t.Fatal("expected no match when the text is not present")
	}
}

func TestRuleEqualsIgnoresCaseAndSpaces(t *testing.T) {
	category := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	r := rule(1, 100, category, domain.MatchFieldMerchant, domain.MatchEquals, "  café san alberto ")
	if got := MatchRule([]domain.Rule{r}, sampleRow()); got == nil {
		t.Fatal("expected a merchant match")
	}
}

func TestRuleRegex(t *testing.T) {
	category := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	r := rule(1, 100, category, domain.MatchFieldReference, domain.MatchRegex, `^ref-\d+$`)
	if got := MatchRule([]domain.Rule{r}, sampleRow()); got == nil {
		t.Fatal("expected a reference regex match")
	}
	// Regex matches anywhere in the value, not only at the start.
	r2 := rule(2, 100, category, domain.MatchFieldDescription, domain.MatchRegex, `norte$`)
	if got := MatchRule([]domain.Rule{r2}, sampleRow()); got == nil {
		t.Fatal("expected a description regex match")
	}
}

func TestRuleAmountMagnitude(t *testing.T) {
	category := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	gt := rule(1, 100, category, domain.MatchFieldAmount, domain.MatchGT, "70000")
	if got := MatchRule([]domain.Rule{gt}, sampleRow()); got == nil {
		t.Fatal("expected 85000 to match amount > 70000")
	}
	// An income of the same magnitude matches magnitude comparisons too.
	income := sampleRow()
	income.Amount = money.New("2500000")
	lt := rule(2, 100, category, domain.MatchFieldAmount, domain.MatchLT, "3000000")
	if got := MatchRule([]domain.Rule{lt}, income); got == nil {
		t.Fatal("expected 2500000 to match amount < 3000000")
	}
	eq := rule(3, 100, category, domain.MatchFieldAmount, domain.MatchEquals, "85000")
	if got := MatchRule([]domain.Rule{eq}, sampleRow()); got == nil {
		t.Fatal("expected 85000 to equal 85000")
	}
	if got := MatchRule([]domain.Rule{eq}, income); got != nil {
		t.Fatal("expected different magnitudes not to match")
	}
}

func TestRulePriorityFirstMatchWins(t *testing.T) {
	low, high := uuid.MustParse("11111111-1111-4111-8111-111111111111"), uuid.MustParse("22222222-2222-4222-8222-222222222222")
	rules := []domain.Rule{
		rule(1, 50, high, domain.MatchFieldDescription, domain.MatchContains, "exito"),
		rule(2, 100, low, domain.MatchFieldAmount, domain.MatchGT, "1000"),
	}
	got := MatchRule(rules, sampleRow())
	if got == nil || got.CategoryID != high {
		t.Fatalf("expected the priority 50 rule to win, got %+v", got)
	}
}

func TestRuleNilFieldsAndInvalidRegex(t *testing.T) {
	category := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	nilMerchant := rule(1, 100, category, domain.MatchFieldMerchant, domain.MatchContains, "café")
	row := sampleRow()
	row.Merchant = nil
	if got := MatchRule([]domain.Rule{nilMerchant}, row); got != nil {
		t.Fatal("expected no match with nil merchant")
	}

	broken := domain.Rule{
		ID: uuid.New(), Priority: 1, CategoryID: category,
		Field: domain.MatchFieldDescription, MatchType: domain.MatchRegex,
		Pattern: "(" /* invalid */, Enabled: true,
	}
	if got := MatchRule([]domain.Rule{broken}, sampleRow()); got != nil {
		t.Fatal("an invalid regex must be skipped, not panic or match")
	}
}

func TestValidateRule(t *testing.T) {
	cases := []struct {
		name    string
		field   domain.MatchField
		match   domain.MatchType
		pattern string
		wantErr bool
	}{
		{"contains ok", domain.MatchFieldDescription, domain.MatchContains, "netflix", false},
		{"equals ok", domain.MatchFieldMerchant, domain.MatchEquals, "Nequi", false},
		{"regex ok", domain.MatchFieldDescription, domain.MatchRegex, "even?app", false},
		{"regex invalid", domain.MatchFieldDescription, domain.MatchRegex, "(unclosed", true},
		{"amount gt ok", domain.MatchFieldAmount, domain.MatchGT, "1500000", false},
		{"amount contains invalid", domain.MatchFieldAmount, domain.MatchContains, "1500000", true},
		{"amount bad number", domain.MatchFieldAmount, domain.MatchGT, "12,5x", true},
		{"amount negative", domain.MatchFieldAmount, domain.MatchLT, "-3", true},
		{"strings gt invalid", domain.MatchFieldDescription, domain.MatchGT, "x", true},
		{"empty pattern", domain.MatchFieldDescription, domain.MatchContains, "   ", true},
		{"bad field", domain.MatchField("nope"), domain.MatchContains, "x", true},
	}
	for _, tc := range cases {
		err := validateRule(tc.field, tc.match, tc.pattern)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: validateRule(%s, %s, %q) err = %v, wantErr %v",
				tc.name, tc.field, tc.match, tc.pattern, err, tc.wantErr)
		}
	}
}
