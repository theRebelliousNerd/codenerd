package transparency

import (
	"testing"
)

func TestGlassBoxCategoryString(t *testing.T) {
	if CategoryKernel.String() != "kernel" {
		t.Errorf("Category.String()=%q, want kernel", CategoryKernel.String())
	}
}

func TestGlassBoxCategoryDisplayPrefix(t *testing.T) {
	if got := CategoryJIT.DisplayPrefix(); got != "[JIT]" {
		t.Errorf("DisplayPrefix()=%q, want [JIT]", got)
	}
}

func TestGlassBoxEventHasDetails(t *testing.T) {
	if (GlassBoxEvent{}).HasDetails() {
		t.Error("event with empty Details should report HasDetails=false")
	}
	if !(GlassBoxEvent{Details: "x"}).HasDetails() {
		t.Error("event with Details should report HasDetails=true")
	}
}

func TestEventBusEnableVerbose(t *testing.T) {
	b := NewGlassBoxEventBus()
	if b.IsEnabled() {
		t.Error("a fresh event bus should be disabled")
	}
	b.Enable()
	if !b.IsEnabled() {
		t.Error("Enable should activate the bus")
	}

	if b.IsVerbose() {
		t.Error("verbose should default to false")
	}
	b.SetVerbose(true)
	if !b.IsVerbose() {
		t.Error("SetVerbose(true) should enable verbose mode")
	}
	b.SetVerbose(false)
	if b.IsVerbose() {
		t.Error("SetVerbose(false) should disable verbose mode")
	}
}

func TestExplainerSetters(t *testing.T) {
	e := NewExplainer()
	// Setters mutate configuration without panicking; verified indirectly via
	// the constructor defaults differing from the values we set.
	e.SetMaxDepth(12)
	e.SetShowDetails(false)
	if e.maxDepth != 12 {
		t.Errorf("SetMaxDepth not applied: %d", e.maxDepth)
	}
	if e.showDetails {
		t.Error("SetShowDetails(false) not applied")
	}
}
