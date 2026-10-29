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
// A generic receiver keeps its base type in both places: a method on Box[T]
// is fn:<pkg>.Box.Get, never fn:<pkg>.Get, so a same-named plain function
// cannot discharge the method's witness obligation.
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
	if len(got) != 1 || got[0] != "fn:elemprobe.Box.Get" {
		t.Fatalf("generic method refs = %v, want [fn:elemprobe.Box.Get]", got)
	}
	if !refs[got[0]] {
		t.Errorf("turn_changed_element ref %q is not a code_element ref the world produces for the same file", got[0])
	}

	// A plain func Get beside methods on Box[T] and Pair[A, B]: every
	// receiver spelling (pointer, value, multi-parameter, parenthesized)
	// keeps its base type, so each declaration gets its own ref and the
	// turn facts stay byte-identical to the world elements.
	collideBefore := "package elemprobe\n\ntype Box[T any] struct{ v T }\n\ntype Pair[A any, B any] struct{ a A; b B }\n"
	collideAfter := collideBefore +
		"\nfunc Get() int { return 1 }\n" +
		"\nfunc (b Box[T]) Get() T { return b.v }\n" +
		"\nfunc (p *Pair[A, B]) First() A { return p.a }\n" +
		"\nfunc (b (Box[T])) Size() int { return 0 }\n"
	got = changedElementRefs(collideAfter, existed(collideBefore), "box.go")
	refs = worldRefs(collideAfter)
	collideWant := []string{"fn:elemprobe.Box.Get", "fn:elemprobe.Box.Size", "fn:elemprobe.Get", "fn:elemprobe.Pair.First"}
	if len(got) != len(collideWant) {
		t.Fatalf("colliding-name refs = %v, want %v", got, collideWant)
	}
	for i := range collideWant {
		if got[i] != collideWant[i] {
			t.Fatalf("colliding-name refs = %v, want %v", got, collideWant)
		}
	}
	for _, ref := range got {
		if !refs[ref] {
			t.Errorf("turn_changed_element ref %q is not a code_element ref the world produces for the same file", ref)
		}
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

// A turn that changes only a _test.go file, a go-tool-ignored fixture, or an
// init function owes no turn_changed_element facts: a witness rule must not
// demand a test of a test, a testdata fixture is not built, and init has no
// ref that names which one changed.
func TestTurnChangedElement_SkipsTestsFixturesAndInit(t *testing.T) {
	testBefore := "package elemprobe\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {}\n"
	testAfter := "package elemprobe\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) { if Greet(\"x\") == \"\" { t.Fatal(\"empty\") } }\n"
	fixtureBefore := "package elemprobe\n\nfunc Fixture() int { return 1 }\n"
	fixtureAfter := "package elemprobe\n\nfunc Fixture() int { return 2 }\n"
	initBefore := "package elemprobe\n\nvar Started = \"\"\n\nfunc init() { Started = \"old\" }\n"
	initAfter := "package elemprobe\n\nvar Started = \"\"\n\nfunc init() { Started = \"new\" }\n"
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":              elemGoMod,
		"calc_test.go":        testAfter,
		"testdata/fixture.go": fixtureAfter,
		"_old/legacy.go":      fixtureAfter,
		"calc.go":             initAfter,
	})
	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws

	result := writeTurnResult()
	result.WrittenPaths = []string{"calc_test.go", "testdata/fixture.go", "_old/legacy.go", "calc.go"}
	result.PreWriteContents = map[string]PreImage{
		"calc_test.go":        existed(testBefore),
		"testdata/fixture.go": existed(fixtureBefore),
		"_old/legacy.go":      existed(fixtureBefore),
		"calc.go":             existed(initBefore),
	}
	e.assertTurnEvidence(testTurn, "/fix", result)

	if got := queryCount(t, e, "turn_changed_element"); got != 0 {
		t.Errorf("turn_changed_element = %d for test/fixture/init-only changes, want 0", got)
	}
}

// A file the go tool excludes by build constraint owes no witness either,
// the way the pin gate treats it (pinUnits in pin_gate.go skips what
// build.Default.MatchFile excludes). //go:build ignore never compiles on
// any platform, so the test holds on every GOOS.
func TestTurnChangedElement_SkipsBuildExcludedFiles(t *testing.T) {
	ignoredBefore := "//go:build ignore\n\npackage elemprobe\n\nfunc Greet() string { return \"hi\" }\n"
	ignoredAfter := "//go:build ignore\n\npackage elemprobe\n\nfunc Greet() string { return \"hello\" }\n"
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":     elemGoMod,
		"ignored.go": ignoredAfter,
	})
	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws

	result := writeTurnResult()
	result.WrittenPaths = []string{"ignored.go"}
	result.PreWriteContents = map[string]PreImage{"ignored.go": existed(ignoredBefore)}
	e.assertTurnEvidence(testTurn, "/fix", result)

	if got := queryCount(t, e, "turn_changed_element"); got != 0 {
		t.Errorf("turn_changed_element = %d for a //go:build ignore change, want 0", got)
	}
}
