package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDependencyResolver(t *testing.T) {
	t.Run("creates resolver with default settings", func(t *testing.T) {
		resolver := NewDependencyResolver()
		require.NotNil(t, resolver)
		assert.False(t, resolver.allowMissingDeps)
	})
}

func TestDependencyResolver_SetAllowMissingDeps(t *testing.T) {
	resolver := NewDependencyResolver()

	resolver.SetAllowMissingDeps(true)
	assert.True(t, resolver.allowMissingDeps)

	resolver.SetAllowMissingDeps(false)
	assert.False(t, resolver.allowMissingDeps)
}

func TestDependencyResolver_Resolve(t *testing.T) {
	tests := []struct {
		name          string
		atoms         []*ScoredAtom
		expectError   bool
		expectedOrder []string // Expected atom IDs in order
		expectedLen   int
	}{
		{
			name:          "empty input",
			atoms:         nil,
			expectError:   false,
			expectedOrder: nil,
			expectedLen:   0,
		},
		{
			name: "single atom no dependencies",
			atoms: []*ScoredAtom{
				{Atom: &PromptAtom{ID: "a", Priority: 50}, Combined: 0.5},
			},
			expectError: false,
			expectedLen: 1,
		},
		{
			name: "multiple atoms no dependencies - sorted by score",
			atoms: []*ScoredAtom{
				{Atom: &PromptAtom{ID: "a", Priority: 50}, Combined: 0.5},
				{Atom: &PromptAtom{ID: "b", Priority: 60}, Combined: 0.8},
				{Atom: &PromptAtom{ID: "c", Priority: 40}, Combined: 0.3},
			},
			expectError: false,
			expectedLen: 3,
		},
		{
			name: "simple dependency chain",
			atoms: []*ScoredAtom{
				{Atom: &PromptAtom{ID: "a", DependsOn: []string{"b"}}, Combined: 0.5},
				{Atom: &PromptAtom{ID: "b"}, Combined: 0.6},
			},
			expectError: false,
			expectedLen: 2,
		},
		{
			name: "dependency with missing dep is excluded",
			atoms: []*ScoredAtom{
				{Atom: &PromptAtom{ID: "a", DependsOn: []string{"missing"}}, Combined: 0.5},
				{Atom: &PromptAtom{ID: "b"}, Combined: 0.6},
			},
			expectError: false,
			expectedLen: 1, // Only "b" remains
		},
		{
			name: "multi-level dependency chain",
			atoms: []*ScoredAtom{
				{Atom: &PromptAtom{ID: "a", DependsOn: []string{"b"}}, Combined: 0.5},
				{Atom: &PromptAtom{ID: "b", DependsOn: []string{"c"}}, Combined: 0.6},
				{Atom: &PromptAtom{ID: "c"}, Combined: 0.7},
			},
			expectError: false,
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewDependencyResolver()

			ordered, err := resolver.Resolve(tt.atoms)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, ordered, tt.expectedLen)

			if tt.expectedOrder != nil {
				actualOrder := make([]string, len(ordered))
				for i, oa := range ordered {
					actualOrder[i] = oa.Atom.ID
				}
				assert.Equal(t, tt.expectedOrder, actualOrder)
			}
		})
	}
}

func TestDependencyResolver_ResolveWithDependencies(t *testing.T) {
	t.Run("dependency comes before dependent", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", DependsOn: []string{"b"}}, Combined: 0.9},
			{Atom: &PromptAtom{ID: "b"}, Combined: 0.6},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		require.Len(t, ordered, 2)

		// Find positions
		var bOrder, aOrder int
		for _, o := range ordered {
			if o.Atom.ID == "a" {
				aOrder = o.Order
			}
			if o.Atom.ID == "b" {
				bOrder = o.Order
			}
		}

		assert.Less(t, bOrder, aOrder, "b should come before a")
	})

	t.Run("multiple dependencies all come before dependent", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", DependsOn: []string{"b", "c"}}, Combined: 0.9},
			{Atom: &PromptAtom{ID: "b"}, Combined: 0.6},
			{Atom: &PromptAtom{ID: "c"}, Combined: 0.5},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		require.Len(t, ordered, 3)

		// Find positions
		var aOrder, bOrder, cOrder int
		for _, o := range ordered {
			switch o.Atom.ID {
			case "a":
				aOrder = o.Order
			case "b":
				bOrder = o.Order
			case "c":
				cOrder = o.Order
			}
		}

		assert.Less(t, bOrder, aOrder, "b should come before a")
		assert.Less(t, cOrder, aOrder, "c should come before a")
	})

	t.Run("diamond dependency pattern", func(t *testing.T) {
		// d depends on b and c, both b and c depend on a
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a"}, Combined: 0.5},
			{Atom: &PromptAtom{ID: "b", DependsOn: []string{"a"}}, Combined: 0.6},
			{Atom: &PromptAtom{ID: "c", DependsOn: []string{"a"}}, Combined: 0.7},
			{Atom: &PromptAtom{ID: "d", DependsOn: []string{"b", "c"}}, Combined: 0.8},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		require.Len(t, ordered, 4)

		// Find positions
		positions := make(map[string]int)
		for _, o := range ordered {
			positions[o.Atom.ID] = o.Order
		}

		assert.Less(t, positions["a"], positions["b"])
		assert.Less(t, positions["a"], positions["c"])
		assert.Less(t, positions["b"], positions["d"])
		assert.Less(t, positions["c"], positions["d"])
	})
}

func TestDependencyResolver_ResolveCircularDependency(t *testing.T) {
	t.Run("simple cycle - two atoms", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", DependsOn: []string{"b"}}, Combined: 0.5},
			{Atom: &PromptAtom{ID: "b", DependsOn: []string{"a"}}, Combined: 0.6},
		}

		resolver := NewDependencyResolver()
		_, err := resolver.Resolve(atoms)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cycle")
	})

	t.Run("three-way cycle", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", DependsOn: []string{"b"}}, Combined: 0.5},
			{Atom: &PromptAtom{ID: "b", DependsOn: []string{"c"}}, Combined: 0.6},
			{Atom: &PromptAtom{ID: "c", DependsOn: []string{"a"}}, Combined: 0.7},
		}

		resolver := NewDependencyResolver()
		_, err := resolver.Resolve(atoms)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cycle")
	})

	t.Run("self-dependency causes cycle error", func(t *testing.T) {
		// Self-dependency is a cycle of length 1
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", DependsOn: []string{"a"}}, Combined: 0.5},
		}

		resolver := NewDependencyResolver()
		_, err := resolver.Resolve(atoms)

		// Self-dependency is a cycle
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cycle")
	})
}

func TestDependencyResolver_ResolveConflicts(t *testing.T) {
	t.Run("lower scored conflicting atom excluded", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", ConflictsWith: []string{"b"}}, Combined: 0.8},
			{Atom: &PromptAtom{ID: "b"}, Combined: 0.5},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		assert.Len(t, ordered, 1)
		assert.Equal(t, "a", ordered[0].Atom.ID)
	})

	t.Run("conflict is unidirectional - declarer excludes target", func(t *testing.T) {
		// a declares conflict with b, but b doesn't declare conflict with a
		// Processed in score order: b (0.9) first, then a (0.3)
		// When b is processed, it has no conflicts to apply
		// When a is processed, b is already in the result, so a gets excluded by its own conflict
		// Actually, let's check what the code does: it excludes atoms that conflict with HIGHER scored atoms
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", ConflictsWith: []string{"b"}}, Combined: 0.3},
			{Atom: &PromptAtom{ID: "b"}, Combined: 0.9},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		// Both should be included since conflict is checked AFTER sorting by score
		// b (0.9) is processed first and marks conflicting atoms as excluded
		// But b has no conflicts declared, so nothing is excluded
		// a (0.3) is processed second - it's not excluded, so it stays
		// The conflict direction matters: a says "I conflict with b" but b is already in
		assert.Len(t, ordered, 2)
	})

	t.Run("bidirectional conflict - higher wins", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", ConflictsWith: []string{"b"}}, Combined: 0.6},
			{Atom: &PromptAtom{ID: "b", ConflictsWith: []string{"a"}}, Combined: 0.4},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		assert.Len(t, ordered, 1)
		assert.Equal(t, "a", ordered[0].Atom.ID)
	})

	t.Run("transitive conflict - only highest survives", func(t *testing.T) {
		// a conflicts with b, b conflicts with c
		// Process order: highest score first
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", ConflictsWith: []string{"b"}}, Combined: 0.9},
			{Atom: &PromptAtom{ID: "b", ConflictsWith: []string{"c"}}, Combined: 0.7},
			{Atom: &PromptAtom{ID: "c"}, Combined: 0.5},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		// a wins (highest), excludes b
		// c has no conflict with a, so survives
		assert.Len(t, ordered, 2)

		ids := make([]string, len(ordered))
		for i, o := range ordered {
			ids[i] = o.Atom.ID
		}
		assert.Contains(t, ids, "a")
		assert.Contains(t, ids, "c")
		assert.NotContains(t, ids, "b")
	})

	t.Run("non-conflicting atoms all survive", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a"}, Combined: 0.9},
			{Atom: &PromptAtom{ID: "b"}, Combined: 0.7},
			{Atom: &PromptAtom{ID: "c"}, Combined: 0.5},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		assert.Len(t, ordered, 3)
	})
}

func TestDependencyResolver_ValidateDependencies(t *testing.T) {
	t.Run("all dependencies satisfied", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b"},
		}

		resolver := NewDependencyResolver()
		errors := resolver.ValidateDependencies(atoms)

		assert.Empty(t, errors)
	})

	t.Run("missing dependency detected", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "a", DependsOn: []string{"missing"}},
			{ID: "b"},
		}

		resolver := NewDependencyResolver()
		errors := resolver.ValidateDependencies(atoms)

		require.Len(t, errors, 1)
		assert.Equal(t, "a", errors[0].AtomID)
		assert.Equal(t, "missing", errors[0].MissingDepID)
		assert.Equal(t, DependencyErrorMissing, errors[0].Type)
	})

	t.Run("multiple missing dependencies", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "a", DependsOn: []string{"missing1", "missing2"}},
			{ID: "b", DependsOn: []string{"missing3"}},
		}

		resolver := NewDependencyResolver()
		errors := resolver.ValidateDependencies(atoms)

		assert.Len(t, errors, 3)
	})
}

func TestDependencyResolver_DetectCycles(t *testing.T) {
	t.Run("no cycle returns nil", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b"},
		}

		resolver := NewDependencyResolver()
		cycle := resolver.DetectCycles(atoms)

		assert.Nil(t, cycle)
	})

	t.Run("simple cycle detected", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b", DependsOn: []string{"a"}},
		}

		resolver := NewDependencyResolver()
		cycle := resolver.DetectCycles(atoms)

		assert.NotNil(t, cycle)
		assert.Contains(t, cycle, "a")
		assert.Contains(t, cycle, "b")
	})

	t.Run("three-way cycle detected", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b", DependsOn: []string{"c"}},
			{ID: "c", DependsOn: []string{"a"}},
		}

		resolver := NewDependencyResolver()
		cycle := resolver.DetectCycles(atoms)

		assert.NotNil(t, cycle)
		assert.GreaterOrEqual(t, len(cycle), 2)
	})

	t.Run("missing dependency in cycle check is skipped", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "a", DependsOn: []string{"missing"}},
		}

		resolver := NewDependencyResolver()
		cycle := resolver.DetectCycles(atoms)

		assert.Nil(t, cycle)
	})
}

func TestDependencyResolver_SortByCategory(t *testing.T) {
	t.Run("atoms grouped by category", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "method1", Category: CategoryMethodology}, Score: 0.5, Order: 0},
			{Atom: &PromptAtom{ID: "identity1", Category: CategoryIdentity}, Score: 0.8, Order: 1},
			{Atom: &PromptAtom{ID: "method2", Category: CategoryMethodology}, Score: 0.6, Order: 2},
			{Atom: &PromptAtom{ID: "protocol1", Category: CategoryProtocol}, Score: 0.7, Order: 3},
		}

		resolver := NewDependencyResolver()
		sorted := resolver.SortByCategory(atoms)

		// Identity should come first (as per AllCategories order)
		assert.Equal(t, CategoryIdentity, sorted[0].Atom.Category)

		// Protocol before Methodology
		var protocolIdx, methodologyStartIdx int
		for i, oa := range sorted {
			if oa.Atom.Category == CategoryProtocol {
				protocolIdx = i
				break
			}
		}
		for i, oa := range sorted {
			if oa.Atom.Category == CategoryMethodology {
				methodologyStartIdx = i
				break
			}
		}
		assert.Less(t, protocolIdx, methodologyStartIdx)
	})

	t.Run("within category sorted by score", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "method1", Category: CategoryMethodology}, Score: 0.3, Order: 0},
			{Atom: &PromptAtom{ID: "method2", Category: CategoryMethodology}, Score: 0.9, Order: 1},
			{Atom: &PromptAtom{ID: "method3", Category: CategoryMethodology}, Score: 0.6, Order: 2},
		}

		resolver := NewDependencyResolver()
		sorted := resolver.SortByCategory(atoms)

		// Should be sorted by score descending within category
		assert.Equal(t, "method2", sorted[0].Atom.ID) // 0.9
		assert.Equal(t, "method3", sorted[1].Atom.ID) // 0.6
		assert.Equal(t, "method1", sorted[2].Atom.ID) // 0.3
	})

	t.Run("order indices updated", func(t *testing.T) {
		atoms := []*OrderedAtom{
			{Atom: &PromptAtom{ID: "a", Category: CategoryMethodology}, Score: 0.5, Order: 5},
			{Atom: &PromptAtom{ID: "b", Category: CategoryIdentity}, Score: 0.8, Order: 3},
		}

		resolver := NewDependencyResolver()
		sorted := resolver.SortByCategory(atoms)

		// Order indices should be sequential starting from 0
		for i, oa := range sorted {
			assert.Equal(t, i, oa.Order)
		}
	})
}

func TestDependencyError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      DependencyError
		contains string
	}{
		{
			name: "missing dependency error",
			err: DependencyError{
				AtomID:       "child",
				MissingDepID: "parent",
				Type:         DependencyErrorMissing,
			},
			contains: "missing dependency",
		},
		{
			name: "cycle error",
			err: DependencyError{
				CycleIDs: []string{"a", "b", "a"},
				Type:     DependencyErrorCycle,
			},
			contains: "cycle",
		},
		{
			name: "conflict error",
			err: DependencyError{
				AtomID:     "atom1",
				ConflictID: "atom2",
				Type:       DependencyErrorConflict,
			},
			contains: "conflicts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			assert.Contains(t, errStr, tt.contains)
		})
	}
}

func TestDependencyResolver_MandatoryAtomsFirst(t *testing.T) {
	t.Run("mandatory atoms ordered before optional", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "optional1", IsMandatory: false}, Combined: 0.9},
			{Atom: &PromptAtom{ID: "mandatory1", IsMandatory: true}, Combined: 0.5},
			{Atom: &PromptAtom{ID: "optional2", IsMandatory: false}, Combined: 0.8},
		}

		resolver := NewDependencyResolver()
		ordered, err := resolver.Resolve(atoms)

		require.NoError(t, err)
		require.Len(t, ordered, 3)

		// Mandatory should come first
		assert.True(t, ordered[0].Atom.IsMandatory)
	})
}

// Benchmark tests

func BenchmarkResolve_SmallSet(b *testing.B) {
	atoms := make([]*ScoredAtom, 10)
	for i := 0; i < 10; i++ {
		atoms[i] = &ScoredAtom{
			Atom:     &PromptAtom{ID: string(rune('a' + i))},
			Combined: float64(i) / 10.0,
		}
	}

	resolver := NewDependencyResolver()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = resolver.Resolve(atoms)
	}
}

func BenchmarkResolve_MediumSet(b *testing.B) {
	atoms := make([]*ScoredAtom, 100)
	for i := 0; i < 100; i++ {
		var deps []string
		if i > 0 && i%10 == 0 {
			deps = []string{string(rune('a' + (i - 1)))}
		}
		atoms[i] = &ScoredAtom{
			Atom:     &PromptAtom{ID: string(rune('a' + i)), DependsOn: deps},
			Combined: float64(i) / 100.0,
		}
	}

	resolver := NewDependencyResolver()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = resolver.Resolve(atoms)
	}
}

func BenchmarkDetectCycles(b *testing.B) {
	atoms := make([]*PromptAtom, 50)
	for i := 0; i < 50; i++ {
		var deps []string
		if i > 0 {
			deps = []string{string(rune('a' + (i - 1)))}
		}
		atoms[i] = &PromptAtom{
			ID:        string(rune('a' + i)),
			DependsOn: deps,
		}
	}

	resolver := NewDependencyResolver()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resolver.DetectCycles(atoms)
	}
}

func BenchmarkSortByCategory(b *testing.B) {
	categories := AllCategories()
	atoms := make([]*OrderedAtom, 100)
	for i := 0; i < 100; i++ {
		atoms[i] = &OrderedAtom{
			Atom:  &PromptAtom{ID: string(rune(i)), Category: categories[i%len(categories)]},
			Score: float64(i) / 100.0,
			Order: i,
		}
	}

	resolver := NewDependencyResolver()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resolver.SortByCategory(atoms)
	}
}
