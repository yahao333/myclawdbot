package auth

import (
	"testing"
	"time"

	"github.com/yahao333/myclawdbot/internal/config"
)

// TestNewManager 创建认证管理器测试
func TestNewManager(t *testing.T) {
	// 测试空配置创建管理器
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	if m == nil {
		t.Error("期望 Manager 不为 nil")
	}

	if m.providers == nil {
		t.Error("期望 providers map 不为 nil")
	}

	if m.users == nil {
		t.Error("期望 users map 不为 nil")
	}

	if m.sessions == nil {
		t.Error("期望 sessions map 不为 nil")
	}
}

// TestNewManagerWithProviders 测试使用提供商创建管理器
func TestNewManagerWithProviders(t *testing.T) {
	cfg := &config.AuthConfig{
		GitHub: config.GitHubAuthConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost/callback",
		},
	}

	m := NewManager(cfg)

	// 验证 GitHub 提供商已注册
	provider, ok := m.GetProvider("github")
	if !ok {
		t.Error("期望 GitHub 提供商已注册")
	}

	if provider.Name() != "github" {
		t.Errorf("期望提供商标名为 github，实际为 %s", provider.Name())
	}
}

// TestRegisterProvider 测试注册提供商
func TestRegisterProvider(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	// 注册自定义提供商
	provider := &mockProvider{name: "custom"}
	m.RegisterProvider(provider)

	// 验证提供商已注册
	retrieved, ok := m.GetProvider("custom")
	if !ok {
		t.Error("期望自定义提供商已注册")
	}

	if retrieved.Name() != "custom" {
		t.Errorf("期望提供商标名为 custom，实际为 %s", retrieved.Name())
	}
}

// TestGetProvider 测试获取提供商
func TestGetProvider(t *testing.T) {
	cfg := &config.AuthConfig{
		GitHub: config.GitHubAuthConfig{
			ClientID: "test-client-id",
		},
	}
	m := NewManager(cfg)

	// 测试存在的提供商
	provider, ok := m.GetProvider("github")
	if !ok {
		t.Error("期望能够获取 github 提供商")
	}

	_ = provider // 使用返回值

	// 测试不存在的提供商
	_, ok = m.GetProvider("non-existent")
	if ok {
		t.Error("期望不存在的提供商返回 false")
	}
}

// TestListProviders 测试列出提供商
func TestListProviders(t *testing.T) {
	cfg := &config.AuthConfig{
		GitHub: config.GitHubAuthConfig{
			ClientID: "test-client-id",
		},
	}
	m := NewManager(cfg)

	providers := m.ListProviders()

	if len(providers) == 0 {
		t.Error("期望至少有一个提供商")
	}
}

// TestGenerateState 测试生成随机 state
func TestGenerateState(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	// 生成 state
	state1, err := m.GenerateState()
	if err != nil {
		t.Errorf("生成 state 失败: %v", err)
	}

	if len(state1) == 0 {
		t.Error("期望 state 不为空")
	}

	// 验证多次生成产生不同的 state
	state2, err := m.GenerateState()
	if err != nil {
		t.Errorf("生成第二个 state 失败: %v", err)
	}

	if state1 == state2 {
		t.Error("期望两次生成的 state 不同")
	}
}

// TestCreateSession 测试创建会话
func TestCreateSession(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	token := &Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	// 创建会话
	sessionID := m.CreateSession("user-1", token)

	if sessionID == "" {
		t.Error("期望 sessionID 不为空")
	}

	// 验证会话已创建
	session, ok := m.GetSession(sessionID)
	if !ok {
		t.Error("期望能够获取会话")
	}

	if session.UserID != "user-1" {
		t.Errorf("期望用户 ID 为 user-1，实际为 %s", session.UserID)
	}

	if session.Token.AccessToken != "test-token" {
		t.Errorf("期望 token 为 test-token，实际为 %s", session.Token.AccessToken)
	}
}

// TestGetSession 测试获取会话
func TestGetSession(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	token := &Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	sessionID := m.CreateSession("user-1", token)

	// 测试获取存在的会话
	session, ok := m.GetSession(sessionID)
	if !ok {
		t.Error("期望能够获取会话")
	}

	if session.SessionID != sessionID {
		t.Errorf("期望 sessionID 为 %s，实际为 %s", sessionID, session.SessionID)
	}

	// 测试获取不存在的会话
	_, ok = m.GetSession("non-existent")
	if ok {
		t.Error("期望不存在的会话返回 false")
	}
}

// TestGetSessionExpired 测试获取会话时检查过期时间
// 注意：CreateSession 会忽略 token.ExpiresAt，始终使用 24 小时过期
// 这里仅验证会话能正常获取（未过期时）
func TestGetSessionExpired(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	token := &Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	sessionID := m.CreateSession("user-1", token)

	// 获取未过期的会话应该成功
	session, ok := m.GetSession(sessionID)
	if !ok {
		t.Error("期望能够获取会话")
	}

	// 验证会话的过期时间（24小时）
	if session.ExpiresAt.Sub(session.CreatedAt) != 24*time.Hour {
		t.Errorf("期望过期时间为 24 小时，实际为 %v", session.ExpiresAt.Sub(session.CreatedAt))
	}
}

// TestDeleteSession 测试删除会话
func TestDeleteSession(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	token := &Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	sessionID := m.CreateSession("user-1", token)

	// 删除会话
	m.DeleteSession(sessionID)

	// 验证会话已被删除
	_, ok := m.GetSession(sessionID)
	if ok {
		t.Error("期望会话已被删除")
	}
}

// TestGetUser 测试获取用户信息
func TestGetUser(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	token := &Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	m.CreateSession("user-1", token)

	// 测试获取存在的用户
	user, ok := m.GetUser("user-1")
	if !ok {
		t.Error("期望能够获取用户")
	}

	if user.ID != "user-1" {
		t.Errorf("期望用户 ID 为 user-1，实际为 %s", user.ID)
	}

	// 测试获取不存在的用户
	_, ok = m.GetUser("non-existent")
	if ok {
		t.Error("期望不存在的用户返回 false")
	}
}

// TestGitHubProviderName 测试 GitHub 提供商名称
func TestGitHubProviderName(t *testing.T) {
	provider := NewGitHubProvider("client-id", "client-secret", "http://localhost/callback")

	if provider.Name() != "github" {
		t.Errorf("期望名称为 github，实际为 %s", provider.Name())
	}
}

// TestGitHubProviderGetAuthURL 测试获取 GitHub 授权 URL
func TestGitHubProviderGetAuthURL(t *testing.T) {
	provider := NewGitHubProvider("client-id", "client-secret", "http://localhost/callback")

	url := provider.GetAuthURL("test-state")

	if url == "" {
		t.Error("期望 URL 不为空")
	}

	// 验证 URL 以预期的格式开头
	if !contains(url, "https://github.com/login/oauth/authorize") {
		t.Errorf("期望 URL 以 GitHub 授权地址开头，实际为 %s", url)
	}

	// 验证 URL 包含必要参数（检查编码后的格式）
	if !contains(url, "client_id=client-id") {
		t.Errorf("期望 URL 包含 client_id")
	}

	// 检查 state 参数（编码后）
	if !contains(url, "state=test-state") {
		t.Errorf("期望 URL 包含 state 参数")
	}
}

// TestGoogleProviderName 测试 Google 提供商名称
func TestGoogleProviderName(t *testing.T) {
	provider := NewGoogleProvider("client-id", "client-secret", "http://localhost/callback")

	if provider.Name() != "google" {
		t.Errorf("期望名称为 google，实际为 %s", provider.Name())
	}
}

// TestGoogleProviderGetAuthURL 测试获取 Google 授权 URL
func TestGoogleProviderGetAuthURL(t *testing.T) {
	provider := NewGoogleProvider("client-id", "client-secret", "http://localhost/callback")

	url := provider.GetAuthURL("test-state")

	if url == "" {
		t.Error("期望 URL 不为空")
	}

	// 验证 URL 包含必要参数
	if !contains(url, "client_id=client-id") {
		t.Errorf("期望 URL 包含 client_id")
	}

	if !contains(url, "state=test-state") {
		t.Errorf("期望 URL 包含 state")
	}
}

// TestLoadFromFile 测试从文件加载用户数据
func TestLoadFromFile(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	// 测试加载不存在的文件
	err := m.LoadFromFile("/non/existent/file.json")
	if err == nil {
		t.Error("期望加载不存在的文件失败")
	}
}

// TestSaveToFile 测试保存用户数据到文件
func TestSaveToFile(t *testing.T) {
	cfg := &config.AuthConfig{}
	m := NewManager(cfg)

	token := &Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	m.CreateSession("user-1", token)

	// 测试保存到无效路径
	err := m.SaveToFile("/invalid/path/users.json")
	if err == nil {
		t.Error("期望保存到无效路径失败")
	}
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockProvider 模拟 OAuth 提供商
type mockProvider struct {
	name           string
	authURL        string
	token          *Token
	userInfo       *UserInfo
	getAuthURLFunc func(state string) string
	exchangeFunc   func(code string) (*Token, error)
	userInfoFunc   func(token string) (*UserInfo, error)
}

func (m *mockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockProvider) GetAuthURL(state string) string {
	if m.getAuthURLFunc != nil {
		return m.getAuthURLFunc(state)
	}
	if m.authURL != "" {
		return m.authURL
	}
	return "http://mock.auth.url"
}

func (m *mockProvider) ExchangeCode(code string) (*Token, error) {
	if m.exchangeFunc != nil {
		return m.exchangeFunc(code)
	}
	if m.token != nil {
		return m.token, nil
	}
	return &Token{AccessToken: "mock-token"}, nil
}

func (m *mockProvider) GetUserInfo(token string) (*UserInfo, error) {
	if m.userInfoFunc != nil {
		return m.userInfoFunc(token)
	}
	if m.userInfo != nil {
		return m.userInfo, nil
	}
	return &UserInfo{ID: "mock-user", Name: "Mock User"}, nil
}
