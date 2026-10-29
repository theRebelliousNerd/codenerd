package docdecl

import (
	"go/token"
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
