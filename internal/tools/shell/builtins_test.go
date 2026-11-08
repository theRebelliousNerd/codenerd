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

func TestSplitFlags_PinParsing(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantFlags map[rune]bool
		wantVals  map[string]string
		wantOps   []string
	}{
		{name: "combined shorts", args: []string{"-ln", "beta.txt"}, wantFlags: map[rune]bool{'l': true, 'n': true}, wantVals: map[string]string{}, wantOps: []string{"beta.txt"}},
		{name: "n consumes next", args: []string{"-n", "2", "beta.txt"}, wantFlags: map[rune]bool{}, wantVals: map[string]string{"n": "2"}, wantOps: []string{"beta.txt"}},
		{name: "n attached", args: []string{"-n5", "beta.txt"}, wantFlags: map[rune]bool{}, wantVals: map[string]string{"n": "5"}, wantOps: []string{"beta.txt"}},
		{name: "dash dash ends flags", args: []string{"-l", "--", "-n"}, wantFlags: map[rune]bool{'l': true}, wantVals: map[string]string{}, wantOps: []string{"-n"}},
		{name: "lone dash is operand", args: []string{"-"}, wantFlags: map[rune]bool{}, wantVals: map[string]string{}, wantOps: []string{"-"}},
		{name: "operand stops nothing but recorded", args: []string{"beta.txt", "-l"}, wantFlags: map[rune]bool{'l': true}, wantVals: map[string]string{}, wantOps: []string{"beta.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags, vals, ops := splitFlags(tc.args)
			for r := range tc.wantFlags {
				if !flags[r] {
					t.Fatalf("splitFlags(%q) missing flag %q, got %v", tc.args, string(r), flags)
				}
			}
			if len(flags) != len(tc.wantFlags) {
				t.Fatalf("splitFlags(%q) flags = %v, want %v", tc.args, flags, tc.wantFlags)
			}
			if len(vals) != len(tc.wantVals) {
				t.Fatalf("splitFlags(%q) vals = %v, want %v", tc.args, vals, tc.wantVals)
			}
			for k, v := range tc.wantVals {
				if vals[k] != v {
					t.Fatalf("splitFlags(%q) vals[%q] = %q, want %q", tc.args, k, vals[k], v)
				}
			}
			if len(ops) != len(tc.wantOps) {
				t.Fatalf("splitFlags(%q) operands = %v, want %v", tc.args, ops, tc.wantOps)
			}
			for i := range tc.wantOps {
				if ops[i] != tc.wantOps[i] {
					t.Fatalf("splitFlags(%q) operands = %v, want %v", tc.args, ops, tc.wantOps)
				}
			}
		})
	}
}

func TestBuiltinLs_HiddenMultiAndMissing(t *testing.T) {
	dir := setupTree(t)
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	scope := builtinScope{ctx: ctx, root: dir, dir: dir}
	plain := builtinLs(nil, scope)
	if strings.Contains(plain, ".hidden") {
		t.Fatalf("ls without -a listed dotfile: %q", plain)
	}
	withAll := builtinLs([]string{"-a"}, scope)
	if !strings.Contains(withAll, ".hidden") {
		t.Fatalf("ls -a hid dotfile: %q", withAll)
	}
	multi := builtinLs([]string{"alpha.go", "beta.txt"}, scope)
	if !strings.Contains(multi, "alpha.go") || !strings.Contains(multi, "beta.txt") {
		t.Fatalf("ls multi-operand lost entries: %q", multi)
	}
	missing := builtinLs([]string{"nope.txt"}, scope)
	if !strings.Contains(missing, "cannot access") {
		t.Fatalf("ls missing file should report access error, got: %q", missing)
	}
}

func TestBuiltinPwd_ReturnsAbsolute(t *testing.T) {
	dir := setupTree(t)
	got := builtinPwd(dir)
	if !filepath.IsAbs(got) {
		t.Fatalf("builtinPwd(%q) = %q, want absolute path", dir, got)
	}
}
