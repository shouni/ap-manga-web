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

const defaultSessionName = "ap-manga-session"

// TaskExecutor は、非同期タスクのペイロードを受け取り、
// 対応するビジネスロジックを実行する責務を抽象化します。
type TaskExecutor interface {
	// Execute はデコードされたペイロードを受け取り、漫画生成の各パイプラインを実行します。
	Execute(ctx context.Context, payload domain.GenerateTaskPayload) error
}

// NewServerHandler は HTTP ルーティング、認証、各Handlerの依存関係をすべて組み立てます。
// 起動時に ServiceURL のチェックを行い、各種Handlerの初期化に失敗した場合はエラーを返します。
func NewServerHandler(
	appCtx *AppContext,
	executor TaskExecutor,
) (http.Handler, error) {
	if appCtx.Config.ServiceURL == "" {
		return nil, fmt.Errorf("認証リダイレクトのために ServiceURL の設定が必要です")
	}

	// 1. 各Handlerの初期化

	// 認証Handlerの初期化
	authHandler, err := createAuthHandler(appCtx.Config)
	if err != nil {
		return nil, fmt.Errorf("認証Handlerの初期化に失敗しました: %w", err)
	}

	// Web UI 用Handlerの初期化
	webHandler, err := web.NewHandler(appCtx.Config, appCtx.TaskEnqueuer, appCtx.Reader, appCtx.Signer)
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
// 公開ルート、認証必須ルート、Cloud Tasks 専用ルートに論理的に分割します。
func setupRoutes(
	r chi.Router,
	cfg config.Config,
	authH *auth.Handler,
	webH *web.Handler,
	workerH *worker.Handler[domain.GenerateTaskPayload],
) {
	// --- 公開ルート (OAuth2 認証フロー) ---
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", authH.Login)
		r.Get("/callback", authH.Callback)
	})

	// --- 認証が必要なルート (Web UI 用) ---
	r.Group(func(r chi.Router) {
		r.Use(authH.Middleware)

		r.Get("/", webH.Index)
		r.Get("/design", webH.Design)
		r.Get("/script", webH.Script)
		r.Get("/panel", webH.Panel)
		r.Get("/page", webH.Page)

		r.Post("/generate", webH.HandleSubmit)

		// 生成済み成果物の配信エンドポイントを設定
		setupOutputRoutes(r, cfg.BaseOutputDir, webH)
	})

	// --- Cloud Tasks 専用ルート (Worker 用) ---
	// OIDC トークンによる検証ミドルウェアを適用します。
	r.Group(func(r chi.Router) {
		r.Use(authH.TaskOIDCVerificationMiddleware)
		r.Post("/tasks/generate", workerH.ProcessTask)
	})
}

// setupOutputRoutes は生成された漫画成果物を表示するための動的ルーティングを設定します。
// 末尾のスラッシュを正規化するリダイレクト処理を含みます。
func setupOutputRoutes(r chi.Router, baseDir string, webH *web.Handler) {
	prefix := getOutputRoutePrefix(baseDir)
	if prefix == "" {
		return
	}

	r.Route(prefix, func(r chi.Router) {
		// /{title} を ServeOutput にマッピング
		r.Get("/{title}", webH.ServeOutput)
		// /title/ へのアクセスを /title に正規化
		r.Get("/{title}/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
		})
	})
}

// createAuthHandler は config.Config から認証ライブラリ用の設定を構築し、ハンドラーを生成します。
func createAuthHandler(cfg config.Config) (*auth.Handler, error) {
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("リダイレクトURLの構築に失敗しました: %w", err)
	}

	return auth.NewHandler(auth.Config{
		ClientID:          cfg.GoogleClientID,
		ClientSecret:      cfg.GoogleClientSecret,
		RedirectURL:       redirectURL,
		SessionAuthKey:    cfg.SessionSecret,
		SessionEncryptKey: cfg.SessionSecret,
		SessionName:       defaultSessionName,
		IsSecureCookie:    strings.HasPrefix(cfg.ServiceURL, "https"),
		AllowedEmails:     cfg.AllowedEmails,
		AllowedDomains:    cfg.AllowedDomains,
		TaskAudienceURL:   cfg.ServiceURL,
	})
}

// getOutputRoutePrefix は BaseOutputDir を基に、URL のプレフィックス（例: "/output"）を生成します。
func getOutputRoutePrefix(baseDir string) string {
	if baseDir == "" {
		return ""
	}
	return "/" + strings.Trim(baseDir, "/")
}
