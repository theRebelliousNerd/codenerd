package prompt

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

// `nerd jit` prints five numbers and three of them were filled by nothing:
// GetStats built the struct with ShardDBCount and EmbeddedAtomCount and
// returned it, so the command reported "Project Atoms: 0 / Compilations: 0 /
// Avg Time (ms): 0.00" on a machine that had been compiling prompts all day.
//
// Zeros are the worst possible failure for a stats command. They read as a
// system that has not run, rather than as a command that is not looking, and
// the reader has no way to tell those apart.
func TestJITStatsCountTheCompilesThatActuallyRan(t *testing.T) {
	compiler := cacheProbeCompiler(t)

	if got := compiler.GetStats().TotalCompilations; got != 0 {
		t.Fatalf("a fresh compiler reports %d compilations", got)
	}

	_, err := compiler.Compile(context.Background(), NewCompilationContext().WithTokenBudget(10000, 1000))
	require.NoError(t, err)

	stats := compiler.GetStats()
	if stats.TotalCompilations != 1 {
		t.Errorf("TotalCompilations = %d after one compile, want 1", stats.TotalCompilations)
	}

	// A cache hit is not a compilation, it is an avoided one. Counting it would
	// make the number grow while the work stops, and would drag the average
	// towards zero the better the cache performed — a metric that improves as
	// its subject does nothing.
	_, err = compiler.Compile(context.Background(), NewCompilationContext().WithTokenBudget(10000, 1000))
	require.NoError(t, err)

	if got := compiler.GetStats().TotalCompilations; got != 1 {
		t.Errorf("TotalCompilations = %d after a cache hit, want 1", got)
	}
}

// The two numbers share a denominator on purpose, so a reader can multiply
// them back into total time without being wrong.
func TestTheAverageIsOverTheCompilesThatAreCounted(t *testing.T) {
	compiler := cacheProbeCompiler(t)

	for i := range 3 {
		// A different budget each time, so each is a distinct cache key and a
		// real compile rather than a hit.
		_, err := compiler.Compile(context.Background(),
			NewCompilationContext().WithTokenBudget(10000+i, 1000))
		require.NoError(t, err)
	}

	stats := compiler.GetStats()
	if stats.TotalCompilations != 3 {
		t.Fatalf("TotalCompilations = %d, want 3", stats.TotalCompilations)
	}

	// The average is checked against the accumulator rather than against zero.
	//
	// "AverageTimeMs > 0 after a real compile" is the assertion that reads
	// right and is wrong: this repo's CI runs Windows, where the monotonic
	// clock granularity is about half a millisecond, and a two-atom compile
	// finishes inside one tick. time.Since then returns 0 honestly and the
	// test failed on a platform difference rather than on a defect — which is
	// precisely what that Windows job exists to find, so it found one of mine.
	want := float64(atomic.LoadInt64(&compiler.compileNanos)) / 3 / float64(time.Millisecond)
	if stats.AverageTimeMs != want {
		t.Errorf("AverageTimeMs = %v, want %v (accumulator over three compiles)", stats.AverageTimeMs, want)
	}
}

// And the average must come from the accumulator with the right denominator,
// which is pinned by seeding the accumulator directly: a real compile cannot
// be relied on to take a measurable amount of time, and a test that needs it to
// is a test that fails on the fastest machine rather than the slowest.
func TestTheAverageIsTheAccumulatorOverTheCompileCount(t *testing.T) {
	compiler := cacheProbeCompiler(t)

	_, err := compiler.Compile(context.Background(), NewCompilationContext().WithTokenBudget(10000, 1000))
	require.NoError(t, err)

	// A duration no clock granularity can round away.
	atomic.StoreInt64(&compiler.compileNanos, int64(50*time.Millisecond))

	stats := compiler.GetStats()
	if stats.TotalCompilations != 1 {
		t.Fatalf("TotalCompilations = %d, want 1", stats.TotalCompilations)
	}
	if stats.AverageTimeMs != 50 {
		t.Errorf("AverageTimeMs = %v over one compile of 50ms, want 50", stats.AverageTimeMs)
	}

	// Two more compiles with the same accumulated time must divide it three
	// ways.
	for i := range 2 {
		_, err := compiler.Compile(context.Background(),
			NewCompilationContext().WithTokenBudget(20000+i, 1000))
		require.NoError(t, err)
	}
	atomic.StoreInt64(&compiler.compileNanos, int64(90*time.Millisecond))

	if got := compiler.GetStats().AverageTimeMs; got != 30 {
		t.Errorf("AverageTimeMs = %v over three compiles of 90ms total, want 30", got)
	}

	// And a CACHE HIT must not move it. This is the case that distinguishes the
	// right denominator from the plausible one: a hit adds a call and no
	// compile, so an average taken over calls drops here while the work has not
	// changed. Without this the two denominators are indistinguishable, because
	// every compile above has a distinct key and the hit count is zero.
	_, err = compiler.Compile(context.Background(), NewCompilationContext().WithTokenBudget(10000, 1000))
	require.NoError(t, err)

	if got := compiler.GetStats().AverageTimeMs; got != 30 {
		t.Errorf("AverageTimeMs = %v after a cache hit, want 30 — the average is being "+
			"taken over calls rather than over the compiles that actually ran", got)
	}
}

// ProjectAtomCount is the third of the three zeros, and it is a query rather
// than a counter: the project corpus is a database, not a slice.
//
// It is tested rather than assumed for the same reason every other line on
// this branch is. "SELECT COUNT(*) FROM prompt_atoms" against the wrong handle,
// or against a schema whose table is named something else, returns an error
// that this code deliberately swallows into a debug line — so the failure mode
// of getting it wrong is the exact zero it was written to replace.
func TestJITStatsCountTheProjectCorpus(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "corpus.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE prompt_atoms (
		atom_id TEXT PRIMARY KEY, content TEXT, description TEXT,
		embedding BLOB, embedding_task TEXT, source_file TEXT)`)
	require.NoError(t, err)
	for _, id := range []string{"a", "b", "c"} {
		_, err = db.Exec("INSERT INTO prompt_atoms (atom_id, content) VALUES (?, ?)", id, "x")
		require.NoError(t, err)
	}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(NewEmbeddedCorpus(nil)),
		WithProjectDB(db),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = compiler.Close() })

	if got := compiler.GetStats().ProjectAtomCount; got != 3 {
		t.Errorf("ProjectAtomCount = %d, want 3", got)
	}
}

// And a compiler with no project corpus reports zero rather than failing. An
// unscanned workspace is the ordinary state, not an error, and a stats command
// that errors on it is a stats command nobody runs.
func TestJITStatsSurviveAMissingProjectCorpus(t *testing.T) {
	compiler := cacheProbeCompiler(t)
	if got := compiler.GetStats().ProjectAtomCount; got != 0 {
		t.Errorf("ProjectAtomCount = %d with no project DB, want 0", got)
	}
}

// A consumer must not be able to change what the cache holds.
//
// The compiler keeps an LRU and, on a MISS, hands back the very object it just
// stored — so a caller that appends to result.Prompt writes into the cache
// entry, and every later hit on that context serves a prompt the compiler did
// not produce. internal/articulation did exactly that with the Piggyback
// suffix. TestCompiledPromptIsByteStable cannot see it: it compares what the
// compiler returns, not what a consumer did to it afterwards.
//
// This is the compiler-side half of the pin. It does not stop a caller
// mutating what it is given — Go has no way to — but it fails if the SECOND
// compile of a context ever reflects the first caller's edit, which is the
// only consequence that reaches a model.
func TestACallerEditingItsResultCannotReachTheCache(t *testing.T) {
	compiler := cacheProbeCompiler(t)
	ctxFor := func() *CompilationContext { return NewCompilationContext().WithTokenBudget(10000, 1000) }

	first, err := compiler.Compile(context.Background(), ctxFor())
	require.NoError(t, err)
	original := first.Prompt
	require.NotEmpty(t, original, "an empty prompt proves nothing here")

	// A consumer doing what articulation used to do.
	first.Prompt += "\n\nAPPENDED BY A CONSUMER"

	second, err := compiler.Compile(context.Background(), ctxFor())
	require.NoError(t, err)

	if strings.Contains(second.Prompt, "APPENDED BY A CONSUMER") {
		t.Error("a later compile served a consumer's edit back out of the cache; " +
			"the prompt the model sees is no longer the prompt the compiler built")
	}
	if second.Prompt != original {
		t.Errorf("the cached prompt changed after a consumer edited its copy:\n got %d bytes\nwant %d bytes",
			len(second.Prompt), len(original))
	}
}
