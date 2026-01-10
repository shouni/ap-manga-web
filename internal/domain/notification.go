package domain

// NotificationRequest は Slack 等の通知コンポーネントで共有されるデータ構造なのだ。
// 生成された漫画のメタデータを通知先に伝えるために使用するのだ。
type NotificationRequest struct {
	// SourceURL は漫画の元になった記事やスクリプトのURL（旧 RepoURL）
	SourceURL string `json:"source_url"`

	// OutputCategory は出力先の種別（例: "manga-output" や "character-design"）
	OutputCategory string `json:"output_category"`

	// TargetTitle は生成物のタイトルやファイル名（旧 FeatureBranch）
	TargetTitle string `json:"target_title"`

	// ExecutionMode は実行されたコマンドとモード（例: "generate / duet"）
	ExecutionMode string `json:"execution_mode"`
}
