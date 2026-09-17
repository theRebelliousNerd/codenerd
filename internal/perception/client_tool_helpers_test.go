package perception

import (
	"strings"
	"testing"
)

func TestMapToolDefinitionsToOpenAI(t *testing.T) {
	tools := []ToolDefinition{
		{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"arg": map[string]any{"type": "string"},
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

	internalCalls, err := MapOpenAIToolCallsToInternal(calls)
	if err != nil {
		t.Fatalf("expected invalid JSON arguments to be reported via ArgsError, got error: %v", err)
	}

	if internalCalls[0].ArgsError == "" {
		t.Error("expected ArgsError to be set for invalid JSON arguments")
	}

	if len(internalCalls[0].Input) != 0 {
		t.Errorf("expected empty Input for invalid JSON arguments, got %v", internalCalls[0].Input)
	}
}

func TestMapOpenAIToolCallsToInternal_TruncatedJSONKeepsValidCall(t *testing.T) {
	calls := []OpenAIToolCall{
		{
			ID:   "call_truncated",
			Type: "function",
			Function: OpenAIFunctionCall{
				Name:      "edit_lines",
				Arguments: `{"path":"a.go","start_line":`,
			},
		},
		{
			ID:   "call_ok",
			Type: "function",
			Function: OpenAIFunctionCall{
				Name:      "read_file",
				Arguments: `{"path":"b.go"}`,
			},
		},
	}

	internalCalls, err := MapOpenAIToolCallsToInternal(calls)
	if err != nil {
		t.Fatalf("expected no error when one call has truncated arguments, got: %v", err)
	}

	if len(internalCalls) != 2 {
		t.Fatalf("expected 2 internal calls, got %d", len(internalCalls))
	}

	if internalCalls[0].ArgsError == "" {
		t.Error("expected ArgsError to be set for the truncated call")
	} else {
		if !strings.Contains(internalCalls[0].ArgsError, "not valid JSON") {
			t.Errorf("expected ArgsError to mention 'not valid JSON', got %q", internalCalls[0].ArgsError)
		}
		if !strings.Contains(internalCalls[0].ArgsError, "edit_lines") {
			t.Errorf("expected ArgsError to mention the tool name, got %q", internalCalls[0].ArgsError)
		}
	}

	if len(internalCalls[0].Input) != 0 {
		t.Errorf("expected empty Input for the truncated call, got %v", internalCalls[0].Input)
	}

	if path, ok := internalCalls[1].Input["path"].(string); !ok || path != "b.go" {
		t.Errorf("expected decoded path 'b.go' for the valid call, got %v", internalCalls[1].Input["path"])
	}

	if internalCalls[1].ArgsError != "" {
		t.Errorf("expected empty ArgsError for the valid call, got %q", internalCalls[1].ArgsError)
	}
}
