package builder

import (
	"ap-manga-web/internal/adapters"
	"fmt"
	"net/http"
	"net/url"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/controllers/auth"
	"ap-manga-web/internal/controllers/web"
	"ap-manga-web/internal/controllers/worker"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewServerHandler(
	cfg config.Config,
	taskAdapter adapters.TaskAdapter,
	pipelineExecutor worker.MangaPipelineExecutor, // インターフェースで受け取る
) (http.Handler, error) {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// --- 1. Auth Handler の初期化 ---
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("failed to build auth redirect URL: %w", err)
	}

	authHandler := auth.NewHandler(auth.AuthConfig{
		RedirectURL:     redirectURL,
		TaskAudienceURL: cfg.ServiceURL,
		ClientID:        cfg.GoogleClientID,
		ClientSecret:    cfg.GoogleClientSecret,
		SessionKey:      cfg.SessionSecret,
		IsSecureCookie:  config.IsSecureURL(cfg.ServiceURL),
		AllowedEmails:   cfg.AllowedEmails,
		AllowedDomains:  cfg.AllowedDomains,
	})

	// --- 2. Web Handler (UI) の初期化 ---
	webHandler, err := web.NewHandler(cfg, taskAdapter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize web handler: %w", err)
	}

	// --- 3. Worker Handler の初期化 ---
	workerHandler := worker.NewHandler(cfg, pipelineExecutor)

	// --- 4. 公開ルート (Authentication Entry Points) ---
	r.Get("/auth/login", authHandler.Login)
	r.Get("/auth/callback", authHandler.Callback)

	// --- 5. 認証が必要なルート (Web UI 用) ---
	r.Group(func(r chi.Router) {
		r.Use(authHandler.Middleware) // Google OAuth セッションチェック
		r.Get("/", webHandler.Index)
		r.Post("/generate", webHandler.HandleSubmit)
	})

	// --- 6. Cloud Tasks 専用ルート (Worker 用) ---
	r.Group(func(r chi.Router) {
		// Cloud Tasks が付与する Authorization: Bearer [ID_TOKEN] を検証する
		r.Use(authHandler.TaskOIDCVerificationMiddleware)
		r.Post("/tasks/generate", workerHandler.GenerateTask)
	})

	return r, nil
}
