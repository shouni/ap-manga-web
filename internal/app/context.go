package app

import (
	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/domain"
	"log/slog"

	"ap-manga-web/internal/config"

	"github.com/shouni/gcp-kit/tasks"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-manga-kit/pkg/workflow"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// AppContext はアプリケーションの依存関係を保持します。
type AppContext struct {
	Config *config.Config

	// I/O and Storage
	IOFactory remoteio.IOFactory
	Reader    remoteio.InputReader
	Writer    remoteio.OutputWriter
	Signer    remoteio.URLSigner

	// Asynchronous Task
	TaskEnqueuer *tasks.Enqueuer[domain.GenerateTaskPayload]

	// Business Logic
	Workflow workflow.Workflow

	// External Adapters
	HTTPClient    httpkit.ClientInterface
	SlackNotifier adapters.SlackNotifier
}

// Close は、AppContextが保持するすべてのリソースを解放します。
func (a *AppContext) Close() {
	if a.IOFactory != nil {
		if err := a.IOFactory.Close(); err != nil {
			slog.Error("failed to close IOFactory", "error", err)
		}
	}
	if a.TaskEnqueuer != nil {
		if err := a.TaskEnqueuer.Close(); err != nil {
			slog.Error("failed to close task enqueuer", "error", err)
		}
	}
}
