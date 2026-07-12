package handlers

import (
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/shouni/ap-manga-web/internal/domain"
)

var validTargetPanels = regexp.MustCompile(`^[0-9, ]*$`)

// HandleSubmit タスク生成リクエストのフォーム送信を処理します。
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Warn("フォームの解析に失敗しました", "error", err)
		http.Error(w, "リクエストの解析に失敗しました", http.StatusBadRequest)
		return
	}

	// --- Seed値のパース ---
	seedStr := r.FormValue("seed")
	seed, err := strconv.ParseInt(seedStr, 10, 64)
	if err != nil {
		slog.Info("Seed値が不正または未入力のため、デフォルト値を使用します", "input", seedStr)
		seed = 10000
	}

	targetPanels := r.FormValue("target_panels")
	if !validTargetPanels.MatchString(targetPanels) {
		slog.WarnContext(r.Context(), "target_panels に不正な文字が含まれています", "input", targetPanels)
		http.Error(w, "不正なパネル形式です。数字とカンマのみ使用できます。", http.StatusBadRequest)
		return
	}

	// input_text は Design モードで <select multiple> として送信される場合があり、その場合
	// ブラウザは同名の input_text を複数個送る（FormValue は先頭の1件しか拾えない）ため、
	// r.Form の全件をカンマ区切りに結合する。他モードの単一テキスト入力ではこれは単一要素の
	// スライスになるため、常にこの結合ロジックで問題ない。
	inputText := strings.Join(r.Form["input_text"], ",")

	payload := domain.GenerateTaskPayload{
		Command:      r.FormValue("command"),
		ScriptURL:    r.FormValue("script_url"),
		InputText:    inputText,
		Mode:         r.FormValue("mode"),
		Seed:         seed,
		TargetPanels: targetPanels,
		AspectRatio:  r.FormValue("aspect_ratio"),
		DesignLayout: r.FormValue("design_layout"),
		ReferenceURL: r.FormValue("reference_url"),
		VisualCues:   r.FormValue("visual_cues"),
	}

	if payload.Command == "" {
		http.Error(w, "コマンド（Command）は必須項目です", http.StatusBadRequest)
		return
	}

	if err := h.taskEnqueuer.Enqueue(r.Context(), payload); err != nil {
		slog.Error("タスクのエンキューに失敗しました", "error", err)
		http.Error(w, "タスクのスケジュールに失敗しました。管理者にお問い合わせください。", http.StatusInternalServerError)
		return
	}

	h.render(w, r, http.StatusAccepted, "accepted.html", "タスク受付完了", payload)
}
