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

func TestCartographer_MapFile_Go_Error(t *testing.T) {
	c := NewCartographer()
	defer c.Close()

	// Parse non-existent file
	_, err := c.MapFile("does_not_exist.go")
	if err == nil {
		t.Error("Expected error for non-existent Go file, got nil")
	}
}
