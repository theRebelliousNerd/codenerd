//go:build integration

package e2e_test

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

type mechanicsCatalogCompiler struct{ available []string }

func (m *mechanicsCatalogCompiler) Compile(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	m.available = append([]string(nil), cc.AvailableTools...)
	return &prompt.CompilationResult{Prompt: "mechanics catalog control"}, nil
}

type mechanicsCatalogModel struct {
	*mockLLMClient
	offered []string
}

func (m *mechanicsCatalogModel) CompleteWithTools(ctx context.Context, system, input string, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	for _, d := range definitions {
		m.offered = append(m.offered, d.Name)
	}
	return m.mockLLMClient.CompleteWithTools(ctx, system, input, definitions)
}

func TestMechanicsCatalogDispatchBoundary(t *testing.T) {
	for _, mode := range []string{"active_catalog", "empty_override", "registered_ungranted", "production_kernel_refusal"} {
		t.Run(mode, func(t *testing.T) {
			ownMechanicsFixtureRegistry(t)
			root := t.TempDir()
			var kernel types.Kernel
			if mode == "production_kernel_refusal" {
				real, err := core.NewRealKernel()
				if err != nil {
					t.Fatal(err)
				}
				real.SetWorkspace(root)
				kernel = real
			} else {
				scripted := newMockKernel()
				if mode == "empty_override" {
					scripted.queryResults["turn_catalog"] = nil
				}
				kernel = scripted
			}
			calls := map[string]int{}
			for _, name := range []string{"mock_tool", "registered_ungranted"} {
				if err := tools.Global().Register(&tools.Tool{Name: name, Effect: tools.EffectRead, Execute: func(context.Context, map[string]any) (string, error) {
					calls[name]++
					return "actual fixture callback", nil
				}}); err != nil {
					t.Fatal(err)
				}
			}
			name := "mock_tool"
			if mode == "registered_ungranted" {
				name = mode
			}
			call := types.ToolCall{ID: "catalog-probe", Name: name, Input: map[string]any{}}
			if mode == "production_kernel_refusal" {
				for _, fact := range []types.Fact{
					{Predicate: "permitted_action", Args: []any{"exec-" + call.ID, types.MangleAtom("/" + name), "unknown", "{}", time.Now().Unix()}},
					{Predicate: "permission_check_result", Args: []any{"exec-" + call.ID, types.MangleAtom("/permit"), "exact control approval", time.Now().Unix()}},
				} {
					if err := kernel.Assert(fact); err != nil {
						t.Fatal(err)
					}
				}
			}
			model := &mechanicsCatalogModel{mockLLMClient: &mockLLMClient{completeFunc: func(context.Context, string, string) (*types.LLMToolResponse, error) {
				return &types.LLMToolResponse{Text: "catalog probe", ToolCalls: []types.ToolCall{call}}, nil
			}}}
			compiler := &mechanicsCatalogCompiler{}
			executor := session.NewExecutor(kernel, &mockVirtualStore{}, model, compiler, &mockConfigFactory{}, &mockTransducer{intent: perception.Intent{Verb: "/explain"}})
			cfg := session.DefaultExecutorConfig()
			cfg.WorkspaceRoot = root
			if mode != "production_kernel_refusal" {
				// These existing inert mechanics doubles do not simulate the
				// constitution. The production-kernel negative keeps it enabled.
				cfg.EnableSafetyGate = false
			}
			executor.SetConfig(cfg)
			result, err := executor.Process(context.Background(), "read the controlled fixture")
			if result == nil {
				t.Fatalf("missing measured result: %v", err)
			}
			if mode == "active_catalog" {
				if err != nil || calls[name] != 1 || result.ToolCallsExecuted != 1 || result.SuccessfulToolCalls != 1 {
					t.Fatalf("declared callback never executed: calls=%v result=%+v err=%v", calls, result, err)
				}
			} else if calls[name] != 0 || result.SuccessfulToolCalls != 0 {
				t.Fatalf("catalog refusal ran a registered callback: mode=%s calls=%v result=%+v", mode, calls, result)
			}
			want := []string(nil)
			if mode == "active_catalog" || mode == "registered_ungranted" {
				want = []string{"mock_tool"}
			}
			if !reflect.DeepEqual(compiler.available, want) || !reflect.DeepEqual(model.offered, want) {
				t.Fatalf("compile/provider catalog drift: compile=%v offered=%v want=%v", compiler.available, model.offered, want)
			}
		})
	}
}

func TestMechanicsCatalogRetirementMatchesProduction(t *testing.T) {
	for _, missingKey := range []bool{false, true} {
		t.Run(map[bool]string{false: "first_argument_only", true: "missing_key_refused"}[missingKey], func(t *testing.T) {
			mock := newMockKernel()
			real, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			real.SetWorkspace(t.TempDir())
			for _, turn := range []string{"/turn_1_1", "/turn_1_2"} {
				f := types.Fact{Predicate: "turn_verb", Args: []any{types.MangleAtom(turn), types.MangleAtom("/explain")}}
				if err := mock.Assert(f); err != nil {
					t.Fatal(err)
				}
				if err := real.Assert(f); err != nil {
					t.Fatal(err)
				}
			}
			f := types.Fact{Predicate: "turn_verb", Args: []any{types.MangleAtom("/turn_1_1"), types.MangleAtom("/different_verb")}}
			if missingKey {
				f.Args = nil
			}
			mockErr, realErr := mock.RetractFact(f), real.RetractFact(f)
			if (mockErr != nil) != (realErr != nil) {
				t.Fatalf("retraction admission drift: mock=%v real=%v", mockErr, realErr)
			}
			rows, err := real.Query("turn_verb")
			if err != nil {
				t.Fatal(err)
			}
			var got, want []string
			for _, f := range mock.facts {
				got = append(got, types.ExtractString(f.Args[0]))
			}
			for _, f := range rows {
				want = append(want, types.ExtractString(f.Args[0]))
			}
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("sibling turn was lost: mock=%v real=%v", got, want)
			}
			tools, err := prompt.DeriveTurnTools(mock, "/explain", "/turn_1_2")
			if err != nil || len(tools) != len(mechanicsFixtureTools()) {
				t.Fatalf("live sibling catalog unavailable: tools=%v err=%v", tools, err)
			}
			if tools, err := prompt.DeriveTurnTools(mock, "/explain", "/turn_9_9"); err == nil || len(tools) != 0 {
				t.Fatal("foreign turn obtained a fixture catalog")
			}
		})
	}
}

func TestMechanicsFixtureRegistryOwnership(t *testing.T) {
	prior := tools.Global()
	priorRoot, priorTool := prior.WorkspaceRoot(), prior.Get("mock_tool")
	t.Run("owned_repetition", func(t *testing.T) {
		setupTestExecutor(t)
		if tools.Global() == prior {
			t.Fatal("fixture reused another owner's registry/callbacks")
		}
	})
	if tools.Global() != prior || prior.WorkspaceRoot() != priorRoot || prior.Get("mock_tool") != priorTool {
		t.Fatal("completed fixture failed to restore the untouched prior registry")
	}
	later := tools.NewRegistry()
	ownedCalls, laterCalls := 0, 0
	t.Cleanup(func() {
		if tools.Global() == later {
			tools.SwapGlobal(prior)
		}
	})
	t.Run("later_owner", func(t *testing.T) {
		executor, _, model := setupTestExecutor(t)
		if err := tools.Global().Register(&tools.Tool{Name: "mock_tool", Effect: tools.EffectRead, Execute: func(context.Context, map[string]any) (string, error) {
			ownedCalls++
			return "owned", nil
		}}); err != nil {
			t.Fatal(err)
		}
		if err := later.Register(&tools.Tool{Name: "mock_tool", Effect: tools.EffectRead, Execute: func(context.Context, map[string]any) (string, error) {
			laterCalls++
			return "later", nil
		}}); err != nil {
			t.Fatal(err)
		}
		model.completeFunc = func(context.Context, string, string) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "owned registry", ToolCalls: []types.ToolCall{{ID: "owner-call", Name: "mock_tool"}}}, nil
		}
		tools.SwapGlobal(later)
		result, err := executor.Process(context.Background(), "read the owned fixture")
		if err != nil || result == nil || result.SuccessfulToolCalls != 1 || ownedCalls != 1 || laterCalls != 0 {
			t.Fatalf("later global binding redirected owned execution: result=%+v err=%v own=%d later=%d", result, err, ownedCalls, laterCalls)
		}
	})
	if tools.Global() != later {
		t.Fatal("fixture cleanup overwrote a later registry owner")
	}
}
