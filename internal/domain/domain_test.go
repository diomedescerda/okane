package domain

import "testing"

func TestNormalizeLanguage(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", LanguageEN, true},
		{"en", LanguageEN, true},
		{"es", LanguageES, true},
		{"ja", LanguageJA, true},
		{"fr", "", false},
		{"EN", "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizeLanguage(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("NormalizeLanguage(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
