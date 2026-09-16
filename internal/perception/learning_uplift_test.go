package perception

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordingCritic captures the critic prompt instead of calling a model.
type recordingCritic struct {
	baseMockLLMClient
	lastPrompt string
	response   string
}

func (c *recordingCritic) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	c.lastPrompt = userPrompt
	return c.response, nil
}

func testTaxonomyEngine(t *testing.T) *TaxonomyEngine {
	t.Helper()
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("NewTaxonomyEngine: %v", err)
	}
	t.Cleanup(func() { eng.StopWorker() })
	return eng
}

// TestPersistLearnedFact_RefusesUnparseable pins the fail-closed hot-load:
// a critic payload that is not exactly one well-formed learned_exemplar
// (trailing clauses, wrong arity) must be refused BEFORE reaching the
// engine — never hot-loaded as unvalidated source.
func TestPersistLearnedFact_RefusesUnparseable(t *testing.T) {
	eng := testTaxonomyEngine(t)
	eng.SetWorkspace(t.TempDir())

	payloads := []string{
		`learned_exemplar("x", /delete, "db", "", 0.99). rogue_clause.`,
		`learned_exemplar("x", /delete).`,
		`not_a_fact_at_all`,
		`learned_exemplar("x", /delete, "db", "", notanumber).`,
	}
	for _, p := range payloads {
		if err := eng.PersistLearnedFact(p); err == nil {
			t.Errorf("payload %q accepted, want refusal", p)
		}
	}
	// Nothing reached the engine or the file.
	facts, err := eng.engine.GetFacts("learned_exemplar")
	if err != nil {
		t.Fatalf("GetFacts: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("engine holds %d learned_exemplar facts after refusals, want 0", len(facts))
	}
}

// TestPersistLearnedFact_RoundTripsValid pins the happy path: a well-formed
// critic fact appends to learned_taxonomy.mg with integer confidence AND
// hot-loads into the engine's program. Hot-load efficacy is pinned
// behaviorally, not structurally: schema-fragment facts are invisible to
// GetFacts, so the test classifies with the learned sentence and asserts
// the learned-pattern +40 boost fired (72+40=112 beats the +30 medium
// boost's 102 — only the hot-loaded exemplar can produce 112).
func TestPersistLearnedFact_RoundTripsValid(t *testing.T) {
	eng := testTaxonomyEngine(t)
	root := t.TempDir()
	eng.SetWorkspace(root)

	fact := `learned_exemplar("xyzzy batter", /document, "", "", 0.95).`
	if err := eng.PersistLearnedFact(fact); err != nil {
		t.Fatalf("PersistLearnedFact: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".nerd", "mangle", "learned_taxonomy.mg"))
	if err != nil {
		t.Fatalf("read learned_taxonomy.mg: %v", err)
	}
	if !strings.Contains(string(raw), `learned_exemplar("xyzzy batter", /document, "", "", 95).`) {
		t.Fatalf("file lacks normalized fact, got:\n%s", raw)
	}

	candidates := []VerbEntry{{Verb: "/test", Priority: 88}, {Verb: "/document", Priority: 72}}
	matches := []SemanticMatch{{TextContent: "xyzzy batter", Verb: "/document", Similarity: 0.75, Rank: 1, Source: "embedded"}}
	verb, _, err := eng.ClassifyInputWithMatches("xyzzy batter", candidates, matches)
	if err != nil {
		t.Fatalf("ClassifyInputWithMatches: %v", err)
	}
	if verb != "/document" {
		t.Fatalf("winner=%s, want /document", verb)
	}
	scores, err := eng.engine.GetFacts("potential_score")
	if err != nil {
		t.Fatalf("GetFacts potential_score: %v", err)
	}
	best := -1.0
	for _, f := range scores {
		if len(f.Args) != 2 {
			continue
		}
		v, _ := f.Args[0].(string)
		var s float64
		switch n := f.Args[1].(type) {
		case int64:
			s = float64(n)
		case float64:
			s = n
		case int:
			s = float64(n)
		}
		if v == "/document" && s > best {
			best = s
		}
	}
	if best != 112 {
		t.Errorf("best /document score=%v, want 112 (learned +40 boost proves hot-load)", best)
	}
}

// TestLearnFromInteraction_BoundsTranscript pins that a pasted 50KB prompt
// cannot blow the critic call to 250KB: each trace side truncates.
func TestLearnFromInteraction_BoundsTranscript(t *testing.T) {
	eng := testTaxonomyEngine(t)
	critic := &recordingCritic{response: ""}
	eng.SetClient(critic)

	big := strings.Repeat("x", 10000)
	history := make([]ReasoningTrace, 5)
	for i := range history {
		history[i] = ReasoningTrace{UserPrompt: big, Response: big, Success: true}
	}
	if _, err := eng.LearnFromInteraction(context.Background(), history); err != nil {
		t.Fatalf("LearnFromInteraction: %v", err)
	}
	// 5 turns x 2 sides x 2000 chars + framing must stay far below 100KB.
	if len(critic.lastPrompt) > 30000 {
		t.Errorf("critic prompt is %d chars, want bounded (~22KB max)", len(critic.lastPrompt))
	}
	if !strings.Contains(critic.lastPrompt, "... [truncated]") {
		t.Error("critic prompt lacks truncation markers")
	}
}

// TestLearnFromInteraction_EmptyHistoryNeedsNoClient pins the short-circuit:
// no work means no error even with no critic configured.
func TestLearnFromInteraction_EmptyHistoryNeedsNoClient(t *testing.T) {
	eng := testTaxonomyEngine(t)
	eng.client = nil
	fact, err := eng.LearnFromInteraction(context.Background(), nil)
	if err != nil {
		t.Errorf("empty history with nil client: err=%v, want nil", err)
	}
	if fact != "" {
		t.Errorf("empty history: fact=%q, want empty", fact)
	}
}

// TestTruncateForCritic pins rune-safe head truncation with a cut marker.
func TestTruncateForCritic(t *testing.T) {
	if got := truncateForCritic("short", 2000); got != "short" {
		t.Errorf("short input changed to %q", got)
	}
	if got := truncateForCritic("anything", 0); got != "" {
		t.Errorf("zero limit gave %q, want empty", got)
	}
	multi := strings.Repeat("héllo ", 1000)
	got := truncateForCritic(multi, 2000)
	if strings.ContainsRune(got, '\uFFFD') {
		t.Error("truncation produced U+FFFD")
	}
	body := strings.TrimSuffix(got, "... [truncated]")
	if n := len([]rune(body)); n != 2000 {
		t.Errorf("truncated to %d runes, want 2000", n)
	}
}

// TestConsolidationWorker_NilEngine pins that a misconstructed worker cannot
// panic the background goroutine: process is a no-op without an engine.
func TestConsolidationWorker_NilEngine(t *testing.T) {
	cw := NewConsolidationWorker(nil)
	cw.Start()
	cw.Enqueue([]ReasoningTrace{{UserPrompt: "hi"}})
	cw.Stop() // must not panic or hang
	cw.Stop() // idempotent
}

// TestLearnedPatternContext_HasDeadline pins that pattern learning embeds
// under a real timeout instead of context.Background.
func TestLearnedPatternContext_HasDeadline(t *testing.T) {
	ctx, cancel := learnedPatternContext()
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Error("learnedPatternContext has no deadline, want 60s timeout")
	}
}
