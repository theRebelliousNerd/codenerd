package session

import (
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// The executor asks the kernel for the target's needs at every compile
// boundary; it does not decide them. A turn aimed at a policy file carries the
// derived /authoring_mangle need into its compilation context, a Go turn none.
func TestCompilationContextCarriesTheKernelsDerivedNeeds(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)

	mg := e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: "internal/context/working_set.mg"})
	if !slices.Contains(mg.DerivedNeeds, "authoring_mangle") {
		t.Errorf("a turn aimed at a .mg file carries needs %v, want authoring_mangle from the kernel", mg.DerivedNeeds)
	}
	goCtx := e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: "internal/session/executor.go"})
	if len(goCtx.DerivedNeeds) != 0 {
		t.Errorf("a turn aimed at a Go file carries needs %v, want none", goCtx.DerivedNeeds)
	}
	if len(e.targetNeeds("not an atom")) != 0 {
		t.Errorf("a malformed language reached the kernel query")
	}
}
