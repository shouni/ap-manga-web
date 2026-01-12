package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/pipeline"
)

func main() {
	// JSON形式のログをデフォルトに設定するのだ
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(context.Background()); err != nil {
		slog.Error("Application fatal error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// 1. 設定のロードとバリデーション
	cfg := config.LoadConfig()
	if err := config.ValidateEssentialConfig(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// 2. アダプターの初期化とライフサイクル管理
	taskAdapter, err := adapters.NewCloudTasksAdapter(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize cloud tasks adapter: %w", err)
	}
	defer func() {
		slog.Info("Closing task adapter...")
		if err := taskAdapter.Close(); err != nil {
			slog.Error("Failed to close task adapter", "error", err)
		}
	}()

	// 3. アプリケーションコンテキストの構築
	appCtx, err := builder.BuildAppContext(ctx, cfg)
	if err != nil {
		// ここでは Fatal せず、run の戻り値としてエラーを返すのが綺麗なのだ
		return fmt.Errorf("failed to build application context: %w", err)
	}
	mangaPipeline := pipeline.NewMangaPipeline(appCtx)

	// 4. ハンドラーの作成 (Web & Worker を含む)
	handler, err := builder.NewServerHandler(cfg, appCtx, taskAdapter, mangaPipeline)
	if err != nil {
		return fmt.Errorf("failed to create server handler: %w", err)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	// 5. サーバー起動
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("🚀 Server starting...",
			"port", cfg.Port,
			"service_url", cfg.ServiceURL,
			"project_id", cfg.ProjectID,
		)
		serverErrors <- srv.ListenAndServe()
	}()

	// 6. シグナル待機 (Graceful Shutdown)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}

	case <-shutdown:
		slog.Info("Starting graceful shutdown...")

		// ShutdownTimeout が設定されていない場合の安全策
		timeout := cfg.ShutdownTimeout
		if timeout == 0 {
			timeout = 30 // デフォルト30秒なのだ
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Graceful shutdown failed", "error", err)
			if err := srv.Close(); err != nil {
				return fmt.Errorf("could not stop server gracefully: %w", err)
			}
		}
		slog.Info("Server stopped cleanly")
	}

	return nil
}
