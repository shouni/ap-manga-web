package app

import (
	"log/slog"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	"github.com/shouni/gcp-kit/tasks"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-manga-kit/pkg/workflow"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// Container はアプリケーションの依存関係（DIコンテナ）を保持します。
type Container struct {
	Config *config.Config

	// I/O and Storage
	RemoteIO *RemoteIO

	// Asynchronous Task
	TaskEnqueuer *tasks.Enqueuer[domain.GenerateTaskPayload]

	// Business Logic
	Workflow *Workflow

	// External Adapters
	HTTPClient    httpkit.ClientInterface
	SlackNotifier adapters.SlackNotifier
}

type RemoteIO struct {
	Factory remoteio.IOFactory
	Reader  remoteio.InputReader
	Writer  remoteio.OutputWriter
	Signer  remoteio.URLSigner
}

type Workflow struct {
	DesignRunner     workflow.DesignRunner
	ScriptRunner     workflow.ScriptRunner
	PanelImageRunner workflow.PanelImageRunner
	PageImageRunner  workflow.PageImageRunner
	PublishRunner    workflow.PublishRunner
}

// Close は、Context が保持するすべての外部接続リソースを安全に解放します。
func (c *Container) Close() {
	if c.RemoteIO.Factory != nil {
		if err := c.RemoteIO.Factory.Close(); err != nil {
			slog.Error("failed to close IOFactory", "error", err)
		}
	}
	if c.TaskEnqueuer != nil {
		if err := c.TaskEnqueuer.Close(); err != nil {
			slog.Error("failed to close task enqueuer", "error", err)
		}
	}
}
