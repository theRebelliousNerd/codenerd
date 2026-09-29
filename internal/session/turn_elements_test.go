package session

import (
	"sort"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// A turn that rewrites one function body, adds one method, deletes one
// function, and leaves a third untouched owes exactly the first two elements:
// the untouched one did not change, and a deleted function is not an element
// to witness.
const (
	elemGoMod  = "module elemprobe\n\ngo 1.21\n"
	elemBefore = `package elemprobe

func Greet(name string) string { return "hi " + name }

func Untouched() int { return 1 }

func Removed() int { return 2 }
`
	elemAfter = `package elemprobe

func Greet(name string) string { return "hello " + name }

func Untouched() int { return 1 }

type Greeter struct{}

func (g *Greeter) Greet() string { return "hi" }
`
)

func TestTurnChangedElement_RecordsChangedAndAddedFunctions(t *testing.T) {
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":  elemGoMod,
		"calc.go": elemAfter,
	})
	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws

	result := writeTurnResult()
	result.WrittenPaths = []string{"calc.go"}
	result.PreWriteContents = map[string]PreImage{"calc.go": existed(elemBefore)}
	e.assertTurnEvidence(testTurn, "/fix", result)

	facts, err := e.kernel.Query("turn_changed_element")
	if err != nil {
		t.Fatalf("query turn_changed_element: %v", err)
	}
	var refs []string
	for _, f := range facts {
		if len(f.Args) != 2 {
			t.Fatalf("turn_changed_element%v: want (Turn, Ref)", f.Args)
		}
		if got := types.ExtractString(f.Args[0]); got != string(testTurn) {
			t.Errorf("turn_changed_element turn = %v, want %v", got, testTurn)
		}
		refs = append(refs, types.ExtractString(f.Args[1]))
	}
	sort.Strings(refs)
	want := []string{"fn:elemprobe.Greet", "fn:elemprobe.Greeter.Greet"}
	if len(refs) != len(want) {
		t.Fatalf("turn_changed_element refs = %v, want %v", refs, want)
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Fatalf("turn_changed_element refs = %v, want %v: one rewritten body and one added method, the untouched and the deleted function owed nothing", refs, want)
		}
	}

	// Retracted with the turn's other per-turn facts.
	e.cleanupTurnFacts()
	if got := queryCount(t, e, "turn_changed_element"); got != 0 {
		t.Errorf("turn_changed_element = %d after cleanup, want 0", got)
	}
}

// The refs are code_element refs: the world's Go parser produces the same
// strings for the same file, so a future witness rule joins them directly.
// The generic-receiver case pins the exact match, quirk included: the world's
// receiver reader keeps identifiers and pointers only
// (extractReceiverTypeInfo in internal/world/go_parser.go), so a method on
// Box[T] is fn:<pkg>.Get in both places, not fn:<pkg>.Box.Get.
func TestTurnChangedElement_RefsMatchWorldCodeElements(t *testing.T) {
	worldRefs := func(content string) map[string]bool {
		t.Helper()
		elems, err := world.NewGoCodeParser(t.TempDir()).Parse("calc.go", []byte(content))
		if err != nil {
			t.Fatalf("world parse: %v", err)
		}
		refs := make(map[string]bool)
		for _, elem := range elems {
			if elem.Type == world.ElementFunction || elem.Type == world.ElementMethod {
				refs[elem.Ref] = true
			}
		}
		return refs
	}

	got := changedElementRefs(elemAfter, existed(elemBefore), "calc.go")
	refs := worldRefs(elemAfter)
	for _, ref := range got {
		if !refs[ref] {
			t.Errorf("turn_changed_element ref %q is not a code_element ref the world produces for the same file", ref)
		}
	}

	genericBefore := "package elemprobe\n\ntype Box[T any] struct{ v T }\n"
	genericAfter := genericBefore + "\nfunc (b *Box[T]) Get() T { return b.v }\n"
	got = changedElementRefs(genericAfter, existed(genericBefore), "box.go")
	refs = worldRefs(genericAfter)
	if len(got) != 1 {
		t.Fatalf("generic method refs = %v, want one", got)
	}
	if !refs[got[0]] {
		t.Errorf("turn_changed_element ref %q is not a code_element ref the world produces for the same file", got[0])
	}
}

func TestTurnChangedElement_ModelCannotAssert(t *testing.T) {
	update := `turn_changed_element(/turn_test, "fn:elemprobe.Greet").`
	permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
	if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
		t.Errorf("the model can assert %s; the verdict's evidence must be the harness's alone", update)
	}
	if kept, _ := core.FilterMangleUpdates(nil, []string{update}, core.ModelObservationPolicy()); len(kept) != 0 {
		t.Errorf("the model can assert %s through the observation policy", update)
	}
}
