package auth_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kenma/okane/internal/auth"
	"github.com/kenma/okane/internal/db"
	"github.com/kenma/okane/internal/domain"
)

func TestUserLanguageFlow(t *testing.T) {
	dsn := os.Getenv("OKANE_DATABASE_URL")
	if dsn == "" {
		t.Skip("OKANE_DATABASE_URL not set; skipping database integration test")
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	svc := auth.NewService(pool, time.Hour)
	email := fmt.Sprintf("lang-test-%d@example.com", time.Now().UnixNano())
	user, token, err := svc.Register(ctx, email, "password123", "Test User", "ja")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	}()

	if user.Language != domain.LanguageJA {
		t.Fatalf("registered user language = %q, want %q", user.Language, domain.LanguageJA)
	}

	updated, err := svc.UpdateLanguage(ctx, user.ID, "es")
	if err != nil {
		t.Fatalf("update language: %v", err)
	}
	if updated.Language != domain.LanguageES {
		t.Fatalf("updated language = %q, want %q", updated.Language, domain.LanguageES)
	}

	authed, err := svc.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if authed.Language != domain.LanguageES {
		t.Fatalf("authenticated user language = %q, want %q", authed.Language, domain.LanguageES)
	}

	if _, err := svc.UpdateLanguage(ctx, user.ID, "de"); err == nil {
		t.Fatal("expected error for unsupported language")
	}
}
