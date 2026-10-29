package campaign

import (
	"fmt"
	"sort"
)

// TopoOrder is the parity oracle for the sweep order policy derives
// (recurse.mg recurse_next_node): Kahn's algorithm with lexicographic
// tie-breaks, the order production used before the kernel decided it. It
// fails closed on duplicate IDs, unknown dependencies, and cycles.
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
