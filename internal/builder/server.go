package builder

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/controllers/web"
	"ap-manga-web/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shouni/gcp-kit/auth"
	"github.com/shouni/gcp-kit/worker"
)

// taskExecutor は、非同期タスクを受け取りレビュー処理のパイプラインを実行するインターフェースです。
type taskExecutor interface {
	Execute(ctx context.Context, payload domain.GenerateTaskPayload) error
}

// NewServerHandler は HTTP ルーティング、認証、各ハンドラーの依存関係をすべて組み立てます。
func NewServerHandler(
	appCtx *AppContext,
	taskExecutor taskExecutor,
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
	workerHandler := worker.NewHandler(taskExecutor)

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
		r.Post("/tasks/generate", workerHandler.ProcessTask)
	})

	return r, nil
}

// --- ヘルパー関数 ---

// createAuthHandler は認証ハンドラーを初期化します
func createAuthHandler(cfg config.Config) (*auth.Handler, error) {
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("failed to build auth redirect URL: %w", err)
	}

	// SessionEncryptKey は AES 用に 16/24/32 バイトである必要があります
	// SessionSecret を流用していますが、本来は環境変数で個別に持つのが理想です
	return auth.NewHandler(auth.Config{
		ClientID:          cfg.GoogleClientID,
		ClientSecret:      cfg.GoogleClientSecret,
		RedirectURL:       redirectURL,
		SessionAuthKey:    cfg.SessionSecret, // 署名用
		SessionEncryptKey: cfg.SessionSecret, // 暗号化用 (長さが16,24,32であること)
		SessionName:       "ap-manga-session",
		IsSecureCookie:    strings.HasPrefix(cfg.ServiceURL, "https"),
		AllowedEmails:     cfg.AllowedEmails,
		AllowedDomains:    cfg.AllowedDomains,
		TaskAudienceURL:   cfg.ServiceURL,
	})
}

// getOutputRoutePrefix generates a URL route prefix by trimming slashes from baseDir and prefixing with a single slash.
func getOutputRoutePrefix(baseDir string) string {
	if baseDir == "" {
		return ""
	}
	return "/" + strings.Trim(baseDir, "/")
}
