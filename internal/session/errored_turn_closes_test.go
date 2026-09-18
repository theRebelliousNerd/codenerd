package session

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// REVIEW-wave1 F5 and open item 7. checkHollowSuccess is where the turn closes:
// it asserts the turn's evidence, reads the kernel's verdict once
// (captureTurnOutcome) and, via defer, retracts every per-turn fact. Until
// 2026-09-18 the call sat behind `if result.Error == nil`, so a turn whose
// tool error survived to the end (surfaceToolErrors) never reached it: the
// comment on captureTurnOutcome said it "runs on EVERY path", and on that path
// it did not run at all — TurnOutcome was left empty for resolveTurnOutcome to
// classify from the error — and the facts recordGoFileCreations asserted from
// inside the tool loop (created_source, tracked in perTurnCreatedSourceFacts)
// were never retracted, so a file created in an errored turn could raise "new
// source was created without a test file" against a later turn forever.
//
// The turn now closes on every path. The error still outranks the verdict:
// the outcome is /failed, and no hollow-success reason overwrites the real
// error.
func TestErroredTurnStillClosesAndRetractsItsFacts(t *testing.T) {
	var executor *Executor
	leaked := types.Fact{Predicate: "created_source", Args: []any{"internal/x/new.go"}}

	tool := &tools.Tool{
		Effect:      tools.EffectRead,
		Name:        "readFile",
		Description: "Reads a file",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Required:   []string{"path"},
			Properties: map[string]tools.Property{"path": {Type: "string"}},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			// Stand in for recordGoFileCreations, which asserts from inside
			// the tool loop and tracks the fact for the turn's cleanup.
			executor.mu.Lock()
			executor.perTurnCreatedSourceFacts = append(executor.perTurnCreatedSourceFacts, leaked)
			executor.mu.Unlock()
			return "", errors.New("disk on fire")
		},
	}
	registerTestTool(t, tool)

	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			// An empty answer after a failed tool: nothing recovered it, so
			// surfaceToolErrors leaves the error on the result.
			return &types.LLMToolResponse{
				Text: "",
				ToolCalls: []types.ToolCall{{
					ID:    "call_1",
					Name:  "readFile",
					Input: map[string]any{"path": "/test/file.txt"},
				}},
			}, nil
		},
	}
	mockKernel := &MockKernel{}
	mockKernel.Assert(types.Fact{
		Predicate: "permitted",
		Args:      []any{MangleAtom("/readFile"), "/test/file.txt", `{"path":"/test/file.txt"}`},
	})
	mockConfig := &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			return &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"readFile"}}, nil
		},
	}
	executor = NewExecutor(mockKernel, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, mockConfig, &MockTransducer{})
	executor.config.EnableSafetyGate = true
	executor.config.WorkspaceRoot = t.TempDir()

	result, _ := executor.Process(context.Background(), "Read /test/file.txt")
	if result == nil {
		t.Fatal("Process returned no result")
	}
	if result.Error == nil {
		t.Fatalf("precondition: the tool error must survive the turn (empty answer, no gate), got no error")
	}

	if result.TurnOutcome == "" {
		t.Errorf("TurnOutcome is empty after an errored turn: the verdict was never captured, so " +
			"the turn was classified by resolveTurnOutcome instead of read from the kernel")
	} else if result.TurnOutcome != types.MangleAtom("/failed") {
		t.Errorf("TurnOutcome = %v, want /failed: the error outranks any verdict", result.TurnOutcome)
	}

	executor.mu.Lock()
	remaining := len(executor.perTurnCreatedSourceFacts)
	executor.mu.Unlock()
	if remaining != 0 {
		t.Errorf("%d per-turn created_source fact(s) survived an errored turn; they would raise "+
			"\"new source was created without a test file\" against every later turn", remaining)
	}
}
