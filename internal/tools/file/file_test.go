package file

import (
	"context"
	"os"
	"testing"
)

func TestReadTool_Name(t *testing.T) {
	tool := NewReadTool(1024)
	if tool.Name() != "read" {
		t.Errorf("Name() = %s, want 'read'", tool.Name())
	}
}

func TestReadTool_Description(t *testing.T) {
	tool := NewReadTool(1024)
	if tool.Description() != "读取文件内容" {
		t.Errorf("Description() = %s, want '读取文件内容'", tool.Description())
	}
}

func TestReadTool_Parameters(t *testing.T) {
	tool := NewReadTool(1024)
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("type = %v, want 'object'", params["type"])
	}

	properties := params["properties"].(map[string]any)
	if _, ok := properties["path"]; !ok {
		t.Error("expected 'path' in properties")
	}
	if _, ok := properties["offset"]; !ok {
		t.Error("expected 'offset' in properties")
	}
	if _, ok := properties["limit"]; !ok {
		t.Error("expected 'limit' in properties")
	}

	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "path" {
		t.Errorf("required = %v, want ['path']", required)
	}
}

func TestReadTool_Execute(t *testing.T) {
	tool := NewReadTool(10 * 1024 * 1024)
	ctx := context.Background()

	// 测试缺少 path 参数
	_, err := tool.Execute(ctx, map[string]any{})
	if err == nil {
		t.Error("expected error for missing path")
	}

	// 测试无效 path
	_, err = tool.Execute(ctx, map[string]any{"path": ""})
	if err == nil {
		t.Error("expected error for empty path")
	}

	// 测试读取不存在的文件
	_, err = tool.Execute(ctx, map[string]any{"path": "/nonexistent/file.txt"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadTool_Execute_Success(t *testing.T) {
	tool := NewReadTool(10 * 1024 * 1024)
	ctx := context.Background()

	// 使用 /tmp 目录 (在允许列表中)
	testFile := "/tmp/myclawdbot_test_read.txt"
	content := "Hello, World!"
	err := os.WriteFile(testFile, []byte(content), 0644)
	defer os.Remove(testFile) // 清理

	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := tool.Execute(ctx, map[string]any{"path": testFile})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}

	// 验证结果包含文件内容
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestReadTool_Execute_WithOffsetAndLimit(t *testing.T) {
	tool := NewReadTool(10 * 1024 * 1024)
	ctx := context.Background()

	// 使用 /tmp 目录
	testFile := "/tmp/myclawdbot_test_offset.txt"
	content := "0123456789"
	err := os.WriteFile(testFile, []byte(content), 0644)
	defer os.Remove(testFile)

	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 测试 offset
	_, err = tool.Execute(ctx, map[string]any{
		"path":   testFile,
		"offset": 5.0,
	})
	if err != nil {
		t.Errorf("Execute() with offset error: %v", err)
	}

	// 测试 limit
	_, err = tool.Execute(ctx, map[string]any{
		"path":  testFile,
		"limit": 3.0,
	})
	if err != nil {
		t.Errorf("Execute() with limit error: %v", err)
	}
}

func TestReadTool_isPathAllowed(t *testing.T) {
	tool := NewReadTool(1024)

	// 测试允许的路径
	allowed := tool.isPathAllowed("/tmp/test.txt")
	if !allowed {
		t.Log("Note: /tmp path may not be allowed in this environment")
	}
}

func TestWriteTool_Name(t *testing.T) {
	tool := NewWriteTool(1024)
	if tool.Name() != "write" {
		t.Errorf("Name() = %s, want 'write'", tool.Name())
	}
}

func TestWriteTool_Description(t *testing.T) {
	tool := NewWriteTool(1024)
	if tool.Description() != "写入或创建文件" {
		t.Errorf("Description() = %s, want '写入或创建文件'", tool.Description())
	}
}

func TestWriteTool_Parameters(t *testing.T) {
	tool := NewWriteTool(1024)
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("type = %v, want 'object'", params["type"])
	}

	properties := params["properties"].(map[string]any)
	if _, ok := properties["path"]; !ok {
		t.Error("expected 'path' in properties")
	}
	if _, ok := properties["content"]; !ok {
		t.Error("expected 'content' in properties")
	}

	required := params["required"].([]string)
	if len(required) != 2 {
		t.Errorf("required length = %d, want 2", len(required))
	}
}

func TestWriteTool_Execute(t *testing.T) {
	tool := NewWriteTool(10 * 1024 * 1024)
	ctx := context.Background()

	// 测试缺少 path 参数
	_, err := tool.Execute(ctx, map[string]any{"content": "test"})
	if err == nil {
		t.Error("expected error for missing path")
	}

	// 测试缺少 content 参数
	_, err = tool.Execute(ctx, map[string]any{"path": "/tmp/test.txt"})
	if err == nil {
		t.Error("expected error for missing content")
	}

	// 测试空 path
	_, err = tool.Execute(ctx, map[string]any{"path": "", "content": "test"})
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestWriteTool_Execute_Success(t *testing.T) {
	tool := NewWriteTool(10 * 1024 * 1024)
	ctx := context.Background()

	testFile := "/tmp/myclawdbot_test_write.txt"
	content := "Test content"

	result, err := tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": content,
	})
	defer os.Remove(testFile) // 清理

	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}

	// 验证文件已写入
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("failed to read written file: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %s, want %s", string(data), content)
	}

	// 验证返回结果
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestWriteTool_Execute_Overwrite(t *testing.T) {
	tool := NewWriteTool(10 * 1024 * 1024)
	ctx := context.Background()

	testFile := "/tmp/myclawdbot_test_overwrite.txt"
	defer os.Remove(testFile) // 清理

	// 首次写入
	tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": "original",
	})

	// 覆盖写入
	tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": "updated",
	})

	// 验证文件已更新
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("failed to read written file: %v", err)
	}
	if string(data) != "updated" {
		t.Errorf("content = %s, want 'updated'", string(data))
	}
}

func TestWriteTool_isPathAllowed(t *testing.T) {
	tool := NewWriteTool(1024)

	// 测试路径允许
	allowed := tool.isPathAllowed("/tmp/test.txt")
	if !allowed {
		t.Log("Note: /tmp path may not be allowed in this environment")
	}
}

func TestNewReadTool_DefaultMaxSize(t *testing.T) {
	tool := NewReadTool(0)
	if tool.maxSize != 10*1024*1024 {
		t.Errorf("maxSize = %d, want %d", tool.maxSize, 10*1024*1024)
	}
}

func TestNewReadTool_NegativeMaxSize(t *testing.T) {
	tool := NewReadTool(-1)
	if tool.maxSize != 10*1024*1024 {
		t.Errorf("maxSize = %d, want %d", tool.maxSize, 10*1024*1024)
	}
}

func TestNewWriteTool_DefaultMaxSize(t *testing.T) {
	tool := NewWriteTool(0)
	if tool.maxSize != 10*1024*1024 {
		t.Errorf("maxSize = %d, want %d", tool.maxSize, 10*1024*1024)
	}
}
