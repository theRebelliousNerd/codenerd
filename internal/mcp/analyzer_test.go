package mcp

import "testing"

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
