package domain

// GenerateTaskPayload は Cloud Tasks 経由で渡される生成指示なのだ
type GenerateTaskPayload struct {
	// Command は実行するワークフローを指定するのだ (design, script, image, generate, story)
	Command string `json:"command"`

	// ScriptURL は Web サイト等からコンテンツを取得するための URL（Generate/Scriptモードで使用）
	ScriptURL string `json:"script_url"`

	// InputText は画面から直接入力されたテキストや台本JSON（Image/Story/Designモードで使用）
	InputText string `json:"input_text"`

	// Mode は使用するAIモデルの指定（config.GeminiModel などを上書きする場合に使用）
	Mode string `json:"mode"`

	// PanelLimit は生成する最大パネル数
	PanelLimit int `json:"panel_limit"`
}
