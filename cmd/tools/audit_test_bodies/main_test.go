package main

import (
	"errors"
	"github.com/google/go-cmp/cmp"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogOnlyNegativeControl(t *testing.T) {
	for _, tc := range []struct {
		body  string
		inert bool
	}{{`t.Log("cancellation tested")`, true}, {``, true}, {`cancel(); if err != context.Canceled { t.Fatal(err) }`, false}} {
		file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nfunc TestProbe(t *testing.T){"+tc.body+"}", 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := inert(file.Decls[0].(*ast.FuncDecl).Body); got != tc.inert {
			t.Fatalf("inert(%q)=%v", tc.body, got)
		}
	}
}

func TestInertClassifiesStatements(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		declaration string
		want        bool
	}{
		{name: "absent body", declaration: `func TestProbe(t *testing.T)`, want: true},
		{name: "empty body", declaration: `func TestProbe(t *testing.T) {}`, want: true},
		{name: "formatted logging", declaration: `func TestProbe(t *testing.T) { t.Logf("value: %d", 1) }`, want: true},
		{name: "multiple logging calls", declaration: `func TestProbe(t *testing.T) { t.Log("first"); t.Logf("second: %d", 2) }`, want: true},
		{name: "empty statement only", declaration: `func TestProbe(t *testing.T) { ; }`, want: true},
		{name: "stray semicolons with logging", declaration: `func TestProbe(t *testing.T) { ;; t.Log("only logging");; }`, want: true},
		{name: "skip", declaration: `func TestProbe(t *testing.T) { t.Skip("not applicable") }`, want: true},
		{name: "formatted skip", declaration: `func TestProbe(t *testing.T) { t.Skipf("skip %d", 1) }`, want: true},
		{name: "skip now", declaration: `func TestProbe(t *testing.T) { t.SkipNow() }`, want: true},
		{name: "helper mark", declaration: `func TestProbe(t *testing.T) { t.Helper() }`, want: true},
		{name: "mixed logging and skip", declaration: `func TestProbe(t *testing.T) { t.Log("note"); t.Skip("not applicable") }`, want: true},
		{name: "skip before behavior", declaration: `func TestProbe(t *testing.T) { t.Skip("start"); verify() }`},
		{name: "helper before behavior", declaration: `func TestProbe(t *testing.T) { t.Helper(); verify() }`},
		{name: "assignment", declaration: `func TestProbe(t *testing.T) { value := 1 }`},
		{name: "noncall expression", declaration: `func TestProbe(t *testing.T) { value }`},
		{name: "unqualified call", declaration: `func TestProbe(t *testing.T) { verify() }`},
		{name: "assertion", declaration: `func TestProbe(t *testing.T) { t.Fatal("failure") }`},
		{name: "logging before behavior", declaration: `func TestProbe(t *testing.T) { t.Log("start"); verify() }`},
		{name: "behavior before logging", declaration: `func TestProbe(t *testing.T) { verify(); t.Log("done") }`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\n"+tt.declaration, 0)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if got := inert(file.Decls[0].(*ast.FuncDecl).Body); got != tt.want {
				t.Errorf("inert() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuditReportsInertTests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixtures := map[string]string{
		"a_test.go":        "package fixture\nfunc TestEmpty(t *testing.T) {}\nfunc TestLogging(t *testing.T) { t.Log(\"only logging\"); t.Logf(\"value %d\", 1) }\nfunc TestActive(t *testing.T) { t.Fatal(\"assertion\") }\n",
		"nested/b_test.go": "package fixture\nfunc TestNested(t *testing.T) {}\n",
	}
	for name, source := range fixtures {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	got, err := audit(root)
	if err != nil {
		t.Fatalf("audit(): %v", err)
	}
	want := []string{
		filepath.Join(root, "a_test.go") + ":2:1: TestEmpty has no executable behavior beyond logging",
		filepath.Join(root, "a_test.go") + ":3:1: TestLogging has no executable behavior beyond logging",
		filepath.Join(root, "nested", "b_test.go") + ":2:1: TestNested has no executable behavior beyond logging",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("audit() mismatch (-want +got):\n%s", diff)
	}
}

func TestAuditIgnoresExcludedFilesAndDeclarations(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		files map[string]string
	}{
		{name: "empty directory"},
		{name: "non-test files", files: map[string]string{"main.go": "not valid Go", "notes.txt": "not valid Go"}},
		{name: "excluded directories", files: map[string]string{
			".hidden/broken_test.go":         "not valid Go",
			"vendor/broken_test.go":          "not valid Go",
			"testdata/broken_test.go":        "not valid Go",
			"nested/.hidden/broken_test.go":  "not valid Go",
			"nested/vendor/broken_test.go":   "not valid Go",
			"nested/testdata/broken_test.go": "not valid Go",
		}},
		{name: "non-test declarations and active tests", files: map[string]string{
			"valid_test.go": "package fixture\nvar value = 1\nfunc helper(t *testing.T) {}\nfunc TestNoParameters() {}\nfunc TestTwoParameters(t *testing.T, extra int) {}\nfunc TestActive(t *testing.T) { if value != 1 { t.Fatal(value) } }\n",
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for name, source := range tt.files {
				path := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatalf("create fixture directory: %v", err)
				}
				if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
			}
			failures, err := audit(root)
			if err != nil {
				t.Fatalf("audit(): %v", err)
			}
			if len(failures) != 0 {
				t.Errorf("audit() failures = %v, want none", failures)
			}
		})
	}
}

func TestAuditSkipsTestMainValueReceiverDotfileAndSorts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixtures := map[string]string{
		"z_test.go":    "package fixture\nfunc TestZoo(t *testing.T) {}\n",
		"a_test.go":    "package fixture\nfunc TestApple(t *testing.T) { t.Skip(\"not applicable\") }\n",
		"main_test.go": "package fixture\nfunc TestMain(m *testing.M) {}\nfunc TestValueParam(t testing.T) {}\nfunc TestTwoValues(t testing.T, extra int) {}\n",
		".dot_test.go": "package fixture\nfunc TestDot(t *testing.T) {}\n",
	}
	for name, source := range fixtures {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	got, err := audit(root)
	if err != nil {
		t.Fatalf("audit(): %v", err)
	}
	want := []string{
		filepath.Join(root, "a_test.go") + ":2:1: TestApple has no executable behavior beyond logging",
		filepath.Join(root, "z_test.go") + ":2:1: TestZoo has no executable behavior beyond logging",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("audit() mismatch (-want +got):\n%s", diff)
	}
}

func TestAuditReturnsTraversalAndParseErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		missing bool
	}{
		{name: "missing root", missing: true},
		{name: "malformed test file"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if tt.missing {
				root = filepath.Join(root, "missing")
			} else if err := os.WriteFile(filepath.Join(root, "broken_test.go"), []byte("package fixture\nfunc TestBroken("), 0o600); err != nil {
				t.Fatalf("write malformed fixture: %v", err)
			}
			failures, err := audit(root)
			if err == nil {
				t.Fatal("audit() returned no error")
			}
			if tt.missing {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("audit() error = %v, want fs.ErrNotExist", err)
				}
			} else {
				var parseErrors scanner.ErrorList
				if !errors.As(err, &parseErrors) {
					t.Errorf("audit() error type = %T, want scanner.ErrorList", err)
				}
			}
			if len(failures) != 0 {
				t.Errorf("audit() failures = %v, want none", failures)
			}
		})
	}
}

func TestAuditIgnoresNonStandardTestSignatures(t *testing.T) {
	dir := t.TempDir()
	src := "package foo\nimport \"testing\"\nfunc TestNoParams() { t.Log(\"hi\") }\nfunc TestTwoParams(a int, b int) {}\nfunc TestStandardLogOnly(t *testing.T) { t.Log(\"hi\") }\n"
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(src), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	failures, err := audit(dir)
	if err != nil {
		t.Fatalf("audit returned error: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure for standard log-only test, got %d: %v", len(failures), failures)
	}
	if !strings.Contains(failures[0], "TestStandardLogOnly") {
		t.Fatalf("expected failure to name TestStandardLogOnly, got %v", failures)
	}
	for _, f := range failures {
		if strings.Contains(f, "TestNoParams") || strings.Contains(f, "TestTwoParams") {
			t.Fatalf("non-standard signature was flagged but must be ignored: %v", failures)
		}
	}
}
