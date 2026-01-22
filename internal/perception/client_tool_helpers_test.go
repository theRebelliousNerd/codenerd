package perception

import (
	"testing"
)

func TestMapToolDefinitionsToOpenAI(t *testing.T) {
	tools := []ToolDefinition{
		{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"arg": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	openAITools := MapToolDefinitionsToOpenAI(tools)

	if len(openAITools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(openAITools))
	}

	if openAITools[0].Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", openAITools[0].Type)
	}

	if openAITools[0].Function.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", openAITools[0].Function.Name)
	}

	if openAITools[0].Function.Description != "A test tool" {
		t.Errorf("Expected description 'A test tool', got '%s'", openAITools[0].Function.Description)
	}
}

func TestMapOpenAIToolCallsToInternal(t *testing.T) {
	calls := []OpenAIToolCall{
		{
			ID:   "call_123",
			Type: "function",
			Function: OpenAIFunctionCall{
				Name:      "test_tool",
				Arguments: `{"arg": "value"}`,
			},
		},
	}

	internalCalls, err := MapOpenAIToolCallsToInternal(calls)
	if err != nil {
		t.Fatalf("Failed to map tool calls: %v", err)
	}

	if len(internalCalls) != 1 {
		t.Fatalf("Expected 1 internal call, got %d", len(internalCalls))
	}

	if internalCalls[0].ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got '%s'", internalCalls[0].ID)
	}

	if internalCalls[0].Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", internalCalls[0].Name)
	}

	if val, ok := internalCalls[0].Input["arg"].(string); !ok || val != "value" {
		t.Errorf("Expected arg 'value', got %v", internalCalls[0].Input["arg"])
	}
}

func TestMapOpenAIToolCallsToInternal_InvalidJSON(t *testing.T) {
	calls := []OpenAIToolCall{
		{
			ID:   "call_bad",
			Type: "function",
			Function: OpenAIFunctionCall{
				Name:      "bad_tool",
				Arguments: `{invalid json`,
			},
		},
	}

	_, err := MapOpenAIToolCallsToInternal(calls)
	if err == nil {
		t.Error("Expected error for invalid JSON arguments, got nil")
	}
}
