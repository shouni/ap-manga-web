package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(context.Background()); err != nil {
		slog.Error("アプリケーションの実行に失敗しました", "error", err)
		os.Exit(1)
	}

}

// run は、設定ロード、バリデーション、サーバーのライフサイクル管理を行う
func run(ctx context.Context) error {
	cfg := config.LoadConfig()

	// サーバーの構築
	handler, err := builder.NewServerHandler(ctx, cfg)
	if err != nil {
		return fmt.Errorf("サーバーの構築に失敗しました: %w", err)
	}

	// 5. サーバー起動
	slog.Info("🚀 サーバーを起動中...", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("サーバーの起動に失敗しました: %w", err)
	}

	return nil
}
