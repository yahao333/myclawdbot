package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/yahao333/myclawdbot/pkg/types"
)

// Registry 工具注册表
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// Tool 工具接口
type Tool interface {
	// Name 工具名称
	Name() string
	// Description 工具描述
	Description() string
	// Parameters 工具参数 schema
	Parameters() map[string]any
	// Execute 执行工具
	Execute(ctx context.Context, params map[string]any) (string, error)
}

// BaseTool 基础工具实现
type BaseTool struct {
	name        string
	description string
	parameters  map[string]any
	execute     func(ctx context.Context, params map[string]any) (string, error)
}

func (t *BaseTool) Name() string        { return t.name }
func (t *BaseTool) Description() string { return t.description }
func (t *BaseTool) Parameters() map[string]any { return t.parameters }
func (t *BaseTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	return t.execute(ctx, params)
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[tool.Name()]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name())
	}

	r.tools[tool.Name()] = tool
	return nil
}

// Get 获取工具
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// List 列出所有工具
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ToToolDefinitions 转换为工具定义
func (r *Registry) ToToolDefinitions() []types.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]types.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, types.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.Parameters(),
		})
	}
	return defs
}

// ExecuteTool 执行工具
func (r *Registry) ExecuteTool(ctx context.Context, name string, params map[string]any) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("tool %s not found", name)
	}

	return tool.Execute(ctx, params)
}

// DefaultRegistry 默认工具注册表
var defaultRegistry = NewRegistry()

// Register 注册工具到默认注册表
func Register(tool Tool) error {
	return defaultRegistry.Register(tool)
}

// Get 获取默认注册表中的工具
func Get(name string) (Tool, bool) {
	return defaultRegistry.Get(name)
}

// List 列出默认注册表中的所有工具
func List() []Tool {
	return defaultRegistry.List()
}

// ToToolDefinitions 获取默认注册表的工具定义
func ToToolDefinitions() []types.ToolDefinition {
	return defaultRegistry.ToToolDefinitions()
}

// Execute 执行默认注册表中的工具
func Execute(ctx context.Context, name string, params map[string]any) (string, error) {
	return defaultRegistry.ExecuteTool(ctx, name, params)
}
