package main

import (
	"ap-manga-web/internal/config"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ap-manga-web/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadConfig()

	// ここでバリデーションを呼ぶと、より堅牢になるのだ！
	if err := config.ValidateEssentialConfig(cfg); err != nil {
		slog.Error("Config validation failed", "error", err)
		os.Exit(1)
	}

	if err := server.Run(ctx, cfg); err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}
