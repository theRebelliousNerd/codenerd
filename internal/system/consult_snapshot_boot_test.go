package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

type consultSnapshotRequests struct {
	*MockLLMClient
	mu      sync.Mutex
	systems []string
}

func (p *consultSnapshotRequests) CompleteWithToolResults(_ context.Context, system string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.mu.Lock()
	p.systems = append(p.systems, system)
	p.mu.Unlock()
	return &types.LLMToolResponse{Text: "The view renders the status line."}, nil
}

func (p *consultSnapshotRequests) CompleteWithTools(ctx context.Context, system, _ string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return p.CompleteWithToolResults(ctx, system, nil, tools)
}

type consultSnapshotReader struct {
	workspace string
	mu        sync.Mutex
	seen      []string
}

func (r *consultSnapshotReader) SpawnConsultation(_ context.Context, _, _ string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Exercise the real CodeDOM parser and source read at the consultation
	// boundary. This is a controlled reader, not a live specialist model.
	facts, err := world.NewCodeElementFacts(r.workspace).FileFacts("view.go")
	if err != nil || len(facts) == 0 {
		return "", fmt.Errorf("consultation has no CodeDOM observations: %v", err)
	}
	file := filepath.Join(r.workspace, "view.go")
	body, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	r.seen = append(r.seen, string(body))
	if len(r.seen) == 1 {
		updated := strings.ReplaceAll(string(body), "ViewAtA", "ViewAtB")
		if err := os.WriteFile(file, []byte(updated), 0o644); err != nil {
			return "", err
		}
		return "OLD_REVISION_A_ADVICE: ViewAtA renders the status.", nil
	}
	return "CURRENT_REVISION_B_ADVICE: ViewAtB renders the status.", nil
}

// Real boot, sharded policy, expert selection, JIT, world/CodeDOM providers,
// and model-request assembly. The controlled consultant and serving provider
// isolate delivery from model quality and require no network inference.
func TestBootConsultSnapshot_DoesNotDeliverOldAdviceToTheNextModelRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	workspace := t.TempDir()
	dir := filepath.Join(workspace, ".nerd", "agents", "BubbleTeaExpert")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	atomYAML := strings.ReplaceAll(strings.ReplaceAll(teaExpertPromptsYAML, "TeaExpert", "BubbleTeaExpert"), "teaexpert", "bubbleteaexpert")
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(atomYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAgentsJSON(t, workspace, []map[string]any{{
		"name": "BubbleTeaExpert", "type": "persistent", "status": "ready", "domains": []string{"bubbletea"},
	}})
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/view\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := "package view\n\nimport _ \"github.com/charmbracelet/bubbletea\"\n\nfunc ViewAtA() string { return \"status\" }\n"
	if err := os.WriteFile(filepath.Join(workspace, "view.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &consultSnapshotRequests{MockLLMClient: &MockLLMClient{}}
	userConfig := config.DefaultUserConfig()
	userConfig.Embedding = &config.EmbeddingConfig{Provider: "none"}
	// Keep every model call on the injected serving client.
	userConfig.ShardProfiles = nil
	userConfig.DefaultShard = nil
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortexWithConfig(ctx, BootConfig{
		Workspace: workspace, UserConfigOverride: userConfig, LLMClientOverride: client,
	})
	if err != nil {
		t.Fatalf("BootCortexWithConfig: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, workspace)
	reader := &consultSnapshotReader{workspace: workspace}
	cortex.SessionExecutor.SetExpertConsultant(reader)
	for call := range 2 {
		client.mu.Lock()
		start := len(client.systems)
		client.mu.Unlock()
		if _, err := cortex.SessionExecutor.CloneForTask().ProcessWithIntent(ctx, "explain how the view renders the status",
			&perception.Intent{Verb: "/explain", Category: "/query", Target: "view.go"}); err != nil {
			// Delivery is checked independently of the final task verdict.
			t.Logf("turn %d verdict: %v", call+1, err)
		}
		client.mu.Lock()
		requests := append([]string(nil), client.systems[start:]...)
		client.mu.Unlock()
		want := "OLD_REVISION_A_ADVICE"
		if call == 1 {
			want = "CURRENT_REVISION_B_ADVICE"
		}
		delivered := false
		for _, system := range requests {
			if !strings.Contains(system, "## Shard agent consultation") {
				continue
			}
			delivered = true
			if !strings.Contains(system, want) || strings.Contains(system, "files unchanged since") ||
				(call == 1 && strings.Contains(system, "OLD_REVISION_A_ADVICE")) {
				t.Fatalf("turn %d received advice from the wrong revision: %s", call+1, system)
			}
		}
		if !delivered {
			t.Fatalf("turn %d never sent the consultation to the serving model", call+1)
		}
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if len(reader.seen) != 2 || !strings.Contains(reader.seen[0], "ViewAtA") || !strings.Contains(reader.seen[1], "ViewAtB") {
		t.Fatalf("consultant did not consume both revisions: %q", reader.seen)
	}
}
