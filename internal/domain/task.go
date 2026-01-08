package domain

// GenerateTaskPayload は Cloud Tasks 経由で渡される生成指示なのだ
type GenerateTaskPayload struct {
	ScriptURL  string `json:"script_url"`
	Mode       string `json:"mode"`
	PanelLimit int    `json:"panel_limit"`
}
