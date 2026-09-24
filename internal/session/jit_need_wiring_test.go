package session

import (
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
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

// The compile carries what the answer's reader needs, asked of the kernel with
// the channel the turn's client answers on (consumer_need). A client that
// carries tools in the envelope is taught envelope tool_requests
// (/envelope_tool_requests); a native client is not, and no executor compile is
// taught knowledge_requests, which only the chat turn's articulation consults.
func TestCompilationContextCarriesTheConsumersNeeds(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	for _, tc := range []struct {
		name     string
		client   types.LLMClient
		wantTool bool
	}{
		{"envelope-tool client", &piggybackStaticClient{MockLLMClient: &MockLLMClient{}}, true},
		{"native client", &MockLLMClient{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewExecutor(k, nil, tc.client, nil, nil, nil)
			cc := e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: "internal/session/executor.go"})
			if got := slices.Contains(cc.DerivedNeeds, "envelope_tool_requests"); got != tc.wantTool {
				t.Errorf("needs %v: envelope_tool_requests = %v, want %v", cc.DerivedNeeds, got, tc.wantTool)
			}
			if slices.Contains(cc.DerivedNeeds, "envelope_knowledge_requests") {
				t.Errorf("an executor compile was taught knowledge_requests nothing on this path consults: %v", cc.DerivedNeeds)
			}
		})
	}
}
