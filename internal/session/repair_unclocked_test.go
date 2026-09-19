package session

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// A repair episode has no clock of its own: how long a model thinks is not
// evidence about whether it is converging. Ladder run R1-4d (2026-09-19): the
// coverage round's model call ran 368 s -- this model thinks for minutes --
// and the episode's 6.2-minute clock cut it with nothing returned, so the
// round made no edit and the change went out unverified. The turn's own
// deadline, the user's constraint, is the only one a repair call runs under.
func TestRepairLoop_AModelCallRunsUnderTheTurnsDeadlineOnly(t *testing.T) {
	h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 1 })
	testPath := filepath.Join(h.ws, "main_test.go")
	initial := func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", testPath, repairTestBroken),
		}}
	}
	repairCalls := 0
	var callDeadline time.Time
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return initial(), nil
			},
		},
		CompleteWithToolResultsFunc: func(ctx context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			if last := history[len(history)-1]; strings.Contains(last.Text, "broken") {
				repairCalls++
				callDeadline, _ = ctx.Deadline()
				return &types.LLMToolResponse{Text: "fixed", ToolCalls: []types.ToolCall{
					h.writeCall("r1", "write_file", testPath, repairTestFixed),
				}}, nil
			}
			for _, m := range history {
				if len(m.ToolResults) > 0 {
					return &types.LLMToolResponse{Text: "done"}, nil
				}
			}
			return initial(), nil
		},
	}

	// A turn with a long deadline: an hours-long run is a normal one.
	turnDeadline := time.Now().Add(30 * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), turnDeadline)
	defer cancel()
	result, err := h.executor.ProcessWithIntent(ctx, "fix the probe test", presetIntentForTask("/fix", "fix the probe test", ""))
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if repairCalls == 0 {
		t.Fatal("the test gate never asked the model to repair")
	}
	if callDeadline.IsZero() || !callDeadline.Equal(turnDeadline) {
		t.Errorf("repair call deadline = %v, want the turn's own %v: the harness adds no clock of its own", callDeadline, turnDeadline)
	}
	if result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("TestCheck = %+v, want the repair re-verified", result.TestCheck)
	}
}

// The attempts bound the episode, not the time they take: every attempt the
// budget allows runs, however long the ones before it thought.
func TestRepairLoop_EveryAttemptRunsHoweverLongTheModelThinks(t *testing.T) {
	h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 3 })
	testPath := filepath.Join(h.ws, "main_test.go")
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", testPath, repairTestBroken),
		}}
	}
	repairs := 0
	h.onRepair = func(int, []types.Message) *types.LLMToolResponse {
		repairs++
		time.Sleep(200 * time.Millisecond) // a model that thinks
		content := fmt.Sprintf("package main\n\nimport \"testing\"\n\nfunc TestProbe(t *testing.T) {\n\tt.Fatal(\"still broken %d\")\n}\n", repairs)
		if repairs == 3 {
			content = repairTestFixed
		}
		return &types.LLMToolResponse{Text: "trying", ToolCalls: []types.ToolCall{
			h.writeCall(fmt.Sprintf("r%d", repairs), "write_file", testPath, content),
		}}
	}

	result, err := h.drive(t, "fix the probe test")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	rec := result.TestCheck.Repair
	if rec == nil || !rec.Passed || rec.Cost.Attempts != 3 {
		t.Fatalf("repair record = %+v, want convergence on the third attempt", rec)
	}
}
