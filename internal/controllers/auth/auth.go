package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

// UserSession キー
const (
	sessionName     = "ap-manga-session"
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
	AllowedDomains []string // 許可するドメインのリスト
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

	store := sessions.NewCookieStore([]byte(cfg.SessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   cfg.IsSecureCookie,
		SameSite: http.SameSiteLaxMode,
	}

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
	state, err := generateState()
	if err != nil {
		slog.Error("State生成失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

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
	queryState := r.URL.Query().Get("state")
	cookieState, err := r.Cookie(stateCookieName)
	if err != nil || cookieState.Value != queryState {
		slog.Warn("CSRF攻撃の可能性があるため拒否しました", "query_state", queryState, "cookie_error", err)
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

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

	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		slog.Error("トークン交換失敗", "error", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

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

	if !h.isAuthorized(userInfo.Email) {
		slog.Warn("未許可ユーザーからのアクセス試行", "email", userInfo.Email)
		http.Error(w, "Unauthorized email address", http.StatusForbidden)
		return
	}

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

// Middleware は「人間」の認証済みセッションをチェックするミドルウェアです
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

// TaskOIDCVerificationMiddleware は「Cloud Tasks」の OIDC トークンを検証するミドルウェアです
// [Blocker] セキュリティ対応：外部からの不正なジョブ投入を防止します
func (h *Handler) TaskOIDCVerificationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			slog.Warn("認証ヘッダーが欠落しています")
			http.Error(w, "Unauthorized: OIDC token required", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Googleの公開鍵を使用してIDトークンを検証
		// Audienceは通常、呼び出し先のサービスURLになります
		payload, err := idtoken.Validate(r.Context(), token, "")
		if err != nil {
			slog.Warn("IDトークンの検証に失敗しました", "error", err)
			http.Error(w, fmt.Sprintf("Invalid OIDC token: %v", err), http.StatusForbidden)
			return
		}

		slog.Debug("Cloud Tasks 認証成功", "sub", payload.Subject, "email", payload.Claims["email"])
		next.ServeHTTP(w, r)
	})
}

// isAuthorized はユーザーがアクセス許可を持っているか判定します
func (h *Handler) isAuthorized(email string) bool {
	if len(h.allowedEmails) == 0 && len(h.allowedDomains) == 0 {
		return false
	}
	if _, ok := h.allowedEmails[email]; ok {
		return true
	}
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		domain := parts[1]
		if _, ok := h.allowedDomains[domain]; ok {
			return true
		}
	}
	return false
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("failed to generate state")
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
