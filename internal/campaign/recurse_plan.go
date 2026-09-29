package campaign

import (
	"context"
	"fmt"
	"sort"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// RecurseConfig is the operator-facing shape of a recurse run, from the CLI
// flags or the /recurse arguments. Every field is settable from flags; the
// CLI coverage test enforces that.
type RecurseConfig struct {
	// MaxWaves bounds the run in passes over the DAG. Zero means unbounded,
	// which the CLIs only allow with yolo mode.
	MaxWaves int `json:"max_waves"`
	// Subsystems focuses the sweep; dependencies are pulled in automatically.
	Subsystems []string `json:"subsystems,omitzero"`
	// ContextBudget overrides each attempt campaign's context budget. Zero
	// keeps the default.
	ContextBudget int `json:"context_budget"`
}

// Normalize validates the configuration.
func (c RecurseConfig) Normalize() (RecurseConfig, error) {
	if c.MaxWaves < 0 {
		return c, fmt.Errorf("recurse: passes must be >= 0 (0 = unbounded with yolo)")
	}
	if c.ContextBudget < 0 {
		return c, fmt.Errorf("recurse: context budget must be >= 0")
	}
	return c, nil
}

// RecurseSweepOrder derives the workspace's DAG and returns it bottom to top,
// narrowed to subsystems (and what they depend on) when any are named. The
// order is what policy/recurse.mg derives (recurse_next_node), which is also
// the order a pass visits and the order `nerd campaign recurse --plan` prints.
func RecurseSweepOrder(ctx context.Context, workspace string, subsystems []string) ([]SubsystemNode, error) {
	derived, err := deriveRecurseDAG(ctx, workspace)
	if err != nil {
		return nil, err
	}
	// A planning kernel: the run's own kernel is not this one, and
	// policySweepOrder retracts every visit fact it uses.
	k, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	// Ordering the whole graph first is what rejects a cycle anywhere in it,
	// including outside a focused sweep.
	ordered, err := policySweepOrder(k, derived)
	if err != nil {
		return nil, err
	}
	if len(subsystems) == 0 {
		return ordered, nil
	}
	focused, err := FilterDAG(derived, subsystems)
	if err != nil {
		return nil, err
	}
	return policySweepOrder(k, focused)
}

// sweepProbePass is the pass number a soundness walk visits under. Real passes
// are >= 0 (a resumed run starts at 0), so the probe cannot be mistaken for
// one of them, and its visits are retracted before the real pass starts.
const sweepProbePass = -1

// The sweep graph, replaced each pass. Visits are not in this set: a pass
// number distinguishes them, and the run keeps every pass's visits.
var sweepGraphFacts = map[string]struct{}{
	"subsystem_node":     {},
	"subsystem_depends":  {},
	"subsystem_node_ord": {},
}

// validateSweepGraph rejects a measurement that cannot be asserted as the
// facts the sweep rules join. A cycle is not decided here: the walk asks the
// kernel, and a graph with no ready node is recurse_sweep_stuck.
func validateSweepGraph(nodes []SubsystemNode) error {
	seen := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if n.ID == "" {
			return fmt.Errorf("recurse DAG: node with empty ID (%q)", n.Title)
		}
		if seen[n.ID] {
			return fmt.Errorf("recurse DAG: duplicate node ID %q", n.ID)
		}
		seen[n.ID] = true
	}
	for _, n := range nodes {
		for _, dep := range n.DependsOn {
			if !seen[dep] {
				return fmt.Errorf("recurse DAG: node %q depends on unknown node %q", n.ID, dep)
			}
			if dep == n.ID {
				return fmt.Errorf("recurse DAG: node %q depends on itself", n.ID)
			}
		}
	}
	return nil
}

// sweepGraphFactList is the nodes, their dependency edges, and each node's
// lexicographic identity key. The key is not an order: it is the id's rank
// among the ids, which recurse_next_node minimizes over the nodes whose
// dependencies are already visited.
func sweepGraphFactList(nodes []SubsystemNode) []core.Fact {
	ranked := make([]string, len(nodes))
	for i, n := range nodes {
		ranked[i] = n.ID
	}
	sort.Strings(ranked)
	ord := make(map[string]int, len(ranked))
	for i, id := range ranked {
		ord[id] = i
	}
	facts := make([]core.Fact, 0, len(nodes)*2)
	for _, n := range nodes {
		facts = append(facts,
			core.Fact{Predicate: "subsystem_node", Args: []interface{}{n.ID}},
			core.Fact{Predicate: "subsystem_node_ord", Args: []interface{}{n.ID, ord[n.ID]}},
		)
		for _, dep := range n.DependsOn {
			facts = append(facts, core.Fact{Predicate: "subsystem_depends", Args: []interface{}{n.ID, dep}})
		}
	}
	return facts
}

func assertSweepGraph(k core.Kernel, nodes []SubsystemNode) error {
	if err := validateSweepGraph(nodes); err != nil {
		return err
	}
	if err := k.RemoveFactsByPredicateSet(sweepGraphFacts); err != nil {
		return fmt.Errorf("recurse: clear sweep graph: %w", err)
	}
	facts := sweepGraphFactList(nodes)
	if len(facts) == 0 {
		return nil
	}
	if err := k.AssertBatch(facts); err != nil {
		return fmt.Errorf("recurse: assert sweep graph: %w", err)
	}
	return nil
}

func assertSweepPass(k core.Kernel, pass int) error {
	if err := k.RemoveFactsByPredicateSet(map[string]struct{}{"recurse_current_pass": {}}); err != nil {
		return fmt.Errorf("recurse: clear sweep pass: %w", err)
	}
	if err := k.Assert(core.Fact{Predicate: "recurse_current_pass", Args: []interface{}{pass}}); err != nil {
		return fmt.Errorf("recurse: assert sweep pass: %w", err)
	}
	return nil
}

func assertNodeVisited(k core.Kernel, pass int, id string) error {
	if err := k.Assert(core.Fact{Predicate: "recurse_node_visited", Args: []interface{}{pass, id}}); err != nil {
		return fmt.Errorf("recurse: assert visit: %w", err)
	}
	return nil
}

// assertSweepBudget replaces the budget and how many passes this invocation
// has finished. The rule reads one of each; leaving the previous count would
// make a spent budget stay spent.
func assertSweepBudget(k core.Kernel, budget, finished int) error {
	if err := k.RemoveFactsByPredicateSet(map[string]struct{}{
		"recurse_pass_budget":     {},
		"recurse_passes_finished": {},
	}); err != nil {
		return fmt.Errorf("recurse: clear sweep budget: %w", err)
	}
	if err := k.AssertBatch([]core.Fact{
		{Predicate: "recurse_pass_budget", Args: []interface{}{budget}},
		{Predicate: "recurse_passes_finished", Args: []interface{}{finished}},
	}); err != nil {
		return fmt.Errorf("recurse: assert sweep budget: %w", err)
	}
	return nil
}

func factColumn(rows []core.Fact, i int) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range rows {
		if len(f.Args) <= i {
			continue
		}
		v := types.ExtractString(f.Args[i])
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func sweepNextNode(k core.Kernel) (string, error) {
	rows, err := k.Query("recurse_next_node")
	if err != nil {
		return "", fmt.Errorf("recurse: query recurse_next_node: %w", err)
	}
	ids := factColumn(rows, 0)
	switch len(ids) {
	case 0:
		return "", nil
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("recurse: the kernel derived %d next nodes (%v); the policy must decide one", len(ids), ids)
	}
}

func sweepPassComplete(k core.Kernel, pass int) (bool, error) {
	rows, err := k.Query("recurse_pass_complete")
	if err != nil {
		return false, fmt.Errorf("recurse: query recurse_pass_complete: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) == 1 && argInt(f.Args[0]) == pass {
			return true, nil
		}
	}
	return false, nil
}

func sweepRunStop(k core.Kernel) (bool, error) {
	rows, err := k.Query("recurse_run_stop")
	if err != nil {
		return false, fmt.Errorf("recurse: query recurse_run_stop: %w", err)
	}
	return len(rows) > 0, nil
}

func sweepStuckError(k core.Kernel) error {
	rows, err := k.Query("recurse_sweep_stuck")
	if err != nil {
		return fmt.Errorf("recurse: query recurse_sweep_stuck: %w", err)
	}
	ids := factColumn(rows, 0)
	if len(ids) == 0 {
		return fmt.Errorf("recurse: the sweep has nodes left but the kernel named none next and none stuck")
	}
	return fmt.Errorf("recurse DAG: dependency cycle involving %v", ids)
}

func retractVisited(k core.Kernel, pass int, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	facts := make([]core.Fact, len(ids))
	for i, id := range ids {
		facts[i] = core.Fact{Predicate: "recurse_node_visited", Args: []interface{}{pass, id}}
	}
	if err := k.RetractExactFactsBatch(facts); err != nil {
		return fmt.Errorf("recurse: retract probe visits: %w", err)
	}
	return nil
}

// policySweepOrder asks the kernel for the sweep order of nodes. It retracts
// every recurse_node_visited fact first, so it is for a kernel that is not
// holding a run's visit history.
func policySweepOrder(k core.Kernel, nodes []SubsystemNode) ([]SubsystemNode, error) {
	if err := k.RemoveFactsByPredicateSet(map[string]struct{}{"recurse_node_visited": {}}); err != nil {
		return nil, fmt.Errorf("recurse: clear sweep visits: %w", err)
	}
	if err := assertSweepGraph(k, nodes); err != nil {
		return nil, err
	}
	if err := assertSweepPass(k, 0); err != nil {
		return nil, err
	}
	byID := make(map[string]SubsystemNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	ordered := make([]SubsystemNode, 0, len(nodes))
	for {
		done, err := sweepPassComplete(k, 0)
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
		id, err := sweepNextNode(k)
		if err != nil {
			return nil, err
		}
		if id == "" {
			return nil, sweepStuckError(k)
		}
		n, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("recurse: the kernel's next node %q is not in this sweep", id)
		}
		ordered = append(ordered, n)
		if err := assertNodeVisited(k, 0, id); err != nil {
			return nil, err
		}
	}
	if len(ordered) != len(nodes) {
		return nil, fmt.Errorf("recurse: sweep ordered %d of %d nodes", len(ordered), len(nodes))
	}
	return ordered, nil
}

// sweepGraphSound asks the kernel to order nodes and returns an error when it
// cannot, before a pass attempts anything. The probe's visits are retracted;
// the graph facts stay asserted.
func sweepGraphSound(k core.Kernel, nodes []SubsystemNode) error {
	if err := assertSweepGraph(k, nodes); err != nil {
		return err
	}
	if err := assertSweepPass(k, sweepProbePass); err != nil {
		return err
	}
	var seen []string
	for {
		done, err := sweepPassComplete(k, sweepProbePass)
		if err != nil {
			return err
		}
		if done {
			break
		}
		id, err := sweepNextNode(k)
		if err != nil {
			return err
		}
		if id == "" {
			return sweepStuckError(k)
		}
		if err := assertNodeVisited(k, sweepProbePass, id); err != nil {
			return err
		}
		seen = append(seen, id)
	}
	return retractVisited(k, sweepProbePass, seen)
}
