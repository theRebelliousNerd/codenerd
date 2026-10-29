package session

import (
	"context"
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/experts"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

func expertRosterKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{experts.AgentAtom("MangleExpert")}},
		{Predicate: "shard_agent_domain", Args: []any{experts.AgentAtom("MangleExpert"), types.MangleAtom("/mangle"), types.MangleAtom("/registry")}},
	}); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
	return k
}

// The turn's compile carries the shard experts the kernel derived for its
// target: a `nerd fix` aimed at a policy file pushes the Mangle expert's
// knowledge, a turn on a Go file does not, and nothing of the selection is
// left in the kernel. Before expert_needed existed this compile never named
// an expert on any path but two chat delegations.
func TestCompilationContextCarriesTheKernelsNeededExperts(t *testing.T) {
	t.Parallel()
	k := expertRosterKernel(t)
	e := NewExecutor(k, nil, nil, nil, nil, nil)

	cc := e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: "internal/core/defaults/policy/shards.mg"})
	if !slices.Equal(cc.Experts, []string{"mangleexpert"}) {
		t.Errorf("a fix aimed at a policy file compiles with experts %v, want [mangleexpert]", cc.Experts)
	}
	cc = e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: "internal/session/executor.go"})
	if len(cc.Experts) != 0 {
		t.Errorf("a fix aimed at a Go file compiles with experts %v, want none", cc.Experts)
	}
	if rows, _ := k.Query("expert_task"); len(rows) != 0 {
		t.Errorf("the selection left expert_task rows behind: %v", rows)
	}
}

// A planned step re-asks for its own file: a turn aimed at a Go file whose
// plan edits a policy file compiles that step with the Mangle expert.
func TestStepCompileDerivesTheStepFilesExperts(t *testing.T) {
	t.Parallel()
	k := expertRosterKernel(t)
	var compiled []string
	jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		compiled = slices.Clone(cc.Experts)
		return &prompt.CompilationResult{Prompt: "step prompt"}, nil
	}}
	e := NewExecutor(k, nil, nil, jit, nil, nil)
	turn := e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: "internal/session/work_steps.go"})
	if len(turn.Experts) != 0 {
		t.Fatalf("the Go turn carries experts %v", turn.Experts)
	}
	step := turn.Clone()
	step.IntentTarget = "internal/context/working_set.mg"
	e.stepSystemPrompt(t.Context(), "turn prompt", step, nil)
	if !slices.Equal(compiled, []string{"mangleexpert"}) {
		t.Errorf("the policy step compiled with experts %v, want [mangleexpert]", compiled)
	}
}
