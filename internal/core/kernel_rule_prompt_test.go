package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/types"
)

// ruleGeneratorPromptText is the system prompt fakeRuleRolePrompts compiles.
const ruleGeneratorPromptText = "test mangle rule generator role prompt"

type fakeRuleRolePrompts struct {
	err error
	got []types.RolePromptRequest
}

func (f *fakeRuleRolePrompts) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	f.got = append(f.got, req)
	if f.err != nil {
		return "", f.err
	}
	return ruleGeneratorPromptText, nil
}

// recordingRuleLLM records every system prompt and message it is sent.
type recordingRuleLLM struct {
	mu      sync.Mutex
	systems []string
	users   []string
	reply   string
}

func (r *recordingRuleLLM) Complete(_ context.Context, system, user string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.systems = append(r.systems, system)
	r.users = append(r.users, user)
	return r.reply, nil
}

// The rule generator's system prompt is its role compile with the declared
// predicates appended; its message is the purpose and context, no
// instructions.
func TestGenerateValidatedRule_SendsTheRolePromptAndTheRequestAsData(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	rp := &fakeRuleRolePrompts{}
	k.SetRolePrompts(rp)
	llm := &recordingRuleLLM{reply: "not a rule"}
	_, _ = k.GenerateValidatedRule(context.Background(), llm, "flag stale files", map[string]string{"zeta": "2", "alpha": "1"}, "executive")

	if len(rp.got) != 1 || rp.got[0].Role != "mangle_rule_generator" || rp.got[0].Intent != "judge" {
		t.Fatalf("compiled %+v, want one compile of role mangle_rule_generator intent judge", rp.got)
	}
	if len(llm.systems) == 0 {
		t.Fatal("the model was never asked")
	}
	system, user := llm.systems[0], llm.users[0]
	if !strings.HasPrefix(system, ruleGeneratorPromptText) {
		t.Errorf("system prompt does not start with the compiled role prompt:\n%s", system)
	}
	if !strings.Contains(system, "## Declared Predicates (use ONLY these):") {
		t.Error("the declared predicates were not appended to the role prompt")
	}
	if !strings.HasPrefix(user, "Purpose:\nflag stale files\n\n## Additional Context:\n- alpha: 1\n- zeta: 2\n") {
		t.Errorf("message = %q, want the purpose and the sorted context", user)
	}
	for _, instruction := range []string{"You are", "Generate a Mangle rule", "Output ONLY the rule"} {
		if strings.Contains(user, instruction) {
			t.Errorf("the message carries the instruction %q; it belongs to the role prompt", instruction)
		}
	}
}

// No compiler, or a failed compile, fails rule generation before the model is
// asked: there is no Go prompt to generate from instead.
func TestGenerateValidatedRule_RolePromptUnavailableIsAnError(t *testing.T) {
	t.Parallel()
	for name, rp := range map[string]types.RolePromptCompiler{
		"no compiler":    nil,
		"failed compile": &fakeRuleRolePrompts{err: types.ErrRolePromptUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			k, err := NewRealKernel()
			if err != nil {
				t.Fatalf("NewRealKernel: %v", err)
			}
			k.SetRolePrompts(rp)
			llm := &recordingRuleLLM{}
			if _, err := k.GenerateValidatedRule(context.Background(), llm, "flag stale files", nil, "executive"); !errors.Is(err, types.ErrRolePromptUnavailable) {
				t.Errorf("err = %v, want ErrRolePromptUnavailable", err)
			}
			if len(llm.systems) != 0 {
				t.Error("the model was asked without the role prompt")
			}
		})
	}
}
