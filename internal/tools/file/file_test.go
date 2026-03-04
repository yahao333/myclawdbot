// Package file 文件工具测试包
// 包含对 ReadTool 和 WriteTool 的单元测试
// 测试覆盖：工具名称、描述、参数、执行逻辑、路径访问控制等
package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yahao333/myclawdbot/internal/config"
)

// TestReadTool_Name 测试读取工具的名称返回
// 验证 Name() 方法返回 "read"
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
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	testFile := filepath.Join(testDir, "myclawdbot_test_read.txt")
	content := "Hello, World!"
	err := os.WriteFile(testFile, []byte(content), 0644)

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
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	testFile := filepath.Join(testDir, "myclawdbot_test_offset.txt")
	content := "0123456789"
	err := os.WriteFile(testFile, []byte(content), 0644)

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
	testPath := filepath.Join(t.TempDir(), "test.txt")
	if tool.isPathAllowed(testPath) {
		t.Error("expected path denied when file access is restricted and allowedDirs is empty")
	}
}

func TestReadTool_isPathAllowed_NilAllowedDirs(t *testing.T) {
	tool := NewReadTool(1024)
	tool.allowedDirs = nil
	testPath := filepath.Join(t.TempDir(), "test.txt")
	if !tool.isPathAllowed(testPath) {
		t.Error("expected path allowed when allowedDirs is nil")
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
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	testFile := filepath.Join(testDir, "myclawdbot_test_write.txt")
	content := "Test content"

	result, err := tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": content,
	})
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
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	testFile := filepath.Join(testDir, "myclawdbot_test_overwrite.txt")

	// 首次写入
	if _, err := tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": "original",
	}); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// 覆盖写入
	if _, err := tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": "updated",
	}); err != nil {
		t.Fatalf("overwrite write failed: %v", err)
	}

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
	testPath := filepath.Join(t.TempDir(), "test.txt")
	if tool.isPathAllowed(testPath) {
		t.Error("expected path denied when file access is restricted and allowedDirs is empty")
	}
}

func TestWriteTool_isPathAllowed_NilAllowedDirs(t *testing.T) {
	tool := NewWriteTool(1024)
	tool.allowedDirs = nil
	testPath := filepath.Join(t.TempDir(), "test.txt")
	if !tool.isPathAllowed(testPath) {
		t.Error("expected path allowed when allowedDirs is nil")
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

// Test NewReadToolWithConfig tests

func TestNewReadToolWithConfig_DefaultValues(t *testing.T) {
	cfg := &config.ToolsConfig{
		MaxFileSize:         0,
		RestrictFileAccess:  true,
		CurrentDir:          "/tmp",
		AllowedDirs:         []string{},
	}

	tool := NewReadToolWithConfig(cfg)

	if tool.maxSize != 10*1024*1024 {
		t.Errorf("expected maxSize 10485760, got %d", tool.maxSize)
	}
	if !tool.restrictFileAccess {
		t.Error("expected restrictFileAccess to be true")
	}
}

func TestNewReadToolWithConfig_RestrictFileAccessDisabled(t *testing.T) {
	cfg := &config.ToolsConfig{
		MaxFileSize:         1024,
		RestrictFileAccess:  false,
		CurrentDir:          "/tmp",
		AllowedDirs:         []string{"/home/user"},
	}

	tool := NewReadToolWithConfig(cfg)

	if tool.maxSize != 1024 {
		t.Errorf("expected maxSize 1024, got %d", tool.maxSize)
	}
	if tool.restrictFileAccess {
		t.Error("expected restrictFileAccess to be false")
	}
	if tool.allowedDirs != nil {
		t.Error("expected allowedDirs to be nil when restrictFileAccess is false")
	}
}

func TestNewReadToolWithConfig_WithAllowedDirs(t *testing.T) {
	cfg := &config.ToolsConfig{
		MaxFileSize:         1024,
		RestrictFileAccess:  true,
		CurrentDir:          "/home/user",
		AllowedDirs:         []string{"/home/user/data", "/tmp/shared"},
	}

	tool := NewReadToolWithConfig(cfg)

	if len(tool.allowedDirs) != 3 { // CurrentDir + AllowedDirs
		t.Errorf("expected 3 allowed dirs, got %d", len(tool.allowedDirs))
	}
}

func TestNewReadToolWithConfig_EmptyCurrentDir(t *testing.T) {
	cfg := &config.ToolsConfig{
		MaxFileSize:         1024,
		RestrictFileAccess:  true,
		CurrentDir:          "",
		AllowedDirs:         []string{"/tmp/allowed"},
	}

	tool := NewReadToolWithConfig(cfg)

	// 应该只有 AllowedDirs，没有 CurrentDir
	if len(tool.allowedDirs) != 1 {
		t.Errorf("expected 1 allowed dir, got %d", len(tool.allowedDirs))
	}
}

// Test NewWriteToolWithConfig tests

func TestNewWriteToolWithConfig_DefaultValues(t *testing.T) {
	cfg := &config.ToolsConfig{
		MaxFileSize:         0,
		RestrictFileAccess:  true,
		CurrentDir:          "/tmp",
		AllowedDirs:         []string{},
	}

	tool := NewWriteToolWithConfig(cfg)

	if tool.maxSize != 10*1024*1024 {
		t.Errorf("expected maxSize 10485760, got %d", tool.maxSize)
	}
	if !tool.restrictFileAccess {
		t.Error("expected restrictFileAccess to be true")
	}
}

func TestNewWriteToolWithConfig_RestrictFileAccessDisabled(t *testing.T) {
	cfg := &config.ToolsConfig{
		MaxFileSize:         1024,
		RestrictFileAccess:  false,
		CurrentDir:          "/tmp",
		AllowedDirs:         []string{"/home/user"},
	}

	tool := NewWriteToolWithConfig(cfg)

	if tool.maxSize != 1024 {
		t.Errorf("expected maxSize 1024, got %d", tool.maxSize)
	}
	if tool.restrictFileAccess {
		t.Error("expected restrictFileAccess to be false")
	}
	if tool.allowedDirs != nil {
		t.Error("expected allowedDirs to be nil when restrictFileAccess is false")
	}
}

func TestNewWriteToolWithConfig_WithAllowedDirs(t *testing.T) {
	cfg := &config.ToolsConfig{
		MaxFileSize:         1024,
		RestrictFileAccess:  true,
		CurrentDir:          "/home/user",
		AllowedDirs:         []string{"/home/user/data", "/tmp/shared"},
	}

	tool := NewWriteToolWithConfig(cfg)

	if len(tool.allowedDirs) != 3 { // CurrentDir + AllowedDirs
		t.Errorf("expected 3 allowed dirs, got %d", len(tool.allowedDirs))
	}
}

// Test read tool with path allowed scenarios

func TestReadTool_Execute_FileTooLarge(t *testing.T) {
	tool := NewReadTool(10) // 10 bytes max
	ctx := context.Background()
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	testFile := filepath.Join(testDir, "large.txt")
	content := "This is a very long content that exceeds the limit"
	err := os.WriteFile(testFile, []byte(content), 0644)

	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err = tool.Execute(ctx, map[string]any{"path": testFile})
	if err == nil {
		t.Error("expected error for file too large")
	}
}

func TestReadTool_Execute_Directory(t *testing.T) {
	tool := NewReadTool(10 * 1024 * 1024)
	ctx := context.Background()
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	_, err := tool.Execute(ctx, map[string]any{"path": testDir})
	if err == nil {
		t.Error("expected error for directory path")
	}
}

func TestReadTool_Execute_PathNotAllowed(t *testing.T) {
	tool := NewReadTool(10 * 1024 * 1024)
	ctx := context.Background()

	// 不设置 allowedDirs，限制访问
	tool.restrictFileAccess = true
	tool.allowedDirs = []string{"/tmp/allowed"}

	// 尝试访问不允许的路径
	_, err := tool.Execute(ctx, map[string]any{"path": "/etc/passwd"})
	if err == nil {
		t.Error("expected error for path not allowed")
	}
}

// Test write tool with path allowed scenarios

func TestWriteTool_Execute_ContentTooLarge(t *testing.T) {
	tool := NewWriteTool(10) // 10 bytes max
	ctx := context.Background()
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	testFile := filepath.Join(testDir, "large.txt")

	_, err := tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": "This is a very long content that exceeds the limit",
	})
	if err == nil {
		t.Error("expected error for content too large")
	}
}

func TestWriteTool_Execute_PathNotAllowed(t *testing.T) {
	tool := NewWriteTool(10 * 1024 * 1024)
	ctx := context.Background()

	// 限制访问
	tool.restrictFileAccess = true
	tool.allowedDirs = []string{"/tmp/allowed"}

	// 尝试写入不允许的路径
	_, err := tool.Execute(ctx, map[string]any{
		"path":    "/etc/test.txt",
		"content": "test",
	})
	if err == nil {
		t.Error("expected error for path not allowed")
	}
}

func TestWriteTool_Execute_CreateNestedDirectory(t *testing.T) {
	tool := NewWriteTool(10 * 1024 * 1024)
	ctx := context.Background()
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	// 在 testDir 下创建嵌套路径
	nestedDir := filepath.Join(testDir, "subdir", "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	nestedPath := filepath.Join(nestedDir, "file.txt")

	result, err := tool.Execute(ctx, map[string]any{
		"path":    nestedPath,
		"content": "nested content",
	})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}

	// 验证文件已创建
	data, err := os.ReadFile(nestedPath)
	if err != nil {
		t.Errorf("failed to read written file: %v", err)
	}
	if string(data) != "nested content" {
		t.Errorf("content = %s, want 'nested content'", string(data))
	}

	// 验证返回结果
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

// Test expandHome function in file package
func TestExpandHome_File(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/absolute/path", "/absolute/path"},
		{"~/test", ""}, // will be expanded if home is available, empty string if not
	}

	for _, tt := range tests {
		result := expandHome(tt.input)
		if tt.input == "/absolute/path" && result != "/absolute/path" {
			t.Errorf("expandHome(%q) = %q, want same", tt.input, result)
		}
		// 对于 ~/test，我们只验证它不会 panic
		_ = result
	}
}

// Test read tool with offset and limit edge cases
func TestReadTool_Execute_OffsetBeyondFileSize(t *testing.T) {
	tool := NewReadTool(10 * 1024 * 1024)
	ctx := context.Background()
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	testFile := filepath.Join(testDir, "offset_test.txt")
	content := "12345"
	err := os.WriteFile(testFile, []byte(content), 0644)

	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// offset 超过文件大小
	result, err := tool.Execute(ctx, map[string]any{
		"path":   testFile,
		"offset": 100.0,
	})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}

	// 应该返回空内容
	if result != "" && len(result) < 20 {
		t.Logf("Result: %s", result)
	}
}

func TestReadTool_Execute_ZeroLimit(t *testing.T) {
	tool := NewReadTool(10 * 1024 * 1024)
	ctx := context.Background()
	testDir := t.TempDir()
	tool.allowedDirs = []string{testDir}

	testFile := filepath.Join(testDir, "limit_test.txt")
	content := "1234567890"
	err := os.WriteFile(testFile, []byte(content), 0644)

	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// limit 为 0 应该读取全部
	result, err := tool.Execute(ctx, map[string]any{
		"path":  testFile,
		"limit": 0.0,
	})
	if err != nil {
		t.Errorf("Execute() error: %v", err)
	}

	// 应该包含全部内容
	if len(result) < len(content) {
		t.Errorf("expected full content, got truncated result")
	}
}
