package auth

import (
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("VerifyPassword returned false for the correct password")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("VerifyPassword returned true for a wrong password")
	}
	if VerifyPassword("not-a-hash", "whatever") {
		t.Fatal("VerifyPassword accepted a malformed hash")
	}
	if VerifyPassword("", "") {
		t.Fatal("VerifyPassword accepted an empty hash")
	}
}

func TestHashIsSalty(t *testing.T) {
	a, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of the same password must differ (random salt)")
	}
}
