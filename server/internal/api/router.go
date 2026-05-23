package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/agentzero/server/internal/agent"
	"github.com/agentzero/server/internal/auth"
	"github.com/agentzero/server/internal/tools"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Deps 是 router 装配所需的全部外部依赖。
type Deps struct {
	DB       *sql.DB
	Apple    *auth.AppleVerifier
	Tokens   *auth.TokenIssuer
	Logger   *slog.Logger
	Runner   *agent.Runner
	Broker   *agent.Broker
	Registry *tools.Registry
}

func NewRouter(d Deps) http.Handler {
	h := &Handlers{db: d.DB, apple: d.Apple, tokens: d.Tokens, logger: d.Logger}
	m := &missionAPI{
		Handlers: h,
		runner:   d.Runner,
		broker:   d.Broker,
		registry: d.Registry,
	}

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(requestLogger(d.Logger))
	r.Use(corsMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(optionalAuth(d.Tokens))

		// 公开
		r.Post("/auth/apple", h.appleSignIn)
		r.Get("/tools", m.listTools)

		// 需登录
		r.Group(func(r chi.Router) {
			r.Use(requireAuth)
			r.Get("/me", h.me)

			r.Get("/missions", m.listMine)
			r.Post("/missions", m.dispatch)
			r.Get("/missions/{id}", m.detail)
			r.Get("/missions/{id}/stream", m.stream)
			r.Post("/missions/{id}/abort", m.abort)
			r.Get("/missions/{id}/artifacts/{aid}/content", m.artifactContent)
		})
	})

	return r
}
