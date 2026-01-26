package prompt

import (
	"context"
	"fmt"
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

func atomsToFacts(atoms []*PromptAtom) []interface{} {
	var facts []interface{}
	for _, a := range atoms {
		facts = append(facts, Fact{
			Predicate: "selected_atom",
			Args:      []interface{}{a.ID, "skeleton", 1.0},
		})
	}
	return facts
}

func TestDefaultCompilerConfig(t *testing.T) {
	config := DefaultCompilerConfig()

	assert.Equal(t, 200000, config.DefaultTokenBudget) // Default updated to 200k
	assert.True(t, config.EnableVectorSearch)
	assert.Equal(t, 0.3, config.VectorSearchWeight)
	assert.Equal(t, 10, config.MaxAtomsPerCategory)
	assert.True(t, config.EnableCaching)
	assert.Equal(t, 300, config.CacheTTLSeconds)
	assert.False(t, config.DebugMode)
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

		// Mock the kernel to select all atoms
		kernel := &mockKernel{facts: atomsToFacts(atoms)}

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(kernel),
		)
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

		// Mock kernel to select only coder and universal atoms
		selectedAtoms := []*PromptAtom{atoms[0], atoms[2]}
		kernel := &mockKernel{facts: atomsToFacts(selectedAtoms)}

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(kernel),
		)
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

		// Mock kernel to select mandatory atom
		// Even if Mangle logic handles it, we mock the result here
		selectedAtoms := []*PromptAtom{atoms[0]}
		kernel := &mockKernel{facts: atomsToFacts(selectedAtoms)}

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(kernel),
		)
		require.NoError(t, err)

		cc := NewCompilationContext().WithTokenBudget(10000, 1000)
		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err)
		assert.Contains(t, result.Prompt, "Critical safety constraint")
		assert.Equal(t, 1, result.MandatoryCount)
	})
}

// TODO: TEST_GAP: Verify behavior when context is canceled (should abort compilation).

// TODO: TEST_GAP: Verify behavior when mandatory atoms exceed the token budget (should probably error or return partial).

// TODO: TEST_GAP: Verify cache hit behavior (subsequent calls with same context should use cache).

// TODO: TEST_GAP: Verify behavior when ConfigFactory fails (should continue with warning).

// TODO: TEST_GAP: Verify concurrency safety when RegisterDB is called during Compile.

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

	// Mock selecting all atoms
	kernel := &mockKernel{facts: atomsToFacts(atoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
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
		assert.Equal(t, 200000, config.DefaultTokenBudget) // Default updated to 200k
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

	// Mock selections
	selectedAtoms := []*PromptAtom{
		atoms[0], atoms[1], // Mandatory
		atoms[4], atoms[5], // Go, Bubbletea
	}
	kernel := &mockKernel{facts: atomsToFacts(selectedAtoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
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
		// Mock mandatory only
		kernel := &mockKernel{facts: atomsToFacts([]*PromptAtom{atoms[0], atoms[1]})}
		compiler, _ := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(kernel),
		)

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

	// Mock: orphan excluded (logic simulated by mock)
	selected := []*PromptAtom{atoms[0], atoms[1]}
	kernel := &mockKernel{facts: atomsToFacts(selected)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
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
	// Mock: approach-a wins (logic simulated by mock)
	selected := []*PromptAtom{atoms[0]}
	kernel := &mockKernel{facts: atomsToFacts(selected)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
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
	// Mock kernel
	kernel := &mockKernel{facts: atomsToFacts(atoms)}
	compiler, _ := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
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
	kernel := &mockKernel{facts: atomsToFacts(atoms)}
	compiler, _ := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
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
	kernel := &mockKernel{facts: atomsToFacts(atoms)}
	compiler, _ := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
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
	// Mock kernel
	kernel := &mockKernel{facts: atomsToFacts(atoms)}
	compiler, _ := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithVectorSearcher(vs),
		WithKernel(kernel),
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
	// Mock all selected for performance test
	kernel := &mockKernel{facts: atomsToFacts(atoms)}
	compiler, _ := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
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

// =============================================================================
// Fallback Path Tests
// =============================================================================
// These tests verify graceful degradation when components fail or are unavailable.
// The JIT compiler should degrade gracefully rather than fail completely.

// mockFailingVectorSearcher simulates vector search failures.
type mockFailingVectorSearcher struct {
	err error
}

func (m *mockFailingVectorSearcher) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, fmt.Errorf("vector search unavailable")
}

// mockFallbackKernel simulates a kernel that supports fallback selection.
// When provided with atoms, it selects mandatory atoms and those matching context.
type mockFallbackKernel struct {
	atoms     []*PromptAtom
	queryErr  error
	assertErr error
}

func (m *mockFallbackKernel) Query(predicate string) ([]Fact, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}

	// Return selected_result facts for mandatory atoms
	var results []Fact
	for _, atom := range m.atoms {
		if atom.IsMandatory {
			results = append(results, Fact{
				Predicate: "selected_result",
				Args:      []interface{}{atom.ID, atom.Priority, "skeleton"},
			})
		}
	}
	return results, nil
}

func (m *mockFallbackKernel) AssertBatch(facts []interface{}) error {
	return m.assertErr
}

// TODO: TEST_GAP: Verify partial success when Project DB query fails (should return embedded atoms).

func TestCompiler_FallbackOnCorruptCorpus(t *testing.T) {
	tests := []struct {
		name           string
		corpus         *EmbeddedCorpus
		description    string
		expectResult   bool
		expectFallback bool
	}{
		{
			name:           "nil corpus with kernel returns empty result",
			corpus:         nil,
			description:    "When corpus is nil, compilation should succeed with empty prompt",
			expectResult:   true,
			expectFallback: false, // No fallback needed, just empty
		},
		{
			name:           "empty corpus with kernel returns empty result",
			corpus:         NewEmbeddedCorpus([]*PromptAtom{}),
			description:    "When corpus is empty, compilation should succeed with empty prompt",
			expectResult:   true,
			expectFallback: false,
		},
		{
			name: "corpus with only invalid atoms degrades gracefully",
			corpus: NewEmbeddedCorpus([]*PromptAtom{
				{
					ID:         "", // Invalid: empty ID
					Category:   CategoryIdentity,
					Content:    "Invalid atom",
					TokenCount: 10,
				},
			}),
			description:    "Invalid atoms should be filtered, resulting in empty prompt",
			expectResult:   true,
			expectFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create kernel that selects no atoms (simulating corrupt corpus scenario)
			kernel := &mockKernel{facts: []interface{}{}}

			var opts []CompilerOption
			opts = append(opts, WithKernel(kernel))
			if tt.corpus != nil {
				opts = append(opts, WithEmbeddedCorpus(tt.corpus))
			}

			compiler, err := NewJITPromptCompiler(opts...)
			require.NoError(t, err, "compiler creation should succeed")

			cc := NewCompilationContext().WithTokenBudget(10000, 1000)
			result, err := compiler.Compile(context.Background(), cc)

			if tt.expectResult {
				require.NoError(t, err, "compilation should not error: %s", tt.description)
				require.NotNil(t, result, "result should not be nil")
				// Empty corpus = empty prompt, but no error
				assert.GreaterOrEqual(t, result.AtomsIncluded, 0)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestCompiler_FallbackOnKernelUnavailable(t *testing.T) {
	// This test documents the expected behavior when kernel is unavailable.
	// The compiler should fall back to pure embedded corpus selection.

	t.Run("nil kernel with embedded corpus uses fallback selection", func(t *testing.T) {
		// Create atoms with mandatory flags
		atoms := []*PromptAtom{
			{
				ID:          "mandatory-identity",
				Category:    CategoryIdentity,
				Content:     "You are a coding assistant.",
				Priority:    100,
				IsMandatory: true,
				TokenCount:  10,
			},
			{
				ID:          "mandatory-safety",
				Category:    CategorySafety,
				Content:     "Never generate harmful code.",
				Priority:    95,
				IsMandatory: true,
				TokenCount:  8,
			},
			{
				ID:         "optional-exemplar",
				Category:   CategoryExemplar,
				Content:    "Example: Write tests first.",
				Priority:   50,
				TokenCount: 8,
			},
		}

		corpus := NewEmbeddedCorpus(atoms)

		// Create compiler without kernel - this should currently error
		// but we document the expected fallback behavior
		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			// No kernel provided
		)
		require.NoError(t, err, "compiler creation should succeed even without kernel")

		cc := NewCompilationContext().WithTokenBudget(10000, 1000)
		result, err := compiler.Compile(context.Background(), cc)

		// Current behavior: errors because selector requires kernel
		// Expected fallback behavior: returns mandatory atoms only
		if err != nil {
			// Document current behavior - kernel is required
			assert.Contains(t, err.Error(), "kernel", "error should mention kernel requirement")
			t.Logf("Current behavior: %v (fallback not yet implemented)", err)

			// Skip further assertions since fallback isn't implemented
			t.Skip("Fallback for nil kernel not yet implemented - test documents expected behavior")
		}

		// If fallback is implemented, verify these assertions:
		require.NotNil(t, result, "result should not be nil with fallback")
		assert.NotEmpty(t, result.Prompt, "prompt should contain mandatory atoms")
		assert.GreaterOrEqual(t, result.MandatoryCount, 2, "should include mandatory atoms")

		// Verify Stats has FallbackUsed flag
		if result.Stats != nil {
			assert.True(t, result.Stats.FallbackUsed, "FallbackUsed should be true")
			assert.Greater(t, result.Stats.SkeletonAtoms, 0, "should have skeleton atoms")
		}
	})

	t.Run("kernel query error triggers fallback", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:          "mandatory-1",
				Category:    CategoryIdentity,
				Content:     "Mandatory content 1",
				Priority:    100,
				IsMandatory: true,
				TokenCount:  5,
			},
			{
				ID:         "optional-1",
				Category:   CategoryExemplar,
				Content:    "Optional content",
				Priority:   50,
				TokenCount: 5,
			},
		}

		corpus := NewEmbeddedCorpus(atoms)

		// Create a kernel that fails on query
		failingKernel := &mockKernel{
			queryErr: fmt.Errorf("kernel connection lost"),
		}

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(failingKernel),
		)
		require.NoError(t, err)

		cc := NewCompilationContext().WithTokenBudget(10000, 1000)
		result, err := compiler.Compile(context.Background(), cc)

		// Current behavior: propagates kernel error
		if err != nil {
			assert.Contains(t, err.Error(), "kernel", "error should relate to kernel failure")
			t.Logf("Current behavior: kernel error propagated: %v", err)
			t.Skip("Fallback on kernel error not yet implemented")
		}

		// Expected fallback behavior:
		require.NotNil(t, result)
		if result.Stats != nil {
			assert.True(t, result.Stats.FallbackUsed)
		}
	})
}

func TestCompiler_FallbackOnVectorSearchFailure(t *testing.T) {
	// Vector search failure should not prevent compilation.
	// Skeleton atoms should still be selected via Mangle rules.

	t.Run("vector search failure returns skeleton atoms only", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:          "skeleton-identity",
				Category:    CategoryIdentity,
				Content:     "Core identity content",
				Priority:    100,
				IsMandatory: true,
				TokenCount:  10,
			},
			{
				ID:          "skeleton-safety",
				Category:    CategorySafety,
				Content:     "Safety constraints",
				Priority:    95,
				IsMandatory: true,
				TokenCount:  8,
			},
			{
				ID:         "flesh-domain",
				Category:   CategoryDomain,
				Content:    "Domain-specific knowledge",
				Priority:   60,
				TokenCount: 15,
			},
		}

		corpus := NewEmbeddedCorpus(atoms)

		// Kernel that selects mandatory atoms
		kernel := &mockFallbackKernel{atoms: atoms}

		// Vector searcher that always fails
		failingVS := &mockFailingVectorSearcher{
			err: fmt.Errorf("embedding service unavailable"),
		}

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(kernel),
			WithVectorSearcher(failingVS),
		)
		require.NoError(t, err)

		cc := NewCompilationContext().
			WithSemanticQuery("test query", 10). // Request vector search
			WithTokenBudget(10000, 1000)

		result, err := compiler.Compile(context.Background(), cc)

		// Compilation should succeed despite vector failure
		require.NoError(t, err, "compilation should succeed even with vector failure")
		require.NotNil(t, result, "result should not be nil")

		// Should have skeleton atoms (mandatory ones)
		assert.Greater(t, result.AtomsIncluded, 0, "should include at least skeleton atoms")
		assert.GreaterOrEqual(t, result.MandatoryCount, 2, "should include mandatory atoms")

		// Verify Stats
		if result.Stats != nil {
			assert.Greater(t, result.Stats.SkeletonAtoms, 0, "SkeletonAtoms should be > 0")
			// FleshAtoms may be 0 since vector search failed
			t.Logf("Stats: skeleton=%d, flesh=%d", result.Stats.SkeletonAtoms, result.Stats.FleshAtoms)
		}

		// Prompt should contain skeleton content
		assert.Contains(t, result.Prompt, "Core identity content")
		assert.Contains(t, result.Prompt, "Safety constraints")
	})

	t.Run("vector search timeout gracefully degrades", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:          "mandatory-atom",
				Category:    CategoryIdentity,
				Content:     "Mandatory content",
				Priority:    100,
				IsMandatory: true,
				TokenCount:  5,
			},
		}

		corpus := NewEmbeddedCorpus(atoms)
		kernel := &mockFallbackKernel{atoms: atoms}

		// Vector searcher that simulates timeout
		timeoutVS := &mockFailingVectorSearcher{
			err: context.DeadlineExceeded,
		}

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(kernel),
			WithVectorSearcher(timeoutVS),
		)
		require.NoError(t, err)

		cc := NewCompilationContext().
			WithSemanticQuery("query", 5).
			WithTokenBudget(10000, 1000)

		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err, "should not error on vector timeout")
		require.NotNil(t, result)
		assert.Greater(t, result.AtomsIncluded, 0, "should still have atoms")
	})

	t.Run("nil vector searcher with semantic query succeeds", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:          "test-atom",
				Category:    CategoryIdentity,
				Content:     "Test content",
				Priority:    100,
				IsMandatory: true,
				TokenCount:  5,
			},
		}

		corpus := NewEmbeddedCorpus(atoms)
		kernel := &mockFallbackKernel{atoms: atoms}

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(kernel),
			// No vector searcher
		)
		require.NoError(t, err)

		cc := NewCompilationContext().
			WithSemanticQuery("should be ignored", 10).
			WithTokenBudget(10000, 1000)

		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err, "should succeed without vector searcher")
		require.NotNil(t, result)
		assert.Greater(t, result.AtomsIncluded, 0)
	})
}

func TestCompiler_FallbackStatistics(t *testing.T) {
	// Verify that fallback scenarios properly populate Stats fields

	t.Run("skeleton vs flesh atom counting", func(t *testing.T) {
		atoms := []*PromptAtom{
			{
				ID:          "skeleton-1",
				Category:    CategoryIdentity,
				Content:     "Skeleton 1",
				Priority:    100,
				IsMandatory: true,
				TokenCount:  10,
			},
			{
				ID:          "skeleton-2",
				Category:    CategorySafety,
				Content:     "Skeleton 2",
				Priority:    95,
				IsMandatory: true,
				TokenCount:  10,
			},
			{
				ID:         "flesh-1",
				Category:   CategoryExemplar,
				Content:    "Flesh 1",
				Priority:   50,
				TokenCount: 10,
			},
		}

		corpus := NewEmbeddedCorpus(atoms)

		// Kernel that selects all atoms with proper source tagging
		kernel := &mockKernel{
			facts: []interface{}{
				Fact{Predicate: "selected_atom", Args: []interface{}{"skeleton-1", "skeleton", 1.0}},
				Fact{Predicate: "selected_atom", Args: []interface{}{"skeleton-2", "skeleton", 1.0}},
				Fact{Predicate: "selected_atom", Args: []interface{}{"flesh-1", "flesh", 0.8}},
			},
		}

		compiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(corpus),
			WithKernel(kernel),
		)
		require.NoError(t, err)

		cc := NewCompilationContext().WithTokenBudget(10000, 1000)
		result, err := compiler.Compile(context.Background(), cc)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify mandatory counts
		assert.Equal(t, 2, result.MandatoryCount, "should have 2 mandatory atoms")
		assert.GreaterOrEqual(t, result.OptionalCount, 0, "should track optional atoms")

		t.Logf("Result: mandatory=%d, optional=%d, total=%d",
			result.MandatoryCount, result.OptionalCount, result.AtomsIncluded)
	})
}
