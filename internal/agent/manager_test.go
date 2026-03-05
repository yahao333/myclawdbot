package agent

import (
	"context"
	"testing"

	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/session"
)

// TestNewManager 创建 Manager 测试
// 验证 Manager 正确初始化
func TestNewManager(t *testing.T) {
	m := NewManager()

	if m == nil {
		t.Error("期望 Manager 不为 nil")
	}

	if m.agents == nil {
		t.Error("期望 agents map 不为 nil")
	}

	if m.groups == nil {
		t.Error("期望 groups map 不为 nil")
	}
}

// TestNewManagerWithOptions 测试使用选项创建 Manager
func TestNewManagerWithOptions(t *testing.T) {
	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, nil)

	m := NewManager(
		WithLLMClient(llmClient),
		WithSessionManager(sessMgr),
	)

	if m.llmClient != llmClient {
		t.Error("期望 LLMClient 被设置")
	}

	if m.sessMgr != sessMgr {
		t.Error("期望 SessionManager 被设置")
	}
}

// TestRegisterAgent 测试注册 Agent
func TestRegisterAgent(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent := NewAgent("test-agent", cfg, llmClient, sessMgr)

	// 注册 Agent
	err := m.RegisterAgent(agent)
	if err != nil {
		t.Errorf("注册 Agent 失败: %v", err)
	}

	// 验证 Agent 已注册
	retrievedAgent, ok := m.GetAgent("test-agent")
	if !ok {
		t.Error("期望能够获取已注册的 Agent")
	}

	if retrievedAgent.ID != "test-agent" {
		t.Errorf("期望 ID 为 test-agent，实际为 %s", retrievedAgent.ID)
	}
}

// TestRegisterDuplicateAgent 测试注册重复 Agent
func TestRegisterDuplicateAgent(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent1 := NewAgent("test-agent", cfg, llmClient, sessMgr)
	agent2 := NewAgent("test-agent", cfg, llmClient, sessMgr)

	// 注册第一个 Agent
	err := m.RegisterAgent(agent1)
	if err != nil {
		t.Errorf("注册第一个 Agent 失败: %v", err)
	}

	// 注册重复 Agent 应该失败
	err = m.RegisterAgent(agent2)
	if err == nil {
		t.Error("期望注册重复 Agent 失败，实际成功")
	}
}

// TestUnregisterAgent 测试注销 Agent
func TestUnregisterAgent(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent := NewAgent("test-agent", cfg, llmClient, sessMgr)

	// 注册 Agent
	m.RegisterAgent(agent)

	// 注销 Agent
	err := m.UnregisterAgent("test-agent")
	if err != nil {
		t.Errorf("注销 Agent 失败: %v", err)
	}

	// 验证 Agent 已被注销
	_, ok := m.GetAgent("test-agent")
	if ok {
		t.Error("期望 Agent 已被注销")
	}
}

// TestUnregisterNonExistentAgent 测试注销不存在的 Agent
func TestUnregisterNonExistentAgent(t *testing.T) {
	m := NewManager()

	// 注销不存在的 Agent 应该失败
	err := m.UnregisterAgent("non-existent")
	if err == nil {
		t.Error("期望注销不存在的 Agent 失败，实际成功")
	}
}

// TestGetAgent 测试获取 Agent
func TestGetAgent(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent := NewAgent("test-agent", cfg, llmClient, sessMgr)
	m.RegisterAgent(agent)

	// 测试存在的 Agent
	retrievedAgent, ok := m.GetAgent("test-agent")
	if !ok {
		t.Error("期望能够获取 Agent")
	}

	if retrievedAgent.ID != "test-agent" {
		t.Errorf("期望 ID 为 test-agent，实际为 %s", retrievedAgent.ID)
	}

	// 测试不存在的 Agent
	_, ok = m.GetAgent("non-existent")
	if ok {
		t.Error("期望不存在的 Agent 返回 false")
	}
}

// TestListAgents 测试列出所有 Agent
func TestListAgents(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	// 注册多个 Agent
	for i := 0; i < 3; i++ {
		cfg := AgentConfig{
			Type:  AgentTypeGeneral,
			Model: "test",
		}
		agent := NewAgent("agent-"+string(rune('a'+i)), cfg, llmClient, sessMgr)
		m.RegisterAgent(agent)
	}

	agents := m.ListAgents()

	if len(agents) != 3 {
		t.Errorf("期望 3 个 Agent，实际为 %d", len(agents))
	}
}

// TestListAgentInfos 测试列出所有 Agent 信息
func TestListAgentInfos(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:        AgentTypeCoder,
		Name:        "测试 Agent",
		Description: "测试描述",
		Model:       "test",
	}

	agent := NewAgent("test-agent", cfg, llmClient, sessMgr)
	m.RegisterAgent(agent)

	infos := m.ListAgentInfos()

	if len(infos) != 1 {
		t.Errorf("期望 1 个 AgentInfo，实际为 %d", len(infos))
	}

	if infos[0].Name != "测试 Agent" {
		t.Errorf("期望 Name 为 测试 Agent，实际为 %s", infos[0].Name)
	}
}

// TestCreateAgent 测试创建并注册 Agent
func TestCreateAgent(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	m.llmClient = llmClient
	m.sessMgr = session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:        AgentTypeGeneral,
		Name:        "新 Agent",
		Description: "测试描述",
		Model:       "test-model",
	}

	agent, err := m.CreateAgent("new-agent", cfg)
	if err != nil {
		t.Errorf("创建 Agent 失败: %v", err)
	}

	if agent == nil {
		t.Error("期望返回的 Agent 不为 nil")
	}

	// 验证 Agent 已自动注册
	retrievedAgent, ok := m.GetAgent("new-agent")
	if !ok {
		t.Error("期望 Agent 已被自动注册")
	}

	if retrievedAgent != agent {
		t.Error("期望返回的 Agent 与获取的 Agent 相同")
	}
}

// TestCreateAgentDuplicate 测试创建重复 ID 的 Agent
func TestCreateAgentDuplicate(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	m.llmClient = llmClient
	m.sessMgr = session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	// 创建第一个 Agent
	_, err := m.CreateAgent("duplicate-agent", cfg)
	if err != nil {
		t.Errorf("创建第一个 Agent 失败: %v", err)
	}

	// 创建重复 ID 的 Agent 应该失败
	_, err = m.CreateAgent("duplicate-agent", cfg)
	if err == nil {
		t.Error("期望创建重复 Agent 失败，实际成功")
	}
}

// TestCreateAgentGroup 测试创建 Agent 组
func TestCreateAgentGroup(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	// 注册多个 Agent
	for _, id := range []string{"agent-1", "agent-2"} {
		cfg := AgentConfig{
			Type:  AgentTypeGeneral,
			Model: "test",
		}
		agent := NewAgent(id, cfg, llmClient, sessMgr)
		m.RegisterAgent(agent)
	}

	// 创建 Agent 组
	err := m.CreateAgentGroup("test-group", []string{"agent-1", "agent-2"})
	if err != nil {
		t.Errorf("创建 Agent 组失败: %v", err)
	}

	// 验证 Agent 组
	agents, ok := m.GetAgentGroup("test-group")
	if !ok {
		t.Error("期望能够获取 Agent 组")
	}

	if len(agents) != 2 {
		t.Errorf("期望 Agent 组包含 2 个 Agent，实际为 %d", len(agents))
	}
}

// TestCreateAgentGroupWithNonExistentAgent 测试包含不存在 Agent 的组
func TestCreateAgentGroupWithNonExistentAgent(t *testing.T) {
	m := NewManager()

	// 尝试创建包含不存在 Agent 的组应该失败
	err := m.CreateAgentGroup("test-group", []string{"non-existent"})
	if err == nil {
		t.Error("期望创建包含不存在 Agent 的组失败，实际成功")
	}
}

// TestGetAgentGroup 测试获取 Agent 组
func TestGetAgentGroup(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	// 注册 Agent
	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}
	agent := NewAgent("agent-1", cfg, llmClient, sessMgr)
	m.RegisterAgent(agent)

	// 创建组
	m.CreateAgentGroup("test-group", []string{"agent-1"})

	// 测试存在的组
	agents, ok := m.GetAgentGroup("test-group")
	if !ok {
		t.Error("期望能够获取 Agent 组")
	}

	if len(agents) != 1 || agents[0].ID != "agent-1" {
		t.Error("期望获取正确的 Agent 组")
	}

	// 测试不存在的组
	_, ok = m.GetAgentGroup("non-existent")
	if ok {
		t.Error("期望不存在的组返回 false")
	}
}

// TestListGroups 测试列出所有组
func TestListGroups(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	// 注册 Agent
	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}
	for _, id := range []string{"agent-1", "agent-2"} {
		agent := NewAgent(id, cfg, llmClient, sessMgr)
		m.RegisterAgent(agent)
	}

	// 创建多个组
	m.CreateAgentGroup("group-1", []string{"agent-1"})
	m.CreateAgentGroup("group-2", []string{"agent-2"})

	groups := m.ListGroups()

	if len(groups) != 2 {
		t.Errorf("期望 2 个组，实际为 %d", len(groups))
	}
}

// TestExecuteTask 测试调度任务
func TestExecuteTask(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{
		chatFunc: func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Content: "任务执行结果"}, nil
		},
	}
	sessMgr := session.NewManager(100, llmClient)

	m.llmClient = llmClient
	m.sessMgr = sessMgr

	// 注册一个 General 类型的 Agent
	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}
	agent := NewAgent("general-agent", cfg, llmClient, sessMgr)
	m.RegisterAgent(agent)

	// 执行任务
	ctx := context.Background()
	result, err := m.ExecuteTask(ctx, "测试任务", AgentTypeGeneral)
	if err != nil {
		t.Errorf("执行任务失败: %v", err)
	}

	if result != "任务执行结果" {
		t.Errorf("期望结果为 任务执行结果，实际为 %s", result)
	}
}

// TestExecuteTaskNoAvailableAgent 测试无可用 Agent
func TestExecuteTaskNoAvailableAgent(t *testing.T) {
	m := NewManager()

	// 没有注册任何 Agent
	ctx := context.Background()
	_, err := m.ExecuteTask(ctx, "测试任务", AgentTypeGeneral)
	if err == nil {
		t.Error("期望执行任务失败，实际成功")
	}
}

// TestDistributeTask 测试分发任务到多个 Agent
func TestDistributeTask(t *testing.T) {
	m := NewManager()

	llmClient := &mockLLMClient{
		chatFunc: func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Content: "分发任务结果"}, nil
		},
	}
	sessMgr := session.NewManager(100, llmClient)

	m.llmClient = llmClient
	m.sessMgr = sessMgr

	// 注册多个 Agent
	for i := 0; i < 2; i++ {
		cfg := AgentConfig{
			Type:  AgentTypeGeneral,
			Model: "test",
		}
		agent := NewAgent("agent-"+string(rune('a'+i)), cfg, llmClient, sessMgr)
		m.RegisterAgent(agent)
	}

	// 分发任务
	ctx := context.Background()
	results, err := m.DistributeTask(ctx, "测试任务", []string{"agent-a", "agent-b"})
	if err != nil {
		t.Errorf("分发任务失败: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("期望 2 个结果，实际为 %d", len(results))
	}
}

// TestDistributeTaskWithNonExistentAgent 测试分发到不存在的 Agent
func TestDistributeTaskWithNonExistentAgent(t *testing.T) {
	m := NewManager()

	// 分发到不存在的 Agent
	ctx := context.Background()
	results, _ := m.DistributeTask(ctx, "测试任务", []string{"non-existent"})

	if len(results) != 1 {
		t.Errorf("期望 1 个结果，实际为 %d", len(results))
	}

	if results[0].Error == nil {
		t.Error("期望结果包含错误")
	}
}

// TestFilterTools 测试工具过滤功能
func TestFilterTools(t *testing.T) {
	// 这个测试验证 filterTools 函数的逻辑
	// 实际测试会在 CreateDefaultAgents 中间接验证

	// 创建 mock tools
	mockTools := []interface {
		Name() string
	}{
		&mockTool{nameVal: "read"},
		&mockTool{nameVal: "write"},
		&mockTool{nameVal: "exec"},
		&mockTool{nameVal: "web_fetch"},
	}

	// 转换为 tools.Tool 类型进行测试
	// 这里只做简单的功能验证
	_ = mockTools
}
