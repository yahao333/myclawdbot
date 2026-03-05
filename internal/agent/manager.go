// Package agent 多 Agent 协作包
// 提供 Agent 核心结构、协作机制和任务分发功能
package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/yahao333/myclawdbot/internal/config"
	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/session"
	"github.com/yahao333/myclawdbot/internal/tools"
)

// Manager Agent 管理器
// 负责创建、管理和调度多个 Agent
type Manager struct {
	agents    map[string]*Agent      // agentID -> Agent
	groups    map[string][]string    // groupID -> []agentID
	llmClient llm.Client             // LLM 客户端
	cfg       *config.Config         // 配置
	sessMgr   *session.Manager       // 会话管理器
	toolReg   *tools.Registry        // 工具注册器
	mu        sync.RWMutex
}

// ManagerOption Manager 配置选项
type ManagerOption func(*Manager)

// WithLLMClient 设置 LLM 客户端
func WithLLMClient(client llm.Client) ManagerOption {
	return func(m *Manager) {
		m.llmClient = client
	}
}

// WithConfig 设置配置
func WithConfig(cfg *config.Config) ManagerOption {
	return func(m *Manager) {
		m.cfg = cfg
	}
}

// WithSessionManager 设置会话管理器
func WithSessionManager(sessMgr *session.Manager) ManagerOption {
	return func(m *Manager) {
		m.sessMgr = sessMgr
	}
}

// WithToolRegistry 设置工具注册器
func WithToolRegistry(reg *tools.Registry) ManagerOption {
	return func(m *Manager) {
		m.toolReg = reg
	}
}

// NewManager 创建 Agent 管理器
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		agents:   make(map[string]*Agent),
		groups:   make(map[string][]string),
		sessMgr:  session.NewManager(100, nil),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// RegisterAgent 注册 Agent
func (m *Manager) RegisterAgent(agent *Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[agent.ID]; exists {
		return fmt.Errorf("agent %s already exists", agent.ID)
	}

	m.agents[agent.ID] = agent
	return nil
}

// UnregisterAgent 注销 Agent
func (m *Manager) UnregisterAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[agentID]; !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	delete(m.agents, agentID)

	// 从所有组中移除
	for groupID, agentIDs := range m.groups {
		for i, id := range agentIDs {
			if id == agentID {
				m.groups[groupID] = append(agentIDs[:i], agentIDs[i+1:]...)
				break
			}
		}
	}

	return nil
}

// GetAgent 获取 Agent
func (m *Manager) GetAgent(agentID string) (*Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, ok := m.agents[agentID]
	return agent, ok
}

// ListAgents 列出所有 Agent
func (m *Manager) ListAgents() []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]*Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		agents = append(agents, agent)
	}
	return agents
}

// ListAgentInfos 列出所有 Agent 信息
func (m *Manager) ListAgentInfos() []AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]AgentInfo, 0, len(m.agents))
	for _, agent := range m.agents {
		infos = append(infos, agent.GetInfo())
	}
	return infos
}

// CreateAgent 创建并注册 Agent
func (m *Manager) CreateAgent(id string, cfg AgentConfig) (*Agent, error) {
	agent := NewAgent(id, cfg, m.llmClient, m.sessMgr)

	if err := m.RegisterAgent(agent); err != nil {
		return nil, err
	}

	return agent, nil
}

// CreateDefaultAgents 创建默认 Agent 组合
func (m *Manager) CreateDefaultAgents(availableTools []tools.Tool) error {
	// 创建通用 Agent
	generalTools := filterTools(availableTools, []string{"read", "write", "exec"})
	_, err := m.CreateAgent("general", AgentConfig{
		Type:        AgentTypeGeneral,
		Name:        "通用助手",
		Description: "处理一般性任务的助手",
		Model:       m.cfg.LLM.Model,
		Tools:       generalTools,
		SystemPrompt: "你是一个有用的 AI 助手，可以帮助用户完成各种任务。",
	})
	if err != nil {
		return err
	}

	// 创建编码 Agent
	coderTools := filterTools(availableTools, []string{"read", "write", "exec"})
	_, err = m.CreateAgent("coder", AgentConfig{
		Type:        AgentTypeCoder,
		Name:        "编码助手",
		Description: "专门处理编程任务的助手",
		Model:       m.cfg.LLM.Model,
		Tools:       coderTools,
		SystemPrompt: "你是一个专业的编程助手，擅长编写、调试和解释代码。请在回答中保持代码的准确性和规范性。",
	})
	if err != nil {
		return err
	}

	// 创建研究 Agent
	researchTools := filterTools(availableTools, []string{"read", "web_fetch"})
	_, err = m.CreateAgent("researcher", AgentConfig{
		Type:        AgentTypeResearch,
		Name:        "研究助手",
		Description: "专门处理研究分析任务的助手",
		Model:       m.cfg.LLM.Model,
		Tools:       researchTools,
		SystemPrompt: "你是一个研究型 AI 助手，擅长分析信息、收集数据、提供详细报告。请确保你的分析全面准确。",
	})
	if err != nil {
		return err
	}

	// 创建规划 Agent
	_, err = m.CreateAgent("planner", AgentConfig{
		Type:        AgentTypePlanner,
		Name:        "规划助手",
		Description: "专门处理任务规划和分解的助手",
		Model:       m.cfg.LLM.Model,
		Tools:       []tools.Tool{},
		SystemPrompt: "你是一个规划型 AI 助手，擅长分析任务需求、制定详细计划。请将复杂任务分解为可执行的步骤。",
	})
	if err != nil {
		return err
	}

	return nil
}

// CreateAgentGroup 创建 Agent 组
func (m *Manager) CreateAgentGroup(groupID string, agentIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证所有 agent 存在
	for _, id := range agentIDs {
		if _, exists := m.agents[id]; !exists {
			return fmt.Errorf("agent %s not found", id)
		}
	}

	m.groups[groupID] = agentIDs
	return nil
}

// GetAgentGroup 获取 Agent 组
func (m *Manager) GetAgentGroup(groupID string) ([]*Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agentIDs, ok := m.groups[groupID]
	if !ok {
		return nil, false
	}

	agents := make([]*Agent, len(agentIDs))
	for i, id := range agentIDs {
		agents[i] = m.agents[id]
	}

	return agents, true
}

// ListGroups 列出所有组
func (m *Manager) ListGroups() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]string, 0, len(m.groups))
	for id := range m.groups {
		groups = append(groups, id)
	}
	return groups
}

// ExecuteTask 调度任务到合适的 Agent
func (m *Manager) ExecuteTask(ctx context.Context, task string, preferredType AgentType) (string, error) {
	m.mu.RLock()
	var target *Agent

	// 查找指定类型的可用 Agent
	for _, agent := range m.agents {
		if agent.Config.Type == preferredType && agent.GetStatus() == AgentStatusIdle {
			target = agent
			break
		}
	}

	// 如果没有找到指定类型，使用通用 Agent
	if target == nil {
		for _, agent := range m.agents {
			if agent.Config.Type == AgentTypeGeneral && agent.GetStatus() == AgentStatusIdle {
				target = agent
				break
			}
		}
	}

	m.mu.RUnlock()

	if target == nil {
		return "", fmt.Errorf("no available agent found")
	}

	return target.Execute(ctx, task)
}

// DistributeTask 分发任务到多个 Agent
func (m *Manager) DistributeTask(ctx context.Context, task string, agentIDs []string) ([]TaskResult, error) {
	results := make([]TaskResult, len(agentIDs))

	for i, agentID := range agentIDs {
		agent, ok := m.GetAgent(agentID)
		if !ok {
			results[i] = TaskResult{
				AgentID: agentID,
				Error:   fmt.Errorf("agent not found"),
			}
			continue
		}

		result, err := agent.Execute(ctx, task)
		results[i] = TaskResult{
			AgentID: agentID,
			Result:  result,
			Error:   err,
		}
	}

	return results, nil
}

// TaskResult 任务执行结果
type TaskResult struct {
	AgentID string
	Result  string
	Error   error
}

// filterTools 根据名称过滤工具
func filterTools(allTools []tools.Tool, names []string) []tools.Tool {
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	result := make([]tools.Tool, 0)
	for _, t := range allTools {
		if nameSet[t.Name()] {
			result = append(result, t)
		}
	}
	return result
}
