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
	// "mangle" is the engine codeNERD pins (go.mod), which lives on Codeberg:
	// the read goes to Codeberg, never to a GitHub repository of another project.
	pinLocalCodebergFiles(t, map[string]string{
		"/TauCeti/mangle-go/main/llms.txt":      "- docs/intro.md: Introduction",
		"/TauCeti/mangle-go/main/docs/intro.md": "# Mangle Intro\ncontent that is sufficiently long to pass the 50 character limit imposed by the context7 parser. Otherwise it gets ignored.",
	})

	tool := Context7Tool()
	res, err := tool.Execute(context.Background(), map[string]any{
		"topic": "mangle",
	})
	if err != nil {
		t.Fatalf("executeContext7 failed: %v", err)
	}
	if !strings.Contains(res, "# Mangle Intro") {
		t.Errorf("Expected the Codeberg document from the inferred repo, got: %s", res)
	}
}

// The inferred repository for "mangle" is the module go.mod pins, not the
// upstream project the atoms used to cite.
func TestInferRepo_MangleIsThePinnedModule(t *testing.T) {
	for _, topic := range []string{"mangle", "Mangle", "mangle-go"} {
		got := inferRepo(topic)
		if got != "codeberg.org/TauCeti/mangle-go" {
			t.Errorf("inferRepo(%q) = %q, want codeberg.org/TauCeti/mangle-go", topic, got)
		}
		if strings.Contains(got, "google") {
			t.Errorf("inferRepo(%q) = %q names the upstream project", topic, got)
		}
	}
	ref, ok := parseRepoRef(inferRepo("mangle"))
	if !ok || ref.host != hostCodeberg || ref.owner != "TauCeti" || ref.name != "mangle-go" {
		t.Errorf("the inferred mangle repo does not parse as a Codeberg repo: %+v ok=%v", ref, ok)
	}
}

func TestParseRepoRef(t *testing.T) {
	tests := []struct {
		in   string
		want repoRef
		ok   bool
	}{
		{"facebook/react", githubRepo("facebook", "react"), true},
		{"github.com/spf13/cobra", githubRepo("spf13", "cobra"), true},
		{"codeberg.org/TauCeti/mangle-go", repoRef{host: hostCodeberg, owner: "TauCeti", name: "mangle-go"}, true},
		{"example.com/o/r", repoRef{}, false},
		{"single", repoRef{}, false},
		{"a/b/c/d", repoRef{}, false},
		{"/r", repoRef{}, false},
	}
	for _, tt := range tests {
		got, ok := parseRepoRef(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("parseRepoRef(%q) = %+v, %v; want %+v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRawFileURL_UsesTheOwningHost(t *testing.T) {
	cb := rawFileURL(repoRef{host: hostCodeberg, owner: "TauCeti", name: "mangle-go"}, "main", "/README.md")
	if cb != "https://codeberg.org/TauCeti/mangle-go/raw/branch/main/README.md" {
		t.Errorf("codeberg raw URL = %q", cb)
	}
	gh := rawFileURL(githubRepo("facebook", "react"), "main", "README.md")
	if gh != "https://raw.githubusercontent.com/facebook/react/main/README.md" {
		t.Errorf("github raw URL = %q", gh)
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
		{"github.com/charmbracelet/bubbletea", "charmbracelet/bubbletea"},
		{"github.com/spf13/cobra/doc", "spf13/cobra"},
		{"github.com/go-rod", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := inferRepo(tt.topic)
		if got != tt.expected {
			t.Errorf("inferRepo(%q) = %q, want %q", tt.topic, got, tt.expected)
		}
	}
}
