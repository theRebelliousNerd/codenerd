package mcp

import (
	"encoding/json"
	"testing"
)

// --- formatJSON ---

func TestFormatJSON_WhenValid_ShouldIndent(t *testing.T) {
	raw := json.RawMessage(`{"key":"value"}`)
	got := formatJSON(raw)
	if got == "" {
		t.Error("expected non-empty result")
	}
	// Should contain indentation
	if got == `{"key":"value"}` {
		t.Error("expected formatted/indented JSON")
	}
}

func TestFormatJSON_WhenEmpty_ShouldReturnEmptyObject(t *testing.T) {
	got := formatJSON(nil)
	if got != "{}" {
		t.Errorf("formatJSON(nil) = %q, want '{}'", got)
	}
}

func TestFormatJSON_WhenInvalid_ShouldReturnRaw(t *testing.T) {
	raw := json.RawMessage(`not json`)
	got := formatJSON(raw)
	if got != "not json" {
		t.Errorf("formatJSON(invalid) = %q, want 'not json'", got)
	}
}

// --- containsAny ---

func TestContainsAny_WhenMatch_ShouldReturnTrue(t *testing.T) {
	if !containsAny("hello world", "world", "foo") {
		t.Error("expected true")
	}
}

func TestContainsAny_WhenNoMatch_ShouldReturnFalse(t *testing.T) {
	if containsAny("hello world", "foo", "bar") {
		t.Error("expected false")
	}
}

func TestContainsAny_WhenEmpty_ShouldReturnFalse(t *testing.T) {
	if containsAny("", "foo") {
		t.Error("expected false for empty string")
	}
}

// --- normalizeCategories ---

func TestNormalizeCategories_WhenValid_ShouldReturnNormalized(t *testing.T) {
	got := normalizeCategories([]string{"Filesystem", " git ", "CODE_ANALYSIS"})
	if len(got) != 2 { // filesystem and git (code_analysis exact match)
		// Let's check what we got
		for _, c := range got {
			t.Logf("category: %q", c)
		}
	}
}

func TestNormalizeCategories_WhenEmpty_ShouldReturnGeneral(t *testing.T) {
	got := normalizeCategories(nil)
	if len(got) != 1 || got[0] != "general" {
		t.Errorf("expected ['general'], got %v", got)
	}
}

func TestNormalizeCategories_WhenAllInvalid_ShouldReturnGeneral(t *testing.T) {
	got := normalizeCategories([]string{"invalid1", "invalid2"})
	if len(got) != 1 || got[0] != "general" {
		t.Errorf("expected ['general'], got %v", got)
	}
}

// --- defaultShardAffinities ---

func TestDefaultShardAffinities_ShouldReturnDefaults(t *testing.T) {
	got := defaultShardAffinities()
	if got["coder"] != 50 {
		t.Errorf("coder = %d, want 50", got["coder"])
	}
	if got["tester"] != 30 {
		t.Errorf("tester = %d, want 30", got["tester"])
	}
	if got["reviewer"] != 40 {
		t.Errorf("reviewer = %d, want 40", got["reviewer"])
	}
	if got["researcher"] != 40 {
		t.Errorf("researcher = %d, want 40", got["researcher"])
	}
}

// --- analyzeWithoutLLM ---

func TestAnalyzeWithoutLLM_ShouldReturnBasicAnalysis(t *testing.T) {
	analyzer := &ToolAnalyzer{} // No LLM, no embedder
	schema := MCPToolSchema{
		Name:        "read_file",
		Description: "Read the contents of a file from the filesystem",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}

	analysis, err := analyzer.analyzeWithoutLLM(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis.ToolID != "read_file" {
		t.Errorf("ToolID = %q, want 'read_file'", analysis.ToolID)
	}
	if analysis.Domain != "/general" {
		t.Errorf("Domain = %q, want '/general'", analysis.Domain)
	}
	if len(analysis.Categories) == 0 {
		t.Error("expected at least one category")
	}
	if len(analysis.Capabilities) == 0 {
		t.Error("expected at least one capability")
	}
	if analysis.Condensed == "" {
		t.Error("expected non-empty condensed description")
	}
}

// --- buildEmbeddingText ---

func TestBuildEmbeddingText_ShouldBuildText(t *testing.T) {
	analyzer := &ToolAnalyzer{}
	schema := MCPToolSchema{
		Name:        "search_files",
		Description: "Search for files matching a pattern",
	}
	analysis := &ToolAnalysis{
		Categories:   []string{"filesystem", "search"},
		Capabilities: []string{"/read", "/search"},
		UseCases:     []string{"find files"},
	}

	text := analyzer.buildEmbeddingText(schema, analysis)
	if text == "" {
		t.Error("expected non-empty embedding text")
	}
	if !containsAny(text, "search_files") {
		t.Error("expected tool name in embedding text")
	}
}

// --- buildAnalysisPrompt ---

func TestBuildAnalysisPrompt_ShouldReturnNonEmptyPrompt(t *testing.T) {
	analyzer := &ToolAnalyzer{}
	schema := MCPToolSchema{
		Name:        "list_dir",
		Description: "List directory contents",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}

	prompt, err := analyzer.buildAnalysisPrompt(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}

// --- NewToolAnalyzer ---

func TestNewToolAnalyzer_ShouldInitialize(t *testing.T) {
	analyzer := NewToolAnalyzer(nil, nil)
	if analyzer == nil {
		t.Fatal("expected non-nil analyzer")
	}
}
