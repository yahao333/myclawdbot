// Package auth 认证授权包
//
// 提供 OAuth 2.0 认证支持，包括多种登录方式（GitHub、Google）。
// 支持用户会话管理、API 密钥验证和用户信息获取。
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/internal/config"
)

// OAuthProvider OAuth 提供商接口
//
// 定义 OAuth 2.0 认证提供商的统一接口。
type OAuthProvider interface {
	// Name 返回提供商名称
	Name() string

	// GetAuthURL 获取授权 URL
	//
	// 返回用于用户授权的 URL。
	// 参数：
	//   - state: 防止 CSRF 攻击的状态码
	//
	// 返回：
	//   - string: 授权页面 URL
	GetAuthURL(state string) string

	// ExchangeCode 交换授权码获取 token
	//
	// 将用户授权后获得的授权码交换为访问令牌。
	// 参数：
	//   - code: 授权码
	//
	// 返回：
	//   - *Token: 访问令牌
	//   - error: 交换失败时返回错误
	ExchangeCode(code string) (*Token, error)

	// GetUserInfo 获取用户信息
	//
	// 使用访问令牌获取用户信息。
	// 参数：
	//   - token: 访问令牌
	//
	// 返回：
	//   - *UserInfo: 用户信息
	//   - error: 获取失败时返回错误
	GetUserInfo(token string) (*UserInfo, error)
}

// Token OAuth 令牌
//
// 包含 OAuth 2.0 访问令牌和刷新令牌信息。
type Token struct {
	AccessToken  string    `json:"access_token"`  // 访问令牌
	RefreshToken string    `json:"refresh_token"` // 刷新令牌
	ExpiresAt    time.Time `json:"expires_at"`    // 过期时间
	TokenType    string    `json:"token_type"`    // 令牌类型（通常是 Bearer）
	Raw          map[string]interface{} // 原始响应数据
}

// UserInfo 用户信息
//
// 从 OAuth 提供商获取的用户基本信息。
type UserInfo struct {
	ID        string `json:"id"`         // 用户唯一标识
	Email     string `json:"email"`     // 用户邮箱
	Name      string `json:"name"`      // 用户名称
	AvatarURL string `json:"avatar_url"` // 头像 URL
	Provider  string `json:"provider"`  // OAuth 提供商
}

// Manager 认证管理器
//
// 管理 OAuth 提供商、用户会话和用户信息。
type Manager struct {
	providers   map[string]OAuthProvider // OAuth 提供商映射
	users      map[string]*UserInfo    // userID -> UserInfo
	sessions   map[string]*UserSession   // sessionID -> session
	mu         sync.RWMutex
	config     *config.AuthConfig
}

// UserSession 用户会话
//
// 表示一个已认证用户的会话信息。
type UserSession struct {
	UserID    string    // 用户 ID
	SessionID string    // 会话 ID
	Token     *Token   // OAuth 令牌
	CreatedAt time.Time // 创建时间
	ExpiresAt time.Time // 过期时间
}

// NewManager 创建认证管理器
//
// 使用给定的配置创建认证管理器，并自动注册配置的 OAuth 提供商。
//
// 参数：
//   - cfg: 认证配置
//
// 返回：
//   - *Manager: 创建的认证管理器
func NewManager(cfg *config.AuthConfig) *Manager {
	m := &Manager{
		providers: make(map[string]OAuthProvider),
		users:    make(map[string]*UserInfo),
		sessions: make(map[string]*UserSession),
		config:   cfg,
	}

	// 注册默认提供商
	if cfg.GitHub.ClientID != "" {
		m.RegisterProvider(NewGitHubProvider(cfg.GitHub.ClientID, cfg.GitHub.ClientSecret, cfg.GitHub.RedirectURL))
	}
	if cfg.Google.ClientID != "" {
		m.RegisterProvider(NewGoogleProvider(cfg.Google.ClientID, cfg.Google.ClientSecret, cfg.Google.RedirectURL))
	}

	return m
}

// RegisterProvider 注册 OAuth 提供商
func (m *Manager) RegisterProvider(provider OAuthProvider) {
	m.providers[provider.Name()] = provider
}

// GetProvider 获取 OAuth 提供商
func (m *Manager) GetProvider(name string) (OAuthProvider, bool) {
	p, ok := m.providers[name]
	return p, ok
}

// ListProviders 列出支持的 OAuth 提供商
func (m *Manager) ListProviders() []string {
	providers := make([]string, 0, len(m.providers))
	for name := range m.providers {
		providers = append(providers, name)
	}
	return providers
}

// GenerateState 生成随机 state
//
// 生成用于防止 CSRF 攻击的随机状态码。
func (m *Manager) GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CreateSession 创建用户会话
func (m *Manager) CreateSession(userID string, token *Token) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID, _ := m.GenerateState()
	now := time.Now()

	session := &UserSession{
		UserID:    userID,
		SessionID: sessionID,
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour), // 默认 24 小时
	}

	m.sessions[sessionID] = session

	// 保存用户信息
	m.users[userID] = &UserInfo{ID: userID, Provider: "oauth"}

	return sessionID
}

// GetSession 获取会话
func (m *Manager) GetSession(sessionID string) (*UserSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// DeleteSession 删除会话
func (m *Manager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// GetUser 获取用户信息
func (m *Manager) GetUser(userID string) (*UserInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[userID]
	return user, ok
}

// GitHubProvider GitHub OAuth 提供商
//
// 实现 OAuthProvider 接口，提供 GitHub OAuth 2.0 认证支持。
type GitHubProvider struct {
	clientID     string // GitHub 应用客户端 ID
	clientSecret string // GitHub 应用客户端密钥
	redirectURL  string // 授权回调 URL
}

// NewGitHubProvider 创建 GitHub OAuth 提供商
func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
	}
}

// Name 返回提供商名称
func (p *GitHubProvider) Name() string {
	return "github"
}

// GetAuthURL 获取授权 URL
func (p *GitHubProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Add("client_id", p.clientID)
	params.Add("redirect_uri", p.redirectURL)
	params.Add("scope", "read:user user:email")
	params.Add("state", state)
	return "https://github.com/login/oauth/authorize?" + params.Encode()
}

// ExchangeCode 交换授权码
func (p *GitHubProvider) ExchangeCode(code string) (*Token, error) {
	data := url.Values{}
	data.Add("client_id", p.clientID)
	data.Add("client_secret", p.clientSecret)
	data.Add("code", code)
	data.Add("redirect_uri", p.redirectURL)

	resp, err := http.PostForm("https://github.com/login/oauth/access_token", data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("no access token received")
	}

	return &Token{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		ExpiresAt:   time.Now().Add(time.Hour),
	}, nil
}

// GetUserInfo 获取用户信息
func (p *GitHubProvider) GetUserInfo(accessToken string) (*UserInfo, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, err
	}

	email := ghUser.Email
	if email == "" {
		// 获取主要邮箱
		email, _ = p.getPrimaryEmail(accessToken)
	}

	return &UserInfo{
		ID:        fmt.Sprintf("github:%d", ghUser.ID),
		Email:     email,
		Name:     ghUser.Name,
		AvatarURL: ghUser.AvatarURL,
		Provider:  "github",
	}, nil
}

func (p *GitHubProvider) getPrimaryEmail(accessToken string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	return "", nil
}

// GoogleProvider Google OAuth 提供商
//
// 实现 OAuthProvider 接口，提供 Google OAuth 2.0 认证支持。
type GoogleProvider struct {
	clientID     string // Google 应用客户端 ID
	clientSecret string // Google 应用客户端密钥
	redirectURL  string // 授权回调 URL
}

// NewGoogleProvider 创建 Google OAuth 提供商
func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
	}
}

// Name 返回提供商名称
func (p *GoogleProvider) Name() string {
	return "google"
}

// GetAuthURL 获取授权 URL
func (p *GoogleProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Add("client_id", p.clientID)
	params.Add("redirect_uri", p.redirectURL)
	params.Add("scope", strings.Join([]string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}, " "))
	params.Add("response_type", "code")
	params.Add("state", state)
	params.Add("access_type", "offline")
	params.Add("prompt", "consent")
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// ExchangeCode 交换授权码
func (p *GoogleProvider) ExchangeCode(code string) (*Token, error) {
	data := url.Values{}
	data.Add("client_id", p.clientID)
	data.Add("client_secret", p.clientSecret)
	data.Add("code", code)
	data.Add("grant_type", "authorization_code")
	data.Add("redirect_uri", p.redirectURL)

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("no access token received: %s", string(body))
	}

	return &Token{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

// GetUserInfo 获取用户信息
func (p *GoogleProvider) GetUserInfo(accessToken string) (*UserInfo, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name         string `json:"name"`
		Picture      string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &UserInfo{
		ID:        "google:" + user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.Picture,
		Provider:  "google",
	}, nil
}

// LoadFromFile 从文件加载用户数据
//
// 从 JSON 文件加载用户信息到管理器中。
func (m *Manager) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var users map[string]*UserInfo
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.users = users

	return nil
}

// SaveToFile 保存用户数据到文件
//
// 将用户信息保存到 JSON 文件中。
func (m *Manager) SaveToFile(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Middleware 返回认证中间件
//
// 返回一个 HTTP 中间件，用于验证请求的会话有效性。
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从 Cookie 或 Header 获取 session
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			if cookie, err := r.Cookie("session_id"); err == nil {
				sessionID = cookie.Value
			}
		}

		if sessionID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		session, ok := m.GetSession(sessionID)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 将用户信息添加到 context
		ctx := context.WithValue(r.Context(), "user_id", session.UserID)
		ctx = context.WithValue(ctx, "token", session.Token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
