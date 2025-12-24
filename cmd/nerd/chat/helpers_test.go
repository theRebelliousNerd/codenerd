package chat

import (
	"codenerd/internal/perception"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePath(t *testing.T) {
	workspace := t.TempDir()
	abs := filepath.Join(workspace, "abs.txt")

	if got := resolvePath(workspace, abs); got != abs {
		t.Fatalf("resolvePath absolute = %q, want %q", got, abs)
	}

	rel := "rel.txt"
	if got := resolvePath(workspace, rel); got != filepath.Join(workspace, rel) {
		t.Fatalf("resolvePath relative = %q, want %q", got, filepath.Join(workspace, rel))
	}
}

func TestReadFileContent_Truncates(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "sample.txt")
	content := "abcdef"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := readFileContent(workspace, "sample.txt", 4)
	if err != nil {
		t.Fatalf("readFileContent: %v", err)
	}
	if got != "abcd" {
		t.Fatalf("readFileContent truncated = %q, want %q", got, "abcd")
	}

	full, err := readFileContent(workspace, "sample.txt", 10)
	if err != nil {
		t.Fatalf("readFileContent full: %v", err)
	}
	if full != content {
		t.Fatalf("readFileContent full = %q, want %q", full, content)
	}
}

func TestCountFileLines(t *testing.T) {
	workspace := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    int64
	}{
		{name: "empty", content: "", want: 0},
		{name: "with newline", content: "a\nb\n", want: 2},
		{name: "without trailing newline", content: "a\nb", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(workspace, tt.name+".txt")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			got, err := countFileLines(workspace, filepath.Base(path))
			if err != nil {
				t.Fatalf("countFileLines: %v", err)
			}
			if got != tt.want {
				t.Fatalf("countFileLines = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSearchInFiles_SkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()
	visible := filepath.Join(root, "visible.txt")
	hiddenDir := filepath.Join(root, ".hidden")
	hidden := filepath.Join(hiddenDir, "hidden.txt")

	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}
	if err := os.WriteFile(visible, []byte("needle"), 0644); err != nil {
		t.Fatalf("write visible: %v", err)
	}
	if err := os.WriteFile(hidden, []byte("needle"), 0644); err != nil {
		t.Fatalf("write hidden: %v", err)
	}

	matches, err := searchInFiles(root, "needle", 10)
	if err != nil {
		t.Fatalf("searchInFiles: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0] != visible {
		t.Fatalf("expected match %q, got %q", visible, matches[0])
	}
}

func TestTruncateForContext(t *testing.T) {
	if got := truncateForContext("line1\nline2\r", 20); got != "line1 line2" {
		t.Fatalf("truncateForContext newline = %q, want %q", got, "line1 line2")
	}

	if got := truncateForContext("abcdef", 4); got != "abcd..." {
		t.Fatalf("truncateForContext truncation = %q, want %q", got, "abcd...")
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "main.go", want: "go"},
		{path: "app.tsx", want: "typescript"},
		{path: "README.md", want: "markdown"},
		{path: "data.unknown", want: "unknown"},
	}

	for _, tt := range tests {
		if got := detectLanguage(tt.path); got != tt.want {
			t.Fatalf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "foo_test.go", want: true},
		{path: "bar.test.ts", want: true},
		{path: "baz.spec.tsx", want: true},
		{path: "WidgetTest.java", want: true},
		{path: "main.go", want: false},
	}

	for _, tt := range tests {
		if got := isTestFile(tt.path); got != tt.want {
			t.Fatalf("isTestFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestBuildFileTopologyFact(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "sample_test.go")
	content := "package main\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	fact := buildFileTopologyFact(path, info)
	if fact.Predicate != "file_topology" {
		t.Fatalf("predicate = %q, want %q", fact.Predicate, "file_topology")
	}
	if len(fact.Args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(fact.Args))
	}

	wantHash := sha256.Sum256([]byte(content))
	if got, ok := fact.Args[1].(string); !ok || got != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("hash arg = %v, want %s", fact.Args[1], hex.EncodeToString(wantHash[:]))
	}

	if got, ok := fact.Args[2].(string); !ok || got != "/go" {
		t.Fatalf("lang arg = %v, want %q", fact.Args[2], "/go")
	}

	if got, ok := fact.Args[4].(string); !ok || got != "/true" {
		t.Fatalf("test flag arg = %v, want %q", fact.Args[4], "/true")
	}

	if got, ok := fact.Args[3].(int64); !ok || got != info.ModTime().Unix() {
		t.Fatalf("mtime arg = %v, want %d", fact.Args[3], info.ModTime().Unix())
	}
}

func TestIsConversationalIntent(t *testing.T) {
	tests := []struct {
		name   string
		intent perception.Intent
		want   bool
	}{
		{
			name:   "always conversational",
			intent: perception.Intent{Verb: "/greet"},
			want:   true,
		},
		{
			name:   "read with empty target",
			intent: perception.Intent{Verb: "/read", Target: ""},
			want:   true,
		},
		{
			name:   "read with none target",
			intent: perception.Intent{Verb: "/read", Target: "none"},
			want:   true,
		},
		{
			name:   "read with file target",
			intent: perception.Intent{Verb: "/read", Target: "main.go"},
			want:   false,
		},
		{
			name:   "non conversational verb",
			intent: perception.Intent{Verb: "/review"},
			want:   false,
		},
		{
			name:   "explain stays routed",
			intent: perception.Intent{Verb: "/explain"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConversationalIntent(tt.intent); got != tt.want {
				t.Fatalf("isConversationalIntent = %v, want %v", got, tt.want)
			}
		})
	}
}
