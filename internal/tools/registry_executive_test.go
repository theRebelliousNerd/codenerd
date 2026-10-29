package tools

import (
	"context"
	"errors"
	"testing"
)

// The guard an executive binds is released by that executive only. A guard
// left bound after its owner closed refused every later call; a blanket
// release would strip a guard another live executive still relies on.
func TestRegistry_ExecutiveIsReleasedByIdentity(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	if err := r.Register(categorizedProbe("probe", CategoryGeneral)); err != nil {
		t.Fatal(err)
	}
	refuse := errors.New("refused")
	first, second := new(int), new(int)
	r.BindExecutive(first, func(context.Context, string, map[string]any) error { return refuse }, nil)
	if _, err := r.Execute(context.Background(), "probe", nil); !errors.Is(err, refuse) {
		t.Fatalf("bound guard did not run: %v", err)
	}

	r.BindExecutive(second, func(context.Context, string, map[string]any) error { return refuse }, nil)
	if r.ReleaseExecutive(first) {
		t.Fatal("an executive released a binding another one made since")
	}
	if _, err := r.Execute(context.Background(), "probe", nil); !errors.Is(err, refuse) {
		t.Fatalf("the live executive's guard was stripped: %v", err)
	}

	if !r.ReleaseExecutive(second) {
		t.Fatal("the binding executive could not release its guard")
	}
	if _, err := r.Execute(context.Background(), "probe", nil); err != nil {
		t.Fatalf("a released guard still refuses: %v", err)
	}
	if r.ReleaseExecutive(nil) {
		t.Fatal("a nil owner released something")
	}
}
