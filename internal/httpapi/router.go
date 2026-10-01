package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenma/okane/internal/auth"
	"github.com/kenma/okane/internal/config"
	"github.com/kenma/okane/internal/finance"
)

// NewRouter builds the application HTTP handler.
func NewRouter(cfg *config.Config, pool *pgxpool.Pool, authSvc *auth.Service, financeSvc *finance.Service, log *slog.Logger) http.Handler {
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
	fh := &financeHandler{svc: financeSvc, log: log}

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

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(authSvc, cfg))
			r.Use(RequireSameOrigin(cfg))

			r.Route("/accounts", func(r chi.Router) {
				r.Get("/", fh.ListAccounts)
				r.Post("/", fh.CreateAccount)
				r.Get("/{accountID}", fh.GetAccount)
				r.Patch("/{accountID}", fh.UpdateAccount)
				r.Delete("/{accountID}", fh.DeleteAccount)
				r.Post("/{accountID}/imports", fh.ImportStatement)
			})

			r.Get("/categories", fh.ListCategories)

			r.Route("/transactions", func(r chi.Router) {
				r.Get("/", fh.ListTransactions)
				r.Post("/", fh.CreateTransaction)
				r.Get("/{transactionID}", fh.GetTransaction)
				r.Patch("/{transactionID}", fh.UpdateTransaction)
				r.Delete("/{transactionID}", fh.DeleteTransaction)
			})

			r.Route("/institutions", func(r chi.Router) {
				r.Get("/", fh.ListInstitutions)
				r.Post("/", fh.CreateInstitution)
				r.Delete("/{institutionID}", fh.DeleteInstitution)
			})

			r.Route("/links", func(r chi.Router) {
				r.Get("/", fh.ListLinks)
				r.Post("/", fh.CreateLink)
				r.Delete("/{linkID}", fh.DeleteLink)
			})

			r.Get("/summary", fh.Summary)
			r.Get("/summary/months", fh.MonthlyTotals)
		})
	})

	return r
}
