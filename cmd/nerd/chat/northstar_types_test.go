package chat

import "testing"

func TestNewNorthstarWizard_StartsWithEmptyViewAndNoBase(t *testing.T) {
	w := NewNorthstarWizard()
	if w.Personas == nil || w.Capabilities == nil || w.Risks == nil || w.Requirements == nil || w.Constraints == nil {
		t.Fatal("expected slices to be initialized")
	}
	if w.edit.vision != nil || w.edit.replace {
		t.Fatal("a fresh wizard has no base vision")
	}
}
