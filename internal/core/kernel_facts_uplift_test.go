package core

import (
	"context"
	"strings"
	"testing"
)

// squeezeKernel caps a booted kernel at its current size so the next
// distinct assert is rejected. Proves rejection paths without staging 250k
// facts.
func squeezeKernel(t *testing.T) *RealKernel {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	k.SetMaxFacts(k.FactCount())
	return k
}

// A rejected first heartbeat must surface: the shard would otherwise look
// dead while its reporter believes it is alive.
func TestHeartbeat_RejectionSurfaces(t *testing.T) {
	k := squeezeKernel(t)
	err := k.Assert(Fact{Predicate: "system_heartbeat", Args: []any{"shard-a", "t1"}})
	if err == nil {
		t.Fatal("expected EDB-limit rejection, got nil")
	}
	if !strings.Contains(err.Error(), "fact limit") {
		t.Fatalf("error does not name the limit: %v", err)
	}
}

func seedRetractKernel(t *testing.T) *RealKernel {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	k.AppendPolicy("Decl setting(Key, Value).")
	k.AppendPolicy("Decl other(X).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, f := range []Fact{
		{Predicate: "setting", Args: []any{"lang", "go"}},
		{Predicate: "setting", Args: []any{"lang", "python"}},
		{Predicate: "setting", Args: []any{"debug", "true"}},
		{Predicate: "other", Args: []any{"x"}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatalf("Assert %v: %v", f, err)
		}
	}
	return k
}

func queryCount(t *testing.T, k *RealKernel, pred string) int {
	t.Helper()
	rows, err := k.Query(pred)
	if err != nil {
		t.Fatalf("Query %s: %v", pred, err)
	}
	return len(rows)
}

// All five retract paths must agree on the surviving set, and derived reads
// must reflect the removal (not just the EDB slice).
func TestRetract_AllPathsAgree(t *testing.T) {
	cases := []struct {
		name        string
		retract     func(k *RealKernel) error
		wantRemoved int
		wantSet     int
	}{
		{"Retract", func(k *RealKernel) error { return k.Retract("setting") }, 3, 0},
		{"RetractFact", func(k *RealKernel) error {
			return k.RetractFact(Fact{Predicate: "setting", Args: []any{"lang"}})
		}, 2, 1},
		{"RetractExactFact", func(k *RealKernel) error {
			return k.RetractExactFact(Fact{Predicate: "setting", Args: []any{"lang", "go"}})
		}, 1, 2},
		{"RetractExactFactsBatch", func(k *RealKernel) error {
			return k.RetractExactFactsBatch([]Fact{
				{Predicate: "setting", Args: []any{"lang", "go"}},
				{Predicate: "other", Args: []any{"x"}},
			})
		}, 2, 2},
		{"RemoveFactsByPredicateSet", func(k *RealKernel) error {
			return k.RemoveFactsByPredicateSet(map[string]struct{}{"setting": {}})
		}, 3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := seedRetractKernel(t)
			baseline := k.FactCount()
			if err := tc.retract(k); err != nil {
				t.Fatalf("retract: %v", err)
			}
			if got, want := baseline-k.FactCount(), tc.wantRemoved; got != want {
				t.Fatalf("removed %d facts, want %d", got, want)
			}
			if got := queryCount(t, k, "setting"); got != tc.wantSet {
				t.Fatalf("Query(setting) = %d rows, want %d", got, tc.wantSet)
			}
			// The survivor must be re-assertable: proves the dedupe index
			// was rebuilt, not left holding removed keys.
			if err := k.Assert(Fact{Predicate: "setting", Args: []any{"lang", "go"}}); err != nil {
				t.Fatalf("re-assert after retract: %v", err)
			}
		})
	}
}

// When critical_path_prefix facts cannot land, the Dreamer must refuse to
// simulate rather than run blind (every guarded-deletion rule would match
// nothing and report safe).
func TestDreamer_RefusesBlindSimulation(t *testing.T) {
	k := squeezeKernel(t)
	d := NewDreamer(k)
	res := d.SimulateAction(context.Background(), ActionRequest{Type: ActionDeleteFile, Target: ".git/config"})
	if !res.Unsafe {
		t.Fatal("expected refusal (Unsafe), got safe verdict on blind dreamer")
	}
	if !strings.Contains(res.Reason, "critical path") {
		t.Fatalf("reason does not name critical paths: %q", res.Reason)
	}
}

// Healthy control: the gate must not false-refuse a normal dreamer.
func TestDreamer_HealthySimulates(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	d := NewDreamer(k)
	res := d.SimulateAction(context.Background(), ActionRequest{Type: ActionReadFile, Target: "safe_file.txt"})
	if res.Unsafe {
		t.Fatalf("healthy dreamer refused safe action: %q", res.Reason)
	}
}
