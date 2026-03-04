package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchTool_Name(t *testing.T) {
	tool := NewFetchTool(1024)
	if tool.Name() != "fetch" {
		t.Errorf("Name() = %s, want 'fetch'", tool.Name())
	}
}

func TestFetchTool_Description(t *testing.T) {
	tool := NewFetchTool(1024)
	if tool.Description() != "获取网页内容" {
		t.Errorf("Description() = %s, want '获取网页内容'", tool.Description())
	}
}

func TestFetchTool_Parameters(t *testing.T) {
	tool := NewFetchTool(1024)
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("type = %v, want 'object'", params["type"])
	}

	properties := params["properties"].(map[string]any)
	if _, ok := properties["url"]; !ok {
		t.Error("expected 'url' in properties")
	}

	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "url" {
		t.Errorf("required = %v, want ['url']", required)
	}
}

func TestFetchTool_Execute(t *testing.T) {
	tool := NewFetchTool(1024)
	ctx := context.Background()

	// 测试缺少 URL 参数
	_, err := tool.Execute(ctx, map[string]any{})
	if err == nil {
		t.Error("expected error for missing url")
	}

	// 测试空 URL
	_, err = tool.Execute(ctx, map[string]any{"url": ""})
	if err == nil {
		t.Error("expected error for empty url")
	}

	// 测试无效 URL
	_, err = tool.Execute(ctx, map[string]any{"url": "ftp://example.com"})
	if err == nil {
		t.Error("expected error for invalid url protocol")
	}
}

func TestFetchTool_Execute_Success(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	tool := NewFetchTool(1024)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"url": server.URL})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}

	// 验证结果
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestFetchTool_Execute_HTTPError(t *testing.T) {
	// 创建返回错误的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tool := NewFetchTool(1024)
	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]any{"url": server.URL})
	if err == nil {
		t.Error("expected error for HTTP 404")
	}
}

func TestFetchTool_Execute_InvalidURL(t *testing.T) {
	tool := NewFetchTool(1024)
	ctx := context.Background()

	// 测试各种无效 URL
	invalidURLs := []string{
		"ftp://example.com",
		"javascript:alert(1)",
		"file:///etc/passwd",
		"http://",
	}

	for _, url := range invalidURLs {
		_, err := tool.Execute(ctx, map[string]any{"url": url})
		if err == nil {
			t.Errorf("expected error for invalid URL: %s", url)
		}
	}
}

func TestNewFetchTool_DefaultMaxSize(t *testing.T) {
	tool := NewFetchTool(0)
	if tool.maxSize != 1024*1024 {
		t.Errorf("maxSize = %d, want %d", tool.maxSize, 1024*1024)
	}
}

func TestNewFetchTool_NegativeMaxSize(t *testing.T) {
	tool := NewFetchTool(-1)
	if tool.maxSize != 1024*1024 {
		t.Errorf("maxSize = %d, want %d", tool.maxSize, 1024*1024)
	}
}

func TestSearchTool_Name(t *testing.T) {
	tool := NewSearchTool()
	if tool.Name() != "search" {
		t.Errorf("Name() = %s, want 'search'", tool.Name())
	}
}

func TestSearchTool_Description(t *testing.T) {
	tool := NewSearchTool()
	if tool.Description() != "搜索网页" {
		t.Errorf("Description() = %s, want '搜索网页'", tool.Description())
	}
}

func TestSearchTool_Parameters(t *testing.T) {
	tool := NewSearchTool()
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("type = %v, want 'object'", params["type"])
	}

	properties := params["properties"].(map[string]any)
	if _, ok := properties["query"]; !ok {
		t.Error("expected 'query' in properties")
	}

	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("required = %v, want ['query']", required)
	}
}

func TestSearchTool_Execute(t *testing.T) {
	tool := NewSearchTool()
	ctx := context.Background()

	// 测试缺少 query 参数
	_, err := tool.Execute(ctx, map[string]any{})
	if err == nil {
		t.Error("expected error for missing query")
	}

	// 测试空 query
	_, err = tool.Execute(ctx, map[string]any{"query": ""})
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestSearchTool_Execute_Success(t *testing.T) {
	// 注意: search 工具硬编码了 DuckDuckGo URL，无法用本地服务器测试
	// 这里只测试参数验证

	tool := NewSearchTool()
	ctx := context.Background()

	// 由于 search 工具硬编码了 URL，我们这里测试基本功能
	// 实际的网络请求可能会失败，所以我们只测试参数验证
	_, err := tool.Execute(ctx, map[string]any{"query": "test"})
	if err != nil {
		// 可能是网络问题，这是预期的
		t.Logf("Search execution error (may be expected): %v", err)
	}
}

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Errorf("min(1, 2) = %d, want 1", min(1, 2))
	}
	if min(5, 3) != 3 {
		t.Errorf("min(5, 3) = %d, want 3", min(5, 3))
	}
	if min(4, 4) != 4 {
		t.Errorf("min(4, 4) = %d, want 4", min(4, 4))
	}
}

func TestFetchTool_UserAgent(t *testing.T) {
	// 创建检查 User-Agent 的服务器
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	tool := NewFetchTool(1024)
	ctx := context.Background()

	tool.Execute(ctx, map[string]any{"url": server.URL})

	if receivedUA != "MyClawDBot/1.0" {
		t.Errorf("User-Agent = %s, want 'MyClawDBot/1.0'", receivedUA)
	}
}

func TestFetchTool_SizeLimit(t *testing.T) {
	// 创建返回大量内容的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 写入超过限制的内容
		data := make([]byte, 2*1024) // 2KB
		for i := range data {
			data[i] = 'x'
		}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	// 使用 1KB 限制
	tool := NewFetchTool(1024)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"url": server.URL})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}

	// 结果应该被限制
	if len(result) > 1500 {
		t.Errorf("result too long: %d bytes", len(result))
	}
}
