package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverageModulePath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, manifest, want string
	}{
		{"plain", "module example.com/probe\n", "example.com/probe"},
		{"quoted", "module \"example.com/probe\" // comment\n", "example.com/probe"},
		{"parenthesized", "module (\n example.com/probe\n)\n", "example.com/probe"},
		{"quoted_parenthesized", "// module example.com/wrong\nmodule ( // comment\n \"example.com/probe\" // comment\n)\ngo 1.21\n", "example.com/probe"},
		{"future_directive", "module (\n \"example.com/probe\"\n)\nfuture_directive value\n", "example.com/probe"},
		{"missing", "go 1.21\n", ""},
		{"empty_block", "module (\n)\n", ""},
		{"unclosed_block", "module (\n example.com/probe\n", ""},
		{"duplicate", "module example.com/first\nmodule example.com/second\n", ""},
		{"malformed", "module example.com/probe extra\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "go.mod")
			if err := os.WriteFile(path, []byte(tc.manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if got := modulePath(f); got != tc.want {
				t.Errorf("modulePath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCoverSource_ParenthesizedModule(t *testing.T) {
	t.Parallel()
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":        "module (\n \"example.com/probe\"\n)\ngo 1.21\n",
		"a.go":          "package probe\n",
		"nested/go.mod": "module (\n \"example.com/nested\"\n)\ngo 1.21\n",
		"nested/a.go":   "package nested\n",
	})
	r := newCoverSourceResolver(ws, []string{"a.go", "nested/a.go"})
	for profile, want := range map[string]string{
		"example.com/probe/a.go":        "a.go",
		"example.com/nested/a.go":       "nested/a.go",
		"example.com/probe/extra.go":    "",
		"example.com/foreign/a.go":      "",
		"example.com/probe/nested/a.go": "",
	} {
		if got := r.source(profile); got != want {
			t.Errorf("source(%q) = %q, want %q", profile, got, want)
		}
	}
}

const (
	collideBefore = "package collide\n\n// Changed is edited by the turn.\nfunc Changed() int {\n\treturn 1\n}\n"
	collideAfter  = "package collide\n\n// Changed is edited by the turn.\nfunc Changed() int {\n\treturn 2\n}\n"
	collideExtra  = "package collide\n\n// Other is what the test runs.\nfunc Other() int {\n\treturn 1\n}\n"
	collideTest   = "package collide\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) {\n\tif Other() != 1 {\n\t\tt.Fatal(\"other\")\n\t}\n}\n"
)

// The turn edits Changed in the module-root file a.go; the test runs only
// Other, in extra.go, whose executed block spans the same lines. The profile
// names collide/a.go and collide/extra.go, and "collide/extra.go" ends with
// "a.go": the suffix match handed extra.go's executed block to a.go, so
// Changed counted as executed and its witness was met though it never ran
// (audit coverage-source-identity-witness). A block belongs to the file the
// profile names, resolved against the module, and nowhere else.
func TestCoverage_AnotherFilesBlockDoesNotExecuteAChangedElement(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles and tests a throwaway package")
	}
	written := "a.go"
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":        "module collide\n\ngo 1.21\n",
		"a.go":          collideAfter,
		"extra.go":      collideExtra,
		"other_test.go": collideTest,
	})
	v, blocks := verifyTestsWithCoverage(context.Background(), ws, []string{"."}, []string{written})
	if !v.Ran || !v.OK {
		t.Fatalf("coverage run Ran=%v OK=%v output=%q", v.Ran, v.OK, v.Output)
	}
	for _, b := range blocks {
		if strings.HasSuffix(b.File, "extra.go") {
			t.Fatalf("a block of extra.go was admitted for the written a.go: %+v", b)
		}
	}

	result := writeTurnResult()
	result.WrittenPaths = []string{written}
	result.PreWriteContents = map[string]PreImage{written: existed(collideBefore)}
	result.UncoveredBlocks = narrowToChangedLines(ws, result, blocks)

	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws
	e.assertTurnEvidence(testTurn, "/fix", result)
	uncovered := factStrings(t, e, "turn_element_uncovered", 1)
	if !containsString(uncovered, "fn:collide.Changed") {
		t.Fatalf("turn_element_uncovered = %v, want fn:collide.Changed: no test ran it", uncovered)
	}
}

// The resolution itself: a profile path belongs to a written path when it is
// the module path joined to it, and only then; without a module to resolve
// against, only a whole trailing path component counts, and the most
// specific written path wins.
func TestCoverSource(t *testing.T) {
	t.Parallel()
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":        "module example.com/m\n\ngo 1.21\n",
		"a.go":          "package m\n",
		"sub/a.go":      "package sub\n",
		"nested/go.mod": "module example.com/nested\n\ngo 1.21\n",
		"nested/n/x.go": "package n\n",
	})
	written := []string{"a.go", "sub/a.go", "nested/n/x.go"}
	r := newCoverSourceResolver(ws, written)
	for profile, want := range map[string]string{
		"example.com/m/a.go":          "a.go",
		"example.com/m/sub/a.go":      "sub/a.go",
		"example.com/m/extra.go":      "",
		"example.com/m/sub/xa.go":     "",
		"example.com/nested/n/x.go":   "nested/n/x.go",
		"example.com/m/nested/n/x.go": "",
	} {
		if got := r.source(profile); got != want {
			t.Errorf("source(%s) = %q, want %q", profile, got, want)
		}
	}

	// No module to resolve against: whole components only.
	bare := newCoverSourceResolver("", written)
	for profile, want := range map[string]string{
		"mod/a.go":     "a.go",
		"mod/sub/a.go": "sub/a.go",
		"mod/extra.go": "",
	} {
		if got := bare.source(profile); got != want {
			t.Errorf("without a module: source(%s) = %q, want %q", profile, got, want)
		}
	}
}
