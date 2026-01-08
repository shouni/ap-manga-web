package worker

import (
	"encoding/json"
	"log"
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
		log.Printf("[ERROR] Failed to decode task payload: %v", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	log.Printf("[INFO] Starting manga generation task for URL: %s", payload.ScriptURL)

	// 2. 実際の生成処理を実行 (Runnerを呼び出す)
	// ここは Cloud Run のタイムアウト（最大60分）まで同期実行されるのだ
	if err := h.runner.Execute(payload); err != nil {
		log.Printf("[ERROR] Manga generation failed: %v", err)
		// 500を返すと Cloud Tasks が自動でリトライしてくれるのだ！
		http.Error(w, "Generation failed", http.StatusInternalServerError)
		return
	}

	log.Printf("[SUCCESS] Manga generation completed for: %s", payload.ScriptURL)
	w.WriteHeader(http.StatusOK)
}
