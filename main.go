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
	"time"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
)

func main() {
	// 構造化ログの設定 (Cloud Logging との親和性)
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

	// 2. サーバー（ルーターと全ハンドラー）の構築
	handler, err := builder.NewServerHandler(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to build server handler: %w", err)
	}

	// 3. HTTP Server の構成
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	// 4. エラーチャネルによる非同期実行
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("🚀 Server starting...", "port", cfg.Port, "service_url", cfg.ServiceURL)
		serverErrors <- srv.ListenAndServe()
	}()

	// 5. シャットダウン信号の待機 (SIGINT, SIGTERM)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}

	case <-shutdown:
		slog.Info("Starting graceful shutdown...")

		// タイムアウト付きのシャットダウンコンテキストを作成
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			// 正常終了に失敗した場合は強制終了
			if err := srv.Close(); err != nil {
				return fmt.Errorf("could not stop server gracefully: %w", err)
			}
		}
		slog.Info("Server stopped")
	}

	return nil
}
