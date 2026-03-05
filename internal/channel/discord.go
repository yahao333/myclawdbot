// Package channel 消息渠道包
// 提供多种消息渠道的实现，包括终端、Telegram、Discord 等
package channel

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/session"
)

// DiscordChannel Discord 渠道
// 实现 Channel 接口，提供 Discord 机器人功能
type DiscordChannel struct {
	session    *discordgo.Session
	botToken   string
	sessMgr    *session.Manager
	llmClient  llm.Client
	channels   map[string]*discordChannelContext // channel id -> context
}

// discordChannelContext Discord 频道上下文
type discordChannelContext struct {
	session   *session.Session
	channelID string
}

// NewDiscordChannel 创建 Discord 渠道
// token: Discord Bot Token
// sessMgr: 会话管理器
// client: LLM 客户端
func NewDiscordChannel(token string, sessMgr *session.Manager, client llm.Client) (*DiscordChannel, error) {
	// 创建 Discord 会话
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	channel := &DiscordChannel{
		session:   session,
		botToken:  token,
		sessMgr:   sessMgr,
		llmClient: client,
		channels:  make(map[string]*discordChannelContext),
	}

	// 注册消息处理函数
	session.AddHandler(channel.handleMessage)

	// 设置意图
	session.Identify.Intents = discordgo.IntentsGuildMessages

	return channel, nil
}

// Start 启动 Discord 渠道
func (d *DiscordChannel) Start(ctx context.Context) error {
	// 打开 WebSocket 连接
	if err := d.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}

	log.Println("Discord channel started")

	// 等待上下文取消
	<-ctx.Done()

	// 关闭连接
	d.session.Close()
	log.Println("Discord channel stopped")

	return nil
}

// handleMessage 处理收到的消息
func (d *DiscordChannel) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 忽略机器人自己的消息
	if m.Author.Bot {
		return
	}

	// 获取或创建会话
	ctx := d.getOrCreateChannelContext(m.ChannelID)

	// 发送消息到 LLM
	response, err := ctx.session.SendMessage(context.Background(), d.llmClient, m.Content)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
		d.sendMessage(m.ChannelID, "抱歉，处理您的消息时发生错误。")
		return
	}

	// 发送响应
	d.sendMessage(m.ChannelID, response)
}

// getOrCreateChannelContext 获取或创建频道上下文
func (d *DiscordChannel) getOrCreateChannelContext(channelID string) *discordChannelContext {
	if ctx, ok := d.channels[channelID]; ok {
		return ctx
	}

	// 创建新会话
	sess := d.sessMgr.CreateSession(channelID)
	ctx := &discordChannelContext{
		session:   sess,
		channelID: channelID,
	}
	d.channels[channelID] = ctx
	return ctx
}

// sendMessage 发送消息到频道
func (d *DiscordChannel) sendMessage(channelID, content string) {
	// Discord 消息限制 2000 字符
	if len(content) > 2000 {
		// 分片发送
		for i := 0; i < len(content); i += 2000 {
			end := i + 2000
			if end > len(content) {
				end = len(content)
			}
			d.session.ChannelMessageSend(channelID, content[i:end])
			time.Sleep(500 * time.Millisecond) // 避免触发速率限制
		}
	} else {
		d.session.ChannelMessageSend(channelID, content)
	}
}

// DiscordConfig Discord 配置
type DiscordConfig struct {
	BotToken string
}

// NewDiscordChannelWithConfig 使用配置创建 Discord 渠道
func NewDiscordChannelWithConfig(cfg DiscordConfig, sessMgr *session.Manager, client llm.Client) (*DiscordChannel, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("discord bot token is required")
	}
	return NewDiscordChannel(cfg.BotToken, sessMgr, client)
}

// DiscordSlashCommands 注册 Discord Slash Commands
// 此功能需要 Bot 具备 application.commands 权限
func (d *DiscordChannel) RegisterSlashCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "chat",
			Description: "与 AI 对话",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "message",
					Description: "要发送的消息",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "clear",
			Description: "清除会话历史",
		},
		{
			Name:        "history",
			Description: "查看会话历史",
		},
	}

	_, err := d.session.ApplicationCommandBulkOverwrite("", "", commands)
	return err
}

// DiscordMessageHandler Discord 消息处理器
// 用于处理 slash commands 和其他事件
type DiscordMessageHandler struct {
	channel *DiscordChannel
}

// NewDiscordMessageHandler 创建消息处理器
func NewDiscordMessageHandler(channel *DiscordChannel) *DiscordMessageHandler {
	return &DiscordMessageHandler{channel: channel}
}

// HandleSlashCommand 处理 slash command
func (h *DiscordMessageHandler) HandleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch i.ApplicationCommandData().Name {
	case "chat":
		h.handleChatCommand(s, i)
	case "clear":
		h.handleClearCommand(s, i)
	case "history":
		h.handleHistoryCommand(s, i)
	}
}

func (h *DiscordMessageHandler) handleChatCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return
	}

	message := options[0].StringValue()

	// 响应 defer
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// 获取会话
	ctx := h.channel.getOrCreateChannelContext(i.ChannelID)

	// 发送消息
	response, err := ctx.session.SendMessage(context.Background(), h.channel.llmClient, message)
	if err != nil {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: PtrString("抱歉，处理您的消息时发生错误。"),
		})
		return
	}

	// 发送响应
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: PtrString(response),
	})
}

func (h *DiscordMessageHandler) handleClearCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := h.channel.getOrCreateChannelContext(i.ChannelID)
	ctx.session.ClearHistory()

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "会话历史已清除",
		},
	})
}

func (h *DiscordMessageHandler) handleHistoryCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := h.channel.getOrCreateChannelContext(i.ChannelID)
	history := ctx.session.GetHistory()

	if len(history) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "暂无历史消息",
			},
		})
		return
	}

	// 构建历史消息
	var sb strings.Builder
	sb.WriteString("最近消息:\n")
	for i, msg := range history {
		if i >= 10 { // 限制显示最近 10 条
			break
		}
		sb.WriteString(fmt.Sprintf("**%s**: %s\n", msg.Role, truncate(msg.Content, 50)))
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: sb.String(),
		},
	})
}

// PtrString 返回字符串指针
func PtrString(s string) *string {
	return &s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
