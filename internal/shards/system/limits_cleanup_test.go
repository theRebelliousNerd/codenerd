// limits_cleanup_test.go pins the LIMITS CLEANUP behavior changes in package
// system: model-facing prompt content passes whole instead of being cut at
// hardcoded byte/rune bounds.
package system

import (
	"strings"
	"testing"

	"codenerd/internal/prompt"
)

// The repair model fixes the previous invalid output, so it must see all of
// it: an 800-byte cut hid the exact syntax it must not repeat.
func TestRepairPrompt_PreviousInvalidOutputReturnedWhole(t *testing.T) {
	shard := NewMangleRepairShard()
	lastResponse := strings.Repeat("broken syntax fragment ", 100) + "TAIL-MARKER-800-PLUS"
	if len(lastResponse) <= 800 {
		t.Fatalf("fixture too short to exercise the old 800-byte cut: %d", len(lastResponse))
	}
	got := shard.buildRepairPrompt("next_action(/start).", []string{"some error"}, nil, lastResponse, "")
	if !strings.Contains(got, lastResponse) {
		t.Error("repair prompt dropped part of the previous invalid output")
	}
	if !strings.Contains(got, "TAIL-MARKER-800-PLUS") {
		t.Error("repair prompt lost the tail past byte 800")
	}
}

// Predicate descriptions carry the argument gloss the repair model needs to
// use the predicate correctly; a 50-rune cut hid it.
func TestFormatSelectedPredicate_LongDescriptionReturnedWhole(t *testing.T) {
	desc := strings.Repeat("argument gloss ", 20) + "TAIL"
	p := prompt.SelectedPredicate{Name: "permitted", Arity: 1, Domain: "safety", Description: desc}
	got := formatSelectedPredicate(p)
	if !strings.Contains(got, desc) {
		t.Errorf("predicate line cut the description: %q", got)
	}
	if strings.HasSuffix(strings.TrimSpace(got), "...") {
		t.Errorf("predicate line still carries a truncation ellipsis: %q", got)
	}
}
