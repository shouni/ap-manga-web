package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"

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
	client              *cloudtasks.Client
	parent              string // キューの親リソース名
	workerURL           string // タスクが送信されるワーカーのエンドポイントURL
	audience            string // OIDCトークンの検証に使用する Audience
	serviceAccountEmail string // OIDCトークン生成に使用するサービスアカウント ★追加
}

// NewCloudTasksAdapter は Cloud Tasks クライアントを初期化し、固定の設定値を事前構築します。
func NewCloudTasksAdapter(ctx context.Context, cfg config.Config) (*CloudTasksAdapter, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud tasks client: %w", err)
	}

	parent := fmt.Sprintf("projects/%s/locations/%s/queues/%s",
		cfg.ProjectID, cfg.LocationID, cfg.QueueID)

	workerURL, err := url.JoinPath(cfg.ServiceURL, "/tasks/generate")
	if err != nil {
		return nil, fmt.Errorf("failed to construct worker URL: %w", err)
	}

	return &CloudTasksAdapter{
		client:              client,
		parent:              parent,
		workerURL:           workerURL,
		audience:            cfg.TaskAudienceURL,
		serviceAccountEmail: cfg.ServiceAccountEmail,
	}, nil
}

// EnqueueGenerateTask は漫画生成タスクを Cloud Tasks キューにエンキューします。
func (a *CloudTasksAdapter) EnqueueGenerateTask(ctx context.Context, payload domain.GenerateTaskPayload) error {
	// ペイロードのJSON変換
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// --- ログ出力の追加なのだ！ ---
	slog.Info("Preparing to create Cloud Task",
		"parent", a.parent,
		"worker_url", a.workerURL,
		"service_account", a.serviceAccountEmail,
		"audience", a.audience,
		"payload_size", len(body),
		"payload_json", string(body),
	)

	// タスクリクエストの構築
	req := &cloudtaskspb.CreateTaskRequest{
		Parent: a.parent,
		Task: &cloudtaskspb.Task{
			MessageType: &cloudtaskspb.Task_HttpRequest{
				HttpRequest: &cloudtaskspb.HttpRequest{
					HttpMethod: cloudtaskspb.HttpMethod_POST,
					Url:        a.workerURL,
					Body:       body,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					// AuthorizationHeader に OIDCトークンを設定
					AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{
						OidcToken: &cloudtaskspb.OidcToken{
							ServiceAccountEmail: a.serviceAccountEmail,
							Audience:            a.audience,
						},
					},
				},
			},
		},
	}

	createdTask, err := a.client.CreateTask(ctx, req)
	if err != nil {
		// エラー時も詳細をログに残すと助かるのだ
		slog.Error("Failed to create task",
			"error", err,
			"target_url", a.workerURL,
		)
		return fmt.Errorf("failed to create task: %w", err)
	}

	slog.Info("Task enqueued successfully",
		"task_name", createdTask.GetName(),
		"audience", a.audience,
		"service_account", a.serviceAccountEmail, // 確認用にログ出力
	)
	return nil
}

// Close は Cloud Tasks クライアントの接続を閉じます。
func (a *CloudTasksAdapter) Close() error {
	return a.client.Close()
}
