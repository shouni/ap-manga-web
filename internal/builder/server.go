package builder

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/controllers/auth"
	"ap-manga-web/internal/controllers/web"
	"ap-manga-web/internal/controllers/worker"
	"ap-manga-web/internal/runner"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewServerHandler(ctx context.Context, cfg config.Config) (http.Handler, error) {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// --- 1. Auth Handler の初期化 ---
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("failed to build auth redirect URL: %w", err)
	}

	authHandler := auth.NewHandler(auth.AuthConfig{
		RedirectURL:    redirectURL,
		ClientID:       cfg.GoogleClientID,
		ClientSecret:   cfg.GoogleClientSecret,
		SessionKey:     cfg.SessionSecret,
		IsSecureCookie: config.IsSecureURL(cfg.ServiceURL),
		AllowedEmails:  cfg.AllowedEmails,
		AllowedDomains: cfg.AllowedDomains,
	})

	// --- 2. Web Handler (UI) の初期化 ---
	webHandler, err := web.NewHandler(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize web handler: %w", err)
	}

	// --- 3. Worker Handler の初期化 ---
	mangaRunner := runner.NewRunner(cfg)
	workerHandler := worker.NewHandler(cfg, mangaRunner)

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
	// [Blocker] セキュリティ修正: OIDCトークン検証を強制適用
	r.Group(func(r chi.Router) {
		// Cloud Tasks が付与する Authorization: Bearer [ID_TOKEN] を検証する
		r.Use(authHandler.TaskOIDCVerificationMiddleware)
		r.Post("/tasks/generate", workerHandler.GenerateTask)
	})

	return r, nil
}
