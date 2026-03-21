package domain

import (
	"context"

	"github.com/shouni/go-manga-kit/ports"
)

// Pipeline は、デコードされたペイロードを受け取って実際の処理を行うインターフェースです。
type Pipeline interface {
	// Execute は、指定されたコンテキストに基づいて GenerateTaskPayload を処理し、問題が発生した場合はエラーを返します。
	Execute(ctx context.Context, payload GenerateTaskPayload) (err error)
}

// Workflows は、漫画の生成、公開などのワークフローを実行するためのインターフェースです。
type Workflows interface {
	// Design は指定されたキャラクターIDのキャラクターを生成します。
	Design(ctx context.Context, charIDs []string, seed int64, outputDir string) (string, int64, error)
	// Script は指定されたURLから台本を作成します。
	Script(ctx context.Context, sourceURL, mode string) (*ports.MangaResponse, error)
	// Panel は指定された漫画のページを保存します。
	Panel(ctx context.Context, manga *ports.MangaResponse, outputPath string) (*ports.MangaResponse, error)
	// Page は指定された漫画のページを保存します。
	Page(ctx context.Context, manga *ports.MangaResponse, outputPath string) ([]string, error)
	// Publish は指定された漫画を公開します。
	Publish(ctx context.Context, manga *ports.MangaResponse, outputDir string) (*ports.PublishResult, error)
}

// Notifier は、生成されたコンテンツまたはエラーに関する通知を指定されたターゲットまたはチャネルに送信するためのインターフェイスです。
type Notifier interface {
	// Notify は、パブリック URL やストレージ URL などのメタデータを含む通知をターゲットに送信します。
	Notify(ctx context.Context, publicURL, storageURI string, req NotificationRequest) error
	// NotifyError は、関連メタデータを含むエラー通知をターゲットに送信します。
	NotifyError(ctx context.Context, err error, req NotificationRequest) error
}
