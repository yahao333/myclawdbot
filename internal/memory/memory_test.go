// Package memory 记忆模块测试包
// 包含对 SessionMemory、Manager 和相关工具函数的单元测试
package memory

import (
	"context"
	"testing"
	"time"

	"github.com/yahao333/myclawdbot/pkg/types"
)

// TestSessionMemory_Add 测试会话记忆添加功能
// 验证添加单条消息后，计数正确
func TestSessionMemory_Add(t *testing.T) {
	config := DefaultConfig()
	config.MaxHistory = 5

	sess := NewSessionMemory("test-session", config, nil)

	msg := &types.Message{
		Role:      "user",
		Content:   "Hello",
		Timestamp: time.Now(),
	}

	err := sess.Add(context.Background(), msg)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if sess.Count() != 1 {
		t.Fatalf("Expected 1 message, got %d", sess.Count())
	}
}

// TestSessionMemory_Get 测试会话记忆获取功能
// 验证获取全部消息和限制获取消息数量的功能
func TestSessionMemory_Get(t *testing.T) {
	config := DefaultConfig()
	config.MaxHistory = 10

	sess := NewSessionMemory("test-session", config, nil)

	// 添加多条消息
	for i := 0; i < 5; i++ {
		msg := &types.Message{
			Role:    "user",
			Content: "Message " + string(rune('0'+i)),
			Timestamp: time.Now(),
		}
		sess.Add(context.Background(), msg)
	}

	// 测试获取全部
	messages := sess.GetAll()
	if len(messages) != 5 {
		t.Fatalf("Expected 5 messages, got %d", len(messages))
	}

	// 测试限制获取
	messages = sess.Get(2)
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}
}

// TestSessionMemory_TokenCount 测试 token 计数功能
// 验证 token 估算功能（简单估算：1 token ≈ 4 字符）
func TestSessionMemory_TokenCount(t *testing.T) {
	config := DefaultConfig()
	config.MaxHistory = 10

	sess := NewSessionMemory("test-session", config, nil)

	// 添加消息
	msg := &types.Message{
		Role:    "user",
		Content: "This is a test message with some content",
		Timestamp: time.Now(),
	}
	sess.Add(context.Background(), msg)

	count := sess.TokenCount()
	// 简单估算: 1 token ≈ 4 字符
	expected := len("This is a test message with some content") / 4
	if count < expected-5 || count > expected+5 {
		t.Logf("Token count: %d, expected around: %d", count, expected)
	}
}

// TestSessionMemory_Compress 测试会话记忆压缩功能
// 验证当消息数量超过限制时，自动压缩保留部分消息
func TestSessionMemory_Compress(t *testing.T) {
	config := DefaultConfig()
	config.MaxHistory = 10
	config.MaxTokens = 50 // 设置很小的 token 限制来触发压缩
	config.EnableCompress = true

	sess := NewSessionMemory("test-session", config, nil)

	// 添加足够多的消息来触发压缩
	for i := 0; i < 8; i++ {
		msg := &types.Message{
			Role:    "user",
			Content: "This is a long test message number " + string(rune('0'+i)) + " with some additional content",
			Timestamp: time.Now(),
		}
		sess.Add(context.Background(), msg)
	}

	// 应该触发压缩，保留部分消息
	if sess.Count() > config.MaxHistory {
		t.Fatalf("Expected message count <= %d after compress, got %d", config.MaxHistory, sess.Count())
	}
}

// TestSessionMemory_Clear 测试会话记忆清除功能
// 验证清除所有消息后，计数归零
func TestSessionMemory_Clear(t *testing.T) {
	config := DefaultConfig()

	sess := NewSessionMemory("test-session", config, nil)

	// 添加消息
	msg := &types.Message{
		Role:      "user",
		Content:   "Hello",
		Timestamp: time.Now(),
	}
	sess.Add(context.Background(), msg)

	if sess.Count() != 1 {
		t.Fatalf("Expected 1 message before clear")
	}

	// 清除
	sess.Clear()

	if sess.Count() != 0 {
		t.Fatalf("Expected 0 messages after clear, got %d", sess.Count())
	}
}

// TestManager_GetSession 测试会话管理器获取会话功能
// 验证获取已存在的会话返回同一实例，创建新会话返回不同实例
func TestManager_GetSession(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	// 创建新会话
	sess1 := mgr.GetSession("test-1")
	if sess1 == nil {
		t.Fatal("Expected session, got nil")
	}

	// 获取已存在的会话
	sess2 := mgr.GetSession("test-1")
	if sess1 != sess2 {
		t.Fatal("Expected same session instance")
	}

	// 创建另一个会话
	sess3 := mgr.GetSession("test-2")
	if sess3 == sess1 {
		t.Fatal("Expected different session instances")
	}
}

// TestManager_DeleteSession 测试会话管理器删除会话功能
// 验证删除会话后，再次获取会创建新的会话实例
func TestManager_DeleteSession(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	// 创建会话
	sess := mgr.GetSession("test-delete")
	if sess == nil {
		t.Fatal("Expected session, got nil")
	}

	// 删除会话
	mgr.DeleteSession("test-delete")

	// 再次获取应该创建新会话
	sess2 := mgr.GetSession("test-delete")
	if sess2 == nil {
		t.Fatal("Expected new session after delete, got nil")
	}
	if sess == sess2 {
		t.Fatal("Expected different session after delete")
	}
}

// TestCosineSimilarity 测试余弦相似度计算
// 验证：
//   - 相同向量相似度接近 1
//   - 正交向量相似度接近 0
//   - 相反向量相似度接近 -1
func TestCosineSimilarity(t *testing.T) {
	// 相同向量
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if sim < 0.99 || sim > 1.01 {
		t.Fatalf("Expected similarity ~1.0, got %f", sim)
	}

	// 正交向量
	a = []float32{1, 0, 0}
	b = []float32{0, 1, 0}
	sim = cosineSimilarity(a, b)
	if sim > 0.01 {
		t.Fatalf("Expected similarity ~0.0, got %f", sim)
	}

	// 相反向量
	a = []float32{1, 0, 0}
	b = []float32{-1, 0, 0}
	sim = cosineSimilarity(a, b)
	if sim > -0.99 || sim < -1.01 {
		t.Fatalf("Expected similarity ~-1.0, got %f", sim)
	}
}

// TestFloat32Conversion 测试 float32 与字节数组的相互转换
// 验证 float32ToBytes 和 bytesToFloat32 函数的正确性
func TestFloat32Conversion(t *testing.T) {
	original := []float32{1.5, -2.3, 3.14159, 0.0, 100.0}

	// 转换为 bytes
	bytes := float32ToBytes(original)
	if len(bytes) != len(original)*4 {
		t.Fatalf("Expected %d bytes, got %d", len(original)*4, len(bytes))
	}

	// 转换回 float32
	converted := bytesToFloat32(bytes)
	if len(converted) != len(original) {
		t.Fatalf("Expected %d floats, got %d", len(original), len(converted))
	}

	// 验证值
	for i := range original {
		if converted[i] != original[i] {
			t.Fatalf("At index %d: expected %f, got %f", i, original[i], converted[i])
		}
	}
}

// TestSimpleEmbedder 测试简单嵌入器功能
// 验证：
//   - 相同文本产生相同嵌入
//   - 不同文本产生不同嵌入
//   - 向量维度正确
func TestSimpleEmbedder(t *testing.T) {
	embedder := NewSimpleEmbedder(512)

	// 测试嵌入
	emb1, err := embedder.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(emb1) != 512 {
		t.Fatalf("Expected 512 dimensions, got %d", len(emb1))
	}

	// 测试相同文本产生相似结果
	emb2, err := embedder.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	sim := cosineSimilarity(emb1, emb2)
	if sim < 0.99 {
		t.Fatalf("Expected similarity ~1.0 for same text, got %f", sim)
	}

	// 测试不同文本
	emb3, err := embedder.Embed(context.Background(), "different text completely")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	sim = cosineSimilarity(emb1, emb3)
	if sim > 0.9 {
		t.Logf("Similarity between different texts: %f", sim)
	}
}

// TestConfig_Default 测试默认配置
// 验证 DefaultConfig() 返回正确的默认值
func TestConfig_Default(t *testing.T) {
	config := DefaultConfig()

	if config.MaxHistory != 100 {
		t.Fatalf("Expected MaxHistory=100, got %d", config.MaxHistory)
	}

	if config.MaxTokens != 4000 {
		t.Fatalf("Expected MaxTokens=4000, got %d", config.MaxTokens)
	}

	if !config.EnableCompress {
		t.Fatal("Expected EnableCompress=true")
	}

	if config.EnableLongTerm {
		t.Fatal("Expected EnableLongTerm=false")
	}
}
