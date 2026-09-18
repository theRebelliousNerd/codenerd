package context

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// Every cut this package makes on its way to the model has to say so. The
// compressor's whole job is to hand the model less than there was; doing that
// without a marker is how a three-atom excerpt of a thirty-atom turn becomes,
// to the reader, the turn.

func TestTrimToTokens_MarksTheCut(t *testing.T) {
	c := newTestCompressor(10000)
	long := strings.Repeat("the assistant explained the change in detail. ", 200)

	got := c.trimToTokens(long, 300)

	if got == long {
		t.Fatalf("trimToTokens returned the input unchanged for a %d-char string over a 300-token budget", len(long))
	}
	if !types.IsClamped(got) {
		t.Fatalf("trimToTokens dropped %d chars with no marker:\n%s", len(long)-len(got), got)
	}
	// The marker names the count, not merely the fact.
	if !strings.Contains(got, fmt.Sprintf("of %d chars", len(long))) {
		t.Errorf("the marker does not name how much was dropped out of how much: %q", tail(got))
	}
	// Budgeted for, not bolted on: a notice that pushes the result back over
	// the ceiling defeats the caller that is trimming to reach the ceiling.
	if n := c.counter.CountString(got); n > 300 {
		t.Errorf("trimmed result is %d tokens, over the 300-token budget it was trimmed to", n)
	}
}

func TestCollectKeyAtoms_MarksDroppedAtoms(t *testing.T) {
	c := newTestCompressor(10000)

	// One turn with far more result atoms than either cap admits.
	turn := CompressedTurn{TurnNumber: 1}
	intent := core.Fact{Predicate: "user_intent", Args: []any{"i1", "/code", "/fix", "auth.go", "none"}}
	turn.IntentAtom = &intent
	for i := range 40 {
		turn.ResultAtoms = append(turn.ResultAtoms, core.Fact{
			Predicate: "diagnostic",
			Args:      []any{fmt.Sprintf("file%d.go", i), "boom"},
		})
	}

	atoms, dropped := c.collectKeyAtoms([]CompressedTurn{turn}, 64)

	if dropped == 0 {
		t.Fatalf("40 result atoms collapsed to %d and nothing was reported dropped", len(atoms))
	}
	if want := 40 - 5; dropped != want {
		t.Errorf("dropped = %d, want %d (the per-turn cap keeps 5 of 40)", dropped, want)
	}

	// And the count has to reach the rendered block, which is the only place a
	// marker does any work.
	c.rollingSummary.Segments = []HistorySegment{{
		ID: "seg_1", StartTurn: 1, EndTurn: 1,
		Summary: "# Compressed History (Turns 1-1)\n", KeyAtoms: atoms, DroppedAtoms: dropped,
	}}
	c.renderRollingSummaryText()
	if !types.IsClamped(c.rollingSummary.Text) {
		t.Errorf("the rendered history block shows %d atoms of %d with no marker:\n%s",
			len(atoms), len(atoms)+dropped, c.rollingSummary.Text)
	}
	if !strings.Contains(c.rollingSummary.Text, "key atoms") {
		t.Errorf("the marker does not name the kind that was dropped:\n%s", c.rollingSummary.Text)
	}
}

// The masked branch of the segment summary has named what it removes since the
// kernel started making the masking decision. The unmasked branch, three lines
// away, sliced to three atoms and said nothing — so the same block could
// announce "12 atoms masked" for one turn and silently show 3 of 30 for the
// next, and a reader had no way to know the second turn was also incomplete.
func TestUnmaskedTurnAtomCap_MarksTheCut(t *testing.T) {
	c := newTestCompressor(10000)
	intent := core.Fact{Predicate: "user_intent", Args: []any{"i1", "/code", "/fix", "auth.go", "none"}}
	turn := CompressedTurn{TurnNumber: 7, IntentAtom: &intent}
	for i := range 30 {
		turn.ResultAtoms = append(turn.ResultAtoms, core.Fact{
			Predicate: "diagnostic",
			Args:      []any{fmt.Sprintf("file%d.go", i), "boom"},
		})
	}

	summary := c.generateSimpleSummary([]CompressedTurn{turn})

	if !types.IsClamped(summary) {
		t.Fatalf("3 of 30 result atoms rendered with no marker:\n%s", summary)
	}
	if !strings.Contains(summary, "result atoms from turn 7") {
		t.Errorf("the marker does not say which turn lost atoms:\n%s", summary)
	}
	if !strings.Contains(summary, "of 30") {
		t.Errorf("the marker does not name the total:\n%s", summary)
	}

	// The masked path must agree with it: both branches of the same block use
	// the same vocabulary for the same event.
	masked, _ := c.generateObservationMaskedSummary([]CompressedTurn{turn}, map[string]bool{})
	if !types.IsClamped(masked) {
		t.Errorf("the masked-summary path's unmasked branch cut 27 atoms with no marker:\n%s", masked)
	}
}

// A turn under the cap is complete, and saying otherwise would teach the model
// to distrust a complete record.
func TestUnmaskedTurnAtomCap_SaysNothingWhenNothingWasCut(t *testing.T) {
	c := newTestCompressor(10000)
	intent := core.Fact{Predicate: "user_intent", Args: []any{"i1", "/code", "/fix", "auth.go", "none"}}
	turn := CompressedTurn{TurnNumber: 2, IntentAtom: &intent, ResultAtoms: []core.Fact{
		{Predicate: "diagnostic", Args: []any{"a.go", "boom"}},
		{Predicate: "diagnostic", Args: []any{"b.go", "boom"}},
	}}

	if got := c.generateSimpleSummary([]CompressedTurn{turn}); types.IsClamped(got) {
		t.Errorf("an uncut summary carries a truncation marker:\n%s", got)
	}
}

// Shedding key atoms to fit the history reserve is a drop like any other. It
// used to take the block from sixty-four atoms to none between two renders
// with nothing said.
func TestKeyAtomShed_MarksTheCut(t *testing.T) {
	c := newTestCompressor(10000)
	c.config.HistoryReserve = 40
	var atoms []core.Fact
	for i := range 20 {
		atoms = append(atoms, core.Fact{Predicate: "diagnostic", Args: []any{fmt.Sprintf("file%d.go", i), "boom"}})
	}
	c.rollingSummary.Segments = []HistorySegment{{
		ID: "seg_1", StartTurn: 0, EndTurn: 9,
		Summary:  strings.Repeat("segment summary text ", 40),
		KeyAtoms: atoms,
	}}

	c.rebuildRollingSummaryText()

	if len(c.rollingSummary.Segments[0].KeyAtoms) != 0 {
		t.Skip("the reserve did not force a shed on this estimator; the shed path is covered by the reserve loop test")
	}
	if c.rollingSummary.Segments[0].DroppedAtoms < 20 {
		t.Errorf("shed %d atoms but recorded DroppedAtoms=%d", len(atoms), c.rollingSummary.Segments[0].DroppedAtoms)
	}
	if !types.IsClamped(c.rollingSummary.Text) {
		t.Errorf("the block lost every key atom with no marker:\n%s", c.rollingSummary.Text)
	}
}

// A fact argument is arbitrary text asserted by whatever produced it. Cut to
// forty-seven characters with a bare "...", a path reads as a real path and an
// error message reads as a complete error.
func TestFactSerializer_MarksDroppedArgChars(t *testing.T) {
	fs := NewFactSerializer()
	long := strings.Repeat("internal/core/defaults/policy/", 8)
	got := fs.renderFact(core.Fact{Predicate: "modified", Args: []any{long}})

	if !types.IsClamped(got) {
		t.Fatalf("a %d-char fact argument was cut with no marker: %s", len(long), got)
	}
	// The total the marker names is the rendered argument's length (quotes
	// included), which is what a reader comparing against the fact would see.
	if !strings.Contains(got, "chars from fact arg") {
		t.Errorf("the marker does not name the count and kind it dropped: %s", got)
	}
	whole := core.Fact{Predicate: "modified", Args: []any{long}}.String()
	if len(got) >= len(whole) {
		t.Errorf("the cut did not shorten the fact (%d -> %d); the marker cost more than it saved",
			len(whole), len(got))
	}
}

// tail keeps a failure message readable without hiding the marker, which lives
// at the end of a clamped string.
func tail(s string) string {
	if len(s) <= 200 {
		return s
	}
	return "…" + s[len(s)-200:]
}
