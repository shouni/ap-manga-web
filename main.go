package main

import (
	"context"
	"log/slog"
	"os"

	"ap-manga-web/internal/server"
)

func main() {
	// アプリケーション全体のロガーをJSON形式に設定します。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := server.Run(context.Background()); err != nil {
		slog.Error("Application fatal error", "error", err)
		os.Exit(1)
	}
}
