package context

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A repair payload a later one supersedes is retired at once -- under the
// ceiling, with no compaction -- while the latest payload stays whole, and a
// superseded tool result still waits for a compaction as before.
func TestWorkingLedger_RetiresASupersededRepairPayloadAtOnce(t *testing.T) {
	t.Parallel()
	w, _ := ledgerSet(t, 1<<20, 2, "a.go")
	payload := func(id string, round int) LedgerEntry {
		e := observe(t, w, id, "", "repair_payload", round, 30000, 0, 0)
		e.Payload = true
		return e
	}
	// Two reads of the same span of a.go: the first is superseded.
	read1 := observe(t, w, "read-1", "a.go", "read_file/x", 1, 500, 1, 10)
	first := payload("payload-1", 1)
	read2 := observe(t, w, "read-2", "a.go", "read_file/y", 2, 500, 1, 10)
	second := payload("payload-2", 2)

	decision, err := w.Ledger(t.Context(), LedgerRound{Room: unboundedRoom, Entries: []LedgerEntry{read1, first}, Round: 1})
	require.NoError(t, err)
	require.Empty(t, decision.Retire, "a payload nothing supersedes stays whole")

	decision, err = w.Ledger(t.Context(), LedgerRound{Room: unboundedRoom, Entries: []LedgerEntry{read1, first, read2, second}, Round: 2})
	require.NoError(t, err)
	require.Equal(t, []string{first.Call}, decision.Retire, "the payload a later one supersedes is retired, the latest is not")
	require.Empty(t, decision.Evict, "under the ceiling nothing is compacted: the superseded read waits for a compaction")
}

// Only a payload row is retired: the same observation carried as an ordinary
// row is not, whatever supersedes it.
func TestWorkingLedger_RetiresOnlyPayloadRows(t *testing.T) {
	t.Parallel()
	w, _ := ledgerSet(t, 1<<20, 2)
	older := observe(t, w, "p-1", "", "repair_payload", 1, 30000, 0, 0)
	newer := observe(t, w, "p-2", "", "repair_payload", 2, 30000, 0, 0)
	decision, err := w.Ledger(t.Context(), LedgerRound{Room: unboundedRoom, Entries: []LedgerEntry{older, newer}, Round: 2})
	require.NoError(t, err)
	require.Empty(t, decision.Retire, "rows not marked as payloads are not retired")
}
