package core

import (
	"fmt"
	"testing"
)

const orderedPolicy = `
	Decl od_a(K).
	Decl od_b(K).
	Decl od_c(K).
	Decl od_blocked(K).
	Decl od_join(K).
	Decl od_open(K).
	od_join(K) :- od_a(K), od_b(K), od_c(K).
	od_open(K) :- od_a(K), od_b(K), !od_blocked(K).
`

// A write that adds rows to every premise of a rule derives each new row
// once. Unordered semi-naive variants read the full new state at every
// other premise, so a row whose three premises were all inserted came out
// of all three variants: three derivations per row, deduplicated later.
// Ordered variants (dredBefore) read the state without the change before
// their delta premise, so the variant of the first changed premise is the
// only one that finds it. The negated case: od_open's rows become true
// because od_a and od_b rows arrive while od_blocked rows go, all at once.
func TestDRedOrderedVariantsDeriveEachRowOnce(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), orderedPolicy)
	const n = 300
	for i := 0; i < n; i++ {
		key := MangleAtom(fmt.Sprintf("/k%d", i))
		for _, p := range []string{"od_blocked"} {
			if err := k.AssertWithoutEval(Fact{Predicate: p, Args: []any{key}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := k.EvaluatePending(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < n; i++ {
		key := MangleAtom(fmt.Sprintf("/k%d", i))
		for _, p := range []string{"od_a", "od_b", "od_c"} {
			if err := k.AssertWithoutEval(Fact{Predicate: p, Args: []any{key}}); err != nil {
				t.Fatal(err)
			}
		}
		if err := k.RetractExactFact(Fact{Predicate: "od_blocked", Args: []any{key}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.EvaluatePending(); err != nil {
		t.Fatal(err)
	}
	ev := k.LastEvaluation()
	if ev.Mode != "dred" {
		t.Fatalf("mode = %q (fallback %q), want dred", ev.Mode, ev.ConeFallback)
	}
	// n od_join rows plus n od_open rows, each found once.
	if ev.Derivations != 2*n {
		t.Fatalf("derivations = %d for %d new rows; each row must be derived once (profile %s)", ev.Derivations, 2*n, ev.Profile)
	}
	for _, pred := range []string{"od_join", "od_open"} {
		got, err := k.Query(pred)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != n {
			t.Fatalf("%s holds %d rows, want %d", pred, len(got), n)
		}
	}
}
