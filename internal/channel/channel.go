// Package channel 消息渠道包
//
// 提供统一的渠道接口和多种消息渠道的实现，包括终端、Telegram、Discord、Slack 和 Web。
// 支持消息的收发、会话管理和多平台机器人功能。
package channel

import (
	"context"
)

// Message 渠道消息
//
// 表示从各个消息渠道接收或发送的消息结构。
// 包含消息类型、发送者、内容和时间戳信息。
type Message struct {
	Type      string // 消息类型：如 text、image、voice 等
	From      string // 发送者标识
	Content   string // 消息内容
	Timestamp int64  // 时间戳（毫秒）
}

// Channel 渠道接口
//
// 定义消息渠道的统一接口，支持消息的发送和接收。
// 各个平台（如 Telegram、Discord、Slack）需实现此接口。
type Channel interface {
	// Init 初始化渠道
	//
	// 使用给定的配置初始化渠道。配置格式因渠道类型而异。
	// 参数：
	//   - cfg: 渠道特定配置
	//
	// 返回：
	//   - error: 初始化失败时返回错误
	Init(cfg interface{}) error

	// SendMessage 发送消息
	//
	// 向指定目标发送文本消息。
	// 参数：
	//   - ctx: 上下文，用于控制请求超时和取消
	//   - target: 目标标识（如聊天室 ID、用户 ID 等）
	//   - text: 要发送的文本内容
	//
	// 返回：
	//   - error: 发送失败时返回错误
	SendMessage(ctx context.Context, target string, text string) error

	// Receive 接收消息
	//
	// 返回一个只读通道，用于接收到达的消息。
	// 注意：某些渠道（如 Telegram）可能使用轮询，此方法返回的通道可能不会被直接使用。
	// 参数：
	//   - ctx: 上下文
	//
	// 返回：
	//   - <-chan *Message: 消息通道
	//   - error: 接收失败时返回错误
	Receive(ctx context.Context) (<-chan *Message, error)

	// Type 返回渠道类型
	//
	// 返回渠道的唯一标识字符串。
	// 返回：
	//   - string: 渠道类型（如 "telegram"、"discord"、"slack"、"web"）
	Type() string
}

// ChannelType 渠道类型
//
// 定义支持的渠道类型枚举。
type ChannelType string

const (
	ChannelTerminal ChannelType = "terminal" // 终端渠道
	ChannelTelegram ChannelType = "telegram" // Telegram 渠道
	ChannelDiscord  ChannelType = "discord"  // Discord 渠道
	ChannelSlack    ChannelType = "slack"    // Slack 渠道
	ChannelWeb      ChannelType = "web"      // Web 渠道
)

// NewChannel 创建渠道
//
// 根据渠道类型创建相应的渠道实例。
// 参数：
//   - channelType: 渠道类型
//   - cfg: 渠道配置
//
// 返回：
//   - Channel: 创建的渠道实例
//   - error: 创建失败时返回错误
func NewChannel(channelType ChannelType, cfg interface{}) (Channel, error) {
	switch channelType {
	case ChannelTelegram:
		handler := &TelegramHandler{}
		if err := handler.Init(cfg); err != nil {
			return nil, err
		}
		return handler, nil
	case ChannelWeb:
		// Web 渠道需要通过 WebChannel 函数单独创建（需要 LLM client）
		// 此处返回错误，请使用 channel.WebChannel() 函数
		return nil, &ChannelError{"web channel requires WebChannel() function"}
	default:
		return nil, ErrUnsupportedChannel
	}
}

// ChannelError 渠道错误
//
// 表示渠道操作过程中发生的错误。
type ChannelError struct {
	msg string // 错误消息
}

// Error 返回错误消息字符串
func (e *ChannelError) Error() string {
	return e.msg
}

// 错误定义
var (
	ErrUnsupportedChannel = &ChannelError{"unsupported channel type"} // 不支持的渠道类型错误
)
