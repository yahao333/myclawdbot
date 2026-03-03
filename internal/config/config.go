package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 配置结构体
type Config struct {
	LLM     LLMConfig     `yaml:"llm"`
	Tools   ToolsConfig   `yaml:"tools"`
	Session SessionConfig `yaml:"session"`
	Gateway GatewayConfig `yaml:"gateway"`
	Channel ChannelConfig `yaml:"channel"`
}

// LLMConfig LLM 配置
type LLMConfig struct {
	Provider string `yaml:"provider"` // anthropic, openai, minimax
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"` // 可选，自定义 API 端点
	GroupID  string `yaml:"group_id"` // Minimax 所需
}

// ChannelConfig 渠道配置
type ChannelConfig struct {
	Type     string         `yaml:"type"` // terminal, telegram
	Telegram TelegramConfig `yaml:"telegram"`
}

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
}

// ToolsConfig 工具配置
type ToolsConfig struct {
	AllowedCommands []string `yaml:"allowed_commands"` // 允许执行的命令
	BlockedPaths    []string `yaml:"blocked_paths"`    // 禁止访问的路径
	MaxFileSize     int64    `yaml:"max_file_size"`    // 最大文件大小 (bytes)
	MaxExecTime     int      `yaml:"max_exec_time"`    // 最大执行时间 (秒)
}

// SessionConfig 会话配置
type SessionConfig struct {
	MaxHistory int    `yaml:"max_history"` // 最大历史消息数
	StorageDir string `yaml:"storage_dir"`
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 设置默认值
	cfg.setDefaults()

	return &cfg, nil
}

// LoadFromEnv 从环境变量加载配置
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
			AllowedCommands: []string{"go", "git", "ls", "cat", "pwd", "echo", "mkdir", "rm", "cp", "mv"},
			BlockedPaths:    []string{"/etc", "/root", "/home/*/.*ssh"},
			MaxFileSize:     10 * 1024 * 1024, // 10MB
			MaxExecTime:     300,              // 5分钟
		},
		Session: SessionConfig{
			MaxHistory: 100,
			StorageDir: getEnv("SESSION_STORAGE_DIR", "~/.myclawdbot/sessions"),
		},
		Gateway: GatewayConfig{
			Host: getEnv("GATEWAY_HOST", "localhost"),
			Port: 8080,
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

	return cfg
}

// setDefaults 设置默认值
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
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

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
