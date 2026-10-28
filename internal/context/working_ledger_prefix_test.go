package context

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codenerd/internal/config"

	"github.com/stretchr/testify/require"
)

// rowKind is how a generated ledger row stands with the policy.
type rowKind int

const (
	rowUnobserved rowKind = iota // no observation: not live, rank 0
	rowLive                      // live, not a hot file: rank 1
	rowHot                       // live observation of a hot file: rank 2
	rowStale                     // observation of a file that changed since
)

type genRow struct {
	kind  rowKind
	bytes int
	round int
}

// referenceEvict is the shed rule as the specification states it, computed
// the plain way: stale rows leave on any compaction; past the ceiling the
// aged candidates, ordered by (rank, round, arrival), leave while the bytes
// ahead of each do not yet cover remain - ceiling.
func referenceEvict(rows []genRow, now, keep, ceiling int) []string {
	total := 0
	for _, r := range rows {
		total += r.bytes
	}
	if total <= ceiling {
		return nil
	}
	var evict []string
	remain := 0
	for i, r := range rows {
		if r.kind == rowStale {
			evict = append(evict, fmt.Sprintf("call-%02d", i))
			continue
		}
		remain += r.bytes
	}
	if remain > ceiling {
		type cand struct{ rank, round, seq, bytes int }
		var cands []cand
		for i, r := range rows {
			if r.kind == rowStale || r.round > now-keep {
				continue
			}
			cands = append(cands, cand{rank: int(r.kind), round: r.round, seq: i, bytes: r.bytes})
		}
		sort.Slice(cands, func(i, j int) bool {
			a, b := cands[i], cands[j]
			if a.rank != b.rank {
				return a.rank < b.rank
			}
			if a.round != b.round {
				return a.round < b.round
			}
			return a.seq < b.seq
		})
		need, before := remain-ceiling, 0
		for _, c := range cands {
			if before < need {
				evict = append(evict, fmt.Sprintf("call-%02d", c.seq))
			}
			before += c.bytes
		}
	}
	sort.Strings(evict)
	return evict
}

// The ordered prefix scan (working_shed_run) decides exactly what the
// specification decides, on mixed ranks, rounds, arrival order, zero-byte
// rows, stale rows and every ceiling from far under to far over -- the
// reference the all-prior relation it replaced also met.
func TestWorkingLedger_PrefixScanMatchesTheOrderedPrefixReference(t *testing.T) {
	rng := rand.New(rand.NewSource(20261027))
	const maxRows = 24
	for trial := range 60 {
		n := 1 + rng.Intn(maxRows)
		rows := make([]genRow, n)
		round := 1
		total := 0
		for i := range rows {
			if rng.Intn(3) == 0 {
				round++
			}
			bytes := rng.Intn(3000)
			if rng.Intn(6) == 0 {
				bytes = 0
			}
			rows[i] = genRow{kind: rowKind(rng.Intn(4)), bytes: bytes, round: round}
			total += bytes
		}
		keep := 1 + rng.Intn(3) // the config floor is 1
		ceiling := rng.Intn(total + 2)
		now := round

		root := t.TempDir()
		spans := config.DefaultWorkingConfig()
		spans.LedgerCeilingBytes = max(ceiling, 1)
		spans.LedgerKeepRounds = keep
		ceiling = spans.LedgerCeilingBytes
		w, err := NewWorkingSet(root, fmt.Sprintf("prefix-%d", trial), spans)
		require.NoError(t, err)

		entries := make([]LedgerEntry, n)
		var hot []string
		for i, r := range rows {
			id := fmt.Sprintf("o%02d", i)
			file := fmt.Sprintf("f%02d.go", i)
			require.NoError(t, os.WriteFile(filepath.Join(root, file), []byte("package f // "+file), 0600))
			entries[i] = LedgerEntry{Call: fmt.Sprintf("call-%02d", i), ID: id, Bytes: r.bytes, Round: r.round}
			if r.kind == rowUnobserved {
				continue
			}
			revision := w.Revision(file)
			if r.kind == rowStale {
				revision = "an-older-revision"
			}
			if r.kind == rowHot {
				hot = append(hot, file)
			}
			require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: id, Entity: file, Revision: revision, Kind: "read_file/" + id, Step: int64(r.round), Body: "body of " + id}))
		}
		decision, err := w.Ledger(t.Context(), LedgerRound{Entries: entries, Round: now, Hot: hot})
		require.NoError(t, err)
		want := referenceEvict(rows, now, keep, ceiling)
		if len(want) == 0 {
			require.Empty(t, decision.Evict, "trial %d", trial)
		} else {
			require.Equal(t, want, decision.Evict, "trial %d: rows %+v keep %d ceiling %d", trial, rows, keep, ceiling)
		}
		require.NoError(t, w.Close())
	}
}

// The scan is linear in the ledger: 3N+1 prefix rows over a ledger of N, and
// none at all below the ceiling. The all-prior relation it replaced held
// m(m-1)/2 pairs for m candidates.
func TestWorkingLedger_PrefixScanIsLinearAndOnlyPastTheCeiling(t *testing.T) {
	w, _ := ledgerSet(t, 1000, 1)
	var entries []LedgerEntry
	for i := range 40 {
		entries = append(entries, LedgerEntry{Call: fmt.Sprintf("call-%02d", i), ID: fmt.Sprintf("u%02d", i), Bytes: 100, Round: 1 + i/4})
	}
	decision, err := w.Ledger(t.Context(), LedgerRound{Entries: entries, Round: 10})
	require.NoError(t, err)
	require.Len(t, decision.Evict, 30, "an excess of 3000 over 100-byte rows: the 30th row is the last whose prefix (2900) falls short of it")
	require.Len(t, w.engine.QueryFacts("working_shed_run"), 3*len(entries)+1)

	under, _ := ledgerSet(t, 1<<20, 1)
	decision, err = under.Ledger(t.Context(), LedgerRound{Entries: entries, Round: 10})
	require.NoError(t, err)
	require.Empty(t, decision.Evict)
	require.Empty(t, under.engine.QueryFacts("working_shed_run"), "below the ceiling no candidate exists and no scan is derived")
}

// The exact threshold: a candidate whose prefix equals the excess stays.
func TestWorkingLedger_PrefixEqualToTheExcessStays(t *testing.T) {
	w, _ := ledgerSet(t, 1000, 1)
	entries := []LedgerEntry{
		{Call: "a", ID: "ua", Bytes: 300, Round: 1},
		{Call: "b", ID: "ub", Bytes: 0, Round: 1},
		{Call: "c", ID: "uc", Bytes: 200, Round: 1},
		{Call: "e", ID: "ue", Bytes: 100, Round: 1},
		{Call: "d", ID: "ud", Bytes: 900, Round: 2},
	}
	// d is the kept round. Excess 500. Before a: 0, b: 300, c: 300, e: 500
	// -- e's prefix equals the excess, so e stays; the zero-byte b leaves
	// with its prefix short.
	decision, err := w.Ledger(t.Context(), LedgerRound{Entries: entries, Round: 2})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, decision.Evict)
}

// The host half of the scan's contract is enforced, not assumed.
func TestWorkingLedger_RefusesALedgerThatBreaksTheShedContract(t *testing.T) {
	w, _ := ledgerSet(t, 1000, 1)
	for name, entries := range map[string][]LedgerEntry{
		"duplicate call":  {{Call: "a", ID: "x", Bytes: 1, Round: 1}, {Call: "a", ID: "y", Bytes: 1, Round: 1}},
		"empty call":      {{Call: "", ID: "x", Bytes: 1, Round: 1}},
		"negative size":   {{Call: "a", ID: "x", Bytes: -1, Round: 1}},
		"round goes back": {{Call: "a", ID: "x", Bytes: 1, Round: 2}, {Call: "b", ID: "y", Bytes: 1, Round: 1}},
	} {
		_, err := w.Ledger(t.Context(), LedgerRound{Entries: entries, Round: 2})
		require.Error(t, err, name)
	}
}
