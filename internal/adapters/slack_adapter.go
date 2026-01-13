package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ap-manga-web/internal/domain"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-notifier/pkg/factory"
	"github.com/shouni/go-notifier/pkg/slack"
)

// --- インターフェース定義 ---

type SlackNotifier interface {
	Notify(ctx context.Context, publicURL, storageURI string, req domain.NotificationRequest) error
	NotifyError(ctx context.Context, errDetail error, req domain.NotificationRequest) error
}

// --- 具象アダプター ---

type SlackAdapter struct {
	httpClient  httpkit.ClientInterface
	webhookURL  string
	slackClient *slack.Client
}

func NewSlackAdapter(httpClient httpkit.ClientInterface, webhookURL string) (*SlackAdapter, error) {
	if webhookURL == "" {
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

// Notify 公開URLとストレージ情報を含む、プロセス完了時のSlack通知送信。
func (a *SlackAdapter) Notify(ctx context.Context, publicURL, storageURI string, req domain.NotificationRequest) error {
	if a.slackClient == nil {
		slog.Info("Slackクライアントが初期化されていないため、通知をスキップします。", "storage_uri", storageURI)
		return nil
	}

	// カテゴリに応じた絵文字の出し分けをすると可愛いのだ！
	icon := "🎨"
	if req.OutputCategory == "design-sheet" {
		icon = "👤"
	} else if req.OutputCategory == "script-json" {
		icon = "📝"
	}

	title := fmt.Sprintf("%s 漫画の錬成が完了しました！", icon)
	content := a.buildSlackContent(publicURL, storageURI, req)

	if err := a.slackClient.SendTextWithHeader(ctx, title, content); err != nil {
		return fmt.Errorf("Slackへの投稿に失敗しました: %w", err)
	}

	slog.Info("Slack に完了通知を送信しました。", "public_url", publicURL)
	return nil
}

// NotifyError エラー詳細と実行メタデータを含むSlackエラー通知の送信。
func (a *SlackAdapter) NotifyError(ctx context.Context, errDetail error, req domain.NotificationRequest) error {
	if a.slackClient == nil {
		slog.Info("Slackクライアントが初期化されていないため、エラー通知をスキップします。", "error", errDetail)
		return nil
	}

	// Slackのmrkdwn形式では、アスタリスク(*)でテキストを囲むと太字として解釈されます。
	title := "❌ 処理中にエラーが発生しました"

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*作品タイトル:* `%s`\n", req.TargetTitle))
	sb.WriteString(fmt.Sprintf("*実行モード:* `%s`\n", req.ExecutionMode))
	sb.WriteString(fmt.Sprintf("*ソース:* %s\n\n", req.SourceURL))

	// エラー詳細をコードブロックで囲むことで、スタックトレースなどの可読性を向上させます。
	sb.WriteString("*エラー内容:*\n")
	sb.WriteString(fmt.Sprintf("```\n%v\n```\n", errDetail))

	// エラー発生時でも保存先カテゴリが判明している場合は、その情報を通知に含めることで調査を容易にします。
	if req.OutputCategory != "" && req.OutputCategory != domain.CategoryNotAvailable {
		sb.WriteString(fmt.Sprintf("\n📍 *カテゴリ:* `%s`", req.OutputCategory))
	}

	content := sb.String()

	if err := a.slackClient.SendTextWithHeader(ctx, title, content); err != nil {
		return fmt.Errorf("Slackへのエラー通知に失敗しました: %w", err)
	}

	slog.Info("Slack にエラー通知を送信しました。", "error", errDetail)
	return nil
}

// buildSlackContent 指定された公開URL、ストレージURI、通知リクエストに基づき、Slack メッセージの内容を生成します。
func (a *SlackAdapter) buildSlackContent(publicURL, storageURI string, req domain.NotificationRequest) string {
	// GCS Console URL の構築
	consoleURL := "https://console.cloud.google.com/storage/browser/" + strings.TrimPrefix(storageURI, "gs://")

	// 基本メッセージの構築
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**作品タイトル:** `%s`\n", req.TargetTitle))
	sb.WriteString(fmt.Sprintf("**実行モード:** `%s`\n", req.ExecutionMode))
	sb.WriteString(fmt.Sprintf("**ソース:** %s\n\n", req.SourceURL))

	// プレビューリンク（publicURLがある場合のみ）
	if publicURL != "" && publicURL != "N/A" {
		sb.WriteString(fmt.Sprintf("🌐 **詳細(ブラウザ):** <%s|ここから確認するのだ！>\n", publicURL))
	}

	// 管理用リンク
	sb.WriteString(fmt.Sprintf("📂 **管理者(Console):** <%s|GCSで直接見るのだ！>\n", consoleURL))
	sb.WriteString(fmt.Sprintf("📍 **保存場所(URI):** `%s`\n\n", storageURI))

	// 集成画像についての案内（Phase 4 がある generate モードのみ）
	if strings.Contains(req.ExecutionMode, "generate") {
		sb.WriteString("✨ _最終ページ画像 (final_page_n.png) も同じフォルダに生成済み様なのだ！_")
	}

	return sb.String()
}
