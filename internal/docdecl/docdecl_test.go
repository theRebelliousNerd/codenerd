package docdecl

import (
	"errors"
	"github.com/google/go-cmp/cmp"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestMangleDeclsIgnoreComments(t *testing.T) {
	src := "# Decl ghost(X).\n/*\nDecl ghost3(X).\n*/\n/* Decl ghost2(X). */\nDecl keep_me(X).\nDecl elsewhere(X) bound [/string].\n"
	got := MangleDecls(src)
	if !got["keep_me"] || !got["elsewhere"] {
		t.Fatalf("decls %#v", got)
	}
	for _, bad := range []string{"ghost", "ghost2", "ghost3", "Declare"} {
		if got[bad] {
			t.Fatalf("counted %s in %#v", bad, got)
		}
	}
	if MangleDecls("Declare something(X).\n")["Declare"] {
		t.Fatal("Declare is not Decl")
	}
}

const sample = `package sample

// CommentOnly is only named in this comment.
const note = "StringOnly"

type Engine struct{ Cache int }

func (e *Engine) Clear() {}

func Good() {}
`

func TestPackageDeclaresOnlyParsedNames(t *testing.T) {
	p := NewPackage("internal/sample")
	p.AddFile(token.NewFileSet(), "sample.go", "internal/sample/sample.go", []byte(sample))
	for _, want := range []string{"Good", "Engine", "Engine.Cache", "(*Engine).Clear", "sample.Good"} {
		if !p.Has(want) {
			t.Errorf("Has(%q) = false", want)
		}
	}
	for _, bad := range []string{"CommentOnly", "StringOnly", "(*Other).Clear"} {
		if p.Has(bad) {
			t.Errorf("Has(%q) = true: comment/string text is not a declaration", bad)
		}
	}
	sites := p.Lookup("(*Engine).Clear")
	if len(sites) == 0 || sites[0].Kind != KindMethod || sites[0].Line != 8 {
		t.Fatalf("Lookup(Clear) = %#v", sites)
	}
}

func TestTestIdentityNeedsTestFileAndSignature(t *testing.T) {
	src := `package sample

import "testing"

func TestReal(t *testing.T) {}
func TestWrongSig(x int) {}
func Testlower(t *testing.T) {}
func BenchmarkB(b *testing.B) {}
func ExampleThing() {}
func (s S) TestMethod(t *testing.T) {}
`
	p := NewPackage("x")
	p.AddFile(token.NewFileSet(), "a_test.go", "x/a_test.go", []byte(src))
	want := map[string]Kind{
		"TestReal": KindTest, "TestWrongSig": KindFunc, "Testlower": KindFunc,
		"BenchmarkB": KindTest, "ExampleThing": KindTest, "TestMethod": KindMethod,
	}
	for name, kind := range want {
		sites := p.Lookup(name)
		if len(sites) == 0 || sites[0].Kind != kind {
			t.Errorf("%s: sites %#v, want kind %s", name, sites, kind)
		}
	}
	// The same signature in a non-test file is not a test identity.
	q := NewPackage("x")
	q.AddFile(token.NewFileSet(), "b.go", "x/b.go", []byte("package sample\n\nimport \"testing\"\n\nfunc TestInProd(t *testing.T) {}\n"))
	if sites := q.Lookup("TestInProd"); len(sites) != 1 || sites[0].Kind != KindFunc {
		t.Fatalf("TestInProd in b.go: %#v", sites)
	}
}

func TestBuildConstraintIsRecorded(t *testing.T) {
	p := NewPackage("x")
	p.AddFile(token.NewFileSet(), "a.go", "x/a.go", []byte("//go:build linux\n\npackage x\n\nfunc OnlyLinux() {}\n"))
	p.AddFile(token.NewFileSet(), "b.go", "x/b.go", []byte("package x\n\nfunc Always() {}\n"))
	if s := p.Lookup("OnlyLinux"); len(s) != 1 || !s[0].Constrained {
		t.Fatalf("OnlyLinux: %#v", s)
	}
	if s := p.Lookup("Always"); len(s) != 1 || s[0].Constrained {
		t.Fatalf("Always: %#v", s)
	}
}

func TestParseErrorKeepsRecoveredDecls(t *testing.T) {
	p := NewPackage("x")
	p.AddFile(token.NewFileSet(), "a.go", "x/a.go", []byte("package x\n\nfunc Kept() {}\n\nfunc Broken( {\n"))
	if _, ok := p.ParseErr["x/a.go"]; !ok {
		t.Fatal("parse error not recorded")
	}
}

func TestLoadPredicates_FiltersCorpus(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		rootName string
		empty    bool
	}{
		{name: "ordinary root", rootName: "corpus"},
		{name: "hidden root is still scanned", rootName: ".corpus"},
		{name: "excluded directory name as explicit root", rootName: "testdata"},
		{name: "empty corpus", rootName: "empty", empty: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), tt.rootName)
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatalf("create corpus root: %v", err)
			}
			want := map[string]bool{}
			if !tt.empty {
				files := map[string]string{
					"main.mg":                     "Decl shared(X).\nDecl top_level(X).\n# Decl comment_only(X).\n",
					"nested/predicates.MG":        "Decl shared(X).\nDecl nested_predicate(X).\n",
					"notes.txt":                   "Decl wrong_extension(X).\n",
					".hidden/debug.mg":            "Decl hidden_only(X).\n",
					"testdata/fixture.mg":         "Decl fixture_only(X).\n",
					"vendor/dependency.mg":        "Decl vendor_only(X).\n",
					"node_modules/dependency.mg":  "Decl node_only(X).\n",
					"nested/.hidden/debug.mg":     "Decl nested_hidden_only(X).\n",
					"nested/testdata/fixture.mg":  "Decl nested_fixture_only(X).\n",
					"nested/vendor/dependency.mg": "Decl nested_vendor_only(X).\n",
					"nested/node_modules/dep.mg":  "Decl nested_node_only(X).\n",
				}
				for rel, src := range files {
					path := filepath.Join(root, filepath.FromSlash(rel))
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatalf("create fixture directory %q: %v", rel, err)
					}
					if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
						t.Fatalf("write fixture %q: %v", rel, err)
					}
				}
				want = map[string]bool{"shared": true, "top_level": true, "nested_predicate": true}
			}
			got, err := LoadPredicates(root)
			if err != nil {
				t.Fatalf("LoadPredicates(%q): %v", root, err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("LoadPredicates() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadPredicates_MissingRootReturnsNoPredicates(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "missing")
	got, err := LoadPredicates(root)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadPredicates() error = %v, want fs.ErrNotExist", err)
	}
	if got != nil {
		t.Errorf("LoadPredicates() = %#v, want nil on failure", got)
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("LoadPredicates() error = %T, want *fs.PathError", err)
	}
	if pathErr.Path != root {
		t.Errorf("error path = %q, want %q", pathErr.Path, root)
	}
}
