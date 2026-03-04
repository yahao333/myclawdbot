// Package config 配置管理包
// 提供应用程序配置加载和管理功能，支持从 YAML 文件和环境变量加载配置
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用程序主配置结构体
// 包含所有模块的配置信息：LLM、工具、会话、记忆、网关、渠道等
type Config struct {
	LLM     LLMConfig     `yaml:"llm"`     // LLM 大语言模型配置
	Tools   ToolsConfig   `yaml:"tools"`   // 工具配置（命令执行、文件访问等）
	Session SessionConfig `yaml:"session"` // 会话配置（历史记录、存储目录）
	Memory  MemoryConfig  `yaml:"memory"`  // 记忆配置（短期/长期记忆、向量嵌入等）
	Gateway GatewayConfig `yaml:"gateway"` // 网关配置（主机、端口）
	Channel ChannelConfig `yaml:"channel"` // 渠道配置（终端、Telegram 等）
}

// LLMConfig 大语言模型配置
// 配置 LLM 供应商、API 密钥、模型等
type LLMConfig struct {
	Provider string `yaml:"provider"` // LLM 供应商：anthropic, openai, minimax
	APIKey   string `yaml:"api_key"`  // API 密钥
	Model    string `yaml:"model"`    // 模型名称
	BaseURL  string `yaml:"base_url"` // 自定义 API 端点（可选）
	GroupID  string `yaml:"group_id"` // 群组 ID（用于 Minimax 兼容模式，当前版本不使用）
}

// ChannelConfig 渠道配置
// 定义与用户交互的渠道类型
type ChannelConfig struct {
	Type     string         `yaml:"type"`     // 渠道类型：terminal, telegram
	Telegram TelegramConfig `yaml:"telegram"` // Telegram 配置
}

// TelegramConfig Telegram 电报机器人配置
type TelegramConfig struct {
	BotToken string `yaml:"bot_token"` // Telegram Bot 令牌
}

// ToolsConfig 工具配置
// 定义可执行的命令、文件访问限制等安全策略
type ToolsConfig struct {
	AllowedCommands []string `yaml:"allowed_commands"` // 允许执行的系统命令列表
	BlockedPaths    []string `yaml:"blocked_paths"`    // 禁止访问的路径模式
	MaxFileSize     int64    `yaml:"max_file_size"`    // 最大文件大小（字节）
	MaxExecTime     int      `yaml:"max_exec_time"`    // 命令最大执行时间（秒）

	// 文件访问限制配置
	AllowedDirs        []string `yaml:"allowed_dirs"`         // 允许访问的目录列表（白名单）
	RestrictFileAccess bool     `yaml:"restrict_file_access"` // 是否限制文件访问（默认 true，仅允许当前目录）
	CurrentDir         string   `yaml:"current_dir"`          // 当前工作目录（用于限制文件访问边界）
}

// SessionConfig 会话配置
// 控制会话历史记录和存储
type SessionConfig struct {
	MaxHistory int    `yaml:"max_history"` // 单个会话最大历史消息数
	StorageDir string `yaml:"storage_dir"` // 会话数据存储目录
}

// MemoryConfig 记忆配置
// 控制短期会话记忆和长期向量记忆
type MemoryConfig struct {
	Enable         bool   `yaml:"enable"`           // 是否启用记忆功能
	MaxHistory     int    `yaml:"max_history"`      // 短期记忆最大消息数
	MaxTokens      int    `yaml:"max_tokens"`       // 最大 token 数限制
	EnableCompress bool   `yaml:"enable_compress"`  // 是否启用自动压缩（当 token 超限时压缩历史）
	EnableLongTerm bool   `yaml:"enable_long_term"` // 是否启用长期记忆（向量存储）
	StorageDir     string `yaml:"storage_dir"`      // 记忆数据存储目录
	EmbeddingModel string `yaml:"embedding_model"`  // 向量嵌入模型：simple, openai, claude
}

// GatewayConfig 网关配置
// WebSocket 网关的监听地址、认证和安全设置
type GatewayConfig struct {
	Host          string   `yaml:"host"`           // 监听主机地址
	Port          int      `yaml:"port"`           // 监听端口号
	EnableAuth    bool     `yaml:"enable_auth"`    // 是否启用认证
	APIKeys       []string `yaml:"api_keys"`       // 允许的 API Key SHA256 哈希值列表
	EnableSandbox bool     `yaml:"enable_sandbox"` // 是否启用沙盒模式（限制文件访问和命令执行）
	SandboxDirs   []string `yaml:"sandbox_dirs"`   // 沙盒允许访问的目录列表
}

// Load 从 YAML 文件加载配置
// path: 配置文件路径
// 返回配置对象或错误信息
func Load(path string) (*Config, error) {
	// 读取 YAML 配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 解析 YAML 到配置结构体
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 设置默认值
	cfg.setDefaults()
	cfg.Gateway.APIKeys = normalizeGatewayAPIKeys(cfg.Gateway.APIKeys)

	return &cfg, nil
}

// LoadFromEnv 从环境变量加载配置
// 优先级：环境变量 > 默认值
// 支持的环境变量包括：
//   - LLM_PROVIDER, LLM_MODEL, LLM_BASE_URL, MINIMAX_API_KEY, MINIMAX_GROUP_ID
//   - TELEGRAM_BOT_TOKEN, CHANNEL_TYPE
//   - MEMORY_ENABLE, MEMORY_LONG_TERM, MEMORY_STORAGE_DIR, MEMORY_EMBEDDING_MODEL
//   - GATEWAY_HOST, GATEWAY_PORT
//   - SESSION_STORAGE_DIR
func LoadFromEnv() *Config {
	// 默认使用 minimax
	provider := getEnv("LLM_PROVIDER", "minimax")
	model := getEnv("LLM_MODEL", "MiniMax-M2.5")

	cfg := &Config{
		LLM: LLMConfig{
			Provider: provider,
			APIKey:   getEnv("MINIMAX_API_KEY", ""),
			Model:    model,
			BaseURL:  getEnv("LLM_BASE_URL", "https://api.minimaxi.com/anthropic"),
			GroupID:  getEnv("MINIMAX_GROUP_ID", ""),
		},
		Tools: ToolsConfig{
			AllowedCommands:    []string{"go", "git", "ls", "cat", "pwd", "echo", "mkdir", "rm", "cp", "mv"},
			BlockedPaths:       []string{"/etc", "/root", "/home/*/.*ssh"},
			MaxFileSize:        10 * 1024 * 1024, // 10MB
			MaxExecTime:        300,              // 5分钟
			RestrictFileAccess: true,             // 默认限制文件访问
			AllowedDirs:        []string{},       // 自定义允许目录
			CurrentDir:         getCwd(),         // 当前工作目录
		},
		Session: SessionConfig{
			MaxHistory: 100,
			StorageDir: getEnv("SESSION_STORAGE_DIR", "~/.myclawdbot/sessions"),
		},
		Memory: MemoryConfig{
			Enable:         getEnv("MEMORY_ENABLE", "false") == "true",
			MaxHistory:     100,
			MaxTokens:      4000,
			EnableCompress: true,
			EnableLongTerm: getEnv("MEMORY_LONG_TERM", "false") == "true",
			StorageDir:     getEnv("MEMORY_STORAGE_DIR", "~/.myclawdbot/memory"),
			EmbeddingModel: getEnv("MEMORY_EMBEDDING_MODEL", "simple"),
		},
		Gateway: GatewayConfig{
			Host:          getEnv("GATEWAY_HOST", "localhost"),
			Port:          8080,
			EnableAuth:    getEnv("GATEWAY_ENABLE_AUTH", "false") == "true",
			APIKeys:       []string{},
			EnableSandbox: getEnv("GATEWAY_ENABLE_SANDBOX", "false") == "true",
			SandboxDirs:   []string{},
		},
		Channel: ChannelConfig{
			Type: getEnv("CHANNEL_TYPE", "terminal"),
			Telegram: TelegramConfig{
				BotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
			},
		},
	}

	// 如果没有设置 MINIMAX_API_KEY，使用用户提供的
	if apiKey := getEnv("MINIMAX_API_KEY", ""); apiKey == "" {
		// 用户直接提供的 key
	}

	// 展开 ~ 为用户目录
	cfg.Session.StorageDir = expandHome(cfg.Session.StorageDir)
	cfg.Gateway.APIKeys = normalizeGatewayAPIKeys(cfg.Gateway.APIKeys)

	return cfg
}

// setDefaults 设置配置默认值
// 为未设置的配置项填充默认值，确保所有配置都有合理的初始值
func (c *Config) setDefaults() {
	if c.LLM.Provider == "" {
		c.LLM.Provider = "anthropic"
	}
	if c.LLM.Model == "" {
		c.LLM.Model = "claude-3-5-sonnet-20241022"
	}
	if c.Tools.MaxFileSize == 0 {
		c.Tools.MaxFileSize = 10 * 1024 * 1024
	}
	if c.Tools.MaxExecTime == 0 {
		c.Tools.MaxExecTime = 300
	}
	if c.Session.MaxHistory == 0 {
		c.Session.MaxHistory = 100
	}
	if c.Session.StorageDir == "" {
		c.Session.StorageDir = "~/.myclawdbot/sessions"
	}
	c.Session.StorageDir = expandHome(c.Session.StorageDir)
	if c.Gateway.Port == 0 {
		c.Gateway.Port = 8080
	}
	if c.Gateway.Host == "" {
		c.Gateway.Host = "localhost"
	}
	// 默认沙盒目录为当前工作目录
	if len(c.Gateway.SandboxDirs) == 0 {
		c.Gateway.SandboxDirs = []string{getCwd()}
	}
}

func normalizeGatewayAPIKeys(apiKeys []string) []string {
	if len(apiKeys) == 0 {
		return apiKeys
	}
	normalized := make([]string, 0, len(apiKeys))
	seen := make(map[string]struct{}, len(apiKeys))
	for _, key := range apiKeys {
		hashed := normalizeAPIKeyHash(key)
		if hashed == "" {
			continue
		}
		if _, exists := seen[hashed]; exists {
			continue
		}
		seen[hashed] = struct{}{}
		normalized = append(normalized, hashed)
	}
	return normalized
}

func normalizeAPIKeyHash(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if len(lower) == 64 {
		if _, err := hex.DecodeString(lower); err == nil {
			return lower
		}
	}
	hash := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(hash[:])
}

// getEnv 获取环境变量值
// key: 环境变量名称
// defaultValue: 默认值（当环境变量未设置时返回）
// 返回环境变量的值或默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getCwd 获取当前工作目录
// 返回当前工作目录的绝对路径，如果获取失败返回根目录 "/"
func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "/"
	}
	return cwd
}

// expandHome 展开路径中的 ~ 为用户主目录
// path: 包含 ~ 的文件路径
// 返回展开后的绝对路径
// 示例：~/.myclawdbot -> /home/user/.myclawdbot
func expandHome(path string) string {
	// 检查路径是否以 ~ 开头
	if len(path) > 0 && path[0] == '~' {
		// 获取用户主目录
		home, err := os.UserHomeDir()
		if err != nil {
			return path // 如果获取失败返回原路径
		}
		// 拼接主目录和剩余路径
		return filepath.Join(home, path[1:])
	}
	return path
}
