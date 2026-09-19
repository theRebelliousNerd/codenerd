package session

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// Adversarial journey tests: full Process turns driven by scripted hostile
// models. Unit tests pin each guard in isolation; these prove the wired loop
// terminates bounded and honest when the model never cooperates.
//
// Rules for this file: every journey asserts on state (tool counts, error
// identity, TurnOutcome, filesystem), never on prose. A journey that hangs is
// a failure, so each runs under a context timeout far above any legitimate
// turn length — hitting it means the loop did not bound itself.

// journeyScriptedModel returns a scriptedProvider that answers every tool-loop
// generation with next(round), where round counts generations from zero
// across both the initial generation and every tool-result follow-up.
//
// It must implement types.ToolResultsProvider: without
// CompleteWithToolResults the loop takes its one-batch graceful-degradation
// path and a journey scripted for persistence silently tests one round.
func journeyScriptedModel(next func(round int) *types.LLMToolResponse) *scriptedProvider {
	var rounds atomic.Int64
	answer := func() *types.LLMToolResponse {
		n := int(rounds.Add(1)) - 1
		return next(n)
	}
	return &scriptedProvider{
		MockLLMClient: &MockLLMClient{
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				if resp := answer(); resp != nil {
					return resp.Text, nil
				}
				return "", nil
			},
		},
		completeWithTools: func(ctx context.Context, sys, user string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return answer(), nil
		},
		completeWithToolResults: func(ctx context.Context, sys string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return answer(), nil
		},
	}
}

func journeyIntent(verb, category string) *MockTransducer {
	return &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: verb, Category: category}, nil
		},
	}
}

// journeyConfigFactory offers exactly the named tools. An empty allowlist
// means the model is never shown a tool, so a journey that forgets this
// tests nothing: the scripted ToolCalls are dropped before execution.
func journeyConfigFactory(toolNames ...string) *MockConfigFactory {
	return &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			return &config.EffectiveAgentRuntimeConfig{AllowedTools: toolNames}, nil
		},
	}
}

// journeyConfig is a production-shaped turn with all post-edit gates off:
// journeys about the loop must not depend on compilers, test runners, or
// critic models. There are no count ceilings to set — what bounds the turn is
// the working policy, which is exactly what these journeys exercise.
func journeyConfig(t *testing.T) ExecutorConfig {
	t.Helper()
	cfg := DefaultExecutorConfig()
	cfg.VerifyBuildAfterEdits = false
	cfg.VerifyTestsAfterEdits = false
	cfg.CriticReviewAfterEdits = false
	cfg.EnableSafetyGate = false
	cfg.ToolTimeout = time.Minute
	cfg.WorkspaceRoot = t.TempDir()
	return cfg
}

// TestJourney_InfiniteReaderEndsBoundedAndHonest is the read-only stall in its
// purest form: a /fix turn whose model reads the same file forever and never
// writes, never verifies, never stops. The turn must end because the kernel
// derived working_stop(/read_only_stall) over the rounds the loop asserted —
// until 2026-09-18 it ended at MaxToolCalls=40 instead — and the ending must
// not read as success.
func TestJourney_InfiniteReaderEndsBoundedAndHonest(t *testing.T) {
	const toolName = "journey_read_alpha"
	var calls atomic.Int64
	registerTestTool(t, &tools.Tool{
		Effect:      tools.EffectRead,
		Name:        toolName,
		Description: "Reads the same kilobyte forever",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Required:   []string{"path"},
			Properties: map[string]tools.Property{"path": {Type: "string"}},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			calls.Add(1)
			return strings.Repeat("x", 1024), nil
		},
	})

	var generations atomic.Int64
	model := journeyScriptedModel(func(round int) *types.LLMToolResponse {
		generations.Add(1)
		// A fresh call ID and a slightly different range every round, the
		// way the live stall re-read one region as 760-830, 700-850,
		// 768-815...: never an exact repeat, never progress.
		return &types.LLMToolResponse{
			Text: "Reading a little more before I start.",
			ToolCalls: []types.ToolCall{{
				ID:   "call_reader_" + strconv.Itoa(round),
				Name: toolName,
				Input: map[string]any{
					"path":       "same/file.go",
					"start_line": 700 + round*7,
					"end_line":   850 - round*3,
				},
			}},
		}
	})

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	executor := NewExecutor(kernel, &MockVirtualStore{}, model, &MockJITCompiler{}, journeyConfigFactory(toolName), journeyIntent("/fix", "/mutation"))
	executor.SetConfig(journeyConfig(t))
	executor.SetSessionID("journey-infinite-reader")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	start := time.Now()
	result, err := executor.Process(ctx, "fix the bug in same/file.go")
	elapsed := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("turn hit the 5-minute test guard with %d tool calls: the loop did not bound itself", calls.Load())
	}
	t.Logf("infinite reader ended after %v: err=%v outcome=%q tools=%d generations=%d",
		elapsed, err, resultOutcome(result), calls.Load(), generations.Load())
	if generations.Load() < 3 {
		t.Fatalf("generations = %d, want >= 3: the model gave up before the loop had anything to bound", generations.Load())
	}

	// working_stall_rounds(24) in internal/context/working_set.mg. One tool
	// call per round, so the stall span is the bound — and it is a derivation
	// over asserted facts, not a number in Go.
	const stallRounds = 24
	if got := calls.Load(); got == 0 {
		t.Fatal("the scripted reader never executed a tool: the journey tested nothing")
	} else if got > stallRounds {
		t.Fatalf("tool calls = %d, want <= %d (working_stall_rounds): the policy never stopped it", got, stallRounds)
	}
	if result != nil && result.ToolCallsExecuted > stallRounds {
		t.Fatalf("result ToolCallsExecuted = %d, want <= %d", result.ToolCallsExecuted, stallRounds)
	}
	if err != nil && !strings.Contains(err.Error(), "read_only_stall") {
		t.Fatalf("err = %v, want the derivation that ended the turn to be named", err)
	}
	// Honesty: a turn that read forever and wrote nothing must not succeed.
	if err == nil {
		if result == nil {
			t.Fatal("nil result with nil error: the turn vanished")
		}
		if result.TurnOutcome == types.MangleAtom("/done") {
			t.Fatalf("outcome is /done after zero writes: response=%q", result.Response)
		}
	}
}

func resultOutcome(r *ExecutionResult) string {
	if r == nil {
		return "<nil result>"
	}
	return string(r.TurnOutcome)
}

// TestJourney_ProseOnlyDoneOnChangeTaskFails is F-RUN-3 end to end: the model
// announces completion in prose and runs zero tools. The result string is
// written by the thing being audited, so it cannot be the evidence.
func TestJourney_ProseOnlyDoneOnChangeTaskFails(t *testing.T) {
	const toolName = "journey_read_prose"
	registerTestTool(t, &tools.Tool{
		Effect:      tools.EffectRead,
		Name:        toolName,
		Description: "Offered but never used",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Required:   []string{"path"},
			Properties: map[string]tools.Property{"path": {Type: "string"}},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "unused", nil
		},
	})
	model := journeyScriptedModel(func(round int) *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "Done. I fixed the bug and verified the fix."}
	})

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	executor := NewExecutor(kernel, &MockVirtualStore{}, model, &MockJITCompiler{}, journeyConfigFactory(toolName), journeyIntent("/fix", "/mutation"))
	executor.SetConfig(journeyConfig(t))
	executor.SetSessionID("journey-prose-done")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := executor.Process(ctx, "fix the bug in same/file.go")

	if err == nil {
		t.Fatalf("prose-only change turn succeeded: outcome=%q response=%q",
			resultOutcome(result), result.Response)
	}
	if !isHollowSuccessError(err) {
		t.Fatalf("expected hollowSuccessError, got %T: %v", err, err)
	}
}

// TestJourney_FabricatedTestOutputOnQueryTurnIsNotDone is the verb-agnostic
// test-claim rule driven through a whole turn: a read-only /explain whose
// model pastes runner output it never produced. Query turns never fail, so
// the verdict must land in TurnOutcome — and it must not be /done.
func TestJourney_FabricatedTestOutputOnQueryTurnIsNotDone(t *testing.T) {
	const toolName = "journey_read_beta"
	registerTestTool(t, &tools.Tool{
		Effect:      tools.EffectRead,
		Name:        toolName,
		Description: "Reads one file once",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Required:   []string{"path"},
			Properties: map[string]tools.Property{"path": {Type: "string"}},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "package alpha", nil
		},
	})

	model := journeyScriptedModel(func(round int) *types.LLMToolResponse {
		if round == 0 {
			return &types.LLMToolResponse{
				Text:      "Let me look first.",
				ToolCalls: []types.ToolCall{{ID: "call_beta", Name: toolName, Input: map[string]any{"path": "alpha.go"}}},
			}
		}
		return &types.LLMToolResponse{Text: "--- PASS: TestAlpha (0.00s)\nok  \t codenerd/internal/alpha\t0.1s"}
	})

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	executor := NewExecutor(kernel, &MockVirtualStore{}, model, &MockJITCompiler{}, journeyConfigFactory(toolName), journeyIntent("/explain", "/query"))
	executor.SetConfig(journeyConfig(t))
	executor.SetSessionID("journey-fabricated-pass")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := executor.Process(ctx, "explain alpha.go and show me its test results")
	if err != nil {
		t.Fatalf("query turn must not fail, got: %v", err)
	}
	if result == nil {
		t.Fatal("nil result with nil error")
	}
	t.Logf("fabricated output verdict: outcome=%q tools=%d writes=%d tests=%d response=%q",
		result.TurnOutcome, result.ToolCallsExecuted, result.SuccessfulWriteTools, result.TestRunCalls, result.Response)
	if result.TurnOutcome == types.MangleAtom("/done") || result.TurnOutcome == types.MangleAtom("/unverified") || result.TurnOutcome == "" {
		t.Fatalf("TurnOutcome = %q, want the kernel /hollow verdict for runner output with no test tool", result.TurnOutcome)
	}
}
