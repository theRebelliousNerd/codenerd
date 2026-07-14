package campaign

import (
	"strings"
	"testing"
)

// TestIsDegenerateGeneration_CatchesRepetitionLoop reproduces the live failure
// from campaign_e6f9b0eb: Grok emitted "N. End. N+1. Finish. ..." ~1500 times as
// a 19KB "document" that the fallback silently counted as success. The guard must
// flag it.
func TestIsDegenerateGeneration_CatchesRepetitionLoop(t *testing.T) {
	cycle := []string{"End.", "Finish.", "Complete.", "Done.", "Stop."}
	var b strings.Builder
	b.WriteString("I'll locate compiler.go and review it. monologue: Looking for compiler.go. ")
	for i := 1; i <= 1500; i++ {
		b.WriteString(itoa(i))
		b.WriteString(". ")
		b.WriteString(cycle[i%len(cycle)])
		b.WriteString(" ")
	}
	if !isDegenerateGeneration(b.String()) {
		t.Fatalf("expected degenerate repetition loop to be flagged")
	}
}

// TestIsDegenerateGeneration_AllCounters flags output that is nothing but numeric
// counters and punctuation (zero real words).
func TestIsDegenerateGeneration_AllCounters(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 400; i++ {
		b.WriteString(itoa(i))
		b.WriteString(". ")
	}
	if !isDegenerateGeneration(b.String()) {
		t.Fatalf("expected all-counter output to be flagged")
	}
}

// TestIsDegenerateGeneration_AllowsRealProse must NOT flag a genuine, varied
// technical document — guarding against false positives that would replace good
// deliverables with placeholders.
func TestIsDegenerateGeneration_AllowsRealProse(t *testing.T) {
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
	if isDegenerateGeneration(doc) {
		t.Fatalf("real prose document was wrongly flagged as degenerate")
	}
}

// TestIsDegenerateGeneration_AllowsShort never flags short outputs regardless of
// repetition (below the token floor).
func TestIsDegenerateGeneration_AllowsShort(t *testing.T) {
	if isDegenerateGeneration("done done done done done") {
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
