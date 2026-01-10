package builder

import (
	"fmt"
	"net/http"
	"net/url"

	"ap-manga-web/internal/adapters"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/controllers/auth"
	"ap-manga-web/internal/controllers/web"
	"ap-manga-web/internal/controllers/worker"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewServerHandler は HTTP ルーティング、認証、各ハンドラーの依存関係をすべて組み立てるのだ。
func NewServerHandler(
	cfg config.Config,
	taskAdapter adapters.TaskAdapter,
	pipelineExecutor worker.MangaPipelineExecutor, // インターフェース経由で Pipeline を受け取るのだ
) (http.Handler, error) {
	r := chi.NewRouter()

	// 標準的なミドルウェア
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// --- 1. Auth Handler の初期化 ---
	// Google OAuth と Cloud Tasks OIDC 検証の両方を担当するのだ
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("failed to build auth redirect URL: %w", err)
	}

	authHandler := auth.NewHandler(auth.AuthConfig{
		RedirectURL:     redirectURL,
		TaskAudienceURL: cfg.ServiceURL,
		ClientID:        cfg.GoogleClientID,
		ClientSecret:    cfg.GoogleClientSecret,
		SessionKey:      cfg.SessionSecret,
		IsSecureCookie:  config.IsSecureURL(cfg.ServiceURL),
		AllowedEmails:   cfg.AllowedEmails,
		AllowedDomains:  cfg.AllowedDomains,
	})

	// --- 2. Web Handler (UI) の初期化 ---
	webHandler, err := web.NewHandler(cfg, taskAdapter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize web handler: %w", err)
	}

	// --- 3. Worker Handler の初期化 ---
	workerHandler := worker.NewHandler(cfg, pipelineExecutor)

	// --- 4. 公開ルート (Authentication Entry Points) ---
	r.Get("/auth/login", authHandler.Login)
	r.Get("/auth/callback", authHandler.Callback)

	// --- 5. 認証が必要なルート (Web UI 用) ---
	r.Group(func(r chi.Router) {
		r.Use(authHandler.Middleware) // ブラウザセッションをチェックするのだ

		// 一括生成 (メイン)
		r.Get("/", webHandler.Index)

		// [追加] 各 Command ごとの入力テンプレート表示
		// Handler 側にそれぞれ Index と同様のメソッドが必要なのだ
		r.Get("/design", webHandler.Design) // キャラ設計画面
		r.Get("/script", webHandler.Script) // 台本抽出画面
		r.Get("/image", webHandler.Image)   // 画像錬成画面
		r.Get("/story", webHandler.Story)   // プロット構成画面

		// 全ての POST リクエストは共通の HandleSubmit で受け止め、
		// payload.Command で後続の挙動を分岐させるのだ。
		r.Post("/generate", webHandler.HandleSubmit)
	})

	// --- 6. Cloud Tasks 専用ルート (Worker 用) ---
	r.Group(func(r chi.Router) {
		// Cloud Tasks が付与する OIDC Token (Authorization: Bearer) を検証するのだ
		r.Use(authHandler.TaskOIDCVerificationMiddleware)

		// Worker が実際に重い処理 (Pipeline) を実行するエンドポイントなのだ
		r.Post("/tasks/generate", workerHandler.GenerateTask)
	})

	// --- 7. 静的ファイル（もしあれば） ---
	// GCS上の成果物を表示したり、CSSを読み込んだりする設定をここに追加できるのだ

	return r, nil
}
