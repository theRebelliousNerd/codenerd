package core

import (
	"testing"
)

// TestSymbolNameFromRef covers the three ref shapes the parsers emit.
func TestSymbolNameFromRef(t *testing.T) {
	cases := map[string]string{
		"fn:world.NewHolographicProvider":         "NewHolographicProvider",
		"fn:world.HolographicProvider.GetContext": "GetContext",
		"fn:TestFoo":                      "TestFoo",
		"struct:world.HolographicContext": "HolographicContext",
		"interface:core.Kernel":           "Kernel",
		"fn:module::test_thing":           "test_thing",
		"":                                "",
	}
	for ref, want := range cases {
		if got := symbolNameFromRef(ref); got != want {
			t.Errorf("symbolNameFromRef(%q) = %q, want %q", ref, got, want)
		}
	}
}

// TestModifiedSymbolFacts is the producer half of the impact chain. impact.mg
// joins modified_function against code_calls to derive who is affected; nothing
// produced that fact until 2026-09-09, so the whole chain derived nothing.
func TestModifiedSymbolFacts(t *testing.T) {
	cases := []struct {
		name     string
		elem     *CodeElement
		wantPred string
		wantName string
	}{
		{
			name:     "function",
			elem:     &CodeElement{Ref: "fn:world.Target", Type: "function", File: "target.go"},
			wantPred: "modified_function",
			wantName: "Target",
		},
		{
			name:     "method keeps its own name, not the receiver's",
			elem:     &CodeElement{Ref: "fn:world.Provider.GetContext", Type: "method", File: "p.go"},
			wantPred: "modified_function",
			wantName: "GetContext",
		},
		{
			name:     "interface",
			elem:     &CodeElement{Ref: "interface:core.Kernel", Type: "interface", File: "k.go"},
			wantPred: "modified_interface",
			wantName: "Kernel",
		},
		// A struct edit is still recorded by element_modified and
		// modified(File); it just does not start a caller walk, because there
		// is no caller relation for a struct to walk.
		{name: "struct produces nothing", elem: &CodeElement{Ref: "struct:world.C", Type: "struct", File: "c.go"}},
		{name: "nil element produces nothing", elem: nil},
		{name: "missing file produces nothing", elem: &CodeElement{Ref: "fn:world.Target", Type: "function"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := modifiedSymbolFacts(tc.elem)
			if tc.wantPred == "" {
				if len(facts) != 0 {
					t.Fatalf("expected no facts, got %+v", facts)
				}
				return
			}
			if len(facts) != 1 {
				t.Fatalf("expected exactly 1 fact, got %d: %+v", len(facts), facts)
			}
			if facts[0].Predicate != tc.wantPred {
				t.Errorf("predicate = %q, want %q", facts[0].Predicate, tc.wantPred)
			}
			if len(facts[0].Args) != 2 {
				t.Fatalf("expected 2 args, got %v", facts[0].Args)
			}
			if facts[0].Args[0] != tc.wantName {
				t.Errorf("symbol name = %v, want %q", facts[0].Args[0], tc.wantName)
			}
			if facts[0].Args[1] != tc.elem.File {
				t.Errorf("file = %v, want %q", facts[0].Args[1], tc.elem.File)
			}
		})
	}
}

// lineRangeScope is a CodeScope that answers only GetCoreElementsByFile.
type lineRangeScope struct {
	CodeScope
	elements []CodeElement
}

func (s *lineRangeScope) GetCoreElementsByFile(string) []CodeElement { return s.elements }

// TestModifiedSymbolFactsForLineRange covers the path the model actually uses.
// edit_lines / insert_lines / delete_lines name a range, not an element, so the
// producer has to resolve overlap — and an edit that touches any line of a
// function has modified that function.
func TestModifiedSymbolFactsForLineRange(t *testing.T) {
	scope := &lineRangeScope{elements: []CodeElement{
		{Ref: "fn:p.Alpha", Type: "function", File: "p.go", StartLine: 1, EndLine: 10},
		{Ref: "fn:p.Beta", Type: "function", File: "p.go", StartLine: 12, EndLine: 20},
		{Ref: "struct:p.S", Type: "struct", File: "p.go", StartLine: 22, EndLine: 25},
	}}

	names := func(facts []Fact) []string {
		out := make([]string, 0, len(facts))
		for _, f := range facts {
			out = append(out, f.Args[0].(string))
		}
		return out
	}

	cases := []struct {
		name       string
		start, end int
		want       []string
	}{
		{name: "inside one function", start: 5, end: 5, want: []string{"Alpha"}},
		{name: "spanning two functions", start: 9, end: 13, want: []string{"Alpha", "Beta"}},
		{name: "gap between elements touches nothing", start: 11, end: 11},
		{name: "struct range yields no caller-walk fact", start: 23, end: 24},
		{name: "boundary line is inside", start: 10, end: 10, want: []string{"Alpha"}},
		{name: "inverted range is refused", start: 9, end: 1},
		{name: "range past the file touches nothing", start: 100, end: 200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := names(modifiedSymbolFactsForLineRange(scope, "p.go", tc.start, tc.end))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestModifiedSymbolFactsForLineRange_NilScope keeps the handlers safe: the line
// tools run whether or not a CodeDOM scope is open.
func TestModifiedSymbolFactsForLineRange_NilScope(t *testing.T) {
	if got := modifiedSymbolFactsForLineRange(nil, "p.go", 1, 2); got != nil {
		t.Fatalf("nil scope must produce no facts, got %+v", got)
	}
	scope := &lineRangeScope{}
	if got := modifiedSymbolFactsForLineRange(scope, "", 1, 2); got != nil {
		t.Fatalf("empty path must produce no facts, got %+v", got)
	}
}
