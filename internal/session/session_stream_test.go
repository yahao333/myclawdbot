package session

import (
	"context"
	"testing"
	"time"

	"github.com/yahao333/myclawdbot/internal/llm"
	"github.com/yahao333/myclawdbot/pkg/types"
)

type streamOnlyClient struct{}

func (c *streamOnlyClient) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "fallback"}, nil
}

func (c *streamOnlyClient) StreamChat(ctx context.Context, req *llm.ChatRequest) (<-chan *llm.ChatResponse, error) {
	ch := make(chan *llm.ChatResponse, 2)
	go func() {
		defer close(ch)
		ch <- &llm.ChatResponse{Content: "你"}
		time.Sleep(20 * time.Millisecond)
		ch <- &llm.ChatResponse{Content: "你好"}
	}()
	return ch, nil
}

func (c *streamOnlyClient) Tools() []types.ToolDefinition {
	return nil
}

func (c *streamOnlyClient) Close() error {
	return nil
}

func TestSendMessageStreamProducesIncrementalCallback(t *testing.T) {
	sess := &Session{
		ID:       "test",
		Messages: make([]types.Message, 0),
	}
	client := &streamOnlyClient{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	parts := make([]string, 0, 2)
	content, err := sess.SendMessageStream(ctx, client, "hi", func(delta string) {
		parts = append(parts, delta)
	})
	if err != nil {
		t.Fatalf("send message stream failed: %v", err)
	}

	if content != "你好" {
		t.Fatalf("unexpected final content: %q", content)
	}
	if len(parts) != 2 {
		t.Fatalf("unexpected callback count: %d", len(parts))
	}
	if parts[0] != "你" || parts[1] != "好" {
		t.Fatalf("unexpected callback values: %#v", parts)
	}
}
