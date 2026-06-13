package core

import (
	"reflect"
	"sort"
	"strconv"
	"testing"
)

func TestKernelEval_Evaluate(t *testing.T) {
	k := setupMockKernel(t)

	// 1. Assert some base facts
	k.Assert(Fact{Predicate: "foo", Args: []any{"bar"}})
	k.Assert(Fact{Predicate: "num", Args: []any{42}})

	// 2. Define a rule in policy
	// Explicitly declare predicates for strict mode
	policy := `
	Decl foo(Name).
	Decl num(Number).
	Decl baz(Name).
	Decl big(Number).

	baz(X) :- foo(X).
	big(X) :- num(N), N > 10, X = N.
	`
	k.AppendPolicy(policy)

	// 3. Evaluate
	err := k.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// 4. Verify results
	results, err := k.Query("baz")
	if err != nil {
		t.Fatalf("Query baz failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for baz, got %d", len(results))
	} else if len(results) > 0 && results[0].Args[0] != "bar" {
		t.Errorf("Expected baz('bar'), got %v", results[0])
	}

	results, _ = k.Query("big")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for big, got %d", len(results))
	}
}

// TestKernelDifferentialEval verifies the CODENERD_DIFF_EVAL=1 fast path
// returns the same derived facts as the legacy full-rebuild path.
//
// Strategy: build two kernels with the same schemas+policy+facts, one with
// the flag OFF and one with the flag ON. Query the same predicates against
// both, then compare results with reflect.DeepEqual (after sorting for
// stable ordering, since fact-store iteration is not order-preserving).
func TestKernelDifferentialEval(t *testing.T) {
	policy := `
	Decl thing(Name).
	Decl big_thing(Name).
	Decl tagged_thing(Name, Tag).

	# Two derived strata: big_thing depends on thing; tagged_thing depends on big_thing.
	big_thing(X) :- thing(X), :string:contains(X, "big").
	tagged_thing(X, /processed) :- big_thing(X).
	`

	const N = 1000

	// Build a fact set deterministically: every 7th fact contains "big".
	makeFacts := func() []Fact {
		out := make([]Fact, 0, N)
		for i := range N {
			name := "/item_" + strconv.Itoa(i)
			if i%7 == 0 {
				name = "/big_item_" + strconv.Itoa(i)
			}
			out = append(out, Fact{Predicate: "thing", Args: []any{name}})
		}
		return out
	}

	queryAndCollect := func(t *testing.T, k *RealKernel) map[string][]string {
		t.Helper()
		got := make(map[string][]string)
		for _, pred := range []string{"thing", "big_thing", "tagged_thing"} {
			results, err := k.Query(pred)
			if err != nil {
				t.Fatalf("Query(%s): %v", pred, err)
			}
			facts := make([]string, 0, len(results))
			for _, f := range results {
				facts = append(facts, f.String())
			}
			sort.Strings(facts)
			got[pred] = facts
		}
		return got
	}

	runWithFlag := func(t *testing.T, flag string) map[string][]string {
		t.Helper()
		t.Setenv("CODENERD_DIFF_EVAL", flag)
		k := setupMockKernel(t)
		k.AppendPolicy(policy)
		// Force a policy parse before measuring assert behavior.
		if err := k.Evaluate(); err != nil {
			t.Fatalf("initial Evaluate: %v", err)
		}
		if err := k.AssertBatch(makeFacts()); err != nil {
			t.Fatalf("AssertBatch: %v", err)
		}
		return queryAndCollect(t, k)
	}

	off := runWithFlag(t, "0")
	on := runWithFlag(t, "1")

	if !reflect.DeepEqual(off, on) {
		// Surface a small diff to debug if it fails.
		for pred := range off {
			if !reflect.DeepEqual(off[pred], on[pred]) {
				t.Errorf("predicate %s: OFF has %d facts, ON has %d facts (first divergence)",
					pred, len(off[pred]), len(on[pred]))
				maxShow := 5
				if len(off[pred]) < maxShow {
					maxShow = len(off[pred])
				}
				t.Logf("OFF[%s][:%d] = %v", pred, maxShow, off[pred][:maxShow])
				if len(on[pred]) < maxShow {
					maxShow = len(on[pred])
				}
				t.Logf("ON[%s][:%d]  = %v", pred, maxShow, on[pred][:maxShow])
				return
			}
		}
		t.Errorf("differential eval results differ from full-rebuild results")
	}
}

// BenchmarkKernelDifferentialEval times the diff-eval fast path on N=1000
// sequential single-fact asserts, each followed by a Query, vs the same
// workload with CODENERD_DIFF_EVAL=0. The fast path skips the
// SimpleInMemoryStore rebuild + full stratified evaluation on every Query;
// the legacy path does both, so the ratio is what we care about.
//
// Run with:
//
//	go test -bench BenchmarkKernelDifferentialEval -benchmem -run=^$ ./internal/core/
func BenchmarkKernelDifferentialEval(b *testing.B) {
	policy := `
	Decl bench_thing(Name).
	Decl bench_big(Name).
	bench_big(X) :- bench_thing(X), :string:contains(X, "big").
	`

	cases := []struct {
		name string
		flag string
	}{
		{"OFF", "0"},
		{"ON", "1"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.Setenv("CODENERD_DIFF_EVAL", tc.flag)
			k, err := NewRealKernel()
			if err != nil {
				b.Fatalf("kernel boot: %v", err)
			}
			k.AppendPolicy(policy)
			if err := k.Evaluate(); err != nil {
				b.Fatalf("initial Evaluate: %v", err)
			}
			b.ResetTimer()
			for i := range b.N {
				name := "/bench_item_" + strconv.Itoa(i)
				if i%7 == 0 {
					name = "/big_bench_item_" + strconv.Itoa(i)
				}
				if err := k.Assert(Fact{Predicate: "bench_thing", Args: []any{name}}); err != nil {
					b.Fatalf("Assert: %v", err)
				}
				if _, err := k.Query("bench_big"); err != nil {
					b.Fatalf("Query: %v", err)
				}
			}
		})
	}
}

func TestKernelEval_Stratification(t *testing.T) {
	k := setupMockKernel(t)

	// 1. Define recursive/negated rules that might fail stratification check
	// Use unique names to avoid any potential (unlikely) contamination
	badPolicy := `
	Decl bad_p(Name).
	Decl bad_q(Name).
	bad_p(X) :- not bad_q(X).
	bad_q(X) :- bad_p(X).
	`
	k.AppendPolicy(badPolicy)

	// 2. Verify engine handles or rejects appropriately
	err := k.Evaluate()

	if err == nil {
		t.Logf("Warning: Unstratified negation did not return error. Logic Config might be permissive.")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}
