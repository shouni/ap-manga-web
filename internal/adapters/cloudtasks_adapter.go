package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
)

// TaskAdapter はタスク投入のためのインターフェースを定義します。
type TaskAdapter interface {
	EnqueueGenerateTask(ctx context.Context, payload domain.GenerateTaskPayload) error
	Close() error
}

// CloudTasksAdapter は Google Cloud Tasks を使用した TaskAdapter の実装です。
type CloudTasksAdapter struct {
	client *cloudtasks.Client
	cfg    config.Config
}

// NewCloudTasksAdapter は Cloud Tasks クライアントを初期化します。
func NewCloudTasksAdapter(ctx context.Context, cfg config.Config) (*CloudTasksAdapter, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud tasks client: %w", err)
	}

	return &CloudTasksAdapter{
		client: client,
		cfg:    cfg,
	}, nil
}

// EnqueueGenerateTask は漫画生成タスクを Cloud Tasks キューにエンキューします。
func (a *CloudTasksAdapter) EnqueueGenerateTask(ctx context.Context, payload domain.GenerateTaskPayload) error {
	// リソース名の構築
	parent := fmt.Sprintf("projects/%s/locations/%s/queues/%s",
		a.cfg.ProjectID, a.cfg.LocationID, a.cfg.QueueID)

	// ワーカーエンドポイントの構築
	workerURL := fmt.Sprintf("%s/tasks/generate", a.cfg.ServiceURL)

	// ペイロードのJSON変換
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// タスクリクエストの構築
	// AuthorizationHeader に OidcToken を設定することで、Cloud Tasks が自動的に
	// 有効な IDトークンを取得し、Authorization: Bearer <token> ヘッダーを付与します。
	// タスクリクエストの構築
	req := &cloudtaskspb.CreateTaskRequest{
		Parent: parent,
		Task: &cloudtaskspb.Task{
			// MessageType は HttpRequest フィールドを直接持つ構造体に変わっているのだ
			MessageType: &cloudtaskspb.Task_HttpRequest{
				HttpRequest: &cloudtaskspb.HttpRequest{
					// HttpMethod は cloudtaskspb.HttpMethod_POST を参照するのだ
					HttpMethod: cloudtaskspb.HttpMethod_POST,
					Url:        workerURL,
					Body:       body,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					// AuthorizationHeader も Oneof 型のラッパーが必要なのだ
					AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{
						OidcToken: &cloudtaskspb.OidcToken{
							ServiceAccountEmail: "", // 空文字でデフォルトSAが使われるのだ
							Audience:            a.cfg.TaskAudienceURL,
						},
					},
				},
			},
		},
	}

	createdTask, err := a.client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	slog.Info("Task enqueued successfully",
		"task_name", createdTask.GetName(),
		"audience", a.cfg.TaskAudienceURL,
	)
	return nil
}

func (a *CloudTasksAdapter) Close() error {
	return a.client.Close()
}
