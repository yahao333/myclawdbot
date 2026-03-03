package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/internal/session"
)

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
}

// TelegramHandler Telegram 消息处理器
type TelegramHandler struct {
	config     *TelegramConfig
	httpClient *http.Client
	sessionMgr *session.Manager
	offset     int
	mu         sync.Mutex
}

// NewTelegramHandler 创建 Telegram 处理器
func NewTelegramHandler(cfg *TelegramConfig, sessMgr *session.Manager) *TelegramHandler {
	return &TelegramHandler{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		sessionMgr: sessMgr,
		offset:     0,
	}
}

// Start 启动 Telegram bot
func (h *TelegramHandler) Start(ctx context.Context) error {
	if h.config.BotToken == "" {
		return fmt.Errorf("telegram bot token is required")
	}

	// 获取 bot 信息
	if err := h.getMe(ctx); err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}

	// 启动轮询
	go h.poll(ctx)

	return nil
}

// getMe 获取 bot 信息
func (h *TelegramHandler) getMe(ctx context.Context) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", h.config.BotToken)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("telegram api error: %s", result.Description)
	}

	return nil
}

// poll 轮询获取更新
func (h *TelegramHandler) poll(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.getUpdates(ctx)
		}
	}
}

// getUpdates 获取更新
func (h *TelegramHandler) getUpdates(ctx context.Context) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=60&offset=%d",
		h.config.BotToken, h.offset)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool              `json:"ok"`
		Result []json.RawMessage `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if !result.OK || len(result.Result) == 0 {
		return
	}

	// 处理更新
	for _, raw := range result.Result {
		var update struct {
			UpdateID int `json:"update_id"`
			Message  struct {
				MessageID int    `json:"message_id"`
				Chat      struct {
					ID int64 `json:"id"`
				} `json:"chat"`
				From struct {
					ID        int64  `json:"id"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
					Username  string `json:"username"`
				} `json:"from"`
				Text string `json:"text"`
			} `json:"message"`
		}

		if err := json.Unmarshal(raw, &update); err != nil {
			continue
		}

		h.offset = update.UpdateID + 1

		if update.Message.Text != "" {
			h.handleMessage(ctx, update.Message.Chat.ID, update.Message.Text)
		}
	}
}

// handleMessage 处理消息
func (h *TelegramHandler) handleMessage(ctx context.Context, chatID int64, text string) {
	// 获取或创建会话
	sess, ok := h.sessionMgr.GetSession(fmt.Sprintf("tg_%d", chatID))
	if !ok {
		sess = h.sessionMgr.CreateSession(fmt.Sprintf("tg_%d", chatID))
	}

	// 发送消息
	h.sendMessage(ctx, chatID, "处理中...")

	// 这里需要获取 LLM 客户端来发送消息
	// 实际使用中需要通过回调或全局变量传入 LLM 客户端
	_ = sess
}

// sendMessage 发送消息
func (h *TelegramHandler) sendMessage(ctx context.Context, chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", h.config.BotToken)

	body := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Send 发送消息到指定会话
func (h *TelegramHandler) Send(ctx context.Context, chatID int64, text string) error {
	return h.sendMessage(ctx, chatID, text)
}

// Channel 接口实现
var _ Channel = (*TelegramHandler)(nil)

// Init 初始化
func (h *TelegramHandler) Init(cfg interface{}) error {
	if c, ok := cfg.(TelegramConfig); ok {
		h.config = &c
		return nil
	}
	return fmt.Errorf("invalid telegram config")
}

// SendMessage 发送消息
func (h *TelegramHandler) SendMessage(ctx context.Context, target string, text string) error {
	var chatID int64
	if _, err := fmt.Sscanf(target, "%d", &chatID); err != nil {
		return err
	}
	return h.sendMessage(ctx, chatID, text)
}

// Receive 接收消息 (Telegram 使用轮询，这个方法不会被直接调用)
func (h *TelegramHandler) Receive(ctx context.Context) (<-chan *Message, error) {
	ch := make(chan *Message, 10)
	return ch, nil
}

// Type 返回渠道类型
func (h *TelegramHandler) Type() string {
	return "telegram"
}

// TelegramChannel Telegram 渠道构造函数
func TelegramChannel(botToken string, sessMgr *session.Manager) *TelegramHandler {
	return NewTelegramHandler(&TelegramConfig{BotToken: botToken}, sessMgr)
}
