package world

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileScopeOpen_IncludesPackageFiles(t *testing.T) {
	ws := t.TempDir()

	goMod := "module example.com/scope-test\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	a := `package demo

func A() int { return 1 }
`
	b := `package demo

func B() int { return 2 }
`
	testFile := `package demo

import "testing"

func TestA(t *testing.T) {}
`

	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte(a), 0644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "b.go"), []byte(b), 0644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "a_test.go"), []byte(testFile), 0644); err != nil {
		t.Fatalf("write a_test.go: %v", err)
	}

	scope := NewFileScope(ws)
	if err := scope.Open(filepath.Join(ws, "a.go")); err != nil {
		t.Fatalf("Open: %v", err)
	}

	files := scope.GetInScopeFiles()
	got := make(map[string]bool, len(files))
	for _, f := range files {
		got[filepath.Base(f)] = true
	}

	if !got["a.go"] || !got["b.go"] {
		t.Fatalf("expected package files in scope; got=%v", got)
	}
	if got["a_test.go"] {
		t.Fatalf("expected *_test.go to be excluded from package scope; got=%v", got)
	}
}
