package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// readOnlyProvider reads on every call and never writes; it keeps every user
// text message each request carried.
type readOnlyProvider struct {
	*MockLLMClient
	readTool string
	calls    int
	texts    [][]string
}

func (p *readOnlyProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.calls++
	var texts []string
	for _, m := range history {
		if m.Role == "user" && m.Text != "" {
			texts = append(texts, m.Text)
		}
	}
	p.texts = append(p.texts, texts)
	if p.calls > 30 {
		return &types.LLMToolResponse{Text: "giving up"}, nil
	}
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "r", Name: p.readTool, Input: map[string]any{"path": "main.go"}}}}, nil
}

// N17 (found in R1-5's coverage round, 2026-09-19): in a repair round under
// the commit regime, each re-sent demand ended with "Reading is closed for this
// task..." twice -- the repair loop appended it to the prompt, and the round's
// re-send appended it to that prompt again. Every message the model is sent
// carries the sentence at most once, and the re-sent demand still carries it.
// Drives the build repair loop with a model that only reads, so every attempt
// re-sends under the commit regime.
func TestRepairLoop_UnderTheCommitRegimeTheSentenceIsSentOncePerMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a throwaway package")
	}
	ws := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module regimeprobe\n\ngo 1.21\n",
		"main.go": "package main\n\nimport \"os\"\n\nfunc main() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const readTool = "regime_read_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: readTool, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "main.go: lines 1-5 of 5", nil },
	})
	client := &readOnlyProvider{MockLLMClient: &MockLLMClient{}, readTool: readTool}
	e := newWorkingLoopExecutor(t, client)
	e.config.WorkspaceRoot = ws
	e.config.VerifyBuildAfterEdits = true
	e.virtualStore = &testExecutiveStore{}
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix main.go", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "main.go"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeLoop)
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{readTool, "create_file"}}
	result := &ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"main.go"}}
	_, _, _ = e.verifyAndRepairBuild(ctx, client, "system", nil, nil, e.buildToolDefinitions(cfg), cfg, result)

	const sentence = "Reading is closed for this task"
	carried := 0
	for i, texts := range client.texts {
		for _, text := range texts {
			n := strings.Count(text, sentence)
			if n > 1 {
				t.Errorf("request %d carried a message with the regime sentence %d times:\n%s", i+1, n, text)
			}
			if n == 1 {
				carried++
			}
		}
	}
	if carried == 0 {
		t.Errorf("no request under the commit regime carried the regime sentence (%d requests)", client.calls)
	}
}
