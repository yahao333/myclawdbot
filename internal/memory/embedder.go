// Package memory 记忆管理包（已在 sqlite.go 中定义）
// 本文件提供向量嵌入器的实现
package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
)

// OpenAIEmbedder OpenAI 向量嵌入器
// 使用 OpenAI Embedding API 将文本转换为向量嵌入
// 支持自定义模型和第三方兼容 API
type OpenAIEmbedder struct {
	apiKey     string         // OpenAI API 密钥
	model      string         // 嵌入模型名称（如 text-embedding-3-small）
	baseURL    string         // API 端点地址
	dimensions int            // 向量维度
	httpClient *http.Client   // HTTP 客户端
}

// NewOpenAIEmbedder 创建 OpenAI 向量嵌入器
// apiKey: OpenAI API 密钥
// model: 嵌入模型名称，默认使用 text-embedding-3-small
// 返回配置好的 OpenAIEmbedder 实例
func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	baseURL := os.Getenv("OPENAI_EMBED_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIEmbedder{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		dimensions: 1536,
		httpClient: &http.Client{Timeout: 30},
	}
}

// Embed 将文本转换为向量嵌入
// ctx: 上下文对象
// text: 要嵌入的文本内容
// 返回向量嵌入切片或错误信息
// 处理流程：
//   1. 清理文本：替换换行符为空格，移除首尾空白
//   2. 截断过长文本（最大 32000 字符）
//   3. 调用 OpenAI Embedding API
//   4. 解析返回的嵌入向量
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// 清理文本
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)
	if text == "" {
		return make([]float32, e.dimensions), nil
	}

	// 限制输入长度（OpenAI max 8192 tokens）
	if len(text) > 32000 {
		text = text[:32000]
	}

	reqBody := map[string]interface{}{
		"input": text,
		"model": e.model,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("api error: %v", errResp)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

// Dimensions 返回向量维度
// 返回嵌入向量的维度数
func (e *OpenAIEmbedder) Dimensions() int {
	return e.dimensions
}

// ClaudeEmbedder Claude 向量嵌入器
// 注意: Anthropic Claude 本身不提供 embedding 功能
// 此实现通过转发到 OpenAI 兼容服务来实现向量嵌入
type ClaudeEmbedder struct {
	apiKey     string         // API 密钥
	model      string         // 模型名称
	baseURL    string         // API 端点
	dimensions int            // 向量维度
	httpClient *http.Client   // HTTP 客户端
}

// NewClaudeEmbedder 创建 Claude 向量嵌入器
// apiKey: API 密钥
// model: 模型名称（可选）
// 返回配置好的 ClaudeEmbedder 实例
func NewClaudeEmbedder(apiKey, model string) *ClaudeEmbedder {
	baseURL := os.Getenv("ANTHROPIC_EMBED_BASE_URL")
	if baseURL == "" {
		// 使用第三方兼容服务
		baseURL = "https://api.anthropic.com/v1"
	}

	return &ClaudeEmbedder{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		dimensions: 1536,
		httpClient: &http.Client{Timeout: 30},
	}
}

// Embed 将文本转换为向量嵌入
// ctx: 上下文对象
// text: 要嵌入的文本内容
// 返回向量嵌入切片或错误信息
// 注意：由于 Anthropic 不提供 embedding API，此实现会转发到 OpenAI 兼容服务
// 支持通过环境变量配置第三方 API 密钥和端点
func (e *ClaudeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Claude 本身不支持 embedding，转发到 OpenAI 兼容服务
	apiKey := os.Getenv("ANTHROPIC_EMBED_API_KEY")
	if apiKey == "" {
		apiKey = e.apiKey
	}
	baseURL := os.Getenv("ANTHROPIC_EMBED_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// 临时创建 OpenAI embedder
	embedder := NewOpenAIEmbedder(apiKey, "text-embedding-3-small")
	embedder.baseURL = baseURL
	return embedder.Embed(ctx, text)
}

// Dimensions 返回向量维度
func (e *ClaudeEmbedder) Dimensions() int {
	return e.dimensions
}

// SimpleEmbedder 简单文本嵌入器
// 基于词袋模型（Bag of Words）和简单哈希的向量化方法
// 用于在没有外部 embedding API 时提供基本的向量化能力
// 注意：此实现简单粗糙，不适合生产环境的语义搜索
type SimpleEmbedder struct {
	vocabulary map[string]int // 词汇表（当前版本未使用，保留用于扩展）
	dimensions int            // 向量维度
}

// NewSimpleEmbedder 创建简单嵌入器
// dimensions: 向量维度，默认 512
// 返回配置好的 SimpleEmbedder 实例
func NewSimpleEmbedder(dimensions int) *SimpleEmbedder {
	if dimensions <= 0 {
		dimensions = 512
	}
	return &SimpleEmbedder{
		vocabulary: make(map[string]int),
		dimensions: dimensions,
	}
}

// Embed 将文本转换为简单向量嵌入
// ctx: 上下文对象（此实现未使用）
// text: 要嵌入的文本内容
// 返回向量嵌入切片或错误信息
// 实现原理：
//   1. 分词：将文本按空白字符分割为单词
//   2. 哈希映射：使用简单的哈希算法将每个词映射到向量索引
//   3. 词频统计：相同哈希位置的词频累加
//   4. L2 归一化：将向量归一化为单位向量
func (e *SimpleEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// 分词
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return make([]float32, e.dimensions), nil
	}

	// 构建词频向量
	vector := make([]float32, e.dimensions)
	for i, word := range words {
		if i >= e.dimensions {
			break
		}
		// 简单的哈希映射
		hash := 0
		for _, c := range word {
			hash = hash*31 + int(c)
		}
		idx := hash % e.dimensions
		if idx < 0 {
			idx = -idx
		}
		vector[idx] += 1
	}

	// 归一化
	var norm float64
	for _, v := range vector {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vector {
			vector[i] = float32(float64(vector[i]) / norm)
		}
	}

	return vector, nil
}

// Dimensions 返回向量维度
// 返回嵌入向量的维度数
func (e *SimpleEmbedder) Dimensions() int {
	return e.dimensions
}
