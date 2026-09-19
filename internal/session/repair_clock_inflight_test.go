package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// The repair clock decides whether an attempt starts; it does not cut a model
// call already in flight. Ladder run R1-4d (2026-09-19): the coverage round's
// only model call ran 366 s -- this model thinks for minutes -- and the
// episode's 6.2-minute clock cut it with nothing returned, so the round made
// no attempt at all and the change went out unverified. The turn's own
// deadline, the user's constraint, still bounds the call.
func TestRepairLoop_AnAttemptInFlightOutlivesTheEpisodeClock(t *testing.T) {
	h := newRepairHarness(t, func(e *Executor) {
		e.config.RepairWallClock = 50 * time.Millisecond
		e.config.RepairMaxAttempts = 1
	})
	testPath := filepath.Join(h.ws, "main_test.go")
	initial := func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", testPath, repairTestBroken),
		}}
	}
	repairCalls, clockBoundCall := 0, false
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return initial(), nil
			},
		},
		CompleteWithToolResultsFunc: func(ctx context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			if last := history[len(history)-1]; strings.Contains(last.Text, "broken") {
				repairCalls++
				if dl, ok := ctx.Deadline(); ok && time.Until(dl) < 30*time.Second {
					clockBoundCall = true
				}
				// A model that thinks for longer than the whole episode clock,
				// and that stops when its context does, as a real client does.
				select {
				case <-time.After(6 * time.Second):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
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

	result, err := h.drive(t, "fix the probe test")
	if repairCalls == 0 {
		t.Fatal("the test gate never asked the model to repair")
	}
	if clockBoundCall {
		t.Error("the repair's model call ran under the episode clock's deadline; the clock decides whether an attempt starts, not when a call in flight is cut")
	}
	if err != nil {
		t.Fatalf("an attempt in flight when the clock ran out must finish and be judged: %v", err)
	}
	if result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("the attempt's fix must be re-verified: TestCheck = %+v", result.TestCheck)
	}
}

// The clock still bounds the episode: once it has run out, the round in flight
// finishes and no further round of the attempt starts. A model that reads
// past the clock gets its read answered and its attempt rechecked, not
// another call. (The loop cannot be driven there by timing alone: the
// episode clock includes the measured gate time.)
func TestRepairRound_TheClockStopsFurtherRounds(t *testing.T) {
	h := newRepairHarness(t, nil)
	testPath := filepath.Join(h.ws, "main_test.go")
	if err := os.WriteFile(testPath, []byte(repairTestBroken), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	trp := &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{},
		CompleteWithToolResultsFunc: func(context.Context, string, []types.Message, []types.ToolDefinition) (*types.LLMToolResponse, error) {
			calls++
			return &types.LLMToolResponse{Text: "reading", ToolCalls: []types.ToolCall{
				{ID: fmt.Sprintf("read%d", calls), Name: "read_file", Input: map[string]any{"path": testPath}},
			}}, nil
		},
	}
	// The episode clock ran out while the first round was in flight.
	clock, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	var history []types.Message
	last, llmCalls, _, _, results, wrote, err := h.executor.repairRound(
		context.Background(), clock, trp, "", &history, nil,
		h.executor.EffectiveAgentRuntimeConfig, &ExecutionResult{}, "FAIL: TestProbe", false)
	if err != nil {
		t.Fatalf("repairRound: %v", err)
	}
	if calls != 1 || llmCalls != 1 {
		t.Fatalf("model calls = %d (counted %d), want 1: no round starts after the clock has run out", calls, llmCalls)
	}
	if len(results) != 1 {
		t.Fatalf("tool results = %d, want the round's read answered", len(results))
	}
	if wrote || last == nil || last.Text != "reading" {
		t.Fatalf("wrote=%v last=%+v, want the read round returned for the recheck", wrote, last)
	}
}
