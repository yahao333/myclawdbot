package channel

import (
	"context"
	"testing"
)

func TestNewChannel(t *testing.T) {
	tests := []struct {
		name        string
		channelType ChannelType
		cfg         interface{}
		wantError   bool
	}{
		{
			name:        "telegram with valid config",
			channelType: ChannelTelegram,
			cfg:         TelegramConfig{BotToken: "test-token"},
			wantError:   false,
		},
		{
			name:        "web requires dedicated constructor",
			channelType: ChannelWeb,
			cfg:         WebConfig{Host: "127.0.0.1", Port: 8080},
			wantError:   true,
		},
		{
			name:        "unsupported channel",
			channelType: ChannelDiscord,
			cfg:         nil,
			wantError:   true,
		},
		{
			name:        "invalid config type",
			channelType: ChannelTelegram,
			cfg:         "invalid",
			wantError:   true,
		},
		{
			name:        "web invalid config type",
			channelType: ChannelWeb,
			cfg:         "invalid",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := NewChannel(tt.channelType, tt.cfg)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if ch == nil {
				t.Error("expected non-nil channel")
			}
		})
	}
}

func TestChannelError(t *testing.T) {
	err := &ChannelError{"test error"}
	if err.Error() != "test error" {
		t.Errorf("Error() = %s, want 'test error'", err.Error())
	}
}

func TestTelegramHandler_Init(t *testing.T) {
	handler := &TelegramHandler{}

	// 测试有效配置
	cfg := TelegramConfig{BotToken: "test-token"}
	err := handler.Init(cfg)
	if err != nil {
		t.Errorf("Init() with valid config error: %v", err)
	}
	if handler.config.BotToken != "test-token" {
		t.Errorf("BotToken = %s, want 'test-token'", handler.config.BotToken)
	}

	// 测试无效配置类型
	err = handler.Init("invalid")
	if err == nil {
		t.Error("expected error with invalid config")
	}
}

func TestTelegramHandler_Type(t *testing.T) {
	handler := &TelegramHandler{}
	if handler.Type() != "telegram" {
		t.Errorf("Type() = %s, want 'telegram'", handler.Type())
	}
}

func TestTelegramHandler_Receive(t *testing.T) {
	handler := &TelegramHandler{}
	ctx := context.Background()

	ch, err := handler.Receive(ctx)
	if err != nil {
		t.Errorf("Receive() error: %v", err)
	}
	if ch == nil {
		t.Error("expected non-nil channel")
	}
}

func TestTelegramHandler_SendMessage(t *testing.T) {
	handler := &TelegramHandler{
		config: &TelegramConfig{BotToken: "test-token"},
	}

	ctx := context.Background()

	// 测试无效的 target (非数字)
	err := handler.SendMessage(ctx, "invalid", "test message")
	if err == nil {
		t.Error("expected error with invalid target")
	}
}

func TestChannelTypes(t *testing.T) {
	if ChannelTerminal != "terminal" {
		t.Errorf("ChannelTerminal = %s, want 'terminal'", ChannelTerminal)
	}
	if ChannelTelegram != "telegram" {
		t.Errorf("ChannelTelegram = %s, want 'telegram'", ChannelTelegram)
	}
	if ChannelDiscord != "discord" {
		t.Errorf("ChannelDiscord = %s, want 'discord'", ChannelDiscord)
	}
	if ChannelSlack != "slack" {
		t.Errorf("ChannelSlack = %s, want 'slack'", ChannelSlack)
	}
	if ChannelWeb != "web" {
		t.Errorf("ChannelWeb = %s, want 'web'", ChannelWeb)
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Type:      "text",
		From:      "user123",
		Content:   "Hello world",
		Timestamp: 1234567890,
	}

	if msg.Type != "text" {
		t.Errorf("Type = %s, want 'text'", msg.Type)
	}
	if msg.From != "user123" {
		t.Errorf("From = %s, want 'user123'", msg.From)
	}
	if msg.Content != "Hello world" {
		t.Errorf("Content = %s, want 'Hello world'", msg.Content)
	}
	if msg.Timestamp != 1234567890 {
		t.Errorf("Timestamp = %d, want 1234567890", msg.Timestamp)
	}
}
