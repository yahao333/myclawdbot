package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yahao333/myclawdbot/pkg/types"
)

const (
	anthropicBaseURL = "https://api.anthropic.com"
	anthropicVersion = "2023-06-01"
)

// AnthropicClient Anthropic API 客户端
type AnthropicClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewAnthropicClient 创建 Anthropic 客户端
func NewAnthropicClient(apiKey, model, baseURL string) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic api key is required")
	}

	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	if baseURL == "" {
		baseURL = anthropicBaseURL
	}

	return &AnthropicClient{
		apiKey: apiKey,
		model:  model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Chat 发送聊天请求
func (c *AnthropicClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	url := c.baseURL + "/v1/messages"

	// 构建请求体
	body := c.buildRequestBody(req)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	// 解析响应
	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.convertResponse(&apiResp), nil
}

// StreamChat 发送流式聊天请求
func (c *AnthropicClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan *ChatResponse, error) {
	url := c.baseURL + "/v1/messages"

	body := c.buildRequestBody(req)
	body.Stream = true

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	ch := make(chan *ChatResponse, 10)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		decoder := json.NewDecoder(resp.Body)
		var currentContent string

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var event map[string]json.RawMessage
			if err := decoder.Decode(&event); err != nil {
				if err == io.EOF {
					return
				}
				continue
			}

			eventType, ok := event["type"]
			if !ok {
				continue
			}

			switch string(eventType) {
			case "content_block_delta":
				delta, ok := event["delta"]
				if !ok {
					continue
				}
				var deltaObj struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}
				if err := json.Unmarshal(delta, &deltaObj); err != nil {
					continue
				}
				if deltaObj.Type == "text_delta" {
					currentContent += deltaObj.Text
					ch <- &ChatResponse{
						Content: currentContent,
					}
				}
			case "message_stop":
				return
			}
		}
	}()

	return ch, nil
}

// Tools 获取可用工具
func (c *AnthropicClient) Tools() []types.ToolDefinition {
	// 默认工具由 tools 模块提供
	return nil
}

// Close 关闭客户端
func (c *AnthropicClient) Close() error {
	return nil
}

// buildRequestBody 构建请求体
func (c *AnthropicClient) buildRequestBody(req *ChatRequest) anthropicRequest {
	messages := make([]anthropicMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = anthropicMessage{
			Role: msg.Role,
			Content: msg.Content,
		}
	}

	tools := make([]anthropicTool, len(req.Tools))
	for i, tool := range req.Tools {
		tools[i] = anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	return anthropicRequest{
		Model:         c.model,
		Messages:      messages,
		System:        req.SystemPrompt,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		Tools:         tools,
		Stream:        false,
	}
}

// convertResponse 转换响应
func (c *AnthropicClient) convertResponse(resp *anthropicResponse) *ChatResponse {
	var content string
	var toolCalls []types.ToolCall

	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		} else if block.Type == "tool_use" {
			tc := types.ToolCall{
				ID:   block.ID,
				Name: block.Name,
				Args: block.Input,
			}
			toolCalls = append(toolCalls, tc)
		}
	}

	return &ChatResponse{
		ID:            resp.ID,
		Model:         resp.Model,
		Content:       content,
		ToolCalls:     toolCalls,
		StopReason:    resp.StopReason,
		InputTokens:   resp.Usage.InputTokens,
		OutputTokens:  resp.Usage.OutputTokens,
	}
}

// API 请求/响应类型
type anthropicRequest struct {
	Model       string            `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string            `json:"system,omitempty"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature,omitempty"`
	Tools       []anthropicTool   `json:"tools,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

type anthropicResponse struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Role       string            `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	Model      string            `json:"model"`
	StopReason string            `json:"stop_reason"`
	Usage      anthropicUsage    `json:"usage"`
}

type anthropicContentBlock struct {
	Type    string         `json:"type"`
	Text    string         `json:"text,omitempty"`
	ID      string         `json:"id,omitempty"`
	Name    string         `json:"name,omitempty"`
	Input   map[string]any `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
