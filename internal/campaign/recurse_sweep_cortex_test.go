package campaign_test

import (
	"testing"

	"codenerd/internal/core"
	nerdsystem "codenerd/internal/system"
	"codenerd/internal/types"
)

// The sweep's next node, pass completion and run stop on the production
// kernel: the domain shards the factory boots, where recurse.mg's rules fire
// only if their facts share a shard. The same facts go to a single-store
// RealKernel. A diamond (b and c both depend on a, d on both) must visit a,
// then b before c, then d. Nodes waiting on a dependency are
// recurse_sweep_stuck until it is visited; the pass completes only once d is.
func TestRecurseSweep_OnProductionCortex(t *testing.T) {
	ck, err := nerdsystem.NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	single, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	both := recursePair{t: t, cortex: ck, single: single}

	both.assert(
		core.Fact{Predicate: "subsystem_node", Args: []interface{}{"a"}},
		core.Fact{Predicate: "subsystem_node", Args: []interface{}{"b"}},
		core.Fact{Predicate: "subsystem_node", Args: []interface{}{"c"}},
		core.Fact{Predicate: "subsystem_node", Args: []interface{}{"d"}},
		core.Fact{Predicate: "subsystem_node_ord", Args: []interface{}{"a", 0}},
		core.Fact{Predicate: "subsystem_node_ord", Args: []interface{}{"b", 1}},
		core.Fact{Predicate: "subsystem_node_ord", Args: []interface{}{"c", 2}},
		core.Fact{Predicate: "subsystem_node_ord", Args: []interface{}{"d", 3}},
		core.Fact{Predicate: "subsystem_depends", Args: []interface{}{"b", "a"}},
		core.Fact{Predicate: "subsystem_depends", Args: []interface{}{"c", "a"}},
		core.Fact{Predicate: "subsystem_depends", Args: []interface{}{"d", "b"}},
		core.Fact{Predicate: "subsystem_depends", Args: []interface{}{"d", "c"}},
		core.Fact{Predicate: "recurse_current_pass", Args: []interface{}{0}},
	)
	// b, c and d are not ready: a dependency is still unvisited. That is
	// recurse_sweep_stuck. It is the cycle report only once nothing is next.
	both.want("recurse_next_node", "a")
	both.want("recurse_pass_complete")
	both.want("recurse_sweep_stuck", "b", "c", "d")

	both.assert(core.Fact{Predicate: "recurse_node_visited", Args: []interface{}{0, "a"}})
	both.want("recurse_next_node", "b")
	both.want("recurse_pass_complete")
	both.want("recurse_sweep_stuck", "d")

	both.assert(core.Fact{Predicate: "recurse_node_visited", Args: []interface{}{0, "b"}})
	both.want("recurse_next_node", "c")
	both.want("recurse_sweep_stuck", "d")

	both.assert(
		core.Fact{Predicate: "recurse_node_visited", Args: []interface{}{0, "c"}},
		core.Fact{Predicate: "recurse_node_visited", Args: []interface{}{0, "d"}},
	)
	both.want("recurse_next_node")
	both.want("recurse_pass_complete", "0")
	both.want("recurse_sweep_stuck")

	// A positive budget stops the run once that many passes have finished.
	// Zero does not, however many finished. The facts are replaced: a second
	// budget left in place would be a second way for the rule to fire.
	both.replace([]string{"recurse_pass_budget", "recurse_passes_finished"},
		core.Fact{Predicate: "recurse_pass_budget", Args: []interface{}{0}},
		core.Fact{Predicate: "recurse_passes_finished", Args: []interface{}{9}},
	)
	both.wantStop(false)
	both.replace([]string{"recurse_pass_budget", "recurse_passes_finished"},
		core.Fact{Predicate: "recurse_pass_budget", Args: []interface{}{2}},
		core.Fact{Predicate: "recurse_passes_finished", Args: []interface{}{1}},
	)
	both.wantStop(false)
	both.replace([]string{"recurse_pass_budget", "recurse_passes_finished"},
		core.Fact{Predicate: "recurse_pass_budget", Args: []interface{}{2}},
		core.Fact{Predicate: "recurse_passes_finished", Args: []interface{}{2}},
	)
	both.wantStop(true)
}

func (p recursePair) replace(preds []string, facts ...core.Fact) {
	p.t.Helper()
	set := make(map[string]struct{}, len(preds))
	for _, pred := range preds {
		set[pred] = struct{}{}
	}
	if err := p.cortex.RemoveFactsByPredicateSet(set); err != nil {
		p.t.Fatalf("cortex retract: %v", err)
	}
	if err := p.single.RemoveFactsByPredicateSet(set); err != nil {
		p.t.Fatalf("single retract: %v", err)
	}
	p.assert(facts...)
}

// wantStop compares recurse_run_stop, which has no column for recurseColumn.
func (p recursePair) wantStop(want bool) {
	p.t.Helper()
	for name, k := range map[string]core.Kernel{"cortex": p.cortex, "single": p.single} {
		rows, err := k.Query("recurse_run_stop")
		if err != nil {
			p.t.Fatalf("%s query recurse_run_stop: %v", name, err)
		}
		if (len(rows) > 0) != want {
			p.t.Fatalf("%s recurse_run_stop = %v, want %v (%s)", name, len(rows) > 0, want, types.ExtractString(rows))
		}
	}
}
