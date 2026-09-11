package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// CompilationStats.CacheHit had three readers — the stats summary line,
// ToLogFields()["cache_hit"], and the glass-box "(cache hit)" suffix — and no
// writer anywhere in the repository. It reported false on every compilation
// ever done, including the ones served straight out of the LRU.
//
// The counter beside it works, so "how many hits" was answerable all along.
// What was not is the per-compilation flag, which is what any correlation
// between a hit and what that turn actually cost has to be joined on — and
// this branch's whole argument is about what the cacheable prefix costs.
func TestACachedCompilationSaysItWasCached(t *testing.T) {
	compiler := cacheProbeCompiler(t)
	cc := NewCompilationContext().WithTokenBudget(10000, 1000)

	first, err := compiler.Compile(context.Background(), cc)
	require.NoError(t, err)
	require.NotNil(t, first.Stats, "no stats, so this test cannot see the flag")
	if first.Stats.CacheHit {
		t.Fatal("the first compilation of a context reported a cache hit")
	}

	second, err := compiler.Compile(context.Background(), NewCompilationContext().WithTokenBudget(10000, 1000))
	require.NoError(t, err)
	require.NotNil(t, second.Stats)

	if second.Prompt != first.Prompt {
		t.Fatalf("the second compile did not come from the cache, so the flag is "+
			"not what is under test here (%d vs %d bytes)", len(second.Prompt), len(first.Prompt))
	}
	if !second.Stats.CacheHit {
		t.Error("a compilation served from the LRU reported CacheHit false; every " +
			"reader of it — the summary line, the log fields, the glass box — has " +
			"been saying the prompt cache never hits")
	}
}

// And the flag must not travel backwards. The cache stores one
// *CompilationResult and hands the same pointer to every hit, so setting
// CacheHit on it in place would mark the ORIGINAL compilation as a hit
// retroactively — and would race two callers writing the same field.
func TestMarkingAHitDoesNotRewriteTheCompilationThatFilledTheCache(t *testing.T) {
	compiler := cacheProbeCompiler(t)

	first, err := compiler.Compile(context.Background(), NewCompilationContext().WithTokenBudget(10000, 1000))
	require.NoError(t, err)
	require.NotNil(t, first.Stats)

	_, err = compiler.Compile(context.Background(), NewCompilationContext().WithTokenBudget(10000, 1000))
	require.NoError(t, err)

	if first.Stats.CacheHit {
		t.Error("serving a cache hit rewrote the stats of the compilation that " +
			"filled the cache; the caller still holding that result now believes " +
			"its own compile was free")
	}
}

func cacheProbeCompiler(t *testing.T) *JITPromptCompiler {
	t.Helper()
	atoms := []*PromptAtom{
		{
			ID:          "identity/coder",
			Category:    CategoryIdentity,
			Content:     "You are the Coder Shard.",
			Priority:    100,
			IsMandatory: true,
		},
		{
			ID:          "safety/scope",
			Category:    CategorySafety,
			Content:     "Do not widen the change beyond what was asked.",
			Priority:    60,
			IsMandatory: true,
		},
	}
	for _, atom := range atoms {
		atom.TokenCount = EstimateTokens(atom.Content)
		atom.ContentHash = HashContent(atom.Content)
	}
	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(NewEmbeddedCorpus(atoms)),
		WithKernel(&mockKernel{facts: atomsToFacts(atoms)}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = compiler.Close() })
	return compiler
}
