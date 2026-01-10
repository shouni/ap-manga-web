package main

import (
	"ap-manga-web/internal/pipeline"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(context.Background()); err != nil {
		slog.Error("Application failed", "error", err)
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

	// 1. Pipeline の作成 (pipeline は builder をインポートしている)
	mangaPipeline := pipeline.NewMangaPipeline(cfg)

	// 3. Builder を使ってハンドラーを作成
	// ここで pipeline を渡すことで、builder パッケージ自体は pipeline を知る必要がなくなる
	handler, err := builder.NewServerHandler(cfg, taskAdapter, mangaPipeline)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

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
		slog.Info("Starting graceful shutdown...")

		// ハードコードを避け、設定値を使用
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			// エラー内容をログに出力 (指摘の反映)
			slog.Error("Graceful shutdown failed", "error", err)
			if err := srv.Close(); err != nil {
				return fmt.Errorf("could not stop server gracefully: %w", err)
			}
		}
		slog.Info("Server stopped")
	}

	return nil
}
