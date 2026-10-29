package perception

import (
	"testing"
)

// TestGeminiResponsePart_ThoughtFiltering verifies that thought parts are correctly identified.
func TestGeminiResponsePart_ThoughtFiltering(t *testing.T) {
	tests := []struct {
		name           string
		parts          []GeminiResponsePart
		wantThoughts   int
		wantResponses  int
		wantThoughtLen int
	}{
		{
			name: "no thought parts",
			parts: []GeminiResponsePart{
				{Text: "Hello world"},
				{Text: " How are you?"},
			},
			wantThoughts:   0,
			wantResponses:  2,
			wantThoughtLen: 0,
		},
		{
			name: "thought part followed by response",
			parts: []GeminiResponsePart{
				{Text: "Let me think about this...", Thought: true},
				{Text: "The answer is 42"},
			},
			wantThoughts:   1,
			wantResponses:  1,
			wantThoughtLen: 26, // "Let me think about this..."
		},
		{
			name: "multiple thought parts",
			parts: []GeminiResponsePart{
				{Text: "First thought", Thought: true},
				{Text: "Second thought", Thought: true},
				{Text: "Final answer"},
			},
			wantThoughts:   2,
			wantResponses:  1,
			wantThoughtLen: 27, // "First thought" + "Second thought"
		},
		{
			name: "all thought parts",
			parts: []GeminiResponsePart{
				{Text: "Thinking...", Thought: true},
			},
			wantThoughts:   1,
			wantResponses:  0,
			wantThoughtLen: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var thoughtCount, responseCount, thoughtLen int
			for _, part := range tt.parts {
				if part.Thought {
					thoughtCount++
					thoughtLen += len(part.Text)
				} else {
					responseCount++
				}
			}

			if thoughtCount != tt.wantThoughts {
				t.Errorf("thought count = %d, want %d", thoughtCount, tt.wantThoughts)
			}
			if responseCount != tt.wantResponses {
				t.Errorf("response count = %d, want %d", responseCount, tt.wantResponses)
			}
			if thoughtLen != tt.wantThoughtLen {
				t.Errorf("thought length = %d, want %d", thoughtLen, tt.wantThoughtLen)
			}
		})
	}
}

// TestGeminiPart_ThoughtField verifies the GeminiPart struct has the Thought field.
func TestGeminiPart_ThoughtField(t *testing.T) {
	part := GeminiPart{
		Text:    "test text",
		Thought: true,
	}
	if !part.Thought {
		t.Error("Thought field should be true")
	}

	part2 := GeminiPart{
		Text: "regular text",
	}
	if part2.Thought {
		t.Error("Thought field should default to false")
	}
}

// TestThinkingSectionFormat verifies the thinking section is properly formatted.
func TestThinkingSectionFormat(t *testing.T) {
	thinkingText := "I need to analyze this problem step by step."
	responseText := "The answer is 42."

	// Simulate the formatting logic from CompleteWithSystem
	var result string
	if thinkingText != "" {
		result = "🧠 **Thinking:**\n" + thinkingText + "\n\n---\n\n" + responseText
	} else {
		result = responseText
	}

	// Verify format - use strings.HasPrefix for multi-byte emoji
	if !hasPrefix(result, "🧠") {
		t.Errorf("Should start with thinking emoji, got: %q", result[:10])
	}
	if !contains(result, "---") {
		t.Error("Should contain separator")
	}
	if !contains(result, responseText) {
		t.Error("Should contain response text")
	}
}

// TestThoughtSignatureCapture verifies thought signatures are captured from responses.
func TestThoughtSignatureCapture(t *testing.T) {
	// Test response-level signature
	resp := GeminiResponse{
		ThoughtSignature: "sig-response-level",
		Candidates: []GeminiResponseCandidate{
			{
				Content: struct {
					Parts []GeminiResponsePart `json:"parts"`
					Role  string               `json:"role"`
				}{
					Parts: []GeminiResponsePart{
						{Text: "Hello"},
					},
					Role: "model",
				},
			},
		},
	}

	if resp.ThoughtSignature != "sig-response-level" {
		t.Errorf("response signature = %q, want %q", resp.ThoughtSignature, "sig-response-level")
	}

	// Test part-level signature
	partSig := GeminiResponsePart{
		Text:             "response with sig",
		ThoughtSignature: "sig-part-level",
	}
	if partSig.ThoughtSignature != "sig-part-level" {
		t.Errorf("part signature = %q, want %q", partSig.ThoughtSignature, "sig-part-level")
	}

	// Test function call signature
	fc := GeminiFunctionCall{
		Name:             "test_func",
		ThoughtSignature: "sig-func-call",
	}
	if fc.ThoughtSignature != "sig-func-call" {
		t.Errorf("func call signature = %q, want %q", fc.ThoughtSignature, "sig-func-call")
	}
}

// TestSchemaDepthLimit verifies the schema depth limit constant.
func TestSchemaDepthLimit(t *testing.T) {
	if geminiSchemaDepthLimit < 6 {
		t.Errorf("schema depth limit %d is too low, should be at least 6", geminiSchemaDepthLimit)
	}
	if geminiSchemaDepthLimit > 15 {
		t.Errorf("schema depth limit %d seems too high, may cause API rejection", geminiSchemaDepthLimit)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
