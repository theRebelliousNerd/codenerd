package prompt

import (
	"context"
	"errors"
	"testing"

	"github.com/google/mangle/parse"
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

func (m *mockVectorSearcher) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

type validatingKernel struct {
	results []Fact
}

func (k *validatingKernel) Query(predicate string) ([]Fact, error) {
	return k.results, nil
}

func (k *validatingKernel) AssertBatch(facts []any) error {
	for _, f := range facts {
		s, ok := f.(string)
		if !ok {
			continue
		}

		// We mimic internal/mangle's permissive parse behavior: try as-is, then with '.'.
		if _, err := parse.Atom(s); err != nil {
			if _, err2 := parse.Atom(s + "."); err2 != nil {
				return err2
			}
		}
	}
	return nil
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
		{name: "capability is flesh", category: CategoryCapability, want: false},
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
		{name: "autopoiesis is flesh", category: CategoryAutopoiesis, want: false},
		{name: "eval is flesh", category: CategoryEval, want: false},
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
	// Boundary gaps remediated: see selector_gaps_test.go for nil context/nil elements coverage.
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
			facts: []any{
				Fact{Predicate: "selected_result", Args: []any{"identity-1", 100, "mandatory"}},
				Fact{Predicate: "selected_result", Args: []any{"safety-1", 90, "mandatory"}},
				Fact{Predicate: "selected_result", Args: []any{"domain-1", 80, "context_match"}},
				Fact{Predicate: "selected_result", Args: []any{"exemplar-1", 70, "vector_match"}},
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
			facts: []any{
				Fact{Predicate: "selected_result", Args: []any{"identity-1", 100, "mandatory"}},
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

func TestAtomSelector_SelectAtoms_FactsAreMangleParseable(t *testing.T) {
	// Massive flesh atoms gap remediated: see selector_gaps_test.go TestSelector_MassiveAtomCorpus.
	selector := NewAtomSelector()

	evilID := "evil\\\" ) :- dangerous(X). #\n"
	identityID := "identity-1"

	kernel := &validatingKernel{
		results: []Fact{
			{Predicate: "selected_result", Args: []any{identityID, 100, "mandatory"}},
			{Predicate: "selected_result", Args: []any{evilID, 80, "vector_match"}},
		},
	}
	selector.SetKernel(kernel)
	selector.SetVectorSearcher(&mockVectorSearcher{
		results: map[string]float64{
			evilID: 0.99,
		},
	})

	cc := NewCompilationContext()
	cc.SemanticQuery = "query"
	cc.SemanticTopK = 10
	cc.OperationalMode = "/active"
	cc.ShardType = "Coder Shard"
	cc.Language = "/go"
	cc.FailingTestCount = 1

	atoms := []*PromptAtom{
		{ID: identityID, Category: CategoryIdentity, Content: "identity content"},
		{ID: evilID, Category: CategoryDomain, Content: "domain content"},
	}

	selected, err := selector.SelectAtoms(context.Background(), atoms, cc)
	require.NoError(t, err)
	require.NotEmpty(t, selected)
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

		atoms[0].NormalizeSelectors()
		atoms[1].NormalizeSelectors()

		result := selector.fallbackFleshSelection(atoms, vectorScores, cc, nil)

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

		result := selector.fallbackFleshSelection(atoms, vectorScores, cc, nil)

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
			facts: []any{
				Fact{Predicate: "selected_result", Args: []any{"identity-1", 100, "mandatory"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.loadSkeletonAtoms(context.Background(), atoms, NewCompilationContext(), nil)
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

		_, err := selector.loadSkeletonAtoms(context.Background(), atoms, NewCompilationContext(), nil)
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

		_, err := selector.loadSkeletonAtoms(context.Background(), atoms, NewCompilationContext(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no skeleton atoms")
	})

	t.Run("sets source to skeleton", func(t *testing.T) {
		selector := NewAtomSelector()

		atoms := []*PromptAtom{
			{ID: "identity-1", Category: CategoryIdentity, Content: "content"},
		}

		kernel := &mockKernel{
			facts: []any{
				Fact{Predicate: "selected_result", Args: []any{"identity-1", 100, "mandatory"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.loadSkeletonAtoms(context.Background(), atoms, NewCompilationContext(), nil)
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
			facts: []any{
				Fact{Predicate: "selected_result", Args: []any{"domain-1", 80, "context_match"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.loadFleshAtoms(context.Background(), atoms, NewCompilationContext(), nil)
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

		result, err := selector.loadFleshAtoms(context.Background(), atoms, NewCompilationContext(), nil)
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
		result, err := selector.loadFleshAtoms(context.Background(), atoms, NewCompilationContext(), nil)
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
			facts: []any{
				Fact{Predicate: "selected_result", Args: []any{"domain-1", 80, "vector_match"}},
			},
		}
		selector.SetKernel(kernel)

		vectorSearcher := &mockVectorSearcher{
			results: map[string]float64{"domain-1": 0.8},
		}
		selector.SetVectorSearcher(vectorSearcher)

		cc := NewCompilationContext().WithSemanticQuery("test query", 10)
		result, err := selector.loadFleshAtoms(context.Background(), atoms, cc, nil)
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
			facts: []any{
				Fact{Predicate: "selected_result", Args: []any{"domain-1", 80, "context_match"}},
			},
		}
		selector.SetKernel(kernel)

		result, err := selector.loadFleshAtoms(context.Background(), atoms, NewCompilationContext(), nil)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "flesh", result[0].Source)
	})
}

// Boundary Value Analysis: Remediated. See selector_gaps_test.go for comprehensive coverage.
// Findings: nil CompilationContext causes panic in GenerateFacts (known bug).
// mockKernel has unsynchronized state (production RealKernel uses sync.RWMutex).

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

	for i := range 100 {
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
			Args:      []any{a.ID, a.Priority, "benchmark"},
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
	for i := range 20 {
		skeleton[i] = &ScoredAtom{
			Atom:     &PromptAtom{ID: string(rune('s' + i)), Category: CategoryIdentity},
			Combined: float64(i) / 20,
			Source:   "skeleton",
		}
	}

	flesh := make([]*ScoredAtom, 80)
	for i := range 80 {
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

func TestAtomSelector_ExtractStringArg_UnknownTypes(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    string
		wantErr bool
	}{
		{"string", "hello", "hello", false},
		{"int", 42, "42", false},
		{"float64", 3.14, "3.14", false},
		{"bool", true, "true", false},
		{"nil", nil, "", false},
		{"unsupported struct", struct{ X int }{1}, "", true},
		{"unsupported slice", []int{1, 2, 3}, "", true},
		{"unsupported map", map[string]string{"a": "b"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractStringArg(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
