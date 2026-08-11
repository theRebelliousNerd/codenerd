package autopoiesis

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestToolCompiler_Compile_GeneratedTestGating(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go-build cross-boundary test in -short mode")
	}

	t.Run("failing generated test blocks registration", func(t *testing.T) {
		cfg := DefaultOuroborosConfig(t.TempDir())
		cfg.WorkspaceRoot = "" // self-contained build, no codenerd replace
		tc := NewToolCompiler(cfg)

		tool := &GeneratedTool{
			Name: "gated_tool",
			Code: `package tools

import "context"

func Execute(ctx context.Context, input string) (string, error) {
	return "echo:" + input, nil
}
`,
			TestCode: `package tools

import "testing"

func TestExecute(t *testing.T) {
	t.Fatalf("deliberate failure: expected 50 got 0")
}
`,
		}

		res, err := tc.Compile(context.Background(), tool)
		if err == nil {
			t.Fatalf("expected Compile to fail when generated tests fail, got res=%+v", res)
		}
		if !strings.Contains(err.Error(), "generated tool failed its own generated tests") {
			t.Errorf("error should plainly state generated tests failed, got %q", err.Error())
		}
		// The test output (deliberate failure) must be included in the error or result.Errors.
		foundOutput := strings.Contains(err.Error(), "deliberate failure")
		if !foundOutput {
			for _, e := range res.Errors {
				if strings.Contains(e, "deliberate failure") {
					foundOutput = true
					break
				}
			}
		}
		if !foundOutput {
			t.Errorf("expected test output to be included, err=%q errors=%v", err.Error(), res.Errors)
		}
		if res != nil && res.Success {
			t.Error("result.Success should be false when generated tests fail")
		}
		// No binary should be produced. If OutputPath was set, verify it does not exist.
		if res != nil && res.OutputPath != "" {
			if _, statErr := os.Stat(res.OutputPath); statErr == nil {
				t.Errorf("binary should not exist after gating failure, found at %s", res.OutputPath)
			}
		}
		// Also ensure CompiledDir does not contain a binary for this tool.
		compiledPath := res.OutputPath
		if compiledPath == "" {
			// Fallback: check expected location directly.
			compiledPath = cfg.CompiledDir + "/gated_tool"
			if _, statErr := os.Stat(compiledPath); statErr == nil {
				t.Errorf("binary should not exist at expected path %s after gating failure", compiledPath)
			}
			compiledPath += ".exe"
			if _, statErr := os.Stat(compiledPath); statErr == nil {
				t.Errorf("binary should not exist at expected path %s after gating failure", compiledPath)
			}
		}
	})

	t.Run("empty TestCode skips gating and succeeds", func(t *testing.T) {
		cfg := DefaultOuroborosConfig(t.TempDir())
		cfg.WorkspaceRoot = ""
		tc := NewToolCompiler(cfg)

		tool := &GeneratedTool{
			Name: "empty_test_tool",
			Code: `package tools

import "context"

func Execute(ctx context.Context, input string) (string, error) {
	return "ok:" + input, nil
}
`,
			TestCode: "",
		}

		res, err := tc.Compile(context.Background(), tool)
		if err != nil {
			t.Fatalf("Compile with empty TestCode should succeed, got err=%v (errors=%v)", err, res.Errors)
		}
		if !res.Success {
			t.Fatalf("expected successful compile when TestCode empty, got %+v", res)
		}
		if res.OutputPath == "" || res.Hash == "" {
			t.Errorf("expected output path and hash, got %+v", res)
		}
		if _, err := os.Stat(res.OutputPath); err != nil {
			t.Errorf("compiled binary not found at %s: %v", res.OutputPath, err)
		}
	})

	t.Run("non-compiling generated test also blocks", func(t *testing.T) {
		cfg := DefaultOuroborosConfig(t.TempDir())
		cfg.WorkspaceRoot = ""
		tc := NewToolCompiler(cfg)

		tool := &GeneratedTool{
			Name: "bad_test_tool",
			Code: `package tools

import "context"

func Execute(ctx context.Context, input string) (string, error) {
	return "ok", nil
}
`,
			TestCode: `package tools

import "testing"

func TestBad(t *testing.T) {
	undefinedFunction()
}
`,
		}

		res, err := tc.Compile(context.Background(), tool)
		if err == nil {
			t.Fatalf("expected Compile to fail when generated test does not compile, got res=%+v", res)
		}
		if !strings.Contains(err.Error(), "generated tool failed its own generated tests") {
			t.Errorf("error should mention generated tests, got %q", err.Error())
		}
		if res != nil && res.Success {
			t.Error("result.Success should be false when test compilation fails")
		}
	})
}
