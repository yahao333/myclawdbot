// Package memory 记忆管理包
// 提供短期会话记忆和长期向量记忆的存储功能
package memory

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage SQLite 存储结构体
// 基于 SQLite 实现长期记忆存储，支持向量搜索和文本搜索
type SQLiteStorage struct {
	db       *sql.DB           // SQLite 数据库连接
	dir      string            // 存储目录路径
	embedding Embedder         // 向量嵌入器（可选，用于语义搜索）
}

// NewSQLiteStorage 创建 SQLite 存储实例
// dir: 存储目录路径，如果为空使用默认路径 ~/.myclawdbot/memory
// 返回初始化好的 SQLiteStorage 实例
func NewSQLiteStorage(dir string) *SQLiteStorage {
	// 使用默认目录
	if dir == "" {
		dir = "~/.myclawdbot/memory"
	}
	// 展开 ~ 为实际用户目录
	dir = expandHome(dir)

	// 创建存储目录
	if err := createDir(dir); err != nil {
		fmt.Printf("Warning: failed to create memory directory: %v\n", err)
	}

	// 构建数据库文件路径
	dbPath := filepath.Join(dir, "memory.db")
	// 打开 SQLite 数据库连接
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Printf("Warning: failed to open database: %v\n", err)
		return &SQLiteStorage{dir: dir}
	}

	// 创建数据库表结构
	if err := createTables(db); err != nil {
		fmt.Printf("Warning: failed to create tables: %v\n", err)
	}

	return &SQLiteStorage{
		db:  db,
		dir: dir,
	}
}

// createTables 创建数据库表结构
// db: SQLite 数据库连接
// 创建 memories 表用于存储记忆数据，包含以下字段：
//   - id: 记忆唯一标识
//   - session_id: 所属会话标识
//   - content: 记忆内容文本
//   - role: 角色（user/assistant/system）
//   - created_at: 创建时间戳
//   - embedding: 向量嵌入（可选，用于语义搜索）
// 同时创建会话ID和创建时间的索引以提高查询性能
func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			content TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			embedding BLOB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_session ON memories(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at)`,
	}

	// 执行每条 SQL 语句
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// Save 保存记忆
// ctx: 上下文对象
// item: 要保存的记忆条目
// 返回保存过程中的错误（如果有）
// 功能说明：
//   - 如果记忆没有 ID，自动生成唯一标识
//   - 如果没有时间戳，使用当前时间
//   - 如果配置了嵌入器，自动生成向量嵌入
//   - 使用 INSERT OR REPLACE 策略，支持更新已存在的记忆
func (s *SQLiteStorage) Save(ctx context.Context, item *MemoryItem) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if item.ID == "" {
		item.ID = generateID()
	}
	if item.Timestamp.IsZero() {
		item.Timestamp = time.Now()
	}

	// 生成 embedding
	if s.embedding != nil && item.Embedding == nil {
		emb, err := s.embedding.Embed(ctx, item.Content)
		if err == nil {
			item.Embedding = emb
		}
	}

	// 转换为 blob
	var embBlob []byte
	if item.Embedding != nil {
		embBlob = float32ToBytes(item.Embedding)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO memories (id, session_id, content, role, created_at, embedding)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		item.ID, item.SessionID, item.Content, item.Role, item.Timestamp.Unix(), embBlob)

	return err
}

// Search 搜索记忆
// ctx: 上下文对象
// query: 搜索查询字符串
// limit: 返回结果数量限制
// 返回匹配的記憶條目列表和錯誤信息
// 搜索策略：
//   - 如果配置了嵌入器，使用向量语义搜索（计算余弦相似度）
//   - 否则使用文本 LIKE 搜索
func (s *SQLiteStorage) Search(ctx context.Context, query string, limit int) ([]MemoryItem, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if limit <= 0 {
		limit = 10
	}

	// 如果有 embedding 模型，使用向量搜索
	if s.embedding != nil {
		emb, err := s.embedding.Embed(ctx, query)
		if err == nil {
			return s.vectorSearch(ctx, emb, limit)
		}
	}

	// 否则使用文本搜索
	return s.textSearch(ctx, query, limit)
}

// vectorSearch 向量语义搜索
// ctx: 上下文对象
// query: 查询的向量嵌入
// limit: 返回结果数量限制
// 返回按创建时间排序的记忆条目，结果中包含与查询向量的余弦相似度
// 注意：当前实现返回所有记忆并逐个计算相似度，大规模数据时需要优化
func (s *SQLiteStorage) vectorSearch(ctx context.Context, query []float32, limit int) ([]MemoryItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, content, role, created_at, embedding
		 FROM memories
		 ORDER BY created_at DESC
		 LIMIT ?`, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MemoryItem
	for rows.Next() {
		var item MemoryItem
		var embBlob []byte
		var createdAt int64

		err := rows.Scan(&item.ID, &item.SessionID, &item.Content, &item.Role, &createdAt, &embBlob)
		if err != nil {
			continue
		}

		item.Timestamp = time.Unix(createdAt, 0)

		// 计算相似度
		if embBlob != nil {
			item.Embedding = bytesToFloat32(embBlob)
			item.Metadata = map[string]interface{}{
				"similarity": cosineSimilarity(query, item.Embedding),
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// textSearch 文本模糊搜索
// ctx: 上下文对象
// query: 搜索关键词
// limit: 返回结果数量限制
// 使用 SQL LIKE 进行模糊匹配，返回包含关键词的记忆条目
func (s *SQLiteStorage) textSearch(ctx context.Context, query string, limit int) ([]MemoryItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, content, role, created_at
		 FROM memories
		 WHERE content LIKE ?
		 ORDER BY created_at DESC
		 LIMIT ?`, "%"+query+"%", limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MemoryItem
	for rows.Next() {
		var item MemoryItem
		var createdAt int64

		err := rows.Scan(&item.ID, &item.SessionID, &item.Content, &item.Role, &createdAt)
		if err != nil {
			continue
		}

		item.Timestamp = time.Unix(createdAt, 0)
		items = append(items, item)
	}

	return items, nil
}

// Delete 删除指定记忆
// ctx: 上下文对象
// id: 要删除的记忆 ID
// 返回删除操作的结果错误（如果有）
func (s *SQLiteStorage) Delete(ctx context.Context, id string) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM memories WHERE id = ?", id)
	return err
}

// Close 关闭数据库连接
// 返回关闭连接过程中的错误（如果有）
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Embedder 向量嵌入器接口
// 定义将文本转换为向量嵌入的抽象接口
// 实现此接口的结构体可以用于语义搜索
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error) // 将文本转换为向量嵌入
}

// cosineSimilarity 计算两个向量的余弦相似度
// a, b: 要比较的向量
// 返回值范围：-1 到 1
//   - 1 表示完全相同方向
//   - 0 表示正交（无相关性）
//   - -1 表示完全相反方向
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (normA * normB)
}

// float32ToBytes 将 float32 切片转换为字节切片
// 用于将向量嵌入存储到 SQLite 数据库的 BLOB 字段
// 每个 float32 值转换为 4 个字节（IEEE 754 标准）
func float32ToBytes(f []float32) []byte {
	b := make([]byte, len(f)*4)
	for i, v := range f {
		bits := float32ToBits(v)
		b[i*4] = byte(bits)
		b[i*4+1] = byte(bits >> 8)
		b[i*4+2] = byte(bits >> 16)
		b[i*4+3] = byte(bits >> 24)
	}
	return b
}

// bytesToFloat32 将字节切片转换回 float32 切片
// 从数据库 BLOB 字段读取向量嵌入时使用
// 与 float32ToBytes 互为逆操作
func bytesToFloat32(b []byte) []float32 {
	f := make([]float32, len(b)/4)
	for i := 0; i < len(f); i++ {
		bits := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
		f[i] = bitsToFloat32(bits)
	}
	return f
}

// float32ToBits 将 float32 转换为对应的位表示
// 使用 math.Float32bits 获取 IEEE 754 位模式
func float32ToBits(f float32) uint32 {
	return math.Float32bits(f)
}

// bitsToFloat32 将位表示转换回 float32
// 使用 math.Float32frombits 从 IEEE 754 位模式恢复浮点数
func bitsToFloat32(bits uint32) float32 {
	return math.Float32frombits(bits)
}

// generateID 生成唯一标识符
// 格式：时间戳_随机字符串
// 用于为记忆条目生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成指定长度的随机字符串
// n: 字符串长度
// 返回由字母和数字组成的随机字符串
// 注意：此实现不够安全，仅用于生成 ID
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// expandHome 展开路径中的 ~ 为用户主目录
// path: 包含 ~ 的路径
// 返回展开后的绝对路径
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := getHomeDir()
		if home != "" {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// getHomeDir 获取用户主目录
// 返回用户主目录路径和错误信息
func getHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}

// createDir 创建目录（递归创建）
// dir: 要创建的目录路径
// 使用 0755 权限创建目录（所有者读写执行，组和其他读执行）
func createDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// 编译时接口检查：确保 SQLiteStorage 实现了 LongTermMemory 接口
var _ LongTermMemory = (*SQLiteStorage)(nil)
