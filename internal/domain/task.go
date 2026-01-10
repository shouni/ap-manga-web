package domain

// GenerateTaskPayload は、Cloud Tasks経由で渡される生成指示を表します。
type GenerateTaskPayload struct {
	// Command は実行するワークフローを指定します。(例: "design", "script", "image", "generate", "story")
	Command string `json:"command"`
	// ScriptURL はWebサイト等からコンテンツを取得するためのURLです。(Generate/Scriptモードで使用)
	ScriptURL string `json:"script_url"`
	// InputText は画面から直接入力されたテキストや台本JSONです。(Image/Story/Designモードで使用)
	InputText string `json:"input_text"`
	// Mode は使用するAIモデルを指定します。(config.GeminiModelなどを上書きする場合に使用)
	Mode string `json:"mode"`
	// PanelLimit は生成する最大パネル数を指定します。
	PanelLimit int `json:"panel_limit"`
	// Seed は乱数生成のためのシード値です。
	Seed int64 `json:"seed"`
}
