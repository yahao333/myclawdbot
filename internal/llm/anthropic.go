package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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
		apiKey:  apiKey,
		model:   model,
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

	debug := streamDebugEnabled()
	logf := func(format string, args ...any) {
		if debug {
			fmt.Printf(format, args...)
		}
	}
	logln := func(args ...any) {
		if debug {
			fmt.Println(args...)
		}
	}

	logf("[anthropic-stream] request start url=%s model=%s\n", url, c.model)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logf("[anthropic-stream] request error: %v\n", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	logf("[anthropic-stream] response status=%s content-type=%s\n", resp.Status, resp.Header.Get("Content-Type"))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logf("[anthropic-stream] non-200 body=%s\n", string(body))
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	ch := make(chan *ChatResponse, 10)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 1024), 4*1024*1024)
		var currentContent string
		dataLines := make([]string, 0, 4)

		flushEvent := func() bool {
			if len(dataLines) == 0 {
				return true
			}

			data := strings.Join(dataLines, "\n")
			dataLines = dataLines[:0]
			data = strings.TrimSpace(data)
			if data == "" || data == "[DONE]" {
				logln("[anthropic-stream] skip empty or done event payload")
				return true
			}
			logf("[anthropic-stream] event payload bytes=%d\n", len(data))

			var event struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				logf("[anthropic-stream] unmarshal event failed: %v payload=%s\n", err, data)
				return true
			}
			logf("[anthropic-stream] event type=%s delta_type=%s\n", event.Type, event.Delta.Type)

			switch event.Type {
			case "content_block_delta":
				if event.Delta.Type == "text_delta" {
					currentContent += event.Delta.Text
					logf("[anthropic-stream] emit text_delta bytes=%d total_bytes=%d\n", len(event.Delta.Text), len(currentContent))
					ch <- &ChatResponse{Content: currentContent}
				}
			case "content_block_stop":
				logln("[anthropic-stream] content block finished, wait next block")
			case "message_stop":
				logln("[anthropic-stream] message stop received")
				return false
			}

			return true
		}

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				logln("[anthropic-stream] context canceled")
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				if !flushEvent() {
					return
				}
				continue
			}
			if strings.HasPrefix(line, "event:") {
				continue
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			dataLines = append(dataLines, data)
		}
		if err := scanner.Err(); err != nil {
			logf("[anthropic-stream] scanner error: %v\n", err)
		}

		if len(dataLines) > 0 {
			flushEvent()
		}
		logln("[anthropic-stream] stream loop finished")
	}()

	return ch, nil
}

func streamDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LLM_DEBUG_STREAM"))) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
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
			Role:    msg.Role,
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
		Model:       c.model,
		Messages:    messages,
		System:      req.SystemPrompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       tools,
		Stream:      false,
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
		ID:           resp.ID,
		Model:        resp.Model,
		Content:      content,
		ToolCalls:    toolCalls,
		StopReason:   resp.StopReason,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}
}

// API 请求/响应类型
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
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
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	Model      string                  `json:"model"`
	StopReason string                  `json:"stop_reason"`
	Usage      anthropicUsage          `json:"usage"`
}

type anthropicContentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
