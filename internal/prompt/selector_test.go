package prompt

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockVectorSearcher implements VectorSearcher for testing.
type mockVectorSearcher struct {
	results map[string]float64
	err     error
}

func (m *mockVectorSearcher) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}

	var results []SearchResult
	for atomID, score := range m.results {
		results = append(results, SearchResult{
			AtomID: atomID,
			Score:  score,
		})
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// =========================================================================
// isSkeletonCategory Tests
// =========================================================================

func TestIsSkeletonCategory(t *testing.T) {
	tests := []struct {
		name     string
		category AtomCategory
		want     bool
	}{
		// Skeleton categories (should return true)
		{name: "identity is skeleton", category: CategoryIdentity, want: true},
		{name: "protocol is skeleton", category: CategoryProtocol, want: true},
		{name: "safety is skeleton", category: CategorySafety, want: true},
		{name: "methodology is skeleton", category: CategoryMethodology, want: true},

		// Flesh categories (should return false)
		{name: "exemplar is flesh", category: CategoryExemplar, want: false},
		{name: "domain is flesh", category: CategoryDomain, want: false},
		{name: "context is flesh", category: CategoryContext, want: false},
		{name: "language is flesh", category: CategoryLanguage, want: false},
		{name: "framework is flesh", category: CategoryFramework, want: false},
		{name: "hallucination is flesh", category: CategoryHallucination, want: false},
		{name: "campaign is flesh", category: CategoryCampaign, want: false},
		{name: "init is flesh", category: CategoryInit, want: false},
		{name: "northstar is flesh", category: CategoryNorthstar, want: false},
		{name: "ouroboros is flesh", category: CategoryOuroboros, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSkeletonCategory(tt.category)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =========================================================================
// AtomSelector Basic Tests
// =========================================================================

func TestNewAtomSelector(t *testing.T) {
	t.Run("creates selector with defaults", func(t *testing.T) {
		selector := NewAtomSelector()
		require.NotNil(t, selector)

		assert.Equal(t, 0.3, selector.vectorWeight)
		assert.Equal(t, 0.1, selector.minScoreThreshold)
	})
}

func TestAtomSelector_SetVectorWeight(t *testing.T) {
	selector := NewAtomSelector()

	tests := []struct {
		name     string
		weight   float64
		expected float64
	}{
		{"normal value", 0.5, 0.5},
		{"zero", 0.0, 0.0},
		{"one", 1.0, 1.0},
		{"negative clamped to zero", -0.5, 0.0},
		{"over one clamped", 1.5, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector.SetVectorWeight(tt.weight)
			assert.Equal(t, tt.expected, selector.vectorWeight)
		})
	}
}

func TestAtomSelector_SetMinScoreThreshold(t *testing.T) {
	selector := NewAtomSelector()

	selector.SetMinScoreThreshold(0.5)
	assert.Equal(t, 0.5, selector.minScoreThreshold)
}

// =========================================================================
// SelectAtoms System 2 Bifurcation Tests
// =========================================================================

func TestAtomSelector_SelectAtoms_Bifurcation(t *testing.T) {
	t.Run("empty atoms returns nil", func(t *testing.T) {
		selector := NewAtomSelector()
		kernel := &mockKernel{}
		selector.SetKernel(kernel)

		result, err := selector.SelectAtoms(context.Background(), nil, NewCompilationContext())
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("skeleton atoms loaded first", func(t *testing.T) {
		selector := NewAtomSelector()

		// Create atoms of various categories
		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "identity content"},
			{ID: "safety-1", Category: CategorySafety, Content: "safety content"},
			{ID: "domain-1", Category: CategoryDomain, Content: "domain content"},
			{ID: "exemplar-1", Category: CategoryExemplar, Content: "exemplar content"},
		}

		// Mock kernel returns all atoms as selected
		kernel := &mockKernel{
			facts: []interface{}{
				Fact{Predicate: "selected_result", Args: []interface{}{"identity-1", 100, "mandatory"}},
				Fact{Predicate: "selected_result", Args: []interface{}{"safety-1", 90, "mandatory"}},
				Fact{Predicate: "selected_result", Args: []interface{}{"domain-1", 80, "context_match"}},
				Fact{Predicate: "selected_result", Args: []interface{}{"exemplar-1", 70, "vector_match"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
		require.NoError(t, err)
		require.Len(t, result, 4)

		// Skeleton atoms should come first
		assert.True(t, isSkeletonCategory(result[0].Atom.Category), "first atom should be skeleton")
		assert.True(t, isSkeletonCategory(result[1].Atom.Category), "second atom should be skeleton")
	})

	t.Run("skeleton failure is critical", func(t *testing.T) {
		selector := NewAtomSelector()

		// Atoms with only skeleton categories
		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "identity content"},
		}

		// Mock kernel that fails on query
		kernel := &mockKernel{
			queryErr: errors.New("kernel error"),
		}
		selector.SetKernel(kernel)

		_, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CRITICAL")
	})

	t.Run("flesh failure is acceptable", func(t *testing.T) {
		selector := NewAtomSelector()

		// Atoms with both skeleton and flesh
		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "identity content"},
			{ID: "domain-1", Category: CategoryDomain, Content: "domain content"},
		}

		// Create a kernel that returns skeleton atoms but has no flesh results
		kernel := &mockKernel{
			facts: []interface{}{
				Fact{Predicate: "selected_result", Args: []interface{}{"identity-1", 100, "mandatory"}},
				// No flesh atoms returned
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
		require.NoError(t, err)
		// Should have at least skeleton atoms
		assert.NotEmpty(t, result)
	})

	t.Run("no kernel returns error", func(t *testing.T) {
		selector := NewAtomSelector()
		// No kernel set

		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "identity content"},
		}

		_, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CRITICAL")
	})

	t.Run("no skeleton atoms returns error", func(t *testing.T) {
		selector := NewAtomSelector()
		kernel := &mockKernel{}
		selector.SetKernel(kernel)

		// Only flesh atoms, no skeleton
		atoms := []*PromptAtom{
			{ID: "domain-1", Category: CategoryDomain, Content: "domain content"},
			{ID: "exemplar-1", Category: CategoryExemplar, Content: "exemplar content"},
		}

		_, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no skeleton atoms")
	})
}

// =========================================================================
// MergeAtoms Tests
// =========================================================================

func TestAtomSelector_MergeAtoms(t *testing.T) {
	selector := NewAtomSelector()

	t.Run("skeleton comes first", func(t *testing.T) {
		skeleton := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "skel-1", Category: CategoryIdentity}, Combined: 1.0, Source: "skeleton"},
		}
		flesh := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "flesh-1", Category: CategoryDomain}, Combined: 0.9, Source: "flesh"},
		}

		result := selector.mergeAtoms(skeleton, flesh)
		require.Len(t, result, 2)
		assert.Equal(t, "skel-1", result[0].Atom.ID)
		assert.Equal(t, "flesh-1", result[1].Atom.ID)
	})

	t.Run("deduplicates by ID", func(t *testing.T) {
		skeleton := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "shared", Category: CategoryIdentity}, Combined: 1.0, Source: "skeleton"},
		}
		flesh := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "shared", Category: CategoryIdentity}, Combined: 0.5, Source: "flesh"},
			{Atom: &PromptAtom{ID: "unique", Category: CategoryDomain}, Combined: 0.7, Source: "flesh"},
		}

		result := selector.mergeAtoms(skeleton, flesh)
		require.Len(t, result, 2)

		// Skeleton version should be preserved
		assert.Equal(t, "skeleton", result[0].Source)
	})

	t.Run("handles nil flesh", func(t *testing.T) {
		skeleton := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "skel-1", Category: CategoryIdentity}, Combined: 1.0, Source: "skeleton"},
		}

		result := selector.mergeAtoms(skeleton, nil)
		require.Len(t, result, 1)
		assert.Equal(t, "skel-1", result[0].Atom.ID)
	})

	t.Run("handles empty skeleton", func(t *testing.T) {
		flesh := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "flesh-1", Category: CategoryDomain}, Combined: 0.9, Source: "flesh"},
		}

		result := selector.mergeAtoms(nil, flesh)
		require.Len(t, result, 1)
		assert.Equal(t, "flesh-1", result[0].Atom.ID)
	})

	t.Run("mandatory atoms prioritized within type", func(t *testing.T) {
		skeleton := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "skel-1", Category: CategoryIdentity, IsMandatory: false}, Combined: 1.0, Source: "skeleton"},
			{Atom: &PromptAtom{ID: "skel-2", Category: CategoryProtocol, IsMandatory: true}, Combined: 0.5, Source: "skeleton"},
		}

		result := selector.mergeAtoms(skeleton, nil)
		require.Len(t, result, 2)

		// Mandatory should come first
		assert.True(t, result[0].Atom.IsMandatory)
	})
}

// =========================================================================
// FallbackFleshSelection Tests
// =========================================================================

func TestAtomSelector_FallbackFleshSelection(t *testing.T) {
	selector := NewAtomSelector()

	t.Run("context matching", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:               "match",
				Category:         CategoryDomain,
				Content:          "content",
				OperationalModes: []string{"/active"},
			},
			{
				ID:               "no-match",
				Category:         CategoryDomain,
				Content:          "content",
				OperationalModes: []string{"/dream"}, // Won't match /active context
			},
		}

		cc := NewCompilationContext().WithOperationalMode("/active")
		vectorScores := map[string]float64{}

		result := selector.fallbackFleshSelection(atoms, vectorScores, cc)

		// Only "match" should be selected
		require.Len(t, result, 1)
		assert.Equal(t, "match", result[0].Atom.ID)
	})

	t.Run("vector scores boost ranking", func(t *testing.T) {
		atoms := []*PromptAtom{
			{ID: "low-vector", Category: CategoryDomain, Content: "content"},
			{ID: "high-vector", Category: CategoryDomain, Content: "content"},
		}

		cc := NewCompilationContext()
		vectorScores := map[string]float64{
			"low-vector":  0.1,
			"high-vector": 0.9,
		}

		result := selector.fallbackFleshSelection(atoms, vectorScores, cc)

		require.Len(t, result, 2)
		// Higher vector score should come first
		assert.Equal(t, "high-vector", result[0].Atom.ID)
	})
}

// =========================================================================
// LoadSkeletonAtoms Tests
// =========================================================================

func TestAtomSelector_LoadSkeletonAtoms(t *testing.T) {
	t.Run("filters to skeleton categories", func(t *testing.T) {
		selector := NewAtomSelector()

		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "content"},
			{ID: "domain-1", Category: CategoryDomain, Content: "content"},
		}

		kernel := &mockKernel{
			facts: []interface{}{
				Fact{Predicate: "selected_result", Args: []interface{}{"identity-1", 100, "mandatory"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.loadSkeletonAtoms(context.Background(), atoms, NewCompilationContext())
		require.NoError(t, err)

		// Should only include skeleton atoms
		for _, sa := range result {
			assert.True(t, isSkeletonCategory(sa.Atom.Category))
		}
	})

	t.Run("returns error without kernel", func(t *testing.T) {
		selector := NewAtomSelector()
		// No kernel set

		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "content"},
		}

		_, err := selector.loadSkeletonAtoms(context.Background(), atoms, NewCompilationContext())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CRITICAL")
	})

	t.Run("returns error when no skeleton atoms in corpus", func(t *testing.T) {
		selector := NewAtomSelector()
		kernel := &mockKernel{}
		selector.SetKernel(kernel)

		// Only flesh atoms
		atoms := []*PromptAtom{
			{ID: "domain-1", Category: CategoryDomain, Content: "content"},
		}

		_, err := selector.loadSkeletonAtoms(context.Background(), atoms, NewCompilationContext())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no skeleton atoms")
	})

	t.Run("sets source to skeleton", func(t *testing.T) {
		selector := NewAtomSelector()

		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "content"},
		}

		kernel := &mockKernel{
			facts: []interface{}{
				Fact{Predicate: "selected_result", Args: []interface{}{"identity-1", 100, "mandatory"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.loadSkeletonAtoms(context.Background(), atoms, NewCompilationContext())
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "skeleton", result[0].Source)
	})
}

// =========================================================================
// LoadFleshAtoms Tests
// =========================================================================

func TestAtomSelector_LoadFleshAtoms(t *testing.T) {
	t.Run("filters to flesh categories", func(t *testing.T) {
		selector := NewAtomSelector()

		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "content"},
			{ID: "domain-1", Category: CategoryDomain, Content: "content"},
		}

		kernel := &mockKernel{
			facts: []interface{}{
				Fact{Predicate: "selected_result", Args: []interface{}{"domain-1", 80, "context_match"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.loadFleshAtoms(context.Background(), atoms, NewCompilationContext())
		require.NoError(t, err)

		// Should only include flesh atoms
		for _, sa := range result {
			assert.False(t, isSkeletonCategory(sa.Atom.Category))
		}
	})

	t.Run("returns nil for empty flesh corpus", func(t *testing.T) {
		selector := NewAtomSelector()
		kernel := &mockKernel{}
		selector.SetKernel(kernel)

		// Only skeleton atoms
		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "content"},
		}

		result, err := selector.loadFleshAtoms(context.Background(), atoms, NewCompilationContext())
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("falls back on kernel error", func(t *testing.T) {
		selector := NewAtomSelector()

		atoms := []*PromptAtom{
			{ID: "domain-1", Category: CategoryDomain, Content: "content"},
		}

		kernel := &mockKernel{
			queryErr: errors.New("kernel error"),
		}
		selector.SetKernel(kernel)

		// Should not return error - falls back to context matching
		result, err := selector.loadFleshAtoms(context.Background(), atoms, NewCompilationContext())
		require.NoError(t, err)
		// Falls back to context matching
		assert.NotNil(t, result)
	})

	t.Run("integrates vector scores", func(t *testing.T) {
		selector := NewAtomSelector()

		atoms := []*PromptAtom{
			{ID: "domain-1", Category: CategoryDomain, Content: "content"},
		}

		kernel := &mockKernel{
			facts: []interface{}{
				Fact{Predicate: "selected_result", Args: []interface{}{"domain-1", 80, "vector_match"}},
			},
		}
		selector.SetKernel(kernel)

		vectorSearcher := &mockVectorSearcher{
			results: map[string]float64{"domain-1": 0.8},
		}
		selector.SetVectorSearcher(vectorSearcher)

		cc := NewCompilationContext().WithSemanticQuery("test query", 10)
		result, err := selector.loadFleshAtoms(context.Background(), atoms, cc)
		require.NoError(t, err)
		require.Len(t, result, 1)

		// Should have vector score integrated
		assert.Equal(t, 0.8, result[0].VectorScore)
	})

	t.Run("sets source to flesh", func(t *testing.T) {
		selector := NewAtomSelector()

		atoms := []*PromptAtom{
			{ID: "domain-1", Category: CategoryDomain, Content: "content"},
		}

		kernel := &mockKernel{
			facts: []interface{}{
				Fact{Predicate: "selected_result", Args: []interface{}{"domain-1", 80, "context_match"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.loadFleshAtoms(context.Background(), atoms, NewCompilationContext())
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "flesh", result[0].Source)
	})
}

// =========================================================================
// Legacy Tests (for backwards compatibility)
// =========================================================================

func TestAtomSelector_SelectAtomsLegacy(t *testing.T) {
	type mockResult struct {
		id     string
		source string
		score  float64
	}

	tests := []struct {
		name          string
		atoms         []*PromptAtom
		context       *CompilationContext
		vectorResults map[string]float64
		mockResults   []mockResult
		expectedLen   int
		expectedIDs   []string
	}{
		{
			name:        "empty atoms",
			atoms:       nil,
			context:     NewCompilationContext(),
			expectedLen: 0,
		},
		{
			name: "all atoms match context",
			atoms: []*PromptAtom{
				{ID: "a", Content: "content a"},
				{ID: "b", Content: "content b"},
			},
			context: NewCompilationContext(),
			mockResults: []mockResult{
				{"a", "skeleton", 1.0},
				{"b", "skeleton", 1.0},
			},
			expectedLen: 2,
		},
		{
			name: "filters by mock results",
			atoms: []*PromptAtom{
				{ID: "coder-only"},
				{ID: "tester-only"},
			},
			context: NewCompilationContext(),
			mockResults: []mockResult{
				{"coder-only", "skeleton", 1.0},
			},
			expectedLen: 1,
			expectedIDs: []string{"coder-only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := NewAtomSelector()

			// Setup mock kernel
			kernel := &mockKernel{}
			var facts []interface{}
			for _, mr := range tt.mockResults {
				facts = append(facts, Fact{
					Predicate: "selected_result",
					Args:      []interface{}{mr.id, mr.score, mr.source},
				})
			}
			kernel.facts = facts
			selector.SetKernel(kernel)

			if tt.vectorResults != nil {
				selector.SetVectorSearcher(&mockVectorSearcher{results: tt.vectorResults})
			}

			scored, err := selector.SelectAtomsLegacy(context.Background(), tt.atoms, tt.context)

			require.NoError(t, err)
			assert.Len(t, scored, tt.expectedLen)

			if tt.expectedIDs != nil {
				actualIDs := make([]string, len(scored))
				for i, sa := range scored {
					actualIDs[i] = sa.Atom.ID
				}
				assert.ElementsMatch(t, tt.expectedIDs, actualIDs)
			}
		})
	}
}

// =========================================================================
// Benchmarks
// =========================================================================

func BenchmarkSelectAtoms(b *testing.B) {
	// Create a mix of skeleton and flesh atoms
	atoms := make([]*PromptAtom, 100)
	categories := []AtomCategory{
		CategoryIdentity, CategoryProtocol, CategorySafety, CategoryMethodology,
		CategoryDomain, CategoryLanguage, CategoryExemplar, CategoryContext,
	}

	for i := 0; i < 100; i++ {
		atoms[i] = &PromptAtom{
			ID:       string(rune('a' + i%26)),
			Priority: i,
			Content:  "content",
			Category: categories[i%len(categories)],
		}
	}

	// Create mock kernel with all atoms selected
	kernel := &mockKernel{}
	for _, a := range atoms {
		kernel.facts = append(kernel.facts, Fact{
			Predicate: "selected_result",
			Args:      []interface{}{a.ID, a.Priority, "benchmark"},
		})
	}

	selector := NewAtomSelector()
	selector.SetKernel(kernel)
	cc := NewCompilationContext()
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = selector.SelectAtoms(ctx, atoms, cc)
	}
}

func BenchmarkMergeAtoms(b *testing.B) {
	selector := NewAtomSelector()

	skeleton := make([]*ScoredAtom, 20)
	for i := 0; i < 20; i++ {
		skeleton[i] = &ScoredAtom{
			Atom:     &PromptAtom{ID: string(rune('s' + i)), Category: CategoryIdentity},
			Combined: float64(i) / 20,
			Source:   "skeleton",
		}
	}

	flesh := make([]*ScoredAtom, 80)
	for i := 0; i < 80; i++ {
		flesh[i] = &ScoredAtom{
			Atom:     &PromptAtom{ID: string(rune('f' + i)), Category: CategoryDomain},
			Combined: float64(i) / 80,
			Source:   "flesh",
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = selector.mergeAtoms(skeleton, flesh)
	}
}
