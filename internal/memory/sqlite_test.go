package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSQLiteStorage_DefaultDir(t *testing.T) {
	// 创建一个临时目录用于测试
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	if storage.dir != tmpDir {
		t.Errorf("expected dir %s, got %s", tmpDir, storage.dir)
	}
}

func TestNewSQLiteStorage_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	// 验证目录已创建
	if _, err := os.Stat(storage.dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

func TestSQLiteStorage_Save(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	ctx := context.Background()
	item := &MemoryItem{
		SessionID: "test-session",
		Content:   "test content",
		Role:      "user",
	}

	err := storage.Save(ctx, item)
	if err != nil {
		t.Errorf("Save() error = %v", err)
	}

	// 验证 ID 已生成
	if item.ID == "" {
		t.Error("expected ID to be generated")
	}
}

func TestSQLiteStorage_Save_WithExistingID(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	ctx := context.Background()
	item := &MemoryItem{
		ID:        "custom-id",
		SessionID: "test-session",
		Content:   "test content",
		Role:      "user",
	}

	err := storage.Save(ctx, item)
	if err != nil {
		t.Errorf("Save() error = %v", err)
	}

	// 验证 ID 保持不变
	if item.ID != "custom-id" {
		t.Errorf("expected ID 'custom-id', got %s", item.ID)
	}
}

func TestSQLiteStorage_Save_WithExistingTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	ctx := context.Background()
	customTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	item := &MemoryItem{
		SessionID:  "test-session",
		Content:    "test content",
		Role:       "user",
		Timestamp:  customTime,
	}

	err := storage.Save(ctx, item)
	if err != nil {
		t.Errorf("Save() error = %v", err)
	}

	// 验证时间戳保持不变
	if !item.Timestamp.Equal(customTime) {
		t.Errorf("expected timestamp %v, got %v", customTime, item.Timestamp)
	}
}

func TestSQLiteStorage_Save_WithEmbedding(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	// 设置 embedding
	storage.embedding = &mockEmbedder{}

	ctx := context.Background()
	item := &MemoryItem{
		SessionID: "test-session",
		Content:   "test content",
		Role:      "user",
	}

	err := storage.Save(ctx, item)
	if err != nil {
		t.Errorf("Save() error = %v", err)
	}

	// 验证 embedding 已生成
	if item.Embedding == nil {
		t.Error("expected embedding to be generated")
	}
}

func TestSQLiteStorage_Search_Text(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	ctx := context.Background()

	// 保存一些测试数据
	items := []*MemoryItem{
		{SessionID: "session1", Content: "hello world", Role: "user"},
		{SessionID: "session1", Content: "foo bar", Role: "assistant"},
		{SessionID: "session2", Content: "hello there", Role: "user"},
	}

	for _, item := range items {
		if err := storage.Save(ctx, item); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// 搜索
	results, err := storage.Search(ctx, "hello", 10)
	if err != nil {
		t.Errorf("Search() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("expected search results, got empty")
	}
}

func TestSQLiteStorage_Search_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	ctx := context.Background()

	// 保存测试数据
	for i := 0; i < 5; i++ {
		item := &MemoryItem{
			SessionID: "session1",
			Content:   "test content " + string(rune('0'+i)),
			Role:      "user",
		}
		if err := storage.Save(ctx, item); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// 搜索并限制结果数量
	results, err := storage.Search(ctx, "test", 2)
	if err != nil {
		t.Errorf("Search() error = %v", err)
	}

	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}

func TestSQLiteStorage_Search_DefaultLimit(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	ctx := context.Background()

	// 保存多个测试数据
	for i := 0; i < 15; i++ {
		item := &MemoryItem{
			SessionID: "session1",
			Content:   "test content " + string(rune('0'+i)),
			Role:      "user",
		}
		if err := storage.Save(ctx, item); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// 搜索不指定 limit（应使用默认值 10）
	results, err := storage.Search(ctx, "test", 0)
	if err != nil {
		t.Errorf("Search() error = %v", err)
	}

	// 默认 limit 应该是 10
	if len(results) > 10 {
		t.Errorf("expected at most 10 results by default, got %d", len(results))
	}
}

func TestSQLiteStorage_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	ctx := context.Background()

	// 保存数据
	item := &MemoryItem{
		SessionID: "test-session",
		Content:   "test content",
		Role:      "user",
	}
	if err := storage.Save(ctx, item); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 删除
	err := storage.Delete(ctx, item.ID)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// 验证已删除
	results, err := storage.Search(ctx, "test", 10)
	if err != nil {
		t.Errorf("Search() error = %v", err)
	}

	for _, r := range results {
		if r.ID == item.ID {
			t.Error("expected item to be deleted")
			break
		}
	}
}

func TestSQLiteStorage_Close(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)

	err := storage.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestSQLiteStorage_Close_AlreadyClosed(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)

	// 第一次关闭
	storage.Close()
	// 第二次关闭应该不会出错
	err := storage.Close()
	if err != nil {
		t.Errorf("Close() error on second call = %v", err)
	}
}

// mockEmbedder 用于测试的 mock 嵌入器
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// 返回简单的固定向量
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

// Test SQLiteStorage with nil database (error handling)
func TestSQLiteStorage_Save_NoDB(t *testing.T) {
	storage := &SQLiteStorage{db: nil}
	ctx := context.Background()

	err := storage.Save(ctx, &MemoryItem{Content: "test"})
	if err == nil {
		t.Error("expected error when db is nil")
	}
}

func TestSQLiteStorage_Search_NoDB(t *testing.T) {
	storage := &SQLiteStorage{db: nil}
	ctx := context.Background()

	_, err := storage.Search(ctx, "test", 10)
	if err == nil {
		t.Error("expected error when db is nil")
	}
}

func TestSQLiteStorage_Delete_NoDB(t *testing.T) {
	storage := &SQLiteStorage{db: nil}
	ctx := context.Background()

	err := storage.Delete(ctx, "test-id")
	if err == nil {
		t.Error("expected error when db is nil")
	}
}

// Test textSearch function
func TestSQLiteStorage_textSearch(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	ctx := context.Background()

	// 保存包含特定关键词的数据
	items := []*MemoryItem{
		{SessionID: "s1", Content: "golang is great", Role: "user"},
		{SessionID: "s1", Content: "python is ok", Role: "user"},
	}

	for _, item := range items {
		if err := storage.Save(ctx, item); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// 测试文本搜索
	results, err := storage.textSearch(ctx, "golang", 10)
	if err != nil {
		t.Errorf("textSearch() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0].Content != "golang is great" {
		t.Errorf("expected 'golang is great', got '%s'", results[0].Content)
	}
}

// Test expandHome function in memory package
func TestExpandHome_InMemory(t *testing.T) {
	tests := []struct {
		input string
	}{
		{""},
		{"/absolute/path"},
		{"~/test"},
	}

	for _, tt := range tests {
		result := expandHome(tt.input)
		// 只需确保不 panic
		_ = result
	}
}

// Test getHomeDir function
func TestGetHomeDir(t *testing.T) {
	home, err := getHomeDir()
	if err != nil {
		t.Errorf("getHomeDir() error = %v", err)
	}
	if home == "" {
		t.Error("expected non-empty home directory")
	}
}

// Test createDir function
func TestCreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "subdir", "nested")

	err := createDir(testDir)
	if err != nil {
		t.Errorf("createDir() error = %v", err)
	}

	// 验证目录已创建
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

// Test NewSQLiteStorage with invalid directory (should not panic)
func TestNewSQLiteStorage_InvalidDir(t *testing.T) {
	// 使用一个不存在的路径
	tmpDir := "/nonexistent/path/that/cannot/be/created"
	storage := NewSQLiteStorage(tmpDir)
	defer storage.Close()

	// 验证存储对象已创建
	if storage == nil {
		t.Error("expected storage to be created")
	}
}
