package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/auth"
	"github.com/kenma/okane/internal/config"
	"github.com/kenma/okane/internal/domain"
)

func TestUpdateMeValidation(t *testing.T) {
	h := &authHandler{
		svc: auth.NewService(nil, time.Hour),
		cfg: &config.Config{},
	}

	t.Run("requires authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me", strings.NewReader(`{"language":"es"}`))
		rec := httptest.NewRecorder()
		h.UpdateMe(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("rejects unsupported language", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me", strings.NewReader(`{"language":"fr"}`))
		ctx := context.WithValue(req.Context(), userCtxKey, domain.User{
			ID:       uuid.New(),
			Email:    "user@example.com",
			Language: domain.LanguageEN,
		})
		rec := httptest.NewRecorder()
		h.UpdateMe(rec, req.WithContext(ctx))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me", strings.NewReader(`{invalid`))
		ctx := context.WithValue(req.Context(), userCtxKey, domain.User{
			ID:       uuid.New(),
			Email:    "user@example.com",
			Language: domain.LanguageEN,
		})
		rec := httptest.NewRecorder()
		h.UpdateMe(rec, req.WithContext(ctx))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestRegisterRejectsUnsupportedLanguage(t *testing.T) {
	h := &authHandler{
		svc: auth.NewService(nil, time.Hour),
		cfg: &config.Config{},
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"user@example.com","password":"password123","name":"Kenma","language":"de"}`),
	)
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
