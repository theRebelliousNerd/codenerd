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
// ceiling -- whatever file each came from. Past the ceiling a compaction
// moves only enough of the oldest aged results to fit, not every round
// outside the kept window. Until 2026-09-23 the request carried a sliding
// window of rounds plus a section regenerated every round, and the section
// was never served from a provider cache. Until 2026-09-29 the compaction
// was that window again: five live reads over a 4096-byte ceiling dropped
// three, and the model had to recall them to keep working.
func TestWorkingLedger_CompactsOnlyPastTheCeiling(t *testing.T) {
	w, _ := ledgerSet(t, 4096, 2, "a_test.go", "b.go", "c.go", "d.go")
	var entries []LedgerEntry
	for i, name := range []string{"a_test.go", "b.go", "c.go", "d.go"} {
		entries = append(entries, observe(t, w, name, name, "read_file/"+name, i+1, 1000, 0, 0))
	}
	decision, err := w.Ledger(t.Context(), entries, 4, nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, decision.Evict, "4000 bytes is under a 4096-byte ceiling: nothing moves, whatever file it came from")

	entries = append(entries, observe(t, w, "e", "d.go", "outline/d", 5, 1000, 0, 0))
	decision, err = w.Ledger(t.Context(), entries, 5, nil, nil, nil)
	require.NoError(t, err)
	// 5000 bytes, ceiling 4096, keep 2: rounds 1-3 are aged, the excess is 904,
	// and the oldest 1000 covers it. The other two aged live results stay.
	require.Equal(t, []string{"call-a_test.go"}, decision.Evict, "past the ceiling, only the oldest aged result the excess needs moves out")
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
	decision, err := w.Ledger(t.Context(), entries, 4, nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, decision.Evict, "under the ceiling a covered read stays in the ledger")

	// The same body read again under another request: one observation.
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "b2", Entity: "b.go", Revision: w.Revision("b.go"), Kind: "read_file/b-2-40", Step: 5, Body: "body of b1"}))
	entries = append(entries, LedgerEntry{Call: "call-b2", ID: "b2", Bytes: 5000, Round: 4})
	decision, err = w.Ledger(t.Context(), entries, 4, nil, nil, nil)
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
	decision, err := w.Ledger(t.Context(), entries, 1, nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, decision.Restate, "a current observation is not restated")

	require.NoError(t, os.WriteFile(path, []byte("package a // v2"), 0600))
	entries = append(entries, observe(t, w, "after", "a.go", "read_file/y", 2, 100, 0, 0))
	decision, err = w.Ledger(t.Context(), entries, 2, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"before"}, decision.Restate, "the pre-edit read is restated; the post-edit read is current")

	restated := map[string]string{"before": w.Revision("a.go")}
	decision, err = w.Ledger(t.Context(), entries, 2, restated, nil, nil)
	require.NoError(t, err)
	require.Empty(t, decision.Restate, "restated at this revision: not again")

	require.NoError(t, os.WriteFile(path, []byte("package a // v3"), 0600))
	decision, err = w.Ledger(t.Context(), entries, 3, restated, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"after", "before"}, decision.Restate, "a later revision restates both")
}

// A stale observation a compaction moves out is not restated: it is gone from
// the request, and its handle's recall reports it stale.
func TestWorkingLedger_AnEvictedStaleObservationIsNotRestated(t *testing.T) {
	w, root := ledgerSet(t, 4096, 2, "a.go")
	entries := []LedgerEntry{observe(t, w, "old", "a.go", "read_file/x", 3, 5000, 0, 0)}
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // edited"), 0600))
	decision, err := w.Ledger(t.Context(), entries, 3, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-old"}, decision.Evict, "past the ceiling a stale result moves out even within the kept rounds")
	require.Empty(t, decision.Restate)

	handle, err := w.Handle(t.Context(), "old")
	require.NoError(t, err)
	page, err := w.Recall(t.Context(), handle, 0, 0)
	require.NoError(t, err)
	require.True(t, strings.Contains(page, `"stale":true`), "the recall says the evidence is stale: %s", page)
	_, err = w.Recall(t.Context(), "old", 0, 0)
	require.Error(t, err, "the storage id is not a recall handle")
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
		decision, err := w.Ledger(t.Context(), entries, 4, nil, nil, nil)
		require.NoError(t, err)
		// 8000 bytes, ceiling 4096, excess 3904. Each row is 2000 and live.
		// keep 1 ages rounds 1-3 and sheds the first two (4000 covers 3904);
		// keep 3 ages only round 1. The kept rounds stay even when what
		// remains is still over the ceiling: they are what the model just saw.
		want := 2
		if keep == 3 {
			want = 1
		}
		require.Len(t, decision.Evict, want, "ledger_keep_rounds=%d, evict %v", keep, decision.Evict)
	}
	require.Contains(t, workingSetPolicy, "working_ledger_ceiling(N) :- config_param(/working_ledger_ceiling, N).")
	require.Contains(t, workingSetPolicy, "working_ledger_keep_rounds(N) :- config_param(/working_ledger_keep_rounds, N).")
	require.NotContains(t, workingSetPolicy, "working_pinned", "the half-ceiling pin is not a second eviction rule")
	require.NotContains(t, workingSetPolicy, "working_held(")
	require.Contains(t, workingSetPolicy, "working_recall_held(ID) :- working_recalled(ID), working_live(ID).")
}

// An observation of an element is dated by the element's own bytes (R8-3):
// editing one function restates what was read of it and of the whole file,
// and leaves what was read of another function in the same file current.
// With whole-file revisions every edit staled every observation of every
// function in the file.
func TestWorkingLedger_AnEditToOneElementLeavesTheOthersCurrent(t *testing.T) {
	w, root := ledgerSet(t, 1<<20, 2)
	path := filepath.Join(root, "a.go")
	write := func(aBody string) {
		t.Helper()
		src := "package a\n\n// A does a.\nfunc A() int { " + aBody + " }\n\n// B does b.\nfunc B() int { return 2 }\n"
		require.NoError(t, os.WriteFile(path, []byte(src), 0600))
	}
	write("return 1")
	a, b := ElementEntity("a.go", "A"), ElementEntity("a.go", "B")
	require.NotContains(t, []string{"absent", "unavailable"}, w.Revision(a), "the element resolves")
	entries := []LedgerEntry{
		observe(t, w, "read-a", a, "get_element/a", 1, 100, 0, 0),
		observe(t, w, "read-b", b, "get_element/b", 2, 100, 0, 0),
		observe(t, w, "read-file", "a.go", "read_file/x", 3, 100, 0, 0),
	}
	decision, err := w.Ledger(t.Context(), entries, 3, nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, decision.Restate)

	write("return 3")
	decision, err = w.Ledger(t.Context(), entries, 3, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"read-a", "read-file"}, decision.Restate, "the edited element and the whole file changed; B did not")

	require.NoError(t, os.WriteFile(path, []byte("package a\n\n// A does a.\nfunc A() int { return 3 }\n"), 0600))
	require.Equal(t, "absent", w.Revision(b), "a deleted element is absent, so what was read of it is stale")
	require.Equal(t, "a.go", EntityFile(b))
	require.Equal(t, "a.go", EntityFile("a.go"))
}

// A live observation of a hot file -- the focus, or one the loop wrote -- is
// shed after a live observation of any other file, same age included. It is
// not exempt: once the colder result does not cover the excess, the oldest
// hot live result leaves too. The newest kept round stays either way.
func TestWorkingLedger_PinsCurrentObservationsOfHotFiles(t *testing.T) {
	w, _ := ledgerSet(t, 4096, 1, "edit.go", "other.go", "late.go")
	entries := []LedgerEntry{
		observe(t, w, "edit", "edit.go", "read_file/edit", 1, 1000, 0, 0),
		observe(t, w, "other", "other.go", "read_file/other", 1, 1000, 0, 0),
		observe(t, w, "late", "late.go", "read_file/late", 3, 2500, 0, 0),
	}
	decision, err := w.Ledger(t.Context(), entries, 3, nil, []string{"edit.go"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-other"}, decision.Evict, "the hot file's current read stays; the cold one of the same age goes")

	// The cold result is 1000 and the excess is 2404, so the oldest hot live
	// result leaves too. The round inside the keep window does not.
	entries[0].Bytes = 3000
	decision, err = w.Ledger(t.Context(), entries, 3, nil, []string{"edit.go"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-edit", "call-other"}, decision.Evict, "once the cold result does not cover the excess, the oldest hot live result leaves too")
}

// Dropping a superseded read can be what brings the ledger back under the
// ceiling. The live read that covers it stays, even though it is older than
// the keep window: it is not stale, and the excess is already gone.
func TestWorkingLedger_DropsSupersededBeforeAnyLiveResult(t *testing.T) {
	w, _ := ledgerSet(t, 4096, 1, "a.go")
	entries := []LedgerEntry{
		observe(t, w, "narrow", "a.go", "read_file/narrow", 1, 3000, 10, 20),
		observe(t, w, "wide", "a.go", "read_file/wide", 2, 1000, 1, 100),
		observe(t, w, "late", "a.go", "read_file/late", 3, 1000, 0, 0),
	}
	decision, err := w.Ledger(t.Context(), entries, 3, nil, []string{"a.go"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-narrow"}, decision.Evict, "the covered read leaves; the live read that covers it stays")
}

// A stale result leaves even when a live result of the same age would
// otherwise be enough, and a live result leaves only for the excess that
// remains. The old rule dropped every aged row.
func TestWorkingLedger_DropsStaleBeforeSheddingLive(t *testing.T) {
	w, root := ledgerSet(t, 5000, 1, "a.go", "b.go")
	entries := []LedgerEntry{
		observe(t, w, "stale", "a.go", "read_file/stale", 1, 2000, 0, 0),
		observe(t, w, "live2", "b.go", "read_file/2", 2, 2000, 0, 0),
		observe(t, w, "live3", "b.go", "read_file/3", 3, 2000, 0, 0),
		observe(t, w, "live4", "b.go", "read_file/4", 4, 2000, 0, 0),
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // edited"), 0600))
	decision, err := w.Ledger(t.Context(), entries, 4, nil, []string{"b.go"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-live2", "call-stale"}, decision.Evict, "stale leaves, then only the oldest live result the remaining excess needs")
}

// A result the model recalled is not moved out again while it stays live,
// even when it is the oldest aged result and the ledger is over the ceiling.
// The next oldest live result leaves instead.
func TestWorkingLedger_ARecalledLiveResultIsNotShed(t *testing.T) {
	w, _ := ledgerSet(t, 4000, 1, "a.go")
	entries := []LedgerEntry{
		observe(t, w, "a", "a.go", "read_file/a", 1, 3000, 0, 0),
		observe(t, w, "b", "a.go", "read_file/b", 2, 3000, 0, 0),
		observe(t, w, "c", "a.go", "read_file/c", 3, 3000, 0, 0),
		observe(t, w, "d", "a.go", "read_file/d", 4, 3000, 0, 0),
	}
	decision, err := w.Ledger(t.Context(), entries, 4, nil, []string{"a.go"}, []string{"a"})
	require.NoError(t, err)
	require.Equal(t, []string{"call-b", "call-c"}, decision.Evict, "the recalled live result stays; the next oldest live results cover the excess")
}

// A recall does not keep a result that is no longer current. Stale evidence
// does not stand as the file.
func TestWorkingLedger_ARecalledStaleResultStillDrops(t *testing.T) {
	w, root := ledgerSet(t, 4096, 2, "a.go")
	entries := []LedgerEntry{observe(t, w, "old", "a.go", "read_file/x", 3, 5000, 0, 0)}
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // edited"), 0600))
	decision, err := w.Ledger(t.Context(), entries, 3, nil, []string{"a.go"}, []string{"old"})
	require.NoError(t, err)
	require.Equal(t, []string{"call-old"}, decision.Evict)
	require.Empty(t, decision.Restate)
}

// A ledger row with no observation is not live. It leaves before an older
// live result of a hot file: liveness is the fact, not the age.
func TestWorkingLedger_AnUnobservedRowLeavesBeforeALiveOne(t *testing.T) {
	w, _ := ledgerSet(t, 4096, 1, "a.go")
	entries := []LedgerEntry{
		observe(t, w, "live", "a.go", "read_file/live", 1, 1000, 0, 0),
		{Call: "call-orphan", ID: "missing", Bytes: 2000, Round: 2},
		observe(t, w, "late", "a.go", "read_file/late", 3, 2500, 0, 0),
	}
	decision, err := w.Ledger(t.Context(), entries, 3, nil, []string{"a.go"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-orphan"}, decision.Evict, "the row the task did not observe leaves first, even though it is newer")
}

// Arrival order breaks a tie inside one round. Call-id order would shed
// call-a ahead of call-z; the measurement is the order the rows were handed
// to the ledger.
func TestWorkingLedger_ShedsInArrivalOrderWithinARound(t *testing.T) {
	w, _ := ledgerSet(t, 4096, 1, "a.go")
	entries := []LedgerEntry{
		observe(t, w, "z", "a.go", "read_file/z", 1, 1000, 0, 0),
		observe(t, w, "a", "a.go", "read_file/a", 1, 1000, 0, 0),
		observe(t, w, "late", "a.go", "read_file/late", 3, 2500, 0, 0),
	}
	decision, err := w.Ledger(t.Context(), entries, 3, nil, []string{"a.go"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"call-z"}, decision.Evict, "the earlier arrival leaves; the later one of the same round stays")
}

// measuredReadSizes are the 16 tool results compaction moved out of, or kept
// in, the ledger on session 20260929_052520 (nerd fix, cmd/tools/validate_prompt_atoms),
// in ledger order. The first 14 were evicted at round 16; the last two were
// the kept rounds. Sum 56012. None was stale and none was superseded: each
// was a current observation of a file or element that task had read.
var measuredReadSizes = []int{
	310, 7359, 4847, 3976, 556, 2513, 1239, 3124,
	1762, 1588, 2449, 12530, 494, 8771, 2947, 1547,
}

func measuredReads(t *testing.T, w *WorkingSet) []LedgerEntry {
	t.Helper()
	entries := make([]LedgerEntry, 0, len(measuredReadSizes))
	for i, n := range measuredReadSizes {
		id := fmt.Sprintf("r%02d", i+1)
		entries = append(entries, observe(t, w, id, "main.go", "get_element/"+id, i+1, n, 0, 0))
	}
	return entries
}

// The measured run compacted because the ceiling was 65536 and the rule then
// dropped every result older than two rounds. At the derived ceiling the same
// 16 reads fit, so the model never has to recall one to continue. At a ceiling
// the 16 exceed, only the oldest live results the excess needs leave.
func TestWorkingLedger_ReplayOfTheMeasuredReadOnlyStall(t *testing.T) {
	sum := 0
	for _, n := range measuredReadSizes {
		sum += n
	}
	require.Equal(t, 56012, sum)
	require.Len(t, measuredReadSizes, 16)

	t.Run("the derived ceiling carries every live read", func(t *testing.T) {
		spans := config.DefaultWorkingConfig()
		require.Equal(t, 400000, spans.LedgerCeilingBytes)
		require.Equal(t, config.LedgerCeilingBytesFromContext(config.DefaultContextWindowConfig()), spans.LedgerCeilingBytes)
		require.Greater(t, spans.LedgerCeilingBytes, sum)
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600))
		w, err := NewWorkingSet(root, "replay-default", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		decision, err := w.Ledger(t.Context(), measuredReads(t, w), 16, nil, []string{"main.go"}, nil)
		require.NoError(t, err)
		require.Empty(t, decision.Evict, "16 live reads of one target fit the derived ceiling; a recall is not required to continue")
	})

	t.Run("a tight ceiling sheds only the oldest live results", func(t *testing.T) {
		w, _ := ledgerSet(t, 50000, 2, "main.go")
		decision, err := w.Ledger(t.Context(), measuredReads(t, w), 16, nil, []string{"main.go"}, nil)
		require.NoError(t, err)
		// Excess is 6012. The oldest result is 310, the next is 7359.
		// 310 does not cover it; 310+7359 does. The other 12 aged live
		// results stay, where keep_rounds=2 used to drop all 14.
		require.Equal(t, []string{"call-r01", "call-r02"}, decision.Evict)
	})

	t.Run("recalling the oldest live result sheds the next one instead", func(t *testing.T) {
		w, _ := ledgerSet(t, 50000, 2, "main.go")
		decision, err := w.Ledger(t.Context(), measuredReads(t, w), 16, nil, []string{"main.go"}, []string{"r01"})
		require.NoError(t, err)
		require.Equal(t, []string{"call-r02"}, decision.Evict, "the recalled result stays live in the request")
	})
}
