package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/pipeline"
)

// デフォルトのシャットダウン猶予時間
const defaultShutdownTimeout = 30 * time.Second

// Run はサーバーの構築、起動、およびライフサイクル管理を行います。
func Run(ctx context.Context, cfg *config.Config) error {
	appCtx, err := builder.BuildAppContext(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to build application context: %w", err)
	}
	defer func() {
		slog.Info("♻️ Closing application context...")
		appCtx.Close()
	}()

	mangaPipeline := pipeline.NewMangaPipeline(appCtx)
	h, err := builder.BuildHandlers(appCtx, mangaPipeline)
	if err != nil {
		return fmt.Errorf("failed to build handlers: %w", err)
	}

	router := NewRouter(cfg, h)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("🚀 Server starting...", "port", cfg.Port, "service_url", cfg.ServiceURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// シグナル処理は main.go から渡された ctx に一任する
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case <-ctx.Done(): // シグナル受信時にここが通知される
		slog.Info("⚠️ Shutdown signal received via context, starting graceful shutdown...")
		return gracefulShutdown(srv, cfg.ShutdownTimeout)
	}
}

// gracefulShutdown は、サーバーを安全に停止させます。
func gracefulShutdown(srv *http.Server, cfgTimeout time.Duration) error {
	timeout := cfgTimeout
	if timeout == 0 {
		timeout = defaultShutdownTimeout
	}

	// シャットダウン用のタイムアウト付きコンテキスト
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Graceful shutdown failed, forcing close", "error", err)
		if closeErr := srv.Close(); closeErr != nil {
			return errors.Join(err, fmt.Errorf("subsequent server close also failed: %w", closeErr))
		}
		return fmt.Errorf("graceful shutdown failed, server was forcibly closed: %w", err)
	}

	slog.Info("✅ Server stopped cleanly")
	return nil
}
