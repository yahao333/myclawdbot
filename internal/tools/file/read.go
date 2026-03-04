package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yahao333/myclawdbot/internal/config"
)

// ReadTool 文件读取工具
type ReadTool struct {
	maxSize            int64
	allowedDirs        []string
	restrictFileAccess bool
}

func NewReadTool(maxSize int64) *ReadTool {
	return &ReadTool{
		maxSize:            maxSize,
		allowedDirs:        []string{},
		restrictFileAccess: true,
	}
}

// NewReadToolWithConfig 使用配置创建读取工具
func NewReadToolWithConfig(cfg *config.ToolsConfig) *ReadTool {
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = 10 * 1024 * 1024 // 默认 10MB
	}

	// 构建允许目录列表
	allowedDirs := make([]string, 0)

	// 如果不限制文件访问，allowedDirs 为空表示不限制
	if !cfg.RestrictFileAccess {
		allowedDirs = nil
	} else {
		// 添加当前目录
		if cfg.CurrentDir != "" {
			allowedDirs = append(allowedDirs, cfg.CurrentDir)
		}
		// 添加自定义允许目录
		allowedDirs = append(allowedDirs, cfg.AllowedDirs...)
		// 展开路径中的 ~
		for i, dir := range allowedDirs {
			allowedDirs[i] = expandHome(dir)
		}
	}

	return &ReadTool{
		maxSize:            cfg.MaxFileSize,
		allowedDirs:        allowedDirs,
		restrictFileAccess: cfg.RestrictFileAccess,
	}
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
		return "", fmt.Errorf("path not allowed: %s (only files under allowed directories can be accessed)", path)
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

// isPathAllowed 检查路径是否在允许的目录中
func (t *ReadTool) isPathAllowed(path string) bool {
	// 如果不限制，直接允许
	if !t.restrictFileAccess || len(t.allowedDirs) == 0 {
		return true
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// 解析符号链接
	realPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		absPath = realPath
	}

	// 检查是否在允许的目录中
	for _, dir := range t.allowedDirs {
		dir = expandHome(dir)
		// 检查是否是允许目录的子目录
		if strings.HasPrefix(absPath, dir+string(filepath.Separator)) || absPath == dir {
			return true
		}
	}

	return false
}

