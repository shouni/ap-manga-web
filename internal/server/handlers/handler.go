package handlers

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"ap-manga-web/internal/app"
	"ap-manga-web/internal/config"
	"ap-manga-web/internal/domain"

	"github.com/shouni/gcp-kit/tasks"
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
	layoutPath := filepath.Join(cfg.TemplateDir, "layout.html")

	// テンプレートで共通利用する関数群を定義
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("レイアウトテンプレートが見つかりません: %s", layoutPath)
	}

	pagePaths, err := filepath.Glob(filepath.Join(cfg.TemplateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("ページテンプレートの検索に失敗しました: %w", err)
	}

	for _, pagePath := range pagePaths {
		pageName := filepath.Base(pagePath)
		if pageName == "layout.html" {
			continue
		}

		tmpl, err := template.New(pageName).Funcs(funcMap).ParseFiles(layoutPath, pagePath)
		if err != nil {
			return nil, fmt.Errorf("テンプレート %s の解析に失敗しました: %w", pageName, err)
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
