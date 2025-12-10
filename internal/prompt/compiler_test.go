package prompt

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockKernel implements KernelQuerier for testing.
type mockKernel struct {
	facts     []interface{}
	queryErr  error
	assertErr error
}

func (m *mockKernel) Query(predicate string) ([]Fact, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	// Cast stored interface{} facts to Fact
	var result []Fact
	for _, f := range m.facts {
		if fact, ok := f.(Fact); ok {
			result = append(result, fact)
		}
	}
	return result, nil
}

func (m *mockKernel) AssertBatch(facts []interface{}) error {
	if m.assertErr != nil {
		return m.assertErr
	}
	m.facts = append(m.facts, facts...)
	return nil
}

func TestDefaultCompilerConfig(t *testing.T) {
	config := DefaultCompilerConfig()

	assert.Equal(t, 100000, config.DefaultTokenBudget)
	assert.True(t, config.EnableVectorSearch)
	assert.Equal(t, 0.3, config.VectorSearchWeight)
	assert.Equal(t, 10, config.MaxAtomsPerCategory)
	assert.True(t, config.EnableCaching)
	assert.Equal(t, 300, config.CacheTTLSeconds)
}

func TestNewJITPromptCompiler(t *testing.T) {
	t.Run("creates compiler with defaults", func(t *testing.T) {
		compiler, err := NewJITPromptCompiler()

		require.NoError(t, err)
		require.NotNil(t, compiler)

		assert.NotNil(t, compiler.selector)
		assert.NotNil(t, compiler.resolver)
		assert.NotNil(t, compiler.budgetMgr)
		assert.NotNil(t, compiler.assembler)
		assert.NotNil(t, compiler.shardDBs)
	})

	t.Run("applies options", func(t *testing.T) {
		corpus := NewEmbeddedCorpus([]*PromptAtom{
			NewPromptAtom("test", CategoryIdentity, "Test content"),
		})

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithConfig(CompilerConfig{DefaultTokenBudget: 50000}),
		)

		require.NoError(t, err)
		assert.NotNil(t, compiler.embeddedCorpus)
		assert.Equal(t, 50000, compiler.config.DefaultTokenBudget)
	})

	t.Run("with kernel option", func(t *testing.T) {
		kernel := &mockKernel{}
		compiler, err := NewJITPromptCompiler(WithKernel(kernel))

		require.NoError(t, err)
		assert.Equal(t, kernel, compiler.kernel)
	})

	t.Run("with vector searcher option", func(t *testing.T) {
		vs := &mockVectorSearcher{}
		compiler, err := NewJITPromptCompiler(WithVectorSearcher(vs))

		require.NoError(t, err)
		assert.Equal(t, vs, compiler.vectorSearcher)
	})
}

func TestJITPromptCompiler_Compile(t *testing.T) {
	t.Run("nil context returns error", func(t *testing.T) {
		compiler, err := NewJITPromptCompiler()
		require.NoError(t, err)

		result, err := compiler.Compile(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "compilation context is required")
	})

	t.Run("invalid context returns error", func(t *testing.T) {
		compiler, err := NewJITPromptCompiler()
		require.NoError(t, err)

		cc := &CompilationContext{
			TokenBudget:    0, // Invalid
			ReservedTokens: 100,
		}

		result, err := compiler.Compile(context.Background(), cc)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid compilation context")
	})

	t.Run("empty corpus returns empty prompt", func(t *testing.T) {
		compiler, err := NewJITPromptCompiler()
		require.NoError(t, err)

		cc := NewCompilationContext()
		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Prompt)
		assert.Equal(t, 0, result.AtomsIncluded)
	})

	t.Run("compiles from embedded corpus", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:       "identity/coder",
				Category: CategoryIdentity,
				Content:  "You are the Coder Shard, a specialized AI coding agent.",
				Priority: 100,
			},
			{
				ID:          "protocol/piggyback",
				Category:    CategoryProtocol,
				Content:     "Output your response in the Piggyback format.",
				Priority:    90,
				IsMandatory: true,
			},
		}

		// Compute token counts and hashes
		for _, atom := range atoms {
			atom.TokenCount = EstimateTokens(atom.Content)
			atom.ContentHash = HashContent(atom.Content)
		}

		corpus := NewEmbeddedCorpus(atoms)

		compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
		require.NoError(t, err)

		cc := NewCompilationContext().WithTokenBudget(10000, 1000)
		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err)
		assert.NotEmpty(t, result.Prompt)
		assert.Contains(t, result.Prompt, "Coder Shard")
		assert.Contains(t, result.Prompt, "Piggyback")
		assert.GreaterOrEqual(t, result.AtomsIncluded, 1)
	})

	t.Run("respects context filtering", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:         "coder-only",
				Category:   CategoryIdentity,
				Content:    "Coder-specific content",
				ShardTypes: []string{"/coder"},
				TokenCount: 10,
			},
			{
				ID:         "tester-only",
				Category:   CategoryIdentity,
				Content:    "Tester-specific content",
				ShardTypes: []string{"/tester"},
				TokenCount: 10,
			},
			{
				ID:         "universal",
				Category:   CategoryProtocol,
				Content:    "Universal content",
				TokenCount: 10,
			},
		}

		corpus := NewEmbeddedCorpus(atoms)

		compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
		require.NoError(t, err)

		cc := NewCompilationContext().
			WithShard("/coder", "", "").
			WithTokenBudget(10000, 1000)

		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err)
		assert.Contains(t, result.Prompt, "Coder-specific")
		assert.NotContains(t, result.Prompt, "Tester-specific")
		assert.Contains(t, result.Prompt, "Universal")
	})

	t.Run("mandatory atoms always included", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:          "mandatory-safety",
				Category:    CategorySafety,
				Content:     "Critical safety constraint",
				IsMandatory: true,
				TokenCount:  100,
			},
			{
				ID:         "optional",
				Category:   CategoryExemplar,
				Content:    "Optional example",
				TokenCount: 100,
			},
		}

		corpus := NewEmbeddedCorpus(atoms)

		compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
		require.NoError(t, err)

		cc := NewCompilationContext().WithTokenBudget(10000, 1000)
		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err)
		assert.Contains(t, result.Prompt, "Critical safety constraint")
		assert.Equal(t, 1, result.MandatoryCount)
	})
}

func TestJITPromptCompiler_CompileResult(t *testing.T) {
	atoms := []*PromptAtom{
		{
			ID:          "identity",
			Category:    CategoryIdentity,
			Content:     "Identity content",
			TokenCount:  50,
			IsMandatory: true,
		},
		{
			ID:         "protocol",
			Category:   CategoryProtocol,
			Content:    "Protocol content",
			TokenCount: 30,
		},
		{
			ID:         "exemplar",
			Category:   CategoryExemplar,
			Content:    "Example content",
			TokenCount: 20,
		},
	}

	corpus := NewEmbeddedCorpus(atoms)

	compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	require.NoError(t, err)

	cc := NewCompilationContext().WithTokenBudget(10000, 1000)
	result, err := compiler.Compile(context.Background(), cc)

	require.NoError(t, err)

	t.Run("includes prompt text", func(t *testing.T) {
		assert.NotEmpty(t, result.Prompt)
	})

	t.Run("tracks included atoms", func(t *testing.T) {
		assert.NotEmpty(t, result.IncludedAtoms)
	})

	t.Run("calculates token statistics", func(t *testing.T) {
		assert.Greater(t, result.TotalTokens, 0)
	})

	t.Run("tracks mandatory vs optional", func(t *testing.T) {
		assert.Equal(t, 1, result.MandatoryCount)
		assert.GreaterOrEqual(t, result.OptionalCount, 0)
	})

	t.Run("tracks category tokens", func(t *testing.T) {
		assert.NotEmpty(t, result.CategoryTokens)
	})

	t.Run("calculates budget usage", func(t *testing.T) {
		assert.Greater(t, result.BudgetUsed, 0.0)
		assert.LessOrEqual(t, result.BudgetUsed, 1.0)
	})

	t.Run("tracks candidate and selection counts", func(t *testing.T) {
		assert.Equal(t, 3, result.AtomsCandidates)
		assert.GreaterOrEqual(t, result.AtomsSelected, 0)
		assert.GreaterOrEqual(t, result.AtomsIncluded, 0)
	})
}

func TestJITPromptCompiler_RegisterShardDB(t *testing.T) {
	compiler, err := NewJITPromptCompiler()
	require.NoError(t, err)

	// Since we can't easily create a mock *sql.DB, test the map operations
	t.Run("register adds to map", func(t *testing.T) {
		compiler.RegisterShardDB("shard-1", nil)
		assert.Contains(t, compiler.shardDBs, "shard-1")
	})

	t.Run("unregister removes from map", func(t *testing.T) {
		compiler.RegisterShardDB("shard-2", nil)
		assert.Contains(t, compiler.shardDBs, "shard-2")

		compiler.UnregisterShardDB("shard-2")
		assert.NotContains(t, compiler.shardDBs, "shard-2")
	})
}

func TestJITPromptCompiler_GetSetConfig(t *testing.T) {
	compiler, err := NewJITPromptCompiler()
	require.NoError(t, err)

	t.Run("get returns current config", func(t *testing.T) {
		config := compiler.GetConfig()
		assert.Equal(t, 100000, config.DefaultTokenBudget)
	})

	t.Run("set updates config", func(t *testing.T) {
		newConfig := CompilerConfig{
			DefaultTokenBudget: 50000,
			EnableVectorSearch: false,
		}

		compiler.SetConfig(newConfig)
		config := compiler.GetConfig()

		assert.Equal(t, 50000, config.DefaultTokenBudget)
		assert.False(t, config.EnableVectorSearch)
	})
}

func TestJITPromptCompiler_GetStats(t *testing.T) {
	atoms := []*PromptAtom{
		NewPromptAtom("a", CategoryIdentity, "A"),
		NewPromptAtom("b", CategoryProtocol, "B"),
	}

	corpus := NewEmbeddedCorpus(atoms)

	compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	require.NoError(t, err)

	stats := compiler.GetStats()

	assert.Equal(t, 2, stats.EmbeddedAtomCount)
	assert.Equal(t, 0, stats.ShardDBCount)
}

func TestJITPromptCompiler_Close(t *testing.T) {
	compiler, err := NewJITPromptCompiler()
	require.NoError(t, err)

	// Just verify Close doesn't panic
	err = compiler.Close()
	assert.NoError(t, err)
}

// Integration tests

func TestCompileEndToEnd(t *testing.T) {
	// Create a realistic set of atoms
	atoms := []*PromptAtom{
		{
			ID:          "identity/core",
			Category:    CategoryIdentity,
			Content:     "You are a specialized AI coding assistant for the codeNERD project.",
			Priority:    100,
			IsMandatory: true,
			TokenCount:  20,
		},
		{
			ID:          "safety/constitution",
			Category:    CategorySafety,
			Content:     "You must never generate harmful code or reveal sensitive information.",
			Priority:    95,
			IsMandatory: true,
			TokenCount:  15,
		},
		{
			ID:         "protocol/piggyback",
			Category:   CategoryProtocol,
			Content:    "Format your output as JSON with 'surface' and 'control' channels.",
			Priority:   90,
			TokenCount: 15,
		},
		{
			ID:         "methodology/tdd",
			Category:   CategoryMethodology,
			Content:    "Use Test-Driven Development: write tests first, then implementation.",
			Priority:   80,
			TokenCount: 15,
		},
		{
			ID:         "language/go",
			Category:   CategoryLanguage,
			Content:    "Follow Go best practices: error handling, context propagation, etc.",
			Priority:   70,
			Languages:  []string{"/go"},
			TokenCount: 15,
		},
		{
			ID:         "framework/bubbletea",
			Category:   CategoryFramework,
			Content:    "Use the Model-View-Update pattern with Bubbletea.",
			Priority:   60,
			Frameworks: []string{"/bubbletea"},
			TokenCount: 12,
		},
		{
			ID:          "exemplar/fix",
			Category:    CategoryExemplar,
			Content:     "Example: When fixing a bug, first reproduce it with a test.",
			Priority:    40,
			IntentVerbs: []string{"/fix"},
			TokenCount:  15,
		},
	}

	corpus := NewEmbeddedCorpus(atoms)

	compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	require.NoError(t, err)

	t.Run("full context compilation", func(t *testing.T) {
		cc := NewCompilationContext().
			WithShard("/coder", "coder-1", "Primary Coder").
			WithLanguage("/go", "/bubbletea").
			WithIntent("/fix", "bug.go").
			WithOperationalMode("/active").
			WithTokenBudget(10000, 1000)

		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err)
		assert.NotEmpty(t, result.Prompt)

		// Mandatory atoms should be present
		assert.Contains(t, result.Prompt, "coding assistant")
		assert.Contains(t, result.Prompt, "harmful code")

		// Context-relevant atoms should be present
		assert.Contains(t, result.Prompt, "Go best practices")
		assert.Contains(t, result.Prompt, "Bubbletea")

		// Verify counts
		assert.Equal(t, 2, result.MandatoryCount)
		assert.Greater(t, result.OptionalCount, 0)
	})

	t.Run("minimal context compilation", func(t *testing.T) {
		cc := NewCompilationContext().WithTokenBudget(5000, 500)

		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err)
		assert.NotEmpty(t, result.Prompt)

		// At least mandatory atoms should be present
		assert.GreaterOrEqual(t, result.MandatoryCount, 2)
	})
}

func TestCompileWithDependencies(t *testing.T) {
	atoms := []*PromptAtom{
		{
			ID:         "base",
			Category:   CategoryIdentity,
			Content:    "Base identity",
			Priority:   100,
			TokenCount: 10,
		},
		{
			ID:         "dependent",
			Category:   CategoryMethodology,
			Content:    "Dependent methodology",
			Priority:   90,
			DependsOn:  []string{"base"},
			TokenCount: 10,
		},
		{
			ID:         "orphan",
			Category:   CategoryExemplar,
			Content:    "Orphan with missing dep",
			Priority:   80,
			DependsOn:  []string{"nonexistent"},
			TokenCount: 10,
		},
	}

	corpus := NewEmbeddedCorpus(atoms)
	compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	require.NoError(t, err)

	cc := NewCompilationContext().WithTokenBudget(10000, 1000)
	result, err := compiler.Compile(context.Background(), cc)

	require.NoError(t, err)

	// Base and dependent should be included
	assert.Contains(t, result.Prompt, "Base identity")
	assert.Contains(t, result.Prompt, "Dependent methodology")

	// Orphan should be excluded due to missing dependency
	assert.NotContains(t, result.Prompt, "Orphan")
}

func TestCompileWithConflicts(t *testing.T) {
	atoms := []*PromptAtom{
		{
			ID:            "approach-a",
			Category:      CategoryMethodology,
			Content:       "Use approach A",
			Priority:      90,
			ConflictsWith: []string{"approach-b"},
			TokenCount:    10,
		},
		{
			ID:            "approach-b",
			Category:      CategoryMethodology,
			Content:       "Use approach B",
			Priority:      80,
			ConflictsWith: []string{"approach-a"},
			TokenCount:    10,
		},
	}

	corpus := NewEmbeddedCorpus(atoms)
	compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	require.NoError(t, err)

	cc := NewCompilationContext().WithTokenBudget(10000, 1000)
	result, err := compiler.Compile(context.Background(), cc)

	require.NoError(t, err)

	// Only one approach should be included (higher priority wins)
	hasA := strings.Contains(result.Prompt, "approach A")
	hasB := strings.Contains(result.Prompt, "approach B")

	assert.True(t, hasA != hasB, "exactly one conflicting atom should be included")
	assert.True(t, hasA, "higher priority approach-a should win")
}

// Benchmark tests

func BenchmarkCompile_SmallCorpus(b *testing.B) {
	atoms := make([]*PromptAtom, 20)
	categories := AllCategories()
	for i := 0; i < 20; i++ {
		atoms[i] = &PromptAtom{
			ID:         string(rune('a' + i)),
			Category:   categories[i%len(categories)],
			Content:    "Content " + string(rune('a'+i)),
			Priority:   i * 5,
			TokenCount: 10,
		}
	}

	corpus := NewEmbeddedCorpus(atoms)
	compiler, _ := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	cc := NewCompilationContext().WithTokenBudget(10000, 1000)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = compiler.Compile(ctx, cc)
	}
}

func BenchmarkCompile_MediumCorpus(b *testing.B) {
	atoms := make([]*PromptAtom, 100)
	categories := AllCategories()
	for i := 0; i < 100; i++ {
		atoms[i] = &PromptAtom{
			ID:         string(rune(i)),
			Category:   categories[i%len(categories)],
			Content:    strings.Repeat("word ", 50),
			Priority:   i,
			TokenCount: 50,
		}
	}

	corpus := NewEmbeddedCorpus(atoms)
	compiler, _ := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	cc := NewCompilationContext().
		WithShard("/coder", "", "").
		WithLanguage("/go").
		WithTokenBudget(50000, 5000)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = compiler.Compile(ctx, cc)
	}
}

func BenchmarkCompile_LargeCorpus(b *testing.B) {
	atoms := make([]*PromptAtom, 500)
	categories := AllCategories()
	for i := 0; i < 500; i++ {
		atoms[i] = &PromptAtom{
			ID:          string(rune(i)),
			Category:    categories[i%len(categories)],
			Content:     strings.Repeat("word ", 100),
			Priority:    i % 100,
			TokenCount:  100,
			IsMandatory: i < 10,
		}
	}

	corpus := NewEmbeddedCorpus(atoms)
	compiler, _ := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	cc := NewCompilationContext().
		WithShard("/coder", "", "").
		WithLanguage("/go", "/bubbletea").
		WithOperationalMode("/active").
		WithIntent("/fix", "").
		WithTokenBudget(100000, 10000)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = compiler.Compile(ctx, cc)
	}
}

func BenchmarkCompile_WithVectorSearch(b *testing.B) {
	atoms := make([]*PromptAtom, 50)
	categories := AllCategories()
	vectorResults := make(map[string]float64)

	for i := 0; i < 50; i++ {
		id := string(rune('a' + i))
		atoms[i] = &PromptAtom{
			ID:         id,
			Category:   categories[i%len(categories)],
			Content:    strings.Repeat("content ", 20),
			Priority:   i,
			TokenCount: 20,
		}
		vectorResults[id] = float64(i) / 50.0
	}

	corpus := NewEmbeddedCorpus(atoms)
	vs := &mockVectorSearcher{results: vectorResults}
	compiler, _ := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithVectorSearcher(vs),
	)
	cc := NewCompilationContext().
		WithSemanticQuery("test query", 20).
		WithTokenBudget(50000, 5000)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = compiler.Compile(ctx, cc)
	}
}

// Performance verification test
func TestCompilePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	atoms := make([]*PromptAtom, 200)
	categories := AllCategories()
	for i := 0; i < 200; i++ {
		atoms[i] = &PromptAtom{
			ID:          string(rune(i)),
			Category:    categories[i%len(categories)],
			Content:     strings.Repeat("word ", 50),
			Priority:    i % 100,
			TokenCount:  50,
			IsMandatory: i < 5,
		}
	}

	corpus := NewEmbeddedCorpus(atoms)
	compiler, _ := NewJITPromptCompiler(WithEmbeddedCorpus(corpus))
	cc := NewCompilationContext().
		WithShard("/coder", "", "").
		WithLanguage("/go", "/bubbletea").
		WithTokenBudget(100000, 10000)
	ctx := context.Background()

	// Run 100 compilations and check average time
	start := time.Now()
	const iterations = 100

	for i := 0; i < iterations; i++ {
		_, err := compiler.Compile(ctx, cc)
		require.NoError(t, err)
	}

	elapsed := time.Since(start)
	avgMs := float64(elapsed.Milliseconds()) / float64(iterations)

	t.Logf("Average compilation time: %.2f ms", avgMs)

	// Compilation should be under 50ms on average
	assert.Less(t, avgMs, 50.0, "Average compilation time should be < 50ms")
}
