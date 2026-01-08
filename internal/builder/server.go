package builder

import (
	"context"
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

	// 1. Auth Handler
	redirectURL, _ := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	authHandler := auth.NewHandler(auth.AuthConfig{
		RedirectURL:    redirectURL,
		ClientID:       cfg.GoogleClientID,
		ClientSecret:   cfg.GoogleClientSecret,
		SessionKey:     cfg.SessionSecret,
		IsSecureCookie: config.IsSecureURL(cfg.ServiceURL),
		AllowedEmails:  cfg.AllowedEmails,
		AllowedDomains: cfg.AllowedDomains,
	})

	// 2. Web Handler (UI)
	webHandler, err := web.NewHandler(cfg)
	if err != nil {
		return nil, err
	}

	// 3. Worker Handler (Task Execution)
	mangaRunner := runner.NewRunner(cfg)
	workerHandler := worker.NewHandler(cfg, mangaRunner)

	// Routing
	r.Get("/auth/login", authHandler.Login)
	r.Get("/auth/callback", authHandler.Callback)

	r.Group(func(r chi.Router) {
		r.Use(authHandler.Middleware)
		r.Get("/", webHandler.Index)
		// r.Post("/generate", webHandler.HandleSubmit) // 今後実装
	})

	r.Post("/tasks/generate", workerHandler.GenerateTask)

	return r, nil
}
