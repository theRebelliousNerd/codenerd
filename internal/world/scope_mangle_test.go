package world

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileScopeOpen_MangleFile(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "demo.mg")
	prog := `Decl p(A.Type<int>).
p(1).
q(X) :- p(X).
`
	if err := os.WriteFile(path, []byte(prog), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	scope := NewFileScope(ws)
	if err := scope.Open(path); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if got := scope.GetActiveFile(); got == "" {
		t.Fatalf("expected active file")
	}
	if got := len(scope.GetInScopeFiles()); got != 1 {
		t.Fatalf("in-scope file count = %d, want 1", got)
	}
	if got := len(scope.Elements); got == 0 {
		t.Fatalf("expected elements > 0")
	}

	facts := scope.ScopeFacts()
	var sawMangle bool
	for _, f := range facts {
		if f.Predicate != "file_in_scope" || len(f.Args) < 3 {
			continue
		}
		if lang, ok := f.Args[2].(string); ok && lang == "/mangle" {
			sawMangle = true
			break
		}
	}
	if !sawMangle {
		t.Fatalf("expected file_in_scope language /mangle")
	}
}

