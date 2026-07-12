package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"

	characterkit "github.com/shouni/go-character-kit/character"

	"github.com/shouni/ap-manga-web/assets"
)

// ErrInvalidPath は、リクエストされたパスがサニタイズ検証に失敗したことを示します。
var ErrInvalidPath = errors.New("invalid path provided")

// Index は、漫画生成フォームのトップページを表示します。
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "index.html", "Generate", nil)
}

// designPageData は design.html に渡す表示データです。CharactersJSON は、キャラクターを
// 選択した際にSeed/reference_url/visual_cuesをJSでフォームへ自動入力するための埋め込みJSON
// です（go-character-kit のキャラクター定義はアプリ内で完結したデータで外部入力を含まない
// ため、template.JS としてエスケープなしで埋め込んでも安全です）。
type designPageData struct {
	Characters     []characterkit.Character
	CharactersJSON template.JS
}

// Design は、キャラクターデザイン生成フォームを表示します。キャラクターIDを選択すると、
// 既存の Seed / reference_url / visual_cues がフォームに自動入力され、その場限りの上書きが
// できます（characters.json 自体は変更しません）。
func (h *Handler) Design(w http.ResponseWriter, r *http.Request) {
	chars, err := assets.LoadCharacters()
	if err != nil {
		slog.Error("キャラクター定義の読み込みに失敗しました", "error", err)
		http.Error(w, "システムエラーが発生しました（キャラクター定義読み込み失敗）", http.StatusInternalServerError)
		return
	}
	charsJSON, err := json.Marshal(chars.List)
	if err != nil {
		slog.Error("キャラクター定義のJSON変換に失敗しました", "error", err)
		http.Error(w, "システムエラーが発生しました", http.StatusInternalServerError)
		return
	}
	h.render(w, r, http.StatusOK, "design.html", "Character Design", designPageData{
		Characters:     chars.List,
		CharactersJSON: template.JS(charsJSON), //nolint:gosec // 埋め込みデータのみで外部入力なし
	})
}

// Script は、漫画スクリプト生成フォームを表示します。
func (h *Handler) Script(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "script.html", "Script Generation", nil)
}

// Panel は、パネル画像生成フォームを表示します。
func (h *Handler) Panel(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "panel.html", "Panel Generation", nil)
}

// Page は、ページレイアウト生成フォームを表示します。
func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "page.html", "Page Layout", nil)
}
