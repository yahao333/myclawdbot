// Package channel 消息渠道包
//
// 提供多种消息渠道的实现，包括终端、Telegram、Discord、Slack 和 Web。
package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/session"
)

const (
	wsWriteWait  = 10 * time.Second // WebSocket 写入超时
	wsPongWait   = 60 * time.Second // WebSocket pong 等待时间
	wsPingPeriod = (wsPongWait * 9) / 10 // WebSocket ping 周期
	wsMaxMsgSize = 512 * 1024 // WebSocket 最大消息大小（512KB）
)

// WebConfig Web 渠道配置
//
// 配置 Web 服务器的各项参数，包括监听地址、端口、静态文件目录等。
type WebConfig struct {
	Host          string `json:"host"`          // 监听地址，默认为 "0.0.0.0"
	Port          int    `json:"port"`          // 监听端口，默认为 8080
	StaticDir     string `json:"static_dir"`    // 静态文件目录路径
	EnableUpload  bool   `json:"enable_upload"` // 是否启用文件上传
	MaxUploadSize int64  `json:"max_upload_size"` // 最大上传文件大小（字节），默认为 10MB
	CORSEnabled   bool   `json:"cors_enabled"`  // 是否启用 CORS
	AuthEnabled   bool   `json:"auth_enabled"`  // 是否启用认证
	AuthToken     string `json:"auth_token"`    // 认证令牌
}

// WebHandler Web 消息处理器
//
// 实现 Channel 接口，提供基于 HTTP/WebSocket 的 Web 聊天界面。
// 支持实时消息推送、会话管理和 REST API。
type WebHandler struct {
	config     *WebConfig // Web 配置
	httpServer *http.Server // HTTP 服务器
	sessionMgr *session.Manager // 会话管理器
	client     llm.Client // LLM 客户端
	upgrader   websocket.Upgrader // WebSocket 升级器
	clients    map[string]*WebClient // 连接的客户端映射
	clientsMu  sync.RWMutex // 客户端映射的互斥锁
	stopChan   chan struct{} // 停止信号通道
}

// WebClient WebSocket 客户端
//
// 表示一个连接的 WebSocket 客户端，包含连接、会话等信息。
type WebClient struct {
	conn      *websocket.Conn // WebSocket 连接
	send      chan []byte // 发送消息通道
	sessionID string // 关联的会话 ID
}

// WebMessage Web 消息结构
//
// 用于 WebSocket 通信的消息格式。
type WebMessage struct {
	Type      string          `json:"type"`       // 消息类型：message、session_created、history、error 等
	Content   string          `json:"content,omitempty"` // 消息内容
	SessionID string          `json:"session_id,omitempty"` // 会话 ID
	Data      json.RawMessage `json:"data,omitempty"` // 附加数据
}

// NewWebHandler 创建 Web 处理器
//
// 使用给定的配置创建 Web 处理器。
// 参数：
//   - cfg: Web 配置
//   - sessMgr: 会话管理器
//   - client: LLM 客户端
//
// 返回：
//   - *WebHandler: 创建的 Web 处理器
func NewWebHandler(cfg *WebConfig, sessMgr *session.Manager, client llm.Client) *WebHandler {
	// 设置默认值
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.MaxUploadSize == 0 {
		cfg.MaxUploadSize = 10 * 1024 * 1024
	}

	handler := &WebHandler{
		config:     cfg,
		sessionMgr: sessMgr,
		client:     client,
		clients:    make(map[string]*WebClient),
		stopChan:   make(chan struct{}),
	}

	// 配置 WebSocket 升级器
	handler.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// 如果启用了 CORS，允许所有来源
			if cfg.CORSEnabled {
				return true
			}
			// 如果未启用 CORS，允许同源请求
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // 同源请求没有 Origin 头
			}
			// 检查是否同源
			return origin == "http://"+r.Host || origin == "https://"+r.Host
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	return handler
}

// Start 启动 Web 服务器
//
// 启动 HTTP 服务器并开始处理请求。
// 参数：
//   - ctx: 上下文，用于控制服务器生命周期
//
// 返回：
//   - error: 启动失败时返回错误
func (h *WebHandler) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// 注册路由
	mux.HandleFunc("/api/chat", h.handleChat) // 聊天接口
	mux.HandleFunc("/api/chat/stream", h.handleChatStream) // 流式聊天接口
	mux.HandleFunc("/api/session/create", h.handleCreateSession) // 创建会话
	mux.HandleFunc("/api/session/delete", h.handleDeleteSession) // 删除会话
	mux.HandleFunc("/api/session/history", h.handleGetHistory) // 获取历史
	mux.HandleFunc("/api/tools", h.handleListTools) // 工具列表
	mux.HandleFunc("/ws", h.handleWebSocket) // WebSocket 接口
	mux.HandleFunc("/", h.handleIndex) // 首页

	h.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", h.config.Host, h.config.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 启动服务器（非阻塞）
	go func() {
		log.Printf("[Web] 服务器启动于 http://%s:%d", h.config.Host, h.config.Port)
		if err := h.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Web] 服务器错误: %v", err)
		}
	}()

	// 监听上下文取消以停止服务器
	go func() {
		<-ctx.Done()
		h.Stop()
	}()

	return nil
}

// Stop 停止 Web 服务器
//
// 优雅地停止 HTTP 服务器并关闭所有 WebSocket 连接。
func (h *WebHandler) Stop() {
	close(h.stopChan)
	if h.httpServer != nil {
		h.httpServer.Shutdown(context.Background())
	}

	// 关闭所有客户端连接
	h.clientsMu.Lock()
	for _, client := range h.clients {
		close(client.send)
	}
	h.clientsMu.Unlock()

	log.Println("[Web] 服务器已停止")
}

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MyClawDBot - AI 编程助手</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #1a1a2e; color: #eee; height: 100vh; display: flex; flex-direction: column; }
        header { background: #16213e; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #0f3460; }
        header h1 { font-size: 1.5rem; color: #e94560; }
        header .status { font-size: 0.875rem; color: #4caf50; }
        .container { flex: 1; display: flex; overflow: hidden; }
        .sidebar { width: 250px; background: #16213e; padding: 1rem; border-right: 1px solid #0f3460; display: flex; flex-direction: column; }
        .sidebar h3 { font-size: 0.875rem; color: #888; margin-bottom: 0.5rem; }
        .session-list { flex: 1; overflow-y: auto; }
        .session-item { padding: 0.75rem; margin-bottom: 0.5rem; background: #0f3460; border-radius: 8px; cursor: pointer; transition: background 0.2s; }
        .session-item:hover { background: #1a4a7a; }
        .session-item.active { background: #e94560; }
        .new-session { padding: 0.75rem; background: #0f3460; border: 2px dashed #0f3460; border-radius: 8px; text-align: center; cursor: pointer; color: #888; transition: all 0.2s; }
        .new-session:hover { border-color: #e94560; color: #e94560; }
        .chat-area { flex: 1; display: flex; flex-direction: column; }
        .messages { flex: 1; overflow-y: auto; padding: 1rem; }
        .message { margin-bottom: 1rem; display: flex; gap: 0.75rem; }
        .message.user { flex-direction: row-reverse; }
        .message-avatar { width: 36px; height: 36px; border-radius: 50%; background: #e94560; display: flex; align-items: center; justify-content: center; font-weight: bold; flex-shrink: 0; }
        .message.user .message-avatar { background: #4caf50; }
        .message-content { max-width: 70%; background: #16213e; padding: 0.75rem 1rem; border-radius: 12px; }
        .message.user .message-content { background: #0f3460; }
        .message-time { font-size: 0.75rem; color: #666; margin-top: 0.25rem; }
        .input-area { padding: 1rem; background: #16213e; border-top: 1px solid #0f3460; display: flex; gap: 0.5rem; }
        .input-area textarea { flex: 1; background: #0f3460; border: none; border-radius: 8px; padding: 0.75rem; color: #eee; font-size: 1rem; resize: none; }
        .input-area textarea:focus { outline: 2px solid #e94560; }
        .input-area button { background: #e94560; border: none; border-radius: 8px; padding: 0.75rem 1.5rem; color: white; font-size: 1rem; cursor: pointer; transition: background 0.2s; }
        .input-area button:hover { background: #c73e54; }
        .typing { padding: 0.5rem 1rem; color: #888; font-style: italic; display: none; }
        .typing.active { display: block; }
    </style>
</head>
<body>
    <header>
        <h1>MyClawDBot</h1>
        <span class="status">Online</span>
    </header>
    <div class="container">
        <div class="sidebar">
            <h3>Sessions</h3>
            <div class="session-list" id="sessionList"></div>
            <div class="new-session" onclick="createNewSession()">+ New Session</div>
        </div>
        <div class="chat-area">
            <div class="messages" id="messages"></div>
            <div class="typing" id="typing">Thinking...</div>
            <div class="input-area">
                <textarea id="input" placeholder="Type a message..." rows="1"></textarea>
                <button id="sendBtn" onclick="sendMessage()">Send</button>
            </div>
        </div>
    </div>
    <script>
        let ws = null;
        let currentSession = null;
        const sessions = {};

        function connect() {
            ws = new WebSocket('ws://' + location.host + '/ws');
            ws.onopen = () => {
                console.log('Connected');
                // 自动创建会话
                createNewSession();
            };
            ws.onclose = () => { console.log('Disconnected'); setTimeout(connect, 3000); };
            ws.onerror = (e) => console.error('WS Error:', e);
            ws.onmessage = (e) => {
                const msg = JSON.parse(e.data);
                handleMessage(msg);
            };
        }

        function handleMessage(msg) {
            switch(msg.type) {
                case 'message':
                    if (msg.session_id === currentSession) {
                        addMessage('bot', msg.content);
                        document.getElementById('typing').classList.remove('active');
                    }
                    break;
                case 'session_created':
                    currentSession = msg.session_id;
                    sessions[msg.session_id] = [];
                    break;
                case 'history':
                    sessions[msg.session_id] = msg.messages || [];
                    break;
                case 'error':
                    alert('Error: ' + msg.content);
                    break;
            }
        }

        function sendMessage() {
            const input = document.getElementById('input');
            const content = input.value.trim();
            if (!content || !currentSession) return;

            addMessage('user', content);
            input.value = '';

            ws.send(JSON.stringify({
                type: 'message',
                session_id: currentSession,
                content: content
            }));

            document.getElementById('typing').classList.add('active');
        }

        function addMessage(role, content) {
            const container = document.getElementById('messages');
            const div = document.createElement('div');
            div.className = 'message ' + role;
            div.innerHTML = '<div class="message-avatar">' + (role === 'user' ? 'U' : 'B') + '</div><div class="message-content"><div>' + content.replace(/</g, '&lt;').replace(/>/g, '&gt;') + '</div><div class="message-time">' + new Date().toLocaleTimeString() + '</div></div>';
            container.appendChild(div);
            container.scrollTop = container.scrollHeight;
        }

        function createNewSession() {
            ws.send(JSON.stringify({ type: 'create_session' }));
        }

        function selectSession(id) {
            currentSession = id;
            document.getElementById('messages').innerHTML = '';
            if (sessions[id]) {
                sessions[id].forEach(m => addMessage(m.role, m.content));
            }
            ws.send(JSON.stringify({ type: 'get_history', session_id: id }));
        }

        document.getElementById('input').addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });

        connect();
    </script>
</body>
</html>`

func (h *WebHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
	// 返回内置的 HTML 页面
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

// handleChat 处理聊天请求
//
// 处理 POST 请求，接收消息并调用 LLM 生成回复。
func (h *WebHandler) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Content   string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 如果没有提供会话 ID，则创建一个新的
	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("web_%d", time.Now().UnixMilli())
	}

	// 获取或创建会话
	sess, _ := h.sessionMgr.GetSession(req.SessionID)
	if sess == nil {
		sess = h.sessionMgr.CreateSession(req.SessionID)
	}

	ctx := r.Context()
	response, err := sess.SendMessage(ctx, h.client, req.Content)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"session_id": sess.ID,
		"response":   response,
	})
}

// handleChatStream 处理流式聊天请求
//
// 使用 Server-Sent Events (SSE) 实现流式响应。
func (h *WebHandler) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var req struct {
		SessionID string `json:"session_id"`
		Content   string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Fprintf(w, "data: %s\n\n", `{"error": "bad request"}`)
		return
	}

	// 如果没有提供会话 ID，则创建一个新的
	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("web_%d", time.Now().UnixMilli())
	}

	// 获取或创建会话
	sess, _ := h.sessionMgr.GetSession(req.SessionID)
	if sess == nil {
		sess = h.sessionMgr.CreateSession(req.SessionID)
	}

	ctx := r.Context()

	// 首先发送会话 ID
	fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"session_id": "%s", "type": "session"}`, sess.ID))

	// 使用流式发送消息
	sess.SendMessageStream(ctx, h.client, req.Content, func(delta string) {
		fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"type": "content", "content": %s}`, delta))
	})

	// 发送完成信号
	fmt.Fprintf(w, "data: %s\n\n", `{"type": "done"}`)
}

// handleCreateSession 处理创建会话请求
func (h *WebHandler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("web_%d", time.Now().UnixMilli())
	h.sessionMgr.CreateSession(sessionID)
	json.NewEncoder(w).Encode(map[string]string{"session_id": sessionID})
}

// handleDeleteSession 处理删除会话请求
func (h *WebHandler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID != "" {
		h.sessionMgr.DeleteSession(sessionID)
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HistoryResponse 历史消息响应结构
type HistoryResponse struct {
	SessionID string      `json:"session_id"` // 会话 ID
	Messages  interface{} `json:"messages"`    // 消息历史
}

// handleGetHistory 处理获取历史消息请求
func (h *WebHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	sess, ok := h.sessionMgr.GetSession(sessionID)
	if !ok {
		resp := HistoryResponse{Messages: []interface{}{}}
		json.NewEncoder(w).Encode(resp)
		return
	}

	history := sess.GetHistory()
	resp := HistoryResponse{
		SessionID: sessionID,
		Messages:  history,
	}
	json.NewEncoder(w).Encode(resp)
}

// handleListTools 处理获取可用工具列表请求
func (h *WebHandler) handleListTools(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": []string{"read", "write", "bash", "terminal", "fetch", "search"},
	})
}

// handleWebSocket 处理 WebSocket 连接
//
// 将 HTTP 连接升级为 WebSocket 连接，并创建客户端进行处理。
func (h *WebHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Web] WebSocket upgrade failed: %v", err)
		return
	}

	// 创建客户端
	client := &WebClient{
		conn:      conn,
		send:      make(chan []byte, 256),
		sessionID: fmt.Sprintf("web_%d", time.Now().UnixMilli()),
	}

	// 注册客户端
	h.clientsMu.Lock()
	h.clients[client.sessionID] = client
	h.clientsMu.Unlock()

	// 确保在函数退出时移除客户端
	defer func() {
		h.clientsMu.Lock()
		delete(h.clients, client.sessionID)
		h.clientsMu.Unlock()
		close(client.send)
	}()

	// 启动读写协程
	go h.writePump(client)
	h.readPump(client)
}

// readPump WebSocket 读取泵
//
// 从 WebSocket 连接读取消息并处理。
func (h *WebHandler) readPump(client *WebClient) {
	defer client.conn.Close()

	client.conn.SetReadLimit(wsMaxMsgSize)
	client.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			// 检查是否是意外的连接关闭
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Web] WebSocket error: %v", err)
			}
			break
		}

		var msg WebMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			h.sendToClient(client, map[string]string{"type": "error", "content": "parse error"})
			continue
		}

		h.handleClientMessage(client, &msg)
	}
}

// writePump WebSocket 写入泵
//
// 向 WebSocket 连接写入消息，包括心跳。
func (h *WebHandler) writePump(client *WebClient) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				// 通道已关闭，发送关闭消息
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			// 发送 ping 以保持连接活跃
			client.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleClientMessage 处理来自客户端的消息
func (h *WebHandler) handleClientMessage(client *WebClient, msg *WebMessage) {
	ctx := context.Background()

	switch msg.Type {
	case "message":
		sessionID := msg.SessionID
		if sessionID == "" {
			sessionID = client.sessionID
		}

		sess, _ := h.sessionMgr.GetSession(sessionID)
		if sess == nil {
			sess = h.sessionMgr.CreateSession(sessionID)
		}

		// 异步处理消息并发送响应
		go func() {
			response, err := sess.SendMessage(ctx, h.client, msg.Content)
			if err != nil {
				h.sendToClient(client, map[string]string{"type": "error", "content": err.Error()})
				return
			}

			h.sendToClient(client, map[string]string{
				"type":       "message",
				"session_id": sessionID,
				"content":    response,
			})
		}()

	case "get_history":
		sess, ok := h.sessionMgr.GetSession(msg.SessionID)
		if ok {
			history := sess.GetHistory()
			h.sendToClient(client, map[string]interface{}{
				"type":       "history",
				"session_id": msg.SessionID,
				"messages":   history,
			})
		}

	case "create_session":
		sessionID := fmt.Sprintf("web_%d", time.Now().UnixMilli())
		h.sessionMgr.CreateSession(sessionID)
		client.sessionID = sessionID
		h.sendToClient(client, map[string]string{
			"type":       "session_created",
			"session_id": sessionID,
		})
	}
}

// sendToClient 向客户端发送消息
//
// 线程安全地发送消息到客户端的发送通道。
func (h *WebHandler) sendToClient(client *WebClient, data interface{}) {
	msg, _ := json.Marshal(data)
	select {
	case client.send <- msg:
	default:
		log.Printf("[Web] client send queue full")
	}
}

// Init 实现 Channel 接口
func (h *WebHandler) Init(cfg interface{}) error {
	if c, ok := cfg.(WebConfig); ok {
		h.config = &c
		return nil
	}
	return fmt.Errorf("invalid web config")
}

// SendMessage 实现 Channel 接口
func (h *WebHandler) SendMessage(ctx context.Context, target string, text string) error {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	client, ok := h.clients[target]
	if !ok {
		return fmt.Errorf("session not found: %s", target)
	}

	h.sendToClient(client, map[string]string{
		"type":    "message",
		"content": text,
	})
	return nil
}

// Receive 实现 Channel 接口
func (h *WebHandler) Receive(ctx context.Context) (<-chan *Message, error) {
	ch := make(chan *Message, 10)
	return ch, nil
}

// Type 实现 Channel 接口
func (h *WebHandler) Type() string {
	return "web"
}

// WebChannel 创建 Web 渠道的便捷函数
func WebChannel(host string, port int, sessMgr *session.Manager, client llm.Client) *WebHandler {
	return NewWebHandler(&WebConfig{Host: host, Port: port}, sessMgr, client)
}
