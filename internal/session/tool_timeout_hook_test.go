package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/tools"
)

func remainingBudgetExecute(ctx context.Context, _ map[string]any) (string, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return "no-deadline", nil
	}
	return time.Until(deadline).String(), nil
}

func TestExecuteToolCall_ToolTimeoutHookExtendsBudget(t *testing.T) {
	longName := "test_tool_timeout_hook_long"
	registerTestTool(t, &tools.Tool{
		Name:        longName,
		Effect:      tools.EffectRead,
		Description: "reports remaining context budget",
		Category:    tools.CategoryTest,
		Execute:     remainingBudgetExecute,
		Timeout: func(_ map[string]any) time.Duration {
			return 2 * time.Hour
		},
	})
	e := &Executor{config: ExecutorConfig{ToolTimeout: time.Minute}, virtualStore: &MockVirtualStore{}}
	out, err := e.executeToolCall(context.Background(), ToolCall{ID: "t", Name: longName}, &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{longName}})
	if err != nil {
		t.Fatalf("executeToolCall long tool: %v", err)
	}
	d, err := time.ParseDuration(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("parse long tool remaining %q: %v", out, err)
	}
	if d <= time.Hour {
		t.Fatalf("long tool budget not extended: remaining %v, want > 1h", d)
	}

	shortName := "test_tool_timeout_hook_short"
	registerTestTool(t, &tools.Tool{
		Name:        shortName,
		Effect:      tools.EffectRead,
		Description: "reports remaining context budget",
		Category:    tools.CategoryTest,
		Execute:     remainingBudgetExecute,
	})
	out, err = e.executeToolCall(context.Background(), ToolCall{ID: "t", Name: shortName}, &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{shortName}})
	if err != nil {
		t.Fatalf("executeToolCall short tool: %v", err)
	}
	d, err = time.ParseDuration(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("parse short tool remaining %q: %v", out, err)
	}
	if d > time.Minute {
		t.Fatalf("short tool budget exceeds default: remaining %v, want <= 1m", d)
	}
}
