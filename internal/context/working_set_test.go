package context

import (
	"codenerd/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Eviction is not deletion: a result a compaction moved out of the ledger is
// recalled whole from the archive, and only by its own scope.
func TestWorkingSetEvictionRecallAndRevision(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package b"), 0600))
	w, err := NewWorkingSet(root, "task", config.DefaultWorkingConfig())
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	var entries []LedgerEntry
	for i := 0; i < 120; i++ {
		entity := "b.go"
		if i == 0 {
			entity = "a.go"
		}
		body := strings.Repeat("payload ", 80) + fmt.Sprintf(" fact-%d", i)
		require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: fmt.Sprint(i), Entity: entity, Revision: w.Revision(entity), Kind: fmt.Sprint(i), Step: int64(i), Body: body}))
		entries = append(entries, LedgerEntry{Call: fmt.Sprintf("call-%d", i), ID: fmt.Sprint(i), Bytes: len(body), Round: i + 1})
	}
	decision, err := w.Ledger(t.Context(), entries, 120, nil)
	require.NoError(t, err)
	require.Contains(t, decision.Evict, "call-0", "the first round's result leaves a ledger far over its ceiling")
	page, err := w.Recall(t.Context(), "0", 0, 1000)
	require.NoError(t, err)
	require.Contains(t, page, "fact-0", "eviction is not deletion")
	other, err := NewWorkingSet(root, "sibling", config.DefaultWorkingConfig())
	require.NoError(t, err)
	defer other.Close()
	_, err = other.Recall(t.Context(), "0", 0, 1000)
	require.Error(t, err, "sibling scopes cannot recover each other's records")
	// The model reads this error: it names the id and the way forward, not the
	// driver's "sql: no rows in result set".
	require.Contains(t, err.Error(), `no archived observation has id "0"`)
	require.Contains(t, err.Error(), "query=")
	require.NotContains(t, err.Error(), "sql: no rows")
}

// The working policy decides continuation from the loop's whole-turn report,
// not from a repeated-trace flag and a failure count alone. Observed
// 2026-09-11: with only those two signals, an open loop on a change task read
// 300 files in 25 minutes before a one-line edit the brief had named by file
// and line, then read on for the rest of a 30-minute ceiling without running
// the test the task named.
func TestWorkingSetContinuePolicy(t *testing.T) {
	w, err := NewWorkingSet(t.TempDir(), "policy", config.DefaultWorkingConfig())
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	ctx := t.Context()

	cases := []struct {
		name string
		p    WorkingProgress
		want WorkingDecision
	}{
		{"a fresh read task continues", WorkingProgress{Rounds: 1, SinceWrite: 1, SinceVerify: 1}, WorkingDecision{Continue: true}},
		// Campaign 7b853890, 2026-09-22: a /research task repeated after 21
		// reads, was stopped, failed and retried from nothing. A read task's
		// product is its conclusion; a repeat means the reading is done.
		{"a repeated cycle on a read task finalizes", WorkingProgress{Cycle: true, Rounds: 2, SinceWrite: 2, SinceVerify: 2}, WorkingDecision{Continue: true, Finalize: "repeat_after_reading"}},
		// A repeat of calls that all failed read nothing: there is no
		// conclusion to ask for, and the failures run on to working_stop.
		{"a repeated cycle of failed reads does not finalize", WorkingProgress{Cycle: true, FailedRounds: 2, Rounds: 2, SinceWrite: 2, SinceVerify: 2}, WorkingDecision{Continue: true}},
		{"a repeated cycle of failed reads stops at three", WorkingProgress{Cycle: true, FailedRounds: 3, Rounds: 3, SinceWrite: 3, SinceVerify: 3}, WorkingDecision{Stop: "tool_failures"}},
		{"a repeated cycle on a change task that has not written stops", WorkingProgress{WriteIntent: true, Cycle: true, Rounds: 5, SinceWrite: 5, SinceVerify: 5}, WorkingDecision{Stop: "repeated_cycle"}},
		// Campaign 7b853890, 2026-09-21: a document written, read back until the
		// repeat detector fired, failed, rolled back and rewritten -- three times.
		// After a write a repeat finalizes with the write kept; the gates judge it.
		{"a repeated cycle after a write finalizes and keeps the write", WorkingProgress{WriteIntent: true, Cycle: true, Rounds: 12, Writes: 1, SinceWrite: 4, SinceVerify: 4}, WorkingDecision{Continue: true, Finalize: "repeat_after_write", Nudge: "verify"}},
		{"three failed rounds stop", WorkingProgress{FailedRounds: 3, Rounds: 3, SinceWrite: 3, SinceVerify: 3}, WorkingDecision{Stop: "tool_failures"}},
		{"a read task is nudged to conclude at the nudge span", WorkingProgress{Rounds: 8, SinceWrite: 8, SinceVerify: 8}, WorkingDecision{Continue: true, Nudge: "conclude"}},
		{"a change task is nudged to implement at the nudge span", WorkingProgress{WriteIntent: true, Rounds: 8, SinceWrite: 8, SinceVerify: 8}, WorkingDecision{Continue: true, Nudge: "implement"}},
		{"a change task that ignored the implement nudge for a span is put in the commit regime", WorkingProgress{WriteIntent: true, Rounds: 16, SinceWrite: 16, SinceVerify: 16}, WorkingDecision{Continue: true, Nudge: "implement", Regime: "commit"}},
		{"a change task that only read for the stall span stops", WorkingProgress{WriteIntent: true, Rounds: 24, SinceWrite: 24, SinceVerify: 24}, WorkingDecision{Stop: "read_only_stall"}},
		{"a change task is nudged to verify three rounds after a write", WorkingProgress{WriteIntent: true, Rounds: 5, Writes: 1, SinceWrite: 3, SinceVerify: 5}, WorkingDecision{Continue: true, Nudge: "verify"}},
		{"a change task that wrote and drifted for a nudge span is put in the commit regime", WorkingProgress{WriteIntent: true, Rounds: 10, Writes: 1, SinceWrite: 8, SinceVerify: 10}, WorkingDecision{Continue: true, Nudge: "verify", Regime: "commit"}},
		{"a change task that wrote and drifted for the finalize span finalizes", WorkingProgress{WriteIntent: true, Rounds: 18, Writes: 1, SinceWrite: 16, SinceVerify: 18}, WorkingDecision{Continue: true, Finalize: "verify_after_write", Nudge: "verify", Regime: "commit"}},
		{"a write with reading open leaves it open", WorkingProgress{WriteIntent: true, Rounds: 11, Writes: 2, SinceWrite: 0, SinceVerify: 11}, WorkingDecision{Continue: true}},
		{"a write under the commit regime does not lift it", WorkingProgress{WriteIntent: true, Regime: "commit", Rounds: 11, Writes: 2, SinceWrite: 0, SinceVerify: 11}, WorkingDecision{Continue: true, Regime: "commit"}},
		{"a verification under the commit regime lifts it", WorkingProgress{WriteIntent: true, Regime: "commit", Rounds: 12, Writes: 2, SinceWrite: 1, SinceVerify: 0}, WorkingDecision{Continue: true}},
		{"a verified write keeps going", WorkingProgress{WriteIntent: true, Rounds: 12, Writes: 1, SinceWrite: 10, SinceVerify: 1}, WorkingDecision{Continue: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := w.Continue(ctx, tc.p)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// Recall returns the rest of a body from the offset unless the caller pages,
// and reports where the next page would start. The default page was 2000
// characters, which handed a 14 KB read back seven calls at a time.
func TestWorkingSetRecallReturnsTheWholeBodyUnlessPaged(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	w, err := NewWorkingSet(root, "task", config.DefaultWorkingConfig())
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	body := strings.Repeat("0123456789", 2500) + "tail-marker"
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "whole", Entity: "a.go", Revision: w.Revision("a.go"), Kind: "read", Step: 1, Body: body}))

	whole, err := w.Recall(t.Context(), "whole", 0, 0)
	require.NoError(t, err)
	require.Contains(t, whole, "tail-marker")
	require.Contains(t, whole, fmt.Sprintf(`"total_chars":%d,"next_offset":%d`, len(body), len(body)))

	page, err := w.Recall(t.Context(), "whole", 10, 100)
	require.NoError(t, err)
	require.NotContains(t, page, "tail-marker")
	require.Contains(t, page, `"offset":10,`)
	require.Contains(t, page, `"next_offset":110`)
}

// Search discovers handles, not bodies: every hit reports body_chars and the
// envelope says how to read the body. Observed 2026-09-17: Search serialized
// full WorkingRecords, so every hit carried "body":"" — including several
// multi-KB reads — and the model concluded its archive was empty and re-read
// the same files until its budget ran out.
func TestWorkingSetSearchReportsBodyCharsNotAnEmptyBody(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "executor_tools.go"), []byte("package session"), 0600))
	w, err := NewWorkingSet(root, "search", config.DefaultWorkingConfig())
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	body := strings.Repeat("x", 5000)
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "hit-1", Entity: "internal/session/executor_tools.go", Revision: w.Revision("internal/session/executor_tools.go"), Kind: "read_file", Step: 1, Body: body}))

	out, err := w.Search(t.Context(), "executor_tools", 0, 10)
	require.NoError(t, err)
	require.Contains(t, out, `"id":"hit-1"`)
	require.Contains(t, out, `"body_chars":5000`, "the hit must report the body's size")
	require.NotContains(t, out, `"body":`, "an explicit empty body reads as an empty observation")
	require.Contains(t, out, `read_with`, "the envelope must say how to read the body")

	page, err := w.Recall(t.Context(), "hit-1", 0, 0)
	require.NoError(t, err)
	require.Contains(t, page, `"total_chars":5000`, "the id must round-trip into a full-body read")
}
