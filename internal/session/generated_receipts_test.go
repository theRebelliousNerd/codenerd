package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

type generatedReceiptReader interface {
	GeneratedToolReceipts() []types.GeneratedToolReceipt
}

func readGeneratedReceipts(t *testing.T, result *ExecutionResult) []types.GeneratedToolReceipt {
	t.Helper()
	reader, ok := any(result).(generatedReceiptReader)
	if !ok {
		t.Fatal("session result lost the generated bridge's typed receipt")
	}
	return reader.GeneratedToolReceipts()
}

func TestGeneratedReceiptsPreserveEveryReturnedOutcome(t *testing.T) {
	for _, mode := range []string{"success", "partial_error", "missing_feedback", "mismatched_request", "cancel_after_start", "refused_before_bridge"} {
		t.Run(mode, func(t *testing.T) {
			kernel := realKernel(t)
			root := t.TempDir()
			kernel.SetWorkspace(root)
			registry := core.NewToolRegistry(root)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if err := registry.RegisterTool("receipt_probe", executable, "/all"); err != nil {
				t.Fatal(err)
			}
			call := types.ToolCall{ID: "original-call", Name: "receipt_probe", Input: map[string]any{"path": filepath.Join(root, "effect.txt")}}
			canonical, err := types.CanonicalGeneratedArgs(call.Input)
			if err != nil {
				t.Fatal(err)
			}
			for _, fact := range []types.Fact{
				{Predicate: "permitted_action", Args: []any{"exec-" + call.ID, types.MangleAtom("/receipt_probe"), call.Input["path"], canonical, time.Now().Unix()}},
				{Predicate: "permission_check_result", Args: []any{"exec-" + call.ID, types.MangleAtom("/permit"), "exact unit admission", time.Now().Unix()}},
			} {
				if err := kernel.Assert(fact); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			var actual types.GeneratedToolReceipt
			store := &generatedSessionStore{run: func(request types.GeneratedToolRequest) (types.GeneratedToolReceipt, error) {
				calls++
				actual = types.GeneratedToolReceipt{Request: request, Attempted: true, ProcessStarted: true,
					Output: "decoded result", Stdout: "raw_receipt_only_marker", ValidationCompleted: true, ValidationPassed: true,
					Feedback: types.GeneratedLearningAck{ExecutionID: request.Key(), Fingerprint: request.Fingerprint(), Path: "unit fake acknowledgment", CommittedAt: time.Now(), Durable: true}}
				switch mode {
				case "partial_error":
					actual.BackendError, actual.PartialOutput = errors.New("partial backend failure"), true
				case "missing_feedback":
					actual.Feedback = types.GeneratedLearningAck{}
				case "mismatched_request":
					actual.Request.CallID = "not-the-admitted-call"
				case "cancel_after_start":
					cancel()
					actual.BackendError = context.Canceled
				}
				return actual, actual.Err()
			}}
			executor := NewExecutor(kernel, store, nil, nil, nil, nil)
			executor.SetConfig(ExecutorConfig{WorkspaceRoot: root})
			executor.SetModularTools(tools.NewRegistry())
			executor.SetOuroborosRegistry(registry)
			t.Cleanup(executor.cleanupTurnFacts)
			cfg := validCapabilityTestConfig(call.Name)
			if mode == "refused_before_bridge" {
				cfg.AllowedTools = nil
			}
			result := &ExecutionResult{}
			results, errs := executor.executeToolBatch(ctx, []types.ToolCall{call}, cfg, result)
			if result.WriteToolEffects != 0 || len(result.WrittenPaths) != 0 || len(result.ToolReceipts) != 0 {
				t.Fatal("retained generated evidence fabricated recognized-write accounting")
			}
			if len(results) != 1 || (mode == "success") != (len(errs) == 0 && !results[0].IsError) {
				t.Fatalf("existing execution/error semantics changed: results=%+v errs=%v", results, errs)
			}
			receipts := readGeneratedReceipts(t, result)
			if mode == "refused_before_bridge" {
				if calls != 0 || len(receipts) != 0 {
					t.Fatal("pre-bridge refusal dispatched or invented a receipt")
				}
				return
			}
			if calls != 1 || len(receipts) != 1 || receipts[0] != actual {
				t.Fatalf("actual returned receipt was lost, normalized or replayed: calls=%d retained=%+v actual=%+v", calls, receipts, actual)
			}
			receipts[0].Output = "caller-local mutation"
			if readGeneratedReceipts(t, result)[0] != actual {
				t.Fatal("caller changed the owned receipt slice")
			}
			encoded, err := json.Marshal(result)
			if err != nil || strings.Contains(string(encoded), actual.Stdout) {
				t.Fatalf("raw receipt payload entered automatic result serialization: %v", err)
			}
		})
	}
}

func TestGeneratedReceiptsDoNotInventOtherExecution(t *testing.T) {
	kernel := realKernel(t)
	root := t.TempDir()
	kernel.SetWorkspace(root)
	registry := tools.NewRegistry()
	if err := registry.Register(&tools.Tool{Name: "receipt_read", Effect: tools.EffectRead, Execute: func(context.Context, map[string]any) (string, error) { return "read", nil }}); err != nil {
		t.Fatal(err)
	}
	call := types.ToolCall{ID: "modular", Name: "receipt_read", Input: map[string]any{"path": filepath.Join(root, "doc.txt")}}
	canonical, err := types.CanonicalGeneratedArgs(call.Input)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []types.Fact{
		{Predicate: "permitted_action", Args: []any{"exec-" + call.ID, types.MangleAtom("/receipt_read"), call.Input["path"], canonical, time.Now().Unix()}},
		{Predicate: "permission_check_result", Args: []any{"exec-" + call.ID, types.MangleAtom("/permit"), "exact read admission", time.Now().Unix()}},
	} {
		if err := kernel.Assert(fact); err != nil {
			t.Fatal(err)
		}
	}
	executor := NewExecutor(kernel, &MockVirtualStore{}, nil, nil, nil, nil)
	cfg := DefaultExecutorConfig()
	cfg.WorkspaceRoot = root
	executor.SetConfig(cfg)
	executor.SetModularTools(registry)
	t.Cleanup(executor.cleanupTurnFacts)
	result := &ExecutionResult{}
	if _, errs := executor.executeToolBatch(context.Background(), []types.ToolCall{call}, validCapabilityTestConfig("receipt_read"), result); len(errs) != 0 {
		t.Fatal(errs)
	}
	if result.SuccessfulToolCalls != 1 || len(readGeneratedReceipts(t, result)) != 0 {
		t.Fatal("ordinary modular execution changed or invented generated evidence")
	}
	var absent *ExecutionResult
	if got := readGeneratedReceipts(t, absent); len(got) != 0 {
		t.Fatal("nil result invented generated evidence")
	}
}
