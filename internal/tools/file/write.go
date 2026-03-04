// Package file 文件操作工具包（已在 read.go 中定义）
// 本文件提供文件写入工具的实现
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
// 实现 Tool 接口，提供安全的文件写入功能
// 支持路径访问控制、文件大小限制、自动创建目录
type WriteTool struct {
	maxSize            int64   // 最大允许写入的内容大小（字节）
	allowedDirs        []string // 允许写入的目录列表（白名单）
	restrictFileAccess bool    // 是否启用文件访问限制
}

// NewWriteTool 创建写入工具实例
// maxSize: 最大允许写入的内容大小（字节），如果 <= 0 则使用默认值 10MB
// 返回配置了默认值的 WriteTool 实例
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
// cfg: 工具配置对象，从中读取文件访问限制等配置
// 根据配置构建允许写入的目录列表，支持以下策略：
//   - 如果 RestrictFileAccess 为 false，不限制文件访问
//   - 否则，只允许写入 CurrentDir 和 AllowedDirs 中指定的目录
//   - 自动展开路径中的 ~ 为用户主目录
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

// Name 返回工具名称
func (t *WriteTool) Name() string        { return "write" }

// Description 返回工具描述
func (t *WriteTool) Description() string { return "写入或创建文件" }

// Parameters 返回工具参数 schema
// 返回 JSON Schema 格式的参数定义
//   - path: 必需，要写入的文件路径
//   - content: 必需，要写入的文件内容
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

// Execute 执行文件写入操作
// ctx: 上下文对象
// params: 参数映射，必须包含 "path" 和 "content"
// 返回写入结果字符串或错误信息
// 执行流程：
//   1. 验证 path 和 content 参数
//   2. 检查路径是否在允许的目录中
//   3. 检查内容大小是否超过限制
//   4. 确保目标目录存在（自动创建）
//   5. 写入文件内容
//   6. 返回格式化的结果
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

// isPathAllowed 检查路径是否在允许访问的目录中
// path: 要检查的文件路径
// 返回 true 表示路径允许访问，false 表示拒绝访问
// 安全检查逻辑：
//   1. 如果未启用限制或允许目录为空，允许所有访问
//   2. 将路径转换为绝对路径
//   3. 解析符号链接以防止绕过
//   4. 检查路径是否在允许目录的子目录中
//   5. 同时检查目录本身（允许访问整个目录）
func (t *WriteTool) isPathAllowed(path string) bool {
	if !t.restrictFileAccess || t.allowedDirs == nil {
		return true
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		absPath = realPath
	} else {
		parentDir := filepath.Dir(absPath)
		realParent, parentErr := filepath.EvalSymlinks(parentDir)
		if parentErr == nil {
			absPath = filepath.Join(realParent, filepath.Base(absPath))
		}
	}

	for _, dir := range t.allowedDirs {
		dir = expandHome(dir)
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		realDir, err := filepath.EvalSymlinks(absDir)
		if err == nil {
			dir = realDir
		} else {
			dir = absDir
		}
		if strings.HasPrefix(absPath, dir+string(filepath.Separator)) || absPath == dir {
			return true
		}
	}

	return false
}

// expandHome 展开路径中的 ~ 为用户主目录
// path: 包含 ~ 的文件路径
// 返回展开后的绝对路径
// 示例：~/.config/app -> /home/user/.config/app
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
// 注意：当前版本采用手动注册方式，此 init 函数保留但未使用
