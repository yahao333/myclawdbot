package llm

import "testing"

func TestNewClientMinimaxAnthropicCompatibleRoute(t *testing.T) {
	client, err := NewClient("minimax", "k", "MiniMax-M2.5", "https://api.minimaxi.com/anthropic", "")
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	defer client.Close()

	if _, ok := client.(*AnthropicClient); !ok {
		t.Fatalf("expected AnthropicClient, got %T", client)
	}
}

func TestNewClientMinimaxLegacyRouteWithGroupID(t *testing.T) {
	client, err := NewClient("minimax", "k", "MiniMax-M2.5", "https://api.minimax.chat", "group-1")
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	defer client.Close()

	minimaxClient, ok := client.(*MinimaxClient)
	if !ok {
		t.Fatalf("expected MinimaxClient, got %T", client)
	}
	if minimaxClient.groupID != "group-1" {
		t.Fatalf("expected groupID to be set, got %q", minimaxClient.groupID)
	}
}
