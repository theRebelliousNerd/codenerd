package prompt

import (
	"testing"
)

func TestConfigAtom_Merge(t *testing.T) {
	base := ConfigAtom{
		Tools:    []string{"read"},
		Policies: []string{"base.mg"},
		Priority: 10,
	}

	override := ConfigAtom{
		Tools:    []string{"write"},
		Policies: []string{"override.mg"},
		Priority: 20,
	}

	merged := base.Merge(override)

	if len(merged.Tools) != 2 {
		t.Errorf("Merge() Tools count = %v, want 2", len(merged.Tools))
	}

	if len(merged.Policies) != 2 {
		t.Errorf("Merge() Policies count = %v, want 2", len(merged.Policies))
	}

	// Test Priority
	if merged.Priority != 20 {
		t.Errorf("Merge() Priority = %v, want 20", merged.Priority)
	}
}

func TestSimpleRegistry_GetAtom(t *testing.T) {
	reg := NewSimpleRegistry()

	coderAtom := ConfigAtom{
		Tools: []string{"write_file"},
	}
	reg.Register("/coder", coderAtom)

	got, ok := reg.GetAtom("/coder")
	if !ok {
		t.Errorf("GetAtom(/coder) returned false")
	}

	if len(got.Tools) != 1 {
		t.Errorf("GetAtom(/coder) tools count = %v, want 1", len(got.Tools))
	}
}
