package tools

import (
	"context"
	"testing"

	"github.com/yahao333/myclawdbot/pkg/types"
)

// mockTool 用于测试的工具实现
type mockTool struct {
	name        string
	description string
	parameters  map[string]any
	execute     func(ctx context.Context, params map[string]any) (string, error)
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return t.description }
func (t *mockTool) Parameters() map[string]any { return t.parameters }
func (t *mockTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	return t.execute(ctx, params)
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Error("NewRegistry() returned nil")
	}
	if registry.tools == nil {
		t.Error("tools map is nil")
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	// 注册新工具
	tool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		parameters:  map[string]any{"key": "value"},
		execute: func(ctx context.Context, params map[string]any) (string, error) {
			return "executed", nil
		},
	}

	err := registry.Register(tool)
	if err != nil {
		t.Errorf("Register() error: %v", err)
	}

	// 验证工具已注册
	got, ok := registry.Get("test_tool")
	if !ok {
		t.Error("Get() returned false for registered tool")
	}
	if got.Name() != "test_tool" {
		t.Errorf("Name() = %s, want 'test_tool'", got.Name())
	}

	// 测试重复注册
	err = registry.Register(tool)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	// 测试获取不存在的工具
	_, ok := registry.Get("nonexistent")
	if ok {
		t.Error("Get() returned true for nonexistent tool")
	}

	// 注册并获取
	tool := &mockTool{
		name: "get_test",
		execute: func(ctx context.Context, params map[string]any) (string, error) {
			return "result", nil
		},
	}
	registry.Register(tool)

	got, ok := registry.Get("get_test")
	if !ok {
		t.Error("Get() returned false for registered tool")
	}
	if got.Name() != "get_test" {
		t.Errorf("Name() = %s, want 'get_test'", got.Name())
	}
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	// 初始列表应该为空
	tools := registry.List()
	if len(tools) != 0 {
		t.Errorf("List() = %d, want 0", len(tools))
	}

	// 注册工具
	tool1 := &mockTool{name: "tool1", execute: func(ctx context.Context, params map[string]any) (string, error) { return "", nil }}
	tool2 := &mockTool{name: "tool2", execute: func(ctx context.Context, params map[string]any) (string, error) { return "", nil }}
	registry.Register(tool1)
	registry.Register(tool2)

	tools = registry.List()
	if len(tools) != 2 {
		t.Errorf("List() = %d, want 2", len(tools))
	}
}

func TestRegistry_ToToolDefinitions(t *testing.T) {
	registry := NewRegistry()

	tool := &mockTool{
		name:        "def_test",
		description: "Test tool for definitions",
		parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{"type": "string"},
			},
		},
		execute: func(ctx context.Context, params map[string]any) (string, error) { return "", nil },
	}
	registry.Register(tool)

	defs := registry.ToToolDefinitions()
	if len(defs) != 1 {
		t.Errorf("ToToolDefinitions() = %d, want 1", len(defs))
	}

	if defs[0].Name != "def_test" {
		t.Errorf("Name = %s, want 'def_test'", defs[0].Name)
	}
	if defs[0].Description != "Test tool for definitions" {
		t.Errorf("Description = %s, want 'Test tool for definitions'", defs[0].Description)
	}
}

func TestRegistry_ExecuteTool(t *testing.T) {
	registry := NewRegistry()

	// 测试执行不存在的工具
	_, err := registry.ExecuteTool(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}

	// 注册并执行
	executed := false
	tool := &mockTool{
		name: "exec_test",
		execute: func(ctx context.Context, params map[string]any) (string, error) {
			executed = true
			return "execution result", nil
		},
	}
	registry.Register(tool)

	result, err := registry.ExecuteTool(context.Background(), "exec_test", map[string]any{"key": "value"})
	if err != nil {
		t.Errorf("ExecuteTool() error: %v", err)
	}
	if result != "execution result" {
		t.Errorf("result = %s, want 'execution result'", result)
	}
	if !executed {
		t.Error("tool was not executed")
	}
}

func TestBaseTool(t *testing.T) {
	base := BaseTool{
		name:        "base_tool",
		description: "Base tool description",
		parameters:  map[string]any{"param": "value"},
		execute: func(ctx context.Context, params map[string]any) (string, error) {
			return "base_executed", nil
		},
	}

	if base.Name() != "base_tool" {
		t.Errorf("Name() = %s, want 'base_tool'", base.Name())
	}
	if base.Description() != "Base tool description" {
		t.Errorf("Description() = %s, want 'Base tool description'", base.Description())
	}

	result, err := base.Execute(context.Background(), nil)
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}
	if result != "base_executed" {
		t.Errorf("result = %s, want 'base_executed'", result)
	}
}

// 注意：这里测试的是默认注册表
// 由于 tools 包的 init() 会注册默认工具，所以默认注册表非空
func TestDefaultRegistry(t *testing.T) {
	// 测试默认注册表中的工具
	tools := List()
	if len(tools) == 0 {
		t.Log("default registry is empty")
	}

	// 测试获取工具
	for _, tool := range tools {
		if tool.Name() == "" {
			t.Error("tool name is empty")
		}
	}
}

func TestToToolDefinitions_Global(t *testing.T) {
	defs := ToToolDefinitions()
	// 检查是否有默认工具
	if len(defs) == 0 {
		t.Log("no default tools registered")
	}
}

func TestExecute_Global(t *testing.T) {
	// 测试执行默认工具（如果注册了的话）
	ctx := context.Background()

	// 尝试执行一个可能存在的工具
	_, err := Execute(ctx, "nonexistent_tool_xyz", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestTypesToolDefinition(t *testing.T) {
	def := types.ToolDefinition{
		Name:        "test",
		Description: "test description",
		InputSchema: map[string]any{"type": "object"},
	}

	if def.Name != "test" {
		t.Errorf("Name = %s, want 'test'", def.Name)
	}
	if def.Description != "test description" {
		t.Errorf("Description = %s, want 'test description'", def.Description)
	}
}
