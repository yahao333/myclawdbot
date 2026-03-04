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
	"github.com/yahao333/myclawdbot/internal/session"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = (wsPongWait * 9) / 10
	wsMaxMsgSize = 512 * 1024
)

// WebConfig Web 渠道配置
type WebConfig struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	StaticDir     string `json:"static_dir"`
	EnableUpload  bool   `json:"enable_upload"`
	MaxUploadSize int64  `json:"max_upload_size"`
	CORSEnabled   bool   `json:"cors_enabled"`
	AuthEnabled   bool   `json:"auth_enabled"`
	AuthToken     string `json:"auth_token"`
}

// WebHandler Web 消息处理器
type WebHandler struct {
	config     *WebConfig
	httpServer *http.Server
	sessionMgr *session.Manager
	upgrader   websocket.Upgrader
	clients    map[string]*WebClient
	clientsMu  sync.RWMutex
	stopChan   chan struct{}
}

// WebClient WebSocket 客户端
type WebClient struct {
	conn      *websocket.Conn
	send      chan []byte
	sessionID string
}

// WebMessage Web 消息结构
type WebMessage struct {
	Type      string          `json:"type"`
	Content   string          `json:"content,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// NewWebHandler 创建 Web 处理器
func NewWebHandler(cfg *WebConfig, sessMgr *session.Manager) *WebHandler {
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
		clients:   make(map[string]*WebClient),
		stopChan:  make(chan struct{}),
	}

	handler.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return cfg.CORSEnabled
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	return handler
}

// Start 启动 Web 服务器
func (h *WebHandler) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/chat", h.handleChat)
	mux.HandleFunc("/api/chat/stream", h.handleChatStream)
	mux.HandleFunc("/api/session/create", h.handleCreateSession)
	mux.HandleFunc("/api/session/delete", h.handleDeleteSession)
	mux.HandleFunc("/api/session/history", h.handleGetHistory)
	mux.HandleFunc("/api/tools", h.handleListTools)
	mux.HandleFunc("/ws", h.handleWebSocket)
	mux.HandleFunc("/", h.handleIndex)

	h.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", h.config.Host, h.config.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("[Web] 服务器启动于 http://%s:%d", h.config.Host, h.config.Port)
		if err := h.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Web] 服务器错误: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		h.Stop()
	}()

	return nil
}

// Stop 停止 Web 服务器
func (h *WebHandler) Stop() {
	close(h.stopChan)
	if h.httpServer != nil {
		h.httpServer.Shutdown(context.Background())
	}

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
            ws.onopen = () => console.log('Connected');
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

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

	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("web_%d", time.Now().UnixMilli())
	}

	sess, _ := h.sessionMgr.GetSession(req.SessionID)
	if sess == nil {
		sess = h.sessionMgr.CreateSession(req.SessionID)
	}

	ctx := r.Context()
	response, err := sess.SendMessage(ctx, nil, req.Content)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"session_id": sess.ID,
		"response":   response,
	})
}

func (h *WebHandler) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("web_%d", time.Now().UnixMilli())
	}

	sess, _ := h.sessionMgr.GetSession(req.SessionID)
	if sess == nil {
		sess = h.sessionMgr.CreateSession(req.SessionID)
	}

	ctx := r.Context()

	fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"session_id": "%s", "type": "session"}`, sess.ID))

	sess.SendMessageStream(ctx, nil, req.Content, func(delta string) {
		fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"type": "content", "content": %s}`, delta))
	})

	fmt.Fprintf(w, "data: %s\n\n", `{"type": "done"}`)
}

func (h *WebHandler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("web_%d", time.Now().UnixMilli())
	h.sessionMgr.CreateSession(sessionID)
	json.NewEncoder(w).Encode(map[string]string{"session_id": sessionID})
}

func (h *WebHandler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID != "" {
		h.sessionMgr.DeleteSession(sessionID)
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type HistoryResponse struct {
	SessionID string      `json:"session_id"`
	Messages  interface{} `json:"messages"`
}

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

func (h *WebHandler) handleListTools(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": []string{"read", "write", "bash", "terminal", "fetch", "search"},
	})
}

func (h *WebHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Web] WebSocket upgrade failed: %v", err)
		return
	}

	client := &WebClient{
		conn:      conn,
		send:      make(chan []byte, 256),
		sessionID: fmt.Sprintf("web_%d", time.Now().UnixMilli()),
	}

	h.clientsMu.Lock()
	h.clients[client.sessionID] = client
	h.clientsMu.Unlock()

	defer func() {
		h.clientsMu.Lock()
		delete(h.clients, client.sessionID)
		h.clientsMu.Unlock()
		close(client.send)
	}()

	go h.writePump(client)
	h.readPump(client)
}

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
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

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

		go func() {
			response, err := sess.SendMessage(ctx, nil, msg.Content)
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

func (h *WebHandler) sendToClient(client *WebClient, data interface{}) {
	msg, _ := json.Marshal(data)
	select {
	case client.send <- msg:
	default:
		log.Printf("[Web] client send queue full")
	}
}

var _ Channel = (*WebHandler)(nil)

func (h *WebHandler) Init(cfg interface{}) error {
	if c, ok := cfg.(WebConfig); ok {
		h.config = &c
		return nil
	}
	return fmt.Errorf("invalid web config")
}

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

func (h *WebHandler) Receive(ctx context.Context) (<-chan *Message, error) {
	ch := make(chan *Message, 10)
	return ch, nil
}

func (h *WebHandler) Type() string {
	return "web"
}

func WebChannel(host string, port int, sessMgr *session.Manager) *WebHandler {
	return NewWebHandler(&WebConfig{Host: host, Port: port}, sessMgr)
}
