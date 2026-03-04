package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/internal/tools"
)

// TerminalSession 交互式终端会话
type TerminalSession struct {
	ID        string
	Dir       string    // 当前工作目录
	Env       []string  // 环境变量
	CreatedAt time.Time // 创建时间
	LastActive time.Time // 最后活跃时间
	mu        sync.Mutex
}

// TerminalManager 终端会话管理器
type TerminalManager struct {
	mu        sync.RWMutex
	sessions  map[string]*TerminalSession
	maxIdle   time.Duration // 最大空闲时间
}

var (
	globalTerminalManager = &TerminalManager{
		sessions: make(map[string]*TerminalSession),
		maxIdle:  10 * time.Minute, // 默认 10 分钟无活动则清理
	}
)

// GetOrCreateSession 获取或创建终端会话
func (m *TerminalManager) GetOrCreateSession(sessionID string, workDir string) *TerminalSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[sessionID]; ok {
		sess.LastActive = time.Now()
		return sess
	}

	// 获取默认工作目录
	if workDir == "" {
		workDir = "/tmp"
	}

	sess := &TerminalSession{
		ID:         sessionID,
		Dir:        workDir,
		Env:        os.Environ(),
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}
	m.sessions[sessionID] = sess
	return sess
}

// GetSession 获取会话
func (m *TerminalManager) GetSession(sessionID string) (*TerminalSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if ok {
		sess.LastActive = time.Now()
	}
	return sess, ok
}

// DeleteSession 删除会话
func (m *TerminalManager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
}

// ListSessions 列出所有会话
func (m *TerminalManager) ListSessions() []*TerminalSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*TerminalSession, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

// CleanupIdleSessions 清理空闲会话
func (m *TerminalManager) CleanupIdleSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, sess := range m.sessions {
		if now.Sub(sess.LastActive) > m.maxIdle {
			delete(m.sessions, id)
		}
	}
}

// ExecuteInSession 在会话中执行命令
func (sess *TerminalSession) Execute(ctx context.Context, command string, timeout int) (string, error) {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.LastActive = time.Now()

	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	// 创建命令 - 使用当前会话的工作目录
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = sess.Dir
	cmd.Env = sess.Env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 设置超时
	if timeout <= 0 {
		timeout = 60 // 默认 60 秒
	}
	if timeout > 300 {
		timeout = 300 // 最大 5 分钟
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	err := cmd.Run()

	output := stdout.String()
	if stderr.String() != "" {
		output += "\nSTDERR: " + stderr.String()
	}

	// 如果命令成功，更新工作目录 (cd 命令会改变目录)
	if err == nil && len(command) >= 2 && command[:2] == "cd" {
		// 获取新的工作目录
		newDir := exec.CommandContext(ctx, "sh", "-c", "pwd")
		newDir.Dir = sess.Dir
		newDir.Env = sess.Env
		if dirOutput, dirErr := newDir.Output(); dirErr == nil {
			sess.Dir = string(bytes.TrimSpace(dirOutput))
		}
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %d seconds", timeout)
		}
		return output + "\nerror: " + err.Error(), nil
	}

	return output, nil
}

// TerminalTool 交互式终端工具
type TerminalTool struct {
	manager       *TerminalManager
	maxExecTime   time.Duration
	defaultWorkDir string
}

func NewTerminalTool(maxExecTime int, defaultWorkDir string) *TerminalTool {
	if maxExecTime <= 0 {
		maxExecTime = 300
	}
	if defaultWorkDir == "" {
		defaultWorkDir = "/tmp"
	}

	return &TerminalTool{
		manager:       globalTerminalManager,
		maxExecTime:   time.Duration(maxExecTime) * time.Second,
		defaultWorkDir: defaultWorkDir,
	}
}

func (t *TerminalTool) Name() string        { return "terminal" }
func (t *TerminalTool) Description() string { return "交互式终端工具 - 保持会话状态的命令执行" }

func (t *TerminalTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "要执行的命令",
			},
			"session_id": map[string]any{
				"type":        "string",
				"description": "会话 ID，用于保持会话状态。不提供则使用默认会话",
				"default":     "default",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "超时时间（秒）",
				"default":     60,
			},
			"work_dir": map[string]any{
				"type":        "string",
				"description": "初始工作目录（仅在创建新会话时有效）",
			},
			"action": map[string]any{
				"type": "string",
				"enum": []string{"execute", "create", "delete", "list"},
				"description": "操作类型：execute(执行命令), create(创建会话), delete(删除会话), list(列出会话)",
				"default": "execute",
			},
		},
		"required": []string{"command"},
	}
}

func (t *TerminalTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	// 获取操作类型
	action := "execute"
	if a, ok := params["action"].(string); ok {
		action = a
	}

	switch action {
	case "create":
		return t.createSession(ctx, params)
	case "delete":
		return t.deleteSession(ctx, params)
	case "list":
		return t.listSessions(ctx, params)
	case "execute":
		fallthrough
	default:
		return t.executeCommand(ctx, params)
	}
}

func (t *TerminalTool) createSession(ctx context.Context, params map[string]any) (string, error) {
	sessionID := "default"
	if sid, ok := params["session_id"].(string); ok && sid != "" {
		sessionID = sid
	}

	workDir := t.defaultWorkDir
	if wd, ok := params["work_dir"].(string); ok && wd != "" {
		workDir = wd
	}

	sess := t.manager.GetOrCreateSession(sessionID, workDir)
	return fmt.Sprintf("会话已创建/更新:\nID: %s\n工作目录: %s\n创建时间: %s",
		sess.ID, sess.Dir, sess.CreatedAt.Format("2006-01-02 15:04:05")), nil
}

func (t *TerminalTool) deleteSession(ctx context.Context, params map[string]any) (string, error) {
	sessionID := "default"
	if sid, ok := params["session_id"].(string); ok && sid != "" {
		sessionID = sid
	}

	_, ok := t.manager.GetSession(sessionID)
	if !ok {
		return "", fmt.Errorf("session %s not found", sessionID)
	}

	t.manager.DeleteSession(sessionID)
	return fmt.Sprintf("会话 %s 已删除", sessionID), nil
}

func (t *TerminalTool) listSessions(ctx context.Context, params map[string]any) (string, error) {
	sessions := t.manager.ListSessions()
	if len(sessions) == 0 {
		return "暂无活跃会话", nil
	}

	var output string
	for _, sess := range sessions {
		output += fmt.Sprintf("ID: %s\n工作目录: %s\n创建时间: %s\n最后活跃: %s\n\n",
			sess.ID, sess.Dir, sess.CreatedAt.Format("2006-01-02 15:04:05"),
			sess.LastActive.Format("2006-01-02 15:04:05"))
	}
	return output, nil
}

func (t *TerminalTool) executeCommand(ctx context.Context, params map[string]any) (string, error) {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("command is required")
	}

	sessionID := "default"
	if sid, ok := params["session_id"].(string); ok && sid != "" {
		sessionID = sid
	}

	// 获取会话，不存在则创建
	sess := t.manager.GetOrCreateSession(sessionID, t.defaultWorkDir)

	// 超时设置
	timeout := 60
	if timeoutVal, ok := params["timeout"].(float64); ok {
		timeout = int(timeoutVal)
	}
	if timeout > int(t.maxExecTime.Seconds()) {
		timeout = int(t.maxExecTime.Seconds())
	}

	// 执行命令
	output, err := sess.Execute(ctx, command, timeout)
	if err != nil {
		return "", err
	}

	// 添加会话信息
	return fmt.Sprintf("[会话: %s | 目录: %s]\n%s", sess.ID, sess.Dir, output), nil
}

// init 注册终端工具
func init() {
	tools.Register(NewTerminalTool(300, "/tmp"))
}
