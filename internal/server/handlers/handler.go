package handlers

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"ap-manga-web/internal/app"
)

const titleSuffix = " - AP Manga Web"

type Handler struct {
	appCtx *app.AppContext
	//cfg           *config.Config
	templateCache map[string]*template.Template
	//taskEnqueuer  *tasks.Enqueuer[domain.GenerateTaskPayload]
	//reader        remoteio.InputReader
	//signer        remoteio.URLSigner
}

// NewHandler は指定された構成に基づいて新しいハンドラーを初期化します。
// テンプレートをコンパイルし、レイアウトファイルが存在することを確認します。
func NewHandler(
	appCtx *app.AppContext,
	//	cfg *config.Config,
	//	taskEnqueuer *tasks.Enqueuer[domain.GenerateTaskPayload],
	//
	// reader remoteio.InputReader,
	// signer remoteio.URLSigner,
) (*Handler, error) {
	cache := make(map[string]*template.Template)
	layoutPath := filepath.Join(appCtx.Config.TemplateDir, "layout.html")
	if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("レイアウトテンプレートが見つかりません: %s", layoutPath)
	}

	pagePaths, err := filepath.Glob(filepath.Join(appCtx.Config.TemplateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("ページテンプレートの検索に失敗しました: %w", err)
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	for _, pagePath := range pagePaths {
		pageName := filepath.Base(pagePath)
		if pageName == "layout.html" {
			continue
		}

		tmpl := template.New(pageName).Funcs(funcMap)
		tmpl, err = tmpl.ParseFiles(layoutPath, pagePath)
		if err != nil {
			return nil, fmt.Errorf("テンプレート %s の解析に失敗しました: %w", pageName, err)
		}
		cache[pageName] = tmpl
	}

	return &Handler{
		appCtx: appCtx,
	}, nil
}
