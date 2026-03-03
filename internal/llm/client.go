package llm

import (
	"context"

	"github.com/yahao333/myclawdbot/pkg/types"
)

// Client LLM 客户端接口
type Client interface {
	// Chat 发送聊天请求
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChat 发送流式聊天请求
	StreamChat(ctx context.Context, req *ChatRequest) (<-chan *ChatResponse, error)

	// Tools 获取可用工具列表
	Tools() []types.ToolDefinition

	// Close 关闭客户端
	Close() error
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string
	Messages    []types.Message
	SystemPrompt string
	MaxTokens   int
	Temperature float64
	Tools       []types.ToolDefinition
}

// ChatResponse 聊天响应
type ChatResponse struct {
	ID            string
	Model         string
	Content       string
	ToolCalls     []types.ToolCall
	StopReason    string
	InputTokens   int
	OutputTokens  int
}

// NewClient 创建 LLM 客户端
// groupID 是 Minimax 所需的 GroupId 参数
func NewClient(provider, apiKey, model, baseURL, groupID string) (Client, error) {
	switch provider {
	case "anthropic":
		return NewAnthropicClient(apiKey, model, baseURL)
	case "openai":
		return NewOpenAIClient(apiKey, model, baseURL)
	case "minimax":
		client, err := NewMinimaxClient(apiKey, model, baseURL)
		if err != nil {
			return nil, err
		}
		if groupID != "" {
			client.SetGroupID(groupID)
		}
		return client, nil
	default:
		return NewAnthropicClient(apiKey, model, baseURL)
	}
}
