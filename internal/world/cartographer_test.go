package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
)

func TestNewCartographer(t *testing.T) {
	c := NewCartographer()
	if c == nil {
		t.Fatal("NewCartographer() returned nil")
	}
	if c.dataFlowExtractor == nil {
		t.Error("NewCartographer() did not initialize dataFlowExtractor")
	}
}

func TestCartographer_MapFile_Unsupported(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	facts, err := c.MapFile("test.txt")
	if err != nil {
		t.Errorf("Expected nil error for unsupported file, got %v", err)
	}
	if facts != nil {
		t.Errorf("Expected nil facts for unsupported file, got %v", facts)
	}
}

func TestCartographer_MapFile_Go(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	// Create a temporary Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")

	// A simple program that should emit some predictable facts
	content := []byte(`package main

type MyStruct struct {}

func hello() {
	println("hello")
}

func (m *MyStruct) method() {
	hello()
}
`)
	if err := os.WriteFile(goFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	facts, err := c.MapFile(goFile)
	if err != nil {
		t.Errorf("Expected nil error for Go file, got %v", err)
	}
	if len(facts) == 0 {
		t.Error("Expected facts for Go file, got empty")
	}

	// Verify we got the expected facts
	foundHello := false
	foundStruct := false
	foundMethod := false
	foundCall := false
	foundDualCall := false

	for _, f := range facts {
		if f.Predicate == "code_defines" {
			id, ok1 := f.Args[1].(string)
			typeAtom, ok2 := f.Args[2].(core.MangleAtom)

			if ok1 && ok2 {
				typeStr := string(typeAtom)

				if id == "main.hello" && typeStr == "/function" {
					foundHello = true
				} else if id == "main.MyStruct" && typeStr == "/struct" {
					foundStruct = true
				} else if id == "main.MyStruct.method" && typeStr == "/function" {
					foundMethod = true
				}
			}
		} else if f.Predicate == "code_calls" {
			caller, ok1 := f.Args[0].(string)
			callee, ok2 := f.Args[1].(string)

			if ok1 && ok2 {
				if caller == "main.MyStruct.method" && callee == "main.hello" {
					foundCall = true
				}
				if caller == "fn:main.MyStruct.method" && callee == "fn:main.hello" {
					foundDualCall = true
				}
			}
		}
	}

	if !foundHello {
		t.Error("Expected to find code_defines fact for main.hello")
	}
	if !foundStruct {
		t.Error("Expected to find code_defines fact for main.MyStruct")
	}
	if !foundMethod {
		t.Error("Expected to find code_defines fact for main.MyStruct.method")
	}
	if !foundCall {
		t.Error("Expected to find code_calls fact from main.MyStruct.method to main.hello")
	}
	if !foundDualCall {
		t.Error("Expected to find dual code_calls fact from fn:main.MyStruct.method to fn:main.hello")
	}
}

// TestCartographer_FnCallRows_MatchCodeElementRefs locks the fn: spelling
// of code_calls against buildRef (fn:<pkg>.<Name>, fn:<pkg>.<Recv>.<Name>).
// A selector used to be copied off the expression (fn:s.M, fn:t.Fail), and
// the enclosing function was never restored, so a package-level call after
// a function was credited to it.
func TestCartographer_FnCallRows_MatchCodeElementRefs(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "callers.go")
	src := `package p

import (
	"fmt"
	"testing"
)

type S struct{ N int }

func (s *S) M() int { return sink() }

func (b *Box[T]) Get() int { return sink() }

type Box[T any] struct{ V T }

func sink() int { return 1 }

func byComposite() {
	s := S{}
	s.M()
}

func byPointerLit() {
	s := &S{}
	s.M()
}

func byExplicit() {
	var s S
	s.M()
}

func byPointerVar() {
	var s *S
	s.M()
}

func byParam(s S) {
	s.M()
	fmt.Println(s.N)
}

func byParamPtr(s *S) {
	s.M()
}

func TestValue(t *testing.T) {
	t.Fail()
}

func earlier() {}

var leaked = helper()

var wrapped = func() int { return helper() }

func helper() int { return 1 }
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	facts, err := c.MapFile(path)
	if err != nil {
		t.Fatalf("MapFile: %v", err)
	}

	calls := map[[2]string]bool{}
	for _, f := range facts {
		if f.Predicate != "code_calls" || len(f.Args) < 2 {
			continue
		}
		caller, ok1 := f.Args[0].(string)
		callee, ok2 := f.Args[1].(string)
		if !ok1 || !ok2 {
			t.Fatalf("code_calls args are not strings: %#v", f.Args)
		}
		calls[[2]string{caller, callee}] = true
	}

	for _, caller := range []string{
		"fn:p.byComposite",
		"fn:p.byPointerLit",
		"fn:p.byExplicit",
		"fn:p.byPointerVar",
		"fn:p.byParam",
		"fn:p.byParamPtr",
	} {
		if !calls[[2]string{caller, "fn:p.S.M"}] {
			t.Errorf("missing code_calls(%s, fn:p.S.M)", caller)
		}
	}
	if !calls[[2]string{"fn:p.S.M", "fn:p.sink"}] {
		t.Error("missing code_calls(fn:p.S.M, fn:p.sink) for the same-package ident call")
	}
	if !calls[[2]string{"fn:p.Box.Get", "fn:p.sink"}] {
		t.Error("missing code_calls(fn:p.Box.Get, fn:p.sink); generic receiver was dropped from the caller ref")
	}
	if !calls[[2]string{"p.TestValue", "t.Fail"}] {
		t.Error("bare code_calls(p.TestValue, t.Fail) was dropped")
	}
	if !calls[[2]string{"p.byParam", "fmt.Println"}] {
		t.Error("bare code_calls(p.byParam, fmt.Println) was dropped")
	}

	for pair := range calls {
		if strings.HasPrefix(pair[1], "fn:") && (strings.Contains(pair[1], "Fail") || strings.Contains(pair[1], "Println") || pair[1] == "fn:s.M") {
			t.Errorf("fn: row does not name a code_element: code_calls(%s, %s)", pair[0], pair[1])
		}
		if strings.Contains(pair[0], "earlier") && strings.Contains(pair[1], "helper") {
			t.Errorf("package-level helper() attributed to earlier: code_calls(%s, %s)", pair[0], pair[1])
		}
		if pair[0] == "p.Get" || pair[0] == "fn:p.Get" {
			t.Errorf("generic method collapsed onto Get: code_calls(%s, %s)", pair[0], pair[1])
		}
	}
	for _, caller := range []string{"p.earlier", "fn:p.earlier", "p.helper", "fn:p.helper"} {
		for pair := range calls {
			if pair[0] == caller && strings.Contains(pair[1], "helper") {
				t.Errorf("helper() call attributed to %s -> %s", pair[0], pair[1])
			}
		}
	}
}

// mapGoFixture writes a mini module and maps every .go file. fns is the
// code_defines spelling (fn:<pkg>.<Name> / fn:<pkg>.<Recv>.<Name>) so a
// callee that is not a real function element fails the test.
func mapGoFixture(t *testing.T, files map[string]string) (calls map[[2]string]bool, fns map[string]bool) {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := NewCartographer()
	t.Cleanup(func() { c.Close() })
	calls = map[[2]string]bool{}
	fns = map[string]bool{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		facts, mapErr := c.MapFile(path)
		if mapErr != nil {
			t.Fatalf("MapFile %s: %v", path, mapErr)
		}
		for _, f := range facts {
			switch f.Predicate {
			case "code_calls":
				if len(f.Args) < 2 {
					continue
				}
				caller, ok1 := f.Args[0].(string)
				callee, ok2 := f.Args[1].(string)
				if ok1 && ok2 {
					calls[[2]string{caller, callee}] = true
				}
			case "code_defines":
				if len(f.Args) < 3 {
					continue
				}
				id, ok1 := f.Args[1].(string)
				atom, ok2 := f.Args[2].(core.MangleAtom)
				if ok1 && ok2 && string(atom) == "/function" {
					fns["fn:"+id] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for pair := range calls {
		if strings.HasPrefix(pair[1], "fn:") && !fns[pair[1]] {
			t.Errorf("fn: row is not a code_element: code_calls(%s, %s)", pair[0], pair[1])
		}
	}
	return calls, fns
}

func requireCall(t *testing.T, calls map[[2]string]bool, caller, callee string) {
	t.Helper()
	if !calls[[2]string{caller, callee}] {
		t.Errorf("missing code_calls(%s, %s)", caller, callee)
	}
}

func forbidCall(t *testing.T, calls map[[2]string]bool, caller, callee string) {
	t.Helper()
	if calls[[2]string{caller, callee}] {
		t.Errorf("unexpected code_calls(%s, %s)", caller, callee)
	}
}

// TestCartographer_CertainCallEdges covers call shapes goCallRef used to
// drop, and the cross-file conversion that used to invent a fn: row.
// Every fn: callee must be a code_defines element from the fixture
// (mapGoFixture). Promoted methods and aliases to another package are not
// guessed. A dot import keeps fn: rows for functions this package declares
// and adds none for any other name.
func TestCartographer_CertainCallEdges(t *testing.T) {
	t.Run("dot import keeps same-package functions", func(t *testing.T) {
		// Import paths are not resolved. A name this package declares keeps
		// fn:<pkg>.<Name>, including an exported name the dot import might
		// also declare. A name it does not declare gets no fn: row, and a
		// function in a parent or child directory is a different package.
		calls, _ := mapGoFixture(t, map[string]string{
			"a.go": `package p

func helper() int { return 1 }

func NotInFmt() int { return 2 }

func Println(a ...any) (int, error) { return 0, nil }

func OnlyInParent() int { return 4 }
`,
			"b_test.go": `package p

import . "fmt"

func TestHelper(t *testing.T) {
	if helper() != 1 {
		t.Fail()
	}
	NotInFmt()
	Println("x")
	FromDot()
	OnlyInChild()
}
`,
			"p/p.go": `package p

func Run() int { return helper() }

func helper() int { return 3 }

func OnlyInChild() int { return 5 }
`,
			"p/p_test.go": `package p

import . "example.com/m/lib"

func TestRun(t *testing.T) {
	helper()
	Run()
	Assert(true)
	OnlyInParent()
}
`,
			"q/q.go": `package q

func Exported() int { return 1 }

func hidden() int { return 2 }
`,
			"q/q_test.go": `package q_test

import . "example.com/m/q"

func TestExported(t *testing.T) {
	Exported()
	hidden()
}
`,
		})
		requireCall(t, calls, "fn:p.TestHelper", "fn:p.helper")
		requireCall(t, calls, "fn:p.TestHelper", "fn:p.NotInFmt")
		requireCall(t, calls, "fn:p.TestHelper", "fn:p.Println")
		forbidCall(t, calls, "fn:p.TestHelper", "fn:fmt.Println")
		forbidCall(t, calls, "fn:p.TestHelper", "fn:p.FromDot")
		forbidCall(t, calls, "fn:p.TestHelper", "fn:p.Fail")
		forbidCall(t, calls, "fn:p.TestHelper", "fn:p.OnlyInChild")

		requireCall(t, calls, "fn:p.TestRun", "fn:p.helper")
		requireCall(t, calls, "fn:p.TestRun", "fn:p.Run")
		forbidCall(t, calls, "fn:p.TestRun", "fn:lib.Assert")
		forbidCall(t, calls, "fn:p.TestRun", "fn:p.Assert")
		forbidCall(t, calls, "fn:p.TestRun", "fn:p.OnlyInParent")

		forbidCall(t, calls, "fn:q_test.TestExported", "fn:q.Exported")
		forbidCall(t, calls, "fn:q_test.TestExported", "fn:q_test.Exported")
		forbidCall(t, calls, "fn:q_test.TestExported", "fn:q.hidden")
		forbidCall(t, calls, "fn:q_test.TestExported", "fn:q_test.hidden")
	})

	t.Run("explicit instantiation", func(t *testing.T) {
		calls, _ := mapGoFixture(t, map[string]string{
			"f.go": `package p

func F[T any](v T) T { return v }

func G[A any, B any](a A, b B) {}

type Box[T any] struct{ V T }

func (b Box[T]) Get() T { return b.V }

func caller() {
	F[int](1)
	G[int, string](1, "x")
	F[Box[int]](Box[int]{})
}

func instMethod() {
	var b Box[int]
	Box[int].Get(b)
}

func indexCall() {
	fns := []func(){caller}
	fns[0]()
}
`,
			"other.go": `package p

type Other int
`,
			"use.go": `package p

func conv(x int) {
	Other(x)
	F[Other](0)
}
`,
		})
		requireCall(t, calls, "fn:p.caller", "fn:p.F")
		requireCall(t, calls, "fn:p.caller", "fn:p.G")
		requireCall(t, calls, "fn:p.instMethod", "fn:p.Box.Get")
		requireCall(t, calls, "fn:p.conv", "fn:p.F")
		forbidCall(t, calls, "fn:p.conv", "fn:p.Other")
		forbidCall(t, calls, "fn:p.indexCall", "fn:p.fns")
		forbidCall(t, calls, "fn:p.indexCall", "fn:p.caller")
	})

	t.Run("method expression and paren receiver", func(t *testing.T) {
		calls, _ := mapGoFixture(t, map[string]string{
			"s.go": `package p

type S struct{}

func (s S) M() int { return 1 }

func (s *S) N() int { return 2 }

func (s *S) P() int { return 3 }

func (s *S) Q() int { return 4 }

func methodExpr(s S, p *S) {
	S.M(s)
	(*S).N(p)
}

func deref(p *S) {
	(*p).P()
}

func paren(p *S) {
	(p).Q()
	((*p)).Q()
}
`,
		})
		requireCall(t, calls, "fn:p.methodExpr", "fn:p.S.M")
		requireCall(t, calls, "fn:p.methodExpr", "fn:p.S.N")
		requireCall(t, calls, "fn:p.deref", "fn:p.S.P")
		requireCall(t, calls, "fn:p.paren", "fn:p.S.Q")
	})

	t.Run("alias resolves and promoted is not guessed", func(t *testing.T) {
		calls, _ := mapGoFixture(t, map[string]string{
			"a.go": `package p

import "fmt"

type Inner struct{}

func (Inner) M() int { return 1 }

type Outer struct{ Inner }

type S struct{}

func (s S) M() int { return 2 }

type A = S

type B = A

type C = fmt.Stringer

func promoted(o Outer) { o.M() }

func alias(a A, b B) {
	a.M()
	b.M()
	A.M(a)
}

func external(c C) { c.String() }
`,
		})
		requireCall(t, calls, "fn:p.alias", "fn:p.S.M")
		forbidCall(t, calls, "fn:p.alias", "fn:p.A.M")
		forbidCall(t, calls, "fn:p.alias", "fn:p.B.M")
		forbidCall(t, calls, "fn:p.promoted", "fn:p.Outer.M")
		forbidCall(t, calls, "fn:p.promoted", "fn:p.Inner.M")
		forbidCall(t, calls, "fn:p.external", "fn:p.C.String")
		forbidCall(t, calls, "fn:p.external", "fn:fmt.Stringer.String")
	})
}

// TestCartographer_SiblingSymbolsFollowFileChange locks the directory symbol
// cache: a sibling rewrite changes the stamp (name, size, mtime), and a
// .go file in a child directory is a different package.
func TestCartographer_SiblingSymbolsFollowFileChange(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "n.go"), []byte("package p\n\nfunc Nested() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	caller := filepath.Join(dir, "call.go")
	if err := os.WriteFile(caller, []byte(`package p

func Caller() {
	Target()
	Other()
	Nested()
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	sib := filepath.Join(dir, "sib.go")
	writeSib := func(src string) {
		t.Helper()
		if err := os.WriteFile(sib, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSib("package p\n\nfunc Target() int { return 1 }\n")

	c := NewCartographer()
	t.Cleanup(func() { c.Close() })
	callsOf := func() map[[2]string]bool {
		t.Helper()
		facts, err := c.MapFile(caller)
		if err != nil {
			t.Fatal(err)
		}
		out := map[[2]string]bool{}
		for _, f := range facts {
			if f.Predicate != "code_calls" || len(f.Args) < 2 {
				continue
			}
			a, ok1 := f.Args[0].(string)
			b, ok2 := f.Args[1].(string)
			if ok1 && ok2 {
				out[[2]string{a, b}] = true
			}
		}
		return out
	}

	first := callsOf()
	requireCall(t, first, "fn:p.Caller", "fn:p.Target")
	forbidCall(t, first, "fn:p.Caller", "fn:p.Other")
	forbidCall(t, first, "fn:p.Caller", "fn:p.Nested")

	writeSib("package p\n\nfunc Other() int { return 22 }\n")
	second := callsOf()
	forbidCall(t, second, "fn:p.Caller", "fn:p.Target")
	requireCall(t, second, "fn:p.Caller", "fn:p.Other")
	forbidCall(t, second, "fn:p.Caller", "fn:p.Nested")
}

func TestCartographer_MapFile_Go_Error(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	// Parse non-existent file
	_, err := c.MapFile("does_not_exist.go")
	if err == nil {
		t.Error("Expected error for non-existent Go file, got nil")
	}
}
