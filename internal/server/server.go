package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/pipeline"
)

// デフォルトのシャットダウン猶予時間
const defaultShutdownTimeout = 30 * time.Second

// Run は、サーバーの構築、起動、およびライフサイクル管理を行います。
// Configを引数で受け取ることで、環境変数への直接依存を排除しています。
func Run(ctx context.Context, cfg *config.Config) error {
	// 1. アプリケーションコンテキストの構築
	appCtx, err := builder.BuildAppContext(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to build application context: %w", err)
	}
	defer func() {
		slog.Info("♻️ Closing application context...")
		appCtx.Close()
	}()

	// 2. ハンドラーとルーターの組み立て
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

	// 3. サーバー起動とシグナル待機
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("🚀 Server starting...", "port", cfg.Port, "service_url", cfg.ServiceURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// システムシグナルの待機
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-quit:
		slog.Info("⚠️ Signal received, starting graceful shutdown...", "signal", sig)
		return gracefulShutdown(srv, cfg.ShutdownTimeout)
	}
}

// gracefulShutdown は、サーバーを安全に停止させます。
func gracefulShutdown(srv *http.Server, cfgTimeout time.Duration) error {
	timeout := cfgTimeout
	if timeout == 0 {
		timeout = defaultShutdownTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Graceful shutdown failed, forcing close", "error", err)
		if closeErr := srv.Close(); closeErr != nil {
			return errors.Join(err, fmt.Errorf("subsequent server close also failed: %w", closeErr))
		}
		return fmt.Errorf("graceful shutdown failed, server was forcibly closed: %w", err)
	}

	slog.Info("✅ Server stopped cleanly")
	return nil
}
