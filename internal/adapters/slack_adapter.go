package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"ap-manga-web/internal/domain"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-notifier/pkg/slack"
)

const (
	slackErrorTitle         = "❌ 処理中にエラーが発生しました"
	slackErrorContentHeader = "*エラー内容:*\n"
)

type SlackAdapter struct {
	webhookURL  string
	slackClient *slack.Client
}

func NewSlackAdapter(httpClient httpkit.RequestExecutor, webhookURL string) (*SlackAdapter, error) {
	if webhookURL == "" {
		// オプショナル機能として扱い、空のままインスタンスを返す
		return &SlackAdapter{}, nil
	}

	if httpClient == nil {
		return nil, errors.New("http client cannot be nil")
	}

	client, err := slack.NewClient(httpClient, webhookURL)
	if err != nil {
		return nil, fmt.Errorf("Slackクライアントの初期化に失敗しました: %w", err)
	}

	return &SlackAdapter{
		webhookURL:  webhookURL,
		slackClient: client,
	}, nil
}

// Notify 公開URLとストレージ情報を含む、プロセス完了時のSlack通知送信。
func (s *SlackAdapter) Notify(ctx context.Context, publicURL, storageURI string, req domain.NotificationRequest) error {
	if s.webhookURL == "" || s.slackClient == nil {
		slog.Info("Slack通知が無効化されているか、クライアントが未初期化のためスキップします。", "storage_uri", storageURI)
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
	content := s.buildSlackContent(publicURL, storageURI, req)

	if err := s.slackClient.SendTextWithHeader(ctx, title, content); err != nil {
		return fmt.Errorf("Slackへの投稿に失敗しました: %w", err)
	}

	slog.Info("Slack に完了通知を送信しました。", "public_url", publicURL)
	return nil
}

// NotifyError エラー詳細と実行メタデータを含むSlackエラー通知の送信。
func (s *SlackAdapter) NotifyError(ctx context.Context, errDetail error, req domain.NotificationRequest) error {
	if s.slackClient == nil {
		slog.Info("Slackクライアントが初期化されていないため、エラー通知をスキップします。", "error", errDetail)
		return nil
	}

	title := slackErrorTitle
	var sb strings.Builder
	fmt.Fprintf(&sb, "*作品タイトル:* `%s`\n", req.TargetTitle)
	fmt.Fprintf(&sb, "*実行モード:* `%s`\n", req.ExecutionMode)
	fmt.Fprintf(&sb, "*ソース:* %s\n\n", req.SourceURL)

	// エラー詳細をコードブロックで囲むことで、スタックトレースなどの可読性を向上させます。
	sb.WriteString(slackErrorContentHeader)
	fmt.Fprintf(&sb, "```\n%+v\n```\n", errDetail)

	// エラー発生時でも保存先カテゴリが判明している場合は、その情報を通知に含めることで調査を容易にします。
	if req.OutputCategory != "" && req.OutputCategory != domain.NotAvailable {
		fmt.Fprintf(&sb, "\n📍 *カテゴリ:* `%s`", req.OutputCategory)
	}

	content := sb.String()

	if err := s.slackClient.SendTextWithHeader(ctx, title, content); err != nil {
		return fmt.Errorf("Slackへのエラー通知に失敗しました: %w", err)
	}

	slog.Info("Slack にエラー通知を送信しました。", "error", errDetail)
	return nil
}

// buildSlackContent 指定された公開URL、ストレージURI、通知リクエストに基づき、Slack メッセージの内容を生成します。
func (s *SlackAdapter) buildSlackContent(publicURL, storageURI string, req domain.NotificationRequest) string {
	// GCS Console URL の構築
	trimmedPath := strings.TrimPrefix(storageURI, "gs://")
	consoleURL := "https://console.cloud.google.com/storage/browser/" + trimmedPath

	// ファイル名にドットが含まれる場合は、詳細画面 (_details/) へのリンクに差し替える
	if strings.Contains(path.Base(trimmedPath), ".") {
		consoleURL = "https://console.cloud.google.com/storage/browser/_details/" + trimmedPath
	}

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
