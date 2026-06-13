package shards

import (
	"errors"
	"strings"
	"testing"
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

	// Oversized output is truncated in the shard_output fact.
	big := strings.Repeat("x", 5000)
	bigFacts := sm.ResultToFacts("s1", "coder", "task", big, nil)
	var out string
	for _, f := range bigFacts {
		if f.Predicate == "shard_output" && len(f.Args) >= 2 {
			out, _ = f.Args[1].(string)
		}
	}
	if !strings.HasSuffix(out, "... (truncated)") {
		t.Errorf("oversized shard_output should be truncated, got len=%d", len(out))
	}
}

func TestExtractSummary(t *testing.T) {
	sm := &ShardManager{}
	if got := sm.extractSummary("coder", "short"); got != "[coder] short" {
		t.Errorf("extractSummary short=%q", got)
	}
	long := strings.Repeat("a", 300)
	got := sm.extractSummary("tester", long)
	if !strings.HasPrefix(got, "[tester] ") || len(got) != len("[tester] ")+200 {
		t.Errorf("extractSummary should cap the body at 200 chars, got len=%d", len(got))
	}
}
