package context

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

// Exercise the real kernel, compression, optional atom-summary replacement,
// segment merge and the public renderer used for provider context injection.
// Turn atoms are history fixtures, not current kernel facts: masking history
// does not authorize deleting independently selected current logical state.
func TestObservationMasking_FinalContextDoesNotResurrectResults(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		name := "summary"
		if fallback {
			name = "atom_fallback"
		}
		t.Run(name, func(t *testing.T) {
			comp := newKernelBackedCompressor(t)
			t.Cleanup(func() { _ = comp.store.Close() })
			comp.config = NewConfigWithBudget(1000000)
			comp.config.RecentTurnWindow = 1
			comp.config.TargetCompressionRatio = 0
			comp.budget = NewTokenBudget(comp.config)
			comp.turnNumber = 60

			intent := fact("user_intent", "mask_probe", "/code", "/fix", "unit.go", "none")
			makeTurn := func(number int, results ...string) CompressedTurn {
				turn := CompressedTurn{
					TurnNumber: number, OriginalTokens: 400, IntentAtom: &intent,
					FocusAtoms:  []core.Fact{fact("focus_resolution", "focus_kept")},
					ActionAtoms: []core.Fact{fact("action_taken", "action_kept")},
				}
				for _, result := range results {
					turn.ResultAtoms = append(turn.ResultAtoms, fact("diagnostic", result))
				}
				return turn
			}
			turns := []CompressedTurn{
				makeTurn(1, "ancient_hidden", "shared_fresh"),
				makeTurn(51, "old_hidden"),               // age 9: /old
				makeTurn(52, "mid_kept", "shared_fresh"), // age 8: /mid
				makeTurn(59, "recent_kept"),              // retained recent window
			}
			if fallback {
				// Repeated reasoning makes the narrative exceed an atom-sized
				// budget while leaving the deduplicated sidecar small.
				for i := 2; i <= 20; i++ {
					turns = append(turns[:i-1], append([]CompressedTurn{makeTurn(i, "ancient_hidden")}, turns[i-1:]...)...)
				}
				allAtoms, _ := comp.collectKeyAtoms(turns[:len(turns)-1], 64)
				atomBudget := comp.counter.CountString(comp.serializer.SerializeFacts(allAtoms)) + 5
				original := comp.countOriginalTokens(turns[:len(turns)-1])
				comp.config.TargetCompressionRatio = float64(original) / float64(atomBudget)
			}
			comp.recentTurns = turns
			if err := comp.compress(t.Context()); err != nil {
				t.Fatal(err)
			}
			if len(comp.rollingSummary.Segments) != 1 {
				t.Fatalf("segments = %d, want 1", len(comp.rollingSummary.Segments))
			}
			seg := comp.rollingSummary.Segments[0]
			wantMasked := len(turns) - 2 // every compressed turn except /mid
			if seg.MaskedTurns != wantMasked || comp.rollingSummary.TotalMaskedTurns != wantMasked {
				t.Fatalf("kernel masking did not run: segment=%d total=%d want=%d", seg.MaskedTurns, comp.rollingSummary.TotalMaskedTurns, wantMasked)
			}
			if fallback == strings.HasPrefix(seg.Summary, "# Compressed History") {
				t.Fatalf("wrong summary path: fallback=%v, summary=%q", fallback, seg.Summary)
			}
			if seg.DroppedAtoms != 0 {
				t.Errorf("policy-masked observations counted as cap losses: %d", seg.DroppedAtoms)
			}
			if len(turns[0].ResultAtoms) != 2 {
				t.Error("compression mutated the original history atoms")
			}

			assertRendered := func(stage string) {
				t.Helper()
				rendered, err := comp.GetContextString(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(rendered, "COMPRESSED HISTORY") || !strings.Contains(rendered, "# Key Atoms") {
					t.Fatalf("%s did not reach the final history/sidecar renderer", stage)
				}
				for _, hidden := range []string{"ancient_hidden", "old_hidden"} {
					if strings.Contains(rendered, hidden) {
						t.Errorf("%s: masked result %q resurrected in provider context:\n%s", stage, hidden, rendered)
					}
				}
				for _, kept := range []string{"mask_probe", "focus_kept", "action_kept", "mid_kept", "shared_fresh", "recent_kept"} {
					if !strings.Contains(rendered, kept) {
						t.Errorf("%s lost reasoning or unmasked result %q", stage, kept)
					}
				}
			}
			assertRendered("compressed")

			// Compress another unmasked turn, then use the real merge path.
			// The first segment's accepted mask must survive sidecar merging.
			comp.turnNumber = 61
			comp.recentTurns = append(comp.recentTurns, makeTurn(61, "next_recent"))
			comp.config.TargetCompressionRatio = 0
			if err := comp.compress(t.Context()); err != nil {
				t.Fatal(err)
			}
			comp.mergeOldestSegments(2)
			comp.rebuildRollingSummaryText()
			if len(comp.rollingSummary.Segments) != 1 || comp.rollingSummary.Segments[0].MaskedTurns != wantMasked {
				t.Fatal("merge lost segment coverage or masking count")
			}
			assertRendered("merged")
		})
	}
}
