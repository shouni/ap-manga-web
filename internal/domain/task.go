package domain

// GenerateTaskPayload は、Cloud Tasks経由で渡される生成指示を表します。
type GenerateTaskPayload struct {
	// Command は実行するワークフローを指定します。(例: "design", "script", "image", "generate", "story")
	Command string `json:"command"`
	// ScriptURL はWebサイト等からコンテンツを取得するためのURLです。(Generate/Scriptモードで使用)
	ScriptURL string `json:"script_url"`
	// InputText は画面から直接入力されたテキストや台本JSONです。(Image/Story/Designモードで使用)
	InputText string `json:"input_text"`
	// Mode は使用するAIモデルを指定します。
	Mode string `json:"mode"`
	// TargetPanels は生成したいパネルのインデックスをカンマ区切りで指定します（例: "0,2"）。
	TargetPanels string `json:"target_panels"`
	// Seed は乱数生成のためのシード値です。
	Seed int64 `json:"seed"`
}
