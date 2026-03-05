// Package channel 消息渠道包
// 提供 Slack 机器人功能
package channel

import (
	"context"
	"fmt"
	"strings"

	"github.com/slack-go/slack"
	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/logger"
	"github.com/yahao333/myclawdbot/internal/session"
)

// SlackChannel Slack 渠道
// 实现 Slack Bot 功能，支持消息处理和 Slash Commands
type SlackChannel struct {
	api       *slack.Client
	signinSecret string
	sessMgr   *session.Manager
	llmClient llm.Client
	channels  map[string]*slackChannelContext // channel id -> context
}

// slackChannelContext Slack 频道上下文
type slackChannelContext struct {
	session   *session.Session
	channelID string
}

// SlackConfig Slack 配置
type SlackConfig struct {
	BotToken       string // Slack Bot User OAuth Token
	SigninSecret   string // Signing Secret 用于验证请求
}

// NewSlackChannel 创建 Slack 渠道
func NewSlackChannel(token string, signinSecret string, sessMgr *session.Manager, client llm.Client) (*SlackChannel, error) {
	if token == "" {
		return nil, fmt.Errorf("slack bot token is required")
	}

	channel := &SlackChannel{
		api:          slack.New(token),
		signinSecret: signinSecret,
		sessMgr:      sessMgr,
		llmClient:    client,
		channels:     make(map[string]*slackChannelContext),
	}

	return channel, nil
}

// NewSlackChannelWithConfig 使用配置创建 Slack 渠道
func NewSlackChannelWithConfig(cfg SlackConfig, sessMgr *session.Manager, client llm.Client) (*SlackChannel, error) {
	return NewSlackChannel(cfg.BotToken, cfg.SigninSecret, sessMgr, client)
}

// Start 启动 Slack 渠道
func (s *SlackChannel) Start(ctx context.Context) error {
	// 获取 bot 信息
	authTest, err := s.api.AuthTest()
	if err != nil {
		return fmt.Errorf("slack auth test failed: %w", err)
	}

	logger.Default().Info("Slack bot started",
		logger.String("user", authTest.User),
		logger.String("team", authTest.Team),
	)

	// 保持运行
	<-ctx.Done()
	logger.Default().Info("Slack channel stopped")
	return nil
}

// HandleEvent 处理 Slack 事件
// 用于处理 RTM 或 Events API 事件
func (s *SlackChannel) HandleEvent(event interface{}) error {
	switch e := event.(type) {
	case *slack.MessageEvent:
		return s.handleMessage(e)
	case *slack.SlashCommand:
		return s.handleSlashCommand(e)
	default:
		return nil
	}
}

// handleMessage 处理消息事件
func (s *SlackChannel) handleMessage(event *slack.MessageEvent) error {
	// 忽略机器人自己的消息
	if event.User == "" || event.BotID != "" {
		return nil
	}

	// 获取消息文本（处理 @mention）
	text := event.Text
	botUserID := s.getBotUserID()

	// 如果消息提到了机器人，移除 mention
	mention := fmt.Sprintf("<@%s>", botUserID)
	if strings.Contains(text, mention) {
		text = strings.ReplaceAll(text, mention, "")
		text = strings.TrimSpace(text)
	}

	// 如果没有有效消息内容，跳过
	if text == "" {
		return nil
	}

	// 获取或创建会话
	ctx := s.getOrCreateChannelContext(event.Channel)

	// 发送消息到 LLM
	response, err := ctx.session.SendMessage(context.Background(), s.llmClient, text)
	if err != nil {
		logger.Default().Error("Failed to send message",
			logger.Err(err),
		)
		s.sendMessage(event.Channel, "抱歉，处理您的消息时发生错误。")
		return err
	}

	// 发送响应
	s.sendMessage(event.Channel, response)
	return nil
}

// handleSlashCommand 处理 Slash Command
func (s *SlackChannel) handleSlashCommand(cmd *slack.SlashCommand) error {
	switch cmd.Command {
	case "/chat":
		return s.handleChatSlashCommand(cmd)
	case "/clear":
		return s.handleClearSlashCommand(cmd)
	case "/history":
		return s.handleHistorySlashCommand(cmd)
	default:
		return nil
	}
}

func (s *SlackChannel) handleChatSlashCommand(cmd *slack.SlashCommand) error {
	text := strings.TrimSpace(cmd.Text)
	if text == "" {
		return s.sendSlashResponse(cmd, "请输入消息内容")
	}

	ctx := s.getOrCreateChannelContext(cmd.ChannelID)
	response, err := ctx.session.SendMessage(context.Background(), s.llmClient, text)
	if err != nil {
		return s.sendSlashResponse(cmd, "抱歉，处理您的消息时发生错误。")
	}

	return s.sendSlashResponse(cmd, response)
}

func (s *SlackChannel) handleClearSlashCommand(cmd *slack.SlashCommand) error {
	ctx := s.getOrCreateChannelContext(cmd.ChannelID)
	ctx.session.ClearHistory()
	return s.sendSlashResponse(cmd, "会话历史已清除")
}

func (s *SlackChannel) handleHistorySlashCommand(cmd *slack.SlashCommand) error {
	ctx := s.getOrCreateChannelContext(cmd.ChannelID)
	history := ctx.session.GetHistory()

	if len(history) == 0 {
		return s.sendSlashResponse(cmd, "暂无历史消息")
	}

	var sb strings.Builder
	sb.WriteString("最近消息:\n")
	for i, msg := range history {
		if i >= 10 {
			break
		}
		sb.WriteString(fmt.Sprintf("• %s: %s\n", msg.Role, truncateStr(msg.Content, 50)))
	}

	return s.sendSlashResponse(cmd, sb.String())
}

func (s *SlackChannel) sendSlashResponse(cmd *slack.SlashCommand, text string) error {
	_, _, err := s.api.PostMessage(cmd.ChannelID, slack.MsgOptionText(text, false))
	return err
}

// getOrCreateChannelContext 获取或创建频道上下文
func (s *SlackChannel) getOrCreateChannelContext(channelID string) *slackChannelContext {
	if ctx, ok := s.channels[channelID]; ok {
		return ctx
	}

	sess := s.sessMgr.CreateSession(channelID)
	ctx := &slackChannelContext{
		session:   sess,
		channelID: channelID,
	}
	s.channels[channelID] = ctx
	return ctx
}

// sendMessage 发送消息到频道
func (s *SlackChannel) sendMessage(channelID, text string) {
	_, _, err := s.api.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		logger.Default().Error("Failed to send message",
			logger.Err(err),
		)
	}
}

// getBotUserID 获取机器人用户 ID
func (s *SlackChannel) getBotUserID() string {
	authTest, err := s.api.AuthTest()
	if err != nil {
		return ""
	}
	return authTest.UserID
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
