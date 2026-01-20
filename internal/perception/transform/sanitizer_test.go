package transform

import (
	"testing"
)

func TestSanitizeCrossModelPayload_GeminiToClaude(t *testing.T) {
	// Gemini conversation history with thoughtSignature
	contents := []map[string]interface{}{
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{
					"thought":          true,
					"text":             "Let me think about this...",
					"thoughtSignature": "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def",
				},
				map[string]interface{}{
					"text": "Here is my response",
				},
			},
		},
	}

	result := SanitizeCrossModelPayload(contents, "claude-sonnet-4-5")

	if !result.Modified {
		t.Error("Expected Modified=true when stripping Gemini signatures for Claude")
	}
	if result.SignaturesStripped != 1 {
		t.Errorf("Expected 1 signature stripped, got %d", result.SignaturesStripped)
	}

	// Verify thoughtSignature was removed
	parts := contents[0]["parts"].([]interface{})
	thinkingPart := parts[0].(map[string]interface{})
	if _, exists := thinkingPart["thoughtSignature"]; exists {
		t.Error("thoughtSignature should have been removed for Claude target")
	}
}

func TestSanitizeCrossModelPayload_ClaudeToGemini(t *testing.T) {
	// Claude conversation history with signature in thinking blocks
	contents := []map[string]interface{}{
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{
					"type":      "thinking",
					"text":      "My reasoning process...",
					"signature": "xyz789abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890",
				},
				map[string]interface{}{
					"text": "Here is my response",
				},
			},
		},
	}

	result := SanitizeCrossModelPayload(contents, "gemini-3-flash")

	if !result.Modified {
		t.Error("Expected Modified=true when stripping Claude signatures for Gemini")
	}
	if result.SignaturesStripped != 1 {
		t.Errorf("Expected 1 signature stripped, got %d", result.SignaturesStripped)
	}

	// Verify signature was removed
	parts := contents[0]["parts"].([]interface{})
	thinkingPart := parts[0].(map[string]interface{})
	if _, exists := thinkingPart["signature"]; exists {
		t.Error("signature should have been removed for Gemini target")
	}
}

func TestSanitizeCrossModelPayload_NestedGoogleMetadata(t *testing.T) {
	// Gemini metadata nested in metadata.google
	contents := []map[string]interface{}{
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{
					"text": "Response with nested metadata",
					"metadata": map[string]interface{}{
						"google": map[string]interface{}{
							"thoughtSignature": "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def",
							"thinkingMetadata": map[string]interface{}{"tokens": 1024},
							"otherField":       "should stay",
						},
					},
				},
			},
		},
	}

	result := SanitizeCrossModelPayload(contents, "claude-sonnet-4-5-thinking")

	if result.SignaturesStripped != 2 {
		t.Errorf("Expected 2 signatures stripped (thoughtSignature + thinkingMetadata), got %d", result.SignaturesStripped)
	}

	// Verify nested metadata was cleaned but other fields preserved
	parts := contents[0]["parts"].([]interface{})
	part := parts[0].(map[string]interface{})
	metadata := part["metadata"].(map[string]interface{})
	google := metadata["google"].(map[string]interface{})

	if _, exists := google["thoughtSignature"]; exists {
		t.Error("thoughtSignature should have been removed from nested google metadata")
	}
	if _, exists := google["thinkingMetadata"]; exists {
		t.Error("thinkingMetadata should have been removed from nested google metadata")
	}
	if google["otherField"] != "should stay" {
		t.Error("Non-signature fields should be preserved")
	}
}

func TestSanitizeCrossModelPayload_NoModificationNeeded(t *testing.T) {
	// Clean conversation without signatures
	contents := []map[string]interface{}{
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{
					"text": "Hello",
				},
			},
		},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{
					"text": "Hi there!",
				},
			},
		},
	}

	result := SanitizeCrossModelPayload(contents, "gemini-3-flash")

	if result.Modified {
		t.Error("Expected Modified=false when no signatures to strip")
	}
	if result.SignaturesStripped != 0 {
		t.Errorf("Expected 0 signatures stripped, got %d", result.SignaturesStripped)
	}
}

func TestSanitizeCrossModelPayload_UnknownModel(t *testing.T) {
	contents := []map[string]interface{}{
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{
					"thoughtSignature": "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def",
				},
			},
		},
	}

	result := SanitizeCrossModelPayload(contents, "some-unknown-model")

	if result.Modified {
		t.Error("Expected Modified=false for unknown model (no sanitization applied)")
	}
}

func TestIsThinkingPart(t *testing.T) {
	tests := []struct {
		name     string
		part     map[string]interface{}
		expected bool
	}{
		{
			name:     "nil part",
			part:     nil,
			expected: false,
		},
		{
			name:     "Gemini thought=true",
			part:     map[string]interface{}{"thought": true, "text": "reasoning..."},
			expected: true,
		},
		{
			name:     "Gemini thought=false",
			part:     map[string]interface{}{"thought": false, "text": "normal text"},
			expected: false,
		},
		{
			name:     "Claude thinking type",
			part:     map[string]interface{}{"type": "thinking", "text": "reasoning..."},
			expected: true,
		},
		{
			name:     "Claude redacted_thinking",
			part:     map[string]interface{}{"type": "redacted_thinking"},
			expected: true,
		},
		{
			name:     "Claude reasoning type",
			part:     map[string]interface{}{"type": "reasoning", "text": "reasoning..."},
			expected: true,
		},
		{
			name:     "Text part only",
			part:     map[string]interface{}{"text": "Hello world"},
			expected: false,
		},
		{
			name:     "Tool call part",
			part:     map[string]interface{}{"functionCall": map[string]interface{}{"name": "read_file"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsThinkingPart(tt.part)
			if got != tt.expected {
				t.Errorf("IsThinkingPart() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHasValidSignature(t *testing.T) {
	longSig := "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def"
	shortSig := "abc123"

	tests := []struct {
		name     string
		part     map[string]interface{}
		expected bool
	}{
		{
			name:     "Gemini thoughtSignature (valid length)",
			part:     map[string]interface{}{"thoughtSignature": longSig},
			expected: true,
		},
		{
			name:     "Claude signature (valid length)",
			part:     map[string]interface{}{"signature": longSig},
			expected: true,
		},
		{
			name:     "Short thoughtSignature",
			part:     map[string]interface{}{"thoughtSignature": shortSig},
			expected: false,
		},
		{
			name:     "Short signature",
			part:     map[string]interface{}{"signature": shortSig},
			expected: false,
		},
		{
			name:     "No signature fields",
			part:     map[string]interface{}{"text": "Hello"},
			expected: false,
		},
		{
			name:     "Empty part",
			part:     map[string]interface{}{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasValidSignature(tt.part)
			if got != tt.expected {
				t.Errorf("HasValidSignature() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStripAllThinkingBlocks(t *testing.T) {
	contents := []map[string]interface{}{
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"thought": true, "text": "Thinking..."},
				map[string]interface{}{"text": "Response"},
			},
		},
		{
			"role": "user",
			"parts": []interface{}{
				map[string]interface{}{"text": "Follow up"},
			},
		},
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"type": "thinking", "text": "More thinking"},
				map[string]interface{}{"text": "Another response"},
			},
		},
	}

	result := StripAllThinkingBlocks(contents)

	// Check first message - thinking stripped
	parts0 := result[0]["parts"].([]interface{})
	if len(parts0) != 1 {
		t.Errorf("Expected 1 part after stripping, got %d", len(parts0))
	}
	text0 := parts0[0].(map[string]interface{})["text"]
	if text0 != "Response" {
		t.Errorf("Expected 'Response', got %v", text0)
	}

	// Check user message - unchanged
	parts1 := result[1]["parts"].([]interface{})
	if len(parts1) != 1 {
		t.Error("User message should be unchanged")
	}

	// Check third message - thinking stripped
	parts2 := result[2]["parts"].([]interface{})
	if len(parts2) != 1 {
		t.Errorf("Expected 1 part after stripping, got %d", len(parts2))
	}
	text2 := parts2[0].(map[string]interface{})["text"]
	if text2 != "Another response" {
		t.Errorf("Expected 'Another response', got %v", text2)
	}
}

func TestStripAllThinkingBlocks_PreservesOnlyThinkingMessage(t *testing.T) {
	// If a message has ONLY thinking, keep it (avoid empty messages)
	contents := []map[string]interface{}{
		{
			"role": "model",
			"parts": []interface{}{
				map[string]interface{}{"thought": true, "text": "Only thinking, no response"},
			},
		},
	}

	result := StripAllThinkingBlocks(contents)

	// Should keep the original message to avoid empty parts
	if len(result) != 1 {
		t.Errorf("Expected 1 message preserved, got %d", len(result))
	}
	parts := result[0]["parts"].([]interface{})
	if len(parts) != 1 {
		t.Error("Original parts should be preserved when stripping would result in empty")
	}
}

func TestFilterUnsignedThinkingBlocks(t *testing.T) {
	longSig := "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def"

	contents := []map[string]interface{}{
		{
			"role": "model",
			"parts": []interface{}{
				// Signed thinking - should be kept
				map[string]interface{}{"thought": true, "text": "Signed thinking", "thoughtSignature": longSig},
				// Unsigned thinking - should be removed
				map[string]interface{}{"thought": true, "text": "Unsigned thinking"},
				// Normal text - should be kept
				map[string]interface{}{"text": "Response"},
			},
		},
	}

	result := FilterUnsignedThinkingBlocks(contents)

	parts := result[0]["parts"].([]interface{})
	if len(parts) != 2 {
		t.Errorf("Expected 2 parts (signed thinking + response), got %d", len(parts))
	}

	// First part should be the signed thinking
	part0 := parts[0].(map[string]interface{})
	if part0["text"] != "Signed thinking" {
		t.Error("Signed thinking should be preserved")
	}

	// Second part should be the response
	part1 := parts[1].(map[string]interface{})
	if part1["text"] != "Response" {
		t.Error("Response text should be preserved")
	}
}

func TestStripClaudeThinkingFields_RedactedThinking(t *testing.T) {
	longSig := "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def"

	part := map[string]interface{}{
		"type":      "redacted_thinking",
		"signature": longSig,
	}

	stripped := stripClaudeThinkingFields(part)

	if stripped != 1 {
		t.Errorf("Expected 1 field stripped, got %d", stripped)
	}
	if _, exists := part["signature"]; exists {
		t.Error("signature should have been removed from redacted_thinking block")
	}
}

func TestStripGeminiThinkingMetadata_CleansUpEmptyObjects(t *testing.T) {
	longSig := "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567abc890def"

	part := map[string]interface{}{
		"text": "Response",
		"metadata": map[string]interface{}{
			"google": map[string]interface{}{
				"thoughtSignature": longSig,
			},
		},
	}

	stripped := stripGeminiThinkingMetadata(part)

	if stripped != 1 {
		t.Errorf("Expected 1 field stripped, got %d", stripped)
	}

	// Both metadata and metadata.google should be cleaned up (empty)
	if _, exists := part["metadata"]; exists {
		t.Error("Empty metadata object should have been removed")
	}
}
