package handlers

import (
	"fmt"
	"html/template"
	"io/fs"

	"github.com/shouni/gcp-kit/tasks"

	"ap-manga-web/assets"
	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"
)

const titleSuffix = " - AP Manga Web"

type Handler struct {
	cfg           *config.Config
	templateCache map[string]*template.Template
	taskEnqueuer  *tasks.Enqueuer[domain.GenerateTaskPayload]
	remoteIO      *app.RemoteIO
}

// NewHandler は指定された構成に基づいて新しいハンドラーを初期化します。
// テンプレートをコンパイルし、レイアウトファイルが存在することを確認します。
func NewHandler(
	cfg *config.Config,
	taskEnqueuer *tasks.Enqueuer[domain.GenerateTaskPayload],
	remoteIO *app.RemoteIO,
) (*Handler, error) {
	cache := make(map[string]*template.Template)

	// 共通関数
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	// assets.Templates 内の "templates" ディレクトリを走査
	// embed.FS はパス区切りに "/" を使うため filepath ではなく path か直書きが安全です
	entries, err := fs.ReadDir(assets.Templates, "templates")
	if err != nil {
		return nil, fmt.Errorf("テンプレートディレクトリの読み込み失敗: %w", err)
	}

	// レイアウトファイルの存在確認（埋め込みFS内）
	layoutPath := "templates/layout.html"

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "layout.html" {
			continue
		}

		pageName := entry.Name()
		pagePath := "templates/" + pageName

		// ParseFS を使い、埋め込まれたファイルからパース
		tmpl, err := template.New(pageName).
			Funcs(funcMap).
			ParseFS(assets.Templates, layoutPath, pagePath)

		if err != nil {
			return nil, fmt.Errorf("テンプレート %s の解析失敗: %w", pageName, err)
		}
		cache[pageName] = tmpl
	}

	return &Handler{
		cfg:           cfg,
		templateCache: cache,
		taskEnqueuer:  taskEnqueuer,
		remoteIO:      remoteIO,
	}, nil
}
