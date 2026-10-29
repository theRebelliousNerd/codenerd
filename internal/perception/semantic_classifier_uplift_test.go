package perception

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// countingEngine is a configurable-dimension embedding engine for store tests.
type countingEngine struct {
	mu    sync.Mutex
	dims  int
	calls int
	vec   []float32
}

func (e *countingEngine) Name() string    { return "counting" }
func (e *countingEngine) Dimensions() int { return e.dims }
func (e *countingEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	out := make([]float32, len(e.vec))
	copy(out, e.vec)
	return out, nil
}
func (e *countingEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		vec, _ := e.Embed(ctx, texts[i])
		out[i] = vec
	}
	return out, nil
}
func (e *countingEngine) numCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

// TestSearchInMemory_DeterministicTies pins total ordering: entries with
// identical cosine similarity order by verb, identically on every call. A
// bare unstable sort flips the few-shot exemplar order run to run.
func TestSearchInMemory_DeterministicTies(t *testing.T) {
	store, err := NewEmbeddedCorpusStore(2)
	if err != nil {
		t.Fatalf("NewEmbeddedCorpusStore: %v", err)
	}
	store.entries = []CorpusEntry{
		{TextContent: "bravo text", Verb: "/zzz"},
		{TextContent: "alpha text", Verb: "/aaa"},
		{TextContent: "mid text", Verb: "/mmm"},
	}
	store.embeddings = map[string][]float32{
		"bravo text": {1, 0},
		"alpha text": {1, 0},
		"mid text":   {0, 1},
	}
	var first []string
	for i := 0; i < 50; i++ {
		results, err := store.Search([]float32{1, 0}, 5)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("Search returned %d results, want 3", len(results))
		}
		order := []string{results[0].Verb, results[1].Verb, results[2].Verb}
		if i == 0 {
			first = order
			// Identical similarity (1.0) breaks by verb ascending; the
			// orthogonal entry (0.0) sorts last.
			if first[0] != "/aaa" || first[1] != "/zzz" || first[2] != "/mmm" {
				t.Fatalf("tie order=%v, want [/aaa /zzz /mmm]", first)
			}
			continue
		}
		for j := range first {
			if order[j] != first[j] {
				t.Fatalf("run %d order=%v, want stable %v", i, order, first)
			}
		}
	}
}

// TestMergeResults_BoostCopiesInput pins that learned boosting caps at 1.0
// and never mutates the caller's slice (store-owned backing arrays would
// accumulate the boost on repeat searches).
func TestMergeResults_BoostCopiesInput(t *testing.T) {
	sc := &SemanticClassifier{}
	learned := []SemanticMatch{{Verb: "/fix", Similarity: 0.9, Source: "learned"}}
	embedded := []SemanticMatch{{Verb: "/explain", Similarity: 0.5, Source: "embedded"}}
	merged := sc.mergeResults(embedded, learned, SemanticConfig{LearnedBoost: 0.2, TopK: 5})
	if len(merged) != 2 {
		t.Fatalf("merged %d, want 2", len(merged))
	}
	if merged[0].Verb != "/fix" || merged[0].Similarity != 1.0 {
		t.Errorf("boosted=%+v, want /fix at capped 1.0", merged[0])
	}
	if learned[0].Similarity != 0.9 {
		t.Errorf("input slice mutated to %v, want untouched 0.9", learned[0].Similarity)
	}
	// Second merge over the same input must give the same result (no accumulation).
	again := sc.mergeResults(embedded, learned, SemanticConfig{LearnedBoost: 0.2, TopK: 5})
	if again[0].Similarity != merged[0].Similarity {
		t.Errorf("repeat merge=%v, want stable %v", again[0].Similarity, merged[0].Similarity)
	}
}

// TestClassify_TruncatesRuneSafely pins that oversized multibyte input is cut
// on rune boundaries before embedding: no U+FFFD poison reaches the model.
func TestClassify_TruncatesRuneSafely(t *testing.T) {
	engine := &mockEmbedEngine{}
	sc := NewSemanticClassifier(&mockKernel{}, nil, nil, engine)
	big := strings.Repeat("héllo ", 10000) // 60000 runes
	_, _ = sc.ClassifyWithoutInjection(context.Background(), big)
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if strings.ContainsRune(engine.lastInput, '\uFFFD') {
		t.Error("embedded input contains U+FFFD replacement chars")
	}
	body := strings.TrimSuffix(engine.lastInput, "... [Input truncated]")
	if n := len([]rune(body)); n > 32768 {
		t.Errorf("embedded %d runes, want at most 32768", n)
	}
}

// TestAddLearnedPattern_Validation pins fail-fast input checks: no store or
// engine is touched for empty patterns or verbs.
func TestAddLearnedPattern_Validation(t *testing.T) {
	sc := &SemanticClassifier{}
	ctx := context.Background()
	if err := sc.AddLearnedPattern(ctx, "   ", "/fix", "", "", 0.9); err == nil {
		t.Error("empty pattern accepted, want error")
	}
	if err := sc.AddLearnedPattern(ctx, "fix it", "  ", "", "", 0.9); err == nil {
		t.Error("empty verb accepted, want error")
	}
}

// TestAddLearnedPattern_InMemoryEmbedsOnce pins the memory-fallback path:
// one embed call per learned pattern, and the pattern is retrievable after.
func TestAddLearnedPattern_InMemoryEmbedsOnce(t *testing.T) {
	engine := &countingEngine{dims: 3, vec: []float32{1, 0, 0}}
	store, err := NewLearnedCorpusStore(nil, 3, nil)
	if err != nil {
		t.Fatalf("NewLearnedCorpusStore: %v", err)
	}
	if store.HasBackend() {
		t.Fatal("nil-config store reports a backend, want in-memory")
	}
	sc := NewSemanticClassifier(&mockKernel{}, nil, store, engine)
	if err := sc.AddLearnedPattern(context.Background(), "repair the login flow", "/fix", "", "", 0.9); err != nil {
		t.Fatalf("AddLearnedPattern: %v", err)
	}
	if engine.numCalls() != 1 {
		t.Errorf("embed calls=%d, want exactly 1", engine.numCalls())
	}
	results, err := store.Search([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].Verb != "/fix" {
		t.Errorf("Search=%+v, want the learned /fix pattern", results)
	}
}

// TestClassifyInputWithMatches_SemanticOverride pins the neuro-symbolic
// bridge end to end: a rank-1 neural match at 0.92 must override a
// higher-priority regex candidate through the >= 85 Mangle rule — the rule
// that could never fire while matches were injected into the wrong engine.
// The input carries no synonym signal so only priorities + semantics score.
func TestClassifyInputWithMatches_SemanticOverride(t *testing.T) {
	if SharedTaxonomy == nil {
		t.Skip("SharedTaxonomy not initialized")
	}
	candidates := []VerbEntry{
		{Verb: "/test", Priority: 88},
		{Verb: "/document", Priority: 72},
	}
	matches := []SemanticMatch{
		{TextContent: "write something", Verb: "/document", Target: "", Similarity: 0.92, Rank: 1, Source: "embedded"},
	}
	verb, _, err := SharedTaxonomy.ClassifyInputWithMatches("xyzzy", candidates, matches)
	if err != nil {
		t.Fatalf("ClassifyInputWithMatches: %v", err)
	}
	if verb != "/document" {
		t.Errorf("winner=%s, want /document (semantic override at 92)", verb)
	}
	// Without the neural signal the higher base priority must win instead.
	verb, _, err = SharedTaxonomy.ClassifyInputWithMatches("xyzzy", candidates, nil)
	if err != nil {
		t.Fatalf("ClassifyInputWithMatches(nil): %v", err)
	}
	if verb != "/test" {
		t.Errorf("winner without matches=%s, want /test (base priority 88)", verb)
	}
}

// TestClassifyInputWithMatches_NilEqualsClassify pins that the legacy entry
// point behaves identically to the bridged one with no matches: both seed
// the same inert 50.0 fallback after Clear.
func TestClassifyInputWithMatches_NilEqualsClassify(t *testing.T) {
	if SharedTaxonomy == nil {
		t.Skip("SharedTaxonomy not initialized")
	}
	inputs := []string{"run the tests now", "fix the broken login", "xyzzy nothing matches this"}
	for _, input := range inputs {
		candidates := getRegexCandidates(input, GetVerbCorpus())
		want, _, err := SharedTaxonomy.ClassifyInput(input, candidates)
		if err != nil {
			t.Fatalf("ClassifyInput(%q): %v", input, err)
		}
		got, _, err := SharedTaxonomy.ClassifyInputWithMatches(input, candidates, nil)
		if err != nil {
			t.Fatalf("ClassifyInputWithMatches(%q): %v", input, err)
		}
		if got != want {
			t.Errorf("input %q: WithMatches(nil)=%s, ClassifyInput=%s, want identical", input, got, want)
		}
	}
}
