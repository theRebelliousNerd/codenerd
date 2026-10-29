package core

import (
	"testing"
)

// TestSymbolIDFromRef pins the identifier shape the world model's call graph is
// keyed by. It must match world.Cartographer exactly — <pkg>.<Name> for a
// function and <pkg>.<Receiver>.<Name> for a method — or the join in impact.mg
// never fires.
func TestSymbolIDFromRef(t *testing.T) {
	cases := map[string]string{
		"fn:world.NewHolographicProvider":         "world.NewHolographicProvider",
		"fn:world.HolographicProvider.GetContext": "world.HolographicProvider.GetContext",
		"fn:TestFoo":                      "TestFoo",
		"struct:world.HolographicContext": "world.HolographicContext",
		"interface:core.Kernel":           "core.Kernel",
		"":                                "",
	}
	for ref, want := range cases {
		if got := symbolIDFromRef(ref); got != want {
			t.Errorf("symbolIDFromRef(%q) = %q, want %q", ref, got, want)
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
			wantName: "world.Target",
		},
		{
			name:     "method keeps its own name, not the receiver's",
			elem:     &CodeElement{Ref: "fn:world.Provider.GetContext", Type: "method", File: "p.go"},
			wantPred: "modified_function",
			wantName: "world.Provider.GetContext",
		},
		{
			name:     "interface",
			elem:     &CodeElement{Ref: "interface:core.Kernel", Type: "interface", File: "k.go"},
			wantPred: "modified_interface",
			wantName: "core.Kernel",
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
			facts := modifiedSymbolFacts(tc.elem, elemFile(tc.elem))
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

// elemFile mirrors the handlers, which pass the element's file as the fact
// identity; the test fixtures already use canonical spellings.
func elemFile(elem *CodeElement) string {
	if elem == nil {
		return ""
	}
	return elem.File
}
