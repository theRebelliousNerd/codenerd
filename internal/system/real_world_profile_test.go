package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/world"
)

// TestRealWorldProfile loads this repository's own fast scan into the
// production kernel, gives it a turn's facts and prints what the policy
// derives from them: rows per predicate, largest first, and what each step
// cost. It is a measuring instrument, not a gate -- a relation that is large
// here is a rule whose output grows with the workspace, and the place to look
// for a product. Run with NERD_REAL_WORLD_PROFILE=1 (about a minute).
func TestRealWorldProfile(t *testing.T) {
	if os.Getenv("NERD_REAL_WORLD_PROFILE") == "" {
		t.Skip("set NERD_REAL_WORLD_PROFILE=1 to profile the policy over this repository's world")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	facts, err := world.NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	t.Logf("scan: %d facts in %s", len(facts), time.Since(start).Round(time.Millisecond))

	ck, err := NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	counts := func() map[string]int {
		all, err := ck.QueryAll()
		if err != nil {
			t.Fatalf("QueryAll: %v", err)
		}
		out := make(map[string]int, len(all))
		for pred, rows := range all {
			out[pred] = len(rows)
		}
		return out
	}
	step := func(name string, do func() error) map[string]int {
		before := counts()
		begin := time.Now()
		if err := do(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// Evaluation is deferred to the first read; the read is part of the cost.
		after := counts()
		took := time.Since(begin)
		type delta struct {
			pred string
			n    int
		}
		var grew []delta
		total := 0
		for pred, n := range after {
			if d := n - before[pred]; d != 0 {
				grew = append(grew, delta{pred, d})
				total += d
			}
		}
		sort.Slice(grew, func(i, j int) bool {
			if grew[i].n != grew[j].n {
				return grew[i].n > grew[j].n
			}
			return grew[i].pred < grew[j].pred
		})
		line := ""
		for i, d := range grew {
			if i == 30 {
				break
			}
			line += fmt.Sprintf("\n    %8d  %s", d.n, d.pred)
		}
		t.Logf("%s: %s, %+d rows over %d predicates%s", name, took.Round(time.Millisecond), total, len(grew), line)
		return after
	}

	step("load the world", func() error { return ck.LoadFacts(facts) })
	atom := func(s string) core.MangleAtom { return core.MangleAtom(s) }
	step("a turn's intent", func() error {
		return ck.Assert(core.Fact{Predicate: "user_intent", Args: []any{atom("/current_intent"), atom("/mutation"), atom("/fix"), "internal/core/kernel_eval.go", ""}})
	})
	step("a hub file modified", func() error {
		return ck.Assert(core.Fact{Predicate: "modified", Args: []any{"internal/core/kernel_eval.go"}})
	})
	step("a second file modified", func() error {
		return ck.Assert(core.Fact{Predicate: "modified", Args: []any{"internal/world/dependency_links.go"}})
	})
	step("the first modification retracted", func() error {
		return ck.RetractFact(core.Fact{Predicate: "modified", Args: []any{"internal/core/kernel_eval.go"}})
	})
}
