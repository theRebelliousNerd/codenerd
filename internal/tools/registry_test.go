package tools

import (
	"context"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if reg.Count() != 0 {
		t.Errorf("new registry should be empty, got %d tools", reg.Count())
	}
}

func TestRegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Category:    CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "success", nil
		},
		Schema: ToolSchema{
			Required: []string{},
		},
	}

	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got := reg.Get("test_tool")
	if got == nil {
		t.Fatal("Get returned nil for registered tool")
	}
	if got.Name != "test_tool" {
		t.Errorf("got name %q, want %q", got.Name, "test_tool")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	reg := NewRegistry()

	tool := &Tool{
		Name:     "dupe",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "", nil
		},
	}

	if err := reg.Register(tool); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := reg.Register(tool)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegisterValidation(t *testing.T) {
	reg := NewRegistry()

	tests := []struct {
		name    string
		tool    *Tool
		wantErr error
	}{
		{
			name:    "empty name",
			tool:    &Tool{Name: "", Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
			wantErr: ErrToolNameEmpty,
		},
		{
			name:    "nil execute",
			tool:    &Tool{Name: "test", Execute: nil},
			wantErr: ErrToolExecuteNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reg.Register(tt.tool)
			if err == nil {
				t.Errorf("expected error %v, got nil", tt.wantErr)
			}
		})
	}
}

func TestGetByCategory(t *testing.T) {
	reg := NewRegistry()

	tools := []*Tool{
		{Name: "research1", Category: CategoryResearch, Priority: 80, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
		{Name: "research2", Category: CategoryResearch, Priority: 60, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
		{Name: "code1", Category: CategoryCode, Priority: 50, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
	}

	for _, tool := range tools {
		reg.MustRegister(tool)
	}

	research := reg.GetByCategory(CategoryResearch)
	if len(research) != 2 {
		t.Errorf("expected 2 research tools, got %d", len(research))
	}

	// Should be sorted by priority (highest first)
	if research[0].Name != "research1" {
		t.Errorf("expected research1 first (priority 80), got %s", research[0].Name)
	}
}

func TestExecute(t *testing.T) {
	reg := NewRegistry()

	tool := &Tool{
		Name:     "echo",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			msg, _ := args["message"].(string)
			return "Echo: " + msg, nil
		},
		Schema: ToolSchema{
			Required:   []string{"message"},
			Properties: map[string]Property{"message": {Type: "string"}},
		},
	}

	reg.MustRegister(tool)

	// Test successful execution
	result, err := reg.Execute(context.Background(), "echo", map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Result != "Echo: hello" {
		t.Errorf("got result %q, want %q", result.Result, "Echo: hello")
	}
	if !result.IsSuccess() {
		t.Error("expected IsSuccess to be true")
	}

	// Test missing required arg
	_, err = reg.Execute(context.Background(), "echo", map[string]any{})
	if err == nil {
		t.Error("expected error for missing required arg")
	}

	// Test tool not found
	_, err = reg.Execute(context.Background(), "nonexistent", map[string]any{})
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestFilterByIntent(t *testing.T) {
	reg := NewRegistry()

	tools := []*Tool{
		{Name: "context7", Category: CategoryResearch, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
		{Name: "file_write", Category: CategoryCode, Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }},
	}

	for _, tool := range tools {
		reg.MustRegister(tool)
	}

	research := reg.FilterByIntent("/research")
	if len(research) != 1 || research[0].Name != "context7" {
		t.Errorf("FilterByIntent(/research) returned wrong tools: %v", research)
	}

	code := reg.FilterByIntent("/fix")
	if len(code) != 1 || code[0].Name != "file_write" {
		t.Errorf("FilterByIntent(/fix) returned wrong tools: %v", code)
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Reset global registry for test
	globalRegistry = NewRegistry()

	tool := &Tool{
		Name:     "global_test",
		Category: CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "global", nil
		},
	}

	if err := Register(tool); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got := Get("global_test")
	if got == nil {
		t.Fatal("Get returned nil for globally registered tool")
	}

	result, err := Execute(context.Background(), "global_test", map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Result != "global" {
		t.Errorf("got result %q, want %q", result.Result, "global")
	}
}
