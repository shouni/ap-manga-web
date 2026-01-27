package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/server"
)

func main() {
	// 1. ロガーの設定（構造化ログの復元）
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// 2. シグナルに反応するコンテキストの作成
	// これにより、SIGINT/SIGTERM受信時に ctx.Done() が閉じる
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 3. 設定のロードとバリデーション
	cfg := config.LoadConfig()
	if err := config.ValidateEssentialConfig(cfg); err != nil {
		slog.Error("Config validation failed", "error", err)
		os.Exit(1)
	}

	// 4. サーバーの実行
	if err := server.Run(ctx, cfg); err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}
