package builder

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/controllers/auth"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewServerHandler(ctx context.Context, cfg config.Config) (h http.Handler, err error) {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// OAuth ハンドラーの初期化 (auth パッケージは流用想定)
	// - Auth Handler設定
	isSecure := config.IsSecureURL(cfg.ServiceURL)
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("failed to build auth redirect URL: %w", err)
	}

	authHandler := auth.NewHandler(auth.AuthConfig{
		RedirectURL:    redirectURL,
		ClientID:       cfg.GoogleClientID,
		ClientSecret:   cfg.GoogleClientSecret,
		SessionKey:     cfg.SessionSecret,
		IsSecureCookie: isSecure,
		AllowedEmails:  cfg.AllowedEmails,
		AllowedDomains: cfg.AllowedDomains,
	})

	// 公開ルート
	r.Get("/auth/login", authHandler.Login)
	r.Get("/auth/callback", authHandler.Callback)

	// 認証が必要なルート
	r.Group(func(r chi.Router) {
		r.Use(authHandler.Middleware) // 認証チェック

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello World! AP Manga Runner is Ready."))
		})

		// ここに /generate などのフォーム受付を追加していく
	})

	return r, nil
}
