package autopoiesis

import (
	"context"
	"os"
	"testing"
)

func TestFindEntryPoint(t *testing.T) {
	tc := NewToolCompiler(DefaultOuroborosConfig(t.TempDir()))

	code := `package main

import "context"

func Execute(ctx context.Context, input string) (string, error) {
	return "echo:" + input, nil
}
`
	name, err := tc.findEntryPoint(code)
	if err != nil || name != "Execute" {
		t.Errorf("findEntryPoint=(%q,%v), want (Execute,nil)", name, err)
	}

	// Code with no matching (ctx, string) -> (string, error) function fails.
	noEntry := `package main

func Helper() int { return 1 }
`
	if _, err := tc.findEntryPoint(noEntry); err == nil {
		t.Error("expected an error when no entry point is present")
	}
}

func TestToolCompiler_Compile_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go-build cross-boundary test in -short mode")
	}
	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.WorkspaceRoot = "" // skip the codenerd replace directive for a self-contained build
	tc := NewToolCompiler(cfg)

	tool := &GeneratedTool{
		Name: "echo_tool",
		Code: `package tools

import "context"

func Execute(ctx context.Context, input string) (string, error) {
	return "echo:" + input, nil
}
`,
	}

	res, err := tc.Compile(context.Background(), tool)
	if err != nil {
		t.Fatalf("Compile failed: %v (errors: %v)", err, res.Errors)
	}
	if !res.Success {
		t.Fatalf("expected successful compile, errors: %v", res.Errors)
	}
	if res.OutputPath == "" || res.Hash == "" {
		t.Errorf("expected output path and hash, got %+v", res)
	}
	if _, err := os.Stat(res.OutputPath); err != nil {
		t.Errorf("compiled binary not found at %s: %v", res.OutputPath, err)
	}
}
