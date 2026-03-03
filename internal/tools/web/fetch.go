package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yahao333/myclawdbot/internal/tools"
)

// FetchTool 网页获取工具
type FetchTool struct {
	httpClient *http.Client
	maxSize    int64
}

func NewFetchTool(maxSize int64) *FetchTool {
	if maxSize <= 0 {
		maxSize = 1024 * 1024 // 默认 1MB
	}

	return &FetchTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxSize: maxSize,
	}
}

func (t *FetchTool) Name() string        { return "fetch" }
func (t *FetchTool) Description() string { return "获取网页内容" }
func (t *FetchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "要获取的 URL",
			},
		},
		"required": []string{"url"},
	}
}

func (t *FetchTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	url, ok := params["url"].(string)
	if !ok || url == "" {
		return "", fmt.Errorf("url is required")
	}

	// 安全检查：只允许 http/https
	if len(url) < 8 || (url[:7] != "http://" && url[:8] != "https://") {
		return "", fmt.Errorf("url must start with http:// or https://")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 User-Agent
	req.Header.Set("User-Agent", "MyClawDBot/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// 限制读取大小
	reader := io.LimitReader(resp.Body, t.maxSize)

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return fmt.Sprintf("URL: %s\n状态码: %d\n内容长度: %d bytes\n\n%s",
		url, resp.StatusCode, len(content), string(content)), nil
}

// SearchTool 网页搜索工具
type SearchTool struct {
	httpClient *http.Client
}

func NewSearchTool() *SearchTool {
	return &SearchTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *SearchTool) Name() string        { return "search" }
func (t *SearchTool) Description() string { return "搜索网页" }
func (t *SearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "搜索关键词",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required")
	}

	// 简化实现：使用 DuckDuckGo HTML 搜索
	url := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", query)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "MyClawDBot/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search error: %d %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, 50000))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// 简化：返回原始 HTML 的前一部分
	// 实际应该解析 HTML 提取搜索结果
	return fmt.Sprintf("搜索: %s\n\n搜索结果 (简化版):\n%s", query, string(content[:min(len(content), 2000)])), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// init 注册工具
func init() {
	tools.Register(NewFetchTool(1024 * 1024))
	tools.Register(NewSearchTool())
}
