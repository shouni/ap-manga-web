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
	// AspectRatio はデザインシート生成のアスペクト比です（"1:1"/"9:16"/"16:9"、Designモードで使用）。
	// 空文字の場合は go-manga-kit 側の既定値（16:9）にフォールバックします。
	AspectRatio string `json:"aspect_ratio"`
	// DesignLayout はデザインシートのレイアウト種別です（Designモードで使用）。
	// runner.DesignLayoutSingleView（"single"）を指定すると、他の生成物のアスペクト比別
	// 参照アンカー向けの単一ポーズレイアウトになります。空文字なら従来通りの3面図です。
	DesignLayout string `json:"design_layout"`
}
