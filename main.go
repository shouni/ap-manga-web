// ap-manga-web は、AIによる漫画生成ワークフローを提供するWebアプリケーションです。
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shouni/ap-manga-web/internal/config"
	"github.com/shouni/ap-manga-web/internal/server"
)

func main() {
	// ロガーの設定（構造化ログの復元）
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run はアプリケーションの初期化とサーバー起動を行います。defer によるクリーンアップが
// os.Exit で無視されないよう、終了コードの決定は main 側に委ねます。
func run() error {
	// シグナルに反応するコンテキストの作成
	// これにより、SIGINT/SIGTERM受信時に ctx.Done() が閉じる
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 設定のロードとバリデーション
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Config loading failed", "error", err)
		return err
	}
	if err := cfg.ValidateEssentialConfig(); err != nil {
		slog.Error("Config validation failed", "error", err)
		return err
	}

	// サーバーの実行
	if err := server.Run(ctx, cfg); err != nil {
		slog.Error("Application failed", "error", err)
		return err
	}
	return nil
}
