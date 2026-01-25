package main

import (
	"context"
	"log/slog"
	"os"

	"ap-manga-web/internal/server"
)

func main() {
	// JSON形式のログをデフォルトに設定するのだ
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := server.Run(context.Background()); err != nil {
		slog.Error("Application fatal error", "error", err)
		os.Exit(1)
	}
}
