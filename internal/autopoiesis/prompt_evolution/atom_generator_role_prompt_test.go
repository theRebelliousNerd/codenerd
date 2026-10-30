package prompt_evolution

import (
	"context"
	"errors"
	"slices"
	"testing"

	"codenerd/internal/types"
)

// recordingRoles records each role request and compiles "<role> prompt".
type recordingRoles struct{ requests []types.RolePromptRequest }

func (r *recordingRoles) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	r.requests = append(r.requests, req)
	return req.Role + " prompt", nil
}

// systemCapture records the system prompt of each call and answers with an
// atom and a refined strategy both parsers accept.
type systemCapture struct{ systems []string }

func (s *systemCapture) Complete(context.Context, string) (string, error) {
	return "", errors.New("a call without a system prompt")
}

func (s *systemCapture) CompleteWithSystem(_ context.Context, system, _ string) (string, error) {
	s.systems = append(s.systems, system)
	return "```yaml\n- id: test/evolved/captured\n  category: methodology\n  content: Always check the error.\n```", nil
}

// Atom generation and strategy refinement send the prompts compiled for
// their roles (/prompt_atom_generator, /strategy_refiner) with intent /judge.
// Until 2026-10-28 both sent Go constants and their atoms were never compiled.
func TestAtomGenerator_SendsItsCompiledRolePrompts(t *testing.T) {
	llm := &systemCapture{}
	roles := &recordingRoles{}
	ag := NewAtomGeneratorWithPinScope(llm, roles, nil, DefaultPinScope)
	ctx := context.Background()

	if _, err := ag.GenerateFromFailures(ctx, testGenerationCohort(t, oneFailure()), "/coder", string(ProblemDebugging),
		ServingPin{Provider: "anthropic", Model: "claude-opus-4-20260501"}); err != nil {
		t.Fatalf("GenerateFromFailures: %v", err)
	}
	if _, err := ag.RefineStrategy(ctx, &Strategy{ID: "s1", ProblemType: ProblemDebugging, ShardType: "/coder", Content: "1. Read the error."}, oneFailure()); err != nil {
		t.Fatalf("RefineStrategy: %v", err)
	}

	wantRequests := []types.RolePromptRequest{
		{Role: "prompt_atom_generator", Intent: "judge"},
		{Role: "strategy_refiner", Intent: "judge"},
	}
	if !slices.Equal(roles.requests, wantRequests) {
		t.Errorf("role requests = %+v, want %+v", roles.requests, wantRequests)
	}
	if want := []string{"prompt_atom_generator prompt", "strategy_refiner prompt"}; !slices.Equal(llm.systems, want) {
		t.Errorf("system prompts sent = %q, want %q", llm.systems, want)
	}
}

// Without a role prompt compiler neither call reaches the model; each is an
// error wrapping types.ErrRolePromptUnavailable.
func TestAtomGenerator_NoRolePromptsIsAnError(t *testing.T) {
	llm := &systemCapture{}
	ag := NewAtomGeneratorWithPinScope(llm, nil, nil, DefaultPinScope)
	ctx := context.Background()

	if _, err := ag.GenerateFromFailures(ctx, testGenerationCohort(t, oneFailure()), "/coder", string(ProblemDebugging),
		ServingPin{Provider: "anthropic", Model: "claude-opus-4-20260501"}); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("GenerateFromFailures error = %v, want ErrRolePromptUnavailable", err)
	}
	if _, err := ag.RefineStrategy(ctx, &Strategy{ID: "s1", ProblemType: ProblemDebugging, ShardType: "/coder", Content: "1. Read the error."}, oneFailure()); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("RefineStrategy error = %v, want ErrRolePromptUnavailable", err)
	}
	if len(llm.systems) != 0 {
		t.Errorf("the model was called %d times without a compiled prompt", len(llm.systems))
	}
}
