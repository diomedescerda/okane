// Package money provides a decimal money amount type that is safe for
// financial arithmetic and serializes consistently as a JSON string.
package money

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"
)

// Amount wraps decimal.Decimal so that:
//   - it is always marshalled to JSON as a string ("1234.56"),
//   - it unmarshals from either a string or a JSON number,
//   - it scans from and writes to Postgres NUMERIC values.
type Amount struct {
	decimal.Decimal
}

func New(s string) Amount {
	return Amount{Decimal: decimal.RequireFromString(s)}
}

// Parse converts a string into an Amount, reporting the error instead of
// panicking; used on input paths where the string comes from users.
func Parse(s string) (Amount, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Amount{}, fmt.Errorf("invalid money amount %q", s)
	}
	return Amount{Decimal: d}, nil
}

func FromDecimal(d decimal.Decimal) Amount { return Amount{Decimal: d} }

func FromFloat(f float64) Amount { return Amount{Decimal: decimal.NewFromFloat(f)} }

func Zero() Amount { return Amount{Decimal: decimal.Zero} }

var _ json.Marshaler = Amount{}
var _ json.Unmarshaler = &Amount{}

// String renders the amount in canonical decimal form, preserving precision.
// UI layers are responsible for currency formatting.
func (a Amount) String() string { return a.Decimal.String() }

func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(a.String())), nil
}

func (a *Amount) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		d, err := decimal.NewFromString(s)
		if err != nil {
			return fmt.Errorf("invalid money amount %q: %w", s, err)
		}
		a.Decimal = d
		return nil
	}
	d, err := decimal.NewFromString(string(b))
	if err != nil {
		return fmt.Errorf("invalid money amount %q: %w", string(b), err)
	}
	a.Decimal = d
	return nil
}

// Scan implements sql.Scanner by delegating to decimal.Decimal.
func (a *Amount) Scan(src any) error {
	if src == nil {
		a.Decimal = decimal.Zero
		return nil
	}
	return a.Decimal.Scan(src)
}

// Value implements driver.Valuer by delegating to decimal.Decimal.
func (a Amount) Value() (any, error) { return a.Decimal.Value() }

func (a Amount) Add(b Amount) Amount    { return Amount{a.Decimal.Add(b.Decimal)} }
func (a Amount) Sub(b Amount) Amount    { return Amount{a.Decimal.Sub(b.Decimal)} }
func (a Amount) Neg() Amount            { return Amount{a.Decimal.Neg()} }
func (a Amount) Abs() Amount            { return Amount{a.Decimal.Abs()} }
func (a Amount) IsZero() bool           { return a.Decimal.IsZero() }
func (a Amount) IsNegative() bool       { return a.Decimal.IsNegative() }
func (a Amount) LessThan(b Amount) bool { return a.Decimal.LessThan(b.Decimal) }
func (a Amount) Float64() float64       { f, _ := a.Decimal.Float64(); return f }
func (a Amount) RoundCents() Amount     { return Amount{a.Decimal.Round(2)} }
