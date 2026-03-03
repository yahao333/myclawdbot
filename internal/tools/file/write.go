package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteTool 文件写入工具
type WriteTool struct {
	maxSize int64
}

func NewWriteTool(maxSize int64) *WriteTool {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 默认 10MB
	}
	return &WriteTool{maxSize: maxSize}
}

func (t *WriteTool) Name() string        { return "write" }
func (t *WriteTool) Description() string { return "写入或创建文件" }
func (t *WriteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "要写入的文件路径",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "文件内容",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	content, ok := params["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	// 安全检查
	if !t.isPathAllowed(path) {
		return "", fmt.Errorf("path not allowed: %s", path)
	}

	// 检查文件大小
	if int64(len(content)) > t.maxSize {
		return "", fmt.Errorf("content too large: %d bytes (max: %d)", len(content), t.maxSize)
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("文件已写入: %s (%d bytes)", path, len(content)), nil
}

func (t *WriteTool) isPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	home, _ := os.UserHomeDir()
	allowedPrefixes := []string{
		"/tmp",
		"/Users/yanghao/Work",
	}

	if home != "" {
		allowedPrefixes = append(allowedPrefixes, home)
	}

	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(absPath, prefix) {
			return true
		}
	}

	return false
}

// init 注册工具（在外部手动注册）
