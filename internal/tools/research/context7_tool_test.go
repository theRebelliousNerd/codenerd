package research

import (
	"context"
	"strings"
	"testing"
)

func TestExecuteContext7_Success_LlmsTxt(t *testing.T) {
	pinLocalGitHubFiles(t, map[string]string{
		"/owner/repo/main/llms.txt":      "- docs/intro.md: Introduction",
		"/owner/repo/main/docs/intro.md": "# Introduction\nHello World. This is a longer document to satisfy the length requirement of the parser. It needs to be at least 50 characters long.",
	})

	tool := Context7Tool()
	res, err := tool.Execute(context.Background(), map[string]any{
		"topic": "test-topic",
		"repo":  "owner/repo",
	})
	if err != nil {
		t.Fatalf("executeContext7 failed: %v", err)
	}

	if !strings.Contains(res, "# Documentation for test-topic") {
		t.Errorf("Expected header, got: %s", res)
	}
	if !strings.Contains(res, "# Introduction") {
		t.Errorf("Expected doc content, got: %s", res)
	}
}

func TestExecuteContext7_InferRepo(t *testing.T) {
	pinLocalGitHubFiles(t, map[string]string{
		"/google/mangle/main/llms.txt":      "- docs/intro.md: Introduction",
		"/google/mangle/main/docs/intro.md": "# Mangle Intro\ncontent that is sufficiently long to pass the 50 character limit imposed by the context7 parser. Otherwise it gets ignored.",
	})

	tool := Context7Tool()
	res, err := tool.Execute(context.Background(), map[string]any{
		"topic": "mangle",
	})
	if err != nil {
		t.Fatalf("executeContext7 failed: %v", err)
	}

	if !strings.Contains(res, "google/mangle") && !strings.Contains(res, "Documentation for mangle") {
		if !strings.Contains(res, "content") {
			t.Errorf("Expected content from inferred repo, got: %s", res)
		}
	}
}

func TestInferRepo_Logic(t *testing.T) {
	tests := []struct {
		topic    string
		expected string
	}{
		{"react", "facebook/react"},
		{"go-rod", "go-rod/rod"},
		{"owner/repo", "owner/repo"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := inferRepo(tt.topic)
		if got != tt.expected {
			t.Errorf("inferRepo(%q) = %q, want %q", tt.topic, got, tt.expected)
		}
	}
}
