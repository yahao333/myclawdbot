package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yahao333/myclawdbot/internal/config"
)

// WriteTool 文件写入工具
type WriteTool struct {
	maxSize            int64
	allowedDirs        []string
	restrictFileAccess bool
}

func NewWriteTool(maxSize int64) *WriteTool {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 默认 10MB
	}
	return &WriteTool{
		maxSize:            maxSize,
		allowedDirs:        []string{},
		restrictFileAccess: true,
	}
}

// NewWriteToolWithConfig 使用配置创建写入工具
func NewWriteToolWithConfig(cfg *config.ToolsConfig) *WriteTool {
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = 10 * 1024 * 1024
	}

	allowedDirs := make([]string, 0)

	if !cfg.RestrictFileAccess {
		allowedDirs = nil
	} else {
		if cfg.CurrentDir != "" {
			allowedDirs = append(allowedDirs, cfg.CurrentDir)
		}
		allowedDirs = append(allowedDirs, cfg.AllowedDirs...)
		for i, dir := range allowedDirs {
			allowedDirs[i] = expandHome(dir)
		}
	}

	return &WriteTool{
		maxSize:            cfg.MaxFileSize,
		allowedDirs:        allowedDirs,
		restrictFileAccess: cfg.RestrictFileAccess,
	}
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

// isPathAllowed 检查路径是否在允许的目录中
func (t *WriteTool) isPathAllowed(path string) bool {
	if !t.restrictFileAccess || len(t.allowedDirs) == 0 {
		return true
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		absPath = realPath
	}

	for _, dir := range t.allowedDirs {
		dir = expandHome(dir)
		if strings.HasPrefix(absPath, dir+string(filepath.Separator)) || absPath == dir {
			return true
		}
	}

	return false
}

// expandHome 展开路径中的 ~
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// init 注册工具（在外部手动注册）
