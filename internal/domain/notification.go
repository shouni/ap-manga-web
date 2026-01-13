package domain

const CategoryNotAvailable = "N/A"

// NotificationRequest は Slack 等の通知コンポーネントで共有されるデータ構造です。
// 生成された漫画のメタデータを通知先に伝えるために使用します。
type NotificationRequest struct {
	// SourceURL は、漫画の元になった記事やスクリプトのURLです。
	SourceURL string `json:"source_url"`

	// OutputCategory は、出力先の種別です。(例: "manga-output", "character-design")
	OutputCategory string `json:"output_category"`

	// TargetTitle は、生成物のタイトルやファイル名です。(旧 FeatureBranch)
	TargetTitle string `json:"target_title"`

	// ExecutionMode は、実行されたコマンドとモードです。(例: "generate / duet")
	ExecutionMode string `json:"execution_mode"`
}
