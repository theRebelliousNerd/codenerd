package session

import (
	"context"
	"strings"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/tools"
)

func TestExecuteToolCall_ShellEffectGateStopsIncidentBeforeExecution(t *testing.T) {
	executed := false
	toolName := "run_shell"
	if err := tools.Global().Register(&tools.Tool{
		Name:     toolName,
		Category: tools.CategoryCode,
		Schema: tools.ToolSchema{
			Required: []string{"command"},
		},
		Execute: func(context.Context, map[string]any) (string, error) {
			executed = true
			return "executed", nil
		},
	}); err != nil {
		t.Fatalf("register shell probe: %v", err)
	}

	executorCfg := DefaultExecutorConfig()
	executorCfg.EnableSafetyGate = false
	e := &Executor{config: executorCfg}
	runtimeCfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}}

	for _, command := range []string{
		"git checkout -- internal/logging/logging.go internal/logging/audit.go",
		`python -c "import shutil; shutil.rmtree('internal/browser/repotrace')"`,
	} {
		executed = false
		_, err := e.executeToolCall(context.Background(), ToolCall{
			Name: toolName,
			Args: map[string]any{"command": command},
		}, runtimeCfg)
		if err == nil {
			t.Fatalf("incident command %q was allowed", command)
		}
		if !strings.HasPrefix(err.Error(), "blocked by shell-effect gate:") {
			t.Fatalf("incident command reached a later gate: %v", err)
		}
		if executed {
			t.Fatalf("incident command %q reached its tool handler", command)
		}
	}

	executed = false
	result, err := e.executeToolCall(context.Background(), ToolCall{
		Name: toolName,
		Args: map[string]any{"command": "git status --short"},
	}, runtimeCfg)
	if err != nil {
		t.Fatalf("read-only command was denied: %v", err)
	}
	if !executed || result != "executed" {
		t.Fatalf("read-only command did not reach handler: executed=%v result=%q", executed, result)
	}
}
