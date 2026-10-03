package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/types"
)

type generatedCoreBackend struct {
	calls  int
	effect bool
}

func (*generatedCoreBackend) ExecuteTool(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("legacy router must not execute")
}
func (*generatedCoreBackend) ListTools() []ToolInfo            { return nil }
func (*generatedCoreBackend) GetTool(string) (*ToolInfo, bool) { return nil, false }
func (b *generatedCoreBackend) ExecuteGenerated(ctx context.Context, request types.GeneratedToolRequest, validate types.GeneratedToolValidator) (types.GeneratedToolReceipt, error) {
	b.calls++
	args, err := request.Args()
	if err != nil {
		return types.GeneratedToolReceipt{}, err
	}
	if b.effect {
		if err := os.WriteFile(request.Target, []byte(args["content"].(string)), 0600); err != nil {
			return types.GeneratedToolReceipt{}, err
		}
	}
	receipt := types.GeneratedToolReceipt{Request: request, Attempted: true, ProcessStarted: true, Output: "literal", ExitCode: 0, ValidationCompleted: true}
	receipt.ValidationError = validate(ctx, request, receipt)
	receipt.ValidationPassed = receipt.ValidationError == nil
	if receipt.Err() != nil {
		return receipt, &types.GeneratedExecutionError{Receipt: receipt}
	}
	return receipt, nil
}

func TestGeneratedVirtualStoreExactEnvelopeAndRealValidation(t *testing.T) {
	workspace := t.TempDir()
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	kernel.SetWorkspace(workspace)
	configuration := DefaultVirtualStoreConfig()
	configuration.WorkingDir = workspace
	store := NewVirtualStoreWithConfig(nil, configuration)
	store.SetKernel(kernel)
	registry := store.GetToolRegistry()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterTool("generated_probe", executable, "/all"); err != nil {
		t.Fatal(err)
	}
	identity, err := registry.GeneratedToolIdentity("generated_probe")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(workspace, "effect.txt")
	canonical, err := types.CanonicalGeneratedArgs(map[string]any{"path": target, "content": "literal bytes"})
	if err != nil {
		t.Fatal(err)
	}
	request := types.GeneratedToolRequest{ScopeID: "scope", CallID: "call", AuthorizationID: "exec-call", Action: "/generated_probe", Target: target, CanonicalArgs: canonical, Tool: identity}
	for _, fact := range []Fact{
		{Predicate: "pending_action", Args: []any{request.AuthorizationID, request.Action, request.Target, canonical, time.Now().Unix()}},
		{Predicate: "permitted_action", Args: []any{request.AuthorizationID, request.Action, request.Target, canonical, time.Now().Unix()}},
		{Predicate: "permission_check_result", Args: []any{request.AuthorizationID, MangleAtom("/permit"), "exact fixture admission", time.Now().Unix()}},
	} {
		if err := kernel.Assert(fact); err != nil {
			t.Fatal(err)
		}
	}
	backend := &generatedCoreBackend{effect: true}
	store.SetToolExecutor(backend)
	receipt, err := store.ExecuteGeneratedToolCall(context.Background(), request)
	if err != nil || !receipt.ValidationPassed || backend.calls != 1 {
		t.Fatalf("receipt=%+v err=%v calls=%d", receipt, err, backend.calls)
	}
	bytes, err := os.ReadFile(target)
	if err != nil || string(bytes) != "literal bytes" {
		t.Fatalf("effect=%q err=%v", bytes, err)
	}
	// Red twin: keep backend success but disconnect the actual effect.
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	backend.effect = false
	if receipt, err := store.ExecuteGeneratedToolCall(context.Background(), request); err == nil || receipt.ValidationPassed {
		t.Fatalf("validator accepted absent effect: %+v %v", receipt, err)
	}
	before := backend.calls
	mutated := request
	mutated.CanonicalArgs = `{"content":"changed","path":"other.txt"}`
	if _, err := store.ExecuteGeneratedToolCall(context.Background(), mutated); err == nil {
		t.Fatal("mutated authorization payload admitted")
	}
	mutated = request
	mutated.AuthorizationID = "replacement"
	if _, err := store.ExecuteGeneratedToolCall(context.Background(), mutated); err == nil {
		t.Fatal("replacement authorization admitted")
	}
	mutated = request
	mutated.Tool.Protocol = "unknown"
	if _, err := store.ExecuteGeneratedToolCall(context.Background(), mutated); err == nil {
		t.Fatal("unknown protocol admitted")
	}
	if backend.calls != before {
		t.Fatal("refused call reached backend")
	}
	store.SetToolExecutor(nil)
	if _, err := store.ExecuteGeneratedToolCall(context.Background(), request); err == nil {
		t.Fatal("disconnected bridge admitted")
	}
}
