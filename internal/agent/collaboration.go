// Package agent 多 Agent 协作包
// 提供 Agent 核心结构、协作机制和任务分发功能
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Message 协作消息
type Message struct {
	ID         string                 // 消息 ID
	FromAgent  string                 // 发送者 Agent ID
	ToAgent    string                 // 接收者 Agent ID
	Content    string                 // 消息内容
	Type       MessageType            // 消息类型
	Metadata   map[string]interface{} // 元数据
	Timestamp  time.Time              // 时间戳
	ReplyTo    string                 // 回复的消息 ID
}

// MessageType 消息类型
type MessageType string

const (
	MessageTypeRequest  MessageType = "request"  // 请求
	MessageTypeResponse MessageType = "response" // 响应
	MessageTypeTask    MessageType = "task"    // 任务分发
	MessageTypeResult  MessageType = "result"  // 结果返回
	MessageTypeError   MessageType = "error"   // 错误
	MessageTypePing    MessageType = "ping"    // 心跳
	MessageTypePong    MessageType = "pong"    // 响应心跳
)

// Collaboration 协作管理器
// 负责 Agent 之间的消息传递和任务协调
type Collaboration struct {
	manager    *Manager
	inbox      map[string]chan *Message // agentID -> inbox
	history    []Message                 // 消息历史
	mu         sync.RWMutex
}

// NewCollaboration 创建协作管理器
func NewCollaboration(manager *Manager) *Collaboration {
	return &Collaboration{
		manager: manager,
		inbox:  make(map[string]chan *Message),
		history: make([]Message, 0),
	}
}

// Subscribe Agent 订阅消息
func (c *Collaboration) Subscribe(agentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.inbox[agentID]; exists {
		return fmt.Errorf("agent %s already subscribed", agentID)
	}

	c.inbox[agentID] = make(chan *Message, 100)
	return nil
}

// Unsubscribe Agent 取消订阅
func (c *Collaboration) Unsubscribe(agentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, exists := c.inbox[agentID]
	if !exists {
		return fmt.Errorf("agent %s not subscribed", agentID)
	}

	close(ch)
	delete(c.inbox, agentID)
	return nil
}

// Send 发送消息
func (c *Collaboration) Send(msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 验证发送者和接收者存在
	if _, ok := c.manager.agents[msg.FromAgent]; !ok {
		return fmt.Errorf("sender agent %s not found", msg.FromAgent)
	}

	if _, ok := c.manager.agents[msg.ToAgent]; !ok {
		return fmt.Errorf("receiver agent %s not found", msg.ToAgent)
	}

	msg.Timestamp = time.Now()
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}

	// 添加到历史
	c.history = append(c.history, msg)

	// 发送到接收者邮箱
	if ch, ok := c.inbox[msg.ToAgent]; ok {
		select {
		case ch <- &msg:
		default:
			return fmt.Errorf("receiver %s inbox is full", msg.ToAgent)
		}
	}

	return nil
}

// Receive 接收消息（阻塞）
func (c *Collaboration) Receive(agentID string, timeout time.Duration) (*Message, error) {
	c.mu.RLock()
	ch, ok := c.inbox[agentID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("agent %s not subscribed", agentID)
	}

	select {
	case msg := <-ch:
		return msg, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for message")
	}
}

// ReceiveNonBlock 非阻塞接收
func (c *Collaboration) ReceiveNonBlock(agentID string) (*Message, bool) {
	c.mu.RLock()
	ch, ok := c.inbox[agentID]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	select {
	case msg := <-ch:
		return msg, true
	default:
		return nil, false
	}
}

// Broadcast 广播消息
func (c *Collaboration) Broadcast(fromAgentID string, content string, msgType MessageType) error {
	c.mu.RLock()
	agentIDs := make([]string, 0, len(c.inbox))
	for id := range c.inbox {
		if id != fromAgentID {
			agentIDs = append(agentIDs, id)
		}
	}
	c.mu.RUnlock()

	for _, toAgentID := range agentIDs {
		msg := Message{
			FromAgent: fromAgentID,
			ToAgent:   toAgentID,
			Content:   content,
			Type:      msgType,
		}
		if err := c.Send(msg); err != nil {
			return err
		}
	}

	return nil
}

// GetHistory 获取消息历史
func (c *Collaboration) GetHistory(agentID string, limit int) []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []Message
	count := 0

	for i := len(c.history) - 1; i >= 0 && count < limit; i-- {
		msg := c.history[i]
		if msg.FromAgent == agentID || msg.ToAgent == agentID {
			results = append(results, msg)
			count++
		}
	}

	return results
}

// ClearHistory 清除历史
func (c *Collaboration) ClearHistory() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = make([]Message, 0)
}

// CollaborationWorkflow 协作工作流
type CollaborationWorkflow struct {
	collaboration *Collaboration
	manager       *Manager
	workflows     map[string]*Workflow
	mu            sync.RWMutex
}

// Workflow 工作流定义
type Workflow struct {
	ID          string
	Name        string
	Description string
	Steps       []WorkflowStep
	CreatedAt   time.Time
}

// WorkflowStep 工作流步骤
type WorkflowStep struct {
	AgentID    string       // 执行的 Agent
	Action     string       // 动作类型: execute, delegate, aggregate
	Input      string       // 输入模板
	OutputKey  string       // 输出变量名
	DelegateTo string       // 委托给其他 Agent
	Aggregator string       // 聚合方式: concat, merge, select
}

// NewCollaborationWorkflow 创建协作工作流
func NewCollaborationWorkflow(collaboration *Collaboration, manager *Manager) *CollaborationWorkflow {
	return &CollaborationWorkflow{
		collaboration: collaboration,
		manager:       manager,
		workflows:     make(map[string]*Workflow),
	}
}

// RegisterWorkflow 注册工作流
func (w *CollaborationWorkflow) RegisterWorkflow(workflow *Workflow) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.workflows[workflow.ID]; exists {
		return fmt.Errorf("workflow %s already exists", workflow.ID)
	}

	workflow.CreatedAt = time.Now()
	w.workflows[workflow.ID] = workflow
	return nil
}

// ExecuteWorkflow 执行工作流
func (w *CollaborationWorkflow) ExecuteWorkflow(ctx context.Context, workflowID string, initialInput string) (map[string]string, error) {
	w.mu.RLock()
	workflow, ok := w.workflows[workflowID]
	w.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("workflow %s not found", workflowID)
	}

	// 存储每一步的输出
	outputs := make(map[string]string)
	outputs["_initial_input"] = initialInput

	// 执行每一步
	for i, step := range workflow.Steps {
		agent, exists := w.manager.GetAgent(step.AgentID)
		if !exists {
			return outputs, fmt.Errorf("agent %s not found at step %d", step.AgentID, i)
		}

		// 替换输入变量
		input := w.replaceVariables(step.Input, outputs)

		var result string
		var err error

		switch step.Action {
		case "execute":
			result, err = agent.Execute(ctx, input)
		case "delegate":
			// 委托给其他 Agent
			delegateAgent, ok := w.manager.GetAgent(step.DelegateTo)
			if !ok {
				return outputs, fmt.Errorf("delegate agent %s not found", step.DelegateTo)
			}
			result, err = delegateAgent.Execute(ctx, input)

			// 发送结果给原 Agent
			if err == nil {
				msg := Message{
					FromAgent: step.DelegateTo,
					ToAgent:   step.AgentID,
					Content:   result,
					Type:      MessageTypeResult,
				}
				w.collaboration.Send(msg)
			}
		default:
			result, err = agent.Execute(ctx, input)
		}

		if err != nil {
			return outputs, fmt.Errorf("step %d failed: %w", i, err)
		}

		outputs[step.OutputKey] = result
	}

	// 聚合结果
	finalOutput := w.aggregateOutputs(workflow.Steps, outputs)

	return map[string]string{
		"_final_output": finalOutput,
		"_outputs":      fmt.Sprintf("%v", outputs),
	}, nil
}

// replaceVariables 替换输入中的变量
func (w *CollaborationWorkflow) replaceVariables(input string, outputs map[string]string) string {
	result := input
	for key, value := range outputs {
		placeholder := fmt.Sprintf("{%s}", key)
		result = replaceAll(result, placeholder, value)
	}
	return result
}

// replaceAll 替换所有
func replaceAll(s, old, new string) string {
	result := s
	for {
		result = replaceOne(result, old, new)
		if result == s {
			break
		}
		s = result
	}
	return result
}

func replaceOne(s, old, new string) string {
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

// aggregateOutputs 聚合输出
func (w *CollaborationWorkflow) aggregateOutputs(steps []WorkflowStep, outputs map[string]string) string {
	var results []string
	for _, step := range steps {
		if output, ok := outputs[step.OutputKey]; ok {
			results = append(results, output)
		}
	}

	switch len(results) {
	case 0:
		return ""
	case 1:
		return results[0]
	default:
		// 用分隔符连接
		return joinStrings(results, "\n---\n")
	}
}

// joinStrings 连接字符串
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// generateMessageID 生成消息 ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), randInt())
}

func randInt() int {
	// 简单随机数
	now := time.Now()
	return int(now.Nanosecond())
}
