package core

import (
	"context"
	"testing"

	"codeberg.org/TauCeti/mangle-go/factstore"

	"codenerd/internal/mangle"
)

// The defect these guard (F-WHY-1, observed live): `nerd why` classified every
// predicate it did not recognise as EDB, and buildDerivationNode only recurses
// into premises for IDB facts. So a genuinely derived fact rendered as a bare
// one-line leaf with no chain.
//
// Two independent causes, both fixed here:
//  1. classifyFact consulted only the hand-maintained rule_metadata /
//     is_edb_predicate facts in the .mg corpus. Anything added after that list
//     was written fell through to the EDB default.
//  2. findPremises switched over four hardcoded rule names out of the hundreds
//     in the corpus. Every other rule produced no premises even when correctly
//     classified as IDB.
//
// Live symptom: project_write_protected, derived at policy/projectdoc.mg:10,
// printed "[EDB]" with no supporting facts. A glass box you cannot see through
// is worse than none, because it still reads as an answer.

// newTracedKernel builds a kernel holding only the given program, so a
// classification result is attributable to that program rather than to
// whichever of the 244 corpus .mg files happens to declare a similarly named
// predicate. Same construction shape as TestKernelQueryPatternFiltering.
func newTracedKernel(t *testing.T, program string, facts []Fact) *RealKernel {
	t.Helper()

	k := &RealKernel{
		facts:       make([]Fact, 0, len(facts)),
		factIndex:   make(map[string]struct{}),
		store:       factstore.NewSimpleInMemoryStore(),
		policyDirty: true,
	}
	k.schemas = program
	k.facts = append(k.facts, facts...)
	k.rebuildFactIndexLocked()

	// Installing clauses does not run the fixpoint; without this every derived
	// predicate reads empty and the test passes for the wrong reason.
	if err := k.evaluate(); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	return k
}

const traceSchema = `
Decl base_thing(Name) bound [/string].
Decl derived_thing(Name) bound [/string].
derived_thing(N) :- base_thing(N).
`

func TestClassifyFact_DerivedPredicateIsIDBWithoutRuleMetadata(t *testing.T) {
	k := newTracedKernel(t, traceSchema, []Fact{
		{Predicate: "base_thing", Args: []any{"alpha"}},
	})

	source, _ := k.classifyFact(mangle.Fact{Predicate: "derived_thing", Args: []any{"alpha"}})

	if source != mangle.SourceIDB {
		t.Errorf("derived_thing classified as %v; a rule head is derived regardless of whether rule_metadata mentions it", source)
	}
}

func TestClassifyFact_BasePredicateStaysEDB(t *testing.T) {
	k := newTracedKernel(t, traceSchema, []Fact{
		{Predicate: "base_thing", Args: []any{"alpha"}},
	})

	source, _ := k.classifyFact(mangle.Fact{Predicate: "base_thing", Args: []any{"alpha"}})

	if source != mangle.SourceEDB {
		t.Errorf("base_thing classified as %v, want EDB", source)
	}
}

// The chain itself: a derived fact must name what supported it, for ANY rule,
// not just the four the old switch happened to list.
func TestPremisesFromProgram_FindsSupportForAnUnlistedRule(t *testing.T) {
	k := newTracedKernel(t, traceSchema, []Fact{
		{Predicate: "base_thing", Args: []any{"alpha"}},
	})

	premises := k.premisesFromProgram(mangle.Fact{Predicate: "derived_thing", Args: []any{"alpha"}})

	if len(premises) == 0 {
		t.Fatal("no premises found; nerd why would render this derived fact as a leaf with no explanation")
	}
	found := false
	for _, p := range premises {
		if p.Predicate == "base_thing" {
			found = true
		}
	}
	if !found {
		t.Errorf("premises did not include base_thing, the only body predicate: %+v", premises)
	}
}

// A zero-arity head has nothing to correlate on, so existence-triggered rules
// must still show their support. project_write_protected() is exactly this
// shape and was the live reproduction.
func TestPremisesFromProgram_ZeroArityHeadShowsAllSupport(t *testing.T) {
	const schema = `
Decl forbidden(Path, Reason) bound [/string, /string].
Decl protected() bound [].
protected() :- forbidden(_, _).
`
	k := newTracedKernel(t, schema, []Fact{
		{Predicate: "forbidden", Args: []any{"a.json", "reason a"}},
		{Predicate: "forbidden", Args: []any{"b.json", "reason b"}},
	})

	premises := k.premisesFromProgram(mangle.Fact{Predicate: "protected", Args: []any{}})

	if len(premises) != 2 {
		t.Fatalf("got %d premises for a zero-arity head, want both supporting facts: %+v", len(premises), premises)
	}
}

// End to end through the public entry point the CLI uses.
func TestTraceQuery_DerivedFactCarriesItsPremises(t *testing.T) {
	k := newTracedKernel(t, traceSchema, []Fact{
		{Predicate: "base_thing", Args: []any{"alpha"}},
	})

	trace, err := k.TraceQuery(context.Background(), "derived_thing")
	if err != nil {
		t.Fatalf("TraceQuery: %v", err)
	}
	if len(trace.RootNodes) == 0 {
		t.Fatal("no root nodes")
	}

	root := trace.RootNodes[0]
	if root.Source != mangle.SourceIDB {
		t.Errorf("root classified %v, want IDB", root.Source)
	}
	if len(root.Children) == 0 {
		t.Error("derived root has no children; the rendered trace would explain nothing")
	}
}
