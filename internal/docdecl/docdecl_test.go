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

func TestPackageStabilizeNilGuardsAndLookupCopy(t *testing.T) {
	var nilPkg *Package
	nilPkg.AddFile(nil, "a.go", "x/a.go", []byte("package x\nfunc A() {}\n"))
	if nilPkg.Has("A") {
		t.Errorf("Has on nil *Package = true, want false")
	}
	if got := nilPkg.Lookup("A"); got != nil {
		t.Errorf("Lookup on nil *Package = %#v, want nil", got)
	}
	if nilPkg.Has("") {
		t.Errorf("Has(\"\") on nil *Package = true, want false")
	}

	var zero Package
	zero.AddFile(nil, "a.go", "x/a.go", []byte("package x\nfunc Zeroed() {}\n"))
	if zero.Names == nil || zero.PkgNames == nil || zero.ParseErr == nil {
		t.Fatalf("AddFile on zero Package did not initialise maps: %#v", &zero)
	}
	if !zero.Has("Zeroed") {
		t.Errorf("Has(Zeroed) = false after AddFile with nil FileSet on zero Package")
	}
	if !zero.Has("x.Zeroed") {
		t.Errorf("Has(x.Zeroed) = false, want PkgNames-qualified match")
	}
	if zero.Has("other.Zeroed") {
		t.Errorf("Has(other.Zeroed) = true, want false")
	}
	if got := zero.Lookup("Missing"); got != nil {
		t.Errorf("Lookup(Missing) = %#v, want nil", got)
	}
	if got := zero.Lookup(""); got != nil {
		t.Errorf("Lookup(\"\") = %#v, want nil", got)
	}

	ghost := NewPackage("x")
	ghost.Names["Ghost"] = true
	if !ghost.Has("Ghost") {
		t.Fatalf("Has(Ghost) = false for manually recorded name")
	}
	if got := ghost.Lookup("Ghost"); got != nil {
		t.Errorf("Lookup(Ghost) = %#v, want nil when no site was recorded", got)
	}

	p := NewPackage("internal/sample")
	p.AddFile(token.NewFileSet(), "sample.go", "internal/sample/sample.go", []byte(sample))
	sites := p.Lookup("Good")
	if len(sites) != 1 {
		t.Fatalf("Lookup(Good) = %#v, want one site", sites)
	}
	sites[0].Name = "MUTATED"
	again := p.Lookup("Good")
	if len(again) != 1 || again[0].Name != "Good" {
		t.Fatalf("Lookup copy is aliased: second Lookup(Good) = %#v", again)
	}
}

func TestPackageStabilizeCitationSpellings(t *testing.T) {
	p := NewPackage("internal/sample")
	p.AddFile(token.NewFileSet(), "sample.go", "internal/sample/sample.go", []byte(sample))
	for _, want := range []string{"Engine.Clear", "(Engine).Clear", "(*Engine).Clear", "Clear"} {
		if !p.Has(want) {
			t.Errorf("Has(%q) = false, want receiver spelling to resolve", want)
		}
		if sites := p.Lookup(want); len(sites) == 0 || sites[0].Kind != KindMethod {
			t.Errorf("Lookup(%q) = %#v, want method sites", want, sites)
		}
	}
	for _, bad := range []string{"", "Missing", "Other.Clear", "(Other).Clear", "(*Other).Clear"} {
		if p.Has(bad) {
			t.Errorf("Has(%q) = true, want false", bad)
		}
		if got := p.Lookup(bad); got != nil {
			t.Errorf("Lookup(%q) = %#v, want nil", bad, got)
		}
	}

	dir := NewPackage("internal/sample")
	dir.AddFile(token.NewFileSet(), "a.go", "internal/sample/a.go", []byte("package other\nfunc Good() {}\n"))
	if !dir.Has("sample.Good") {
		t.Errorf("Has(sample.Good) = false, want DirBase fallback when package clause is other")
	}
	if !dir.Has("other.Good") {
		t.Errorf("Has(other.Good) = false, want PkgNames match")
	}
	if dir.Has("bogus.Good") {
		t.Errorf("Has(bogus.Good) = true, want false")
	}

	blank := NewPackage("x")
	blank.AddFile(token.NewFileSet(), "a.go", "x/a.go", []byte("package x\nvar _ = 1\nfunc _() {}\ntype _ struct{}\nfunc Real() {}\n"))
	if blank.Has("_") {
		t.Errorf("Has(_) = true, want blank identifier ignored")
	}
	if !blank.Has("Real") {
		t.Errorf("Has(Real) = false, want true alongside blank identifiers")
	}

	broken := NewPackage("x")
	broken.AddFile(token.NewFileSet(), "a.go", "x/a.go", []byte("package x\n\nfunc Kept() {}\n\nfunc Broken( {\n"))
	if _, ok := broken.ParseErr["x/a.go"]; !ok {
		t.Fatalf("parse error not recorded for broken file")
	}
	if !broken.Has("Kept") {
		t.Errorf("Has(Kept) = false, want recovered declaration to survive parse error")
	}
}

func TestAddFilePreservesExistingState(t *testing.T) {
	p := &Package{}
	p.AddFile(nil, "a.go", "a.go", []byte("package alpha\nfunc First() {}\n"))
	p.AddFile(nil, "err.go", "err.go", []byte("package alpha\nfunc Broken( {}\n"))
	shared := token.NewFileSet()
	p.AddFile(shared, "b.go", "b.go", []byte("package beta\nfunc Second() {}\n"))
	if !p.Names["First"] {
		t.Fatalf("Names lost First after second AddFile: %v", p.Names)
	}
	if !p.Names["Second"] {
		t.Fatalf("Names missing Second after AddFile: %v", p.Names)
	}
	if !p.PkgNames["alpha"] {
		t.Fatalf("PkgNames lost alpha after AddFile: %v", p.PkgNames)
	}
	if !p.PkgNames["beta"] {
		t.Fatalf("PkgNames missing beta after AddFile: %v", p.PkgNames)
	}
	if _, ok := p.ParseErr["err.go"]; !ok {
		t.Fatalf("ParseErr lost err.go after AddFile: %v", p.ParseErr)
	}
	if !p.Has("First") || !p.Has("Second") {
		t.Fatalf("Has lost entries after AddFile: First=%v Second=%v", p.Has("First"), p.Has("Second"))
	}
	if len(p.Lookup("First")) == 0 {
		t.Fatalf("Lookup lost First sites after AddFile")
	}
	if len(p.Lookup("Second")) == 0 {
		t.Fatalf("Lookup missing Second sites after AddFile")
	}
	found := false
	shared.Iterate(func(f *token.File) bool {
		if f.Name() == "b.go" {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatalf("provided FileSet was not used for parse of b.go")
	}
}
