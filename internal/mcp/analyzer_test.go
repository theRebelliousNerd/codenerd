package mcp

import "testing"

// TODO: TEST_GAP: Null/Undefined - Verify behavior of Analyze() when LLM returns empty string/nil (should fall back gracefully to analyzeWithoutLLM).
// TODO: TEST_GAP: Type Coercion - Verify extractJSON() behavior with unbalanced braces, truncated JSON strings, or massive 10MB strings.
// TODO: TEST_GAP: Type Coercion - Verify parseAnalysisResponse() with valid JSON structure but incorrect data types (e.g. strings instead of ints for ShardAffinities).
// TODO: TEST_GAP: User Request Extremes - Verify behavior when tools have massive input/output schemas (token limits and json.Indent performance).
// TODO: TEST_GAP: User Request Extremes - Verify normalizeCapabilities with extreme whitespace and duplicate slashes like [" / r e a d ", "///write", ""].
// TODO: TEST_GAP: State Conflicts - Verify behavior of Analyze() when context is canceled (should NOT fall back to analyzeWithoutLLM, but return context error).

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
