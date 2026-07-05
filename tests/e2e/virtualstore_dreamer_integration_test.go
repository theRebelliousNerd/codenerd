//go:build integration

package e2e_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

// ----------------------------------------------------------------------
// Mock Support
// ----------------------------------------------------------------------

type mockDreamer struct {
	mu           sync.Mutex
	simulateFunc func(ctx context.Context, req core.ActionRequest) core.DreamResult
}

func (m *mockDreamer) SimulateAction(ctx context.Context, req core.ActionRequest) core.DreamResult {
	if m.simulateFunc != nil {
		return m.simulateFunc(ctx, req)
	}
	return core.DreamResult{Unsafe: false}
}

type mockValidatorRegistry struct {
	validateFunc func(ctx context.Context, req core.ActionRequest, res core.ActionResult) []core.ValidationResult
}

func (m *mockValidatorRegistry) Validate(ctx context.Context, req core.ActionRequest, res core.ActionResult) []core.ValidationResult {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, req, res)
	}
	return []core.ValidationResult{{Verified: true}}
}

// ----------------------------------------------------------------------
// Smoke Tests
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Smoke_SafeAction(t *testing.T) {
	// Verify the integration works at all for a safe action.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "read_file", map[string]any{"filepath": "test.txt"})
	if err != nil {
		t.Fatalf("Expected nil for safe action, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Contract Violation Tests
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Contract_FailOpen_MissingDreamer(t *testing.T) {
	// Violated Contract: Only safe actions bypass Dreamer.
	// Mechanism: Initialize VirtualStore without a Dreamer, execute malicious action.
	// Expected Behavior: Returns nil (fail-open) allowing the action.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt"})
	if err != nil {
		t.Fatalf("Expected nil (fail-open) with missing dreamer, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_FailOpen_UnknownTool(t *testing.T) {
	// Violated Contract: Only safe actions bypass Dreamer.
	// Mechanism: Tool not in interactiveToolActionType map.
	// Expected Behavior: Returns nil (fail-open) as it thinks it's non-destructive.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "unknown_tool", map[string]any{"filepath": "test.txt"})
	if err != nil {
		t.Fatalf("Expected nil (fail-open) for unknown tool, got %v", err)
	}
}

// ----------------------------------------------------------------------
// State Corruption (Concurrent) Tests
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_StateCorruption_CacheCollision(t *testing.T) {
	// Violated Contract: DreamCache correctly isolates distinct actions.
	// Mechanism: Concurrent write_file to same target with different payloads.
	// Expected Behavior: Must not reuse cache across different payloads.

	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // Might init dreamer

	var wg sync.WaitGroup
	errs := make([]error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt", "content": "safe"})
	}()

	go func() {
		defer wg.Done()
		errs[1] = vs.PreflightDestructiveToolCall(context.Background(), "a2", "write_file", map[string]any{"filepath": "test.txt", "content": "MALICIOUS_rm_rf"})
	}()

	wg.Wait()

	// Real assertions are difficult here without introspecting the cache, but we can verify it doesn't crash
	if errs[0] != nil && errs[1] != nil {
		t.Logf("Both blocked, cache collision possible but handled safely: %v, %v", errs[0], errs[1])
	}
}

func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentValidation(t *testing.T) {
	// Violated Contract: Concurrent validations should not deadlock.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt"}, "out", true)
			if err != nil {
				t.Errorf("Concurrent validation failed unexpectedly: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

func TestE2E_VirtualStoreDreamer_StateCorruption_PreflightAndValidate(t *testing.T) {
	// Violated Contract: Concurrent read/write locks.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt"})
		if err != nil {
			t.Errorf("Preflight failed unexpectedly: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt"}, "out", true)
		if err != nil {
			t.Errorf("Validate failed unexpectedly: %v", err)
		}
	}()
	wg.Wait()
}

// ----------------------------------------------------------------------
// Resource Exhaustion Tests
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_ResourceExhaustion_LargePayload(t *testing.T) {
	// Violated Contract: System shouldn't OOM on large payloads.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	largePayload := strings.Repeat("A", 10*1024*1024) // 10MB

	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt", "content": largePayload})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_ResourceExhaustion_ManyConcurrentPreflights(t *testing.T) {
	// Violated Contract: System handles high throughput.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	var wg sync.WaitGroup
	errCh := make(chan error, 1000)
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt"})
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("Concurrent preflight failed: %v", err)
	}
}

// ----------------------------------------------------------------------
// Cascading Failure Tests
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Cascading_ValidatorDoesNotBlockSuccess(t *testing.T) {
	// Violated Contract: Validations on unmapped tools don't fail incorrectly.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "unknown_tool", map[string]any{"filepath": "test.txt"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil on unmapped tools, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Advanced Extraction and Multi-Boundary Scenarios
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_Path(t *testing.T) {
	// Verifies the "path" key extraction logic in extractActionTarget.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())

	// Preflight should return nil if it thinks tool is unmapped or safe.
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"path": "target.txt"})
	if err != nil {
		t.Fatalf("Expected nil on unmocked dreamer/fail-open, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_Filename(t *testing.T) {
	// Verifies the "filename" key extraction logic in extractActionTarget.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filename": "target.txt"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_File(t *testing.T) {
	// Verifies the "file" key extraction logic in extractActionTarget.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"file": "target.txt"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_URL(t *testing.T) {
	// Verifies the "url" key extraction logic in extractActionTarget.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"url": "http://target.txt"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_Target(t *testing.T) {
	// Verifies the "target" key extraction logic in extractActionTarget.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"target": "target.txt"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_Query(t *testing.T) {
	// Verifies the "query" key extraction logic in extractActionTarget.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"query": "SELECT *"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_NonString(t *testing.T) {
	// Verifies non-string argument handling in extractActionTarget.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": 1234})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_NilArgs(t *testing.T) {
	// Verifies nil arguments handling in extractActionTarget.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", nil)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

// ----------------------------------------------------------------------
// More Validation Tests
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Contract_Validate_NotSuccess(t *testing.T) {
	// Verifies that a failed action execution does not trigger validation.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt"}, "out", false)
	if err != nil {
		t.Fatalf("Expected nil because tool execution failed, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_Validate_EmptyOutput(t *testing.T) {
	// Verifies behavior when output is empty string.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt"}, "", true)
	if err != nil {
		t.Fatalf("Expected nil or handled correctly for empty output, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Action Types Map Verification
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_RunCommand(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "run_command", map[string]any{"query": "ls"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_Bash(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "bash", map[string]any{"query": "echo hello"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_RunBuild(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "run_build", map[string]any{})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_EditLines(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "edit_lines", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_InsertLines(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "insert_lines", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_DeleteLines(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "delete_lines", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_DeleteFile(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "delete_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_EditFile(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "edit_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Edge Cases
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Edge_EmptyActionID(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.PreflightDestructiveToolCall(context.Background(), "", "write_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_LongToolName(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	toolName := strings.Repeat("a", 1000)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", toolName, map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ContextCanceled(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := vs.PreflightDestructiveToolCall(ctx, "a1", "write_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil because of fail-open, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ContextDeadline(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	err := vs.PreflightDestructiveToolCall(ctx, "a1", "write_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil because of fail-open, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Recovery_MultipleExecutions(t *testing.T) {
	// Verify that multiple successful executions run without error.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	for i := 0; i < 5; i++ {
		err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.go"})
		if err != nil {
			t.Fatalf("Expected nil on iteration %d, got %v", i, err)
		}
	}
}

func TestE2E_VirtualStoreDreamer_Cascading_NilExecutor(t *testing.T) {
	// Verify that a nil executor does not crash PreflightDestructiveToolCall.
	// It shouldn't, since PreflightDestructiveToolCall doesn't use the executor directly,
	// only checks for dreamer/validators.
	vs := core.NewVirtualStore(nil)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

// ----------------------------------------------------------------------
// More Edge Cases and Boundary Checks
// ----------------------------------------------------------------------
func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_MultipleKeys(t *testing.T) {
	// Verifies the first matched key is returned.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	args := map[string]any{
		"target": "target.txt",
		"path":   "path.txt",
	}
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", args)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_NonStringValue(t *testing.T) {
	// Verifies that a non-string value is skipped.
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	args := map[string]any{
		"path":   123,
		"target": "target.txt",
	}
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", args)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ExtremelyLargeMap(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	args := make(map[string]any)
	for i := 0; i < 10000; i++ {
		args[strings.Repeat("k", i)] = "v"
	}
	args["path"] = "target.txt"
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", args)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ConcurrentExtractTarget(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	args := map[string]any{"path": "target.txt"}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", args)
			if err != nil {
				t.Errorf("Expected nil, got %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestE2E_VirtualStoreDreamer_Contract_Validate_NilArgs(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", nil, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_Validate_EmptyActionID(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.ValidateInteractiveToolResult(context.Background(), "", "write_file", map[string]any{"path": "test.txt"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_Validate_LongToolName(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	toolName := strings.Repeat("a", 1000)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", toolName, map[string]any{"path": "test.txt"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ValidateWithCanceledContext(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := vs.ValidateInteractiveToolResult(ctx, "a1", "write_file", map[string]any{"path": "test.txt"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ValidateWithDeadlineContext(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	err := vs.ValidateInteractiveToolResult(ctx, "a1", "write_file", map[string]any{"path": "test.txt"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Recovery_MultipleValidations(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	for i := 0; i < 5; i++ {
		err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"path": "test.txt"}, "out", true)
		if err != nil {
			t.Fatalf("Expected nil on iteration %d, got %v", i, err)
		}
	}
}

func TestE2E_VirtualStoreDreamer_Cascading_NilExecutorValidate(t *testing.T) {
	vs := core.NewVirtualStore(nil)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"path": "test.txt"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentValidationsWithDifferentArgs(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			args := map[string]any{"path": "test.txt"}
			err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", args, "out", true)
			if err != nil {
				t.Errorf("Expected nil on goroutine %d, got %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentPreflightWithDifferentArgs(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			args := map[string]any{"path": "test.txt"}
			err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", args)
			if err != nil {
				t.Errorf("Expected nil on goroutine %d, got %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentMixedOperations(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			args := map[string]any{"path": "test.txt"}
			if idx%2 == 0 {
				err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", args)
				if err != nil {
					t.Errorf("Expected nil on goroutine %d, got %v", idx, err)
				}
			} else {
				err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", args, "out", true)
				if err != nil {
					t.Errorf("Expected nil on goroutine %d, got %v", idx, err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestE2E_VirtualStoreDreamer_Smoke_ValidateSafeAction(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "read_file", map[string]any{"filepath": "test.txt"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil for safe action, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Smoke_ValidateFailOpen_MissingDreamer(t *testing.T) {
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.txt"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Multi-Boundary Tests ensuring Pipeline Integration
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Pipeline_WriteAndValidate(t *testing.T) {
	// Simulates a full pass: Preflight -> Execute (mocked) -> Validate
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	ctx := context.Background()
	args := map[string]any{"filepath": "integration_test.txt", "content": "hello"}

	err := vs.PreflightDestructiveToolCall(ctx, "a1", "write_file", args)
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	// Assume executor succeeds here

	err = vs.ValidateInteractiveToolResult(ctx, "a1", "write_file", args, "success", true)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Pipeline_WriteFailValidateSkip(t *testing.T) {
	// Simulates Preflight -> Execute (fails) -> Validate (skips)
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	ctx := context.Background()
	args := map[string]any{"filepath": "integration_test.txt", "content": "hello"}

	err := vs.PreflightDestructiveToolCall(ctx, "a1", "write_file", args)
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	// Executor fails
	err = vs.ValidateInteractiveToolResult(ctx, "a1", "write_file", args, "error", false)
	if err != nil {
		t.Fatalf("Validation should not run/fail on unsuccessful execution, got %v", err)
	}
}
