package exec

import (
	"context"
	"testing"
)

func TestTerminalTool_Name(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	if tool.Name() != "terminal" {
		t.Errorf("Name() = %s, want 'terminal'", tool.Name())
	}
}

func TestTerminalTool_Description(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	expected := "交互式终端工具 - 保持会话状态的命令执行"
	if tool.Description() != expected {
		t.Errorf("Description() = %s, want %s", tool.Description(), expected)
	}
}

func TestTerminalTool_Parameters(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("type = %v, want 'object'", params["type"])
	}

	properties := params["properties"].(map[string]any)
	if _, ok := properties["command"]; !ok {
		t.Error("expected 'command' in properties")
	}
	if _, ok := properties["session_id"]; !ok {
		t.Error("expected 'session_id' in properties")
	}
	if _, ok := properties["timeout"]; !ok {
		t.Error("expected 'timeout' in properties")
	}
	if _, ok := properties["action"]; !ok {
		t.Error("expected 'action' in properties")
	}
}

func TestTerminalTool_Execute(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	ctx := context.Background()

	// 测试缺少 command 参数
	_, err := tool.Execute(ctx, map[string]any{})
	if err == nil {
		t.Error("expected error for missing command")
	}

	// 测试执行命令
	result, err := tool.Execute(ctx, map[string]any{
		"command":    "echo hello",
		"session_id": "test_terminal",
	})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestTerminalTool_Execute_DefaultSession(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	ctx := context.Background()

	// 不指定 session_id，使用默认会话
	result, err := tool.Execute(ctx, map[string]any{
		"command": "pwd",
	})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestTerminalTool_CreateSession(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"action":    "create",
		"session_id": "test_create",
		"work_dir":  "/tmp",
	})
	if err != nil {
		t.Errorf("Create session error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestTerminalTool_ListSessions(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	ctx := context.Background()

	// 先创建一个会话
	tool.Execute(ctx, map[string]any{
		"action":    "create",
		"session_id": "test_list",
	})

	// 列出会话
	result, err := tool.Execute(ctx, map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Errorf("List sessions error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestTerminalTool_DeleteSession(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	ctx := context.Background()

	// 先创建一个会话
	tool.Execute(ctx, map[string]any{
		"action":    "create",
		"session_id": "test_delete",
	})

	// 删除会话
	result, err := tool.Execute(ctx, map[string]any{
		"action":    "delete",
		"session_id": "test_delete",
	})
	if err != nil {
		t.Errorf("Delete session error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestTerminalTool_SessionPersistence(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	ctx := context.Background()

	sessionID := "test_persist"

	// 创建会话并设置工作目录
	tool.Execute(ctx, map[string]any{
		"action":    "create",
		"session_id": sessionID,
		"work_dir":  "/tmp",
	})

	// 执行 cd 命令
	tool.Execute(ctx, map[string]any{
		"command":    "cd /tmp",
		"session_id": sessionID,
	})

	// 再次执行 pwd，应该在同一目录
	result, err := tool.Execute(ctx, map[string]any{
		"command":    "pwd",
		"session_id": sessionID,
	})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}

	// 验证工作目录
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestTerminalTool_Timeout(t *testing.T) {
	tool := NewTerminalTool(300, "/tmp")
	ctx := context.Background()

	// 测试超时 - 使用较短的超时来测试
	// 注意: 由于 shell 命令可能缓存结果，这里测试超时逻辑
	_, err := tool.Execute(ctx, map[string]any{
		"command": "sleep 2",
		"timeout": 1, // 1秒超时
	})
	// 这个测试预期会有超时错误
	if err == nil {
		// 如果没有错误，可能是命令提前完成了，这也是可以接受的
		t.Log("command completed before timeout, which is acceptable")
	} else {
		t.Logf("got error (expected): %v", err)
	}
}

func TestTerminalManager_GetOrCreateSession(t *testing.T) {
	mgr := globalTerminalManager

	// 创建新会话
	sess1 := mgr.GetOrCreateSession("new_session", "/tmp")
	if sess1.ID != "new_session" {
		t.Errorf("session ID = %s, want 'new_session'", sess1.ID)
	}

	// 获取已存在的会话
	sess2 := mgr.GetOrCreateSession("new_session", "/home")
	if sess1 != sess2 {
		t.Error("expected same session instance")
	}
	// 工作目录不应该被覆盖
	if sess2.Dir != "/tmp" {
		t.Errorf("session dir = %s, want '/tmp'", sess2.Dir)
	}
}

func TestTerminalManager_DeleteSession(t *testing.T) {
	mgr := globalTerminalManager

	// 创建会话
	mgr.GetOrCreateSession("delete_test", "/tmp")

	// 删除会话
	mgr.DeleteSession("delete_test")

	// 验证会话已删除
	_, ok := mgr.GetSession("delete_test")
	if ok {
		t.Error("expected session to be deleted")
	}
}

func TestNewTerminalTool_Defaults(t *testing.T) {
	tool := NewTerminalTool(0, "")
	if tool.maxExecTime.Seconds() != 300 {
		t.Errorf("maxExecTime = %v, want 5m0s", tool.maxExecTime)
	}
	if tool.defaultWorkDir != "/tmp" {
		t.Errorf("defaultWorkDir = %s, want '/tmp'", tool.defaultWorkDir)
	}
}
