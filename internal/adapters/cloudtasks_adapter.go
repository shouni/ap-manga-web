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
	client    *cloudtasks.Client
	parent    string // キューの親リソース名 (プロジェクト、ロケーション、キューIDを含む)
	workerURL string // タスクが送信されるワーカーのエンドポイントURL
	audience  string // OIDCトークンの検証に使用する Audience
}

// NewCloudTasksAdapter は Cloud Tasks クライアントを初期化し、固定の設定値を事前構築します。
func NewCloudTasksAdapter(ctx context.Context, cfg config.Config) (*CloudTasksAdapter, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud tasks client: %w", err)
	}

	// 実行時に変化しない値を初期化時に構築しておくことで、パフォーマンスを最適化します
	parent := fmt.Sprintf("projects/%s/locations/%s/queues/%s",
		cfg.ProjectID, cfg.LocationID, cfg.QueueID)

	workerURL := fmt.Sprintf("%s/tasks/generate", cfg.ServiceURL)

	return &CloudTasksAdapter{
		client:    client,
		parent:    parent,
		workerURL: workerURL,
		audience:  cfg.TaskAudienceURL,
	}, nil
}

// EnqueueGenerateTask は漫画生成タスクを Cloud Tasks キューにエンキューします。
func (a *CloudTasksAdapter) EnqueueGenerateTask(ctx context.Context, payload domain.GenerateTaskPayload) error {
	// ペイロードのJSON変換
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// タスクリクエストの構築
	req := &cloudtaskspb.CreateTaskRequest{
		Parent: a.parent,
		Task: &cloudtaskspb.Task{
			// MessageType は oneof であり、ここでは HttpRequest を指定します。
			MessageType: &cloudtaskspb.Task_HttpRequest{
				HttpRequest: &cloudtaskspb.HttpRequest{
					// HttpMethod は enum 値で指定します。
					HttpMethod: cloudtaskspb.HttpMethod_POST,
					Url:        a.workerURL,
					Body:       body,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					// AuthorizationHeader は oneof であり、ここでは OidcToken を指定します。
					// Cloud Tasks が有効な ID トークンを自動的に取得し、Authorization ヘッダーを付与します。
					AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{
						OidcToken: &cloudtaskspb.OidcToken{
							// 空文字の場合、環境のデフォルトサービスアカウントが使用されます。
							ServiceAccountEmail: "",
							Audience:            a.audience,
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
		"audience", a.audience,
	)
	return nil
}

// Close は Cloud Tasks クライアントの接続を閉じます。
func (a *CloudTasksAdapter) Close() error {
	return a.client.Close()
}
