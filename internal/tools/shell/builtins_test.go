package shell

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("alpha.go", "package alpha\n\nfunc Foo() {}\nfunc Bar() {}\n")
	mustWrite("beta.txt", "one\ntwo\nthree\n")
	mustWrite("sub/gamma.go", "package sub\n// TODO: fix Foo\n")
	return dir
}

func TestBuiltin_Ls(t *testing.T) {
	dir := setupTree(t)
	out, handled := runBuiltinFallback(context.Background(), []string{"ls"}, dir, dir)
	if !handled {
		t.Fatal("ls not handled")
	}
	if !strings.Contains(out, "alpha.go") || !strings.Contains(out, "beta.txt") || !strings.Contains(out, "sub/") {
		t.Fatalf("ls output missing entries: %q", out)
	}
}

func TestBuiltin_Cat(t *testing.T) {
	dir := setupTree(t)
	out, handled := runBuiltinFallback(context.Background(), []string{"cat", "beta.txt"}, dir, dir)
	if !handled {
		t.Fatal("cat not handled")
	}
	if out != "one\ntwo\nthree" {
		t.Fatalf("cat output = %q", out)
	}
}

func TestBuiltin_WcLines(t *testing.T) {
	dir := setupTree(t)
	out, handled := runBuiltinFallback(context.Background(), []string{"wc", "-l", "beta.txt"}, dir, dir)
	if !handled {
		t.Fatal("wc not handled")
	}
	if !strings.HasPrefix(out, "3 ") {
		t.Fatalf("wc -l output = %q, want line count 3", out)
	}
}

func TestBuiltin_HeadTail(t *testing.T) {
	dir := setupTree(t)
	head, _ := runBuiltinFallback(context.Background(), []string{"head", "-n", "2", "beta.txt"}, dir, dir)
	if head != "one\ntwo" {
		t.Fatalf("head -n 2 = %q", head)
	}
	tail, _ := runBuiltinFallback(context.Background(), []string{"tail", "-n", "1", "beta.txt"}, dir, dir)
	if tail != "three" {
		t.Fatalf("tail -n 1 = %q", tail)
	}
}

func TestBuiltin_GrepRecursive(t *testing.T) {
	dir := setupTree(t)
	// rg defaults to recursive with line numbers.
	out, handled := runBuiltinFallback(context.Background(), []string{"rg", "Foo"}, dir, dir)
	if !handled {
		t.Fatal("rg not handled")
	}
	if !strings.Contains(out, "alpha.go") || !strings.Contains(out, "gamma.go") {
		t.Fatalf("rg Foo should match both files, got: %q", out)
	}
	if !strings.Contains(out, ":1:") && !strings.Contains(out, ":3:") {
		t.Fatalf("rg output should carry line numbers, got: %q", out)
	}
}

func TestBuiltin_GrepIgnoreCase(t *testing.T) {
	dir := setupTree(t)
	out, _ := runBuiltinFallback(context.Background(), []string{"grep", "-i", "foo", "alpha.go"}, dir, dir)
	if !strings.Contains(out, "Foo") {
		t.Fatalf("grep -i foo should match Foo, got: %q", out)
	}
}

func TestBuiltin_GrepFilesOnly(t *testing.T) {
	dir := setupTree(t)
	out, _ := runBuiltinFallback(context.Background(), []string{"rg", "-l", "TODO"}, dir, dir)
	if !strings.Contains(out, "gamma.go") || strings.Contains(out, ":") {
		t.Fatalf("rg -l should list only the filename, got: %q", out)
	}
}

// A result past the old 2000-match ceiling arrives whole: the working context
// ledger archives large tool results behind recall handles, so grep never
// cuts what the model has not seen.
func TestBuiltin_GrepReturnsEveryMatchWhole(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 2500; i++ {
		fmt.Fprintf(&sb, "needle line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "hay.txt"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out, handled := runBuiltinFallback(context.Background(), []string{"rg", "needle"}, dir, dir)
	if !handled {
		t.Fatal("rg not handled")
	}
	if strings.Contains(out, "truncated") {
		t.Fatalf("grep cut a 2500-match result: %.200s...", out)
	}
	if got := strings.Count(out, "needle line"); got != 2500 {
		t.Fatalf("grep returned %d of 2500 matches", got)
	}
}

// A line past the 4MB scanner ceiling stops that file, and the stop is
// announced: silent would read as "no more matches below".
func TestBuiltin_GrepAnnouncesUnreadableLine(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x", 5<<20) + "\nneedle tail\n"
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := runBuiltinFallback(context.Background(), []string{"grep", "needle", "big.txt"}, dir, dir)
	if !strings.Contains(out, "rest of file skipped") {
		t.Fatalf("grep did not announce the unreadable line: %q", out)
	}
}

func TestIsLikelyPowerShell(t *testing.T) {
	yes := []string{"Get-ChildItem", "Select-String", "get-content", "Measure-Object",
		"Where-Object", "Test-Path", "Resolve-Path.exe", "New-Item", "Remove-Item"}
	for _, c := range yes {
		if !isLikelyPowerShell(c) {
			t.Errorf("expected %q to be detected as PowerShell", c)
		}
	}
	no := []string{"rg", "wc", "ls", "cat", "grep", "go", "git",
		"some-tool", "clang-format", "docker-compose"} // hyphenated non-PS binaries
	for _, c := range no {
		if isLikelyPowerShell(c) {
			t.Errorf("expected %q NOT to be detected as PowerShell", c)
		}
	}
}

func TestBuiltin_Unhandled(t *testing.T) {
	if _, handled := runBuiltinFallback(context.Background(), []string{"somefancytool", "--x"}, "", ""); handled {
		t.Fatal("unknown command must fall through (handled=false)")
	}
}

func TestBuiltin_ExeSuffixAndEcho(t *testing.T) {
	out, handled := runBuiltinFallback(context.Background(), []string{"echo.exe", "hello", "world"}, "", "")
	if !handled || out != "hello world" {
		t.Fatalf("echo.exe = %q handled=%v", out, handled)
	}
}
