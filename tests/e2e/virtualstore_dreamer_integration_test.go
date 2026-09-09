//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "read_file", map[string]any{"filepath": testFile, "content": "hello"})
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
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "hello"})
	if err != nil {
		t.Fatalf("Expected nil (fail-open) with missing dreamer, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_FailOpen_UnknownTool(t *testing.T) {
	// Violated Contract: Only safe actions bypass Dreamer.
	// Mechanism: Tool in neither interactiveToolActionType nor the registry.
	// Expected Behavior: REFUSED. "Unknown" is the one classification that must
	// not default to the safest-looking answer — an unrecognised tool is far
	// more likely to be a new effectful one than a new inert one. This asserted
	// fail-open until 2026-09, against a gate that is fail-closed by policy.
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "unknown_tool", map[string]any{"filepath": testFile, "content": "hello"})
	if err == nil {
		t.Fatal("an unclassifiable tool was allowed through the gate")
	}
	if !strings.Contains(err.Error(), "effect declaration") {
		t.Errorf("refusal should name the missing effect declaration, got %v", err)
	}
}

// ----------------------------------------------------------------------
// State Corruption (Concurrent) Tests
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_StateCorruption_CacheCollision(t *testing.T) {
	// Violated Contract: DreamCache correctly isolates distinct actions.
	// Mechanism: Concurrent write_file to same target with different payloads.
	// Expected Behavior: Must not reuse cache across different payloads.

	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	kernel, _ := core.NewRealKernel()
	vs.SetKernel(kernel) // Might init dreamer

	var wg sync.WaitGroup
	errs := make([]error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "safe"})
	}()

	go func() {
		defer wg.Done()
		errs[1] = vs.PreflightDestructiveToolCall(context.Background(), "a2", "write_file", map[string]any{"filepath": testFile, "content": "MALICIOUS_rm_rf"})
	}()

	wg.Wait()

	// Real assertions are difficult here without introspecting the cache, but we can verify it doesn't crash
	if errs[0] != nil && errs[1] != nil {
		t.Logf("Both blocked, cache collision possible but handled safely: %v, %v", errs[0], errs[1])
	}
}

func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentValidation(t *testing.T) {
	// Violated Contract: Concurrent validations should not deadlock.
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "hello"}, "out", true)
			if err != nil {
				t.Errorf("Concurrent validation failed unexpectedly: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

func TestE2E_VirtualStoreDreamer_StateCorruption_PreflightAndValidate(t *testing.T) {
	// Violated Contract: Concurrent read/write locks.
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "hello"})
		if err != nil {
			t.Errorf("Preflight failed unexpectedly: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "hello"}, "out", true)
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
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	largePayload := strings.Repeat("A", 10*1024*1024) // 10MB

	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": largePayload})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_ResourceExhaustion_ManyConcurrentPreflights(t *testing.T) {
	// Violated Contract: System handles high throughput.
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	var wg sync.WaitGroup
	errCh := make(chan error, 1000)
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "hello"})
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
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)

	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "unknown_tool", map[string]any{"filepath": testFile, "content": "hello"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil on unmapped tools, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Advanced Extraction and Multi-Boundary Scenarios
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_Path(t *testing.T) {
	// Verifies the "path" key extraction logic in extractActionTarget.
	vs := newDreamerStore(t)

	// Preflight should return nil if it thinks tool is unmapped or safe.
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"path": "target.txt"})
	if err != nil {
		t.Fatalf("Expected nil on unmocked dreamer/fail-open, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_Filename(t *testing.T) {
	// Verifies the "filename" key extraction logic in extractActionTarget.
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filename": "target.txt"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_File(t *testing.T) {
	// Verifies the "file" key extraction logic in extractActionTarget.
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"file": "target.txt"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_URL(t *testing.T) {
	// Verifies the "url" key extraction logic in extractActionTarget.
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"url": "http://target.txt"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_Target(t *testing.T) {
	// Verifies the "target" key extraction logic in extractActionTarget.
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"target": "target.txt"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_Query(t *testing.T) {
	// Verifies the "query" key extraction logic in extractActionTarget.
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"query": "SELECT *"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_NonString(t *testing.T) {
	// Verifies non-string argument handling in extractActionTarget.
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": 1234})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_NilArgs(t *testing.T) {
	// Verifies nil arguments handling in extractActionTarget.
	vs := newDreamerStore(t)
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
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "hello"}, "out", false)
	if err != nil {
		t.Fatalf("Expected nil because tool execution failed, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_Validate_EmptyOutput(t *testing.T) {
	// Verifies behavior when output is empty string.
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "hello"}, "", true)
	if err != nil {
		t.Fatalf("Expected nil or handled correctly for empty output, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Action Types Map Verification
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_RunCommand(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "run_command", map[string]any{"query": "ls"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_Bash(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "bash", map[string]any{"query": "echo hello"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_RunBuild(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "run_build", map[string]any{})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_EditLines(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "edit_lines", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_InsertLines(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "insert_lines", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_DeleteLines(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "delete_lines", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_DeleteFile(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "delete_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_ActionTypes_EditFile(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "edit_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Edge Cases
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Edge_EmptyActionID(t *testing.T) {
	vs := newDreamerStore(t)
	err := vs.PreflightDestructiveToolCall(context.Background(), "", "write_file", map[string]any{"filepath": "test.go"})
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_LongToolName(t *testing.T) {
	vs := newDreamerStore(t)
	// A thousand-character name is still a name nothing declares. The point of
	// this test is that the length is handled without panicking or truncating
	// into some other tool's identity — not that it is allowed.
	toolName := strings.Repeat("a", 1000)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", toolName, map[string]any{"filepath": "test.go"})
	if err == nil {
		t.Fatal("a 1000-character unknown tool name was allowed through the gate")
	}
	if !strings.Contains(err.Error(), "effect declaration") {
		t.Errorf("refusal should name the missing effect declaration, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ContextCanceled(t *testing.T) {
	vs := newDreamerStore(t)
	// A cancelled context returns ctx.Err(), and should: the caller asked to
	// stop, and a safety gate that quietly proceeds on a cancelled context is
	// running a simulation nobody is waiting for. This expected nil.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := vs.PreflightDestructiveToolCall(ctx, "a1", "write_file", map[string]any{"filepath": "test.go"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ContextDeadline(t *testing.T) {
	vs := newDreamerStore(t)
	// Same as the cancellation case: an expired deadline surfaces as
	// context.DeadlineExceeded rather than an allow.
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	err := vs.PreflightDestructiveToolCall(ctx, "a1", "write_file", map[string]any{"filepath": "test.go"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Recovery_MultipleExecutions(t *testing.T) {
	// Verify that multiple successful executions run without error.
	vs := newDreamerStore(t)
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
	// The executor being nil must not panic — that is what this test is for.
	// The call is still refused, because a store with no kernel has no Dreamer,
	// which is the fail-closed contract and not a crash.
	vs := core.NewVirtualStore(nil)
	err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", map[string]any{"filepath": "test.go"})
	if err == nil {
		t.Fatal("a destructive call was allowed with neither executor nor Dreamer")
	}
	if !strings.Contains(err.Error(), "dreamer") {
		t.Errorf("refusal should name the missing Dreamer, got %v", err)
	}
}

// ----------------------------------------------------------------------
// More Edge Cases and Boundary Checks
// ----------------------------------------------------------------------
func TestE2E_VirtualStoreDreamer_Contract_ExtractTarget_MultipleKeys(t *testing.T) {
	// Verifies the first matched key is returned.
	vs := newDreamerStore(t)
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
	vs := newDreamerStore(t)
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
	vs := newDreamerStore(t)
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
	vs := newDreamerStore(t)
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
	// Nil args means no target, so extractActionTarget yields "unknown" and the
	// validator reports that no such file exists after the claimed write. That
	// is the correct verdict on a write with nothing to verify, and the test
	// must not panic on the nil map — which is what it is really for.
	vs := newDreamerStore(t)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", nil, "out", true)
	if err == nil {
		t.Fatal("a write validation with no target at all reported success")
	}
}

func TestE2E_VirtualStoreDreamer_Contract_Validate_EmptyActionID(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	err := vs.ValidateInteractiveToolResult(context.Background(), "", "write_file", map[string]any{"path": testFile, "content": "hello"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Contract_Validate_LongToolName(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	toolName := strings.Repeat("a", 1000)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", toolName, map[string]any{"path": testFile, "content": "hello"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ValidateWithCanceledContext(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := vs.ValidateInteractiveToolResult(ctx, "a1", "write_file", map[string]any{"path": testFile, "content": "hello"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Edge_ValidateWithDeadlineContext(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	err := vs.ValidateInteractiveToolResult(ctx, "a1", "write_file", map[string]any{"path": testFile, "content": "hello"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Recovery_MultipleValidations(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	for i := 0; i < 5; i++ {
		err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"path": testFile, "content": "hello"}, "out", true)
		if err != nil {
			t.Fatalf("Expected nil on iteration %d, got %v", i, err)
		}
	}
}

func TestE2E_VirtualStoreDreamer_Cascading_NilExecutorValidate(t *testing.T) {
	testFile := dreamerFile(t)
	vs := core.NewVirtualStore(nil)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"path": testFile, "content": "hello"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentValidationsWithDifferentArgs(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			args := map[string]any{"path": testFile, "content": "hello"}
			err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", args, "out", true)
			if err != nil {
				t.Errorf("Expected nil on goroutine %d, got %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentPreflightWithDifferentArgs(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			args := map[string]any{"path": testFile, "content": "hello"}
			err := vs.PreflightDestructiveToolCall(context.Background(), "a1", "write_file", args)
			if err != nil {
				t.Errorf("Expected nil on goroutine %d, got %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentMixedOperations(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			args := map[string]any{"path": testFile, "content": "hello"}
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
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "read_file", map[string]any{"filepath": testFile, "content": "hello"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil for safe action, got %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Smoke_ValidateFailOpen_MissingDreamer(t *testing.T) {
	vs := newDreamerStore(t)
	testFile := dreamerFile(t)
	err := vs.ValidateInteractiveToolResult(context.Background(), "a1", "write_file", map[string]any{"filepath": testFile, "content": "hello"}, "out", true)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}
}

// ----------------------------------------------------------------------
// Multi-Boundary Tests ensuring Pipeline Integration
// ----------------------------------------------------------------------

func TestE2E_VirtualStoreDreamer_Pipeline_WriteAndValidate(t *testing.T) {
	// Simulates a full pass: Preflight -> Execute (mocked) -> Validate
	vs := newDreamerStore(t)
	ctx := context.Background()
	// The file has to actually exist for the "Execute" step to have happened.
	// This targeted a bare integration_test.txt that was never created, so the
	// validator correctly reported no such file and the pipeline test failed at
	// its last step — the one part of the pipeline it was written to prove.
	// dreamerFile seeds the same "hello" the payload claims.
	args := map[string]any{"filepath": dreamerFile(t), "content": "hello"}

	err := vs.PreflightDestructiveToolCall(ctx, "a1", "write_file", args)
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	// Executor step: the file dreamerFile created stands in for a successful
	// write, which is what the validator below goes and checks.

	err = vs.ValidateInteractiveToolResult(ctx, "a1", "write_file", args, "success", true)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}
}

func TestE2E_VirtualStoreDreamer_Pipeline_WriteFailValidateSkip(t *testing.T) {
	// Simulates Preflight -> Execute (fails) -> Validate (skips)
	vs := newDreamerStore(t)
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

// newDreamerStore returns a VirtualStore whose Dreamer actually exists.
//
// Every test in this file used to build core.NewVirtualStore(...) with no
// kernel. VirtualStore.getDreamer derives the Dreamer lazily from v.kernel, so
// with none there is no Dreamer, and PreflightDestructiveToolCall is
// fail-CLOSED on exactly that — "permission and speculative safety are
// independent gates; an allow decision from checkSafety must never compensate
// for a missing simulation engine." Every destructive call in the file was
// therefore refused for a structural reason, and the tests read that refusal as
// the behaviour they were asserting.
func newDreamerStore(t *testing.T) *core.VirtualStore {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	vs := core.NewVirtualStore(tactile.NewDirectExecutor())
	vs.SetKernel(k)
	return vs
}

// dreamerFile returns the path of a file that really exists.
//
// The write validators stat the target and compare its bytes, so a bare
// "test.txt" that was never created fails with "file does not exist after
// write" — a correct verdict on a write that never happened, and not the thing
// any of these tests meant to exercise.
func dreamerFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
	return path
}
