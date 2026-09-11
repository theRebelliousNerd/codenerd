package session

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// repairScriptProvider answers the first repair round with a read and the
// second with a write, and keeps what each round offered and said.
type repairScriptProvider struct {
	*MockLLMClient
	readTool, writeTool string
	fixedContent        string
	calls               int
	catalogs            [][]string
	prompts             []string
}

func (p *repairScriptProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.calls++
	names := make([]string, 0, len(definitions))
	for _, def := range definitions {
		names = append(names, def.Name)
	}
	p.catalogs = append(p.catalogs, names)
	last := ""
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" && history[i].Text != "" {
			last = history[i].Text
			break
		}
	}
	p.prompts = append(p.prompts, last)
	if p.calls == 1 {
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "repair-read", Name: p.readTool, Input: map[string]any{"path": "main.go"}}}}, nil
	}
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "repair-write", Name: p.writeTool, Input: map[string]any{"path": "main.go", "content": p.fixedContent}}}}, nil
}

// A repair round that only reads is followed by one more with reading closed.
// Observed 2026-09-11: handed `"context" imported and not used` with its line
// number, the model spent its single repair round on twenty reads and no
// edit, and the turn ended on a broken build. The second round offers no read
// tool and carries the regime text; the model then edits, and the build is
// checked again.
func TestVerifyAndRepairBuild_ClosesReadingOnTheSecondRound(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a throwaway package")
	}
	ws := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module verifyprobe\n\ngo 1.21\n")
	write("main.go", "package main\n\nimport \"os\"\n\nfunc main() {}\n")
	const fixed = "package main\n\nfunc main() {}\n"

	const readTool = "repair_read_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: readTool, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "main.go: lines 1-5 of 5", nil },
	})
	// create_file counts as a write mutation (projectdoc.IsWriteMutationTool),
	// so the round that calls it is a round that wrote.
	const writeTool = "create_file"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectWrite, Name: writeTool, Category: tools.CategoryCode,
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			content, _ := args["content"].(string)
			return "written", os.WriteFile(filepath.Join(ws, "main.go"), []byte(content), 0o644)
		},
	})

	client := &repairScriptProvider{MockLLMClient: &MockLLMClient{}, readTool: readTool, writeTool: writeTool, fixedContent: fixed}
	e := newWorkingLoopExecutor(t, client)
	e.config.WorkspaceRoot = ws
	e.config.VerifyBuildAfterEdits = true
	e.virtualStore = &testExecutiveStore{}
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix main.go", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "main.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)

	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{readTool, writeTool}}
	toolDefs := e.buildToolDefinitions(cfg)
	result := &ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"main.go"}}

	if _, _, err := e.verifyAndRepairBuild(ctx, client, "system", nil, nil, toolDefs, cfg, result); err != nil {
		t.Fatalf("verifyAndRepairBuild: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("repair rounds = %d, want 2: an open round, then a closed one", client.calls)
	}
	if !slices.Contains(client.catalogs[0], readTool) {
		t.Fatalf("the first repair round must offer the read tool; offered %v", client.catalogs[0])
	}
	if slices.Contains(client.catalogs[1], readTool) || !slices.Contains(client.catalogs[1], writeTool) {
		t.Fatalf("the second repair round must withhold the read tool and keep the write tool; offered %v", client.catalogs[1])
	}
	if !strings.Contains(client.prompts[1], "Reading is closed for this task") || !strings.Contains(client.prompts[1], "imported and not used") {
		t.Fatalf("the second round must carry the compiler output and the regime; prompt: %q", client.prompts[1])
	}
	if got, _ := os.ReadFile(filepath.Join(ws, "main.go")); string(got) != fixed {
		t.Fatalf("main.go was not repaired: %q", got)
	}
	if !result.BuildCheck.OK {
		t.Fatalf("build check after repair = %+v, want OK", result.BuildCheck)
	}
	// A loop that leaves the repair path is back in its previous regime.
	if loop := activeWorkingLoop(ctx); loop.regime == commitRegime {
		t.Fatal("the commit regime must not outlive the repair round")
	}
}
