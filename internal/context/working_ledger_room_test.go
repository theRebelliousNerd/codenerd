package context

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// The request's window binds the ledger as the ceiling does. Measured
// 2026-10-29: the ledger grew to 625 KB under an 800 KB ceiling and never
// compacted, while each request past the input window had its budget guard
// archive one more result from its middle -- a cache break every round.
func TestWorkingLedger_TheRequestsRoomIsALimitToo(t *testing.T) {
	t.Parallel()
	w, _ := ledgerSet(t, 1<<20, 1, "a.go", "b.go", "c.go", "d.go")
	var entries []LedgerEntry
	for i, name := range []string{"a.go", "b.go", "c.go", "d.go"} {
		entries = append(entries, observe(t, w, name, name, "read_file/"+name, i+1, 1000, 0, 0))
	}
	decision, err := w.Ledger(t.Context(), LedgerRound{Room: 4000, Entries: entries, Round: 4})
	require.NoError(t, err)
	require.Empty(t, decision.Evict, "4000 bytes fit a 4000-byte room: nothing moves")

	decision, err = w.Ledger(t.Context(), LedgerRound{Room: 3500, Entries: entries, Round: 4})
	require.NoError(t, err)
	require.Equal(t, []string{"call-a.go"}, decision.Evict, "a 3500-byte room under a 1 MB ceiling is the limit: the oldest aged result covers the 500-byte excess")
}

// A compaction leaves the ledger at its low-water target, so the rounds after
// it add results without compacting -- the request's prefix stays the same
// and the provider keeps serving it from cache -- until the ledger crosses
// the limit again. Shed only to the limit, every round after the first
// crossing compacted again.
func TestWorkingLedger_ACompactionLeavesHeadroomForTheNextRounds(t *testing.T) {
	t.Parallel()
	const room, bytes = 10000, 1000
	var files []string
	for round := 1; round <= 30; round++ {
		files = append(files, fmt.Sprintf("f%02d.go", round))
	}
	run := func(lowWater int) (compactions int) {
		w, _ := ledgerSetLowWater(t, 1<<20, 1, lowWater, files...)
		var entries []LedgerEntry
		evicted := map[string]bool{}
		for round := 1; round <= 30; round++ {
			name := fmt.Sprintf("f%02d.go", round)
			entries = append(entries, observe(t, w, name, name, "read_file/"+name, round, bytes, 0, 0))
			var carried []LedgerEntry
			for _, e := range entries {
				if !evicted[e.Call] {
					carried = append(carried, e)
				}
			}
			decision, err := w.Ledger(t.Context(), LedgerRound{Room: room, Entries: carried, Round: round})
			require.NoError(t, err)
			if len(decision.Evict) > 0 {
				compactions++
			}
			for _, call := range decision.Evict {
				evicted[call] = true
			}
		}
		return compactions
	}
	// 30 rounds of 1000 bytes against a 10000-byte room: the first crossing
	// is round 11. Shed to the limit, rounds 11-30 each compact (20). Shed to
	// 70% (7000 bytes), a compaction leaves room for three more rounds, so
	// one comes every fourth round (5).
	require.Equal(t, 20, run(100), "low water 100 is the old behavior: a compaction every round past the limit")
	require.Equal(t, 5, run(70), "low water 70: a compaction every fourth round")
}

func TestWorkingLedger_NegativeRoomIsRefused(t *testing.T) {
	t.Parallel()
	w, _ := ledgerSet(t, 4096, 1, "a.go")
	_, err := w.Ledger(t.Context(), LedgerRound{Room: -1, Round: 1})
	require.Error(t, err)
}
