package channel

import (
	"context"
)

// Message 渠道消息
type Message struct {
	Type      string
	From      string
	Content   string
	Timestamp int64
}

// Channel 渠道接口
type Channel interface {
	// Init 初始化渠道
	Init(cfg interface{}) error

	// SendMessage 发送消息
	SendMessage(ctx context.Context, target string, text string) error

	// Receive 接收消息
	Receive(ctx context.Context) (<-chan *Message, error)

	// Type 返回渠道类型
	Type() string
}

// ChannelType 渠道类型
type ChannelType string

const (
	ChannelTerminal ChannelType = "terminal"
	ChannelTelegram ChannelType = "telegram"
	ChannelDiscord  ChannelType = "discord"
	ChannelSlack    ChannelType = "slack"
	ChannelWeb      ChannelType = "web"
)

// NewChannel 创建渠道
func NewChannel(channelType ChannelType, cfg interface{}, sessMgr interface{}) (Channel, error) {
	switch channelType {
	case ChannelTelegram:
		handler := &TelegramHandler{}
		if err := handler.Init(cfg); err != nil {
			return nil, err
		}
		return handler, nil
	case ChannelWeb:
		// Web 渠道需要特殊处理，因为它是服务器而不是处理器
		return nil, ErrUnsupportedChannel
	default:
		return nil, ErrUnsupportedChannel
	}
}

// 错误定义
var (
	ErrUnsupportedChannel = &ChannelError{"unsupported channel type"}
)

type ChannelError struct {
	msg string
}

func (e *ChannelError) Error() string {
	return e.msg
}
