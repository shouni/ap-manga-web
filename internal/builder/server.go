package builder

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/controllers/auth"
	"ap-manga-web/internal/controllers/web"
	"ap-manga-web/internal/controllers/worker"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewServerHandler は HTTP ルーティング、認証、各ハンドラーの依存関係をすべて組み立てます。
func NewServerHandler(
	appCtx *AppContext,
	pipelineExecutor worker.MangaPipelineExecutor,
) (http.Handler, error) {
	// 1. 基本的なバリデーション（起動時の不備を早期に防ぐ）
	if appCtx.Config.ServiceURL == "" {
		return nil, fmt.Errorf("config ServiceURL is required for auth redirect")
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)

	// --- 各ハンドラーの初期化 ---

	// Auth Handler
	authHandler, err := createAuthHandler(appCtx.Config)
	if err != nil {
		return nil, err
	}

	// Web Handler (UI)
	webHandler, err := web.NewHandler(appCtx.Config, appCtx.TaskEnqueuer, appCtx.Reader, appCtx.Signer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize web handler: %w", err)
	}

	// Worker Handler
	workerHandler := worker.NewHandler(appCtx.Config, pipelineExecutor)

	// --- ルーティング定義 ---

	// 公開ルート
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", authHandler.Login)
		r.Get("/callback", authHandler.Callback)
	})

	// 認証が必要なルート (Web UI 用)
	r.Group(func(r chi.Router) {
		r.Use(authHandler.Middleware)

		// ページ表示
		r.Get("/", webHandler.Index)
		r.Get("/design", webHandler.Design)
		r.Get("/script", webHandler.Script)
		r.Get("/panel", webHandler.Panel)
		r.Get("/page", webHandler.Page)

		// アクション
		r.Post("/generate", webHandler.HandleSubmit)

		// Output Delivery Routes (Manga Viewer)
		if prefix := getOutputRoutePrefix(appCtx.Config.BaseOutputDir); prefix != "" {
			r.Route(prefix, func(r chi.Router) {
				// Maps directly to /{title} and passes it to ServeOutput.
				// This ensures that the viewer has a clean, title-based URL.
				r.Get("/{title}", webHandler.ServeOutput)

				// Normalize trailing slash: redirect /output/title/ to /output/title
				r.Get("/{title}/", func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
				})
			})
		}
	})

	// Cloud Tasks 専用ルート (Worker 用)
	r.Group(func(r chi.Router) {
		r.Use(authHandler.TaskOIDCVerificationMiddleware)
		r.Post("/tasks/generate", workerHandler.GenerateTask)
	})

	return r, nil
}

// --- ヘルパー関数 ---

// createAuthHandler initializes and returns an authentication handler based on the given configuration.
func createAuthHandler(cfg config.Config) (*auth.Handler, error) {
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("failed to build auth redirect URL: %w", err)
	}

	return auth.NewHandler(auth.AuthConfig{
		RedirectURL:     redirectURL,
		TaskAudienceURL: cfg.ServiceURL,
		ClientID:        cfg.GoogleClientID,
		ClientSecret:    cfg.GoogleClientSecret,
		SessionKey:      cfg.SessionSecret,
		IsSecureCookie:  config.IsSecureURL(cfg.ServiceURL),
		AllowedEmails:   cfg.AllowedEmails,
		AllowedDomains:  cfg.AllowedDomains,
	}), nil
}

// getOutputRoutePrefix generates a URL route prefix by trimming slashes from baseDir and prefixing with a single slash.
func getOutputRoutePrefix(baseDir string) string {
	if baseDir == "" {
		return ""
	}
	return "/" + strings.Trim(baseDir, "/")
}
