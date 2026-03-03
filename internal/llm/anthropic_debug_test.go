package llm

import "testing"

func TestStreamDebugEnabled(t *testing.T) {
	t.Setenv("LLM_DEBUG_STREAM", "1")
	if !streamDebugEnabled() {
		t.Fatalf("expected stream debug enabled for value 1")
	}

	t.Setenv("LLM_DEBUG_STREAM", "true")
	if !streamDebugEnabled() {
		t.Fatalf("expected stream debug enabled for value true")
	}

	t.Setenv("LLM_DEBUG_STREAM", "on")
	if !streamDebugEnabled() {
		t.Fatalf("expected stream debug enabled for value on")
	}

	t.Setenv("LLM_DEBUG_STREAM", "0")
	if streamDebugEnabled() {
		t.Fatalf("expected stream debug disabled for value 0")
	}

	t.Setenv("LLM_DEBUG_STREAM", "")
	if streamDebugEnabled() {
		t.Fatalf("expected stream debug disabled for empty value")
	}
}
