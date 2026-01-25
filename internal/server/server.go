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

// Run は、設定ロード、バリデーション、サーバーのライフサイクル管理を行います。
func Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	if err := config.ValidateEssentialConfig(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	appCtx, err := builder.BuildAppContext(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to build application context: %w", err)
	}
	defer func() {
		slog.Info("♻️ Closing application context...")
		appCtx.Close()
	}()

	// 1. ハンドラーの組み立て
	mangaPipeline := pipeline.NewMangaPipeline(appCtx)
	h, err := builder.BuildHandlers(appCtx, mangaPipeline)
	if err != nil {
		return fmt.Errorf("failed to build handlers: %w", err)
	}

	// 2. ルーターの構築
	router := NewRouter(cfg, h)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// --- サーバー起動とシグナル待機 ---
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("🚀 Server starting...", "port", cfg.Port, "service_url", cfg.ServiceURL)
		serverErrors <- srv.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}

	case <-shutdown:
		slog.Info("⚠️ Starting graceful shutdown...")

		// タイムアウト値の決定
		timeout := cfg.ShutdownTimeout
		if timeout == 0 {
			timeout = defaultShutdownTimeout
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// グレースフルシャットダウンの実行
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Graceful shutdown failed, forcing close", "error", err)

			// シャットダウンに失敗した場合は強制的にクローズしてリソースを解放する
			if closeErr := srv.Close(); closeErr != nil {
				return fmt.Errorf("could not stop server: shutdown error: %v, close error: %v", err, closeErr)
			}
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}

		slog.Info("✅ Server stopped cleanly")
	}

	return nil
}
