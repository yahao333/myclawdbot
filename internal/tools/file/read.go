// Package file 文件操作工具包
// 提供文件读取和写入的工具实现，支持基于配置的文件访问控制
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
// 实现 Tool 接口，提供安全的文件读取功能
// 支持路径访问控制、文件大小限制、偏移量和读取长度控制
type ReadTool struct {
	maxSize            int64   // 最大允许读取的文件大小（字节）
	allowedDirs        []string // 允许访问的目录列表（白名单）
	restrictFileAccess bool    // 是否启用文件访问限制
}

// NewReadTool 创建读取工具实例
// maxSize: 最大允许读取的文件大小（字节），如果 <= 0 则使用默认值 10MB
// 返回配置了默认值的 ReadTool 实例
func NewReadTool(maxSize int64) *ReadTool {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	return &ReadTool{
		maxSize:            maxSize,
		allowedDirs:        []string{},
		restrictFileAccess: true,
	}
}

// NewReadToolWithConfig 使用配置创建读取工具
// cfg: 工具配置对象，从中读取文件访问限制等配置
// 根据配置构建允许访问的目录列表，支持以下策略：
//   - 如果 RestrictFileAccess 为 false，不限制文件访问
//   - 否则，只允许访问 CurrentDir 和 AllowedDirs 中指定的目录
//   - 自动展开路径中的 ~ 为用户主目录
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

// Name 返回工具名称
func (t *ReadTool) Name() string        { return "read" }

// Description 返回工具描述
func (t *ReadTool) Description() string { return "读取文件内容" }

// Parameters 返回工具参数 schema
// 返回 JSON Schema 格式的参数定义
//   - path: 必需，文件路径
//   - offset: 可选，读取起始位置（字节），默认 0
//   - limit: 可选，读取字节数限制，0 表示读取到文件末尾
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

// Execute 执行文件读取操作
// ctx: 上下文对象
// params: 参数映射，必须包含 "path"
// 返回读取结果字符串或错误信息
// 执行流程：
//   1. 验证 path 参数
//   2. 检查路径是否在允许的目录中
//   3. 检查文件大小是否超过限制
//   4. 读取文件内容（支持 offset 和 limit）
//   5. 返回格式化的结果
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

// readFile 读取文件内容
// path: 文件路径
// offset: 读取起始位置（字节）
// limit: 读取字节数限制，0 表示读取到文件末尾
// 返回文件内容字符串或错误信息
// 注意：offset 和 limit 用于部分读取，而非字符偏移
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

// isPathAllowed 检查路径是否在允许访问的目录中
// path: 要检查的文件路径
// 返回 true 表示路径允许访问，false 表示拒绝访问
// 安全检查逻辑：
//   1. 如果未启用限制或允许目录为空，允许所有访问
//   2. 将路径转换为绝对路径
//   3. 解析符号链接以防止绕过
//   4. 检查路径是否在允许目录的子目录中
//   5. 同时检查目录本身（允许访问整个目录）
func (t *ReadTool) isPathAllowed(path string) bool {
	// 如果未启用限制或允许目录为空，允许所有访问
	if !t.restrictFileAccess || t.allowedDirs == nil {
		return true
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// 解析符号链接，防止通过符号链接绕过限制
	realPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		absPath = realPath
	}

	// 遍历允许目录列表，检查路径是否在允许目录中
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
		// 检查是否是允许目录的子目录或是允许目录本身
		if strings.HasPrefix(absPath, dir+string(filepath.Separator)) || absPath == dir {
			return true
		}
	}

	return false
}
