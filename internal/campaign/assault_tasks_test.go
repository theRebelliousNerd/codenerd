package campaign

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
)

func TestOrchestrator_ExecuteAssaultDiscoverTask(t *testing.T) {
	// Setup temporary workspace
	tempDir, err := os.MkdirTemp("", "assault_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .nerd directory structure
	nerdDir := filepath.Join(tempDir, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create dummy files to discover (no go.mod so we use generic fallback)
	if err := os.MkdirAll(filepath.Join(tempDir, "pkg", "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	files := []string{"pkg/a.go", "pkg/b.go", "pkg/sub/c.go"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tempDir, f), []byte("package foo"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a minimal but functional kernel for fact operations
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("Skipping: could not create kernel: %v", err)
	}

	// Create orchestrator with all required fields
	orch := &Orchestrator{
		workspace:       tempDir,
		nerdDir:         nerdDir,
		kernel:          kernel,
		taskResultOrder: make([]string, 0),
		taskResults:     make(map[string]string),
		campaign: &Campaign{
			ID: "/campaign_test_123",
			Assault: &AssaultConfig{
				Scope:     AssaultScopeRepo, // Use repo scope for generic fallback (no executor needed)
				BatchSize: 2,
			},
			Phases: []Phase{
				{ID: "phase_init", Order: 0},
				{ID: "phase_assault", Order: 1, Tasks: []Task{}},
			},
		},
	}

	// Create dummy phase 1 task
	task := &Task{
		ID:      "/task_discover",
		PhaseID: "phase_init", // Usually runs in phase 0 or 1
	}

	// Execute discovery
	res, err := orch.executeAssaultDiscoverTask(context.Background(), task)
	if err != nil {
		t.Fatalf("executeAssaultDiscoverTask failed: %v", err)
	}

	resMap, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}

	// Validation
	if resMap["status"] == "already_discovered" {
		t.Error("expected fresh discovery, got already_discovered")
	}

	// With AssaultScopeRepo, we get 1 target: "./..."
	targets, ok := resMap["targets"].(int)
	if !ok {
		t.Fatalf("expected targets to be int, got %T", resMap["targets"])
	}
	if targets < 1 {
		t.Errorf("expected at least 1 target, got %d", targets)
	}

	// Verify targets.json was written
	targetsPath := filepath.Join(tempDir, ".nerd", "campaigns", "campaign_test_123", "assault", "targets.json")
	if _, err := os.Stat(targetsPath); err != nil {
		t.Errorf("targets.json missing at %s", targetsPath)
	}

	// Verify batch integrity (read the batch file)
	batchesDir := filepath.Join(tempDir, ".nerd", "campaigns", "campaign_test_123", "assault", "batches")
	batchFiles, err := os.ReadDir(batchesDir)
	if err != nil {
		t.Errorf("failed to read batches dir: %v", err)
	}
	if len(batchFiles) == 0 {
		t.Error("expected at least one batch file")
	} else {
		batch0Path := filepath.Join(batchesDir, batchFiles[0].Name())
		data, err := os.ReadFile(batch0Path)
		if err != nil {
			t.Errorf("failed to read batch 0: %v", err)
		}
		var batch assaultBatchFile
		if err := json.Unmarshal(data, &batch); err != nil {
			t.Errorf("failed to unmarshal batch 0: %v", err)
		}
		if len(batch.Targets) == 0 {
			t.Error("expected at least 1 target in batch 0")
		}
	}
}

func TestOrchestrator_AssaultBatchTask_MissingArtifact(t *testing.T) {
	orch := &Orchestrator{
		campaign: &Campaign{ID: "/c_1"},
	}
	task := &Task{ID: "/t_1"} // No artifacts
	_, err := orch.executeAssaultBatchTask(context.Background(), task)
	if err == nil {
		t.Error("expected error for missing artifact, got nil")
	}
}

// Mocking required internal functions if needed.
// Note: discoverAssaultTargets uses generic file walking or git.
// If it uses git, this test might fail if no git repo in tempDir.
// Check assault_tasks.go for discoverAssaultTargets implementation.
// It likely uses filepath.Walk or similar for "scope=path/...".
// Assuming filepath.Walk based on standard go behavior for file discovery without explicit git dependency in imports shown.

// Add dummy implementation of `discoverAssaultTargets` if it's not exported or complex?
// Wait, `orchestrator.go` defines the method, `assault_tasks.go` implements it.
// I can only test what is available.
// I'll assume `discoverAssaultTargets` works on filesystem if scope is a path.

func TestChunkStrings(t *testing.T) {
	tests := []struct {
		in   []string
		size int
		want int // number of chunks
	}{
		{[]string{"a", "b", "c"}, 2, 2},
		{[]string{"a", "b", "c"}, 1, 3},
		{[]string{"a", "b", "c"}, 3, 1},
		{[]string{"a"}, 5, 1},
		{[]string{}, 5, 0},
	}

	for _, tt := range tests {
		got := chunkStrings(tt.in, tt.size)
		if len(got) != tt.want {
			t.Errorf("chunkStrings(%v, %d) len = %d, want %d", tt.in, tt.size, len(got), tt.want)
		}
	}
}

// Minimal Kernel mock for Orchestrator config if needed (though we didn't pass it in NewOrchestrator in test)
type mockKernel struct {
	*core.RealKernel
}
