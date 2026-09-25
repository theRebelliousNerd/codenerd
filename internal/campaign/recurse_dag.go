package campaign

import (
	"fmt"
	"sort"
)

// The recurse subsystem DAG: the order a self-improvement sweep attacks the
// codebase. Lowest base primitives first (the logic everything stands on),
// then up through the layers each one depends on, then across (integration
// wiring), then review (architectural read-back), then benchmarks (prove the
// gains). A wave that hardened the CLI before the kernel it drives would be
// testing conclusions before premises.
//
// Derived, not curated (recurse_workspace.go). The DAG was a hand-written
// table of codeNERD's own packages, which made recurse useless in any other
// workspace; it is now the workspace's own import graph.

// SubsystemNode is one sweep target: a slice of the tree plus the nodes whose
// soundness it assumes.
type SubsystemNode struct {
	// ID is stable; waves, findings, and resume logic key on it.
	ID string
	// Title names the node in phase names and reports.
	Title string
	// Paths are workspace-relative roots the node's tasks may mutate. Empty
	// means the whole tree (cross-cutting nodes only).
	Paths []string
	// DependsOn lists node IDs that must sweep first within a wave.
	DependsOn []string
	// CrossCutting marks the across/review/benchmark nodes that close a wave.
	CrossCutting bool
	// Languages are the toolchains the node's files belong to ("go",
	// "python", "js/ts", "rust"); a node-scoped gate runs only on nodes of
	// its language.
	Languages []string
}

// TopoOrder sorts nodes so every dependency sweeps before its dependents. It
// fails closed on duplicate IDs, unknown dependencies, and cycles: a sweep
// plan with an unorderable graph is not a plan.
func TopoOrder(nodes []SubsystemNode) ([]SubsystemNode, error) {
	byID := make(map[string]SubsystemNode, len(nodes))
	for _, n := range nodes {
		if n.ID == "" {
			return nil, fmt.Errorf("recurse DAG: node with empty ID (%q)", n.Title)
		}
		if _, dup := byID[n.ID]; dup {
			return nil, fmt.Errorf("recurse DAG: duplicate node ID %q", n.ID)
		}
		byID[n.ID] = n
	}
	for _, n := range nodes {
		for _, dep := range n.DependsOn {
			if _, ok := byID[dep]; !ok {
				return nil, fmt.Errorf("recurse DAG: node %q depends on unknown node %q", n.ID, dep)
			}
			if dep == n.ID {
				return nil, fmt.Errorf("recurse DAG: node %q depends on itself", n.ID)
			}
		}
	}

	// Kahn's algorithm with lexicographic tie-breaks so the order is stable
	// no matter how the input slice is shuffled.
	indegree := make(map[string]int, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		indegree[n.ID] = len(n.DependsOn)
		for _, dep := range n.DependsOn {
			dependents[dep] = append(dependents[dep], n.ID)
		}
	}
	var ready []string
	for id, deg := range indegree {
		if deg == 0 {
			ready = append(ready, id)
		}
	}
	ordered := make([]SubsystemNode, 0, len(nodes))
	for len(ready) > 0 {
		sort.Strings(ready)
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byID[id])
		for _, dep := range dependents[id] {
			indegree[dep]--
			if indegree[dep] == 0 {
				ready = append(ready, dep)
			}
		}
	}
	if len(ordered) != len(nodes) {
		var stuck []string
		for id, deg := range indegree {
			if deg > 0 {
				stuck = append(stuck, id)
			}
		}
		sort.Strings(stuck)
		return nil, fmt.Errorf("recurse DAG: dependency cycle involving %v", stuck)
	}
	return ordered, nil
}

// FilterDAG keeps the named subsystems plus their transitive dependencies, so
// a focused sweep still sweeps premises before conclusions. Unknown names fail
// closed.
func FilterDAG(nodes []SubsystemNode, keep []string) ([]SubsystemNode, error) {
	if len(keep) == 0 {
		return nodes, nil
	}
	byID := make(map[string]SubsystemNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	wanted := make(map[string]bool, len(nodes))
	var visit func(id string) error
	visit = func(id string) error {
		n, ok := byID[id]
		if !ok {
			return fmt.Errorf("recurse DAG: unknown subsystem %q", id)
		}
		if wanted[id] {
			return nil
		}
		wanted[id] = true
		for _, dep := range n.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		return nil
	}
	for _, id := range keep {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	filtered := make([]SubsystemNode, 0, len(wanted))
	for _, n := range nodes {
		if wanted[n.ID] {
			filtered = append(filtered, n)
		}
	}
	return filtered, nil
}
