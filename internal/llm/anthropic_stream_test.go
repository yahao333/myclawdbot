package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yahao333/myclawdbot/pkg/types"
)

func TestAnthropicStreamChatParsesSSEDataWithoutSpace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("response writer does not support flushing")
		}

		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data:{\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}\n\n")
		flusher.Flush()

		time.Sleep(80 * time.Millisecond)

		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data:{\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n")
		flusher.Flush()

		fmt.Fprint(w, "event: message_stop\n")
		fmt.Fprint(w, "data:{\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := NewAnthropicClient("test-key", "MiniMax-M2.5", server.URL)
	if err != nil {
		t.Fatalf("new anthropic client failed: %v", err)
	}

	req := &ChatRequest{
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		MaxTokens: 128,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := client.StreamChat(ctx, req)
	if err != nil {
		t.Fatalf("stream chat failed: %v", err)
	}

	select {
	case first, ok := <-ch:
		if !ok {
			t.Fatalf("stream closed before first delta")
		}
		if first.Content != "Hel" {
			t.Fatalf("unexpected first content: %q", first.Content)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("did not receive first delta in time")
	}

	select {
	case second, ok := <-ch:
		if !ok {
			t.Fatalf("stream closed before second delta")
		}
		if second.Content != "Hello" {
			t.Fatalf("unexpected second content: %q", second.Content)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatalf("did not receive second delta in time")
	}
}

func TestAnthropicStreamChatDoesNotStopAtThinkingBlockEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("response writer does not support flushing")
		}

		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"text\":\"思考中\"}}\n\n")
		flusher.Flush()

		fmt.Fprint(w, "event: content_block_stop\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_stop\"}\n\n")
		flusher.Flush()

		time.Sleep(60 * time.Millisecond)

		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"秋\"}}\n\n")
		flusher.Flush()

		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"风\"}}\n\n")
		flusher.Flush()

		fmt.Fprint(w, "event: message_stop\n")
		fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := NewAnthropicClient("test-key", "MiniMax-M2.5", server.URL)
	if err != nil {
		t.Fatalf("new anthropic client failed: %v", err)
	}

	req := &ChatRequest{
		Messages:  []types.Message{{Role: "user", Content: "写诗"}},
		MaxTokens: 128,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := client.StreamChat(ctx, req)
	if err != nil {
		t.Fatalf("stream chat failed: %v", err)
	}

	select {
	case first, ok := <-ch:
		if !ok {
			t.Fatalf("stream closed before text delta")
		}
		if first.Content != "秋" {
			t.Fatalf("unexpected first content: %q", first.Content)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("did not receive first text delta in time")
	}

	select {
	case second, ok := <-ch:
		if !ok {
			t.Fatalf("stream closed before second text delta")
		}
		if second.Content != "秋风" {
			t.Fatalf("unexpected second content: %q", second.Content)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("did not receive second text delta in time")
	}
}
