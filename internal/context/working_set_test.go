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
	selected, err := w.Select(t.Context(), "b.go", nil, nil, 1800)
	require.NoError(t, err)
	require.NotContains(t, selected.Text, "fact-0")
	require.LessOrEqual(t, len(selected.Text), 1800)
	recalled, err := w.Select(t.Context(), "a.go", nil, nil, 1800)
	require.NoError(t, err)
	require.Contains(t, recalled.Text, "fact-0", "early fact must return after many intervening observations")
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package changed"), 0600))
	stale, err := w.Select(t.Context(), "a.go", nil, nil, 1800)
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

// A selected observation is shown whole when the budget allows, whatever its
// length. Select used to read at most a 16000-character page of each body and
// then treat a longer body as unshowable, so a 20 KB read the policy had
// selected was replaced by a pointer however much budget the request had.
func TestWorkingSetSelectShowsALongObservationWithinBudget(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	w, err := NewWorkingSet(nil, root, "task")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	body := strings.Repeat("payload line\n", 3000) + "tail-marker"
	require.Greater(t, len(body), 16000)
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "long", Entity: "a.go", Revision: w.Revision("a.go"), Kind: "read", Step: 1, Body: body}))

	shown, err := w.Select(t.Context(), "a.go", []string{"long"}, nil, 2*len(body))
	require.NoError(t, err)
	require.Contains(t, shown.Text, "tail-marker", "a body that fits the budget is shown whole")
	require.Equal(t, []string{"long"}, shown.Selected)

	pointed, err := w.Select(t.Context(), "a.go", []string{"long"}, nil, len(body)/2)
	require.NoError(t, err)
	require.NotContains(t, pointed.Text, "tail-marker")
	require.Contains(t, pointed.Text, "recover with recall_context", "a body outside the budget is pointed at, not dropped")
}

// Recall returns the rest of a body from the offset unless the caller pages,
// and reports where the next page would start. The default page was 2000
// characters, which handed a 14 KB read back seven calls at a time.
func TestWorkingSetRecallReturnsTheWholeBodyUnlessPaged(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	w, err := NewWorkingSet(nil, root, "task")
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

// After an edit, the observations made at the new revision are the working
// set; the ones from before it are stale. Every round's runtime facts must
// replace the previous round's: with them accumulating, the file's old
// revision stayed asserted, every post-edit observation derived working_stale
// against it, and from the first edit on nothing about the edited file was
// ever selected again.
func TestWorkingSetSelectFollowsTheFileAcrossAnEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	require.NoError(t, os.WriteFile(path, []byte("package a\n// v1\n"), 0600))
	w, err := NewWorkingSet(nil, root, "task")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	before := w.Revision("a.go")
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "read-before", Entity: "a.go", Revision: before, Kind: "read_file/x", Step: 1, Body: "body before"}))
	first, err := w.Select(t.Context(), "a.go", []string{"read-before"}, nil, 100000)
	require.NoError(t, err)
	require.Equal(t, []string{"read-before"}, first.Selected)

	require.NoError(t, os.WriteFile(path, []byte("package a\n// v2\n"), 0600))
	after := w.Revision("a.go")
	require.NotEqual(t, before, after)
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "edit", Entity: "a.go", Revision: after, Kind: "edit_lines/y", Step: 2, Body: "body edit"}))
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "read-after", Entity: "a.go", Revision: after, Kind: "read_file/x", Step: 3, Body: "body after"}))

	second, err := w.Select(t.Context(), "a.go", []string{"read-before", "edit", "read-after"}, nil, 100000)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"edit", "read-after"}, second.Selected, "the post-edit observations are the working set")
	require.Equal(t, []string{"read-before"}, second.Omitted, "the pre-edit read is stale")
	require.Contains(t, second.Text, "body after")
	require.NotContains(t, second.Text, "body before")
}

// Two requests that differ in their arguments but returned the same body are
// one observation; only the latest is shown. Supersession used to key on the
// request alone, so a read whose range snapped to the same projection five
// times filled the section with five copies of it.
func TestWorkingSetSelectCollapsesRepeatedBodies(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	w, err := NewWorkingSet(nil, root, "task")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	rev := w.Revision("a.go")
	body := "a.go: lines 10-40 of 90\nsame projection"
	for i, kind := range []string{"read_file/range-8-40", "read_file/range-10-40", "read_file/range-12-40"} {
		require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: fmt.Sprintf("r%d", i), Entity: "a.go", Revision: rev, Kind: kind, Step: int64(i + 1), Body: body}))
	}
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "other", Entity: "a.go", Revision: rev, Kind: "read_file/range-50-60", Step: 4, Body: "a.go: lines 50-60 of 90\ndifferent projection"}))

	sel, err := w.Select(t.Context(), "a.go", []string{"r0", "r1", "r2", "other"}, nil, 100000)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"r2", "other"}, sel.Selected, "the latest copy of a repeated body and the distinct body")
	require.Equal(t, 1, strings.Count(sel.Text, "same projection"))
}

// A later read of the same file at the same revision that covers an earlier
// read's span replaces it; a read of a disjoint span does not. Observed
// 2026-09-11: one region read five times with slightly different ranges was
// five observations, and the section ran to 146 KB a round.
func TestWorkingSetSelectCollapsesCoveredSpans(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	w, err := NewWorkingSet(nil, root, "task")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	rev := w.Revision("a.go")
	save := func(id string, step, start, end int64, body string) {
		t.Helper()
		require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: id, Entity: "a.go", Revision: rev, Kind: "read_file/" + id, Step: step, Start: start, End: end, Body: body}))
	}
	save("narrow", 1, 760, 830, "a.go: lines 760-830\nnarrow view")
	save("wide", 2, 700, 850, "a.go: lines 700-850\nwide view")
	save("apart", 3, 1, 50, "a.go: lines 1-50\nhead view")
	save("outline", 4, 0, 0, "a.go: outline\nsymbols")

	sel, err := w.Select(t.Context(), "a.go", []string{"narrow", "wide", "apart", "outline"}, nil, 100000)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"wide", "apart", "outline"}, sel.Selected, "the covering read replaces the narrow one; a disjoint read and a span-less record stay")
	require.NotContains(t, sel.Text, "narrow view")

	save("whole", 5, 1, wholeSpanEnd, "a.go: whole file\nall of it")
	sel, err = w.Select(t.Context(), "a.go", []string{"narrow", "wide", "apart", "outline", "whole"}, nil, 100000)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"whole", "outline"}, sel.Selected, "a whole-file read replaces every ranged read at the same revision")
}

// wholeSpanEnd mirrors the session recorder's "to the end of the file" span.
const wholeSpanEnd = 1_000_000_000

// An observation whose call/result pair the request already carries in the
// transcript is neither selected into the section nor reported omitted, so
// nothing is sent twice; the span of such rounds is the policy's.
func TestWorkingSetSelectSkipsObservationsShownInTheTranscript(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0600))
	w, err := NewWorkingSet(nil, root, "task")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	rounds, err := w.TranscriptRounds(t.Context())
	require.NoError(t, err)
	require.Equal(t, 3, rounds, "the policy's working_transcript_rounds")
	rev := w.Revision("a.go")
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "older", Entity: "a.go", Revision: rev, Kind: "read_file/1", Step: 1, Body: "older body"}))
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "current", Entity: "a.go", Revision: rev, Kind: "read_file/2", Step: 2, Body: "current body"}))

	sel, err := w.Select(t.Context(), "a.go", []string{"older", "current"}, []string{"current"}, 100000)
	require.NoError(t, err)
	require.Equal(t, []string{"older"}, sel.Selected)
	require.Empty(t, sel.Omitted, "a shown observation is not an omission")
	require.Contains(t, sel.Text, "older body")
	require.NotContains(t, sel.Text, "current body")
}
