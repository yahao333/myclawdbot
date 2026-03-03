package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yahao333/myclawdbot/internal/tools"
)

// ReadTool 文件读取工具
type ReadTool struct {
	maxSize int64
}

func NewReadTool(maxSize int64) *ReadTool {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 默认 10MB
	}
	return &ReadTool{maxSize: maxSize}
}

func (t *ReadTool) Name() string        { return "read" }
func (t *ReadTool) Description() string { return "读取文件内容" }
func (t *ReadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "要读取的文件路径",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "读取起始位置（字节）",
				"default":     0,
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "读取字节数限制",
				"default":     0,
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	// 安全检查
	if !t.isPathAllowed(path) {
		return "", fmt.Errorf("path not allowed: %s", path)
	}

	// 获取文件信息
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory")
	}

	if info.Size() > t.maxSize {
		return "", fmt.Errorf("file too large: %d bytes (max: %d)", info.Size(), t.maxSize)
	}

	// 读取文件
	offset := 0
	limit := 0

	if offsetVal, ok := params["offset"].(float64); ok {
		offset = int(offsetVal)
	}
	if limitVal, ok := params["limit"].(float64); ok {
		limit = int(limitVal)
	}

	content, err := t.readFile(path, offset, limit)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("文件: %s\n大小: %d 字节\n\n%s", path, info.Size(), content), nil
}

func (t *ReadTool) readFile(path string, offset, limit int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	if offset > len(data) {
		offset = len(data)
	}

	data = data[offset:]

	if limit > 0 && limit < len(data) {
		data = data[:limit]
	}

	return string(data), nil
}

func (t *ReadTool) isPathAllowed(path string) bool {
	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// 检查是否在允许的目录中
	// 这里简化处理，实际应该根据配置进行更严格的检查
	home, _ := os.UserHomeDir()
	allowedPrefixes := []string{
		"/tmp",
		"/Users/yanghao/Work", // 开发目录
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

// init 注册工具
func init() {
	tools.Register(NewReadTool(10 * 1024 * 1024))
}
