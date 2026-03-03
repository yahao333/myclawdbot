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
	minimaxBaseURL = "https://api.minimax.chat"
)

// MinimaxClient Minimax API 客户端
type MinimaxClient struct {
	apiKey     string
	model      string
	baseURL    string
	groupID    string
	httpClient *http.Client
}

// NewMinimaxClient 创建 Minimax 客户端
func NewMinimaxClient(apiKey, model, baseURL string) (*MinimaxClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("minimax api key is required")
	}

	if model == "" {
		model = "MiniMax-Text-01"
	}

	if baseURL == "" {
		baseURL = minimaxBaseURL
	}

	return &MinimaxClient{
		apiKey: apiKey,
		model:  model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Chat 发送聊天请求
func (c *MinimaxClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/v1/text/chatcompletion_v2?GroupId=%s", c.baseURL, c.groupID)

	// 构建请求体
	body := c.buildRequestBody(req)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	var apiResp minimaxResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.convertResponse(&apiResp), nil
}

// StreamChat 发送流式聊天请求
func (c *MinimaxClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan *ChatResponse, error) {
	url := fmt.Sprintf("%s/v1/text/chatcompletion_v2?GroupId=%s", c.baseURL, c.groupID)

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
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
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
			line, err := decoder.Token()
			if err != nil {
				if err == io.EOF {
					return
				}
				continue
			}

			text, ok := line.(string)
			if !ok || text == "" {
				continue
			}

			if text == "data: [DONE]" {
				return
			}

			if len(text) < 6 || text[:6] != "data: " {
				continue
			}

			var delta minimaxChunk
			if err := json.Unmarshal([]byte(text[6:]), &delta); err != nil {
				continue
			}

			if len(delta.Choices) > 0 && delta.Choices[0].Delta.Content != "" {
				currentContent += delta.Choices[0].Delta.Content
				ch <- &ChatResponse{
					Content: currentContent,
				}
			}

			if len(delta.Choices) > 0 && delta.Choices[0].FinishReason != "" {
				return
			}
		}
	}()

	return ch, nil
}

// Tools 获取可用工具
func (c *MinimaxClient) Tools() []types.ToolDefinition {
	return nil
}

// Close 关闭客户端
func (c *MinimaxClient) Close() error {
	return nil
}

// SetGroupID 设置 GroupID
func (c *MinimaxClient) SetGroupID(groupID string) {
	c.groupID = groupID
}

// buildRequestBody 构建请求体
func (c *MinimaxClient) buildRequestBody(req *ChatRequest) minimaxRequest {
	messages := make([]minimaxMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = minimaxMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	tools := make([]minimaxTool, len(req.Tools))
	for i, tool := range req.Tools {
		tools[i] = minimaxTool{
			Type: "function",
			Function: minimaxFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}

	return minimaxRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Tools:       tools,
		Stream:      false,
	}
}

// convertResponse 转换响应
func (c *MinimaxClient) convertResponse(resp *minimaxResponse) *ChatResponse {
	if len(resp.Choices) == 0 {
		return &ChatResponse{}
	}

	choice := resp.Choices[0]
	var content string
	var toolCalls []types.ToolCall

	if choice.Message.Content != "" {
		content = choice.Message.Content
	}

	return &ChatResponse{
		ID:            resp.ID,
		Model:         resp.Model,
		Content:       content,
		ToolCalls:     toolCalls,
		StopReason:    choice.FinishReason,
		InputTokens:   resp.Usage.PromptTokens,
		OutputTokens:  resp.Usage.CompletionTokens,
	}
}

// API 请求/响应类型
type minimaxRequest struct {
	Model       string            `json:"model"`
	Messages    []minimaxMessage `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Tools       []minimaxTool   `json:"tools,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type minimaxMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type minimaxTool struct {
	Type      string          `json:"type"`
	Function  minimaxFunction `json:"function"`
}

type minimaxFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  any         `json:"parameters"`
}

type minimaxResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64            `json:"created"`
	Model   string            `json:"model"`
	Choices []minimaxChoice  `json:"choices"`
	Usage   minimaxUsage     `json:"usage"`
}

type minimaxChoice struct {
	Index        int             `json:"index"`
	Message      minimaxMessage  `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type minimaxChunk struct {
	Choices []minimaxChunkChoice `json:"choices"`
}

type minimaxChunkChoice struct {
	Index        int            `json:"index"`
	Delta        minimaxMessage `json:"delta"`
	FinishReason string         `json:"finish_reason"`
}

type minimaxUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
