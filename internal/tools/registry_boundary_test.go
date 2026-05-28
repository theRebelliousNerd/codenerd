package tools

import (
	"context"
	"errors"
	"math"
	"testing"
)

// TestRegistry_RegisterNilTool verifies that Register(nil) returns ErrToolNil
// rather than panicking. Defends against bad callers passing nil tools (e.g.,
// uninitialized factory output).
//
// QA boundary item: TestRegistry_RegisterNilTool.
func TestRegistry_RegisterNilTool(t *testing.T) {
	reg := NewRegistry()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Register(nil) panicked: %v", r)
		}
	}()

	err := reg.Register(nil)
	if err == nil {
		t.Fatal("expected error for nil tool")
	}
	if !errors.Is(err, ErrToolNil) {
		t.Errorf("expected ErrToolNil, got %v", err)
	}

	// Registry must still be empty / functional after a nil registration.
	if reg.Count() != 0 {
		t.Errorf("expected registry to remain empty after nil registration, got %d", reg.Count())
	}
}

// TestRegistry_ExecuteNilContext verifies Execute and ExecuteTool gracefully
// fall back to context.Background() when nil is passed.
//
// QA boundary item: TestRegistry_ExecuteNilContext.
func TestRegistry_ExecuteNilContext(t *testing.T) {
	reg := NewRegistry()

	// Record the ctx the Execute function actually receives.
	var seenCtx context.Context
	tool := &Tool{
		Name:     "ctx_probe",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			seenCtx = ctx
			return "ok", nil
		},
	}
	reg.MustRegister(tool)

	// Must NOT panic on nil ctx.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Execute panicked on nil ctx: %v", r)
		}
	}()

	//nolint:staticcheck // intentionally passing nil to verify guard
	result, err := reg.Execute(nil, "ctx_probe", map[string]any{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Result != "ok" {
		t.Errorf("expected ok result, got %q", result.Result)
	}
	if seenCtx == nil {
		t.Error("expected tool to receive a non-nil context (fallback to Background)")
	}

	// And again via ExecuteTool directly.
	//nolint:staticcheck // intentionally passing nil
	if _, err := reg.ExecuteTool(nil, tool, map[string]any{}); err != nil {
		t.Errorf("ExecuteTool with nil ctx returned error: %v", err)
	}
}

// TestRegistry_FilterByIntent_Empty verifies that empty or unknown intent
// strings return the safe fallback (all tools) rather than an empty slice
// or a panic.
//
// QA boundary item: TestRegistry_FilterByIntent_Empty.
func TestRegistry_FilterByIntent_Empty(t *testing.T) {
	reg := NewRegistry()
	tools := []*Tool{
		{Name: "a", Category: CategoryResearch, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
		{Name: "b", Category: CategoryCode, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
		{Name: "c", Category: CategoryTest, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
	}
	for _, tool := range tools {
		reg.MustRegister(tool)
	}

	// Empty string -> All().
	if got := reg.FilterByIntent(""); len(got) != 3 {
		t.Errorf("FilterByIntent(\"\") = %d tools, want 3 (all)", len(got))
	}

	// Hallucinated intent -> All().
	if got := reg.FilterByIntent("/hallucinated"); len(got) != 3 {
		t.Errorf("FilterByIntent(hallucinated) = %d tools, want 3 (all)", len(got))
	}

	// Verify an empty registry returns an empty (but non-nil-panicking) slice.
	emptyReg := NewRegistry()
	if got := emptyReg.FilterByIntent(""); len(got) != 0 {
		t.Errorf("FilterByIntent on empty registry = %d tools, want 0", len(got))
	}
}

// TestRegistry_Execute_TypeMismatch verifies validateArgs rejects mismatched
// types — e.g., an int passed for a string-schema property.
//
// QA boundary item: validate type mismatch in validateArgs.
func TestRegistry_Execute_TypeMismatch(t *testing.T) {
	reg := NewRegistry()
	tool := &Tool{
		Name:     "echo",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "ok", nil
		},
		Schema: ToolSchema{
			Required:   []string{"message"},
			Properties: map[string]Property{"message": {Type: "string"}},
		},
	}
	reg.MustRegister(tool)

	// int provided for string property — must reject.
	_, err := reg.Execute(context.Background(), "echo", map[string]any{"message": 42})
	if err == nil {
		t.Fatal("expected type mismatch error for int->string")
	}
	if !errors.Is(err, ErrInvalidArgType) {
		t.Errorf("expected ErrInvalidArgType, got %v", err)
	}

	// Correct string passes.
	if _, err := reg.Execute(context.Background(), "echo", map[string]any{"message": "hi"}); err != nil {
		t.Errorf("valid string arg unexpectedly errored: %v", err)
	}
}

// TestRegistry_PrioritySorting_Extremes verifies that GetByCategory correctly
// sorts tools when priorities are at extreme integer values.
//
// QA boundary item: TestRegistry_PrioritySorting_Extremes (MaxInt64, MinInt64).
func TestRegistry_PrioritySorting_Extremes(t *testing.T) {
	reg := NewRegistry()
	tools := []*Tool{
		{Name: "min", Category: CategoryCode, Priority: math.MinInt, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
		{Name: "max", Category: CategoryCode, Priority: math.MaxInt, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
		{Name: "mid", Category: CategoryCode, Priority: 50, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
	}
	for _, tool := range tools {
		reg.MustRegister(tool)
	}

	result := reg.GetByCategory(CategoryCode)
	if len(result) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result))
	}
	// Expected order: max (MaxInt) > mid (50) > min (MinInt).
	if result[0].Name != "max" {
		t.Errorf("expected MaxInt-priority tool first, got %q", result[0].Name)
	}
	if result[2].Name != "min" {
		t.Errorf("expected MinInt-priority tool last, got %q", result[2].Name)
	}
}

// TestRegistry_Execute_NilArgs verifies Execute does not panic on nil args
// (only missing required args should produce an error).
func TestRegistry_Execute_NilArgs(t *testing.T) {
	reg := NewRegistry()
	tool := &Tool{
		Name:     "noargs",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "ok", nil
		},
	}
	reg.MustRegister(tool)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Execute panicked on nil args: %v", r)
		}
	}()

	result, err := reg.Execute(context.Background(), "noargs", nil)
	if err != nil {
		t.Errorf("Execute(nil args) errored unexpectedly: %v", err)
	}
	if result == nil || result.Result != "ok" {
		t.Errorf("expected ok result, got %+v", result)
	}
}
