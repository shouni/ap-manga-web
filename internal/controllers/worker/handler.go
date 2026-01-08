package worker

import (
	"encoding/json"
	"log/slog" // log を log/slog に変更
	"net/http"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

// MangaRunner は実際の生成ロジックを持つインターフェースなのだ
type MangaRunner interface {
	Execute(payload domain.GenerateTaskPayload) error
}

type Handler struct {
	cfg    config.Config
	runner MangaRunner
}

func NewHandler(cfg config.Config, runner MangaRunner) *Handler {
	return &Handler{
		cfg:    cfg,
		runner: runner,
	}
}

// GenerateTask は /tasks/generate へのリクエストを処理するのだ
func (h *Handler) GenerateTask(w http.ResponseWriter, r *http.Request) {
	// 1. Cloud Tasks からのペイロードをデコード
	var payload domain.GenerateTaskPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Error("Failed to decode task payload", "error", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// 処理開始のログ
	slog.Info("Starting manga generation task",
		"script_url", payload.ScriptURL,
		"mode", payload.Mode,
		"panel_limit", payload.PanelLimit,
	)

	// 2. 実際の生成処理を実行 (Runnerを呼び出す)
	// Cloud Run のタイムアウト制限内で同期実行されるのだ
	if err := h.runner.Execute(payload); err != nil {
		// 失敗時は Error レベルでログ出力
		slog.Error("Manga generation failed",
			"script_url", payload.ScriptURL,
			"error", err,
		)
		// 5xxを返すと Cloud Tasks が自動でリトライしてくれるのだ！
		http.Error(w, "Generation failed", http.StatusInternalServerError)
		return
	}

	// 3. 完了時のログ
	slog.Info("Manga generation completed successfully",
		"script_url", payload.ScriptURL,
	)
	w.WriteHeader(http.StatusOK)
}
