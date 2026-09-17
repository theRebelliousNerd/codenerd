package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

type failingValidationStore struct {
	testExecutiveStore
	err error
}

func (s *failingValidationStore) ValidateInteractiveToolResult(context.Context, string, string, map[string]any, string, bool) error {
	return s.err
}

func TestToolErrorOutput_FailedToolOutputReachesModel(t *testing.T) {
	n := "tool_error_output_probe_1"
	tool := tools.Tool{
		Effect:   tools.EffectRead,
		Name:     n,
		Category: tools.CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "BUILD OUTPUT: main.go:1:1: expected 'package'", errors.New("exit status 1")
		},
	}
	registerTestTool(t, &tool)
	store := &testExecutiveStore{}
	e := &Executor{config: ExecutorConfig{}, virtualStore: store}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{n}}
	results, _ := e.executeToolBatch(context.Background(), []types.ToolCall{{ID: "id-1", Name: n}}, cfg, &ExecutionResult{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError {
		t.Fatalf("expected IsError=true, got false (content=%q)", results[0].Content)
	}
	if !strings.Contains(results[0].Content, "exit status 1") {
		t.Fatalf("expected content to contain %q, got %q", "exit status 1", results[0].Content)
	}
	if !strings.Contains(results[0].Content, "main.go:1:1: expected 'package'") {
		t.Fatalf("expected content to contain %q, got %q", "main.go:1:1: expected 'package'", results[0].Content)
	}
}

func TestToolErrorOutput_FailedToolWithoutOutputSendsErrorOnly(t *testing.T) {
	n := "tool_error_output_probe_2"
	tool := tools.Tool{
		Effect:   tools.EffectRead,
		Name:     n,
		Category: tools.CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "", errors.New("boom")
		},
	}
	registerTestTool(t, &tool)
	store := &testExecutiveStore{}
	e := &Executor{config: ExecutorConfig{}, virtualStore: store}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{n}}
	results, _ := e.executeToolBatch(context.Background(), []types.ToolCall{{ID: "id-1", Name: n}}, cfg, &ExecutionResult{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError {
		t.Fatalf("expected IsError=true, got false (content=%q)", results[0].Content)
	}
	if !strings.Contains(results[0].Content, "boom") {
		t.Fatalf("expected content to contain %q, got %q", "boom", results[0].Content)
	}
	if strings.HasSuffix(results[0].Content, "\n") {
		t.Fatalf("expected content to not end with newline, got %q", results[0].Content)
	}
}

func TestInteractiveGateValidation_FailureSurfacesAsError(t *testing.T) {
	n := "tool_error_output_probe_3"
	tool := tools.Tool{
		Effect:   tools.EffectRead,
		Name:     n,
		Category: tools.CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "wrote x.go", nil
		},
	}
	registerTestTool(t, &tool)
	store := &failingValidationStore{err: errors.New("syntax validation failed: x.go:1:1")}
	e := &Executor{config: ExecutorConfig{}, virtualStore: store}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{n}}
	out, err := e.executeToolCall(context.Background(), ToolCall{ID: "id-1", Name: n}, cfg)
	if err == nil {
		t.Fatalf("expected error, got nil (out=%q)", out)
	}
	if !strings.Contains(err.Error(), "post-action validation failed") {
		t.Fatalf("expected error to contain %q, got %q", "post-action validation failed", err.Error())
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}

func TestInteractiveGateValidation_PassReturnsOutput(t *testing.T) {
	n := "tool_error_output_probe_4"
	tool := tools.Tool{
		Effect:   tools.EffectRead,
		Name:     n,
		Category: tools.CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "wrote x.go", nil
		},
	}
	registerTestTool(t, &tool)
	store := &failingValidationStore{err: nil}
	e := &Executor{config: ExecutorConfig{}, virtualStore: store}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{n}}
	out, err := e.executeToolCall(context.Background(), ToolCall{ID: "id-1", Name: n}, cfg)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out != "wrote x.go" {
		t.Fatalf("expected output %q, got %q", "wrote x.go", out)
	}
}
