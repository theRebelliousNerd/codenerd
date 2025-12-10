package prompt

import (
	"context"
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

func TestAtomSelector_SelectAtoms(t *testing.T) {
	tests := []struct {
		name          string
		atoms         []*PromptAtom
		context       *CompilationContext
		vectorResults map[string]float64
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
				{ID: "a", Content: "content a", Category: CategoryIdentity},
				{ID: "b", Content: "content b", Category: CategoryProtocol},
			},
			context:     NewCompilationContext(),
			expectedLen: 2,
		},
		{
			name: "filters by shard type",
			atoms: []*PromptAtom{
				{ID: "coder-only", ShardTypes: []string{"/coder"}, Content: "coder", Category: CategoryIdentity},
				{ID: "tester-only", ShardTypes: []string{"/tester"}, Content: "tester", Category: CategoryIdentity},
				{ID: "both", ShardTypes: []string{"/coder", "/tester"}, Content: "both", Category: CategoryIdentity},
			},
			context:     NewCompilationContext().WithShard("/coder", "", ""),
			expectedLen: 2,
			expectedIDs: []string{"coder-only", "both"},
		},
		{
			name: "mandatory atoms always included",
			atoms: []*PromptAtom{
				{ID: "mandatory", IsMandatory: true, Content: "mandatory", Category: CategorySafety},
				{ID: "optional", ShardTypes: []string{"/tester"}, Content: "optional", Category: CategoryIdentity},
			},
			context:     NewCompilationContext().WithShard("/coder", "", ""),
			expectedLen: 1, // Only mandatory (optional doesn't match context)
			expectedIDs: []string{"mandatory"},
		},
		{
			name: "vector scores boost ranking",
			atoms: []*PromptAtom{
				{ID: "a", Content: "content a", Category: CategoryIdentity},
				{ID: "b", Content: "content b", Category: CategoryIdentity},
			},
			context: NewCompilationContext().WithSemanticQuery("test query", 10),
			vectorResults: map[string]float64{
				"a": 0.9,
				"b": 0.1,
			},
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := NewAtomSelector()

			if tt.vectorResults != nil {
				selector.SetVectorSearcher(&mockVectorSearcher{results: tt.vectorResults})
			}

			scored, err := selector.SelectAtoms(context.Background(), tt.atoms, tt.context)

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

func TestAtomSelector_SelectAtomsSortedByScore(t *testing.T) {
	atoms := []*PromptAtom{
		{ID: "low", Priority: 10, Content: "low", Category: CategoryIdentity},
		{ID: "high", Priority: 90, Content: "high", Category: CategoryIdentity},
		{ID: "medium", Priority: 50, Content: "medium", Category: CategoryIdentity},
	}

	selector := NewAtomSelector()
	scored, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())

	require.NoError(t, err)
	require.Len(t, scored, 3)

	// Verify sorted by combined score descending
	for i := 1; i < len(scored); i++ {
		assert.GreaterOrEqual(t, scored[i-1].Combined, scored[i].Combined)
	}
}

func TestAtomSelector_SelectAtomsMandatoryFirst(t *testing.T) {
	atoms := []*PromptAtom{
		{ID: "optional-high", Priority: 100, Content: "optional high", Category: CategoryIdentity},
		{ID: "mandatory", Priority: 10, IsMandatory: true, Content: "mandatory", Category: CategorySafety},
		{ID: "optional-low", Priority: 20, Content: "optional low", Category: CategoryIdentity},
	}

	selector := NewAtomSelector()
	scored, err := selector.SelectAtoms(context.Background(), atoms, NewCompilationContext())

	require.NoError(t, err)
	require.Len(t, scored, 3)

	// Mandatory should be first
	assert.Equal(t, "mandatory", scored[0].Atom.ID)
	assert.True(t, scored[0].Atom.IsMandatory)
}

// Benchmark tests

func BenchmarkSelectAtoms_SmallSet(b *testing.B) {
	atoms := make([]*PromptAtom, 20)
	for i := 0; i < 20; i++ {
		atoms[i] = &PromptAtom{
			ID:       string(rune('a' + i)),
			Priority: i * 5,
			Content:  "content",
			Category: CategoryIdentity,
		}
	}

	selector := NewAtomSelector()
	cc := NewCompilationContext()
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = selector.SelectAtoms(ctx, atoms, cc)
	}
}

func BenchmarkSelectAtoms_MediumSet(b *testing.B) {
	atoms := make([]*PromptAtom, 100)
	categories := AllCategories()
	for i := 0; i < 100; i++ {
		atoms[i] = &PromptAtom{
			ID:               string(rune(i)),
			Priority:         i,
			ShardTypes:       []string{"/coder"},
			OperationalModes: []string{"/active"},
			Content:          "content",
			Category:         categories[i%len(categories)],
		}
	}

	selector := NewAtomSelector()
	cc := NewCompilationContext().WithShard("/coder", "", "").WithOperationalMode("/active")
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = selector.SelectAtoms(ctx, atoms, cc)
	}
}

func BenchmarkSelectAtoms_WithVectorSearch(b *testing.B) {
	atoms := make([]*PromptAtom, 50)
	vectorResults := make(map[string]float64)
	for i := 0; i < 50; i++ {
		id := string(rune('a' + i))
		atoms[i] = &PromptAtom{
			ID:       id,
			Priority: i,
			Content:  "content",
			Category: CategoryIdentity,
		}
		vectorResults[id] = float64(i) / 50.0
	}

	selector := NewAtomSelector()
	selector.SetVectorSearcher(&mockVectorSearcher{results: vectorResults})
	cc := NewCompilationContext().WithSemanticQuery("test query", 20)
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = selector.SelectAtoms(ctx, atoms, cc)
	}
}

func BenchmarkCalculateLogicScore(b *testing.B) {
	atom := &PromptAtom{
		ID:               "benchmark",
		ShardTypes:       []string{"/coder", "/tester"},
		OperationalModes: []string{"/active", "/debugging"},
		IntentVerbs:      []string{"/fix", "/debug"},
		Languages:        []string{"/go", "/python"},
		Frameworks:       []string{"/bubbletea", "/gin"},
		WorldStates:      []string{"failing_tests"},
	}

	cc := &CompilationContext{
		ShardType:        "/coder",
		OperationalMode:  "/active",
		IntentVerb:       "/fix",
		Language:         "/go",
		Frameworks:       []string{"/bubbletea"},
		FailingTestCount: 5,
	}

	selector := NewAtomSelector()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		selector.calculateLogicScore(atom, cc)
	}
}

func BenchmarkFilterByExclusionGroups(b *testing.B) {
	atoms := make([]*ScoredAtom, 100)
	for i := 0; i < 100; i++ {
		group := ""
		if i%5 == 0 {
			group = string(rune('a' + (i/5)%10))
		}
		atoms[i] = &ScoredAtom{
			Atom:     &PromptAtom{ID: string(rune(i)), IsExclusive: group},
			Combined: float64(i) / 100.0,
		}
	}

	selector := NewAtomSelector()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		selector.FilterByExclusionGroups(atoms)
	}
}
