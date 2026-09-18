package shards

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func TestResultToFacts(t *testing.T) {
	sm := &ShardManager{}

	// Error path: emits execution + error facts, no success/output.
	errFacts := sm.ResultToFacts("s1", "coder", "fix bug", "", errors.New("boom"))
	em := map[string]bool{}
	for _, f := range errFacts {
		em[f.Predicate] = true
	}
	for _, p := range []string{"shard_executed", "last_shard_execution", "shard_error"} {
		if !em[p] {
			t.Errorf("error path missing %q fact", p)
		}
	}
	if em["shard_success"] {
		t.Error("error path should not emit shard_success")
	}

	// Success path: emits success/output/context facts.
	okFacts := sm.ResultToFacts("s1", "coder", "fix bug", "did the thing", nil)
	om := map[string]bool{}
	for _, f := range okFacts {
		om[f.Predicate] = true
	}
	for _, p := range []string{"shard_executed", "shard_success", "shard_output", "recent_shard_context"} {
		if !om[p] {
			t.Errorf("success path missing %q fact", p)
		}
	}

	// Oversized output is truncated in the shard_output fact, and says so with
	// the count. "... (truncated)" told an operator reading a fact dump what
	// had happened and told the model nothing it could use: no size, no way to
	// tell a 4 KB answer from the first 4 KB of a 400 KB one.
	big := "HEADMARK" + strings.Repeat("x", 5000) + "TAILMARK"
	bigFacts := sm.ResultToFacts("s1", "coder", "task", big, nil)
	var out string
	for _, f := range bigFacts {
		if f.Predicate == "shard_output" && len(f.Args) >= 2 {
			out, _ = f.Args[1].(string)
		}
	}
	if !types.IsClamped(out) {
		t.Errorf("oversized shard_output cut with no marker, got len=%d", len(out))
	}
	if !strings.Contains(out, "5016 chars") {
		t.Errorf("the marker does not name the output's true length: %q", out)
	}
	// head+tail: a shard states its plan first and its findings last, so a
	// head-only cut removes exactly the conclusion the next turn needs.
	for _, want := range []string{"HEADMARK", "TAILMARK"} {
		if !strings.Contains(out, want) {
			t.Errorf("shard_output lost %q", want)
		}
	}
}

// recent_shard_context/2 is read back into a later turn, so its 200-char cut
// is model-facing too and used to end mid-sentence in silence.
func TestExtractSummary_MarksTheCut(t *testing.T) {
	sm := &ShardManager{}
	long := strings.Repeat("a", 300)
	got := sm.extractSummary("tester", long)
	if !types.IsClamped(got) {
		t.Errorf("extractSummary cut 100 chars with no marker: %q", got)
	}
	if !strings.HasPrefix(got, "[tester] ") {
		t.Errorf("extractSummary lost its shard-type prefix: %q", got)
	}
}

func TestExtractSummary(t *testing.T) {
	sm := &ShardManager{}
	if got := sm.extractSummary("coder", "short"); got != "[coder] short" {
		t.Errorf("extractSummary short=%q", got)
	}
	long := strings.Repeat("a", 300)
	got := sm.extractSummary("tester", long)
	if !strings.HasPrefix(got, "[tester] ") {
		t.Errorf("extractSummary should keep its shard-type prefix, got %q", got)
	}
	if !strings.HasPrefix(got, "[tester] "+strings.Repeat("a", 200)) {
		t.Errorf("extractSummary should cap the body at 200 chars, got len=%d", len(got))
	}
}
