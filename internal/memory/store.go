package memory

import (
	"context"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/pkg/types"
)

// Memory 记忆接口
type Memory interface {
	// Add 添加消息到记忆
	Add(ctx context.Context, sessionID string, msg *types.Message) error

	// Get 获取会话记忆
	Get(ctx context.Context, sessionID string, limit int) ([]types.Message, error)

	// Search 搜索记忆
	Search(ctx context.Context, query string, limit int) ([]MemoryItem, error)

	// Save 保存记忆到长期存储
	Save(ctx context.Context, sessionID string) error

	// Clear 清除会话记忆
	Clear(ctx context.Context, sessionID string) error

	// Close 关闭
	Close() error
}

// MemoryItem 记忆项
type MemoryItem struct {
	ID        string
	SessionID string
	Content   string
	Role      string
	Timestamp time.Time
	Embedding []float32
	Metadata  map[string]interface{}
}

// Config 记忆配置
type Config struct {
	// 短期记忆配置
	MaxHistory     int // 最大历史消息数
	MaxTokens      int // 最大 token 数
	EnableCompress bool // 是否启用自动压缩

	// 长期记忆配置
	EnableLongTerm bool   // 是否启用长期记忆
	StorageDir      string // 存储目录
	EmbeddingModel  string // 向量嵌入模型

	// 上下文管理
	ContextStrategy string // 上下文策略: full, summary, sliding
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxHistory:     100,
		MaxTokens:      4000,
		EnableCompress: true,
		EnableLongTerm: false,
		StorageDir:     "~/.myclawdbot/memory",
		EmbeddingModel: "default",
		ContextStrategy: "sliding",
	}
}

// Manager 记忆管理器
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionMemory
	config   *Config
	longTerm LongTermMemory
}

// NewManager 创建记忆管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	mgr := &Manager{
		sessions: make(map[string]*SessionMemory),
		config:   config,
	}

	// 如果启用长期记忆，初始化
	if config.EnableLongTerm {
		mgr.longTerm = NewSQLiteStorage(config.StorageDir)
	}

	return mgr
}

// GetSession 获取会话记忆
func (m *Manager) GetSession(sessionID string) *SessionMemory {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[sessionID]; ok {
		return sess
	}

	sess := NewSessionMemory(sessionID, m.config, m.longTerm)
	m.sessions[sessionID] = sess
	return sess
}

// DeleteSession 删除会话记忆
func (m *Manager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[sessionID]; ok {
		sess.Close()
	}
	delete(m.sessions, sessionID)
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sess := range m.sessions {
		sess.Close()
	}

	if m.longTerm != nil {
		return m.longTerm.Close()
	}
	return nil
}

// SessionMemory 会话记忆
type SessionMemory struct {
	sessionID    string
	config       *Config
	longTerm     LongTermMemory
	messages     []types.Message
	mu           sync.RWMutex
	summary      string // 对话摘要
	lastSaveTime time.Time
}

// NewSessionMemory 创建会话记忆
func NewSessionMemory(sessionID string, config *Config, longTerm LongTermMemory) *SessionMemory {
	return &SessionMemory{
		sessionID: sessionID,
		config:    config,
		longTerm:  longTerm,
		messages:  make([]types.Message, 0, config.MaxHistory),
	}
}

// Add 添加消息
func (s *SessionMemory) Add(ctx context.Context, msg *types.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, *msg)

	// 检查是否需要压缩
	if s.config.EnableCompress && len(s.messages) > s.config.MaxHistory/2 {
		s.compressIfNeeded()
	}

	return nil
}

// Get 获取消息
func (s *SessionMemory) Get(limit int) []types.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.messages) {
		limit = len(s.messages)
	}

	result := make([]types.Message, limit)
	copy(result, s.messages[len(s.messages)-limit:])
	return result
}

// GetAll 获取所有消息
func (s *SessionMemory) GetAll() []types.Message {
	return s.Get(0)
}

// Count 获取消息数量
func (s *SessionMemory) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}

// TokenCount 估算 token 数量
func (s *SessionMemory) TokenCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, msg := range s.messages {
		// 简单估算: 1 token ≈ 4 字符
		count += len(msg.Content) / 4
		// 加上角色前缀
		count += len(msg.Role) / 4
	}
	return count
}

// compressIfNeeded 如果需要则压缩
func (s *SessionMemory) compressIfNeeded() {
	if s.TokenCount() <= s.config.MaxTokens {
		return
	}

	// 保留摘要 + 最近的消息
	keepCount := s.config.MaxHistory / 2
	if keepCount > len(s.messages) {
		keepCount = len(s.messages)
	}

	// 保留最近的消息
	s.messages = s.messages[len(s.messages)-keepCount:]
}

// Clear 清除记忆
func (s *SessionMemory) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = make([]types.Message, 0, s.config.MaxHistory)
	s.summary = ""
}

// Close 关闭
func (s *SessionMemory) Close() {
	// 可以在这里保存到长期存储
	if s.longTerm != nil && len(s.messages) > 0 {
		ctx := context.Background()
		for _, msg := range s.messages {
			s.longTerm.Save(ctx, &MemoryItem{
				SessionID: s.sessionID,
				Content:   msg.Content,
				Role:      msg.Role,
				Timestamp: msg.Timestamp,
			})
		}
	}
}

// LongTermMemory 长期记忆接口
type LongTermMemory interface {
	// Save 保存记忆
	Save(ctx context.Context, item *MemoryItem) error

	// Search 搜索记忆
	Search(ctx context.Context, query string, limit int) ([]MemoryItem, error)

	// Delete 删除记忆
	Delete(ctx context.Context, id string) error

	// Close 关闭
	Close() error
}
