package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMessage_JSON(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, World!",
		Timestamp: time.Now(),
	}

	// 序列化
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	// 反序列化
	var decoded Message
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	// 验证
	if decoded.Role != msg.Role {
		t.Errorf("Role = %s, want %s", decoded.Role, msg.Role)
	}
	if decoded.Content != msg.Content {
		t.Errorf("Content = %s, want %s", decoded.Content, msg.Content)
	}
}

func TestMessage_WithToolCalls(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "I'll read that file for you",
		ToolCalls: []ToolCall{
			{
				ID:   "call_123",
				Name: "read",
				Args: map[string]any{"path": "/tmp/test.txt"},
			},
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if len(decoded.ToolCalls) != 1 {
		t.Errorf("ToolCalls length = %d, want 1", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].Name != "read" {
		t.Errorf("ToolCall Name = %s, want 'read'", decoded.ToolCalls[0].Name)
	}
}

func TestToolCall_JSON(t *testing.T) {
	call := ToolCall{
		ID:   "call_abc",
		Name: "write",
		Args: map[string]any{
			"path":    "/tmp/test.txt",
			"content": "hello",
		},
		Result: "File written successfully",
	}

	data, err := json.Marshal(call)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ToolCall
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.ID != call.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, call.ID)
	}
	if decoded.Name != call.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, call.Name)
	}
	if decoded.Result != call.Result {
		t.Errorf("Result = %s, want %s", decoded.Result, call.Result)
	}
}

func TestToolDefinition_JSON(t *testing.T) {
	def := ToolDefinition{
		Name:        "read",
		Description: "Read file contents",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path",
				},
			},
		},
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ToolDefinition
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.Name != def.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, def.Name)
	}
	if decoded.Description != def.Description {
		t.Errorf("Description = %s, want %s", decoded.Description, def.Description)
	}
}

func TestContentBlock_Text(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Text: "Hello, World!",
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ContentBlock
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.Type != "text" {
		t.Errorf("Type = %s, want 'text'", decoded.Type)
	}
	if decoded.Text != "Hello, World!" {
		t.Errorf("Text = %s, want 'Hello, World!'", decoded.Text)
	}
}

func TestContentBlock_ToolUse(t *testing.T) {
	block := ContentBlock{
		Type: "tool_use",
		ToolUse: &ToolUseBlock{
			ID:   "call_123",
			Name: "read",
			Input: map[string]any{
				"path": "/tmp/test.txt",
			},
		},
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ContentBlock
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.ToolUse == nil {
		t.Fatal("ToolUse is nil")
	}
	if decoded.ToolUse.ID != "call_123" {
		t.Errorf("ToolUse.ID = %s, want 'call_123'", decoded.ToolUse.ID)
	}
	if decoded.ToolUse.Name != "read" {
		t.Errorf("ToolUse.Name = %s, want 'read'", decoded.ToolUse.Name)
	}
}

func TestContentBlock_ToolResult(t *testing.T) {
	block := ContentBlock{
		Type: "tool_result",
		ToolResult: &ToolResultBlock{
			ToolUseID: "call_123",
			Content:   "File content here",
			IsError:   false,
		},
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ContentBlock
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.ToolResult == nil {
		t.Fatal("ToolResult is nil")
	}
	if decoded.ToolResult.ToolUseID != "call_123" {
		t.Errorf("ToolResult.ToolUseID = %s, want 'call_123'", decoded.ToolResult.ToolUseID)
	}
	if decoded.ToolResult.IsError != false {
		t.Errorf("ToolResult.IsError = %v, want false", decoded.ToolResult.IsError)
	}
}

func TestStreamDelta(t *testing.T) {
	delta := StreamDelta{
		Type:  "content_block_delta",
		Index: 0,
		Delta: "Hello",
	}

	data, err := json.Marshal(delta)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded StreamDelta
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.Type != "content_block_delta" {
		t.Errorf("Type = %s, want 'content_block_delta'", decoded.Type)
	}
	if decoded.Index != 0 {
		t.Errorf("Index = %d, want 0", decoded.Index)
	}
	if decoded.Delta != "Hello" {
		t.Errorf("Delta = %s, want 'Hello'", decoded.Delta)
	}
}

func TestErrorResponse(t *testing.T) {
	errResp := ErrorResponse{
		Error:   "invalid_request",
		Message: "The request was invalid",
	}

	data, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ErrorResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.Error != "invalid_request" {
		t.Errorf("Error = %s, want 'invalid_request'", decoded.Error)
	}
	if decoded.Message != "The request was invalid" {
		t.Errorf("Message = %s, want 'The request was invalid'", decoded.Message)
	}
}

func TestToolCall_ArgsSerialization(t *testing.T) {
	call := ToolCall{
		ID:   "test",
		Name: "test_tool",
		Args: map[string]any{
			"string_key": "value",
			"int_key":    42,
			"float_key":  3.14,
			"bool_key":   true,
			"array_key":  []any{1, 2, 3},
			"nested": map[string]any{
				"key": "value",
			},
		},
	}

	data, err := json.Marshal(call)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ToolCall
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	// 验证嵌套数据
	strVal, ok := decoded.Args["string_key"].(string)
	if !ok || strVal != "value" {
		t.Errorf("string_key = %v, want 'value'", decoded.Args["string_key"])
	}
	intVal, ok := decoded.Args["int_key"].(float64)
	if !ok || intVal != 42 {
		t.Errorf("int_key = %v, want 42", decoded.Args["int_key"])
	}
}
