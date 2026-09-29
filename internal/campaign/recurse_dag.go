package campaign

import "fmt"

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
