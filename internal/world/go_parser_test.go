package world

import (
	"testing"
)

// TestNewGoCodeParser verifies the constructor behavior for GoCodeParser.
func TestNewGoCodeParser(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
	}{
		{
			name:        "valid project root",
			projectRoot: "/path/to/project",
		},
		{
			name:        "empty project root",
			projectRoot: "",
		},
		{
			name:        "relative project root",
			projectRoot: "./src",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewGoCodeParser(tt.projectRoot)
			if parser == nil {
				t.Fatal("NewGoCodeParser() returned nil")
			}
			if parser.projectRoot != tt.projectRoot {
				t.Errorf("NewGoCodeParser() projectRoot = %v, want %v", parser.projectRoot, tt.projectRoot)
			}
		})
	}
}

// TestGoCodeParser_ImplementsInterface verifies that *GoCodeParser implements the CodeParser interface.
func TestGoCodeParser_ImplementsInterface(t *testing.T) {
	var _ CodeParser = (*GoCodeParser)(nil)
}

// ============================================================================
// Boundary Value Analysis & Negative Testing Gaps (QA Audit)
// ============================================================================

// TODO: Implement TestGoParser_Parse_NilContent
// Vector: Null/Undefined/Empty. Simulate nil slice. Prevent panic.

// TODO: Implement TestGoParser_Parse_EmptyContent
// Vector: Null/Undefined/Empty. Simulate empty byte slice. Ensure clean return.

// TODO: Implement TestGoParser_Parse_EmptyPath
// Vector: Null/Undefined/Empty. Ensure relative pathing and Ref URI generation doesn't crash.

// TODO: Implement TestGoParser_Parse_WhitespaceAndCommentsOnly
// Vector: Boundary. Ensure purely non-functional code yields zero elements, no errors.

// TODO: Implement TestGoParser_Parse_BinaryData
// Vector: Type Coercion. Feed non-UTF8/binary payload. Ensure graceful syntax error.

// TODO: Implement TestGoParser_Parse_TruncatedSyntax
// Vector: Type Coercion/Boundary. Feed incomplete AST (missing braces). Verify bounds extraction.

// TODO: Implement TestGoParser_Parse_InvalidPackage
// Vector: Type Coercion. Feed invalid package declaration. Ensure graceful error.

// TODO: Implement TestGoParser_Parse_PythonSyntax
// Vector: Type Coercion. Feed completely foreign syntax. Ensure fast failure.

// TODO: Implement TestGoParser_Parse_MassiveGeneratedFile
// Vector: User Extremes. Generate 10MB go file in memory. Benchmark parsing time and memory bounds.

// TODO: Implement TestGoParser_Parse_ExtremeNesting
// Vector: User Extremes. Generate AST depth > 1000. Ensure no stack overflow panic.

// TODO: Implement TestGoParser_Parse_MassiveIdentifiers
// Vector: User Extremes. Function name > 10,000 chars. Ensure Ref URI string allocation doesn't OOM.

// TODO: Implement TestGoParser_Parse_PathologicalElementCount
// Vector: User Extremes. 100k tiny structs. Ensure garbage collector survives the slice allocation.

// TODO: Implement TestGoParser_Parse_ConcurrentAccess_RaceCondition
// Vector: State Conflicts. Run 100 concurrent Parse() calls on same instance with race detector enabled.

// TODO: Implement TestGoParser_Parse_PathTraversal
// Vector: State Conflicts/Security. Use path "../../../../etc/passwd". Verify Ref URI sanitization.

// TestGoCodeParser_GenericReceiverRef pins the method ref for every generic
// receiver spelling: the base type survives IndexExpr, IndexListExpr,
// ParenExpr and Star unwrapping, so Box[T].Get is fn:<pkg>.Box.Get and never
// collapses onto a plain func Get with the same name.
func TestGoCodeParser_GenericReceiverRef(t *testing.T) {
	src := `package elemprobe

type Box[T any] struct{ v T }

type Pair[A any, B any] struct{ a A; b B }

func Get() int { return 1 }

func (b *Box[T]) Get() T { return b.v }

func (b Box[T]) Clone() Box[T] { return b }

func (p *Pair[A, B]) First() A { return p.a }

func (b (Box[T])) Size() int { return 0 }
`
	elems, err := NewGoCodeParser(t.TempDir()).Parse("box.go", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := make(map[string]CodeElement)
	for _, elem := range elems {
		if elem.Type != ElementFunction && elem.Type != ElementMethod {
			continue
		}
		if _, dup := got[elem.Ref]; dup {
			t.Errorf("duplicate element ref %q: a generic method collapsed onto a same-named function", elem.Ref)
		}
		got[elem.Ref] = elem
	}
	wants := []struct {
		ref    string
		typ    ElementType
		parent string
	}{
		{"fn:elemprobe.Get", ElementFunction, ""},
		{"fn:elemprobe.Box.Get", ElementMethod, "struct:elemprobe.Box"},
		{"fn:elemprobe.Box.Clone", ElementMethod, "struct:elemprobe.Box"},
		{"fn:elemprobe.Pair.First", ElementMethod, "struct:elemprobe.Pair"},
		{"fn:elemprobe.Box.Size", ElementMethod, "struct:elemprobe.Box"},
	}
	if len(got) != len(wants) {
		refs := make([]string, 0, len(got))
		for ref := range got {
			refs = append(refs, ref)
		}
		t.Fatalf("function/method refs = %v, want %d elements", refs, len(wants))
	}
	for _, w := range wants {
		elem, ok := got[w.ref]
		if !ok {
			t.Errorf("missing element ref %q", w.ref)
			continue
		}
		if elem.Type != w.typ {
			t.Errorf("ref %q type = %v, want %v", w.ref, elem.Type, w.typ)
		}
		if elem.Parent != w.parent {
			t.Errorf("ref %q parent = %q, want %q", w.ref, elem.Parent, w.parent)
		}
	}
}
