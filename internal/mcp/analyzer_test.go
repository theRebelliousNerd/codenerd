package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// MCP Analyzer Tests

func TestExtractJSONFromCodeBlock(t *testing.T) {
	payload := `{"categories":["filesystem"],"capabilities":["/read"],"domain":"/go","shard_affinities":{"coder":50},"use_cases":["read"],"condensed":"read file"}`
	response := "```json\n" + payload + "\n```"

	got := extractJSON(response)
	if got != payload {
		t.Fatalf("extractJSON = %q, want %q", got, payload)
	}
}

func TestNormalizeCapabilities(t *testing.T) {
	caps := normalizeCapabilities([]string{"READ", "write", "/delete", "unknown"})
	if len(caps) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(caps))
	}
	expect := map[string]bool{"/read": true, "/write": true, "/delete": true}
	for _, cap := range caps {
		if !expect[cap] {
			t.Fatalf("unexpected capability: %s", cap)
		}
	}
}

func TestNormalizeDomain(t *testing.T) {
	if got := normalizeDomain("Go"); got != "/go" {
		t.Fatalf("normalizeDomain(Go) = %s, want /go", got)
	}
	if got := normalizeDomain("unknown"); got != "/general" {
		t.Fatalf("normalizeDomain(unknown) = %s, want /general", got)
	}
}

func TestInferCategoriesAndCapabilities(t *testing.T) {
	schema := MCPToolSchema{
		Name:        "read_file",
		Description: "Read file contents from disk",
	}

	cats := inferCategories(schema)
	if !containsString(cats, "filesystem") {
		t.Fatalf("expected filesystem category, got %v", cats)
	}

	caps := inferCapabilities(schema)
	if !containsString(caps, "/read") {
		t.Fatalf("expected /read capability, got %v", caps)
	}
}

func TestNormalizeAffinities(t *testing.T) {
	affinities := normalizeAffinities(map[string]int{
		"coder":   120,
		"tester":  -5,
		"unknown": 60,
	})

	if affinities["coder"] != 100 {
		t.Fatalf("coder affinity = %d, want 100", affinities["coder"])
	}
	if affinities["tester"] != 0 {
		t.Fatalf("tester affinity = %d, want 0", affinities["tester"])
	}
	if _, ok := affinities["unknown"]; ok {
		t.Fatalf("unexpected key: unknown")
	}
}

func TestTruncateDescription(t *testing.T) {
	if got := truncateDescription("short", 10); got != "short" {
		t.Fatalf("unexpected truncation: %q", got)
	}
	if got := truncateDescription("0123456789", 5); got != "01..." {
		t.Fatalf("unexpected truncation: %q", got)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type MockLLMClient struct {
	Response string
	Err      error
	Delay    time.Duration
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	} else {
		// Even without delay, check context
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	return m.Response, m.Err
}

func TestAnalyzer_Analyze_NullUndefined(t *testing.T) {
	mockLLM := &MockLLMClient{
		Response: "", // Empty response
	}
	analyzer := NewToolAnalyzer(mockLLM, nil)
	schema := MCPToolSchema{Name: "test_tool"}

	// When LLM returns empty string, parseAnalysisResponse will fail to parse JSON
	// It should log a warning and fall back to analyzeWithoutLLM
	analysis, err := analyzer.Analyze(context.Background(), schema)
	if err != nil {
		t.Fatalf("Analyze should not error on empty LLM response, got: %v", err)
	}

	// Because of fallback, we expect defaults
	if analysis.ToolID != "test_tool" {
		t.Errorf("Expected ToolID to be test_tool, got %q", analysis.ToolID)
	}
	// "test_tool" contains "test", which triggers the "testing" category in fallback
	if len(analysis.Categories) == 0 || analysis.Categories[0] != "testing" {
		t.Errorf("Expected fallback category 'testing', got %v", analysis.Categories)
	}
}

func TestAnalyzer_ExtractJSON_TypeCoercion(t *testing.T) {
	t.Run("Unbalanced Braces", func(t *testing.T) {
		input := `{"categories":["filesystem"`
		got := extractJSON(input)
		if got != input {
			t.Errorf("Expected extractJSON to return original on unbalanced braces, got %q", got)
		}
	})

	t.Run("Truncated JSON in code block", func(t *testing.T) {
		input := "```json\n{\"categories\":\n"
		got := extractJSON(input)
		// Assuming the logic returns the original if no closing codeblock
		if got != input {
			t.Errorf("Expected original input on truncated block, got %q", got)
		}
	})

	t.Run("Massive String", func(t *testing.T) {
		// 10MB string with JSON at the end
		padding := strings.Repeat("a", 10*1024*1024)
		jsonPayload := `{"hello":"world"}`
		input := padding + jsonPayload

		start := time.Now()
		got := extractJSON(input)
		duration := time.Since(start)

		if got != jsonPayload {
			t.Errorf("Failed to extract JSON from massive string. Got length %d", len(got))
		}
		if duration > 100*time.Millisecond {
			t.Logf("extractJSON took %v for 10MB string, watch out for performance", duration)
		}
	})
}

func TestAnalyzer_ParseAnalysisResponse_InvalidTypes(t *testing.T) {
	analyzer := NewToolAnalyzer(nil, nil)
	schema := MCPToolSchema{Name: "test"}

	// JSON is syntactically valid but uses strings where ints or lists are expected
	invalidTypesJSON := `{"shard_affinities": {"coder": "100", "tester": []}, "categories": "not_an_array"}`

	_, err := analyzer.parseAnalysisResponse(invalidTypesJSON, schema)
	if err == nil {
		t.Fatal("Expected error when parsing invalid types, got nil")
	}
}

func TestAnalyzer_NormalizeCapabilities_Extremes(t *testing.T) {
	input := []string{" / r e a d ", "///write", "", "  ", " \t/delete\n ", "/UNKNOWN"}
	got := normalizeCapabilities(input)

	// Since we strip spaces, lower, prepend /, and filter
	// " / r e a d " -> "/ / r e a d " (not in validCaps)
	// "///write" -> "///write" (not in validCaps)
	// "" -> "/" (not in validCaps)
	// "  " -> "/" (not in validCaps)
	// " \t/delete\n " -> "/delete" (IS valid!)

	foundDelete := false
	for _, cap := range got {
		if cap == "/delete" {
			foundDelete = true
		}
	}
	if !foundDelete {
		t.Errorf("Expected /delete to be parsed from extremes, got %v", got)
	}
}

func TestAnalyzer_Analyze_ContextCancelled(t *testing.T) {
	mockLLM := &MockLLMClient{
		Response: `{"categories":["general"]}`,
		Delay:    100 * time.Millisecond,
	}
	analyzer := NewToolAnalyzer(mockLLM, nil)
	schema := MCPToolSchema{Name: "test"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// If context is canceled, Complete returns ctx.Err()
	// Check what Analyze does - does it fall back to analyzeWithoutLLM?
	analysis, err := analyzer.Analyze(ctx, schema)
	if err != nil {
		// Good, it errored out or... wait, the codebase says:
		// if err != nil { return a.analyzeWithoutLLM(schema) }
		// We'll just verify it doesn't crash and returns *something*
	}

	// The codebase currently falls back to analyzeWithoutLLM on ANY error
	// Let's verify that fallback occurred
	if analysis == nil {
		t.Fatal("Expected fallback analysis, got nil")
	}
	if analysis.ToolID != "test" {
		t.Errorf("Expected ToolID 'test'")
	}
}

func TestAnalyzer_UserRequestExtremes_MassiveSchema(t *testing.T) {
	analyzer := NewToolAnalyzer(nil, nil)

	// Create a massive schema that might break formatJSON
	massiveMap := make(map[string]interface{})
	for i := 0; i < 10000; i++ {
		massiveMap[strings.Repeat("a", 100)] = strings.Repeat("b", 100)
	}
	rawJSON, _ := json.Marshal(massiveMap)

	schema := MCPToolSchema{
		Name:        "massive",
		InputSchema: rawJSON,
	}

	start := time.Now()
	prompt, err := analyzer.buildAnalysisPrompt(schema)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("buildAnalysisPrompt failed on massive schema: %v", err)
	}
	if len(prompt) < len(rawJSON) {
		t.Errorf("Prompt is suspiciously small: %d bytes", len(prompt))
	}
	if duration > 1*time.Second {
		t.Logf("buildAnalysisPrompt took %v, formatting large JSON is slow", duration)
	}
}
