package tools

import (
	"context"
	"testing"
)

// categorizedProbe builds a minimal valid tool. The existing probeTool helper
// in this package takes a ran flag rather than categories, and the category
// index is what Unregister has to sweep.
func categorizedProbe(name string, category ToolCategory, alts ...ToolCategory) *Tool {
	return &Tool{
		Name:          name,
		Effect:        EffectRead,
		Category:      category,
		AltCategories: alts,
		Execute: func(context.Context, map[string]any) (string, error) {
			return "", nil
		},
	}
}

func TestUnregisterRemovesFromBothIndexes(t *testing.T) {
	r := NewRegistry()

	tool := categorizedProbe("probe", CategoryGeneral)
	if err := r.Register(tool); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !r.Has("probe") {
		t.Fatal("tool not registered")
	}

	if !r.Unregister("probe") {
		t.Fatal("Unregister reported the tool absent")
	}
	if r.Has("probe") || r.Get("probe") != nil {
		t.Error("tool still present in the name index")
	}
	// A stale pointer in the category index would keep the tool discoverable
	// through GetByCategory while Get reported it gone -- a harder failure to
	// diagnose than never removing it at all.
	for _, got := range r.GetByCategory(CategoryGeneral) {
		if got.Name == "probe" {
			t.Error("tool still present in the category index")
		}
	}
}

func TestUnregisterClearsAltCategories(t *testing.T) {
	r := NewRegistry()

	tool := categorizedProbe("multi", CategoryGeneral, CategoryCode)
	if err := r.Register(tool); err != nil {
		t.Fatalf("Register: %v", err)
	}
	r.Unregister("multi")

	for _, category := range []ToolCategory{CategoryGeneral, CategoryCode} {
		for _, got := range r.GetByCategory(category) {
			if got.Name == "multi" {
				t.Errorf("tool still indexed under %s", category)
			}
		}
	}
}

func TestUnregisterLeavesSiblingsAlone(t *testing.T) {
	r := NewRegistry()

	for _, name := range []string{"a", "b", "c"} {
		if err := r.Register(categorizedProbe(name, CategoryGeneral)); err != nil {
			t.Fatalf("Register %s: %v", name, err)
		}
	}
	r.Unregister("b")

	if r.Count() != 2 {
		t.Fatalf("count = %d, want 2", r.Count())
	}
	for _, name := range []string{"a", "c"} {
		if !r.Has(name) {
			t.Errorf("%s was removed along with b", name)
		}
	}
	names := map[string]bool{}
	for _, got := range r.GetByCategory(CategoryGeneral) {
		names[got.Name] = true
	}
	if !names["a"] || !names["c"] || names["b"] {
		t.Errorf("category index after removing b = %v", names)
	}
}

func TestUnregisterMissingIsNotAnError(t *testing.T) {
	r := NewRegistry()
	if r.Unregister("never-registered") {
		t.Error("Unregister reported removing a tool that was never there")
	}
}

func TestRegisterAfterUnregisterSucceeds(t *testing.T) {
	r := NewRegistry()

	first := categorizedProbe("cycle", CategoryGeneral)
	if err := r.Register(first); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	// Register rejects duplicates, which is what made every test that registers
	// into the process-wide registry non-repeatable. Removal has to actually
	// free the name, or the isolation helper built on it does nothing.
	if err := r.Register(categorizedProbe("cycle", CategoryGeneral)); err == nil {
		t.Fatal("duplicate Register succeeded; the name was never taken")
	}

	r.Unregister("cycle")

	second := categorizedProbe("cycle", CategoryGeneral)
	if err := r.Register(second); err != nil {
		t.Fatalf("re-Register after Unregister: %v", err)
	}
	if got := r.Get("cycle"); got != second {
		t.Error("the registry kept the first registration; a second test run would " +
			"still execute the first run's closure")
	}
}
