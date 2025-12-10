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

func TestAtomSelector_FilterByContext(t *testing.T) {
	atoms := []*PromptAtom{
		{ID: "match1", OperationalModes: []string{"/active"}, Content: "m1", Category: CategoryIdentity},
		{ID: "match2", OperationalModes: []string{"/active", "/debugging"}, Content: "m2", Category: CategoryIdentity},
		{ID: "nomatch", OperationalModes: []string{"/debugging"}, Content: "nm", Category: CategoryIdentity},
		{ID: "wildcard", Content: "wc", Category: CategoryIdentity}, // Empty selector matches all
	}

	selector := NewAtomSelector()
	cc := NewCompilationContext().WithOperationalMode("/active")

	matched := selector.filterByContext(atoms, cc)

	assert.Len(t, matched, 3) // match1, match2, wildcard

	matchedIDs := make([]string, len(matched))
	for i, a := range matched {
		matchedIDs[i] = a.ID
	}
	assert.Contains(t, matchedIDs, "match1")
	assert.Contains(t, matchedIDs, "match2")
	assert.Contains(t, matchedIDs, "wildcard")
	assert.NotContains(t, matchedIDs, "nomatch")
}

func TestAtomSelector_CalculateLogicScore(t *testing.T) {
	tests := []struct {
		name          string
		atom          *PromptAtom
		context       *CompilationContext
		minScore      float64
		maxScore      float64
		checkExact    bool
		expectedScore float64
	}{
		{
			name:          "nil context returns neutral score",
			atom:          &PromptAtom{ID: "test", ShardTypes: []string{"/coder"}},
			context:       nil,
			checkExact:    true,
			expectedScore: 0.5,
		},
		{
			name:          "no selector constraints returns neutral score",
			atom:          &PromptAtom{ID: "test"},
			context:       NewCompilationContext(),
			checkExact:    true,
			expectedScore: 0.5,
		},
		{
			name: "perfect match on single dimension",
			atom: &PromptAtom{ID: "test", ShardTypes: []string{"/coder"}},
			context: NewCompilationContext().WithShard("/coder", "", ""),
			checkExact:    true,
			expectedScore: 1.0,
		},
		{
			name: "partial match on multiple dimensions",
			atom: &PromptAtom{
				ID:               "test",
				ShardTypes:       []string{"/coder"},
				OperationalModes: []string{"/active"},
				IntentVerbs:      []string{"/create"}, // won't match
			},
			context: NewCompilationContext().
				WithShard("/coder", "", "").
				WithOperationalMode("/active").
				WithIntent("/fix", ""),
			minScore: 0.6, // 2/3 dimensions match
			maxScore: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := NewAtomSelector()
			score := selector.calculateLogicScore(tt.atom, tt.context)

			if tt.checkExact {
				assert.Equal(t, tt.expectedScore, score)
			} else {
				assert.GreaterOrEqual(t, score, tt.minScore)
				assert.LessOrEqual(t, score, tt.maxScore)
			}
		})
	}
}

func TestAtomSelector_ScoreAtom(t *testing.T) {
	t.Run("combines logic and vector scores", func(t *testing.T) {
		atom := &PromptAtom{
			ID:         "test",
			ShardTypes: []string{"/coder"},
			Priority:   50,
			Content:    "test",
			Category:   CategoryIdentity,
		}
		cc := NewCompilationContext().WithShard("/coder", "", "")
		vectorScores := map[string]float64{"test": 0.8}

		selector := NewAtomSelector()
		selector.SetVectorWeight(0.3)

		sa := selector.scoreAtom(atom, cc, vectorScores)

		assert.Greater(t, sa.Combined, 0.0)
		assert.Equal(t, 0.8, sa.VectorScore)
		assert.Greater(t, sa.LogicScore, 0.0)
	})

	t.Run("mandatory atoms get max score", func(t *testing.T) {
		atom := &PromptAtom{
			ID:          "mandatory",
			IsMandatory: true,
			Content:     "mandatory",
			Category:    CategorySafety,
		}
		cc := NewCompilationContext()

		selector := NewAtomSelector()
		sa := selector.scoreAtom(atom, cc, nil)

		assert.Equal(t, 1.0, sa.Combined)
		assert.Equal(t, "mandatory", sa.SelectionReason)
	})

	t.Run("priority adds boost", func(t *testing.T) {
		lowPriority := &PromptAtom{ID: "low", Priority: 10, Content: "low", Category: CategoryIdentity}
		highPriority := &PromptAtom{ID: "high", Priority: 100, Content: "high", Category: CategoryIdentity}
		cc := NewCompilationContext()

		selector := NewAtomSelector()
		lowSA := selector.scoreAtom(lowPriority, cc, nil)
		highSA := selector.scoreAtom(highPriority, cc, nil)

		assert.Greater(t, highSA.Combined, lowSA.Combined)
	})

	t.Run("score capped at 1.0", func(t *testing.T) {
		atom := &PromptAtom{
			ID:       "high-everything",
			Priority: 1000, // Very high priority
			Content:  "content",
			Category: CategoryIdentity,
		}
		cc := NewCompilationContext()
		vectorScores := map[string]float64{"high-everything": 1.0}

		selector := NewAtomSelector()
		sa := selector.scoreAtom(atom, cc, vectorScores)

		assert.LessOrEqual(t, sa.Combined, 1.0)
	})
}

func TestAtomSelector_SelectMandatory(t *testing.T) {
	atoms := []*PromptAtom{
		{ID: "optional1", IsMandatory: false, Content: "opt1", Category: CategoryIdentity},
		{ID: "mandatory1", IsMandatory: true, Content: "mand1", Category: CategorySafety},
		{ID: "optional2", IsMandatory: false, Content: "opt2", Category: CategoryIdentity},
		{ID: "mandatory2", IsMandatory: true, Content: "mand2", Category: CategoryProtocol},
	}

	selector := NewAtomSelector()
	mandatory := selector.SelectMandatory(atoms)

	assert.Len(t, mandatory, 2)

	for _, sa := range mandatory {
		assert.True(t, sa.Atom.IsMandatory)
		assert.Equal(t, 1.0, sa.Combined)
		assert.Equal(t, "mandatory", sa.SelectionReason)
	}
}

func TestAtomSelector_SelectByCategory(t *testing.T) {
	atoms := []*PromptAtom{
		{ID: "identity1", Category: CategoryIdentity, Content: "id1"},
		{ID: "identity2", Category: CategoryIdentity, Content: "id2"},
		{ID: "protocol1", Category: CategoryProtocol, Content: "proto1"},
		{ID: "safety1", Category: CategorySafety, Content: "safe1"},
	}

	selector := NewAtomSelector()
	cc := NewCompilationContext()

	identityAtoms := selector.SelectByCategory(atoms, CategoryIdentity, cc)

	assert.Len(t, identityAtoms, 2)
	for _, sa := range identityAtoms {
		assert.Equal(t, CategoryIdentity, sa.Atom.Category)
	}
}

func TestAtomSelector_FilterByExclusionGroups(t *testing.T) {
	t.Run("non-exclusive atoms all included", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a"}, Combined: 0.8},
			{Atom: &PromptAtom{ID: "b"}, Combined: 0.7},
			{Atom: &PromptAtom{ID: "c"}, Combined: 0.6},
		}

		selector := NewAtomSelector()
		result := selector.FilterByExclusionGroups(atoms)

		assert.Len(t, result, 3)
	})

	t.Run("exclusive group - only highest scored kept", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a", IsExclusive: "group1"}, Combined: 0.6},
			{Atom: &PromptAtom{ID: "b", IsExclusive: "group1"}, Combined: 0.9},
			{Atom: &PromptAtom{ID: "c", IsExclusive: "group1"}, Combined: 0.4},
		}

		selector := NewAtomSelector()
		result := selector.FilterByExclusionGroups(atoms)

		assert.Len(t, result, 1)
		assert.Equal(t, "b", result[0].Atom.ID)
	})

	t.Run("multiple exclusive groups", func(t *testing.T) {
		atoms := []*ScoredAtom{
			{Atom: &PromptAtom{ID: "a1", IsExclusive: "group1"}, Combined: 0.6},
			{Atom: &PromptAtom{ID: "a2", IsExclusive: "group1"}, Combined: 0.8},
			{Atom: &PromptAtom{ID: "b1", IsExclusive: "group2"}, Combined: 0.7},
			{Atom: &PromptAtom{ID: "b2", IsExclusive: "group2"}, Combined: 0.5},
			{Atom: &PromptAtom{ID: "c"}, Combined: 0.9}, // Not exclusive
		}

		selector := NewAtomSelector()
		result := selector.FilterByExclusionGroups(atoms)

		assert.Len(t, result, 3) // One from each group + non-exclusive

		ids := make([]string, len(result))
		for i, sa := range result {
			ids[i] = sa.Atom.ID
		}
		assert.Contains(t, ids, "a2") // Winner of group1
		assert.Contains(t, ids, "b1") // Winner of group2
		assert.Contains(t, ids, "c")  // Non-exclusive
	})
}

func TestReasonFromScores(t *testing.T) {
	tests := []struct {
		name       string
		logicScore float64
		vectorScore float64
		contains   string
	}{
		{"high logic and high vector", 0.9, 0.9, "high logic + high vector"},
		{"high logic only", 0.9, 0.3, "high logic match"},
		{"high vector only", 0.3, 0.9, "high semantic match"},
		{"moderate both", 0.6, 0.6, "moderate match"},
		{"threshold", 0.3, 0.3, "threshold match"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := reasonFromScores(tt.logicScore, tt.vectorScore)
			assert.Contains(t, reason, tt.contains)
		})
	}
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
			group = string(rune('a' + (i / 5) % 10))
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
