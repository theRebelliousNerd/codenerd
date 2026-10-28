package feedback

import (
	"context"
	"strings"
	"testing"
)

type guidedValidator struct {
	MockRuleValidator
	guidance string
}

func (g *guidedValidator) GeneratedRuleGuidance() string { return g.guidance }

type capturingLLM struct {
	response string
	systems  []string
}

func (c *capturingLLM) Complete(_ context.Context, systemPrompt, _ string) (string, error) {
	c.systems = append(c.systems, systemPrompt)
	return c.response, nil
}

// A validator that knows its host's capabilities has them appended to the
// generated-rule system prompt; one that does not leaves the prompt alone.
func TestGenerateAndValidate_AppendsHostCapabilityGuidance(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableAutoRepair = false
	rule := `next_action(/run_tests) :- test_state(/failing).`
	validator := &guidedValidator{
		MockRuleValidator: MockRuleValidator{validRules: map[string]bool{rule: true}, predicates: []string{"test_state/1", "next_action/1"}},
		guidance:          "## Host capabilities\nDo not use temporal syntax.",
	}
	llm := &capturingLLM{response: rule}
	if _, err := NewFeedbackLoop(cfg).GenerateAndValidate(context.Background(), llm, validator, "base system", "make a rule", "executive"); err != nil {
		t.Fatal(err)
	}
	if len(llm.systems) == 0 || !strings.Contains(llm.systems[0], "Do not use temporal syntax.") || !strings.HasPrefix(llm.systems[0], "base system") {
		t.Fatalf("guidance not appended to the system prompt: %q", llm.systems)
	}

	plain := &capturingLLM{response: rule}
	if _, err := NewFeedbackLoop(cfg).GenerateAndValidate(context.Background(), plain, &validator.MockRuleValidator, "base system", "make a rule", "executive"); err != nil {
		t.Fatal(err)
	}
	if len(plain.systems) == 0 || strings.Contains(plain.systems[0], "Host capabilities") {
		t.Fatalf("a validator without a guide changed the prompt: %q", plain.systems)
	}
}
