package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

// MangaPipelineExecutor は、非同期タスクを受け取って漫画生成フローを実行するインターフェースなのだ。
type MangaPipelineExecutor interface {
	Execute(ctx context.Context, payload domain.GenerateTaskPayload) error
}

// Handler は Cloud Tasks からのリクエストを処理する HTTP ハンドラーなのだ。
type Handler struct {
	cfg      config.Config
	pipeline MangaPipelineExecutor
}

// NewHandler は、依存関係を注入して Worker ハンドラーを初期化します。
func NewHandler(cfg config.Config, pipeline MangaPipelineExecutor) *Handler {
	return &Handler{
		cfg:      cfg,
		pipeline: pipeline,
	}
}

// GenerateTask は /tasks/generate へのリクエストを処理するのだ。
func (h *Handler) GenerateTask(w http.ResponseWriter, r *http.Request) {
	// 1. Cloud Tasks からのペイロードをデコード
	var payload domain.GenerateTaskPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Error("Failed to decode task payload", "error", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// 処理開始のログ（Command を追加して、どのワークフローか明確にするのだ）
	slog.Info("Starting worker task execution",
		"command", payload.Command,
		"script_url", payload.ScriptURL,
		"mode", payload.Mode,
		"panel_limit", payload.PanelLimit,
	)

	// 2. パイプラインを実行
	// r.Context() を渡すことで、Cloud Tasks のタイムアウト設定に基づいたキャンセルを伝搬させます。
	if err := h.pipeline.Execute(r.Context(), payload); err != nil {
		slog.Error("Manga generation pipeline failed",
			"command", payload.Command,
			"script_url", payload.ScriptURL,
			"error", err,
		)
		// 5xxを返すと Cloud Tasks が指数バックオフで自動リトライしてくれるのだ！
		// ただし、致命的なエラーでリトライ不要な場合は設計に合わせて調整が必要なのだ。
		http.Error(w, "Internal Server Error during generation", http.StatusInternalServerError)
		return
	}

	// 3. 完了時のログ
	slog.Info("Worker task completed successfully",
		"command", payload.Command,
		"script_url", payload.ScriptURL,
	)

	// 成功を Cloud Tasks に知らせる（200 OK を返せばジョブ完了とみなされるのだ）
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
