package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/tools"
	"github.com/yahao333/myclawdbot/pkg/types"
)

// Manager 会话管理器
type Manager struct {
	mu        sync.RWMutex
	sessions  map[string]*Session
	maxHistory int
	llmClient llm.Client
}

// Session 会话
type Session struct {
	ID        string
	Messages  []types.Message
	CreatedAt time.Time
	UpdatedAt time.Time
	mu        sync.Mutex
}

// NewManager 创建会话管理器
func NewManager(maxHistory int, client llm.Client) *Manager {
	if maxHistory <= 0 {
		maxHistory = 100
	}

	return &Manager{
		sessions:  make(map[string]*Session),
		maxHistory: maxHistory,
		llmClient: client,
	}
}

// CreateSession 创建新会话
func (m *Manager) CreateSession(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		id = generateSessionID()
	}

	sess := &Session{
		ID:        id,
		Messages:  make([]types.Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.sessions[id] = sess
	return sess
}

// GetSession 获取会话
func (m *Manager) GetSession(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[id]
	return sess, ok
}

// DeleteSession 删除会话
func (m *Manager) DeleteSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, id)
}

// SendMessage 发送消息并获取回复
func (s *Session) SendMessage(ctx context.Context, client llm.Client, content string) (string, error) {
	s.mu.Lock()

	// 添加用户消息
	userMsg := types.Message{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
	s.Messages = append(s.Messages, userMsg)

	// 构建请求
	messages := make([]types.Message, len(s.Messages))
	copy(messages, s.Messages)

	s.mu.Unlock()

	// 获取可用工具
	toolDefs := tools.ToToolDefinitions()

	req := &llm.ChatRequest{
		Model:       "",
		Messages:    messages,
		MaxTokens:   4096,
		Temperature: 0.7,
		Tools:       toolDefs,
	}

	// 发送请求（带重试机制）
	var resp *llm.ChatResponse
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		resp, err = client.Chat(ctx, req)
		if err == nil {
			break
		}

		// 检查是否为网络错误，如果是则重试
		errMsg := err.Error()
		isNetworkError := strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "connection") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "network") ||
			strings.Contains(errMsg, "refused") ||
			strings.Contains(errMsg, "reset")

		if !isNetworkError || i == maxRetries-1 {
			return "", fmt.Errorf("llm error: %w", err)
		}

		fmt.Printf("网络错误，%d 秒后重试... (%d/%d)\n", time.Duration(i+1)*2, i+1, maxRetries)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 添加助手消息
	assistantMsg := types.Message{
		Role:      "assistant",
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
		Timestamp: time.Now(),
	}
	s.Messages = append(s.Messages, assistantMsg)

	// 如果有工具调用，执行工具
	for _, tc := range resp.ToolCalls {
		result, err := tools.Execute(ctx, tc.Name, tc.Args)
		if err != nil {
			result = fmt.Sprintf("error: %v", err)
		}

		// 添加工具结果
		toolResultMsg := types.Message{
			Role:      "user", // 工具结果作为用户消息
			Content:   result,
			Timestamp: time.Now(),
		}
		s.Messages = append(s.Messages, toolResultMsg)
	}

	// 限制历史长度
	if len(s.Messages) > 100 {
		s.Messages = s.Messages[len(s.Messages)-100:]
	}

	s.UpdatedAt = time.Now()

	return resp.Content, nil
}

// GetHistory 获取历史消息
func (s *Session) GetHistory() []types.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	history := make([]types.Message, len(s.Messages))
	copy(history, s.Messages)
	return history
}

// ClearHistory 清除历史消息
func (s *Session) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = make([]types.Message, 0)
	s.UpdatedAt = time.Now()
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}
