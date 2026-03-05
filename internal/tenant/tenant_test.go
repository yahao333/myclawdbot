package tenant

import (
	"context"
	"testing"

	"github.com/yahao333/myclawdbot/internal/config"
)

// TestNewManager 创建租户管理器测试
func TestNewManager(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	if m == nil {
		t.Error("期望 Manager 不为 nil")
	}

	if m.tenants == nil {
		t.Error("期望 tenants map 不为 nil")
	}

	if m.users == nil {
		t.Error("期望 users map 不为 nil")
	}
}

// TestCreateTenant 测试创建租户
func TestCreateTenant(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	tenant, err := m.CreateTenant("tenant-1", "测试租户", "owner-1")
	if err != nil {
		t.Errorf("创建租户失败: %v", err)
	}

	if tenant == nil {
		t.Error("期望返回的租户不为 nil")
	}

	if tenant.ID != "tenant-1" {
		t.Errorf("期望租户 ID 为 tenant-1，实际为 %s", tenant.ID)
	}

	if tenant.Name != "测试租户" {
		t.Errorf("期望租户名称为 测试租户，实际为 %s", tenant.Name)
	}

	if tenant.OwnerID != "owner-1" {
		t.Errorf("期望所有者 ID 为 owner-1，实际为 %s", tenant.OwnerID)
	}

	// 验证租户已添加到管理器
	retrieved, ok := m.GetTenant("tenant-1")
	if !ok {
		t.Error("期望能够获取已创建的租户")
	}

	if retrieved != tenant {
		t.Error("期望返回的租户与创建的租户相同")
	}
}

// TestCreateDuplicateTenant 测试创建重复租户
func TestCreateDuplicateTenant(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建第一个租户
	_, err := m.CreateTenant("tenant-1", "测试租户", "owner-1")
	if err != nil {
		t.Errorf("创建第一个租户失败: %v", err)
	}

	// 创建重复 ID 的租户应该失败
	_, err = m.CreateTenant("tenant-1", "另一个租户", "owner-2")
	if err == nil {
		t.Error("期望创建重复租户失败，实际成功")
	}
}

// TestGetTenant 测试获取租户
func TestGetTenant(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 测试获取存在的租户
	tenant, ok := m.GetTenant("tenant-1")
	if !ok {
		t.Error("期望能够获取租户")
	}

	if tenant.ID != "tenant-1" {
		t.Errorf("期望租户 ID 为 tenant-1，实际为 %s", tenant.ID)
	}

	// 测试获取不存在的租户
	_, ok = m.GetTenant("non-existent")
	if ok {
		t.Error("期望不存在的租户返回 false")
	}
}

// TestListTenants 测试列出所有租户
func TestListTenants(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建多个租户
	m.CreateTenant("tenant-1", "租户1", "owner-1")
	m.CreateTenant("tenant-2", "租户2", "owner-2")

	tenants := m.ListTenants()

	if len(tenants) != 2 {
		t.Errorf("期望 2 个租户，实际为 %d", len(tenants))
	}
}

// TestDeleteTenant 测试删除租户
func TestDeleteTenant(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 删除租户
	err := m.DeleteTenant("tenant-1")
	if err != nil {
		t.Errorf("删除租户失败: %v", err)
	}

	// 验证租户已被删除
	_, ok := m.GetTenant("tenant-1")
	if ok {
		t.Error("期望租户已被删除")
	}
}

// TestDeleteNonExistentTenant 测试删除不存在的租户
func TestDeleteNonExistentTenant(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 删除不存在的租户应该失败
	err := m.DeleteTenant("non-existent")
	if err == nil {
		t.Error("期望删除不存在的租户失败，实际成功")
	}
}

// TestTenantAddUser 测试添加用户到租户
func TestTenantAddUser(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	tenant, _ := m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 添加用户
	err := tenant.AddUser("user-1", "张三", "zhangsan@example.com", "admin")
	if err != nil {
		t.Errorf("添加用户失败: %v", err)
	}

	// 验证用户已添加
	user, ok := tenant.GetUser("user-1")
	if !ok {
		t.Error("期望能够获取已添加的用户")
	}

	if user.Name != "张三" {
		t.Errorf("期望用户名为 张三，实际为 %s", user.Name)
	}

	if user.Email != "zhangsan@example.com" {
		t.Errorf("期望邮箱为 zhangsan@example.com，实际为 %s", user.Email)
	}

	if user.Role != "admin" {
		t.Errorf("期望角色为 admin，实际为 %s", user.Role)
	}
}

// TestTenantAddDuplicateUser 测试添加重复用户
func TestTenantAddDuplicateUser(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	tenant, _ := m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 添加第一个用户
	tenant.AddUser("user-1", "张三", "zhangsan@example.com", "admin")

	// 添加重复用户应该失败
	err := tenant.AddUser("user-1", "李四", "lisi@example.com", "member")
	if err == nil {
		t.Error("期望添加重复用户失败，实际成功")
	}
}

// TestTenantRemoveUser 测试从租户移除用户
func TestTenantRemoveUser(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	tenant, _ := m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 添加用户
	tenant.AddUser("user-1", "张三", "zhangsan@example.com", "admin")

	// 移除用户
	err := tenant.RemoveUser("user-1")
	if err != nil {
		t.Errorf("移除用户失败: %v", err)
	}

	// 验证用户已被移除
	_, ok := tenant.GetUser("user-1")
	if ok {
		t.Error("期望用户已被移除")
	}
}

// TestTenantRemoveNonExistentUser 测试移除不存在的用户
func TestTenantRemoveNonExistentUser(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	tenant, _ := m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 移除不存在的用户应该失败
	err := tenant.RemoveUser("non-existent")
	if err == nil {
		t.Error("期望移除不存在的用户失败，实际成功")
	}
}

// TestTenantGetUser 测试获取租户中的用户
func TestTenantGetUser(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	tenant, _ := m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 添加用户
	tenant.AddUser("user-1", "张三", "zhangsan@example.com", "admin")

	// 测试获取存在的用户
	user, ok := tenant.GetUser("user-1")
	if !ok {
		t.Error("期望能够获取用户")
	}

	if user.ID != "user-1" {
		t.Errorf("期望用户 ID 为 user-1，实际为 %s", user.ID)
	}

	// 测试获取不存在的用户
	_, ok = tenant.GetUser("non-existent")
	if ok {
		t.Error("期望不存在的用户返回 false")
	}
}

// TestTenantUpdateConfig 测试更新租户配置
func TestTenantUpdateConfig(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	tenant, _ := m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 更新配置
	newCfg := &config.Config{
		LLM: config.LLMConfig{
			Model: "gpt-4",
		},
	}

	err := tenant.UpdateConfig(newCfg)
	if err != nil {
		t.Errorf("更新配置失败: %v", err)
	}

	// 验证配置已更新
	if tenant.Config.LLM.Model != "gpt-4" {
		t.Errorf("期望 LLM 模型为 gpt-4，实际为 %s", tenant.Config.LLM.Model)
	}
}

// TestTenantGetSessionManager 测试获取租户的会话管理器
func TestTenantGetSessionManager(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建租户
	tenant, _ := m.CreateTenant("tenant-1", "测试租户", "owner-1")

	// 获取会话管理器
	sessMgr := tenant.GetSessionManager()

	if sessMgr == nil {
		t.Error("期望会话管理器不为 nil")
	}
}

// TestNewMiddleware 测试创建租户中间件
func TestNewMiddleware(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// 创建中间件
	middleware := NewMiddleware(m)

	if middleware == nil {
		t.Error("期望中间件不为 nil")
	}

	if middleware.manager != m {
		t.Error("期望 manager 被正确设置")
	}
}

// TestGetTenantFromContext 测试从 context 获取租户
func TestGetTenantFromContext(t *testing.T) {
	// 创建一个租户
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "测试租户",
	}

	// 创建包含租户的 context
	ctx := context.WithValue(context.Background(), "tenant", tenant)
	ctx = context.WithValue(ctx, "user_id", "user-1")

	// 从 context 获取租户
	retrieved, ok := GetTenantFromContext(ctx)
	if !ok {
		t.Error("期望能够从 context 获取租户")
	}

	if retrieved.ID != "tenant-1" {
		t.Errorf("期望租户 ID 为 tenant-1，实际为 %s", retrieved.ID)
	}
}

// TestGetUserIDFromContext 测试从 context 获取用户 ID
func TestGetUserIDFromContext(t *testing.T) {
	// 创建一个租户
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "测试租户",
	}

	// 创建包含用户 ID 的 context
	ctx := context.WithValue(context.Background(), "tenant", tenant)
	ctx = context.WithValue(ctx, "user_id", "user-1")

	// 从 context 获取用户 ID
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		t.Error("期望能够从 context 获取用户 ID")
	}

	if userID != "user-1" {
		t.Errorf("期望用户 ID 为 user-1，实际为 %s", userID)
	}
}

// TestNewTenantContext 测试创建租户上下文
func TestNewTenantContext(t *testing.T) {
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "测试租户",
	}

	// 创建租户上下文
	ctx := NewTenantContext(tenant, "user-1")

	if ctx.Tenant != tenant {
		t.Error("期望 Tenant 被正确设置")
	}

	if ctx.UserID != "user-1" {
		t.Errorf("期望 UserID 为 user-1，实际为 %s", ctx.UserID)
	}
}
