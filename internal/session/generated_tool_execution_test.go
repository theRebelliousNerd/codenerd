package session

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

type generatedSessionStore struct {
	MockVirtualStore
	run func(types.GeneratedToolRequest) (types.GeneratedToolReceipt, error)
}

func (s *generatedSessionStore) ExecuteGeneratedToolCall(_ context.Context, request types.GeneratedToolRequest) (types.GeneratedToolReceipt, error) {
	return s.run(request)
}

func TestGeneratedSessionRetainsExactAuthorizationAndPartialReceipt(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	kernel.SetWorkspace(t.TempDir())
	registry := core.NewToolRegistry(t.TempDir())
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterTool("generated_probe", executable, "/all"); err != nil {
		t.Fatal(err)
	}
	call := ToolCall{ID: "original-call", Name: "generated_probe", Args: map[string]any{"path": "effect.txt", "n": 1}}
	canonical, err := types.CanonicalGeneratedArgs(call.Args)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []types.Fact{
		{Predicate: "permitted_action", Args: []any{"exec-" + call.ID, types.MangleAtom("/generated_probe"), "effect.txt", canonical, time.Now().Unix()}},
		{Predicate: "permission_check_result", Args: []any{"exec-" + call.ID, types.MangleAtom("/permit"), "exact fixture admission", time.Now().Unix()}},
	} {
		if err := kernel.Assert(fact); err != nil {
			t.Fatal(err)
		}
	}
	backendFailure := errors.New("started process failed")
	var calls int
	store := &generatedSessionStore{}
	store.run = func(request types.GeneratedToolRequest) (types.GeneratedToolReceipt, error) {
		calls++
		pending, err := kernel.Query("pending_action")
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) != 1 || types.ExtractString(pending[0].Args[0]) != "exec-"+call.ID || request.CallID != call.ID || request.AuthorizationID != "exec-"+call.ID || request.CanonicalArgs != canonical || request.Action != "/generated_probe" || request.Target != "effect.txt" {
			t.Fatalf("authorization changed: request=%+v pending=%v", request, pending)
		}
		receipt := types.GeneratedToolReceipt{Request: request, ProcessStarted: true, Output: "partial output", BackendError: backendFailure}
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	executor := NewExecutor(kernel, store, nil, nil, nil, nil)
	executor.SetOuroborosRegistry(registry)
	output, err := executor.executeToolCall(context.Background(), call, validCapabilityTestConfig(call.Name))
	if !errors.Is(err, backendFailure) || output != "partial output" || calls != 1 {
		t.Fatalf("output=%q err=%v calls=%d", output, err, calls)
	}
	pending, err := kernel.Query("pending_action")
	if err != nil || len(pending) != 0 {
		t.Fatalf("authorization lease leaked: %v %v", pending, err)
	}
	store.run = func(request types.GeneratedToolRequest) (types.GeneratedToolReceipt, error) {
		return types.GeneratedToolReceipt{Request: request, ProcessStarted: true, ValidationCompleted: true, ValidationPassed: true}, nil
	}
	if _, err := executor.executeToolCall(context.Background(), call, validCapabilityTestConfig(call.Name)); err == nil {
		t.Fatal("feedback-free receipt was acknowledged")
	}
	executor.virtualStore = &MockVirtualStore{}
	if _, err := executor.executeToolCall(context.Background(), call, validCapabilityTestConfig(call.Name)); err == nil {
		t.Fatal("disconnected bridge fell back to raw process execution")
	}
	if calls != 1 {
		t.Fatal("disconnected call invoked backend")
	}
}

func TestGeneratedSessionRefusesUnknownProtocolBeforeAdmission(t *testing.T) {
	registry := core.NewToolRegistry(t.TempDir())
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterToolWithInfo(&core.Tool{Name: "generated_probe", Command: executable}); err != nil {
		t.Fatal(err)
	}
	store := &generatedSessionStore{run: func(types.GeneratedToolRequest) (types.GeneratedToolReceipt, error) {
		t.Fatal("unknown protocol reached backend")
		return types.GeneratedToolReceipt{}, nil
	}}
	executor := NewExecutor(nil, store, nil, nil, nil, nil)
	executor.SetOuroborosRegistry(registry)
	if _, err := executor.executeToolCall(context.Background(), ToolCall{ID: "call", Name: "generated_probe"}, validCapabilityTestConfig("generated_probe")); err == nil {
		t.Fatal("unknown protocol was admitted")
	}
}

func TestGeneratedSessionDuplicateAuthorizationLease(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	kernel.SetWorkspace(t.TempDir())
	call := ToolCall{ID: "same-call", Name: "generated_probe", Args: map[string]any{"path": "effect.txt"}}
	canonical, err := types.CanonicalGeneratedArgs(call.Args)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []types.Fact{
		{Predicate: "permitted_action", Args: []any{"exec-" + call.ID, types.MangleAtom("/generated_probe"), "effect.txt", canonical, time.Now().Unix()}},
		{Predicate: "permission_check_result", Args: []any{"exec-" + call.ID, types.MangleAtom("/permit"), "exact admission", time.Now().Unix()}},
	} {
		if err := kernel.Assert(fact); err != nil {
			t.Fatal(err)
		}
	}
	executor := NewExecutor(kernel, nil, nil, nil, nil, nil)
	var first, second func()
	if ok, reason := executor.checkSafetyWithLease(call, true, canonical, &first); !ok {
		t.Fatal(reason)
	}
	defer first()
	if ok, reason := executor.checkSafetyWithLease(call, true, canonical, &second); !ok {
		t.Fatal(reason)
	}
	defer second()
	conflict := call
	conflict.Args = map[string]any{"path": "different.txt"}
	changed, err := types.CanonicalGeneratedArgs(conflict.Args)
	if err != nil {
		t.Fatal(err)
	}
	var refused func()
	if ok, _ := executor.checkSafetyWithLease(conflict, true, changed, &refused); ok || refused != nil {
		t.Fatal("conflicting authorization admitted")
	}
	first()
	if pending, err := kernel.Query("pending_action"); err != nil || len(pending) != 1 {
		t.Fatalf("duplicate released active authorization: %v %v", pending, err)
	}
	second()
	if pending, err := kernel.Query("pending_action"); err != nil || len(pending) != 0 {
		t.Fatalf("last lease did not release: %v %v", pending, err)
	}
	if executor.generatedScope() != executor.generatedScope() {
		t.Fatal("live retry scope changed")
	}
}
