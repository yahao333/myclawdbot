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
	openAIBaseURL = "https://api.openai.com"
)

// OpenAIClient OpenAI API 客户端
type OpenAIClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewOpenAIClient 创建 OpenAI 客户端
func NewOpenAIClient(apiKey, model, baseURL string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("openai api key is required")
	}

	if model == "" {
		model = "gpt-4o"
	}

	if baseURL == "" {
		baseURL = openAIBaseURL
	}

	return &OpenAIClient{
		apiKey: apiKey,
		model:  model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Chat 发送聊天请求
func (c *OpenAIClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	url := c.baseURL + "/v1/chat/completions"

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

	var apiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.convertResponse(&apiResp), nil
}

// StreamChat 发送流式聊天请求
func (c *OpenAIClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan *ChatResponse, error) {
	url := c.baseURL + "/v1/chat/completions"

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

			var delta openaiChunk
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
func (c *OpenAIClient) Tools() []types.ToolDefinition {
	return nil
}

// Close 关闭客户端
func (c *OpenAIClient) Close() error {
	return nil
}

// buildRequestBody 构建请求体
func (c *OpenAIClient) buildRequestBody(req *ChatRequest) openaiRequest {
	messages := make([]openaiMessage, len(req.Messages))
	for i, msg := range req.Messages {
		content := msg.Content
		toolCalls := make([]openaiToolCall, 0)

		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, openaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: openaiFunction{
					Name:       tc.Name,
					Arguments:  mustMarshalJSON(tc.Args),
				},
			})
		}

		messages[i] = openaiMessage{
			Role:      msg.Role,
			Content:   content,
			ToolCalls: toolCalls,
		}
	}

	tools := make([]openaiTool, len(req.Tools))
	for i, tool := range req.Tools {
		tools[i] = openaiTool{
			Type: "function",
			Function: openaiFunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}

	return openaiRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Tools:       tools,
		Stream:      false,
	}
}

// convertResponse 转换响应
func (c *OpenAIClient) convertResponse(resp *openaiResponse) *ChatResponse {
	if len(resp.Choices) == 0 {
		return &ChatResponse{}
	}

	choice := resp.Choices[0]
	var content string
	var toolCalls []types.ToolCall

	if choice.Message.Content != "" {
		content = choice.Message.Content
	}

	for _, tc := range choice.Message.ToolCalls {
		argsMap := make(map[string]any)
		json.Unmarshal([]byte(tc.Function.Arguments), &argsMap)

		toolCalls = append(toolCalls, types.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: argsMap,
		})
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

func mustMarshalJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// API 请求/响应类型
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Tools       []openaiTool   `json:"tools,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiToolCall struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type      string                 `json:"type"`
	Function  openaiFunctionDefinition `json:"function"`
}

type openaiFunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  any         `json:"parameters"`
}

type openaiResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64           `json:"created"`
	Model   string           `json:"model"`
	Choices []openaiChoice  `json:"choices"`
	Usage   openaiUsage     `json:"usage"`
}

type openaiChoice struct {
	Index        int          `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

type openaiChunk struct {
	Choices []openaiChunkChoice `json:"choices"`
}

type openaiChunkChoice struct {
	Index        int            `json:"index"`
	Delta        openaiMessage  `json:"delta"`
	FinishReason string         `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
