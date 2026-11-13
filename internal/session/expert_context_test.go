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

	cc := turnCC(e, t.Context(), perception.Intent{Verb: "/fix", Target: "internal/core/defaults/policy/shards.mg"})
	if !slices.Equal(cc.Experts, []string{"mangleexpert"}) {
		t.Errorf("a fix aimed at a policy file compiles with experts %v, want [mangleexpert]", cc.Experts)
	}
	cc = turnCC(e, t.Context(), perception.Intent{Verb: "/fix", Target: "internal/session/executor.go"})
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
	turn := turnCC(e, t.Context(), perception.Intent{Verb: "/fix", Target: "internal/session/work_steps.go"})
	if len(turn.Experts) != 0 {
		t.Fatalf("the Go turn carries experts %v", turn.Experts)
	}
	step := turn.Clone()
	step.IntentTarget = "internal/context/working_set.mg"
	e.stepSystemPrompt(t.Context(), "turn prompt", step, nil, "", "", nil)
	if !slices.Equal(compiled, []string{"mangleexpert"}) {
		t.Errorf("the policy step compiled with experts %v, want [mangleexpert]", compiled)
	}
}

type stepReaderFunc func(ctx context.Context, file, change string, techs map[string][]string) (map[string]int64, error)

func (f stepReaderFunc) ReadStep(ctx context.Context, file, change string, techs map[string][]string) (map[string]int64, error) {
	return f(ctx, file, change, techs)
}

// A planned step on a Go file that imports a Mangle library is read for what
// the step says it will change: read as Go-only, the step compiles without the
// Mangle expert; read as writing Mangle code, with it. The reader receives the
// step's own change text.
func TestStepCompileDealsImportedExpertsByTheStepsChange(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		percent int64
		want    []string
	}{
		{"go only", 6, nil},
		{"writes mangle", 97, []string{"mangleexpert"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := expertRosterKernel(t)
			if err := k.AssertBatch([]types.Fact{
				{Predicate: "technology_import_prefix", Args: []any{"codeberg.org/TauCeti/mangle-go", types.MangleAtom("/mangle")}},
				{Predicate: "dependency_link", Args: []any{"internal/core/kernel_query.go", "pkg:codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/ast"}},
				{Predicate: "config_param", Args: []any{types.MangleAtom("/jev_yes_percent"), int64(70)}},
				{Predicate: "config_param", Args: []any{types.MangleAtom("/jev_no_percent"), int64(30)}},
			}); err != nil {
				t.Fatal(err)
			}
			var compiled []string
			jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
				compiled = slices.Clone(cc.Experts)
				return &prompt.CompilationResult{Prompt: "step prompt"}, nil
			}}
			e := NewExecutor(k, nil, nil, jit, nil, nil)
			var gotChange string
			e.SetStepTechnologyReader(stepReaderFunc(func(_ context.Context, _, change string, techs map[string][]string) (map[string]int64, error) {
				gotChange = change
				return map[string]int64{"mangle": tc.percent}, nil
			}))
			step := turnCC(e, t.Context(), perception.Intent{Verb: "/fix", Target: "internal/core/kernel_query.go"}).Clone()
			step.IntentTarget = "internal/core/kernel_query.go"
			const change = "return early from Query when the predicate name is empty"
			e.stepSystemPrompt(t.Context(), "turn prompt", step, nil, "", change, nil)
			if gotChange != change {
				t.Fatalf("reader got change %q, want the step's change", gotChange)
			}
			if !slices.Equal(compiled, tc.want) {
				t.Fatalf("step compiled with experts %v, want %v", compiled, tc.want)
			}
		})
	}
}
