package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/internal/llm"
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
	llmClient  llm.Client
	offset     int
	mu         sync.Mutex
	// 消息处理中状态
	processing map[int64]bool
	// 用户会话映射
	userChats map[int64]string
}

// NewTelegramHandler 创建 Telegram 处理器
func NewTelegramHandler(cfg *TelegramConfig, sessMgr *session.Manager, client llm.Client) *TelegramHandler {
	return &TelegramHandler{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		sessionMgr:  sessMgr,
		llmClient:   client,
		offset:      0,
		processing:  make(map[int64]bool),
		userChats:   make(map[int64]string),
	}
}

// Start 启动 Telegram bot
func (h *TelegramHandler) Start(ctx context.Context) error {
	if h.config.BotToken == "" {
		return fmt.Errorf("telegram bot token is required")
	}

	if h.llmClient == nil {
		return fmt.Errorf("llm client is required")
	}

	// 获取 bot 信息
	botInfo, err := h.getMe(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}

	fmt.Printf("[Telegram] Bot @%s 已启动\n", botInfo.Username)

	// 启动轮询
	go h.poll(ctx)

	return nil
}

// getMe 获取 bot 信息
func (h *TelegramHandler) getMe(ctx context.Context) (*BotInfo, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", h.config.BotToken)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool     `json:"ok"`
		Result BotInfo  `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram api error")
	}

	return &result.Result, nil
}

// BotInfo Bot 信息
type BotInfo struct {
	ID        int    `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
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
		h.processUpdate(ctx, raw)
	}
}

// processUpdate 处理单条更新
func (h *TelegramHandler) processUpdate(ctx context.Context, raw json.RawMessage) {
	var update struct {
		UpdateID int `json:"update_id"`
		Message  struct {
			MessageID int `json:"message_id"`
			Chat      struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
			From struct {
				ID        int64  `json:"id"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Username  string `json:"username"`
			} `json:"from"`
			Text      string `json:"text"`
			Voice     *struct {
				FileID string `json:"file_id"`
			} `json:"voice"`
			Photo     *[]struct {
				FileID string `json:"file_id"`
			} `json:"photo"`
			Document *struct {
				FileID   string `json:"file_id"`
				FileName string `json:"file_name"`
			} `json:"document"`
		} `json:"message"`
		CallbackQuery *struct {
			ID      string `json:"id"`
			From    struct {
				ID int64 `json:"id"`
			} `json:"from"`
			Data string `json:"data"`
		} `json:"callback_query"`
	}

	if err := json.Unmarshal(raw, &update); err != nil {
		return
	}

	h.mu.Lock()
	h.offset = update.UpdateID + 1
	h.mu.Unlock()

	// 处理回调查询
	if update.CallbackQuery != nil {
		h.handleCallbackQuery(ctx, update.CallbackQuery)
		return
	}

	if update.Message.Text != "" {
		h.handleMessage(ctx, update.Message.Chat.ID, update.Message.Text, update.Message.From.FirstName)
	} else if update.Message.Voice != nil {
		h.handleVoice(ctx, update.Message.Chat.ID, update.Message.Voice.FileID)
	} else if update.Message.Photo != nil && len(*update.Message.Photo) > 0 {
		h.handlePhoto(ctx, update.Message.Chat.ID, (*update.Message.Photo)[0].FileID)
	} else if update.Message.Document != nil {
		h.handleDocument(ctx, update.Message.Chat.ID, update.Message.Document.FileID, update.Message.Document.FileName)
	}
}

// handleCallbackQuery 处理回调查询
func (h *TelegramHandler) handleCallbackQuery(ctx context.Context, cq *struct {
	ID      string `json:"id"`
	From    struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Data string `json:"data"`
}) {
	// 回答回调
	h.answerCallback(ctx, cq.ID, "")
}

// answerCallback 回答回调
func (h *TelegramHandler) answerCallback(ctx context.Context, callbackID string, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", h.config.BotToken)

	body := map[string]interface{}{
		"callback_query_id": callbackID,
	}
	if text != "" {
		body["text"] = text
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	req.Header.Set("Content-Type", "application/json")
	h.httpClient.Do(req)
}

// handleMessage 处理文本消息
func (h *TelegramHandler) handleMessage(ctx context.Context, chatID int64, text string, firstName string) {
	// 检查是否正在处理
	h.mu.Lock()
	if h.processing[chatID] {
		h.mu.Unlock()
		return
	}
	h.processing[chatID] = true
	h.mu.Unlock()

	// 确保最后解锁
	defer func() {
		h.mu.Lock()
		h.processing[chatID] = false
		h.mu.Unlock()
	}()

	// 处理命令
	if strings.HasPrefix(text, "/") {
		h.handleCommand(ctx, chatID, text, firstName)
		return
	}

	// 发送"正在输入"状态
	h.sendChatAction(ctx, chatID, "typing")

	// 获取或创建会话
	sess, ok := h.sessionMgr.GetSession(fmt.Sprintf("tg_%d", chatID))
	if !ok {
		sess = h.sessionMgr.CreateSession(fmt.Sprintf("tg_%d", chatID))
		h.userChats[chatID] = sess.ID
	}

	// 调用 LLM
	response, err := sess.SendMessage(ctx, h.llmClient, text)
	if err != nil {
		h.sendMessage(ctx, chatID, fmt.Sprintf("抱歉，发生了错误: %v", err))
		return
	}

	// 发送回复
	if response != "" {
		h.sendMessage(ctx, chatID, response)
	}
}

// handleCommand 处理命令
func (h *TelegramHandler) handleCommand(ctx context.Context, chatID int64, cmd string, firstName string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/start", "/hello":
		h.sendMessage(ctx, chatID, fmt.Sprintf("你好 %s！我是 MyClawDBot，一个 AI 编程助手。\n\n发送任何问题，我会尽力帮助你。\n\n可用命令：\n/start - 显示此帮助\n/clear - 清除会话历史\n/new - 创建新会话", firstName))
	case "/help":
		h.sendMessage(ctx, chatID, `可用命令：
/start - 开始对话
/help - 显示帮助
/clear - 清除会话历史
/new - 创建新会话
/tools - 显示可用工具`)
	case "/clear":
		sess, ok := h.sessionMgr.GetSession(fmt.Sprintf("tg_%d", chatID))
		if ok {
			sess.ClearHistory()
		}
		h.sendMessage(ctx, chatID, "会话历史已清除")
	case "/new":
		newSess := h.sessionMgr.CreateSession(fmt.Sprintf("tg_%d_%d", chatID, time.Now().Unix()))
		h.userChats[chatID] = newSess.ID
		h.sendMessage(ctx, chatID, "已创建新会话")
	case "/tools":
		h.sendMessage(ctx, chatID, `可用工具：
- read: 读取文件
- write: 写入文件
- bash: 执行命令
- fetch: 获取网页
- search: 搜索网页`)
	default:
		h.sendMessage(ctx, chatID, fmt.Sprintf("未知命令: %s\n输入 /help 查看可用命令", parts[0]))
	}
}

// handleVoice 处理语音消息
func (h *TelegramHandler) handleVoice(ctx context.Context, chatID int64, fileID string) {
	h.sendMessage(ctx, chatID, "暂不支持语音消息，请发送文字消息")
}

// handlePhoto 处理图片消息
func (h *TelegramHandler) handlePhoto(ctx context.Context, chatID int64, fileID string) {
	h.sendMessage(ctx, chatID, "暂不支持图片消息，请发送文字消息")
}

// handleDocument 处理文档消息
func (h *TelegramHandler) handleDocument(ctx context.Context, chatID int64, fileID, fileName string) {
	h.sendMessage(ctx, chatID, fmt.Sprintf("收到文档: %s\n暂不支持文档处理", fileName))
}

// sendMessage 发送消息
func (h *TelegramHandler) sendMessage(ctx context.Context, chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", h.config.BotToken)

	// 限制消息长度
	if len(text) > 4000 {
		text = text[:4000] + "\n\n[消息过长，已截断]"
	}

	body := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
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

	// 检查响应
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("telegram api error")
	}

	return nil
}

// sendChatAction 发送聊天动作
func (h *TelegramHandler) sendChatAction(ctx context.Context, chatID int64, action string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendChatAction", h.config.BotToken)

	body := map[string]interface{}{
		"chat_id": chatID,
		"action":  action,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	req.Header.Set("Content-Type", "application/json")
	h.httpClient.Do(req)
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
func TelegramChannel(botToken string, sessMgr *session.Manager, client llm.Client) *TelegramHandler {
	return NewTelegramHandler(&TelegramConfig{BotToken: botToken}, sessMgr, client)
}
