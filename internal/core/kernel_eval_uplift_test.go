package core

import (
	"fmt"
	"sort"
	"testing"
)

const evalParityPolicy = `
Decl thing(Name).
Decl big_thing(Name).
Decl tagged_thing(Name, Tag).

big_thing(X) :- thing(X).
tagged_thing(X, /processed) :- big_thing(X).
`

func evalParitySeed(t *testing.T, k *RealKernel, n int) {
	t.Helper()
	k.AppendPolicy(evalParityPolicy)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	facts := make([]Fact, 0, n)
	for i := 0; i < n; i++ {
		facts = append(facts, Fact{Predicate: "thing", Args: []any{fmt.Sprintf("t%d", i)}})
	}
	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("AssertBatch: %v", err)
	}
}

func evalParityCapture(t *testing.T, k *RealKernel) []string {
	t.Helper()
	var rows []string
	for _, pred := range []string{"thing", "big_thing", "tagged_thing"} {
		got, err := k.Query(pred)
		if err != nil {
			t.Fatalf("Query %s: %v", pred, err)
		}
		for _, f := range got {
			rows = append(rows, f.String())
		}
	}
	sort.Strings(rows)
	return rows
}

// The differential fast path must derive exactly what the full fixpoint
// derives, and LastEvaluation must honestly report which path ran.
func TestEval_DiffFullParity(t *testing.T) {
	captured := map[string][]string{}
	modes := map[string]string{}
	for _, tc := range []struct{ name, flag, wantMode string }{
		{"differential", "1", "differential"},
		{"full", "0", "full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CODENERD_DIFF_EVAL", tc.flag)
			k, err := NewRealKernel()
			if err != nil {
				t.Fatalf("boot: %v", err)
			}
			evalParitySeed(t, k, 200)
			captured[tc.name] = evalParityCapture(t, k)
			modes[tc.name] = k.LastEvaluation().Mode
		})
	}
	if modes["differential"] != "differential" {
		t.Fatalf("flag=1 ran %q, want differential path", modes["differential"])
	}
	if modes["full"] != "full" {
		t.Fatalf("flag=0 ran %q, want full path", modes["full"])
	}
	a, b := captured["differential"], captured["full"]
	if len(a) != len(b) {
		t.Fatalf("row count differs: diff=%d full=%d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("row %d differs:\n diff: %s\n full: %s", i, a[i], b[i])
		}
	}
	if len(a) == 0 {
		t.Fatal("parity over zero rows proves nothing")
	}
}

// Clone must carry config (limits, sandbox, demotion), own a live event
// bus, and evaluate independently of its parent.
func TestClone_FidelityAndIsolation(t *testing.T) {
	parent, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	parent.SetDerivedFactLimit(12345)
	parent.sandbox = true
	parent.diffPathDemoted = true
	parent.AppendPolicy("Decl clone_iso_probe(X).")
	if err := parent.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	clone := parent.Clone()
	if got := clone.GetDerivedFactLimit(); got != 12345 {
		t.Errorf("clone limit = %d, want 12345", got)
	}
	if !clone.sandbox {
		t.Error("clone lost the sandbox marker")
	}
	if !clone.diffPathDemoted {
		t.Error("clone lost the measured demotion")
	}
	if clone.GetEventBus() == nil {
		t.Error("clone has nil event bus")
	}

	// Isolation: a fact asserted only on the clone must derive only there.
	if err := clone.Assert(Fact{Predicate: "clone_iso_probe", Args: []any{"only"}}); err != nil {
		t.Fatalf("clone Assert: %v", err)
	}
	cloneRows, err := clone.Query("clone_iso_probe")
	if err != nil {
		t.Fatalf("clone Query: %v", err)
	}
	if len(cloneRows) != 1 {
		t.Fatalf("clone sees %d rows, want 1", len(cloneRows))
	}
	parentRows, err := parent.Query("clone_iso_probe")
	if err != nil {
		t.Fatalf("parent Query: %v", err)
	}
	if len(parentRows) != 0 {
		t.Fatalf("parent sees %d clone-only rows, want 0", len(parentRows))
	}
}

// Reads after Clear/Reset must lazily re-evaluate to empty results —
// not error with "kernel not initialized".
func TestClearReset_ReadAfterClear(t *testing.T) {
	for _, tc := range []struct {
		name  string
		clear func(k *RealKernel)
	}{
		{"Clear", func(k *RealKernel) { k.Clear() }},
		{"Reset", func(k *RealKernel) { k.Reset() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, err := NewRealKernel()
			if err != nil {
				t.Fatalf("boot: %v", err)
			}
			k.AppendPolicy("Decl clear_probe(X).")
			if err := k.Evaluate(); err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if err := k.Assert(Fact{Predicate: "clear_probe", Args: []any{"v"}}); err != nil {
				t.Fatalf("Assert: %v", err)
			}
			tc.clear(k)
			if got := k.FactCount(); got != 0 {
				t.Fatalf("FactCount after %s = %d, want 0", tc.name, got)
			}
			rows, err := k.Query("clear_probe")
			if err != nil {
				t.Fatalf("Query after %s: %v", tc.name, err)
			}
			if len(rows) != 0 {
				t.Fatalf("Query after %s = %d rows, want 0", tc.name, len(rows))
			}
			// The other read funnels share the self-heal branch.
			cbCount := 0
			if err := k.QueryCallback("clear_probe", func(Fact) error { cbCount++; return nil }); err != nil {
				t.Fatalf("QueryCallback after %s: %v", tc.name, err)
			}
			if cbCount != 0 {
				t.Fatalf("QueryCallback after %s visited %d rows, want 0", tc.name, cbCount)
			}
			all, err := k.QueryAll()
			if err != nil {
				t.Fatalf("QueryAll after %s: %v", tc.name, err)
			}
			if len(all["clear_probe"]) != 0 {
				t.Fatalf("QueryAll after %s has %d clear_probe rows, want 0", tc.name, len(all["clear_probe"]))
			}
			if !k.IsInitialized() {
				t.Fatalf("kernel not initialized after read following %s", tc.name)
			}
		})
	}
}

// ClearSchemas must drop learned rules with the schemas: kept learned
// rules reference cleared predicates and would fail the next analysis.
func TestClearSchemas_DropsLearned(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	k.mu.RLock()
	hadLearned := k.learned != ""
	k.mu.RUnlock()
	if !hadLearned {
		t.Skip("no embedded learned rules in this build; nothing to prove")
	}
	k.ClearSchemas()
	k.mu.RLock()
	schemas, policy, learned := k.schemas, k.policy, k.learned
	k.mu.RUnlock()
	if schemas != "" || policy != "" || learned != "" {
		t.Fatalf("after ClearSchemas: schemas=%d policy=%d learned=%d bytes, want all empty",
			len(schemas), len(policy), len(learned))
	}
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate after ClearSchemas: %v", err)
	}
}
