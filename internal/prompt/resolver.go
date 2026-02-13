package prompt

import (
	"fmt"
	"sort"

	"codenerd/internal/logging"
)

// OrderedAtom is an atom with its assembly order.
// The order determines the position in the final prompt.
type OrderedAtom struct {
	Atom       *PromptAtom
	Order      int
	Score      float64 // Preserved from ScoredAtom
	RenderMode string  // "standard", "concise", "min"
}

// DependencyResolver resolves atom dependencies and detects conflicts.
// It performs:
// 1. Dependency validation (all dependencies satisfied)
// 2. Conflict detection (no conflicting atoms)
// 3. Topological sorting (respecting dependencies)
// 4. Cycle detection (preventing infinite loops)
type DependencyResolver struct {
	// allowMissingDeps allows atoms with missing dependencies to be included
	allowMissingDeps bool
}

// NewDependencyResolver creates a new dependency resolver.
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{
		allowMissingDeps: false,
	}
}

// SetAllowMissingDeps controls whether atoms with missing dependencies are included.
func (r *DependencyResolver) SetAllowMissingDeps(allow bool) {
	r.allowMissingDeps = allow
}

// Resolve orders atoms by dependencies.
// It assumes the input set is already valid (filtered by Mangle/JIT compiler).
// It performs:
// 1. Topological sorting (respecting dependencies)
// 2. Cycle detection (preventing infinite loops)
func (r *DependencyResolver) Resolve(atoms []*ScoredAtom) ([]*OrderedAtom, error) {
	timer := logging.StartTimer(logging.CategoryContext, "DependencyResolver.Resolve")
	defer timer.Stop()

	if len(atoms) == 0 {
		return nil, nil
	}

	// Build lookup map
	atomMap := make(map[string]*ScoredAtom, len(atoms))
	valid := make([]*ScoredAtom, 0, len(atoms))
	for _, sa := range atoms {
		if sa == nil || sa.Atom == nil || sa.Atom.ID == "" {
			logging.Get(logging.CategoryContext).Warn("DependencyResolver.Resolve: skipping nil/invalid atom")
			continue
		}
		atomMap[sa.Atom.ID] = sa
		valid = append(valid, sa)
	}
	if len(valid) == 0 {
		return nil, nil
	}

	// Step 1: Topological sort
	// We rely on Mangle to have already filtered out prohibited/conflicting/missing-dep atoms.
	sorted, err := r.topologicalSort(valid, atomMap)
	if err != nil {
		return nil, err
	}

	// Step 2: Convert to OrderedAtom with sequential order
	result := make([]*OrderedAtom, len(sorted))
	for i, sa := range sorted {
		result[i] = &OrderedAtom{
			Atom:       sa.Atom,
			Order:      i,
			Score:      sa.Combined,
			RenderMode: "standard", // Default mode, can be adjusted by BudgetManager
		}
	}

	logging.Get(logging.CategoryContext).Debug(
		"Resolved %d atoms (from %d candidates)", len(result), len(atoms),
	)

	return result, nil
}

// topologicalSort orders atoms so dependencies come before dependents.
// Uses Kahn's algorithm with cycle detection.
func (r *DependencyResolver) topologicalSort(
	atoms []*ScoredAtom,
	atomMap map[string]*ScoredAtom,
) ([]*ScoredAtom, error) {
	// Build in-degree map (count of incoming edges)
	inDegree := make(map[string]int, len(atoms))
	for _, sa := range atoms {
		if _, ok := inDegree[sa.Atom.ID]; !ok {
			inDegree[sa.Atom.ID] = 0
		}
		for _, depID := range sa.Atom.DependsOn {
			if _, ok := atomMap[depID]; ok {
				inDegree[sa.Atom.ID]++
			}
		}
	}

	// Find all atoms with no dependencies (in-degree 0)
	var queue []*ScoredAtom
	for _, sa := range atoms {
		if inDegree[sa.Atom.ID] == 0 {
			queue = append(queue, sa)
		}
	}

	// Sort queue by score for deterministic ordering among peers
	sort.Slice(queue, func(i, j int) bool {
		// Mandatory first, then by score
		if queue[i].Atom.IsMandatory != queue[j].Atom.IsMandatory {
			return queue[i].Atom.IsMandatory
		}
		return queue[i].Combined > queue[j].Combined
	})

	// Build reverse dependency map (atom -> atoms that depend on it)
	dependents := make(map[string][]*ScoredAtom)
	for _, sa := range atoms {
		for _, depID := range sa.Atom.DependsOn {
			if _, ok := atomMap[depID]; ok {
				dependents[depID] = append(dependents[depID], sa)
			}
		}
	}

	// Process queue
	var result []*ScoredAtom
	for len(queue) > 0 {
		// Pop front
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree for dependents
		for _, dependent := range dependents[current.Atom.ID] {
			inDegree[dependent.Atom.ID]--
			if inDegree[dependent.Atom.ID] == 0 {
				queue = append(queue, dependent)
			}
		}

		// Re-sort queue after adding new items
		sort.Slice(queue, func(i, j int) bool {
			if queue[i].Atom.IsMandatory != queue[j].Atom.IsMandatory {
				return queue[i].Atom.IsMandatory
			}
			return queue[i].Combined > queue[j].Combined
		})
	}

	// Check for cycles
	if len(result) != len(atoms) {
		return nil, fmt.Errorf(
			"dependency cycle detected: processed %d of %d atoms",
			len(result), len(atoms),
		)
	}

	return result, nil
}

// ValidateDependencies checks if all dependencies can be satisfied.
// Returns a list of atoms with unmet dependencies.
func (r *DependencyResolver) ValidateDependencies(atoms []*PromptAtom) []DependencyError {
	atomSet := make(map[string]bool, len(atoms))
	for _, atom := range atoms {
		atomSet[atom.ID] = true
	}

	var errors []DependencyError
	for _, atom := range atoms {
		for _, depID := range atom.DependsOn {
			if !atomSet[depID] {
				errors = append(errors, DependencyError{
					AtomID:       atom.ID,
					MissingDepID: depID,
					Type:         DependencyErrorMissing,
				})
			}
		}
	}

	return errors
}

// DependencyErrorType categorizes dependency errors.
type DependencyErrorType int

const (
	// DependencyErrorMissing means a required dependency is not present.
	DependencyErrorMissing DependencyErrorType = iota

	// DependencyErrorCycle means a dependency cycle was detected.
	DependencyErrorCycle

	// DependencyErrorConflict means conflicting atoms are both selected.
	DependencyErrorConflict
)

// DependencyError describes a dependency resolution error.
type DependencyError struct {
	AtomID       string
	MissingDepID string
	ConflictID   string
	CycleIDs     []string
	Type         DependencyErrorType
}

// Error implements the error interface.
func (e DependencyError) Error() string {
	switch e.Type {
	case DependencyErrorMissing:
		return fmt.Sprintf("atom %s: missing dependency %s", e.AtomID, e.MissingDepID)
	case DependencyErrorCycle:
		return fmt.Sprintf("dependency cycle: %v", e.CycleIDs)
	case DependencyErrorConflict:
		return fmt.Sprintf("atom %s conflicts with %s", e.AtomID, e.ConflictID)
	default:
		return fmt.Sprintf("atom %s: unknown dependency error", e.AtomID)
	}
}

// DetectCycles finds dependency cycles in a set of atoms.
// Returns the cycle path if found, or nil if no cycle exists.
func (r *DependencyResolver) DetectCycles(atoms []*PromptAtom) []string {
	// Build adjacency list
	graph := make(map[string][]string, len(atoms))
	atomSet := make(map[string]bool, len(atoms))
	for _, atom := range atoms {
		if atom == nil || atom.ID == "" {
			continue
		}
		atomSet[atom.ID] = true
		for _, depID := range atom.DependsOn {
			graph[atom.ID] = append(graph[atom.ID], depID)
		}
	}

	// DFS with color marking
	// white = unvisited, gray = in-progress, black = done
	const (
		white = 0
		gray  = 1
		black = 2
	)

	color := make(map[string]int, len(atoms))
	parent := make(map[string]string, len(atoms))

	var cyclePath []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = gray

		for _, neighbor := range graph[node] {
			if !atomSet[neighbor] {
				continue // Dependency not in set, skip
			}

			if color[neighbor] == gray {
				// Found cycle - reconstruct path
				cyclePath = []string{neighbor}
				for cur := node; cur != neighbor; cur = parent[cur] {
					cyclePath = append([]string{cur}, cyclePath...)
				}
				cyclePath = append([]string{neighbor}, cyclePath...)
				return true
			}

			if color[neighbor] == white {
				parent[neighbor] = node
				if dfs(neighbor) {
					return true
				}
			}
		}

		color[node] = black
		return false
	}

	for _, atom := range atoms {
		if atom == nil || atom.ID == "" {
			continue
		}
		if color[atom.ID] == white {
			if dfs(atom.ID) {
				return cyclePath
			}
		}
	}

	return nil
}

// SortByCategory groups and sorts atoms by category, then by score within category.
func (r *DependencyResolver) SortByCategory(atoms []*OrderedAtom) []*OrderedAtom {
	// Group by category
	byCategory := make(map[AtomCategory][]*OrderedAtom)
	for _, oa := range atoms {
		if oa == nil || oa.Atom == nil {
			continue
		}
		cat := oa.Atom.Category
		byCategory[cat] = append(byCategory[cat], oa)
	}

	// Sort within each category by score
	for cat := range byCategory {
		sort.Slice(byCategory[cat], func(i, j int) bool {
			return byCategory[cat][i].Score > byCategory[cat][j].Score
		})
	}

	// Assemble in category order
	var result []*OrderedAtom
	categoryOrder := AllCategories() // Defined order

	for _, cat := range categoryOrder {
		if atoms, ok := byCategory[cat]; ok {
			result = append(result, atoms...)
			delete(byCategory, cat)
		}
	}

	// Append any remaining categories not in standard order
	unknownCats := make([]AtomCategory, 0, len(byCategory))
	for cat := range byCategory {
		unknownCats = append(unknownCats, cat)
	}
	sort.Slice(unknownCats, func(i, j int) bool {
		return string(unknownCats[i]) < string(unknownCats[j])
	})
	for _, cat := range unknownCats {
		result = append(result, byCategory[cat]...)
	}

	// Update order indices
	for i := range result {
		result[i].Order = i
	}

	return result
}
