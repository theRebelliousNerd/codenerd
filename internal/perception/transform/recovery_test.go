package transform

import (
	"strings"
	"testing"
)

func TestAnalyzeConversationState_EmptyContents(t *testing.T) {
	state := AnalyzeConversationState(nil)

	if state.InToolLoop {
		t.Error("Empty contents should not be in tool loop")
	}
	if state.TurnStartIdx != -1 {
		t.Errorf("Expected TurnStartIdx=-1, got %d", state.TurnStartIdx)
	}
	if state.LastModelIdx != -1 {
		t.Errorf("Expected LastModelIdx=-1, got %d", state.LastModelIdx)
	}
}

func TestAnalyzeConversationState_SimpleConversation(t *testing.T) {
	contents := []map[string]interface{}{
		{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Hello"}}},
		{"role": "model", "parts": []interface{}{map[string]interface{}{"text": "Hi!"}}},
	}

	state := AnalyzeConversationState(contents)

	if state.InToolLoop {
		t.Error("Simple conversation should not be in tool loop")
	}
	if state.TurnStartIdx != 1 {
		t.Errorf("Expected TurnStartIdx=1, got %d", state.TurnStartIdx)
	}
	if state.LastModelIdx != 1 {
		t.Errorf("Expected LastModelIdx=1, got %d", state.LastModelIdx)
	}
}

func TestAnalyzeConversationState_InToolLoop(t *testing.T) {
	contents := []map[string]interface{}{
		{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Read file.txt"}}},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
			},
		},
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"functionResponse": map[string]interface{}{"name": "read_file", "response": "contents"}},
			},
		},
	}

	state := AnalyzeConversationState(contents)

	if !state.InToolLoop {
		t.Error("Should detect tool loop when ending with functionResponse")
	}
	if state.TurnStartIdx != 1 {
		t.Errorf("Expected TurnStartIdx=1 (first model after user), got %d", state.TurnStartIdx)
	}
	if state.LastModelIdx != 1 {
		t.Errorf("Expected LastModelIdx=1, got %d", state.LastModelIdx)
	}
	if !state.LastModelHasToolCalls {
		t.Error("LastModelHasToolCalls should be true")
	}
}

func TestAnalyzeConversationState_TurnWithThinking(t *testing.T) {
	contents := []map[string]interface{}{
		{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Explain this"}}},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"thought": true, "text": "Let me think..."},
				map[string]interface{}{"text": "Here's my explanation"},
			},
		},
	}

	state := AnalyzeConversationState(contents)

	if !state.TurnHasThinking {
		t.Error("TurnHasThinking should be true when model message has thinking")
	}
	if !state.LastModelHasThinking {
		t.Error("LastModelHasThinking should be true")
	}
}

func TestAnalyzeConversationState_MultiTurnToolLoop(t *testing.T) {
	// Complex scenario: user -> model (with thinking) -> tool result -> model (tool call) -> tool result
	contents := []map[string]interface{}{
		{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Process files"}}},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"thought": true, "text": "Thinking about approach..."},
				map[string]interface{}{"functionCall": map[string]interface{}{"name": "list_files"}},
			},
		},
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"functionResponse": map[string]interface{}{"name": "list_files", "response": "[file1.txt]"}},
			},
		},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
			},
		},
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"functionResponse": map[string]interface{}{"name": "read_file", "response": "file contents"}},
			},
		},
	}

	state := AnalyzeConversationState(contents)

	if !state.InToolLoop {
		t.Error("Should be in tool loop")
	}
	// Turn started at index 1 (first model message after real user)
	if state.TurnStartIdx != 1 {
		t.Errorf("Expected TurnStartIdx=1, got %d", state.TurnStartIdx)
	}
	// The turn STARTED with thinking (even though later messages don't have it)
	if !state.TurnHasThinking {
		t.Error("TurnHasThinking should be true (turn started with thinking)")
	}
	// Last model message (index 3) does NOT have thinking
	if state.LastModelHasThinking {
		t.Error("LastModelHasThinking should be false (last model msg has no thinking)")
	}
	if state.LastModelIdx != 3 {
		t.Errorf("Expected LastModelIdx=3, got %d", state.LastModelIdx)
	}
}

func TestNeedsThinkingRecovery(t *testing.T) {
	tests := []struct {
		name     string
		state    *ConversationState
		expected bool
	}{
		{
			name:     "In tool loop without thinking",
			state:    &ConversationState{InToolLoop: true, TurnHasThinking: false},
			expected: true,
		},
		{
			name:     "In tool loop with thinking",
			state:    &ConversationState{InToolLoop: true, TurnHasThinking: true},
			expected: false,
		},
		{
			name:     "Not in tool loop without thinking",
			state:    &ConversationState{InToolLoop: false, TurnHasThinking: false},
			expected: false,
		},
		{
			name:     "Not in tool loop with thinking",
			state:    &ConversationState{InToolLoop: false, TurnHasThinking: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsThinkingRecovery(tt.state)
			if got != tt.expected {
				t.Errorf("NeedsThinkingRecovery() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCloseToolLoopForThinking(t *testing.T) {
	contents := []map[string]interface{}{
		{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Read file"}}},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
			},
		},
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"functionResponse": map[string]interface{}{"name": "read_file", "response": "contents"}},
			},
		},
	}

	result := CloseToolLoopForThinking(contents)

	// Should add 2 synthetic messages
	if len(result) != 5 {
		t.Errorf("Expected 5 messages (3 original + 2 synthetic), got %d", len(result))
	}

	// Second-to-last should be synthetic model message
	syntheticModel := result[3]
	if syntheticModel["role"] != "model" {
		t.Error("Synthetic model message should have role='model'")
	}
	modelParts := syntheticModel["parts"].([]interface{})
	modelText := modelParts[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(modelText, "Tool execution completed") && !strings.Contains(modelText, "tool execution") {
		t.Errorf("Synthetic model message should mention tool execution, got: %s", modelText)
	}

	// Last should be synthetic user message
	syntheticUser := result[4]
	if syntheticUser["role"] != "user" {
		t.Error("Synthetic user message should have role='user'")
	}
	userParts := syntheticUser["parts"].([]interface{})
	userText := userParts[0].(map[string]interface{})["text"].(string)
	if userText != "[Continue]" {
		t.Errorf("Expected '[Continue]', got: %s", userText)
	}
}

func TestCloseToolLoopForThinking_StripsExistingThinking(t *testing.T) {
	contents := []map[string]interface{}{
		{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Read file"}}},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"thought": true, "text": "Corrupted thinking"},
				map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
			},
		},
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"functionResponse": map[string]interface{}{"name": "read_file", "response": "contents"}},
			},
		},
	}

	result := CloseToolLoopForThinking(contents)

	// Check that thinking was stripped from original model message
	modelMsg := result[1]
	parts := modelMsg["parts"].([]interface{})

	for _, p := range parts {
		part := p.(map[string]interface{})
		if IsThinkingPart(part) {
			t.Error("Thinking blocks should have been stripped")
		}
	}
}

func TestCloseToolLoopForThinking_MultipleToolResults(t *testing.T) {
	contents := []map[string]interface{}{
		{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Read files"}}},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file", "args": map[string]interface{}{"path": "a.txt"}}},
				map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file", "args": map[string]interface{}{"path": "b.txt"}}},
			},
		},
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"functionResponse": map[string]interface{}{"name": "read_file", "response": "contents a"}},
				map[string]interface{}{"functionResponse": map[string]interface{}{"name": "read_file", "response": "contents b"}},
			},
		},
	}

	result := CloseToolLoopForThinking(contents)

	// Check synthetic model message mentions 2 tool executions
	syntheticModel := result[3]
	modelParts := syntheticModel["parts"].([]interface{})
	modelText := modelParts[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(modelText, "2") {
		t.Errorf("Synthetic model message should mention 2 tool executions, got: %s", modelText)
	}
}

func TestLooksLikeCompactedThinkingTurn(t *testing.T) {
	tests := []struct {
		name     string
		msg      map[string]interface{}
		expected bool
	}{
		{
			name: "Function call without text before - looks compacted",
			msg: map[string]interface{}{
				"role": "model",
				"parts": []interface{}{
					map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
				},
			},
			expected: true,
		},
		{
			name: "Function call with text before - normal",
			msg: map[string]interface{}{
				"role": "model",
				"parts": []interface{}{
					map[string]interface{}{"text": "Let me read that file"},
					map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
				},
			},
			expected: false,
		},
		{
			name: "Function call with thinking before - has thinking",
			msg: map[string]interface{}{
				"role": "model",
				"parts": []interface{}{
					map[string]interface{}{"thought": true, "text": "Thinking..."},
					map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
				},
			},
			expected: false,
		},
		{
			name: "No function call",
			msg: map[string]interface{}{
				"role": "model",
				"parts": []interface{}{
					map[string]interface{}{"text": "Just text response"},
				},
			},
			expected: false,
		},
		{
			name:     "Empty parts",
			msg:      map[string]interface{}{"role": "model", "parts": []interface{}{}},
			expected: false,
		},
		{
			name:     "No parts field",
			msg:      map[string]interface{}{"role": "model"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LooksLikeCompactedThinkingTurn(tt.msg)
			if got != tt.expected {
				t.Errorf("LooksLikeCompactedThinkingTurn() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHasPossibleCompactedThinking(t *testing.T) {
	contents := []map[string]interface{}{
		{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Read files"}}},
		{
			"role": "model",
			"parts": []interface{}{
				// No text before function call - looks compacted
				map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
			},
		},
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"functionResponse": map[string]interface{}{"name": "read_file", "response": "contents"}},
			},
		},
	}

	// Turn starts at index 1
	if !HasPossibleCompactedThinking(contents, 1) {
		t.Error("Should detect possible compacted thinking")
	}

	// Invalid turn start
	if HasPossibleCompactedThinking(contents, -1) {
		t.Error("Should return false for invalid turnStartIdx")
	}
	if HasPossibleCompactedThinking(contents, 10) {
		t.Error("Should return false for out-of-bounds turnStartIdx")
	}
}

func TestMessageHasThinking(t *testing.T) {
	tests := []struct {
		name     string
		msg      map[string]interface{}
		expected bool
	}{
		{
			name: "Gemini format with thought=true",
			msg: map[string]interface{}{
				"role":  "model",
				"parts": []interface{}{map[string]interface{}{"thought": true, "text": "thinking"}},
			},
			expected: true,
		},
		{
			name: "Claude format with type=thinking",
			msg: map[string]interface{}{
				"role":    "assistant",
				"content": []interface{}{map[string]interface{}{"type": "thinking", "text": "thinking"}},
			},
			expected: true,
		},
		{
			name: "Claude format with type=redacted_thinking",
			msg: map[string]interface{}{
				"role":    "assistant",
				"content": []interface{}{map[string]interface{}{"type": "redacted_thinking"}},
			},
			expected: true,
		},
		{
			name: "No thinking content",
			msg: map[string]interface{}{
				"role":  "model",
				"parts": []interface{}{map[string]interface{}{"text": "normal response"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := messageHasThinking(tt.msg)
			if got != tt.expected {
				t.Errorf("messageHasThinking() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMessageHasToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		msg      map[string]interface{}
		expected bool
	}{
		{
			name: "Gemini format with functionCall",
			msg: map[string]interface{}{
				"role":  "model",
				"parts": []interface{}{map[string]interface{}{"functionCall": map[string]interface{}{"name": "read"}}},
			},
			expected: true,
		},
		{
			name: "Claude format with tool_use",
			msg: map[string]interface{}{
				"role":    "assistant",
				"content": []interface{}{map[string]interface{}{"type": "tool_use", "name": "read"}},
			},
			expected: true,
		},
		{
			name: "No tool calls",
			msg: map[string]interface{}{
				"role":  "model",
				"parts": []interface{}{map[string]interface{}{"text": "just text"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := messageHasToolCalls(tt.msg)
			if got != tt.expected {
				t.Errorf("messageHasToolCalls() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCountTrailingToolResults(t *testing.T) {
	tests := []struct {
		name     string
		contents []map[string]interface{}
		expected int
	}{
		{
			name: "Single tool result",
			contents: []map[string]interface{}{
				{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Hello"}}},
				{"role": "model", "parts": []interface{}{map[string]interface{}{"functionCall": map[string]interface{}{"name": "test"}}}},
				{"role": "user", "parts": []interface{}{map[string]interface{}{"functionResponse": map[string]interface{}{"name": "test"}}}},
			},
			expected: 1,
		},
		{
			name: "Multiple tool results in one message",
			contents: []map[string]interface{}{
				{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Hello"}}},
				{"role": "model", "parts": []interface{}{
					map[string]interface{}{"functionCall": map[string]interface{}{"name": "test1"}},
					map[string]interface{}{"functionCall": map[string]interface{}{"name": "test2"}},
				}},
				{"role": "user", "parts": []interface{}{
					map[string]interface{}{"functionResponse": map[string]interface{}{"name": "test1"}},
					map[string]interface{}{"functionResponse": map[string]interface{}{"name": "test2"}},
				}},
			},
			expected: 2,
		},
		{
			name: "Ends with model message",
			contents: []map[string]interface{}{
				{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Hello"}}},
				{"role": "model", "parts": []interface{}{map[string]interface{}{"text": "Hi!"}}},
			},
			expected: 0,
		},
		{
			name: "Ends with real user message",
			contents: []map[string]interface{}{
				{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Hello"}}},
				{"role": "model", "parts": []interface{}{map[string]interface{}{"text": "Hi!"}}},
				{"role": "user", "parts": []interface{}{map[string]interface{}{"text": "Thanks"}}},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countTrailingToolResults(tt.contents)
			if got != tt.expected {
				t.Errorf("countTrailingToolResults() = %d, want %d", got, tt.expected)
			}
		})
	}
}
