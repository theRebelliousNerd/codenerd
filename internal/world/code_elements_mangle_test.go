package world

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodeElementParser_ParseMangleFile(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "demo.mg")

	prog := `# Demo Mangle program
Decl parent(A.Type<name>, B.Type<name>).

parent(/a, /b).

ancestor(X, Y) :-
    parent(X, Y).
`
	if err := os.WriteFile(path, []byte(prog), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	p := NewCodeElementParser()
	elems, err := p.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var haveDecl, haveFact, haveRule bool
	for _, e := range elems {
		switch e.Type {
		case ElementMangleDecl:
			if e.Ref != "decl:parent/2" {
				t.Fatalf("decl ref = %q, want %q", e.Ref, "decl:parent/2")
			}
			haveDecl = true
		case ElementMangleFact:
			if e.Ref == "fact:parent/2#1" {
				haveFact = true
			}
		case ElementMangleRule:
			if e.Ref != "rule:ancestor/2#1" {
				t.Fatalf("rule ref = %q, want %q", e.Ref, "rule:ancestor/2#1")
			}
			if e.StartLine != 6 || e.EndLine != 7 {
				t.Fatalf("rule lines = %d..%d, want %d..%d", e.StartLine, e.EndLine, 6, 7)
			}
			haveRule = true
		}
	}

	if !haveDecl {
		t.Fatalf("expected decl element")
	}
	if !haveFact {
		t.Fatalf("expected fact element")
	}
	if !haveRule {
		t.Fatalf("expected rule element")
	}
}

