package world

import (
	"fmt"
	"slices"

	"codenerd/internal/types"
)

// Import-graph measurements.
//
// Two reviewer findings need facts about the whole import graph that a rule
// cannot derive cheaply. Measured on this repository's 162k file import
// edges (2026-10-29): "a test file imports no project file" enumerated every
// edge of every test file (0.56 s per evaluation), and file-level
// reachability, which import-cycle detection needs, passes 3,000,000 facts.
// Go measures both once, where the edges are built, and the rules read the
// results:
//
//	imports_project_file(File)        File has at least one resolved
//	                                  in-workspace import edge (per file,
//	                                  emitted with the edges, scanner-owned)
//	import_cycle_member(File, Root)   File lies on an import cycle: a strongly
//	                                  connected component of two or more files,
//	                                  named by its smallest path (whole-graph,
//	                                  recomputed whenever the edges change)

// ImportCycleFacts measures the import cycles among the dependency_link rows
// in facts. Only file-to-file edges can close a cycle: an external import's
// pkg:/mod: token has no outgoing edge.
func ImportCycleFacts(facts []types.Fact) []types.Fact {
	succ := make(map[string][]string)
	for _, f := range facts {
		if f.Predicate != "dependency_link" || len(f.Args) < 2 {
			continue
		}
		from, ok1 := f.Args[0].(string)
		to, ok2 := f.Args[1].(string)
		if !ok1 || !ok2 || from == "" || to == "" || from == to {
			continue
		}
		succ[from] = append(succ[from], to)
	}
	var out []types.Fact
	for _, comp := range stronglyConnected(succ) {
		if len(comp) < 2 {
			continue
		}
		slices.Sort(comp)
		for _, file := range comp {
			out = append(out, types.Fact{Predicate: "import_cycle_member", Args: []any{file, comp[0]}})
		}
	}
	return out
}

// stronglyConnected returns the strongly connected components of a directed
// graph (iterative Tarjan, so a long import chain cannot exhaust the stack).
// Nodes are visited in sorted order, so the result is deterministic.
func stronglyConnected(succ map[string][]string) [][]string {
	nodes := make([]string, 0, len(succ))
	for n := range succ {
		nodes = append(nodes, n)
	}
	slices.Sort(nodes)
	index := make(map[string]int)
	low := make(map[string]int)
	onStack := make(map[string]bool)
	var stack []string
	var comps [][]string
	next := 0
	type frame struct {
		node string
		edge int
	}
	for _, root := range nodes {
		if _, seen := index[root]; seen {
			continue
		}
		work := []frame{{node: root}}
		index[root], low[root] = next, next
		next++
		stack = append(stack, root)
		onStack[root] = true
		for len(work) > 0 {
			top := &work[len(work)-1]
			if top.edge < len(succ[top.node]) {
				w := succ[top.node][top.edge]
				top.edge++
				if _, seen := index[w]; !seen {
					index[w], low[w] = next, next
					next++
					stack = append(stack, w)
					onStack[w] = true
					work = append(work, frame{node: w})
				} else if onStack[w] {
					low[top.node] = min(low[top.node], index[w])
				}
				continue
			}
			v := top.node
			work = work[:len(work)-1]
			if len(work) > 0 {
				parent := work[len(work)-1].node
				low[parent] = min(low[parent], low[v])
			}
			if low[v] == index[v] {
				var comp []string
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					comp = append(comp, w)
					if w == v {
						break
					}
				}
				comps = append(comps, comp)
			}
		}
	}
	return comps
}

// refreshImportCycles re-measures import_cycle_member from the kernel's
// current dependency_link rows and replaces it.
// Unchanged measurements write nothing.
func refreshImportCycles(kernel types.Kernel) error {
	edges, err := kernel.Query("dependency_link")
	if err != nil {
		return fmt.Errorf("import cycles: query dependency_link: %w", err)
	}
	held, err := kernel.Query("import_cycle_member")
	if err != nil {
		return fmt.Errorf("import cycles: query import_cycle_member: %w", err)
	}
	cycles := ImportCycleFacts(edges)
	if sameFactRows(held, cycles) {
		return nil
	}
	if len(held) > 0 {
		if err := kernel.Retract("import_cycle_member"); err != nil {
			return fmt.Errorf("import cycles: retract: %w", err)
		}
	}
	if len(cycles) == 0 {
		return nil
	}
	return kernel.LoadFacts(cycles)
}

// sameFactRows reports whether two fact lists hold the same rows.
func sameFactRows(a, b []types.Fact) bool {
	if len(a) != len(b) {
		return false
	}
	count := make(map[string]int, len(a))
	for _, f := range a {
		count[fmt.Sprintf("%q", f.Args)]++
	}
	for _, f := range b {
		key := fmt.Sprintf("%q", f.Args)
		if count[key] == 0 {
			return false
		}
		count[key]--
	}
	return true
}

// importEdgesChanged reports whether a delta scan changed the import graph:
// the dependency_link rows it retracts and the ones it asserts differ. A
// re-parsed file whose imports did not change re-emits the same rows.
func importEdgesChanged(retracted, added []types.Fact) bool {
	set := func(facts []types.Fact) map[string]int {
		out := make(map[string]int)
		for _, f := range facts {
			if f.Predicate == "dependency_link" {
				out[fmt.Sprintf("%q", f.Args)]++
			}
		}
		return out
	}
	before, after := set(retracted), set(added)
	if len(before) != len(after) {
		return true
	}
	for k, n := range before {
		if after[k] != n {
			return true
		}
	}
	return false
}
