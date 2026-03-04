package memory

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage SQLite 存储
type SQLiteStorage struct {
	db       *sql.DB
	dir      string
	embedding Embedder
}

// NewSQLiteStorage 创建 SQLite 存储
func NewSQLiteStorage(dir string) *SQLiteStorage {
	if dir == "" {
		dir = "~/.myclawdbot/memory"
	}
	dir = expandHome(dir)

	// 创建目录
	if err := createDir(dir); err != nil {
		fmt.Printf("Warning: failed to create memory directory: %v\n", err)
	}

	dbPath := filepath.Join(dir, "memory.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Printf("Warning: failed to open database: %v\n", err)
		return &SQLiteStorage{dir: dir}
	}

	// 创建表
	if err := createTables(db); err != nil {
		fmt.Printf("Warning: failed to create tables: %v\n", err)
	}

	return &SQLiteStorage{
		db:  db,
		dir: dir,
	}
}

// createTables 创建表
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

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// Save 保存记忆
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

// vectorSearch 向量搜索
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

// textSearch 文本搜索
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

// Delete 删除记忆
func (s *SQLiteStorage) Delete(ctx context.Context, id string) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM memories WHERE id = ?", id)
	return err
}

// Close 关闭
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Embedder 向量嵌入接口
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// cosineSimilarity 计算余弦相似度
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

// float32ToBytes 转换
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

// bytesToFloat32 转换
func bytesToFloat32(b []byte) []float32 {
	f := make([]float32, len(b)/4)
	for i := 0; i < len(f); i++ {
		bits := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
		f[i] = bitsToFloat32(bits)
	}
	return f
}

// float32ToBits 转换
func float32ToBits(f float32) uint32 {
	return math.Float32bits(f)
}

// bitsToFloat32 转换
func bitsToFloat32(bits uint32) float32 {
	return math.Float32frombits(bits)
}

func generateID() string {
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), randomString(8))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := getHomeDir()
		if home != "" {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

func getHomeDir() (string, error) {
	return filepath.Join("/Users/yanghao"), nil // 简化实现
}

func createDir(dir string) error {
	// 简化实现
	return nil
}

var _ LongTermMemory = (*SQLiteStorage)(nil)
