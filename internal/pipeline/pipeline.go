package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/shouni/go-remote-io/pkg/remoteio"

	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

// MangaPipeline はパイプラインの実行に必要な外部依存関係を保持するサービス構造体です。
type MangaPipeline struct {
	config    *config.Config
	workflows domain.WorkFlows
	writer    remoteio.OutputWriter
	notifier  domain.Notifier
}

// NewMangaPipeline は、Container から必要な依存関係のみを抽出して MangaPipeline を生成します。
func NewMangaPipeline(config *config.Config, workflows domain.WorkFlows, writer remoteio.OutputWriter, notifier domain.Notifier) (*MangaPipeline, error) {
	if writer == nil {
		return nil, fmt.Errorf("MangaPipelineの初期化に失敗しました: 成果物の保存に必要な OutputWriter が Container 内に設定されていません")
	}

	if workflows == nil {
		return nil, fmt.Errorf("MangaPipelineの初期化に失敗しました: 漫画生成ワークフロー (WorkflowsAdapter) が初期化されていません")
	}

	if notifier == nil {
		return nil, fmt.Errorf("MangaPipelineの初期化に失敗しました: 通知コンポーネント (Notifier) が設定されていません")
	}

	return &MangaPipeline{
		config:    config,
		workflows: workflows,
		writer:    writer,
		notifier:  notifier,
	}, nil
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
