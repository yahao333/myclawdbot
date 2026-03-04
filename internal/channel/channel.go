package channel

import (
	"context"

	"github.com/yahao333/myclawdbot/internal/session"
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
		webCfg := WebConfig{}
		switch c := cfg.(type) {
		case nil:
		case WebConfig:
			webCfg = c
		case *WebConfig:
			if c != nil {
				webCfg = *c
			}
		default:
			return nil, &ChannelError{"invalid web config"}
		}

		var manager *session.Manager
		switch m := sessMgr.(type) {
		case *session.Manager:
			manager = m
		case nil:
		default:
			return nil, &ChannelError{"invalid session manager"}
		}

		handler := NewWebHandler(&webCfg, manager)
		if err := handler.Init(webCfg); err != nil {
			return nil, err
		}
		return handler, nil
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
