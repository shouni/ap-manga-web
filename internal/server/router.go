package server

import (
	"log/slog"
	"net/http"
	"strings"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/server/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter は、ミドルウェアとルーティングを統合した http.Handler を構築します。
func NewRouter(cfg *config.Config, h *builder.AppHandlers) http.Handler {
	r := chi.NewRouter()
	setupCommonMiddleware(r)
	setupRoutes(r, cfg, h)

	return r
}

// setupCommonMiddleware は、 ログ記録、リカバリ、パス クリーニングなど、提供されたルーターの共通ミドルウェアを構成します。
func setupCommonMiddleware(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
}

// setupRoutes は、認証、Web UI、ワーカー タスク処理に基づいてアプリケーションの HTTP ルートを初期化します。
func setupRoutes(
	r chi.Router,
	cfg *config.Config,
	h *builder.AppHandlers,
) {
	// --- 1. 公開ルート (ヘルスチェック) ---
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if h == nil {
		slog.Warn("AppHandlers is nil, skipping application routes registration")
		return
	}

	// --- 2. 認証関連エンドポイント (OAuth2 フロー) ---
	if h.Auth != nil {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", h.Auth.Login)
			r.Get("/callback", h.Auth.Callback)
		})
	}

	// --- 認証が必要なルート (Web UI 用) ---
	r.Group(func(r chi.Router) {
		if h.Auth == nil {
			if h.Web != nil {
				slog.Error("Auth handler is nil, skipping protected web routes")
			}
			return
		}

		// ログインチェック & POST時のCSRF検証を適用
		r.Use(h.Auth.Middleware)

		// GETリクエスト時にCSRFトークンがなければ自動生成してセッションに保存するミドルウェア
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					// セッションにトークンがない場合のみ生成
					csrfToken := h.Auth.GetCSRFTokenFromSession(r)
					if csrfToken == "" {
						token, err := h.Auth.GenerateAndSaveCSRFToken(w, r)
						if err != nil {
							slog.Error("Failed to auto-generate CSRF token", "error", err)
							http.Error(w, "Internal Server Error", http.StatusInternalServerError)
							return
						}
						csrfToken = token
					}
					r = r.WithContext(handlers.WithCSRFToken(r.Context(), csrfToken))
				}
				next.ServeHTTP(w, r)
			})
		})

		if h.Web != nil {
			r.Get("/", h.Web.Index)
			r.Get("/design", h.Web.Design)
			r.Get("/script", h.Web.Script)
			r.Get("/panel", h.Web.Panel)
			r.Get("/page", h.Web.Page)

			r.Post("/generate", h.Web.HandleSubmit)

			setupOutputRoutes(r, cfg.BaseOutputDir, h.Web)
		}
	})

	// --- Cloud Tasks 専用ルート (Worker 用) ---
	r.Group(func(r chi.Router) {
		if h.Auth == nil {
			if h.Worker != nil {
				slog.Error("Auth handler is nil, skipping worker routes")
			}
			return
		}

		// Cloud Tasks からの OIDC トークンを検証 (セッション不要)
		r.Use(h.Auth.TaskOIDCVerificationMiddleware)

		if h.Worker != nil {
			r.Post("/tasks/generate", h.Worker.ProcessTask)
		}
	})
}

// setupOutputRoutes は、指定されたベースディレクトリとハンドラを使用して、指定されたルーター上の出力関連のルートを設定します。
func setupOutputRoutes(r chi.Router, baseDir string, webHandler *handlers.Handler) {
	prefix := "/" + strings.Trim(baseDir, "/")
	if prefix == "/" {
		return
	}

	r.Route(prefix, func(r chi.Router) {
		r.Get("/{title}", webHandler.ServePreview)
		r.Get("/{title}/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
		})
	})
}
