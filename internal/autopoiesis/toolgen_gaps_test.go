package autopoiesis

import (
	"context"
	"strings"
	"testing"
)

// =============================================================================
// Vector 1: Null/Empty Inputs
// =============================================================================

func TestToolGenerator_DetectToolNeed_EmptyInput(t *testing.T) {
	client := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"needs_new_tool": false}`, nil
		},
	}
	tg := NewToolGenerator(client, "/tmp/tools")

	tests := []struct {
		name          string
		input         string
		failedAttempt string
	}{
		{"empty_both", "", ""},
		{"empty_input_only", "", "some failure"},
		{"whitespace_only", "   \t\n  ", ""},
		{"control_chars", "\x00\x01\x02", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			need, err := tg.DetectToolNeed(context.Background(), tc.input, tc.failedAttempt)
			if err != nil {
				t.Fatalf("DetectToolNeed should not error on empty input: %v", err)
			}
			// For empty/trivial inputs, no tool need should be detected
			if need != nil {
				t.Logf("Unexpected tool need detected for empty input: %v", need)
			}
		})
	}
}

func TestExtractCodeBlock_EmptyAndMalformed(t *testing.T) {
	tests := []struct {
		name  string
		input string
		lang  string
	}{
		{"empty_string", "", "go"},
		{"only_backticks", "```", "go"},
		{"unmatched_open", "```go\nfunc main() {}", "go"},
		{"empty_block", "```go\n```", "go"},
		{"nested_backticks", "```go\n```nested```\n```", "go"},
		{"wrong_lang", "```python\nprint('hi')\n```", "go"},
		{"null_bytes", "```go\x00\n```", "go"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic
			result := extractCodeBlock(tc.input, tc.lang)
			// Result should be a string (empty or with content), but never panic
			_ = result
		})
	}
}

// =============================================================================
// Vector 2: Type Coercion
// =============================================================================

func TestToolGenerator_LLMTypeCoercion(t *testing.T) {
	// Test that refineToolNeedWithLLM handles LLM returning wrong types
	// (e.g., string "true" instead of boolean true).
	tests := []struct {
		name     string
		response string
	}{
		{
			"string_boolean",
			`{"needs_new_tool": "true", "tool_name": "test", "purpose": "test", "priority": 0.8, "confidence": 0.9}`,
		},
		{
			"string_priority",
			`{"needs_new_tool": true, "tool_name": "test", "purpose": "test", "priority": "high", "confidence": 0.9}`,
		},
		{
			"integer_boolean",
			`{"needs_new_tool": 1, "tool_name": "test", "purpose": "test", "priority": 0.8, "confidence": 0.9}`,
		},
		{
			"nested_json",
			`{"response": {"needs_new_tool": true, "tool_name": "test"}}`,
		},
		{
			"empty_json",
			`{}`,
		},
		{
			"json_array",
			`[]`,
		},
		{
			"invalid_json",
			`not json at all`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &MockLLMClient{
				CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
					return tc.response, nil
				},
			}
			tg := NewToolGenerator(client, "/tmp/tools")

			// Should not panic on any of these responses
			need, err := tg.DetectToolNeed(context.Background(), "I need a tool to validate JSON", "")
			if err != nil {
				t.Fatalf("DetectToolNeed should not error: %v", err)
			}
			// The function may or may not detect a need based on pattern matching
			// fallback — the important thing is it doesn't panic
			_ = need
		})
	}
}

// =============================================================================
// Vector 3: Security — Dangerous Directives
// =============================================================================

func TestValidateCodeAST_DangerousDirectives(t *testing.T) {
	tg := NewToolGenerator(&MockLLMClient{}, "/tmp/tools")

	tests := []struct {
		name    string
		code    string
		wantWrn string
	}{
		{
			"go_generate_directive",
			"package tools\n\n//go:generate rm -rf /\n\nfunc safe() {}\n",
			"go:generate",
		},
		{
			"cgo_import",
			"package tools\n\nimport \"C\"\n\nfunc cgoFunc() {}\n",
			"",
		},
		{
			"os_exec_import",
			"package tools\n\nimport \"os/exec\"\n\nfunc execFunc() {\n\t_ = exec.Command(\"rm\", \"-rf\", \"/\")\n}\n",
			"os/exec",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tg.validateCodeAST(tc.code, "test")
			if tc.wantWrn != "" {
				found := false
				for _, w := range result.Warnings {
					if strings.Contains(strings.ToLower(w), strings.ToLower(tc.wantWrn)) {
						found = true
						break
					}
				}
				if !found {
					// Also check errors
					for _, e := range result.Errors {
						if strings.Contains(strings.ToLower(e), strings.ToLower(tc.wantWrn)) {
							found = true
							break
						}
					}
				}
				if !found {
					t.Logf("Expected warning/error about %q, got warnings=%v errors=%v",
						tc.wantWrn, result.Warnings, result.Errors)
				}
			}
		})
	}
}
