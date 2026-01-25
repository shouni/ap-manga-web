package builder

import (
	"context"
	"fmt"
	"net/url"

	"ap-manga-web/internal/domain"
	"ap-manga-web/internal/server/handlers"

	"github.com/shouni/gcp-kit/auth"
	"github.com/shouni/gcp-kit/worker"
)

const defaultSessionName = "ap-manga-session"

// AppHandlers は生成されたすべての HTTP ハンドラーを保持する構造体です。
// server パッケージはこの構造体を受け取ってルーティングを行います。
type AppHandlers struct {
	Auth   *auth.Handler
	Web    *handlers.Handler
	Worker *worker.Handler[domain.GenerateTaskPayload]
}

// TaskExecutor は、非同期タスクのペイロードを受け取り、
// 対応するビジネスロジックを実行する責務を抽象化します。
type TaskExecutor interface {
	Execute(ctx context.Context, payload domain.GenerateTaskPayload) error
}

// BuildHandlers は各ハンドラーの依存関係をすべて組み立て、AppHandlers 構造体を返します。
func BuildHandlers(
	appCtx *AppContext,
	executor TaskExecutor,
) (*AppHandlers, error) {
	if appCtx.Config.ServiceURL == "" {
		return nil, fmt.Errorf("認証リダイレクトのために ServiceURL の設定が必要です")
	}

	// 1. 認証Handlerの初期化
	authHandler, err := createAuthHandler(appCtx)
	if err != nil {
		return nil, fmt.Errorf("認証Handlerの初期化に失敗しました: %w", err)
	}

	// 2. Web UI 用Handlerの初期化
	webHandler, err := handlers.NewHandler(appCtx.Config, appCtx.TaskEnqueuer, appCtx.Reader, appCtx.Signer)
	if err != nil {
		return nil, fmt.Errorf("WebHandlerの初期化に失敗しました: %w", err)
	}

	// 3. 非同期ワーカー用Handlerの初期化
	workerHandler := worker.NewHandler[domain.GenerateTaskPayload](executor)

	return &AppHandlers{
		Auth:   authHandler,
		Web:    webHandler,
		Worker: workerHandler,
	}, nil
}

// createAuthHandler は AppContext から認証ライブラリ用の設定を構築し、ハンドラーを生成します。
func createAuthHandler(appCtx *AppContext) (*auth.Handler, error) {
	cfg := appCtx.Config
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("リダイレクトURLの構築に失敗しました: %w", err)
	}

	// HttpClient の判定メソッドを使用して Secure 属性を決定
	isSecure := appCtx.HTTPClient.IsSecureServiceURL(cfg.ServiceURL)

	return auth.NewHandler(auth.Config{
		ClientID:          cfg.GoogleClientID,
		ClientSecret:      cfg.GoogleClientSecret,
		RedirectURL:       redirectURL,
		SessionAuthKey:    cfg.SessionSecret,
		SessionEncryptKey: cfg.SessionEncryptKey,
		SessionName:       defaultSessionName,
		IsSecureCookie:    isSecure,
		AllowedEmails:     cfg.AllowedEmails,
		AllowedDomains:    cfg.AllowedDomains,
		TaskAudienceURL:   cfg.ServiceURL, // 必要に応じて audience 調整
	})
}
