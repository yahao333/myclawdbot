package exec

import (
	"context"
	"testing"
	"time"
)

func TestCommandTool_Name(t *testing.T) {
	tool := NewCommandTool([]string{"go"}, 60)
	if tool.Name() != "bash" {
		t.Errorf("Name() = %s, want 'bash'", tool.Name())
	}
}

func TestCommandTool_Description(t *testing.T) {
	tool := NewCommandTool([]string{"go"}, 60)
	if tool.Description() != "执行 shell 命令" {
		t.Errorf("Description() = %s, want '执行 shell 命令'", tool.Description())
	}
}

func TestCommandTool_Parameters(t *testing.T) {
	tool := NewCommandTool([]string{"go"}, 60)
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("type = %v, want 'object'", params["type"])
	}

	properties := params["properties"].(map[string]any)
	if _, ok := properties["command"]; !ok {
		t.Error("expected 'command' in properties")
	}
	if _, ok := properties["timeout"]; !ok {
		t.Error("expected 'timeout' in properties")
	}

	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "command" {
		t.Errorf("required = %v, want ['command']", required)
	}
}

func TestCommandTool_Execute(t *testing.T) {
	tool := NewCommandTool([]string{"echo", "pwd", "ls"}, 60)
	ctx := context.Background()

	// 测试缺少 command 参数
	_, err := tool.Execute(ctx, map[string]any{})
	if err == nil {
		t.Error("expected error for missing command")
	}

	// 测试空 command
	_, err = tool.Execute(ctx, map[string]any{"command": ""})
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestCommandTool_Execute_AllowedCommand(t *testing.T) {
	tool := NewCommandTool([]string{"echo", "pwd", "ls"}, 60)
	ctx := context.Background()

	// 测试允许的命令
	result, err := tool.Execute(ctx, map[string]any{"command": "echo hello"})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestCommandTool_Execute_NotAllowedCommand(t *testing.T) {
	tool := NewCommandTool([]string{"e"}, 60)
	ctx := context.Background()

	// 测试不允许的命令
	_, err := tool.Execute(ctx, map[string]any{"command": "rm -rf /"})
	if err == nil {
		t.Error("expected error for not allowed command")
	}
}

func TestCommandTool_Execute_Timeout(t *testing.T) {
	tool := NewCommandTool([]string{"sleep"}, 1)
	ctx := context.Background()

	// 测试超时
	_, err := tool.Execute(ctx, map[string]any{
		"command": "sleep 10",
	})
	if err == nil {
		t.Error("expected error for timeout")
	}
}

func TestCommandTool_isCommandAllowed(t *testing.T) {
	tool := NewCommandTool([]string{"go", "git", "ls"}, 60)

	tests := []struct {
		command string
		want    bool
	}{
		{"go", true},
		{"git status", true},
		{"ls", true},
		{"ls -la", true},
		{"rm", false},
		{"cat /etc/passwd", false},
	}

	for _, tt := range tests {
		got := tool.isCommandAllowed(tt.command)
		if got != tt.want {
			t.Errorf("isCommandAllowed(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

func TestExtractCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"go test", "go"},
		{"ls -la", "ls"},
		{"git status", "git"},
		{"echo hello", "echo"},
		{"  echo hello", "echo"},
		{"echo", "echo"},
		{"\"echo\" hello", "echo"},
		{"'echo' hello", "echo"},
		{"", ""},
		{"\techo", "echo"},
		{"\necho\n", "echo"},
		{"echo\t\targ", "echo"},
	}

	for _, tt := range tests {
		got := extractCommand(tt.input)
		if got != tt.expected {
			t.Errorf("extractCommand(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNewCommandTool_Defaults(t *testing.T) {
	tool := NewCommandTool(nil, 0)

	// 验证默认命令列表
	if len(tool.allowedCommands) == 0 {
		t.Error("expected default allowed commands")
	}

	// 验证默认超时时间
	if tool.maxExecTime != 300*time.Second {
		t.Errorf("maxExecTime = %v, want 5m0s", tool.maxExecTime)
	}
}

func TestNewCommandTool_CustomValues(t *testing.T) {
	tool := NewCommandTool([]string{"custom"}, 60)

	if len(tool.allowedCommands) != 1 || tool.allowedCommands[0] != "custom" {
		t.Errorf("allowedCommands = %v, want ['custom']", tool.allowedCommands)
	}
	if tool.maxExecTime != 60*time.Second {
		t.Errorf("maxExecTime = %v, want 1m0s", tool.maxExecTime)
	}
}

func TestBashTool(t *testing.T) {
	// 测试 BashTool 是 CommandTool 的别名
	tool := NewBashTool([]string{"echo"}, 60)
	if tool.Name() != "bash" {
		t.Errorf("Name() = %s, want 'bash'", tool.Name())
	}
}

func TestCommandTool_Execute_CustomTimeout(t *testing.T) {
	tool := NewCommandTool([]string{"sleep"}, 60)
	ctx := context.Background()

	// 测试自定义超时（覆盖默认超时）
	_, err := tool.Execute(ctx, map[string]any{
		"command": "sleep 2",
		"timeout": 1.0,
	})
	if err == nil {
		t.Error("expected error for custom timeout")
	}
}

func TestCommandTool_Execute_Stderr(t *testing.T) {
	tool := NewCommandTool([]string{"ls"}, 60)
	ctx := context.Background()

	// 测试 stderr 输出
	result, err := tool.Execute(ctx, map[string]any{"command": "ls /nonexistent 2>&1"})
	if err == nil {
		// 错误会包含在输出中，不会返回 error
		t.Logf("result: %s", result)
	}
}

func TestCommandTool_Execute_MultipleCommands(t *testing.T) {
	tool := NewCommandTool([]string{"echo"}, 60)
	ctx := context.Background()

	// 测试多个命令
	result, err := tool.Execute(ctx, map[string]any{"command": "echo hello && echo world"})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}
