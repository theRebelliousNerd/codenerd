package context

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// A fold keeps every count it inherits (TODO-CTX-05B): turn coverage, masked
// turns, original tokens, and the atoms each segment had already dropped, plus
// the ones the merge's own 64-atom cap drops. None resets to zero, or the
// merged block's marker would announce fewer lost atoms than were lost.
func TestMergeOldestSegments_KeepsEveryInheritedCount(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	atoms := func(seg, n int) []core.Fact {
		out := make([]core.Fact, n)
		for i := range out {
			out[i] = core.Fact{Predicate: "observed", Args: []any{fmt.Sprintf("seg%d_atom%d", seg, i)}}
		}
		return out
	}
	comp.rollingSummary.Segments = []HistorySegment{
		{StartTurn: 1, EndTurn: 5, Summary: "first", OriginalTokens: 1000, MaskedTurns: 2, DroppedAtoms: 3, KeyAtoms: atoms(0, 40)},
		{StartTurn: 6, EndTurn: 9, Summary: "second", OriginalTokens: 700, MaskedTurns: 1, DroppedAtoms: 4, KeyAtoms: atoms(1, 40)},
		{StartTurn: 10, EndTurn: 12, Summary: "third", OriginalTokens: 300},
	}

	comp.mergeOldestSegments(2)

	segs := comp.rollingSummary.Segments
	if len(segs) != 2 {
		t.Fatalf("got %d segments after folding two of three, want 2", len(segs))
	}
	merged := segs[0]
	if merged.StartTurn != 1 || merged.EndTurn != 9 {
		t.Errorf("turn coverage %d..%d, want 1..9", merged.StartTurn, merged.EndTurn)
	}
	if merged.MaskedTurns != 3 || merged.OriginalTokens != 1700 {
		t.Errorf("masked=%d original=%d, want 3 and 1700", merged.MaskedTurns, merged.OriginalTokens)
	}
	// 3 + 4 inherited, and 80 distinct atoms under a 64-atom cap drop 16 more.
	if len(merged.KeyAtoms) != 64 || merged.DroppedAtoms != 3+4+16 {
		t.Errorf("kept %d atoms with DroppedAtoms=%d, want 64 kept and %d dropped", len(merged.KeyAtoms), merged.DroppedAtoms, 3+4+16)
	}
	if total := comp.rollingSummary.TotalOriginalTokens; total != 2000 {
		t.Errorf("TotalOriginalTokens = %d after the fold, want 2000", total)
	}
}
