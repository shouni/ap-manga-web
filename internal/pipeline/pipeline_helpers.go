package pipeline

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"ap-manga-web/internal/domain"
)

const (
	defaultErrorTitle   = "漫画錬成エラー"
	errorReportCategory = "error-report"
)

// notifyError はエラー発生時に SlackAdapter を通じて通知を行います。
func (p *MangaPipeline) notifyError(ctx context.Context, payload domain.GenerateTaskPayload, opErr error, titleHint string) {
	reqTitle := defaultErrorTitle
	if titleHint != "" {
		reqTitle = titleHint
	}

	req := domain.NotificationRequest{
		SourceURL:      payload.ScriptURL,
		OutputCategory: errorReportCategory,
		TargetTitle:    reqTitle,
		ExecutionMode:  payload.Command,
	}

	if err := p.appCtx.SlackNotifier.NotifyError(ctx, opErr, req); err != nil {
		slog.ErrorContext(ctx, "Failed to send error notification", "error", err)
	}
}

// parseTargetPanels はカンマ区切りの文字列を解析し、有効なパネルインデックスのスライスを返します。
func (p *MangaPipeline) parseTargetPanels(ctx context.Context, s string, total int) []int {
	if strings.TrimSpace(s) == "" {
		res := make([]int, total)
		for i := 0; i < total; i++ {
			res[i] = i
		}
		return res
	}

	var res []int
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		// 数値に変換できない、または範囲外のインデックスは意図的に無視します。
		if idx, err := strconv.Atoi(trimmed); err == nil && idx >= 0 && idx < total {
			res = append(res, idx)
		}
	}
	return res
}

// parseCSV はカンマ区切りの入力文字列をスライスに変換します。
func (p *MangaPipeline) parseCSV(input string) []string {
	var res []string
	for _, s := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}
