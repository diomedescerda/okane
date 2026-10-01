package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/kenma/okane/internal/auth"
	"github.com/kenma/okane/internal/config"
	"github.com/kenma/okane/internal/domain"
)

type ctxKey int

const (
	userCtxKey ctxKey = iota
	tokenCtxKey
)

// UserFromContext returns the authenticated user, if any.
func UserFromContext(ctx context.Context) (domain.User, bool) {
	u, ok := ctx.Value(userCtxKey).(domain.User)
	return u, ok
}

// TokenFromContext returns the raw session token used for the request.
func TokenFromContext(ctx context.Context) string {
	t, _ := ctx.Value(tokenCtxKey).(string)
	return t
}

// bearerToken extracts a session token from the Authorization header.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// RequireAuth rejects unauthenticated requests and injects the user into the
// request context. Tokens are accepted from a Bearer header (API clients such
// as the iOS app) or the session cookie (web app).
func RequireAuth(svc *auth.Service, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				if c, err := r.Cookie(cfg.SessionCookieName()); err == nil {
					token = c.Value
				}
			}
			if token == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}

			user, err := svc.Authenticate(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session")
				return
			}

			ctx := context.WithValue(r.Context(), userCtxKey, user)
			ctx = context.WithValue(ctx, tokenCtxKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSameOrigin protects cookie-authenticated state changes from CSRF.
// Requests authenticated with a Bearer token are exempt (browsers never attach
// those automatically). When an Origin header is present it must match the
// configured app URL.
func RequireSameOrigin(cfg *config.Config) func(http.Handler) http.Handler {
	safeMethods := map[string]bool{
		http.MethodGet: true, http.MethodHead: true, http.MethodOptions: true,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if safeMethods[r.Method] || bearerToken(r) != "" {
				next.ServeHTTP(w, r)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && origin != cfg.AppURL {
				writeError(w, http.StatusForbidden, "bad_origin", "request origin is not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Logger logs one line per request.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			level := slog.LevelInfo
			if ww.Status() >= 500 {
				level = slog.LevelError
			}
			log.Log(r.Context(), level, "http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
				"remote", r.RemoteAddr,
			)
		})
	}
}

// CORS allows the configured web app origin with credentials.
func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" && origin == cfg.AppURL {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
