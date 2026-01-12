package builder

import (
	"fmt"
	"net/http"
	"net/url"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/controllers/auth"
	"ap-manga-web/internal/controllers/web"
	"ap-manga-web/internal/controllers/worker"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewServerHandler は HTTP ルーティング、認証、各ハンドラーの依存関係をすべて組み立てるのだ。
func NewServerHandler(
	cfg config.Config,
	taskAdapter adapters.TaskAdapter,
	pipelineExecutor worker.MangaPipelineExecutor,
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

	// --- 4. 公開ルート ---
	r.Get("/auth/login", authHandler.Login)
	r.Get("/auth/callback", authHandler.Callback)

	// --- 5. 認証が必要なルート (Web UI 用) ---
	r.Group(func(r chi.Router) {
		r.Use(authHandler.Middleware)

		r.Get("/", webHandler.Index)        // 一括生成 (main)
		r.Get("/design", webHandler.Design) // キャラ設計
		r.Get("/script", webHandler.Script) // 台本抽出

		// 💡 メソッド名とパスを panel / page に変更したのだ！
		r.Get("/panel", webHandler.Panel) // コマ画像生成 (旧 Image)
		r.Get("/page", webHandler.Page)   // ページ構成 (旧 Story)

		// 全ての POST はここへ集約なのだ
		r.Post("/generate", webHandler.HandleSubmit)
	})

	// --- 6. Cloud Tasks 専用ルート (Worker 用) ---
	r.Group(func(r chi.Router) {
		r.Use(authHandler.TaskOIDCVerificationMiddleware)
		r.Post("/tasks/generate", workerHandler.GenerateTask)
	})

	return r, nil
}
