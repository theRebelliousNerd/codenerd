package shell

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoerceInt(t *testing.T) {
	cases := []struct {
		in   any
		want int
		ok   bool
	}{
		{nil, 0, false},
		{42, 42, true},
		{int32(7), 7, true},
		{int64(9), 9, true},
		{uint(3), 3, true},
		{uint64(11), 11, true},
		{float32(2.9), 2, true},
		{float64(4.7), 4, true},
		{json.Number("15"), 15, true},
		{json.Number("3.8"), 3, true},
		{"123", 123, true},
		{"6.5", 6, true},
		{"", 0, false},
		{"notanumber", 0, false},
		{[]string{"x"}, 0, false}, // unsupported type
	}
	for _, c := range cases {
		got, ok := coerceInt(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("coerceInt(%v=%T)=(%d,%v), want (%d,%v)", c.in, c.in, got, ok, c.want, c.ok)
		}
	}
}

// writeTrivialGoModule creates a minimal buildable+testable Go module in dir.
func writeTrivialGoModule(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module shelltmp\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "package shelltmp\n\nfunc Add(a, b int) int { return a + b }\n"
	if err := os.WriteFile(filepath.Join(dir, "lib.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	test := "package shelltmp\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "lib_test.go"), []byte(test), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteRunBuild_AutoDetectGo(t *testing.T) {
	dir := t.TempDir()
	writeTrivialGoModule(t, dir)

	out, err := executeRunBuild(context.Background(), map[string]any{"working_dir": dir})
	if err != nil {
		t.Fatalf("executeRunBuild: %v (out=%s)", err, out)
	}
	// A clean build should not report a failure in its summary.
	if strings.Contains(strings.ToLower(out), "build failed") {
		t.Errorf("unexpected build failure in output: %s", out)
	}
}

func TestExecuteRunTests_AutoDetectGo(t *testing.T) {
	dir := t.TempDir()
	writeTrivialGoModule(t, dir)

	out, err := executeRunTests(context.Background(), map[string]any{"working_dir": dir})
	if err != nil {
		t.Fatalf("executeRunTests: %v (out=%s)", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "ok") && !strings.Contains(strings.ToLower(out), "pass") {
		t.Errorf("expected a passing test summary, got: %s", out)
	}
}

func TestExecuteRunBuild_NoDetectableCommand(t *testing.T) {
	// An empty directory has no recognizable build file: auto-detection fails.
	dir := t.TempDir()
	if _, err := executeRunBuild(context.Background(), map[string]any{"working_dir": dir}); err == nil {
		t.Error("expected an error when no build command can be detected")
	}
}
