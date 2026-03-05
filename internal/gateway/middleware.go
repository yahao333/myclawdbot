// Package gateway 网关包
//
// 提供 HTTP/WebSocket 网关服务，支持 REST API、实时通信和安全隔离执行。
package gateway

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"
)

// AuthMiddleware 认证中间件
//
// 验证请求是否包含有效的 API Key。
// 如果未启用认证，则直接跳过验证。
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 如果未启用认证，直接跳过
		if !s.config.Gateway.EnableAuth {
			next.ServeHTTP(w, r)
			return
		}

		// 获取 API Key
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// 也支持 URL 参数
			apiKey = r.URL.Query().Get("api_key")
		}

		if apiKey == "" {
			http.Error(w, "Missing API Key", http.StatusUnauthorized)
			return
		}

		// 验证 API Key
		if !s.isValidAPIKey(apiKey) {
			http.Error(w, "Invalid API Key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isValidAPIKey 验证 API Key 是否有效
//
// 使用 SHA256 哈希比较来安全地验证 API Key。
func (s *Server) isValidAPIKey(apiKey string) bool {
	hash := sha256.Sum256([]byte(apiKey))
	hashedKey := hex.EncodeToString(hash[:])

	for _, key := range s.config.Gateway.APIKeys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(hashedKey)) == 1 {
			return true
		}
	}
	return false
}

// LoggingMiddleware 日志中间件
//
// 记录每个请求的处理时间和状态。
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装 ResponseWriter 以记录状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Printf(
			"%s %s %d %v",
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			duration,
		)
	})
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码
//
// 用于在日志中间件中记录 HTTP 响应状态码。
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader 覆盖 WriteHeader 以捕获状态码
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CORSMiddleware CORS 中间件
//
// 允许跨域请求（开发环境使用）。
// 生产环境应限制允许的来源。
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 允许的来源（生产环境应限制）
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		}

		// 处理预检请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimitMiddleware 速率限制中间件
//
// 简单的基于 IP 的速率限制。
// 注意：生产环境应使用 Redis 等分布式存储。
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	// 简单的内存存储（生产环境应使用 Redis 等）
	type clientInfo struct {
		requests int
		resetAt  time.Time
	}
	clients := make(map[string]*clientInfo)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取客户端 IP
		ip := getClientIP(r)

		now := time.Now()
		info, exists := clients[ip]

		if !exists || now.After(info.resetAt) {
			// 新客户端或重置
			clients[ip] = &clientInfo{
				requests: 1,
				resetAt:  now.Add(time.Minute),
			}
			next.ServeHTTP(w, r)
			return
		}

		// 检查速率限制
		if info.requests >= 60 { // 每分钟 60 次
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		info.requests++
		next.ServeHTTP(w, r)
	})
}

// getClientIP 获取客户端真实 IP
//
// 优先从 X-Forwarded-For 和 X-Real-IP 头获取真实 IP。
func getClientIP(r *http.Request) string {
	// 优先检查 X-Forwarded-For
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// 检查 X-Real-IP
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// 直接使用 RemoteAddr
	return r.RemoteAddr
}
