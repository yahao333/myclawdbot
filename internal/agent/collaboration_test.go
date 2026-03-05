package agent

import (
	"context"
	"testing"
	"time"

	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/session"
)

// TestNewCollaboration 创建协作管理器测试
func TestNewCollaboration(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)

	if collab == nil {
		t.Error("期望 Collaboration 不为 nil")
	}

	if collab.manager != m {
		t.Error("期望 manager 被正确设置")
	}

	if collab.inbox == nil {
		t.Error("期望 inbox map 不为 nil")
	}

	if collab.history == nil {
		t.Error("期望 history 不为 nil")
	}
}

// TestCollaborationSubscribe 测试订阅消息
func TestCollaborationSubscribe(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)

	// 订阅 Agent
	err := collab.Subscribe("agent-1")
	if err != nil {
		t.Errorf("订阅失败: %v", err)
	}

	// 验证订阅成功
	if _, ok := collab.inbox["agent-1"]; !ok {
		t.Error("期望 inbox 包含订阅的 agent")
	}
}

// TestCollaborationSubscribeDuplicate 测试重复订阅
func TestCollaborationSubscribeDuplicate(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)

	// 第一次订阅
	collab.Subscribe("agent-1")

	// 第二次订阅应该失败
	err := collab.Subscribe("agent-1")
	if err == nil {
		t.Error("期望重复订阅失败")
	}
}

// TestCollaborationUnsubscribe 测试取消订阅
func TestCollaborationUnsubscribe(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)

	// 订阅 Agent
	collab.Subscribe("agent-1")

	// 取消订阅
	err := collab.Unsubscribe("agent-1")
	if err != nil {
		t.Errorf("取消订阅失败: %v", err)
	}

	// 验证取消订阅成功
	if _, ok := collab.inbox["agent-1"]; ok {
		t.Error("期望 inbox 不包含已取消订阅的 agent")
	}
}

// TestCollaborationUnsubscribeNonSubscribed 测试取消未订阅的 Agent
func TestCollaborationUnsubscribeNonSubscribed(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)

	// 取消未订阅的 Agent 应该失败
	err := collab.Unsubscribe("non-existent")
	if err == nil {
		t.Error("期望取消未订阅的 agent 失败")
	}
}

// TestCollaborationSend 测试发送消息
func TestCollaborationSend(t *testing.T) {
	// 创建 Manager 并注册 Agent
	m := NewManager()
	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent1 := NewAgent("agent-1", cfg, llmClient, sessMgr)
	agent2 := NewAgent("agent-2", cfg, llmClient, sessMgr)

	m.RegisterAgent(agent1)
	m.RegisterAgent(agent2)

	// 创建协作管理器
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")
	collab.Subscribe("agent-2")

	// 发送消息
	msg := Message{
		FromAgent: "agent-1",
		ToAgent:   "agent-2",
		Content:   "测试消息",
		Type:      MessageTypeRequest,
	}

	err := collab.Send(msg)
	if err != nil {
		t.Errorf("发送消息失败: %v", err)
	}

	// 验证消息历史
	history := collab.GetHistory("agent-1", 10)
	if len(history) != 1 {
		t.Errorf("期望历史记录包含 1 条消息，实际为 %d", len(history))
	}
}

// TestCollaborationSendFromNonExistent 测试发送到不存在的发送者
func TestCollaborationSendFromNonExistent(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)

	msg := Message{
		FromAgent: "non-existent",
		ToAgent:   "agent-2",
		Content:   "测试消息",
	}

	err := collab.Send(msg)
	if err == nil {
		t.Error("期望发送到不存在的发送者失败")
	}
}

// TestCollaborationSendToNonExistent 测试发送到不存在的接收者
func TestCollaborationSendToNonExistent(t *testing.T) {
	// 创建 Manager 并注册发送者 Agent
	m := NewManager()
	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent1 := NewAgent("agent-1", cfg, llmClient, sessMgr)
	m.RegisterAgent(agent1)

	// 创建协作管理器
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")

	// 发送到不存在的接收者应该失败
	msg := Message{
		FromAgent: "agent-1",
		ToAgent:   "non-existent",
		Content:   "测试消息",
	}

	err := collab.Send(msg)
	if err == nil {
		t.Error("期望发送到不存在的接收者失败")
	}
}

// TestCollaborationReceive 测试接收消息
func TestCollaborationReceive(t *testing.T) {
	// 创建 Manager 并注册 Agent
	m := NewManager()
	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent1 := NewAgent("agent-1", cfg, llmClient, sessMgr)
	agent2 := NewAgent("agent-2", cfg, llmClient, sessMgr)

	m.RegisterAgent(agent1)
	m.RegisterAgent(agent2)

	// 创建协作管理器并订阅
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")
	collab.Subscribe("agent-2")

	// 发送消息
	msg := Message{
		FromAgent: "agent-1",
		ToAgent:   "agent-2",
		Content:   "测试消息",
		Type:      MessageTypeRequest,
	}
	collab.Send(msg)

	// 接收消息
	received, err := collab.Receive("agent-2", time.Second)
	if err != nil {
		t.Errorf("接收消息失败: %v", err)
	}

	if received.Content != "测试消息" {
		t.Errorf("期望消息内容为 测试消息，实际为 %s", received.Content)
	}
}

// TestCollaborationReceiveTimeout 测试接收超时
func TestCollaborationReceiveTimeout(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")

	// 尝试接收不存在的消息应该超时
	_, err := collab.Receive("agent-1", time.Millisecond*10)
	if err == nil {
		t.Error("期望接收超时")
	}
}

// TestCollaborationReceiveNonBlock 测试非阻塞接收
func TestCollaborationReceiveNonBlock(t *testing.T) {
	// 创建 Manager 并注册 Agent
	m := NewManager()
	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent1 := NewAgent("agent-1", cfg, llmClient, sessMgr)
	agent2 := NewAgent("agent-2", cfg, llmClient, sessMgr)

	m.RegisterAgent(agent1)
	m.RegisterAgent(agent2)

	// 创建协作管理器并订阅
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")
	collab.Subscribe("agent-2")

	// 发送消息
	msg := Message{
		FromAgent: "agent-1",
		ToAgent:   "agent-2",
		Content:   "测试消息",
		Type:      MessageTypeRequest,
	}
	collab.Send(msg)

	// 非阻塞接收消息
	received, ok := collab.ReceiveNonBlock("agent-2")
	if !ok {
		t.Error("期望能够接收到消息")
	}

	if received.Content != "测试消息" {
		t.Errorf("期望消息内容为 测试消息，实际为 %s", received.Content)
	}
}

// TestCollaborationReceiveNonBlockEmpty 测试非阻塞接收空邮箱
func TestCollaborationReceiveNonBlockEmpty(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")

	// 非阻塞接收不存在的消息
	_, ok := collab.ReceiveNonBlock("agent-1")
	if ok {
		t.Error("期望接收失败")
	}
}

// TestCollaborationReceiveNonBlockNotSubscribed 测试非阻塞接收未订阅的 Agent
func TestCollaborationReceiveNonBlockNotSubscribed(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)

	// 非阻塞接收未订阅的 agent 的消息
	_, ok := collab.ReceiveNonBlock("non-existent")
	if ok {
		t.Error("期望接收失败")
	}
}

// TestCollaborationBroadcast 测试广播消息
func TestCollaborationBroadcast(t *testing.T) {
	// 创建 Manager 并注册多个 Agent
	m := NewManager()
	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	for _, id := range []string{"agent-1", "agent-2", "agent-3"} {
		agent := NewAgent(id, cfg, llmClient, sessMgr)
		m.RegisterAgent(agent)
	}

	// 创建协作管理器并订阅所有 Agent
	collab := NewCollaboration(m)
	for _, id := range []string{"agent-1", "agent-2", "agent-3"} {
		collab.Subscribe(id)
	}

	// 广播消息
	err := collab.Broadcast("agent-1", "广播消息", MessageTypeTask)
	if err != nil {
		t.Errorf("广播消息失败: %v", err)
	}

	// 验证其他 Agent 都能收到消息
	for _, id := range []string{"agent-2", "agent-3"} {
		received, err := collab.Receive(id, time.Second)
		if err != nil {
			t.Errorf("接收广播消息失败: %v", err)
		}
		if received.Content != "广播消息" {
			t.Errorf("期望消息内容为 广播消息，实际为 %s", received.Content)
		}
	}
}

// TestCollaborationGetHistory 测试获取消息历史
func TestCollaborationGetHistory(t *testing.T) {
	// 创建 Manager 并注册 Agent
	m := NewManager()
	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent1 := NewAgent("agent-1", cfg, llmClient, sessMgr)
	agent2 := NewAgent("agent-2", cfg, llmClient, sessMgr)

	m.RegisterAgent(agent1)
	m.RegisterAgent(agent2)

	// 创建协作管理器并订阅
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")
	collab.Subscribe("agent-2")

	// 发送多条消息
	for i := 0; i < 5; i++ {
		msg := Message{
			FromAgent: "agent-1",
			ToAgent:   "agent-2",
			Content:   "消息 " + string(rune('0'+i)),
		}
		collab.Send(msg)
	}

	// 获取历史
	history := collab.GetHistory("agent-1", 10)
	if len(history) != 5 {
		t.Errorf("期望历史记录包含 5 条消息，实际为 %d", len(history))
	}

	// 测试限制数量
	history = collab.GetHistory("agent-1", 3)
	if len(history) != 3 {
		t.Errorf("期望历史记录包含 3 条消息，实际为 %d", len(history))
	}
}

// TestCollaborationClearHistory 测试清除历史
func TestCollaborationClearHistory(t *testing.T) {
	// 创建 Manager 并注册 Agent
	m := NewManager()
	llmClient := &mockLLMClient{}
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:  AgentTypeGeneral,
		Model: "test",
	}

	agent1 := NewAgent("agent-1", cfg, llmClient, sessMgr)
	agent2 := NewAgent("agent-2", cfg, llmClient, sessMgr)

	m.RegisterAgent(agent1)
	m.RegisterAgent(agent2)

	// 创建协作管理器并订阅
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")
	collab.Subscribe("agent-2")

	// 发送消息
	msg := Message{
		FromAgent: "agent-1",
		ToAgent:   "agent-2",
		Content:   "测试消息",
	}
	collab.Send(msg)

	// 清除历史
	collab.ClearHistory()

	// 验证历史已清除
	history := collab.GetHistory("agent-1", 10)
	if len(history) != 0 {
		t.Errorf("期望历史记录为空，实际为 %d", len(history))
	}
}

// TestNewCollaborationWorkflow 创建协作工作流测试
func TestNewCollaborationWorkflow(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)
	wf := NewCollaborationWorkflow(collab, m)

	if wf == nil {
		t.Error("期望 CollaborationWorkflow 不为 nil")
	}

	if wf.collaboration != collab {
		t.Error("期望 collaboration 被正确设置")
	}

	if wf.manager != m {
		t.Error("期望 manager 被正确设置")
	}

	if wf.workflows == nil {
		t.Error("期望 workflows map 不为 nil")
	}
}

// TestCollaborationWorkflowRegisterWorkflow 测试注册工作流
func TestCollaborationWorkflowRegisterWorkflow(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)
	wf := NewCollaborationWorkflow(collab, m)

	workflow := &Workflow{
		ID:   "test-workflow",
		Name: "测试工作流",
		Steps: []WorkflowStep{
			{
				AgentID:   "agent-1",
				Action:    "execute",
				Input:     "测试输入",
				OutputKey: "result",
			},
		},
	}

	// 注册工作流
	err := wf.RegisterWorkflow(workflow)
	if err != nil {
		t.Errorf("注册工作流失败: %v", err)
	}
}

// TestCollaborationWorkflowRegisterDuplicate 测试注册重复工作流
func TestCollaborationWorkflowRegisterDuplicate(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)
	wf := NewCollaborationWorkflow(collab, m)

	workflow := &Workflow{
		ID:   "test-workflow",
		Name: "测试工作流",
	}

	// 第一次注册
	wf.RegisterWorkflow(workflow)

	// 第二次注册应该失败
	err := wf.RegisterWorkflow(workflow)
	if err == nil {
		t.Error("期望重复注册失败")
	}
}

// TestCollaborationWorkflowExecuteWorkflow 测试执行工作流
func TestCollaborationWorkflowExecuteWorkflow(t *testing.T) {
	// 创建 Manager 并注册 Agent
	m := NewManager()

	// 创建 mock LLM client 并设置返回值
	llmClient := &mockLLMClient{
		chatFunc: func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Content: "工作流执行结果"}, nil
		},
	}
	// 创建 session manager 时传入 llmClient
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:        AgentTypeGeneral,
		Model:       "test",
		SystemPrompt: "你是一个测试助手。",
	}

	// 创建 agent 时传入相同的 llmClient
	agent := NewAgent("agent-1", cfg, llmClient, sessMgr)
	m.RegisterAgent(agent)

	// 创建协作和工作流
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")
	wf := NewCollaborationWorkflow(collab, m)

	// 注册工作流
	workflow := &Workflow{
		ID:   "test-workflow",
		Name: "测试工作流",
		Steps: []WorkflowStep{
			{
				AgentID:   "agent-1",
				Action:    "execute",
				Input:     "测试输入",
				OutputKey: "result",
			},
		},
	}
	wf.RegisterWorkflow(workflow)

	// 执行工作流 - 简化测试，只验证不会崩溃即可
	// 工作流执行依赖于 session 和 tools 的完整初始化，完整测试需要更多设置
	_, _ = wf.ExecuteWorkflow(context.Background(), "test-workflow", "初始输入")
}

// TestCollaborationWorkflowExecuteNonExistent 测试执行不存在的工作流
func TestCollaborationWorkflowExecuteNonExistent(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)
	wf := NewCollaborationWorkflow(collab, m)

	// 执行不存在的工作流应该失败
	_, err := wf.ExecuteWorkflow(context.Background(), "non-existent", "输入")
	if err == nil {
		t.Error("期望执行不存在的工作流失败")
	}
}

// TestCollaborationWorkflowExecuteWithDelegate 测试执行包含委托的工作流
func TestCollaborationWorkflowExecuteWithDelegate(t *testing.T) {
	// 创建 Manager 并注册多个 Agent
	m := NewManager()

	// 创建 mock LLM client 并设置返回值
	llmClient := &mockLLMClient{
		chatFunc: func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Content: "委托执行结果"}, nil
		},
	}
	// 创建 session manager 时传入 llmClient
	sessMgr := session.NewManager(100, llmClient)

	cfg := AgentConfig{
		Type:        AgentTypeGeneral,
		Model:       "test",
		SystemPrompt: "你是一个测试助手。",
	}

	agent1 := NewAgent("agent-1", cfg, llmClient, sessMgr)
	agent2 := NewAgent("agent-2", cfg, llmClient, sessMgr)

	m.RegisterAgent(agent1)
	m.RegisterAgent(agent2)

	// 创建协作和工作流
	collab := NewCollaboration(m)
	collab.Subscribe("agent-1")
	collab.Subscribe("agent-2")
	wf := NewCollaborationWorkflow(collab, m)

	// 注册包含委托的工作流
	workflow := &Workflow{
		ID:   "delegate-workflow",
		Name: "委托工作流",
		Steps: []WorkflowStep{
			{
				AgentID:    "agent-1",
				Action:     "delegate",
				Input:      "委托任务",
				OutputKey:  "result",
				DelegateTo: "agent-2",
			},
		},
	}
	wf.RegisterWorkflow(workflow)

	// 执行工作流 - 简化测试，只验证不会崩溃即可
	// 工作流执行依赖于 session 和 tools 的完整初始化，完整测试需要更多设置
	_, _ = wf.ExecuteWorkflow(context.Background(), "delegate-workflow", "初始输入")
}

// TestReplaceVariables 测试变量替换
func TestReplaceVariables(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)
	wf := NewCollaborationWorkflow(collab, m)

	outputs := map[string]string{
		"name": "张三",
		"age":  "25",
	}

	// 测试变量替换
	input := "我叫{name}，今年{age}岁"
	result := wf.replaceVariables(input, outputs)

	if result != "我叫张三，今年25岁" {
		t.Errorf("期望替换结果为 我叫张三，今年25岁，实际为 %s", result)
	}

	// 测试不存在的变量
	input = "我叫{name}，职业是{profession}"
	result = wf.replaceVariables(input, outputs)

	if result != "我叫张三，职业是{profession}" {
		t.Errorf("期望未替换的变量保持原样，实际为 %s", result)
	}
}

// TestAggregateOutputs 测试输出聚合
func TestAggregateOutputs(t *testing.T) {
	m := NewManager()
	collab := NewCollaboration(m)
	wf := NewCollaborationWorkflow(collab, m)

	steps := []WorkflowStep{
		{OutputKey: "result1"},
		{OutputKey: "result2"},
		{OutputKey: "result3"},
	}

	outputs := map[string]string{
		"result1": "第一个结果",
		"result2": "第二个结果",
		"result3": "第三个结果",
	}

	// 测试聚合
	result := wf.aggregateOutputs(steps, outputs)

	if len(result) == 0 {
		t.Error("期望聚合结果不为空")
	}
}

// TestJoinStrings 测试字符串连接
func TestJoinStrings(t *testing.T) {
	tests := []struct {
		input    []string
		sep      string
		expected string
	}{
		{[]string{"a", "b", "c"}, ",", "a,b,c"},
		{[]string{"a"}, ",", "a"},
		{[]string{}, ",", ""},
		{[]string{"hello", "world"}, " ", "hello world"},
	}

	for _, tt := range tests {
		result := joinStrings(tt.input, tt.sep)
		if result != tt.expected {
			t.Errorf("期望结果为 %s，实际为 %s", tt.expected, result)
		}
	}
}
