package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenma/okane/internal/auth"
	"github.com/kenma/okane/internal/config"
)

// NewRouter builds the application HTTP handler.
func NewRouter(cfg *config.Config, pool *pgxpool.Pool, authSvc *auth.Service, log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(Logger(log))
	r.Use(chimw.Recoverer)
	r.Use(CORS(cfg))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "db_unavailable", "database is not reachable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	ah := &authHandler{svc: authSvc, cfg: cfg}

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", ah.Register)
			r.Post("/login", ah.Login)

			r.Group(func(r chi.Router) {
				r.Use(RequireAuth(authSvc, cfg))
				r.Use(RequireSameOrigin(cfg))
				r.Post("/logout", ah.Logout)
				r.Get("/me", ah.Me)
			})
		})

		// Authenticated application endpoints are mounted in later milestones
		// (accounts, transactions, categories, budgets, AI reports).
	})

	return r
}
