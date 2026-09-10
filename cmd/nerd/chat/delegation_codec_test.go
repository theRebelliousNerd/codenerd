package chat

import (
	"fmt"
	"strings"
	"testing"
)

// testerTurn is a tester shard's whole turn: a plan, a long build/run log, and
// the failures at the end. The failures come last, which is what made the old
// truncateForTask(RawOutput, 500) window carry the preamble and drop them.
func testerTurn() (transcript, narration string) {
	narration = "Plan: I will run the widget package suite and then the store suite."
	var sb strings.Builder
	sb.WriteString(narration + "\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "=== RUN   TestWidgetCase%d\n--- PASS: TestWidgetCase%d (0.00s)\n", i, i)
	}
	sb.WriteString("--- FAIL: TestFlushPropagatesError (0.01s)\n")
	sb.WriteString("    store_test.go:88: expected error, got nil\n")
	sb.WriteString("FAIL\n")
	return sb.String(), narration
}

// TestPriorShardContext_ShouldHandTheNextShardTheFailuresNotTheLog is the chat
// blackboard producer: a tester's output reaching a coder. The old window was
// the first 500 bytes with newlines flattened, cut wherever byte 500 fell.
func TestPriorShardContext_ShouldHandTheNextShardTheFailuresNotTheLog(t *testing.T) {
	t.Parallel()

	transcript, narration := testerTurn()
	out := priorShardContext(&ShardResult{
		ShardType: "tester",
		Task:      "run_tests",
		RawOutput: transcript,
	})

	if strings.Contains(out, narration) {
		t.Errorf("the blackboard handed the next shard the tester's narration:\n%s", out)
	}
	if !strings.Contains(out, "TestFlushPropagatesError") {
		t.Errorf("the projection dropped the named failure, which is the only thing the coder needed:\n%s", out)
	}
	if !strings.Contains(out, "[reported]") {
		t.Errorf("the projection did not mark the verdict as the tester's claim rather than an observed run:\n%s", out)
	}
	if !strings.Contains(out, "subagent_expand handle=") {
		t.Errorf("the projection elided the log without naming the verb that redeems it:\n%s", out)
	}
	if len(out) >= len(transcript)/3 {
		t.Errorf("context is %d bytes against %d of log; a fixed window is what this replaces, not a smaller one",
			len(out), len(transcript))
	}
}

// TestPriorShardContext_ShouldProjectFromParsedFindingsRatherThanReparseThem.
// extractFindings has already produced file, line, severity and message here;
// re-reading them out of the rendered text would lose exactly the fields that
// survived and would disagree with the copy every other consumer gets.
func TestPriorShardContext_ShouldProjectFromParsedFindingsRatherThanReparseThem(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "Considering internal/widget/file%d.go; nothing to report.\n", i)
	}

	out := priorShardContext(&ShardResult{
		ShardType: "reviewer",
		Task:      "review internal/widget",
		RawOutput: sb.String(),
		Findings: []map[string]any{
			{"file": "internal/widget/store.go", "line": 88, "severity": "critical", "message": "Flush error discarded"},
			{"file": "internal/widget/widget.go", "line": 12, "severity": "low", "message": "missing doc comment"},
		},
	})

	if !strings.Contains(out, "critical internal/widget/store.go:88 Flush error discarded") {
		t.Errorf("the parsed finding did not survive projection with its file, line and severity:\n%s", out)
	}
	// Worst first: the fixer reads the top of the list.
	if strings.Index(out, "Flush error discarded") > strings.Index(out, "missing doc comment") {
		t.Errorf("findings are not ordered worst-first:\n%s", out)
	}
}

func TestPriorShardContext_WhenThereIsNoPriorResult_ShouldReturnNothing(t *testing.T) {
	t.Parallel()

	if got := priorShardContext(nil); got != "" {
		t.Errorf("priorShardContext(nil) = %q, want empty", got)
	}
}

// TestFormatShardTaskWithContext_ShouldKeepTheTaskGrammarTheExecutorParses. The
// projection is multi-line where the old window was one flattened line; the
// verb-and-target prefix in front of it is what the routing layer reads, and
// changing that would re-target the fixer.
func TestFormatShardTaskWithContext_ShouldKeepTheTaskGrammarTheExecutorParses(t *testing.T) {
	t.Parallel()

	transcript, _ := testerTurn()
	task := formatShardTaskWithContext("/fix", "internal/widget/store.go", "none", t.TempDir(),
		&ShardResult{ShardType: "tester", Task: "run_tests", RawOutput: transcript})

	if !strings.HasPrefix(task, "fix file:internal/widget/store.go test_errors:[") {
		t.Fatalf("the task grammar the executor parses has changed:\n%s", task)
	}
	if !strings.Contains(task, "TestFlushPropagatesError") {
		t.Errorf("the failing test did not reach the fixer:\n%s", task)
	}
}
