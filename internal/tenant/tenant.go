// Package tenant 多租户支持包
// 提供多租户隔离、用户管理、租户配置等功能
package tenant

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/internal/config"
	"github.com/yahao333/myclawdbot/internal/session"
)

// Tenant 租户
// 代表一个独立的用户或组织，拥有独立的配置和资源
type Tenant struct {
	ID          string            // 租户 ID
	Name        string            // 租户名称
	OwnerID     string            // 所有者 ID
	Config      *config.Config    // 租户配置
	Users       map[string]*User  // 用户列表
	SessionMgr  *session.Manager  // 会话管理器
	CreatedAt   time.Time         // 创建时间
	UpdatedAt   time.Time         // 更新时间
	mu          sync.RWMutex
}

// User 用户
// 属于租户的用户的身份
type User struct {
	ID        string   // 用户 ID
	TenantID  string   // 租户 ID
	Name      string   // 用户名
	Email     string   // 邮箱
	Role      string   // 角色: admin, member
	CreatedAt time.Time
}

// Manager 租户管理器
// 管理所有租户和全局配置
type Manager struct {
	tenants    map[string]*Tenant  // tenantID -> Tenant
	users      map[string]*User   // userID -> User (全局索引)
	defaultCfg *config.Config     // 默认配置
	mu         sync.RWMutex
}

// NewManager 创建租户管理器
func NewManager(defaultCfg *config.Config) *Manager {
	return &Manager{
		tenants:    make(map[string]*Tenant),
		users:      make(map[string]*User),
		defaultCfg: defaultCfg,
	}
}

// CreateTenant 创建租户
func (m *Manager) CreateTenant(id, name, ownerID string) (*Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tenants[id]; exists {
		return nil, fmt.Errorf("tenant %s already exists", id)
	}

	// 创建租户配置（继承默认配置）
	cfg := *m.defaultCfg

	tenant := &Tenant{
		ID:         id,
		Name:       name,
		OwnerID:    ownerID,
		Config:     &cfg,
		Users:      make(map[string]*User),
		SessionMgr: session.NewManager(cfg.Session.MaxHistory, nil),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.tenants[id] = tenant

	return tenant, nil
}

// GetTenant 获取租户
func (m *Manager) GetTenant(id string) (*Tenant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant, ok := m.tenants[id]
	return tenant, ok
}

// ListTenants 列出所有租户
func (m *Manager) ListTenants() []*Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenants := make([]*Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		tenants = append(tenants, t)
	}
	return tenants
}

// DeleteTenant 删除租户
func (m *Manager) DeleteTenant(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tenants[id]; !exists {
		return fmt.Errorf("tenant %s not found", id)
	}

	delete(m.tenants, id)
	return nil
}

// AddUser 添加用户到租户
func (t *Tenant) AddUser(userID, name, email, role string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.Users[userID]; exists {
		return fmt.Errorf("user %s already exists in tenant %s", userID, t.ID)
	}

	user := &User{
		ID:        userID,
		TenantID:  t.ID,
		Name:      name,
		Email:     email,
		Role:      role,
		CreatedAt: time.Now(),
	}

	t.Users[userID] = user
	t.UpdatedAt = time.Now()

	return nil
}

// RemoveUser 从租户移除用户
func (t *Tenant) RemoveUser(userID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.Users[userID]; !exists {
		return fmt.Errorf("user %s not found in tenant %s", userID, t.ID)
	}

	delete(t.Users, userID)
	t.UpdatedAt = time.Now()

	return nil
}

// GetUser 获取用户
func (t *Tenant) GetUser(userID string) (*User, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	user, ok := t.Users[userID]
	return user, ok
}

// UpdateConfig 更新租户配置
func (t *Tenant) UpdateConfig(cfg *config.Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Config = cfg
	t.UpdatedAt = time.Now()

	return nil
}

// GetSessionManager 获取租户的会话管理器
func (t *Tenant) GetSessionManager() *session.Manager {
	return t.SessionMgr
}

// Middleware 多租户中间件
// 用于从请求中提取租户信息
type Middleware struct {
	manager *Manager
}

// NewMiddleware 创建租户中间件
func NewMiddleware(manager *Manager) *Middleware {
	return &Middleware{manager: manager}
}

// Handler 返回 HTTP 中间件处理函数
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从 Header 或参数获取租户 ID
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = r.URL.Query().Get("tenant_id")
		}

		// 从 Header 或参数获取用户 ID
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			userID = r.URL.Query().Get("user_id")
		}

		// 如果没有租户 ID，使用默认租户
		if tenantID == "" {
			tenantID = "default"
		}

		// 获取租户
		tenant, ok := m.manager.GetTenant(tenantID)
		if !ok {
			http.Error(w, "Tenant not found", http.StatusNotFound)
			return
		}

		// 将租户和用户信息添加到 context
		ctx := context.WithValue(r.Context(), "tenant", tenant)
		if userID != "" {
			ctx = context.WithValue(ctx, "user_id", userID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetTenantFromContext 从 context 获取租户
func GetTenantFromContext(ctx context.Context) (*Tenant, bool) {
	tenant, ok := ctx.Value("tenant").(*Tenant)
	return tenant, ok
}

// GetUserIDFromContext 从 context 获取用户 ID
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value("user_id").(string)
	return userID, ok
}

// TenantContext 租户上下文
// 用于在请求处理中传递租户信息
type TenantContext struct {
	Tenant *Tenant
	UserID string
}

// NewTenantContext 创建租户上下文
func NewTenantContext(tenant *Tenant, userID string) *TenantContext {
	return &TenantContext{
		Tenant: tenant,
		UserID: userID,
	}
}
