package pipeline

import (
	"context"
	"time"

	"ap-manga-web/internal/builder"
	"ap-manga-web/internal/domain"
)

// MangaPipeline はパイプラインの実行に必要な外部依存関係を保持するサービス構造体です。
type MangaPipeline struct {
	appCtx *builder.AppContext
}

// NewMangaPipeline は MangaPipeline の新しいインスタンスを生成します。
func NewMangaPipeline(appCtx *builder.AppContext) *MangaPipeline {
	return &MangaPipeline{appCtx: appCtx}
}

// Execute は名前付き戻り値 `err` を使用し、リクエストごとに独立した実行コンテキストを生成して処理を開始します。
// defer文により、実行中に発生したエラーの補足と後処理（通知など）を確実に行います。
func (p *MangaPipeline) Execute(ctx context.Context, payload domain.GenerateTaskPayload) (err error) {
	// 実行ごとの状態（開始時刻や生成タイトル等）を管理するコンテキストを生成します。
	// これにより、並行実行時における状態の混線を防ぎます。
	exec := &mangaExecution{
		pipeline:  p,
		payload:   payload,
		startTime: time.Now(),
	}

	return exec.run(ctx)
}
