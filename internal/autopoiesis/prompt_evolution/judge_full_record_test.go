package prompt_evolution_test

import (
	"context"
	"strings"
	"testing"

	pe "codenerd/internal/autopoiesis/prompt_evolution"
	nerdconfig "codenerd/internal/config"
	"codenerd/internal/prompt"
)

type capturingJudgeClient struct {
	system string
	user   string
}

func (c *capturingJudgeClient) Complete(_ context.Context, _ string) (string, error) {
	return `{"verdict": "PASS", "explanation": "done", "category": "CORRECT"}`, nil
}

func (c *capturingJudgeClient) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	c.system = system
	c.user = user
	return `{"verdict": "PASS", "explanation": "done", "category": "CORRECT"}`, nil
}

type stubJudgeCompiler struct{}

func (stubJudgeCompiler) Compile(_ context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{Prompt: "stub judge prompt"}, nil
}

// The judge's verdict decides which prompt atoms evolve, so the record the
// judge sees must not be cut short: late actions usually hold the fix.
func TestTaskJudge_IncludesFullRecord(t *testing.T) {
	actions := make([]pe.AgentAction, 0, 15)
	for i := 0; i < 15; i++ {
		actions = append(actions, pe.AgentAction{
			Type:        "edit",
			Description: "action-marker",
			Target:      "file.go",
			Success:     true,
		})
	}
	actions[14].Description = "late-fix-edited-auth"
	actions[14].Target = "internal/auth/late_fix_target.go"

	buildErr := strings.Repeat("E", 500) + "BUILD_TAIL_MARKER"
	output := strings.Repeat("O", 2000) + "OUTPUT_TAIL_MARKER"
	thought := strings.Repeat("T", 3000) + "THOUGHT_TAIL_MARKER"

	exec := &pe.ExecutionRecord{
		TaskID:       "full-record",
		ShardType:    "/coder",
		TaskRequest:  "fix auth",
		AgentActions: actions,
		ExecutionResult: pe.ExecutionResult{
			Success:     true,
			BuildErrors: []string{buildErr},
			Output:      output,
		},
		ThoughtSummary: thought,
	}

	client := &capturingJudgeClient{}
	judge := pe.NewTaskJudge(client, "test-model", stubJudgeCompiler{}, nerdconfig.DefaultJITConfig())
	if _, err := judge.Evaluate(context.Background(), exec); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	cases := []struct {
		name string
		want string
	}{
		{"late action visible", "late-fix-edited-auth"},
		{"late action target visible", "internal/auth/late_fix_target.go"},
		{"full build error", "BUILD_TAIL_MARKER"},
		{"full output", "OUTPUT_TAIL_MARKER"},
		{"full thought summary", "THOUGHT_TAIL_MARKER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(client.user, tc.want) {
				t.Errorf("judge prompt omits %q", tc.want)
			}
		})
	}
	if strings.Contains(client.user, "... and ") {
		t.Error("judge prompt cuts the action list with an unrecoverable '... and N more'")
	}
}
