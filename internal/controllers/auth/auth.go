package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// UserSession キー
const (
	sessionName     = "git-gemini-session"
	userKey         = "user_email"
	stateCookieName = "oauth_state"
)

// AuthConfig は認証ハンドラーの初期化に必要な設定です
type AuthConfig struct {
	ClientID       string
	ClientSecret   string
	RedirectURL    string
	SessionKey     string
	IsSecureCookie bool     // 開発環境(HTTP)と本番(HTTPS)で切り替え
	AllowedEmails  []string // 許可するメールアドレスのリスト
	AllowedDomains []string // 許可するドメインのリスト (例: example.com)
}

// Handler は認証に関連するHTTPハンドラーです
type Handler struct {
	oauthConfig    *oauth2.Config
	store          *sessions.CookieStore
	isSecureCookie bool
	allowedEmails  map[string]struct{}
	allowedDomains map[string]struct{}
}

// NewHandler は新しいAuthHandlerを作成します
func NewHandler(cfg AuthConfig) *Handler {
	// Google OAuth2設定
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	// セッションストア設定
	store := sessions.NewCookieStore([]byte(cfg.SessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   cfg.IsSecureCookie,
		SameSite: http.SameSiteLaxMode,
	}

	// 認可リストの初期化 (高速検索用マップ)
	emailMap := make(map[string]struct{})
	for _, e := range cfg.AllowedEmails {
		if e != "" {
			emailMap[e] = struct{}{}
		}
	}
	domainMap := make(map[string]struct{})
	for _, d := range cfg.AllowedDomains {
		if d != "" {
			domainMap[d] = struct{}{}
		}
	}

	return &Handler{
		oauthConfig:    oauthCfg,
		store:          store,
		isSecureCookie: cfg.IsSecureCookie,
		allowedEmails:  emailMap,
		allowedDomains: domainMap,
	}
}

// Login はGoogleのログイン画面へリダイレクトします
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	// CSRF対策: ランダムなStateを生成
	state, err := generateState()
	if err != nil {
		slog.Error("State生成失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Stateを一時的なCookieに保存 (検証用)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		MaxAge:   600,
		HttpOnly: true,
		Secure:   h.isSecureCookie,
		Path:     "/auth/callback",
	})

	url := h.oauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Callback はGoogleからのコールバックを処理します
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	// 1. State検証 (CSRF対策)
	queryState := r.URL.Query().Get("state")
	cookieState, err := r.Cookie(stateCookieName)
	if err != nil || cookieState.Value != queryState {
		slog.Warn("CSRF攻撃の可能性があるため拒否しました", "query_state", queryState, "cookie_error", err)
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// 使用済みState Cookieを削除
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.isSecureCookie,
		Path:     "/auth/callback",
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	// 2. トークン交換
	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		slog.Error("トークン交換失敗", "error", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	// 3. ユーザー情報取得
	client := h.oauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		slog.Error("ユーザー情報取得失敗", "error", err)
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		slog.Error("JSONデコード失敗", "error", err)
		http.Error(w, "Failed to decode user info", http.StatusInternalServerError)
		return
	}

	// 4. 認可チェック (Allowed List)
	if !h.isAuthorized(userInfo.Email) {
		slog.Warn("未許可ユーザーからのアクセス試行", "email", userInfo.Email)
		http.Error(w, "Unauthorized: Your email is not permitted to access this application.", http.StatusForbidden)
		return
	}

	// 5. セッションに保存
	session, _ := h.store.Get(r, sessionName)
	session.Values[userKey] = userInfo.Email
	if err := session.Save(r, w); err != nil {
		slog.Error("セッション保存失敗", "error", err)
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	slog.Info("ログイン成功", "email", userInfo.Email)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Middleware は認証済みユーザーのみを通すミドルウェアです
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := h.store.Get(r, sessionName)
		if authVal, ok := session.Values[userKey].(string); !ok || authVal == "" {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isAuthorized はユーザーがアクセス許可を持っているか判定します
func (h *Handler) isAuthorized(email string) bool {
	// 認可リストが空の場合、アプリケーションの運用上は起動時にチェックされるべきですが、
	// ここでは最後の防衛線として、デフォルトで拒否するセキュリティ原則を維持します。
	if len(h.allowedEmails) == 0 && len(h.allowedDomains) == 0 {
		slog.Error("致命的な設定エラー: 認可リスト (allowedEmails/allowedDomains) が空です。すべてのアクセスを拒否します。", "email", email)
		return false
	}

	// 1. メールアドレスの直接一致チェック
	if _, ok := h.allowedEmails[email]; ok {
		return true
	}

	// 2. ドメインチェック
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		domain := parts[1]
		if _, ok := h.allowedDomains[domain]; ok {
			return true
		}
	}

	return false
}

// generateState は暗号論的に安全なランダム文字列を生成します
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("failed to generate state")
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
