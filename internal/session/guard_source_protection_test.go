package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

func TestGuardSourceWritesAreRefusedBeforeMutation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "nerd.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := projectdoc.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	paths := []string{"internal/perception/usage_track.go", "internal/broker/broker.go",
		"internal/system/knowledge_refresh_scope.go", "internal/system/knowledge_refresh_boot.go"}
	const before = "package guard\n\n// Owner-controlled source.\n"
	const after = "package guard\n\n// Changed source.\n"
	for _, path := range paths {
		full := filepath.Join(ws, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(before), 0644); err != nil {
			t.Fatal(err)
		}
	}
	kernel := &MockKernel{}
	if err := kernel.LoadFacts(doc.Facts()); err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))
	e := NewExecutor(kernel, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.config.WorkspaceRoot = ws
	// Isolate the shipped project-document gate, as in the existing protected
	// write fixture: a mock kernel cannot derive constitutional permission.
	// The positive write and protected reads must succeed through this same seam.
	e.config.EnableSafetyGate = false
	e.SetProjectDoc(doc)
	cfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "read_file"}}
	var calls []types.ToolCall
	for i, path := range paths {
		calls = append(calls, types.ToolCall{ID: fmt.Sprintf("guard-%d", i), Name: "write_file", Input: map[string]any{"path": path, "content": after}})
	}
	const ordinary = "internal/perception/ordinary.go"
	calls = append(calls, types.ToolCall{ID: "ordinary", Name: "write_file", Input: map[string]any{"path": ordinary, "content": after}})
	result := &ExecutionResult{}
	results, _ := e.executeToolBatch(context.Background(), calls, cfg, result)
	if len(results) != len(calls) {
		t.Fatalf("results=%+v", results)
	}
	for i, path := range paths {
		if !results[i].IsError || !strings.Contains(results[i].Content, "nerd.md") {
			t.Errorf("%s was not refused by its owner rule: %+v", path, results[i])
		}
		data, err := os.ReadFile(filepath.Join(ws, path))
		if err != nil || string(data) != before {
			t.Errorf("protected source mutated: %s: %q %v", path, data, err)
		}
		output, err := e.executeToolCall(context.Background(), ToolCall{Name: "read_file", Args: map[string]any{"path": path}}, cfg)
		if err != nil || !strings.Contains(output, "Owner-controlled source.") || strings.Contains(output, "Changed source.") {
			t.Errorf("protected read refused: %s: %q %v", path, output, err)
		}
	}
	if results[len(paths)].IsError || result.WriteToolEffects != 1 || len(result.WrittenPaths) != 1 || result.WrittenPaths[0] != ordinary {
		t.Fatalf("ordinary source must remain writable: results=%+v writes=%+v", results, result.WrittenPaths)
	}
}
