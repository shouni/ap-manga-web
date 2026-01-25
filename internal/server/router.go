package server

import (
	"net/http"
	"strings"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
	"ap-manga-web/internal/server/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shouni/gcp-kit/auth"
	"github.com/shouni/gcp-kit/worker"
)

// NewRouter は、ミドルウェアとルーティングを統合した http.Handler を構築します。
func NewRouter(cfg config.Config, h *builder.AppHandlers) http.Handler {
	r := chi.NewRouter()

	setupCommonMiddleware(r)
	setupRoutes(r, cfg, h.Auth, h.Web, h.Worker)

	return r
}

func setupCommonMiddleware(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
}

func setupRoutes(
	r chi.Router,
	cfg config.Config,
	authHandler *auth.Handler,
	webHandler *handlers.Handler,
	workerHandler *worker.Handler[domain.GenerateTaskPayload],
) {
	// --- 公開ルート (OAuth2 認証フロー) ---
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", authHandler.Login)
		r.Get("/callback", authHandler.Callback)
	})

	// --- 認証が必要なルート (Web UI 用) ---
	r.Group(func(r chi.Router) {
		r.Use(authHandler.Middleware)

		r.Get("/", webHandler.Index)
		r.Get("/design", webHandler.Design)
		r.Get("/script", webHandler.Script)
		r.Get("/panel", webHandler.Panel)
		r.Get("/page", webHandler.Page)

		r.Post("/generate", webHandler.HandleSubmit)

		setupOutputRoutes(r, cfg.BaseOutputDir, webHandler)
	})

	// --- Cloud Tasks 専用ルート (Worker 用) ---
	r.Group(func(r chi.Router) {
		r.Use(authHandler.TaskOIDCVerificationMiddleware)
		r.Post("/tasks/generate", workerHandler.ProcessTask)
	})
}

func setupOutputRoutes(r chi.Router, baseDir string, webHandler *handlers.Handler) {
	prefix := "/" + strings.Trim(baseDir, "/")
	if prefix == "/" {
		return
	}

	r.Route(prefix, func(r chi.Router) {
		r.Get("/{title}", webHandler.ServeOutput)
		r.Get("/{title}/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
		})
	})
}
