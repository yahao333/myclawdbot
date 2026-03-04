// Package gateway 网关包
// 提供 HTTP/WebSocket 网关服务，支持 REST API、实时通信和安全隔离执行
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/yahao333/myclawdbot/internal/config"
	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/internal/session"
	"github.com/yahao333/myclawdbot/internal/tools"
	"github.com/yahao333/myclawdbot/internal/tools/exec"
	"github.com/yahao333/myclawdbot/internal/tools/file"
	"github.com/yahao333/myclawdbot/internal/tools/web"

	"github.com/gorilla/websocket"
)

// Server 网关服务器
// 负责启动 HTTP 服务器、处理 WebSocket 连接、管理会话和工具
type Server struct {
	httpServer *http.Server
	config     *config.Config
	llmClient  llm.Client
	registry   *tools.Registry
	sessions   *session.Manager
	wsUpgrader websocket.Upgrader
	wsClients  map[string]*wsClient
	mu         sync.RWMutex
}

// wsClient WebSocket 客户端
type wsClient struct {
	id       string
	conn     *websocket.Conn
	sendChan chan []byte
	server   *Server
}

// NewServer 创建新的网关服务器
// cfg: 应用程序配置
// client: LLM 客户端
// 返回配置好的服务器实例
func NewServer(cfg *config.Config, client llm.Client) *Server {
	// 创建工具注册表
	registry := tools.NewRegistry()

	// 根据配置设置工具访问限制
	if cfg.Gateway.EnableSandbox {
		// 沙盒模式：使用受限的工具
		registry = createSandboxRegistry(cfg)
	} else {
		// 正常模式：注册所有工具
		registerTools(registry, cfg)
	}

	return &Server{
		config:    cfg,
		llmClient: client,
		registry:  registry,
		sessions:  session.NewManager(cfg.Session.MaxHistory, client),
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源（生产环境应限制）
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		wsClients: make(map[string]*wsClient),
	}
}

// createSandboxRegistry 创建沙盒模式的工具注册表
// 限制文件访问和命令执行
func createSandboxRegistry(cfg *config.Config) *tools.Registry {
	registry := tools.NewRegistry()

	// 注册受限的文件工具
	readTool := file.NewReadToolWithConfig(&cfg.Tools)
	writeTool := file.NewWriteToolWithConfig(&cfg.Tools)
	registry.Register(readTool)
	registry.Register(writeTool)

	// 限制命令执行工具（如果启用）
	if len(cfg.Tools.AllowedCommands) > 0 {
		cmdTool := exec.NewCommandTool(cfg.Tools.AllowedCommands, cfg.Tools.MaxExecTime)
		registry.Register(cmdTool)
	}

	return registry
}

// registerTools 注册所有可用工具
func registerTools(registry *tools.Registry, cfg *config.Config) {
	// 文件工具
	readTool := file.NewReadToolWithConfig(&cfg.Tools)
	writeTool := file.NewWriteToolWithConfig(&cfg.Tools)
	registry.Register(readTool)
	registry.Register(writeTool)

	// 命令执行工具
	cmdTool := exec.NewCommandTool(cfg.Tools.AllowedCommands, cfg.Tools.MaxExecTime)
	registry.Register(cmdTool)

	// Web 工具
	fetchTool := web.NewFetchTool(10 * 1024 * 1024) // 10MB 默认
	registry.Register(fetchTool)
}

// Start 启动网关服务器
// 阻塞直到服务器关闭
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Gateway.Host, s.config.Gateway.Port)

	// 创建 HTTP 处理 mux
	mux := http.NewServeMux()

	// 注册路由
	s.registerRoutes(mux)

	// 创建 HTTP 服务器
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.loggingMiddleware(s.authMiddleware(mux)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器
	go func() {
		log.Printf("Gateway server starting on %s", addr)
		if s.config.Gateway.EnableAuth {
			log.Printf("Authentication enabled")
		}
		if s.config.Gateway.EnableSandbox {
			log.Printf("Sandbox mode enabled with dirs: %v", s.config.Gateway.SandboxDirs)
		}
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Shutting down server...")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server exited")
	return nil
}

// Stop 停止网关服务器
func (s *Server) Stop() error {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// registerRoutes 注册 HTTP 路由
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// 健康检查
	mux.HandleFunc("/health", s.handleHealth)

	// API 路由
	api := http.HandlerFunc(s.handleAPI)
	mux.Handle("/api/", api)

	// WebSocket 路由
	mux.HandleFunc("/ws", s.handleWebSocket)
}

// handleHealth 健康检查处理函数
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleAPI API 请求处理函数
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	// 从 URL 提取 API 路径
	path := r.URL.Path[len("/api"):]

	switch path {
	case "/chat":
		s.handleChat(w, r)
	case "/sessions":
		s.handleSessions(w, r)
	case "/tools":
		s.handleTools(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleChat 处理聊天请求
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
		Stream    bool   `json:"stream"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 获取或创建会话
	sess := s.getOrCreateSession(req.SessionID)

	// 发送消息并获取响应
	response, err := sess.SendMessage(r.Context(), s.llmClient, req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"session_id": sess.ID,
		"response":   response,
	})
}

// handleSessions 处理会话管理请求
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出所有会话 - 需要通过 Manager 获取，这里简化为返回空列表
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]string{"sessions": []string{}})

	case http.MethodPost:
		// 创建新会话
		sess := s.createSession()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"session_id": sess.ID})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTools 处理工具列表请求
func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	toolsList := s.registry.List()
	toolDefs := make([]map[string]interface{}, 0, len(toolsList))
	for _, t := range toolsList {
		toolDefs = append(toolDefs, map[string]interface{}{
			"name":        t.Name(),
			"description": t.Description(),
			"parameters":  t.Parameters(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tools": toolDefs})
}

// handleWebSocket 处理 WebSocket 连接
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级连接
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// 创建客户端
	client := &wsClient{
		id:       generateID(),
		conn:     conn,
		sendChan: make(chan []byte, 256),
		server:   s,
	}

	// 注册客户端
	s.mu.Lock()
	s.wsClients[client.id] = client
	s.mu.Unlock()

	// 启动读写协程
	go client.writePump()
	client.readPump()

	// 移除客户端
	s.mu.Lock()
	delete(s.wsClients, client.id)
	s.mu.Unlock()
}

// getOrCreateSession 获取或创建会话
func (s *Server) getOrCreateSession(id string) *session.Session {
	// 先尝试获取
	sess, ok := s.sessions.GetSession(id)
	if ok {
		return sess
	}
	// 不存在则创建
	return s.createSession()
}

// createSession 创建新会话
func (s *Server) createSession() *session.Session {
	return s.sessions.CreateSession("")
}

// readPump WebSocket 读取泵
func (c *wsClient) readPump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// 处理消息
		c.handleMessage(message)
	}
}

// writePump WebSocket 写入泵
func (c *wsClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.sendChan:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理 WebSocket 消息
func (c *wsClient) handleMessage(data []byte) {
	var msg struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("Invalid message format")
		return
	}

	switch msg.Type {
	case "chat":
		c.handleChat(msg.Message)
	case "ping":
		c.send(map[string]string{"type": "pong"})
	default:
		c.sendError("Unknown message type")
	}
}

// handleChat 处理聊天消息
func (c *wsClient) handleChat(message string) {
	// 创建临时会话
	sess := c.server.createSession()

	// 发送消息
	resp, err := sess.SendMessage(context.Background(), c.server.llmClient, message)
	if err != nil {
		c.sendError(err.Error())
		return
	}

	// 发送响应
	c.send(map[string]string{
		"type":       "response",
		"session_id": sess.ID,
		"content":    resp,
	})
}

// send 发送 JSON 消息
func (c *wsClient) send(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.sendChan <- data:
	default:
		log.Printf("Client %s send channel full", c.id)
	}
}

// sendError 发送错误消息
func (c *wsClient) sendError(err string) {
	c.send(map[string]string{
		"type": "error",
		"error": err,
	})
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}
