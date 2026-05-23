package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/agentzero/server/internal/auth"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func NewRouter(db *sql.DB, av *auth.AppleVerifier, ti *auth.TokenIssuer, logger *slog.Logger) http.Handler {
	h := &Handlers{db: db, apple: av, tokens: ti, logger: logger}

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(requestLogger(logger))
	r.Use(corsMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(optionalAuth(ti))

		r.Post("/auth/apple", h.appleSignIn)

		r.Get("/feed/today", h.getToday)
		r.Get("/categories", h.listCategories)
		r.Get("/agents", h.listAgents)
		r.Get("/agents/{slug}", h.getAgent)
		r.Get("/agents/{slug}/reviews", h.listReviews)

		r.Group(func(r chi.Router) {
			r.Use(requireAuth)
			r.Get("/me", h.me)
			r.Get("/me/installed", h.listInstalled)
			r.Get("/me/sessions", h.listSessions)
			r.Post("/agents/{slug}/install", h.install)
			r.Delete("/agents/{slug}/install", h.uninstall)
			r.Post("/agents/{slug}/reviews", h.submitReview)
			r.Post("/agents/{slug}/sessions", h.createSession)
			r.Get("/sessions/{id}/messages", h.listMessages)
			r.Post("/sessions/{id}/messages", h.sendMessage)
		})
	})

	return r
}
