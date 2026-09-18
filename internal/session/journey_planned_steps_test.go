package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// End-to-end planned-steps and error-recovery journeys. The step executive
// (parse, per-step passes, ledger) is unit-covered at runToolLoop level;
// these drive whole Process turns and assert on the filesystem and the
// ledger, which is what a user can check.

// TestJourney_PlannedSteps_WritesBothFilesAndLedgers gives the executive a
// two-file change and a provider that plays each step's write from the step
// anchor. Both files must land byte-exact, and the turn's report must name
// both as edited: a ledger that omits a file is the step being lost.
func TestJourney_PlannedSteps_WritesBothFilesAndLedgers(t *testing.T) {
	root := t.TempDir()
	// create_file is the shared test-double name for a write mutation
	// (projectdoc.IsWriteMutationTool); re-registered per test with cleanup,
	// the same way registerStepTools and the gating tests do it. Step
	// planning only engages when the catalog holds a write mutation.
	const writeTool = "create_file"
	var writes atomic.Int64
	registerTestTool(t, &tools.Tool{
		Effect:      tools.EffectWrite,
		Name:        writeTool,
		Description: "Writes content to path under the journey workspace",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Required:   []string{"path", "content"},
			Properties: map[string]tools.Property{"path": {Type: "string"}, "content": {Type: "string"}},
		},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			rel, _ := args["path"].(string)
			content, _ := args["content"].(string)
			writes.Add(1)
			if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
				return "", err
			}
			return "written " + rel, nil
		},
	})

	plan := "STEP a.txt :: create it with the greeting\nSTEP b.txt :: create it with the farewell\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "j-w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "hello"}}},
		"b.txt": {{ID: "j-w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	executor := NewExecutor(kernel, &testExecutiveStore{}, client, &MockJITCompiler{}, journeyConfigFactory(writeTool), journeyIntent("/create", "/mutation"))
	cfg := journeyConfig(t)
	// Step planning requires a working loop, which a workspace root is now the
	// whole requirement for.
	cfg.WorkspaceRoot = root
	executor.SetConfig(cfg)
	executor.workingWorld = &MockKernel{}
	executor.SetSessionID("journey-planned-steps")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, err := executor.Process(ctx, "create a.txt with the greeting and b.txt with the farewell")
	if err != nil {
		t.Fatalf("planned-steps turn failed: %v", err)
	}
	if result == nil {
		t.Fatal("nil result with nil error")
	}
	if writes.Load() != 2 {
		t.Fatalf("writes = %d, want 2 (one per step)", writes.Load())
	}
	for file, want := range map[string]string{"a.txt": "hello", "b.txt": "bye"} {
		got, readErr := os.ReadFile(filepath.Join(root, file))
		if readErr != nil {
			t.Fatalf("read %s: %v", file, readErr)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", file, got, want)
		}
		if !strings.Contains(result.Response, file) {
			t.Errorf("step ledger omits %s: response=%q", file, result.Response)
		}
	}
	// The gates are off, so no acceptance contract exists: /unverified is
	// the honest verdict for writes nothing verified. /done would claim a
	// verification that never ran.
	if result.TurnOutcome != types.MangleAtom("/unverified") {
		t.Errorf("TurnOutcome = %q, want /unverified for unverified writes", result.TurnOutcome)
	}
	if !strings.Contains(result.Response, "edited: 2") {
		t.Errorf("ledger does not report both edits: response=%q", result.Response)
	}
	t.Logf("planned steps: outcome=%q response=%q", result.TurnOutcome, result.Response)
}

// TestJourney_ToolErrorThenRecoveryFailsHonestThenSucceeds plays the most
// ordinary failure an agent hits: a read of a file that is not there. The
// tool answers with the directory listing and the model recovers on the next
// round. The journey proves the error reached the model (it changed its
// call) and the turn's counts tell the truth about the failure.
func TestJourney_ToolErrorThenRecoveryFailsHonestThenSucceeds(t *testing.T) {
	const toolName = "journey_read_gamma"
	registerTestTool(t, &tools.Tool{
		Effect:      tools.EffectRead,
		Name:        toolName,
		Description: "Reads a.go; anything else fails with a listing",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Required:   []string{"path"},
			Properties: map[string]tools.Property{"path": {Type: "string"}},
		},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path != "a.go" {
				return "", fmt.Errorf("file not found: %s. That directory contains: a.go (pick one of those, or use glob to search elsewhere -- do not guess another filename)", path)
			}
			return "package alpha", nil
		},
	})

	model := journeyScriptedModel(func(round int) *types.LLMToolResponse {
		switch round {
		case 0:
			return &types.LLMToolResponse{
				Text:      "Reading the file.",
				ToolCalls: []types.ToolCall{{ID: "j-g-0", Name: toolName, Input: map[string]any{"path": "missing.go"}}},
			}
		case 1:
			return &types.LLMToolResponse{
				Text:      "It listed a.go, reading that instead.",
				ToolCalls: []types.ToolCall{{ID: "j-g-1", Name: toolName, Input: map[string]any{"path": "a.go"}}},
			}
		default:
			return &types.LLMToolResponse{Text: "a.go declares package alpha."}
		}
	})

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	executor := NewExecutor(kernel, &testExecutiveStore{}, model, &MockJITCompiler{}, journeyConfigFactory(toolName), journeyIntent("/explain", "/query"))
	executor.SetConfig(journeyConfig(t))
	executor.SetSessionID("journey-tool-error-recovery")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := executor.Process(ctx, "explain missing.go")
	if err != nil {
		t.Fatalf("query turn must not fail, got: %v", err)
	}
	if result == nil {
		t.Fatal("nil result with nil error")
	}
	t.Logf("recovery: tools=%d ok=%d outcome=%q response=%q",
		result.ToolCallsExecuted, result.SuccessfulToolCalls, result.TurnOutcome, result.Response)
	if result.ToolCallsExecuted != 2 {
		t.Fatalf("ToolCallsExecuted = %d, want 2 (failed read + recovery read)", result.ToolCallsExecuted)
	}
	if result.SuccessfulToolCalls != 1 {
		t.Fatalf("SuccessfulToolCalls = %d, want 1: the failed read must stay failed in the counts", result.SuccessfulToolCalls)
	}
	if !strings.Contains(result.Response, "alpha") {
		t.Fatalf("response does not reflect the recovered read: %q", result.Response)
	}
}
