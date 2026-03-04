package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
llm:
  provider: anthropic
  api_key: test-key
  model: claude-3-5-sonnet-20241022
  base_url: https://api.anthropic.com

tools:
  allowed_commands:
    - go
    - git
  blocked_paths:
    - /etc
  max_file_size: 10485760
  max_exec_time: 300

session:
  max_history: 50
  storage_dir: /tmp/sessions

gateway:
  host: localhost
  port: 8080

channel:
  type: telegram
  telegram:
    bot_token: test-token
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// 测试 Load 函数
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// 验证配置值
	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", cfg.LLM.Provider)
	}
	if cfg.LLM.APIKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got '%s'", cfg.LLM.APIKey)
	}
	if cfg.LLM.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected model 'claude-3-5-sonnet-20241022', got '%s'", cfg.LLM.Model)
	}
	if cfg.Tools.AllowedCommands[0] != "go" {
		t.Errorf("expected allowed_commands[0] 'go', got '%s'", cfg.Tools.AllowedCommands[0])
	}
	if cfg.Session.MaxHistory != 50 {
		t.Errorf("expected max_history 50, got %d", cfg.Session.MaxHistory)
	}
	if cfg.Gateway.Port != 8080 {
		t.Errorf("expected gateway port 8080, got %d", cfg.Gateway.Port)
	}
	if cfg.Channel.Telegram.BotToken != "test-token" {
		t.Errorf("expected bot_token 'test-token', got '%s'", cfg.Channel.Telegram.BotToken)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadFromEnv(t *testing.T) {
	// 保存原始环境变量
	origProvider := os.Getenv("LLM_PROVIDER")
	origModel := os.Getenv("LLM_MODEL")
	origAPIKey := os.Getenv("MINIMAX_API_KEY")
	origBaseURL := os.Getenv("LLM_BASE_URL")
	origStorageDir := os.Getenv("SESSION_STORAGE_DIR")
	origChannelType := os.Getenv("CHANNEL_TYPE")
	origBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	defer func() {
		os.Setenv("LLM_PROVIDER", origProvider)
		os.Setenv("LLM_MODEL", origModel)
		os.Setenv("MINIMAX_API_KEY", origAPIKey)
		os.Setenv("LLM_BASE_URL", origBaseURL)
		os.Setenv("SESSION_STORAGE_DIR", origStorageDir)
		os.Setenv("CHANNEL_TYPE", origChannelType)
		os.Setenv("TELEGRAM_BOT_TOKEN", origBotToken)
	}()

	// 设置环境变量
	os.Setenv("LLM_PROVIDER", "openai")
	os.Setenv("LLM_MODEL", "gpt-4")
	os.Setenv("MINIMAX_API_KEY", "env-api-key")
	os.Setenv("LLM_BASE_URL", "https://api.openai.com")
	os.Setenv("SESSION_STORAGE_DIR", "/custom/sessions")
	os.Setenv("CHANNEL_TYPE", "telegram")
	os.Setenv("TELEGRAM_BOT_TOKEN", "env-bot-token")

	cfg := LoadFromEnv()

	// 验证配置
	if cfg.LLM.Provider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got '%s'", cfg.LLM.Model)
	}
	if cfg.LLM.APIKey != "env-api-key" {
		t.Errorf("expected api_key 'env-api-key', got '%s'", cfg.LLM.APIKey)
	}
	if cfg.LLM.BaseURL != "https://api.openai.com" {
		t.Errorf("expected base_url 'https://api.openai.com', got '%s'", cfg.LLM.BaseURL)
	}
	if cfg.Session.StorageDir != "/custom/sessions" {
		t.Errorf("expected storage_dir '/custom/sessions', got '%s'", cfg.Session.StorageDir)
	}
	if cfg.Channel.Type != "telegram" {
		t.Errorf("expected channel type 'telegram', got '%s'", cfg.Channel.Type)
	}
	if cfg.Channel.Telegram.BotToken != "env-bot-token" {
		t.Errorf("expected bot_token 'env-bot-token', got '%s'", cfg.Channel.Telegram.BotToken)
	}
}

func TestLoadFromEnv_Defaults(t *testing.T) {
	// 清除所有相关环境变量
	os.Unsetenv("LLM_PROVIDER")
	os.Unsetenv("LLM_MODEL")
	os.Unsetenv("MINIMAX_API_KEY")
	os.Unsetenv("LLM_BASE_URL")
	os.Unsetenv("SESSION_STORAGE_DIR")
	os.Unsetenv("CHANNEL_TYPE")
	os.Unsetenv("TELEGRAM_BOT_TOKEN")

	cfg := LoadFromEnv()

	// 验证默认值
	if cfg.LLM.Provider != "minimax" {
		t.Errorf("expected default provider 'minimax', got '%s'", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "MiniMax-M2.5" {
		t.Errorf("expected default model 'MiniMax-M2.5', got '%s'", cfg.LLM.Model)
	}
	if cfg.Tools.MaxFileSize != 10*1024*1024 {
		t.Errorf("expected default max_file_size 10485760, got %d", cfg.Tools.MaxFileSize)
	}
	if cfg.Tools.MaxExecTime != 300 {
		t.Errorf("expected default max_exec_time 300, got %d", cfg.Tools.MaxExecTime)
	}
	if cfg.Session.MaxHistory != 100 {
		t.Errorf("expected default max_history 100, got %d", cfg.Session.MaxHistory)
	}
	if cfg.Gateway.Port != 8080 {
		t.Errorf("expected default gateway port 8080, got %d", cfg.Gateway.Port)
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	cfg := &Config{}

	cfg.setDefaults()

	// 验证默认值设置
	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got '%s'", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected default model 'claude-3-5-sonnet-20241022', got '%s'", cfg.LLM.Model)
	}
	if cfg.Tools.MaxFileSize != 10*1024*1024 {
		t.Errorf("expected default max_file_size 10485760, got %d", cfg.Tools.MaxFileSize)
	}
	if cfg.Tools.MaxExecTime != 300 {
		t.Errorf("expected default max_exec_time 300, got %d", cfg.Tools.MaxExecTime)
	}
	if cfg.Session.MaxHistory != 100 {
		t.Errorf("expected default max_history 100, got %d", cfg.Session.MaxHistory)
	}
	if cfg.Gateway.Port != 8080 {
		t.Errorf("expected default gateway port 8080, got %d", cfg.Gateway.Port)
	}
}

func TestConfig_SetDefaults_PreservesExisting(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4",
		},
		Tools: ToolsConfig{
			MaxFileSize: 5 * 1024 * 1024,
			MaxExecTime: 60,
		},
		Session: SessionConfig{
			MaxHistory: 200,
			StorageDir:  "/custom",
		},
		Gateway: GatewayConfig{
			Port: 9000,
		},
	}

	cfg.setDefaults()

	// 验证已有值不被覆盖
	if cfg.LLM.Provider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got '%s'", cfg.LLM.Model)
	}
	if cfg.Tools.MaxFileSize != 5*1024*1024 {
		t.Errorf("expected max_file_size 5242880, got %d", cfg.Tools.MaxFileSize)
	}
	if cfg.Tools.MaxExecTime != 60 {
		t.Errorf("expected max_exec_time 60, got %d", cfg.Tools.MaxExecTime)
	}
	if cfg.Session.MaxHistory != 200 {
		t.Errorf("expected max_history 200, got %d", cfg.Session.MaxHistory)
	}
	if cfg.Gateway.Port != 9000 {
		t.Errorf("expected gateway port 9000, got %d", cfg.Gateway.Port)
	}
}

func TestGetEnv(t *testing.T) {
	// 保存原始值
	orig := os.Getenv("TEST_KEY")
	defer os.Setenv("TEST_KEY", orig)

	// 测试有值的情况
	os.Setenv("TEST_KEY", "test_value")
	if got := getEnv("TEST_KEY", "default"); got != "test_value" {
		t.Errorf("getEnv() = %s, want 'test_value'", got)
	}

	// 测试默认值的情况
	os.Unsetenv("TEST_KEY")
	if got := getEnv("TEST_KEY", "default"); got != "default" {
		t.Errorf("getEnv() = %s, want 'default'", got)
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		input    string
		wantSame bool // true if we expect the same path (home dir unavailable)
	}{
		{"", false},
		{"/absolute/path", false},
		{"~/test", true}, // will be expanded if home is available
	}

	for _, tt := range tests {
		result := expandHome(tt.input)
		if tt.input == "" && result != "" {
			t.Errorf("expandHome(%q) = %q, want empty", tt.input, result)
		}
		if tt.input == "/absolute/path" && result != "/absolute/path" {
			t.Errorf("expandHome(%q) = %q, want same", tt.input, result)
		}
		if tt.input == "~/test" && len(result) > 0 && result != "~/test" {
			// 成功展开了 ~，这是预期的
		}
	}
}

// TestLoad_FileAccessRestrictions 测试文件访问限制配置
func TestLoad_FileAccessRestrictions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
tools:
  restrict_file_access: true
  allowed_dirs:
    - /tmp/allowed
    - /home/user/data
  current_dir: /home/user
  max_file_size: 5242880
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// 验证文件访问限制配置
	if !cfg.Tools.RestrictFileAccess {
		t.Error("expected restrict_file_access to be true")
	}
	if len(cfg.Tools.AllowedDirs) != 2 {
		t.Errorf("expected 2 allowed_dirs, got %d", len(cfg.Tools.AllowedDirs))
	}
	if cfg.Tools.CurrentDir != "/home/user" {
		t.Errorf("expected current_dir '/home/user', got '%s'", cfg.Tools.CurrentDir)
	}
	if cfg.Tools.MaxFileSize != 5242880 {
		t.Errorf("expected max_file_size 5242880, got %d", cfg.Tools.MaxFileSize)
	}
}

// TestLoad_FileAccessRestrictionsDisabled 测试禁用文件访问限制
func TestLoad_FileAccessRestrictionsDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
tools:
  restrict_file_access: false
  allowed_dirs:
    - /tmp/allowed
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// 验证文件访问限制配置
	if cfg.Tools.RestrictFileAccess {
		t.Error("expected restrict_file_access to be false")
	}
}

// TestLoad_MemoryConfig 测试记忆配置
func TestLoad_MemoryConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
memory:
  enable: true
  max_history: 200
  max_tokens: 8000
  enable_compress: true
  enable_long_term: true
  storage_dir: /tmp/memory
  embedding_model: openai
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// 验证记忆配置
	if !cfg.Memory.Enable {
		t.Error("expected memory.enable to be true")
	}
	if cfg.Memory.MaxHistory != 200 {
		t.Errorf("expected max_history 200, got %d", cfg.Memory.MaxHistory)
	}
	if cfg.Memory.MaxTokens != 8000 {
		t.Errorf("expected max_tokens 8000, got %d", cfg.Memory.MaxTokens)
	}
	if !cfg.Memory.EnableCompress {
		t.Error("expected enable_compress to be true")
	}
	if !cfg.Memory.EnableLongTerm {
		t.Error("expected enable_long_term to be true")
	}
	if cfg.Memory.StorageDir != "/tmp/memory" {
		t.Errorf("expected storage_dir '/tmp/memory', got '%s'", cfg.Memory.StorageDir)
	}
	if cfg.Memory.EmbeddingModel != "openai" {
		t.Errorf("expected embedding_model 'openai', got '%s'", cfg.Memory.EmbeddingModel)
	}
}

// TestLoadFromEnv_MemoryConfig 测试从环境变量加载记忆配置
func TestLoadFromEnv_MemoryConfig(t *testing.T) {
	// 保存原始环境变量
	origEnable := os.Getenv("MEMORY_ENABLE")
	origLongTerm := os.Getenv("MEMORY_LONG_TERM")
	origStorageDir := os.Getenv("MEMORY_STORAGE_DIR")
	origEmbeddingModel := os.Getenv("MEMORY_EMBEDDING_MODEL")

	defer func() {
		os.Setenv("MEMORY_ENABLE", origEnable)
		os.Setenv("MEMORY_LONG_TERM", origLongTerm)
		os.Setenv("MEMORY_STORAGE_DIR", origStorageDir)
		os.Setenv("MEMORY_EMBEDDING_MODEL", origEmbeddingModel)
	}()

	// 设置环境变量
	os.Setenv("MEMORY_ENABLE", "true")
	os.Setenv("MEMORY_LONG_TERM", "true")
	os.Setenv("MEMORY_STORAGE_DIR", "/custom/memory")
	os.Setenv("MEMORY_EMBEDDING_MODEL", "openai")

	cfg := LoadFromEnv()

	// 验证记忆配置
	if !cfg.Memory.Enable {
		t.Error("expected memory.enable to be true")
	}
	if !cfg.Memory.EnableLongTerm {
		t.Error("expected memory.enable_long_term to be true")
	}
	if cfg.Memory.EmbeddingModel != "openai" {
		t.Errorf("expected embedding_model 'openai', got '%s'", cfg.Memory.EmbeddingModel)
	}
}

// TestLoadFromEnv_FileAccessRestrictions 测试从环境变量加载文件访问限制配置
func TestLoadFromEnv_FileAccessRestrictions(t *testing.T) {
	cfg := LoadFromEnv()

	// 验证当前目录设置（通过 getCwd() 获取）
	if cfg.Tools.CurrentDir == "" {
		t.Error("expected non-empty current_dir")
	}

	// 验证默认限制文件访问
	if !cfg.Tools.RestrictFileAccess {
		t.Error("expected restrict_file_access to be true by default")
	}

	// 验证默认 AllowedDirs 为空
	if cfg.Tools.AllowedDirs != nil && len(cfg.Tools.AllowedDirs) != 0 {
		t.Errorf("expected empty allowed_dirs by default, got %v", cfg.Tools.AllowedDirs)
	}
}
