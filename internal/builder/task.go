package builder

import (
	"context"
	"fmt"
	"net/url"

	"github.com/shouni/gcp-kit/tasks"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

// buildTaskEnqueuer は、Cloud Tasks エンキューアを初期化します。
func buildTaskEnqueuer(ctx context.Context, cfg *config.Config) (*tasks.Enqueuer[domain.GenerateTaskPayload], error) {
	workerURL, err := url.JoinPath(cfg.ServiceURL, "/tasks/generate")
	if err != nil {
		return nil, fmt.Errorf("failed to build worker URL: %w", err)
	}

	taskCfg := tasks.Config{
		ProjectID:           cfg.ProjectID,
		LocationID:          cfg.LocationID,
		QueueID:             cfg.QueueID,
		WorkerURL:           workerURL,
		ServiceAccountEmail: cfg.ServiceAccountEmail,
		Audience:            cfg.TaskAudienceURL,
	}
	return tasks.NewEnqueuer[domain.GenerateTaskPayload](ctx, taskCfg)
}
