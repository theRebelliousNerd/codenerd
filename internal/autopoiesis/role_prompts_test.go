package autopoiesis

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"codenerd/internal/types"
)

// recordingRolePrompts records each role request and compiles
// "role prompt for <role>"; with fail set every compile fails.
type recordingRolePrompts struct {
	mu       sync.Mutex
	requests []types.RolePromptRequest
	fail     bool
}

func (r *recordingRolePrompts) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
	if r.fail {
		return "", errors.New("compile failed")
	}
	return "role prompt for " + req.Role, nil
}

// systemRecordingLLM records the system prompt of every call and answers each
// role with a reply its parser accepts.
type systemRecordingLLM struct {
	mu       sync.Mutex
	systems  []string
	complete int
}

func (s *systemRecordingLLM) Complete(context.Context, string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.complete++
	return "", errors.New("a call without a system prompt")
}

func (s *systemRecordingLLM) CompleteWithSystem(_ context.Context, system, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.systems = append(s.systems, system)
	switch system {
	case "role prompt for " + rolePanicMaker:
		return `[{"name":"Nil Input","category":"nil_pointer","input":"","description":"d","expected_failure":"panic"}]`, nil
	case "role prompt for " + roleToolTestWriter:
		return "```go\npackage tools\n\nimport \"testing\"\n\nfunc TestTool(t *testing.T) { t.Log(\"generated\") }\n```", nil
	case "role prompt for " + roleAgentPromptWriter:
		return "Review pull requests for this repository.", nil
	}
	return `{"complexity": "complex", "needs_persistent": true, "agent_type": "watcher", "confidence": 0.9, "tool_name": "csv_tool", "purpose": "p", "input_type": "string", "output_type": "string", "completeness": 0.5}`, nil
}

// Each autopoiesis role call sends the prompt compiled for its own role, with
// intent /judge, as the system prompt, and never calls the model without one.
// Until 2026-10-28 every one of them was a Go-text prompt sent with Complete.
func TestRolePrompts_EachCallSendsItsCompiledRolePrompt(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(LLMClient, types.RolePromptCompiler) error{
		roleComplexityAnalyzer: func(llm LLMClient, roles types.RolePromptCompiler) error {
			ca := NewComplexityAnalyzer(llm)
			ca.SetRolePrompts(roles)
			// A codebase-wide request scores 0.5: the heuristics ask the model.
			_, err := ca.AnalyzeWithLLM(ctx, "tidy up the codebase")
			return err
		},
		rolePersistenceAnalyzer: func(llm LLMClient, roles types.RolePromptCompiler) error {
			pa := NewPersistenceAnalyzer(llm)
			pa.SetRolePrompts(roles)
			_, err := pa.AnalyzeWithLLM(ctx, "Help me track how the codebase evolves")
			return err
		},
		roleAgentPromptWriter: func(llm LLMClient, roles types.RolePromptCompiler) error {
			ac := NewAgentCreator(llm, t.TempDir())
			ac.SetRolePrompts(roles)
			_, err := ac.CreateFromNeed(ctx, PersistenceNeed{AgentType: "reviewer", Purpose: "Review code"})
			return err
		},
		roleToolNeedRefiner: func(llm LLMClient, roles types.RolePromptCompiler) error {
			tg := NewToolGenerator(llm, t.TempDir())
			tg.SetRolePrompts(roles)
			_, err := tg.refineToolNeedWithLLM(ctx, "convert Avro to CSV", "", "converter", nil)
			return err
		},
		roleToolTestWriter: func(llm LLMClient, roles types.RolePromptCompiler) error {
			tg := NewToolGenerator(llm, t.TempDir())
			tg.SetRolePrompts(roles)
			_, err := tg.generateTestCode(ctx, &ToolNeed{Name: "csv_tool", InputType: "string", OutputType: "string"}, "package tools")
			return err
		},
		roleToolProfiler: func(llm LLMClient, roles types.RolePromptCompiler) error {
			o := &Orchestrator{client: llm, profiles: NewProfileStore(t.TempDir()), rolePrompts: roles}
			_, err := o.GenerateToolProfile(ctx, "csv_tool", "converts", "package tools")
			return err
		},
		roleToolQualityAssessor: func(llm LLMClient, roles types.RolePromptCompiler) error {
			qe := NewQualityEvaluator(llm, nil)
			qe.SetRolePrompts(roles)
			_, err := qe.EvaluateWithLLM(ctx, &ExecutionFeedback{ToolName: "csv_tool", Output: "a,b", Success: true})
			return err
		},
		rolePanicMaker: func(llm LLMClient, roles types.RolePromptCompiler) error {
			pm := NewPanicMaker(llm)
			pm.SetRolePrompts(roles)
			_, err := pm.GenerateAttacks(ctx, "package tools")
			return err
		},
	}
	for role, call := range calls {
		t.Run(role, func(t *testing.T) {
			llm := &systemRecordingLLM{}
			roles := &recordingRolePrompts{}
			if err := call(llm, roles); err != nil {
				t.Fatalf("the %s call: %v", role, err)
			}
			if want := []types.RolePromptRequest{{Role: role, Intent: "judge"}}; !slices.Equal(roles.requests, want) {
				t.Errorf("role requests = %+v, want %+v", roles.requests, want)
			}
			if want := []string{"role prompt for " + role}; !slices.Equal(llm.systems, want) || llm.complete != 0 {
				t.Errorf("system prompts sent = %q (and %d calls without one), want %q", llm.systems, llm.complete, want)
			}

			for name, roles := range map[string]types.RolePromptCompiler{
				"no compiler":    nil,
				"failed compile": &recordingRolePrompts{fail: true},
			} {
				unwired := &systemRecordingLLM{}
				if err := call(unwired, roles); !errors.Is(err, types.ErrRolePromptUnavailable) {
					t.Errorf("%s: error = %v, want ErrRolePromptUnavailable", name, err)
				}
				if len(unwired.systems) != 0 || unwired.complete != 0 {
					t.Errorf("%s: the model was called without a compiled prompt", name)
				}
			}
		})
	}
}

// The orchestrator hands its role prompt compiler to every caller, including
// the Ouroboros loop's own tool generator and PanicMaker, and its assembler to
// both tool generators: until 2026-10-28 the loop's generator never received
// the assembler and generated every tool on the Go-text fallback prompts.
func TestOrchestrator_PromptCompilersReachEveryCaller(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	roles := &recordingRolePrompts{}
	orch.SetRolePrompts(roles)
	assembler := stagePromptAssembler{}
	orch.SetPromptAssembler(assembler)

	loop := orch.GetConcreteOuroborosLoop()
	if loop == nil {
		t.Fatal("no Ouroboros loop")
	}
	for name, got := range map[string]types.RolePromptCompiler{
		"complexity":     orch.complexity.rolePrompts,
		"persistence":    orch.persistence.rolePrompts,
		"agent creator":  orch.agentCreate.rolePrompts,
		"tool generator": orch.toolGen.rolePrompts,
		"evaluator":      orch.evaluator.rolePrompts,
		"loop generator": loop.toolGen.rolePrompts,
	} {
		if got != roles {
			t.Errorf("%s has role prompts %v, want the orchestrator's", name, got)
		}
	}
	if loop.panicMaker != nil && loop.panicMaker.rolePrompts != roles {
		t.Error("the loop's PanicMaker lacks the orchestrator's role prompts")
	}
	if loop.toolGen.promptAssembler != assembler || orch.toolGen.promptAssembler != assembler || orch.refiner.promptAssembler != assembler {
		t.Error("an Ouroboros tool generator or the refiner lacks the orchestrator's assembler")
	}
}
