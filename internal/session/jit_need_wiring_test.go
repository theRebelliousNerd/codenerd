package session

import (
	"context"
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The executor asks the kernel for the target's needs at every compile
// boundary; it does not decide them. The need follows the target: a policy
// file carries /authoring_mangle, a Go file /authoring_go, both carry
// /authoring_code, a Markdown document none of them; a compile whose language
// nothing could tell is asked as /undetected and carries /authoring_code only.
func TestCompilationContextCarriesTheKernelsDerivedNeeds(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)

	for _, tc := range []struct {
		target string
		want   []string
		absent []string
	}{
		{"internal/context/working_set.mg", []string{"authoring_mangle", "authoring_code"}, []string{"authoring_go"}},
		{"internal/session/executor.go", []string{"authoring_go", "authoring_code"}, []string{"authoring_mangle"}},
		{"Docs/architecture/features/03-GAP-ANALYSIS.md", nil, []string{"authoring_go", "authoring_code", "authoring_mangle"}},
		{"Makefile", []string{"authoring_code"}, []string{"authoring_go", "authoring_mangle"}},
	} {
		cc := e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: tc.target})
		for _, need := range tc.want {
			if !slices.Contains(cc.DerivedNeeds, need) {
				t.Errorf("a turn aimed at %s carries needs %v, want %s from the kernel", tc.target, cc.DerivedNeeds, need)
			}
		}
		for _, need := range tc.absent {
			if slices.Contains(cc.DerivedNeeds, need) {
				t.Errorf("a turn aimed at %s carries needs %v, must not carry %s", tc.target, cc.DerivedNeeds, need)
			}
		}
	}
	if len(e.targetNeeds("not an atom")) != 0 {
		t.Errorf("a malformed language reached the kernel query")
	}
}

// A planned step re-derives the need for its own file: a turn aimed at a
// document whose plan edits a Go file compiles that step with the Go need.
func TestStepCompileDerivesTheStepFilesNeeds(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	var compiled []string
	jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		compiled = slices.Clone(cc.DerivedNeeds)
		return &prompt.CompilationResult{Prompt: "step prompt"}, nil
	}}
	e := NewExecutor(k, nil, nil, jit, nil, nil)
	turn := e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: "README.md"})
	if slices.Contains(turn.DerivedNeeds, "authoring_go") || slices.Contains(turn.DerivedNeeds, "authoring_code") {
		t.Fatalf("the document turn carries %v", turn.DerivedNeeds)
	}
	step := turn.Clone()
	step.IntentTarget = "internal/session/work_steps.go"
	e.stepSystemPrompt(t.Context(), "turn prompt", step, nil)
	if !slices.Contains(compiled, "authoring_go") || !slices.Contains(compiled, "authoring_code") {
		t.Errorf("the Go step compiled with needs %v, want authoring_go and authoring_code", compiled)
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
