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

// TaskExecutor は、ライブラリ側のインターフェース定義に合わせた
// domain.GenerateTaskPayload 専用のエグゼキューター定義です。
type TaskExecutor interface {
	Execute(ctx context.Context, payload domain.GenerateTaskPayload) error
}

// NewServerHandler は HTTP ルーティング、認証、各ハンドラーの依存関係を組み立てます。
func NewServerHandler(
	appCtx *AppContext,
	executor TaskExecutor,
) (http.Handler, error) {
	if appCtx.Config.ServiceURL == "" {
		return nil, fmt.Errorf("認証リダイレクトのために ServiceURL の設定が必要です")
	}

	// 1. 各ハンドラーの初期化

	// 認証ハンドラー
	authHandler, err := createAuthHandler(appCtx.Config)
	if err != nil {
		return nil, fmt.Errorf("認証ハンドラーの初期化に失敗しました: %w", err)
	}

	// Webハンドラー
	webHandler, err := web.NewHandler(appCtx.Config, appCtx.TaskEnqueuer, appCtx.Reader, appCtx.Signer)
	if err != nil {
		return nil, fmt.Errorf("Webハンドラーの初期化に失敗しました: %w", err)
	}

	// Workerハンドラー
	workerHandler := worker.NewHandler[domain.GenerateTaskPayload](executor)

	// 2. ルーターの構築
	r := chi.NewRouter()
	setupCommonMiddleware(r)
	setupRoutes(r, appCtx.Config, authHandler, webHandler, workerHandler)

	return r, nil
}

func setupCommonMiddleware(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
}

func setupRoutes(
	r chi.Router,
	cfg config.Config,
	authH *auth.Handler,
	webH *web.Handler,
	workerH *worker.Handler[domain.GenerateTaskPayload],
) {
	// --- 公開ルート ---
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

		setupOutputRoutes(r, cfg.BaseOutputDir, webH)
	})

	// --- Cloud Tasks 専用ルート (Worker 用) ---
	r.Group(func(r chi.Router) {
		r.Use(authH.TaskOIDCVerificationMiddleware)
		r.Post("/tasks/generate", workerH.ProcessTask)
	})
}

func setupOutputRoutes(r chi.Router, baseDir string, webH *web.Handler) {
	prefix := getOutputRoutePrefix(baseDir)
	if prefix == "" {
		return
	}

	r.Route(prefix, func(r chi.Router) {
		r.Get("/{title}", webH.ServeOutput)
		r.Get("/{title}/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
		})
	})
}

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

func getOutputRoutePrefix(baseDir string) string {
	if baseDir == "" {
		return ""
	}
	return "/" + strings.Trim(baseDir, "/")
}
