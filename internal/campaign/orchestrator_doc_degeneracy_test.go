package campaign

import (
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

// degenerate asks what the orchestrator asks of a generated document: it
// measures the text, asserts the measurement into a real kernel holding the
// campaign section's thresholds, and queries generated_output_degenerate.
func degenerate(t *testing.T, section config.CampaignConfig, text string) bool {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	policy, err := section.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureParams(k, policy.Params()); err != nil {
		t.Fatal(err)
	}
	got, err := (&Orchestrator{kernel: k}).generationDegenerate("/task_doc", text)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// repetitionLoop is the live failure from campaign_e6f9b0eb: "N. End. N+1.
// Finish. ..." about 1500 times, a 19KB "document" the fallback counted done.
func repetitionLoop() string {
	cycle := []string{"End.", "Finish.", "Complete.", "Done.", "Stop."}
	var b strings.Builder
	b.WriteString("I'll locate compiler.go and review it. monologue: Looking for compiler.go. ")
	for i := 1; i <= 1500; i++ {
		b.WriteString(itoa(i))
		b.WriteString(". ")
		b.WriteString(cycle[i%len(cycle)])
		b.WriteString(" ")
	}
	return b.String()
}

func TestGeneratedOutputDegenerate_CatchesRepetitionLoop(t *testing.T) {
	if !degenerate(t, config.CampaignConfig{}, repetitionLoop()) {
		t.Fatalf("expected degenerate repetition loop to be flagged")
	}
}

// The thresholds are the campaign section's: a floor on length the loop does
// not reach judges nothing.
func TestGeneratedOutputDegenerate_ThresholdsAreTheConfigs(t *testing.T) {
	if degenerate(t, config.CampaignConfig{DegenerateMinTokens: 100000}, repetitionLoop()) {
		t.Fatalf("a document below degenerate_min_tokens was judged a loop")
	}
}

// TestGeneratedOutputDegenerate_AllCounters flags output that is nothing but numeric
// counters and punctuation (zero real words).
func TestGeneratedOutputDegenerate_AllCounters(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 400; i++ {
		b.WriteString(itoa(i))
		b.WriteString(". ")
	}
	if !degenerate(t, config.CampaignConfig{}, b.String()) {
		t.Fatalf("expected all-counter output to be flagged")
	}
}

// TestGeneratedOutputDegenerate_AllowsRealProse must NOT flag a genuine, varied
// technical document — guarding against false positives that would fail a good
// deliverable.
func TestGeneratedOutputDegenerate_AllowsRealProse(t *testing.T) {
	doc := `# Ranked Risk Report: internal/prompt/compiler.go

## R1 (High) — Unbounded atom expansion in CompilePrompt
The compiler concatenates selected atoms without a token ceiling. A pathological
selection set can exceed the model context window, truncating the system prompt
and silently dropping safety instructions. Mitigation: enforce the configured
context_window.max_tokens budget during assembly and log when atoms are elided.

## R2 (Medium) — Nil selector dereference on empty corpus
When the predicate corpus fails to load, selectAtoms returns a nil slice that a
downstream range treats as zero atoms, producing an empty prompt rather than an
error. Mitigation: return an explicit error when the corpus is unavailable.

## R3 (Low) — Non-deterministic atom ordering
Map iteration order leaks into the final prompt, making cache keys unstable and
defeating prompt caching. Mitigation: sort atoms by category then id before
assembly.`
	if degenerate(t, config.CampaignConfig{}, doc) {
		t.Fatalf("real prose document was wrongly flagged as degenerate")
	}
}

// TestGeneratedOutputDegenerate_AllowsShort never flags short outputs regardless of
// repetition (below the token floor).
func TestGeneratedOutputDegenerate_AllowsShort(t *testing.T) {
	if degenerate(t, config.CampaignConfig{}, "done done done done done") {
		t.Fatalf("short output should not be flagged")
	}
}

// itoa is a tiny local base-10 formatter so the test has no external deps and no
// reliance on a nonexistent strings helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
