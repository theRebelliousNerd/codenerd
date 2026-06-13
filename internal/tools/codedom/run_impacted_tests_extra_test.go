package codedom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseStringArray(t *testing.T) {
	if parseStringArray(nil) != nil {
		t.Error("nil -> nil")
	}
	if got := parseStringArray([]string{"a", "b"}); len(got) != 2 || got[0] != "a" {
		t.Errorf("[]string passthrough failed: %v", got)
	}
	// []any with mixed types keeps only the strings.
	got := parseStringArray([]any{"x", 1, "y", true})
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Errorf("[]any filtering failed: %v", got)
	}
	// An unsupported type yields nil.
	if parseStringArray(42) != nil {
		t.Error("unsupported type should yield nil")
	}
}

func TestParseBool(t *testing.T) {
	if parseBool(nil, true) != true {
		t.Error("nil -> default")
	}
	if parseBool(false, true) != false {
		t.Error("explicit false should win over default true")
	}
	if parseBool("notabool", true) != true {
		t.Error("non-bool -> default")
	}
}

func TestParseString(t *testing.T) {
	if parseString(nil, "def") != "def" {
		t.Error("nil -> default")
	}
	if parseString("v", "def") != "v" {
		t.Error("string value should be returned")
	}
	if parseString(123, "def") != "def" {
		t.Error("non-string -> default")
	}
}

// TestRunGoTests drives runGoTests against a real temp Go module, exercising the
// Go toolchain cross-boundary.
func TestRunGoTests(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module cdtmp\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := "package cdtmp\n\nfunc Inc(n int) int { return n + 1 }\n"
	if err := os.WriteFile(filepath.Join(dir, "lib.go"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	test := "package cdtmp\n\nimport \"testing\"\n\nfunc TestInc(t *testing.T) {\n\tif Inc(1) != 2 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "lib_test.go"), []byte(test), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runGoTests(context.Background(), dir, []string{dir}, "60s", true)
	if err != nil {
		t.Fatalf("runGoTests: %v", err)
	}
	if !strings.Contains(out, "go test") {
		t.Errorf("output should echo the command, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "pass") && !strings.Contains(strings.ToLower(out), "ok") {
		t.Errorf("expected a passing run summary, got: %s", out)
	}

	// An invalid timeout string falls back to the default and still runs.
	if _, err := runGoTests(context.Background(), dir, []string{dir}, "notaduration", false); err != nil {
		t.Errorf("runGoTests with bad timeout should fall back, got err: %v", err)
	}
}
