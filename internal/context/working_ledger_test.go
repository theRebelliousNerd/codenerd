package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"

	"github.com/stretchr/testify/require"
)

// ledgerSet builds a working set whose ledger compacts past ceiling bytes and
// keeps keep rounds whole, over a workspace holding the named files.
func ledgerSet(t *testing.T, ceiling, keep int, files ...string) (*WorkingSet, string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range files {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("package a // "+name), 0600))
	}
	spans := config.DefaultWorkingConfig()
	spans.LedgerCeilingBytes = ceiling
	spans.LedgerKeepRounds = keep
	w, err := NewWorkingSet(root, "ledger", spans)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	return w, root
}

// observe saves an observation of entity at its current revision and returns
// the ledger row that carries it.
func observe(t *testing.T, w *WorkingSet, id, entity, kind string, round, bytes int, start, end int64) LedgerEntry {
	t.Helper()
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: id, Entity: entity, Revision: w.Revision(entity), Kind: kind, Step: int64(round), Body: "body of " + id, Start: start, End: end}))
	return LedgerEntry{Call: "call-" + id, ID: id, Bytes: bytes, Round: round}
}

// The ledger carries every result whole until it outgrows the configured
// ceiling -- whatever file each came from -- and a compaction then moves out
// everything older than the kept rounds in one step. Until 2026-09-23 the
// request carried a sliding window of rounds plus a section regenerated every
// round, and the section was never served from a provider cache.
func TestWorkingLedger_CompactsOnlyPastTheCeiling(t *testing.T) {
	w, _ := ledgerSet(t, 4096, 2, "a_test.go", "b.go", "c.go", "d.go")
	var entries []LedgerEntry
	for i, name := range []string{"a_test.go", "b.go", "c.go", "d.go"} {
		entries = append(entries, observe(t, w, name, name, "read_file/"+name, i+1, 1000, 0, 0))
	}
	decision, err := w.Ledger(t.Context(), entries, 4, nil)
	require.NoError(t, err)
	require.Empty(t, decision.Evict, "4000 bytes is under a 4096-byte ceiling: nothing moves, whatever file it came from")

	entries = append(entries, observe(t, w, "e", "d.go", "outline/d", 5, 1000, 0, 0))
	decision, err = w.Ledger(t.Context(), entries, 5, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-a_test.go", "call-b.go", "call-c.go"}, decision.Evict, "past the ceiling, every round older than the two kept moves out")
}

// A compaction also moves out what the kept rounds no longer need: a read
// covered by a later read of the same span, and a repeated body. Outside a
// compaction they stay, so the request's prefix does not change.
func TestWorkingLedger_CompactionMovesCoveredAndRepeatedReadsOut(t *testing.T) {
	w, _ := ledgerSet(t, 4096, 3, "a.go", "b.go")
	entries := []LedgerEntry{
		observe(t, w, "narrow", "a.go", "read_file/narrow", 2, 100, 760, 830),
		observe(t, w, "wide", "a.go", "read_file/wide", 3, 100, 700, 850),
		observe(t, w, "apart", "a.go", "read_file/apart", 3, 100, 1, 50),
		observe(t, w, "b1", "b.go", "read_file/b-1-40", 4, 100, 0, 0),
	}
	decision, err := w.Ledger(t.Context(), entries, 4, nil)
	require.NoError(t, err)
	require.Empty(t, decision.Evict, "under the ceiling a covered read stays in the ledger")

	// The same body read again under another request: one observation.
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "b2", Entity: "b.go", Revision: w.Revision("b.go"), Kind: "read_file/b-2-40", Step: 5, Body: "body of b1"}))
	entries = append(entries, LedgerEntry{Call: "call-b2", ID: "b2", Bytes: 5000, Round: 4})
	decision, err = w.Ledger(t.Context(), entries, 4, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-b1", "call-narrow"}, decision.Evict, "within the kept rounds only the covered read and the earlier copy of a repeated body move out")
}

// An observation the ledger still carries whose file changed after it was
// made is restated -- once per revision -- rather than rewritten, which would
// break the cache from its round on, or left standing as current. The file's
// revision is replaced on every call: when revisions accumulated, every
// observation made after the first edit read as stale against the old one.
func TestWorkingLedger_RestatesAStaleObservationOncePerRevision(t *testing.T) {
	w, root := ledgerSet(t, 1<<20, 2, "a.go")
	path := filepath.Join(root, "a.go")
	entries := []LedgerEntry{observe(t, w, "before", "a.go", "read_file/x", 1, 100, 0, 0)}
	decision, err := w.Ledger(t.Context(), entries, 1, nil)
	require.NoError(t, err)
	require.Empty(t, decision.Restate, "a current observation is not restated")

	require.NoError(t, os.WriteFile(path, []byte("package a // v2"), 0600))
	entries = append(entries, observe(t, w, "after", "a.go", "read_file/y", 2, 100, 0, 0))
	decision, err = w.Ledger(t.Context(), entries, 2, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"before"}, decision.Restate, "the pre-edit read is restated; the post-edit read is current")

	restated := map[string]string{"before": w.Revision("a.go")}
	decision, err = w.Ledger(t.Context(), entries, 2, restated)
	require.NoError(t, err)
	require.Empty(t, decision.Restate, "restated at this revision: not again")

	require.NoError(t, os.WriteFile(path, []byte("package a // v3"), 0600))
	decision, err = w.Ledger(t.Context(), entries, 3, restated)
	require.NoError(t, err)
	require.Equal(t, []string{"after", "before"}, decision.Restate, "a later revision restates both")
}

// A stale observation a compaction moves out is not restated: it is gone from
// the request, and its handle's recall reports it stale.
func TestWorkingLedger_AnEvictedStaleObservationIsNotRestated(t *testing.T) {
	w, root := ledgerSet(t, 4096, 2, "a.go")
	entries := []LedgerEntry{observe(t, w, "old", "a.go", "read_file/x", 3, 5000, 0, 0)}
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // edited"), 0600))
	decision, err := w.Ledger(t.Context(), entries, 3, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-old"}, decision.Evict, "past the ceiling a stale result moves out even within the kept rounds")
	require.Empty(t, decision.Restate)

	page, err := w.Recall(t.Context(), "old", 0, 0)
	require.NoError(t, err)
	require.True(t, strings.Contains(page, `"stale":true`), "the recall says the evidence is stale: %s", page)
}

// The ceiling and the kept rounds are the working section's keys, read by the
// policy as config_param rows -- not Go constants.
func TestWorkingLedger_ReadsTheConfiguredKnobs(t *testing.T) {
	for _, keep := range []int{1, 3} {
		w, _ := ledgerSet(t, 4096, keep, "a.go")
		var entries []LedgerEntry
		for round := 1; round <= 4; round++ {
			entries = append(entries, observe(t, w, fmt.Sprint(round), "a.go", fmt.Sprintf("read_file/%d", round), round, 2000, 0, 0))
		}
		decision, err := w.Ledger(t.Context(), entries, 4, nil)
		require.NoError(t, err)
		require.Len(t, decision.Evict, 4-keep, "ledger_keep_rounds=%d", keep)
	}
	require.Contains(t, workingSetPolicy, "working_ledger_ceiling(N) :- config_param(/working_ledger_ceiling, N).")
	require.Contains(t, workingSetPolicy, "working_ledger_keep_rounds(N) :- config_param(/working_ledger_keep_rounds, N).")
}
