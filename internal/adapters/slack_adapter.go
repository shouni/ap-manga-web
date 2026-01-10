package adapters

import (
	"context"
	"fmt"
	"log/slog"

	"ap-manga-web/internal/domain"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-notifier/pkg/factory"
	"github.com/shouni/go-notifier/pkg/slack"
)

// --- インターフェース定義 ---

// SlackNotifier は Slack への通知機能を提供する契約を定義します。
type SlackNotifier interface {
	Notify(ctx context.Context, publicURL, storageURI string, req domain.NotificationRequest) error
}

// --- 具象アダプター ---

// SlackAdapter は SlackNotifier インターフェースを満たす具象型です。
type SlackAdapter struct {
	httpClient  httpkit.ClientInterface
	webhookURL  string
	slackClient *slack.Client
}

// NewSlackAdapter は新しいアダプターインスタンスを作成します。
func NewSlackAdapter(httpClient httpkit.ClientInterface, webhookURL string) (*SlackAdapter, error) {
	if webhookURL == "" {
		// webhookURL がない場合はクライアントを初期化しない
		return &SlackAdapter{webhookURL: webhookURL}, nil
	}
	client, err := factory.GetSlackClient(httpClient)
	if err != nil {
		return nil, fmt.Errorf("Slackクライアントの初期化に失敗したのだ: %w", err)
	}

	return &SlackAdapter{
		httpClient:  httpClient,
		webhookURL:  webhookURL,
		slackClient: client,
	}, nil
}

// Notify は Slack に漫画生成完了の通知を投稿します。
func (a *SlackAdapter) Notify(ctx context.Context, publicURL, storageURI string, req domain.NotificationRequest) error {
	// 1. Slackクライアントの存在チェック
	if a.slackClient == nil {
		slog.Info("Slackクライアントが初期化されていないため、通知をスキップします。", "storage_uri", storageURI)
		return nil
	}

	// 2. メッセージの作成
	title := "🎨 漫画の錬成が完了しました！"
	content := a.buildSlackContent(publicURL, storageURI, req)

	// 3. Slack 投稿処理を実行 (保持しているクライアントを使用)
	if err := a.slackClient.SendTextWithHeader(ctx, title, content); err != nil {
		return fmt.Errorf("Slackへの投稿に失敗しました: %w", err)
	}

	slog.Info("Slack に完了通知を送信しました。", "public_url", publicURL)
	return nil
}

// buildSlackContent は漫画生成に特化したメッセージ本文を組み立てるのだ。
func (a *SlackAdapter) buildSlackContent(publicURL, storageURI string, req domain.NotificationRequest) string {
	content := fmt.Sprintf(
		"**作品タイトル:** `%s`\n"+
			"**実行モード:** `%s`\n"+
			"**ソース:** %s\n\n"+
			"**詳細(ブラウザ):** <%s|ここから確認するのだ！>\n"+
			"**保存場所(GCS):** `%s`",
		req.TargetTitle,
		req.ExecutionMode,
		req.SourceURL,
		publicURL,
		storageURI,
	)
	return content
}
