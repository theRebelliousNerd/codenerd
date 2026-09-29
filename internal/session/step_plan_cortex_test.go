package session_test

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// TestStepPlan_MatchesOnShardedKernel is the production shape of the step
// plan's retry and completeness rules. The pass measurements are homed on
// the world shard beside turn_brief_site; a split join would derive the
// verdict on a single kernel and not on the one that ships. External test
// package: shards transitively imports session.
func TestStepPlan_MatchesOnShardedKernel(t *testing.T) {
	cortex := bootRetentionCortex(t)
	single, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	both := retentionPair{t: t, cortex: cortex, single: single}

	const (
		owed       = "/step_retry_owed"
		spent      = "/step_retry_spent"
		evidence   = "/step_evidence"
		open       = "/step_open"
		covered    = "/step_covered"
		done       = "/step_done"
		unmeasured = "/step_unmeasured"
	)

	// A pass that wrote nothing, not yet retried, owes one commit retry.
	both.assert(stepExec(owed, 1, "a.txt", 0, 2))
	// The retry was entered and the second pass still wrote nothing.
	both.assert(
		stepExec(spent, 1, "a.txt", 0, 4),
		core.Fact{Predicate: "step_retried", Args: []any{types.MangleAtom(spent), int64(1)}},
	)
	// Evidence after that retry covers the step. The other file was written.
	both.assert(
		stepExec(evidence, 1, "a.txt", 0, 3),
		core.Fact{Predicate: "step_retried", Args: []any{types.MangleAtom(evidence), int64(1)}},
		core.Fact{Predicate: "step_no_change_evidence", Args: []any{types.MangleAtom(evidence), int64(1), types.MangleString("a.txt:4 has no such call")}},
		stepExec(evidence, 2, "b.txt", 1, 1),
	)
	// Retried, still nothing, a different file was written: unresolved.
	both.assert(
		stepExec(open, 1, "a.txt", 0, 2),
		core.Fact{Predicate: "step_retried", Args: []any{types.MangleAtom(open), int64(1)}},
		stepExec(open, 2, "b.txt", 1, 1),
	)
	// The same file, written by step 1 and again by step 3. Step 2 wrote
	// nothing; the earliest writer covers it. The retry is still owed,
	// because coverage is the verdict's question, not the retry's.
	both.assert(
		stepExec(covered, 1, "a.txt", 1, 1),
		stepExec(covered, 2, "a.txt", 0, 2),
		stepExec(covered, 3, "a.txt", 4, 1),
	)
	both.assert(
		stepExec(done, 1, "a.txt", 1, 1),
		stepExec(done, 2, "b.txt", 2, 3),
	)

	both.parity(
		"step_execution", "step_retried", "step_no_change_evidence",
		"step_cover_candidate", "step_file_covered",
		"step_has_retried", "step_has_no_change", "step_has_cover",
		"step_unresolved", "step_next_action",
		"turn_has_step", "turn_has_unresolved_step", "turn_steps_verdict",
	)

	both.has("step_next_action", owed, "1\t/retry_commit")
	both.lacks("step_next_action", spent, "1\t/retry_commit")
	both.lacks("step_next_action", evidence, "1\t/retry_commit")
	both.lacks("step_next_action", open, "1\t/retry_commit")
	both.has("step_next_action", covered, "2\t/retry_commit")
	both.lacks("step_next_action", done, "1\t/retry_commit")
	both.lacks("step_next_action", done, "2\t/retry_commit")

	both.has("step_file_covered", covered, "2\t1")
	both.lacks("step_file_covered", covered, "2\t3")
	both.lacks("step_file_covered", covered, "1\t3")
	both.lacks("step_unresolved", covered, "2\ta.txt")

	both.has("step_unresolved", open, "1\ta.txt")
	both.lacks("step_unresolved", open, "2\tb.txt")
	both.lacks("step_unresolved", evidence, "1\ta.txt")
	both.lacks("step_unresolved", spent, "")
	both.has("step_unresolved", spent, "1\ta.txt")

	both.has("turn_steps_verdict", owed, "/incomplete")
	both.has("turn_steps_verdict", spent, "/incomplete")
	both.has("turn_steps_verdict", evidence, "/complete")
	both.has("turn_steps_verdict", open, "/incomplete")
	both.has("turn_steps_verdict", covered, "/complete")
	both.has("turn_steps_verdict", done, "/complete")
	both.lacks("turn_steps_verdict", unmeasured, "/complete")
	both.lacks("turn_steps_verdict", unmeasured, "/incomplete")
}

func stepExec(turn string, step int64, file string, writes, calls int64) core.Fact {
	return core.Fact{Predicate: "step_execution", Args: []any{
		types.MangleAtom(turn), step, types.MangleString(file), writes, calls,
	}}
}
