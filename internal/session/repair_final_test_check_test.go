package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// Live 2026-11-03 (campaign task 4_2, attempt 3): the whole-repo suite failed
// on a sweep test in another package. Repair attempt 1 fixed the producer the
// sweep named, the sweep went green, and a test in the producer's own package
// that still asserted the old value went red. The recheck reported that red
// suite as the round's own subject, so the next prompt was seeded with the
// already-fixed sweep failure and the episode undid the fix. The edit that
// fixed the final failure must stand, and the next attempt must be told the
// suite it broke on the way.
func TestRepairFinalTestCheck_KeepsTheFixAndSeedsTheSuiteItBroke(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))

	sweep := "package sweep\n\nimport (\n\t\"os\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
		"func TestProducerPassesNames(t *testing.T) {\n" +
		"\tdata, err := os.ReadFile(\"../prod/prod.go\")\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n" +
		"\tif strings.Contains(string(data), `\"old\"`) {\n\t\tt.Fatal(\"prod.go passes the old value\")\n\t}\n}\n"
	prodTest := "package prod\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {\n\tif Value() != \"old\" {\n\t\tt.Fatalf(\"Value() = %q\", Value())\n\t}\n}\n"
	for rel, content := range map[string]string{
		"go.mod":              "module finalprobe\n\ngo 1.25\n",
		"sweep/sweep.go":      "package sweep\n",
		"sweep/sweep_test.go": sweep,
		"prod/prod.go":        "package prod\n\nfunc Value() string { return \"old\" }\n",
		"prod/prod_test.go":   prodTest,
	} {
		path := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var prompts []string
	mockLLM := &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{},
		CompleteWithToolResultsFunc: func(ctx context.Context, sys string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
			for i := len(history) - 1; i >= 0; i-- {
				if history[i].Role == "user" && history[i].Text != "" {
					prompts = append(prompts, history[i].Text)
					break
				}
			}
			prod, _ := os.ReadFile(filepath.Join(ws, "prod", "prod.go"))
			test, _ := os.ReadFile(filepath.Join(ws, "prod", "prod_test.go"))
			switch {
			case strings.Contains(string(prod), `"old"`):
				return &types.LLMToolResponse{Text: "fix the producer", ToolCalls: []types.ToolCall{{
					ID: "f1", Name: "edit_file",
					Input: map[string]any{"path": "prod/prod.go", "old_text": `"old"`, "new_text": `"new"`},
				}}}, nil
			case strings.Contains(string(test), `"old"`):
				return &types.LLMToolResponse{Text: "the test asserts the old value", ToolCalls: []types.ToolCall{{
					ID: "f2", Name: "edit_file",
					Input: map[string]any{"path": "prod/prod_test.go", "old_text": `"old"`, "new_text": `"new"`},
				}}}, nil
			default:
				return &types.LLMToolResponse{Text: "done"}, nil
			}
		},
	}
	executor := NewExecutor(realKernel(t), &testExecutiveStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.config.WorkspaceRoot = ws
	executor.config.EnableSafetyGate = false
	executor.config.RepairMaxAttempts = 3

	ctx, closeLoop, loopErr := executor.beginWorkingLoop(context.Background(), nil, "fix", &prompt.CompilationContext{ShardID: "probe"})
	if loopErr != nil {
		t.Fatalf("beginWorkingLoop: %v", loopErr)
	}
	t.Cleanup(closeLoop)

	final := verifyTests(ctx, ws, []string{"./..."})
	if final.Verdict() != VerifyFailed {
		t.Fatalf("seed suite verdict = %s, want failed:\n%s", final.Verdict(), final.Output)
	}
	result := &ExecutionResult{
		WrittenPaths:     []string{"prod/prod.go", "prod/prod_test.go"},
		WriteToolEffects: 2,
		FinalTestCheck:   &final,
	}
	ctx = WithTurnWriteSet(ctx, append([]string(nil), result.WrittenPaths...))
	history := []types.Message{{Role: "user", Text: "fix"}}
	allowCfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"edit_file", "read_file", "write_file"}}

	_, _, err := executor.repairFinalTestCheck(ctx, mockLLM, "", history, nil, allowCfg, result)
	if err != nil {
		t.Fatalf("repairFinalTestCheck: %v\nprompts:\n%s", err, strings.Join(prompts, "\n-----\n"))
	}
	if data, _ := os.ReadFile(filepath.Join(ws, "prod", "prod.go")); !strings.Contains(string(data), `"new"`) {
		t.Fatalf("the fix for the final failure was undone: %s", data)
	}
	if len(prompts) < 2 {
		t.Fatalf("prompts = %d, want a second attempt seeded by the broken suite", len(prompts))
	}
	second := prompts[1]
	if !strings.Contains(second, "TestValue") {
		t.Fatalf("second attempt was not seeded with the suite the fix broke:\n%s", second)
	}
	// The suite went red because of the fix, so the round must say so and
	// let the model fix whichever side is wrong. The final-failure prompt
	// forbids changing what a failing test expects, which leaves reverting
	// the fix as the only way to green.
	if !strings.Contains(second, "after your last edit") {
		t.Fatalf("second attempt used the final-failure prompt instead of the broke-tests prompt:\n%s", second)
	}
}
