package builder

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
	"ap-manga-web/internal/server"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shouni/gcp-kit/auth"
	"github.com/shouni/gcp-kit/worker"
)

const defaultSessionName = "ap-manga-session"

// TaskExecutor は、非同期タスクのペイロードを受け取り、
// 対応するビジネスロジックを実行する責務を抽象化します。
type TaskExecutor interface {
	// Execute はデコードされたペイロードを受け取り、漫画生成の各パイプラインを実行します。
	Execute(ctx context.Context, payload domain.GenerateTaskPayload) error
}

// NewServerHandler は HTTP ルーティング、認証、各Handlerの依存関係をすべて組み立てます。
func NewServerHandler(
	appCtx *AppContext,
	executor TaskExecutor,
) (http.Handler, error) {
	if appCtx.Config.ServiceURL == "" {
		return nil, fmt.Errorf("認証リダイレクトのために ServiceURL の設定が必要です")
	}

	// 1. 各Handlerの初期化

	// 認証Handlerの初期化 (appCtx を渡して HttpClient を利用可能にする)
	authHandler, err := createAuthHandler(appCtx)
	if err != nil {
		return nil, fmt.Errorf("認証Handlerの初期化に失敗しました: %w", err)
	}

	// Web UI 用Handlerの初期化
	webHandler, err := server.NewHandler(appCtx.Config, appCtx.TaskEnqueuer, appCtx.Reader, appCtx.Signer)
	if err != nil {
		return nil, fmt.Errorf("WebHandlerの初期化に失敗しました: %w", err)
	}

	// 非同期ワーカー用Handlerの初期化
	workerHandler := worker.NewHandler[domain.GenerateTaskPayload](executor)

	// 2. ルーターの構築
	r := chi.NewRouter()
	setupCommonMiddleware(r)
	setupRoutes(r, appCtx.Config, authHandler, webHandler, workerHandler)

	return r, nil
}

// setupCommonMiddleware は標準的なログ出力、パニック復旧、パス正規化のミドルウェアを設定します。
func setupCommonMiddleware(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
}

// setupRoutes はアプリケーションのルーティング構造を定義します。
func setupRoutes(
	r chi.Router,
	cfg config.Config,
	authHandler *auth.Handler,
	webHandler *server.Handler,
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

// setupOutputRoutes は生成された漫画成果物を表示するための動的ルーティングを設定します。
func setupOutputRoutes(r chi.Router, baseDir string, webHandler *server.Handler) {
	prefix := getOutputRoutePrefix(baseDir)
	if prefix == "" {
		return
	}

	r.Route(prefix, func(r chi.Router) {
		r.Get("/{title}", webHandler.ServeOutput)
		r.Get("/{title}/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
		})
	})
}

// createAuthHandler は AppContext から認証ライブラリ用の設定を構築し、ハンドラーを生成します。
func createAuthHandler(appCtx *AppContext) (*auth.Handler, error) {
	cfg := appCtx.Config
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("リダイレクトURLの構築に失敗しました: %w", err)
	}

	// 指定された HttpClient の判定メソッドを使用
	isSecure := appCtx.HTTPClient.IsSecureServiceURL(cfg.ServiceURL)

	return auth.NewHandler(auth.Config{
		ClientID:          cfg.GoogleClientID,
		ClientSecret:      cfg.GoogleClientSecret,
		RedirectURL:       redirectURL,
		SessionAuthKey:    cfg.SessionSecret,
		SessionEncryptKey: cfg.SessionEncryptKey,
		SessionName:       defaultSessionName,
		IsSecureCookie:    isSecure,
		AllowedEmails:     cfg.AllowedEmails,
		AllowedDomains:    cfg.AllowedDomains,
		TaskAudienceURL:   cfg.ServiceURL,
	})
}

// getOutputRoutePrefix は BaseOutputDir を基に URL プレフィックスを生成します。
func getOutputRoutePrefix(baseDir string) string {
	if baseDir == "" {
		return ""
	}
	return "/" + strings.Trim(baseDir, "/")
}
