package domain

import "context"

// Pipeline は、デコードされたペイロードを受け取って実際の処理を行うインターフェースです。
type Pipeline interface {
	// Execute は、指定されたコンテキストに基づいて GenerateTaskPayload を処理し、問題が発生した場合はエラーを返します。
	Execute(ctx context.Context, payload GenerateTaskPayload) (err error)
}

// Notifier は、生成されたコンテンツまたはエラーに関する通知を指定されたターゲットまたはチャネルに送信するためのインターフェイスです。
type Notifier interface {
	// Notify は、パブリック URL やストレージ URL などのメタデータを含む通知をターゲットに送信します。
	Notify(ctx context.Context, publicURL, storageURI string, req NotificationRequest) error
	// NotifyError は、関連メタデータを含むエラー通知をターゲットに送信します。
	NotifyError(ctx context.Context, err error, req NotificationRequest) error
}
