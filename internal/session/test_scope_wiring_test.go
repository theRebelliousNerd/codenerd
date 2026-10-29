package session

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	modulartools "codenerd/internal/tools"
	"codenerd/internal/types"
)

// A test tool called with no packages tests what the turn wrote. The tool can
// only do that if the executor hands it the write set, read at the moment of
// the call: a scope captured when the turn began would be empty for every
// turn, and the tool would report "nothing written" after an edit.
func TestExecutor_HandsTheTurnsWriteSetToTestTools(t *testing.T) {
	toolName := fmt.Sprintf("test_scope_probe_%d", capabilityTestToolCounter.Add(1))
	var seen [][]string
	registerTestTool(t, &modulartools.Tool{
		Effect:      modulartools.EffectRead,
		Name:        toolName,
		Description: "reports the test scope it was given",
		Execute: func(ctx context.Context, _ map[string]any) (string, error) {
			scope, ok := modulartools.TestScopeFrom(ctx)
			if !ok {
				t.Error("the tool context carries no test scope")
				return "", nil
			}
			seen = append(seen, scope())
			return "ok", nil
		},
	})

	executor := &Executor{config: DefaultExecutorConfig()}
	executor.config.EnableSafetyGate = false
	cfg := validCapabilityTestConfig(toolName)
	result := &ExecutionResult{}
	call := types.ToolCall{ID: "1", Name: toolName}

	if _, err := executor.executeAndRecordToolCall(context.Background(), call, cfg, result); err != nil {
		t.Fatalf("first call: %v", err)
	}
	result.WrittenPaths = append(result.WrittenPaths,
		"internal/session/executor.go", "internal/session/executor_test.go", "Docs/notes.md", "cmd/nerd/main.go")
	if _, err := executor.executeAndRecordToolCall(context.Background(), call, cfg, result); err != nil {
		t.Fatalf("second call: %v", err)
	}

	want := [][]string{nil, {"./cmd/nerd", "./internal/session"}}
	if len(seen) != 2 || len(seen[0]) != 0 || !reflect.DeepEqual(seen[1], want[1]) {
		t.Fatalf("scopes seen = %v, want %v", seen, want)
	}
}
