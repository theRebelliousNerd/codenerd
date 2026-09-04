package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/session"
)

// TestFileTask_WriteSetTargetPath verifies that executeFileTask resolves the
// target from the declared WriteSet when no artifact is present, and passes
// that path to the coder shard.
func TestFileTask_WriteSetTargetPath(t *testing.T) {
	workspace := t.TempDir()
	var captured string
	calls := 0
	mockExec := &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			calls++
			captured = req.Task
			full := filepath.Join(workspace, "internal", "x", "y.go")
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatalf("setup: mkdir: %v", err)
			}
			if err := os.WriteFile(full, []byte("package x\n"), 0o644); err != nil {
				t.Fatalf("setup: write: %v", err)
			}
			return "wrote file", nil
		},
	}
	o := &Orchestrator{
		workspace:    workspace,
		taskExecutor: mockExec,
		campaign:     &Campaign{ID: "/campaign_test"},
	}
	task := &Task{
		ID:          "t1",
		PhaseID:     "p1",
		Description: "Create something with no path mentioned",
		Type:        TaskTypeFileCreate,
		WriteSet:    []string{"internal/x/y.go"},
	}
	res, err := o.executeFileTask(context.Background(), task)
	if err != nil {
		t.Fatalf("executeFileTask() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 shard call, got %d", calls)
	}
	if !strings.Contains(captured, "internal/x/y.go") {
		t.Fatalf("shard task %q does not contain write-set path %q", captured, "internal/x/y.go")
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	if m["path"] != "internal/x/y.go" {
		t.Fatalf("result path = %v, want %q", m["path"], "internal/x/y.go")
	}
}

// TestFileTask_WriteSetAbsoluteTargetPath verifies that an absolute WriteSet
// entry (the decomposer's normalized form) is relativized for the shard.
func TestFileTask_WriteSetAbsoluteTargetPath(t *testing.T) {
	workspace := t.TempDir()
	abs := filepath.Join(workspace, "internal", "x", "y.go")
	var captured string
	mockExec := &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			captured = req.Task
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				t.Fatalf("setup: mkdir: %v", err)
			}
			if err := os.WriteFile(abs, []byte("package x\n"), 0o644); err != nil {
				t.Fatalf("setup: write: %v", err)
			}
			return "wrote file", nil
		},
	}
	o := &Orchestrator{
		workspace:    workspace,
		taskExecutor: mockExec,
		campaign:     &Campaign{ID: "/campaign_test"},
	}
	task := &Task{
		ID:          "t1",
		PhaseID:     "p1",
		Description: "Create something with no path mentioned",
		Type:        TaskTypeFileCreate,
		WriteSet:    []string{abs},
	}
	res, err := o.executeFileTask(context.Background(), task)
	if err != nil {
		t.Fatalf("executeFileTask() error = %v", err)
	}
	if !strings.Contains(captured, "internal/x/y.go") {
		t.Fatalf("shard task %q does not contain relativized path %q", captured, "internal/x/y.go")
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	if m["path"] != "internal/x/y.go" {
		t.Fatalf("result path = %v, want %q", m["path"], "internal/x/y.go")
	}
}

// TestFileTask_EmptyTargetPathErrorsBeforeShard verifies that a file task with
// no artifact, no write set, and no extractable description path fails before
// any shard call is wasted.
func TestFileTask_EmptyTargetPathErrorsBeforeShard(t *testing.T) {
	workspace := t.TempDir()
	calls := 0
	mockExec := &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			calls++
			return "should not be called", nil
		},
	}
	o := &Orchestrator{
		workspace:    workspace,
		taskExecutor: mockExec,
		campaign:     &Campaign{ID: "/campaign_test"},
	}
	task := &Task{
		ID:          "t-empty",
		PhaseID:     "p1",
		Description: "Improve error handling",
		Type:        TaskTypeFileModify,
	}
	_, err := o.executeFileTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for pathless file task, got nil")
	}
	if !strings.Contains(err.Error(), "has no target path") {
		t.Fatalf("error %q does not contain %q", err.Error(), "has no target path")
	}
	if calls != 0 {
		t.Fatalf("expected 0 shard calls, got %d", calls)
	}
}

// TestFileTask_DirectoryStatIsNotSuccess verifies that a stat resolving to a
// directory never counts as file-task success: the handler must fall back
// instead of hollow-succeeding.
func TestFileTask_DirectoryStatIsNotSuccess(t *testing.T) {
	workspace := t.TempDir()
	dirPath := filepath.Join(workspace, "internal", "x", "dir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	mockExec := &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			// Succeed without writing a file; the directory already exists.
			return "done", nil
		},
	}
	o := &Orchestrator{
		workspace:    workspace,
		taskExecutor: mockExec,
		campaign:     &Campaign{ID: "/campaign_test"},
		llmClient: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
				return "", context.DeadlineExceeded
			},
		},
	}
	task := &Task{
		ID:          "t-dir",
		PhaseID:     "p1",
		Description: "Modify something with no path mentioned",
		Type:        TaskTypeFileModify,
		WriteSet:    []string{"internal/x/dir"},
	}
	_, err := o.executeFileTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected fallback error for directory target, got nil success (directory counted as success)")
	}
}

// TestRetype_PathlessFileModifyToResearch verifies the plan-time defense: a
// pathless /file_modify is retyped to the analytical type instead of being
// dispatched as a file task with nowhere to write.
func TestRetype_PathlessFileModifyToResearch(t *testing.T) {
	d := NewDecomposer(&MockKernel{}, &mockLLMClient{}, t.TempDir())
	campaign := d.buildCampaign("/campaign_test", DecomposeRequest{
		Goal:          "test",
		CampaignType:  CampaignTypeCustom,
		ContextBudget: 1000,
	}, &RawPlan{
		Title:      "Plan",
		Confidence: 0.9,
		Phases: []RawPhase{{
			Name:        "Phase 0",
			Category:    "/service",
			Description: "service",
			Tasks: []RawTask{
				{Description: "Improve error handling", Type: "/file_modify", Priority: "/high"},
			},
		}},
	})
	if len(campaign.Phases) != 1 || len(campaign.Phases[0].Tasks) != 1 {
		t.Fatalf("expected 1 task, got %#v", campaign.Phases)
	}
	got := campaign.Phases[0].Tasks[0].Type
	if got != TaskTypeResearch && got != TaskTypeVerify {
		t.Fatalf("pathless /file_modify was not retyped to analytical type, got %s", got)
	}
}

// TestTargetPath_ArtifactBeatsWriteSet verifies the resolution priority:
// Artifacts[0].Path wins over WriteSet.
func TestTargetPath_ArtifactBeatsWriteSet(t *testing.T) {
	o := &Orchestrator{workspace: t.TempDir()}
	task := &Task{
		ID:          "t1",
		Description: "Create internal/other/z.go",
		Type:        TaskTypeFileCreate,
		Artifacts:   []TaskArtifact{{Type: "/source_file", Path: "internal/a/b.go"}},
		WriteSet:    []string{"internal/x/y.go"},
	}
	if got := o.resolveFileTaskTargetPath(task); got != "internal/a/b.go" {
		t.Fatalf("resolveFileTaskTargetPath = %q, want %q", got, "internal/a/b.go")
	}
}
