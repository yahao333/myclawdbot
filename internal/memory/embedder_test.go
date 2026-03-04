package memory

import (
	"context"
	"math"
	"testing"
)

// SimpleEmbedder tests

func TestNewSimpleEmbedder_DefaultDimensions(t *testing.T) {
	embedder := NewSimpleEmbedder(0)
	if embedder.dimensions != 512 {
		t.Errorf("expected default dimensions 512, got %d", embedder.dimensions)
	}
}

func TestNewSimpleEmbedder_NegativeDimensions(t *testing.T) {
	embedder := NewSimpleEmbedder(-1)
	if embedder.dimensions != 512 {
		t.Errorf("expected dimensions 512 for negative input, got %d", embedder.dimensions)
	}
}

func TestNewSimpleEmbedder_CustomDimensions(t *testing.T) {
	embedder := NewSimpleEmbedder(256)
	if embedder.dimensions != 256 {
		t.Errorf("expected dimensions 256, got %d", embedder.dimensions)
	}
}

func TestSimpleEmbedder_Embed_EmptyText(t *testing.T) {
	embedder := NewSimpleEmbedder(128)
	ctx := context.Background()

	vector, err := embedder.Embed(ctx, "")
	if err != nil {
		t.Errorf("Embed() error = %v", err)
	}

	if len(vector) != 128 {
		t.Errorf("expected vector length 128, got %d", len(vector))
	}
}

func TestSimpleEmbedder_Embed_Whitespace(t *testing.T) {
	embedder := NewSimpleEmbedder(128)
	ctx := context.Background()

	vector, err := embedder.Embed(ctx, "   \n\t   ")
	if err != nil {
		t.Errorf("Embed() error = %v", err)
	}

	if len(vector) != 128 {
		t.Errorf("expected vector length 128, got %d", len(vector))
	}
}

func TestSimpleEmbedder_Embed_SingleWord(t *testing.T) {
	embedder := NewSimpleEmbedder(128)
	ctx := context.Background()

	vector, err := embedder.Embed(ctx, "hello")
	if err != nil {
		t.Errorf("Embed() error = %v", err)
	}

	if len(vector) != 128 {
		t.Errorf("expected vector length 128, got %d", len(vector))
	}

	// 验证向量已归一化
	var norm float64
	for _, v := range vector {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)
	if norm <= 0 {
		t.Error("expected normalized vector, got zero vector")
	}
}

func TestSimpleEmbedder_Embed_MultipleWords(t *testing.T) {
	embedder := NewSimpleEmbedder(128)
	ctx := context.Background()

	vector, err := embedder.Embed(ctx, "hello world test")
	if err != nil {
		t.Errorf("Embed() error = %v", err)
	}

	if len(vector) != 128 {
		t.Errorf("expected vector length 128, got %d", len(vector))
	}

	// 验证向量已归一化
	var norm float64
	for _, v := range vector {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)
	if norm <= 0 {
		t.Error("expected normalized vector, got zero vector")
	}
}

func TestSimpleEmbedder_Embed_LongText(t *testing.T) {
	embedder := NewSimpleEmbedder(64)
	ctx := context.Background()

	// 创建长文本
	longText := ""
	for i := 0; i < 100; i++ {
		longText += "word" + string(rune(i%10+'0')) + " "
	}

	vector, err := embedder.Embed(ctx, longText)
	if err != nil {
		t.Errorf("Embed() error = %v", err)
	}

	if len(vector) != 64 {
		t.Errorf("expected vector length 64, got %d", len(vector))
	}
}

func TestSimpleEmbedder_Embed_Deterministic(t *testing.T) {
	embedder := NewSimpleEmbedder(128)
	ctx := context.Background()

	text := "hello world"

	vector1, err := embedder.Embed(ctx, text)
	if err != nil {
		t.Errorf("Embed() error = %v", err)
	}

	vector2, err := embedder.Embed(ctx, text)
	if err != nil {
		t.Errorf("Embed() error = %v", err)
	}

	// 相同文本应该产生相同向量
	for i := range vector1 {
		if vector1[i] != vector2[i] {
			t.Error("expected deterministic embedding for same text")
			break
		}
	}
}

func TestSimpleEmbedder_Dimensions(t *testing.T) {
	embedder := NewSimpleEmbedder(256)
	if embedder.Dimensions() != 256 {
		t.Errorf("Dimensions() = %d, want 256", embedder.Dimensions())
	}
}

// Test cosineSimilarity function
func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}

	sim := cosineSimilarity(a, b)
	if sim != 1.0 {
		t.Errorf("cosineSimilarity() = %v, want 1.0 for identical vectors", sim)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}

	sim := cosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("cosineSimilarity() = %v, want 0.0 for orthogonal vectors", sim)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}

	sim := cosineSimilarity(a, b)
	if sim != -1.0 {
		t.Errorf("cosineSimilarity() = %v, want -1.0 for opposite vectors", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0}

	sim := cosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("cosineSimilarity() = %v, want 0.0 for different length vectors", sim)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	a := []float32{}
	b := []float32{}

	sim := cosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("cosineSimilarity() = %v, want 0.0 for empty vectors", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 0, 0}

	sim := cosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("cosineSimilarity() = %v, want 0.0 when one vector is zero", sim)
	}
}

// Test float32 conversion functions
func TestFloat32ToBytesAndBack(t *testing.T) {
	original := []float32{0.0, 1.0, -1.0, 0.5, math.MaxFloat32, math.SmallestNonzeroFloat32}

	bytes := float32ToBytes(original)
	converted := bytesToFloat32(bytes)

	if len(converted) != len(original) {
		t.Errorf("converted length = %d, want %d", len(converted), len(original))
	}

	for i := range original {
		if original[i] != converted[i] {
			t.Errorf("value at index %d = %v, want %v", i, converted[i], original[i])
		}
	}
}

func TestFloat32ToBytes_Empty(t *testing.T) {
	original := []float32{}
	bytes := float32ToBytes(original)
	converted := bytesToFloat32(bytes)

	if len(converted) != 0 {
		t.Errorf("expected empty result, got %d elements", len(converted))
	}
}

// Test OpenAIEmbedder creation
func TestNewOpenAIEmbedder_DefaultModel(t *testing.T) {
	embedder := NewOpenAIEmbedder("test-key", "")
	if embedder.model != "text-embedding-3-small" {
		t.Errorf("expected default model 'text-embedding-3-small', got '%s'", embedder.model)
	}
}

func TestNewOpenAIEmbedder_CustomModel(t *testing.T) {
	embedder := NewOpenAIEmbedder("test-key", "text-embedding-ada-002")
	if embedder.model != "text-embedding-ada-002" {
		t.Errorf("expected model 'text-embedding-ada-002', got '%s'", embedder.model)
	}
}

func TestNewOpenAIEmbedder_DefaultDimensions(t *testing.T) {
	embedder := NewOpenAIEmbedder("test-key", "")
	if embedder.dimensions != 1536 {
		t.Errorf("expected dimensions 1536, got %d", embedder.dimensions)
	}
}

func TestNewOpenAIEmbedder_Dimensions(t *testing.T) {
	embedder := NewOpenAIEmbedder("test-key", "")
	if embedder.Dimensions() != 1536 {
		t.Errorf("Dimensions() = %d, want 1536", embedder.Dimensions())
	}
}

// Test ClaudeEmbedder creation
func TestNewClaudeEmbedder_DefaultValues(t *testing.T) {
	embedder := NewClaudeEmbedder("test-key", "claude-embed")
	if embedder.model != "claude-embed" {
		t.Errorf("expected model 'claude-embed', got '%s'", embedder.model)
	}
	if embedder.dimensions != 1536 {
		t.Errorf("expected dimensions 1536, got %d", embedder.dimensions)
	}
}

func TestNewClaudeEmbedder_Dimensions(t *testing.T) {
	embedder := NewClaudeEmbedder("test-key", "claude-embed")
	if embedder.Dimensions() != 1536 {
		t.Errorf("Dimensions() = %d, want 1536", embedder.Dimensions())
	}
}
