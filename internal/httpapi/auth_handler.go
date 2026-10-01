package httpapi

import (
	"errors"
	"net/http"

	"github.com/kenma/okane/internal/auth"
	"github.com/kenma/okane/internal/config"
)

type authHandler struct {
	svc *auth.Service
	cfg *config.Config
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
}

type authResponse struct {
	User  any    `json:"user"`
	Token string `json:"token"`
}

// Register handles POST /api/v1/auth/register.
func (h *authHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	user, token, err := h.svc.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrEmailTaken):
			writeError(w, http.StatusConflict, "email_taken", err.Error())
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusCreated, authResponse{User: user, Token: token})
}

// Login handles POST /api/v1/auth/login.
func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	user, token, err := h.svc.Login(r.Context(), req.Email, req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not sign in")
		return
	}

	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, authResponse{User: user, Token: token})
}

// Logout handles POST /api/v1/auth/logout.
func (h *authHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), TokenFromContext(r.Context())); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not sign out")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /api/v1/auth/me.
func (h *authHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *authHandler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.svc.TTL().Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clientIP returns the request's source address. chi's RealIP middleware has
// already rewritten RemoteAddr from X-Real-IP/X-Forwarded-For when present.
func clientIP(r *http.Request) string {
	return r.RemoteAddr
}
