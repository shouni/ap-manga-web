package handlers

import (
	"log/slog"
	"net/http"
	"regexp"

	"ap-manga-web/internal/domain"
)

var validTargetPanels = regexp.MustCompile(`^[0-9, ]*$`)

// HandleSubmit タスク生成リクエストのフォーム送信を処理します。
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Warn("フォームの解析に失敗しました", "error", err)
		http.Error(w, "リクエストの解析に失敗しました", http.StatusBadRequest)
		return
	}

	targetPanels := r.FormValue("target_panels")
	if !validTargetPanels.MatchString(targetPanels) {
		slog.WarnContext(r.Context(), "target_panels に不正な文字が含まれています", "input", targetPanels)
		http.Error(w, "不正なパネル形式です。数字とカンマのみ使用できます。", http.StatusBadRequest)
		return
	}

	payload := domain.GenerateTaskPayload{
		Command:      r.FormValue("command"),
		ScriptURL:    r.FormValue("script_url"),
		InputText:    r.FormValue("input_text"),
		Mode:         r.FormValue("mode"),
		TargetPanels: targetPanels,
	}

	if payload.Command == "" {
		http.Error(w, "コマンド（Command）は必須項目です", http.StatusBadRequest)
		return
	}

	if err := h.appCtx.TaskEnqueuer.Enqueue(r.Context(), payload); err != nil {
		slog.Error("タスクのエンキューに失敗しました", "error", err)
		http.Error(w, "タスクのスケジュールに失敗しました。管理者にお問い合わせください。", http.StatusInternalServerError)
		return
	}

	h.render(w, http.StatusAccepted, "accepted.html", "タスク受付完了", payload)
}
