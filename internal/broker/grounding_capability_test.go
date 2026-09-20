package broker

import (
	"testing"

	"codenerd/internal/types"
)

// The broker implements the grounding setters for every client it meters, and
// that is correct: calling a setter that forwards to a client which ignores it
// is indistinguishable from not calling it (see the note at the top of
// passthrough.go, and TestSettersForwardAndAreInertWithoutSupport).
//
// The consequence nobody intended is that `client.(types.GroundingController)`
// then succeeds for every metered client, and that assertion is how
// research.NewGroundingHelper decided whether grounding was available.
// Fourteen production call sites branch on that answer and take a different
// code path -- building documentation URL lists, enabling URL context, calling
// CompleteWithGrounding instead of Complete. Measured 2026-09-19: all 11
// `nerd fix` runs on disk ran on `provider = meta`, `model =
// muse-spark-1.3-contributor`, and every one logged
//
//	[INFO] [autopoiesis] Gemini grounding enabled for autopoiesis (Google Search active)
//	[DEBUG] Gemini grounding: Google Search enabled
//
// while no run ever logged a captured grounding source.
//
// optional.go already draws this line: a capability that selects a control flow
// stays conditional. The setters keep forwarding; the capability answers.
func TestSupportsGroundingAnswersForTheUnderlyingClient(t *testing.T) {
	t.Run("aClientThatControlsGrounding", func(t *testing.T) {
		c := wrapCore(t, newRichClient())
		if !c.SupportsGrounding() {
			t.Fatal("a metered Gemini reports no grounding; every grounded path would be lost")
		}
	})

	t.Run("aClientThatDoesNot", func(t *testing.T) {
		c := wrapCore(t, newFakeClient())
		if c.SupportsGrounding() {
			t.Fatal("a metered plain client reports grounding; fourteen call sites take the grounded path on that answer")
		}
	})

	// The wrapper must still satisfy the controller interface: the setters are
	// unconditional by design, and removing them would change behaviour this
	// package deliberately relies on.
	t.Run("theSetterInterfaceIsStillSatisfied", func(t *testing.T) {
		var c types.LLMClient = wrapCore(t, newFakeClient())
		if _, ok := c.(types.GroundingController); !ok {
			t.Fatal("core no longer satisfies GroundingController; the unconditional setters were removed, which is not the fix")
		}
		if _, ok := c.(types.GroundingCapable); !ok {
			t.Fatal("core does not report its grounding capability")
		}
	})
}
