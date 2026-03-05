// Package agent 多 Agent 协作包
// 提供 Agent 核心结构、协作机制和任务分发功能
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/session"
	"github.com/yahao333/myclawdbot/internal/tools"
	"github.com/yahao333/myclawdbot/pkg/types"
)

// AgentType Agent 类型
type AgentType string

const (
	AgentTypeGeneral  AgentType = "general"  // 通用 Agent
	AgentTypeResearch AgentType = "research" // 研究型 Agent
	AgentTypeCoder   AgentType = "coder"    // 编码 Agent
	AgentTypePlanner AgentType = "planner"  // 规划 Agent
	AgentTypeExecutor AgentType = "executor" // 执行 Agent
)

// AgentStatus Agent 状态
type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"    // 空闲
	AgentStatusBusy    AgentStatus = "busy"    // 工作中
	AgentStatusWaiting AgentStatus = "waiting" // 等待中
	AgentStatusError   AgentStatus = "error"   // 错误
)

// AgentConfig Agent 配置
type AgentConfig struct {
	Type         AgentType     // Agent 类型
	Name         string        // Agent 名称
	Description  string        // Agent 描述
	Model        string        // 使用的模型
	Tools        []tools.Tool  // 可用工具
	MaxRetries   int           // 最大重试次数
	Timeout      time.Duration // 超时时间
	SystemPrompt string        // 系统提示词
}

// Agent 代表一个智能 Agent
// 具有独立的任务处理能力和工具使用权限
type Agent struct {
	ID          string              // Agent ID
	Config      AgentConfig         // Agent 配置
	Status      AgentStatus         // 当前状态
	SessionMgr  *session.Manager    // 会话管理器
	LLMClient   llm.Client          // LLM 客户端
	Tools       map[string]tools.Tool // 工具映射
	Capabilities []string           // 能力列表
	CreatedAt   time.Time           // 创建时间
	UpdatedAt   time.Time           // 更新时间
	mu          sync.RWMutex
}

// NewAgent 创建新 Agent
func NewAgent(id string, cfg AgentConfig, llmClient llm.Client, sessMgr *session.Manager) *Agent {
	// 构建工具映射
	toolMap := make(map[string]tools.Tool)
	for _, t := range cfg.Tools {
		toolMap[t.Name()] = t
	}

	// 根据类型设置默认系统提示词
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = getDefaultSystemPrompt(cfg.Type)
	}

	return &Agent{
		ID:          id,
		Config:      cfg,
		Status:      AgentStatusIdle,
		SessionMgr:  sessMgr,
		LLMClient:   llmClient,
		Tools:       toolMap,
		Capabilities: getCapabilities(cfg.Type),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// getDefaultSystemPrompt 获取默认系统提示词
func getDefaultSystemPrompt(agentType AgentType) string {
	switch agentType {
	case AgentTypeGeneral:
		return "你是一个有用的 AI 助手，可以帮助用户完成各种任务。"
	case AgentTypeResearch:
		return "你是一个研究型 AI 助手，擅长分析信息、收集数据和提供详细报告。"
	case AgentTypeCoder:
		return "你是一个编程助手，擅长编写、调试和解释代码。"
	case AgentTypePlanner:
		return "你是一个规划助手，擅长分析任务、制定计划和管理流程。"
	case AgentTypeExecutor:
		return "你是一个执行助手，擅长执行具体操作和完成任务。"
	default:
		return "你是一个 AI 助手。"
	}
}

// getCapabilities 获取 Agent 能力列表
func getCapabilities(agentType AgentType) []string {
	switch agentType {
	case AgentTypeGeneral:
		return []string{"chat", "analysis", "general_task"}
	case AgentTypeResearch:
		return []string{"research", "analysis", "data_collection", "report"}
	case AgentTypeCoder:
		return []string{"code_write", "code_review", "debug", "refactor"}
	case AgentTypePlanner:
		return []string{"planning", "task_breakdown", "coordination"}
	case AgentTypeExecutor:
		return []string{"execution", "tool_use", "operation"}
	default:
		return []string{"general"}
	}
}

// Execute 执行任务
func (a *Agent) Execute(ctx context.Context, task string) (string, error) {
	a.mu.Lock()
	a.Status = AgentStatusBusy
	a.UpdatedAt = time.Now()
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.Status = AgentStatusIdle
		a.UpdatedAt = time.Now()
		a.mu.Unlock()
	}()

	// 创建会话
	sess := a.SessionMgr.CreateSession(a.ID)

	// 调用 LLM (使用 session.SendMessage 会自动添加消息)
	resp, err := sess.SendMessage(ctx, a.LLMClient, task)
	if err != nil {
		a.mu.Lock()
		a.Status = AgentStatusError
		a.mu.Unlock()
		return "", fmt.Errorf("agent %s execute failed: %w", a.ID, err)
	}

	return resp, nil
}

// ExecuteWithTools 使用工具执行任务
func (a *Agent) ExecuteWithTools(ctx context.Context, task string, toolNames []string) (string, error) {
	a.mu.Lock()
	a.Status = AgentStatusBusy
	a.UpdatedAt = time.Now()
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.Status = AgentStatusIdle
		a.UpdatedAt = time.Now()
		a.mu.Unlock()
	}()

	// 过滤可用工具
	availableTools := make([]tools.Tool, 0)
	for _, name := range toolNames {
		if tool, ok := a.Tools[name]; ok {
			availableTools = append(availableTools, tool)
		}
	}

	if len(availableTools) == 0 {
		return a.Execute(ctx, task)
	}

	// 构建工具描述
	toolDescs := make([]types.ToolDefinition, len(availableTools))
	for i, t := range availableTools {
		params := t.Parameters()
		toolDescs[i] = types.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: params,
		}
	}

	// 调用 LLM 带工具
	req := &llm.ChatRequest{
		Model: a.Config.Model,
		Messages: []types.Message{
			{Role: "system", Content: a.Config.SystemPrompt},
			{Role: "user", Content: task},
		},
		Tools: toolDescs,
	}

	resp, err := a.LLMClient.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	// 执行工具调用
	if len(resp.ToolCalls) > 0 {
		results := ""
		for _, call := range resp.ToolCalls {
			if tool, ok := a.Tools[call.Name]; ok {
				result, err := tool.Execute(ctx, call.Args)
				if err != nil {
					results += fmt.Sprintf("Tool %s error: %v\n", call.Name, err)
				} else {
					results += result + "\n"
				}
			}
		}
		return results, nil
	}

	return resp.Content, nil
}

// GetStatus 获取 Agent 状态
func (a *Agent) GetStatus() AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Status
}

// HasTool 检查是否有指定工具
func (a *Agent) HasTool(name string) bool {
	_, ok := a.Tools[name]
	return ok
}

// AddTool 添加工具
func (a *Agent) AddTool(tool tools.Tool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Tools[tool.Name()] = tool
	a.UpdatedAt = time.Now()
}

// RemoveTool 移除工具
func (a *Agent) RemoveTool(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.Tools, name)
	a.UpdatedAt = time.Now()
}

// GetInfo 获取 Agent 信息
func (a *Agent) GetInfo() AgentInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return AgentInfo{
		ID:           a.ID,
		Name:         a.Config.Name,
		Type:         a.Config.Type,
		Description:  a.Config.Description,
		Status:       a.Status,
		Capabilities: a.Capabilities,
		ToolCount:    len(a.Tools),
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// AgentInfo Agent 信息
type AgentInfo struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Type         AgentType     `json:"type"`
	Description  string        `json:"description"`
	Status       AgentStatus   `json:"status"`
	Capabilities []string      `json:"capabilities"`
	ToolCount    int           `json:"tool_count"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}
