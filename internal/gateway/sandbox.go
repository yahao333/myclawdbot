// Package gateway 网关包
// 提供 HTTP/WebSocket 网关服务，支持 REST API、实时通信和安全隔离执行
package gateway

import (
	"github.com/yahao333/myclawdbot/internal/config"
	"github.com/yahao333/myclawdbot/internal/tools"
	"github.com/yahao333/myclawdbot/internal/tools/exec"
	"github.com/yahao333/myclawdbot/internal/tools/file"
)

// SandboxConfig 沙盒配置
// 定义安全隔离执行环境的规则
type SandboxConfig struct {
	Enabled       bool     // 是否启用沙盒
	AllowedDirs   []string // 允许访问的目录
	AllowedCmds   []string // 允许执行的命令
	MaxExecTime   int      // 最大执行时间（秒）
	MaxFileSize   int64    // 最大文件大小（字节）
	BlockNetwork  bool     // 是否阻止网络访问
	BlockEnvVars  []string // 禁止的环境变量
}

// DefaultSandboxConfig 返回默认沙盒配置
func DefaultSandboxConfig(cfg *config.Config) *SandboxConfig {
	return &SandboxConfig{
		Enabled:      cfg.Gateway.EnableSandbox,
		AllowedDirs:  cfg.Gateway.SandboxDirs,
		AllowedCmds:  cfg.Tools.AllowedCommands,
		MaxExecTime:  cfg.Tools.MaxExecTime,
		MaxFileSize:  cfg.Tools.MaxFileSize,
		BlockNetwork: false,
		BlockEnvVars: []string{},
	}
}

// IsPathAllowed 检查路径是否在沙盒允许范围内
func (s *SandboxConfig) IsPathAllowed(path string) bool {
	if !s.Enabled {
		return true // 沙盒未启用，允许所有
	}

	for _, dir := range s.AllowedDirs {
		if isSubPath(dir, path) {
			return true
		}
	}
	return false
}

// IsCommandAllowed 检查命令是否在沙盒允许范围内
func (s *SandboxConfig) IsCommandAllowed(cmd string) bool {
	if !s.Enabled {
		return true // 沙盒未启用，允许所有
	}

	for _, allowed := range s.AllowedCmds {
		if cmd == allowed {
			return true
		}
	}
	return false
}

// isSubPath 检查 child 是否是 parent 的子目录
func isSubPath(parent, child string) bool {
	// 简单实现：检查 child 是否以 parent 开头
	// 生产环境应使用 filepath.EvalSymlinks 进行真实路径比较
	if len(child) >= len(parent) {
		if child[:len(parent)] == parent {
			// 确保是完整路径分隔
			if len(child) == len(parent) || child[len(parent)] == '/' {
				return true
			}
		}
	}
	return false
}

// CreateSandboxToolRegistry 创建沙盒模式的工具注册表
// 仅注册安全的工具，并应用访问限制
func CreateSandboxToolRegistry(cfg *config.Config) *tools.Registry {
	registry := tools.NewRegistry()

	// 创建带限制的配置
	restrictedTools := config.ToolsConfig{
		AllowedCommands:    cfg.Tools.AllowedCommands,
		MaxExecTime:       cfg.Tools.MaxExecTime,
		MaxFileSize:       cfg.Tools.MaxFileSize,
		RestrictFileAccess: true, // 强制限制文件访问
		CurrentDir:         getSandboxRoot(cfg),
		AllowedDirs:        cfg.Gateway.SandboxDirs,
	}

	// 注册受限的文件读取工具
	readTool := file.NewReadToolWithConfig(&restrictedTools)
	registry.Register(readTool)

	// 注册受限的文件写入工具
	writeTool := file.NewWriteToolWithConfig(&restrictedTools)
	registry.Register(writeTool)

	// 注册受限的命令执行工具
	if len(restrictedTools.AllowedCommands) > 0 {
		cmdTool := exec.NewCommandTool(
			restrictedTools.AllowedCommands,
			restrictedTools.MaxExecTime,
		)
		registry.Register(cmdTool)
	}

	return registry
}

// getSandboxRoot 获取沙盒根目录
func getSandboxRoot(cfg *config.Config) string {
	if len(cfg.Gateway.SandboxDirs) > 0 {
		return cfg.Gateway.SandboxDirs[0]
	}
	return cfg.Tools.CurrentDir
}

// SandboxValidator 沙盒验证器
// 用于验证操作是否在沙盒允许范围内
type SandboxValidator struct {
	config *SandboxConfig
}

// NewSandboxValidator 创建沙盒验证器
func NewSandboxValidator(cfg *SandboxConfig) *SandboxValidator {
	return &SandboxValidator{config: cfg}
}

// ValidatePath 验证文件路径访问
func (v *SandboxValidator) ValidatePath(path string) error {
	if !v.config.Enabled {
		return nil
	}

	if !v.config.IsPathAllowed(path) {
		return &SandboxError{
			Operation: "file_access",
			Message:   "Path not allowed in sandbox: " + path,
		}
	}
	return nil
}

// ValidateCommand 验证命令执行
func (v *SandboxValidator) ValidateCommand(cmd string) error {
	if !v.config.Enabled {
		return nil
	}

	if !v.config.IsCommandAllowed(cmd) {
		return &SandboxError{
			Operation: "command_execution",
			Message:   "Command not allowed in sandbox: " + cmd,
		}
	}
	return nil
}

// SandboxError 沙盒错误
type SandboxError struct {
	Operation string
	Message  string
}

func (e *SandboxError) Error() string {
	return e.Message
}
