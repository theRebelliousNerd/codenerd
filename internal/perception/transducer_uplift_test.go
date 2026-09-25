package perception

import (
	"testing"

	"codenerd/internal/core"
)

// TestGetVerbCorpus_SnapshotIsolation pins the snapshot contract: mutating
// the returned slice must not corrupt the registry for other readers.
func TestGetVerbCorpus_SnapshotIsolation(t *testing.T) {
	first := GetVerbCorpus()
	if len(first) == 0 {
		t.Skip("empty verb corpus")
	}
	first[0].Verb = "/corrupted_by_test"
	again := GetVerbCorpus()
	if again[0].Verb == "/corrupted_by_test" {
		t.Fatal("GetVerbCorpus shares its backing array with callers")
	}
}

// TestIntent_ToFactKernelRoundTrip pins hostile-boundary behavior end to
// end: ToFact's MangleAtom slots land as /name constants and the fact
// asserts and reads back through a real kernel with Decl parity.
func TestIntent_ToFactKernelRoundTrip(t *testing.T) {
	intent := Intent{Category: "/mutation", Verb: "/edit", Target: "auth.go", Constraint: "none"}
	fact := intent.ToFact()
	if fact.Predicate != "user_intent" || len(fact.Args) != 5 {
		t.Fatalf("ToFact shape = %s/%d, want user_intent/5", fact.Predicate, len(fact.Args))
	}
	for i := 0; i < 3; i++ {
		if _, ok := fact.Args[i].(core.MangleAtom); !ok {
			t.Fatalf("arg %d = %T, want MangleAtom for /name slot", i, fact.Args[i])
		}
	}
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.Assert(fact); err != nil {
		t.Fatalf("Assert user_intent: %v", err)
	}
	rows, err := k.Query("user_intent")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("user_intent did not round-trip through the kernel")
	}
}
