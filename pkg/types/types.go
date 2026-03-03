package types

import "time"

// Message 消息结构
type Message struct {
	Role      string    `json:"role"`       // user, assistant, system
	Content   string    `json:"content"`    // 消息内容
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // 工具调用
	Timestamp time.Time `json:"timestamp"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Args     map[string]any `json:"args"`
	Result   string          `json:"result,omitempty"` // 工具执行结果
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

// ContentBlock 内容块
type ContentBlock struct {
	Type string `json:"type"` // text, tool_use, tool_result
	Text string `json:"text,omitempty"`

	// tool_use
	ToolUse *ToolUseBlock `json:"tool_use,omitempty"`

	// tool_result
	ToolResult *ToolResultBlock `json:"tool_result,omitempty"`
}

// ToolUseBlock 工具使用块
type ToolUseBlock struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Input map[string]any `json:"input"`
}

// ToolResultBlock 工具结果块
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// StreamDelta 流式响应增量
type StreamDelta struct {
	Type         string `json:"type"` // content_block_delta, message_stop
	Index        int    `json:"index,omitempty"`
	Delta        string `json:"delta,omitempty"`
	ContentBlock *ContentBlock `json:"content_block,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
