package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkingSetEvictionRecallAndRevision(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package b"), 0600))
	w, err := NewWorkingSet(nil, root, "task")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	for i := 0; i < 120; i++ {
		entity := "b.go"
		if i == 0 {
			entity = "a.go"
		}
		require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: fmt.Sprint(i), Entity: entity, Revision: w.Revision(entity), Kind: fmt.Sprint(i), Step: int64(i), Body: strings.Repeat("payload ", 80) + fmt.Sprintf(" fact-%d", i)}))
	}
	selected, err := w.Select(t.Context(), "b.go", nil, 1800)
	require.NoError(t, err)
	require.NotContains(t, selected.Text, "fact-0")
	require.LessOrEqual(t, len(selected.Text), 1800)
	recalled, err := w.Select(t.Context(), "a.go", nil, 1800)
	require.NoError(t, err)
	require.Contains(t, recalled.Text, "fact-0", "early fact must return after many intervening observations")
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package changed"), 0600))
	stale, err := w.Select(t.Context(), "a.go", nil, 1800)
	require.NoError(t, err)
	require.NotContains(t, stale.Text, "fact-0", "changed source invalidates old evidence")
	page, err := w.Recall(t.Context(), "0", 0, 1000)
	require.NoError(t, err)
	require.Contains(t, page, "fact-0", "eviction is not deletion")
	other, err := NewWorkingSet(nil, root, "sibling")
	require.NoError(t, err)
	defer other.Close()
	_, err = other.Recall(t.Context(), "0", 0, 1000)
	require.Error(t, err, "sibling scopes cannot recover each other's records")
}

// The working policy decides continuation from the loop's whole-turn report,
// not from a repeated-trace flag and a failure count alone. Observed
// 2026-09-11: with only those two signals, an open loop on a change task read
// 300 files in 25 minutes before a one-line edit the brief had named by file
// and line, then read on for the rest of a 30-minute ceiling without running
// the test the task named.
func TestWorkingSetContinuePolicy(t *testing.T) {
	w, err := NewWorkingSet(nil, t.TempDir(), "policy")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	ctx := t.Context()

	cases := []struct {
		name string
		p    WorkingProgress
		want WorkingDecision
	}{
		{"a fresh read task continues", WorkingProgress{Rounds: 1, SinceWrite: 1, SinceVerify: 1}, WorkingDecision{Continue: true}},
		{"a repeated cycle stops", WorkingProgress{Cycle: true, Rounds: 2, SinceWrite: 2, SinceVerify: 2}, WorkingDecision{Stop: "repeated_cycle"}},
		{"three failed rounds stop", WorkingProgress{FailedRounds: 3, Rounds: 3, SinceWrite: 3, SinceVerify: 3}, WorkingDecision{Stop: "tool_failures"}},
		{"a read task is nudged to conclude at the nudge span", WorkingProgress{Rounds: 8, SinceWrite: 8, SinceVerify: 8}, WorkingDecision{Continue: true, Nudge: "conclude"}},
		{"a change task is nudged to implement at the nudge span", WorkingProgress{WriteIntent: true, Rounds: 8, SinceWrite: 8, SinceVerify: 8}, WorkingDecision{Continue: true, Nudge: "implement"}},
		{"a change task that only read for the stall span stops", WorkingProgress{WriteIntent: true, Rounds: 24, SinceWrite: 24, SinceVerify: 24}, WorkingDecision{Stop: "read_only_stall"}},
		{"a change task is nudged to verify three rounds after a write", WorkingProgress{WriteIntent: true, Rounds: 5, Writes: 1, SinceWrite: 3, SinceVerify: 5}, WorkingDecision{Continue: true, Nudge: "verify"}},
		{"a change task that wrote and drifted finalizes", WorkingProgress{WriteIntent: true, Rounds: 10, Writes: 1, SinceWrite: 8, SinceVerify: 10}, WorkingDecision{Continue: true, Finalize: "verify_after_write", Nudge: "verify"}},
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
