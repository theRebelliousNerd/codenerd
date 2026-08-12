package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/session"
)

// TestTypeRoutedContextInjection_FileDocument covers the path that produced the
// fabricated audit report: a /document (and /file_create) task routed by type
// through executeFileTask. It verifies that ContextFrom dependencies are
// injected into the shard input via buildTaskInput, and that an empty
// ContextFrom leaves the input unchanged.
//
// Both assertions are on the string handed to the executor (TaskRequest.Task),
// observed via a fake TaskExecutor that records its TaskRequest — the same
// observation technique used by the existing checkpoint tests.
func TestTypeRoutedContextInjection_FileDocument(t *testing.T) {
	t.Run("file task receives context from dependency", func(t *testing.T) {
		workspace := t.TempDir()
		var captured session.TaskRequest
		executor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req
				if idx := strings.Index(req.Task, "file:"); idx != -1 {
					rest := req.Task[idx+len("file:"):]
					fields := strings.Fields(rest)
					if len(fields) > 0 {
						rel := strings.TrimSpace(fields[0])
						full := filepath.Join(workspace, rel)
						_ = os.MkdirAll(filepath.Dir(full), 0o755)
						_ = os.WriteFile(full, []byte("stub"), 0o644)
					}
				}
				return "substantive file result with enough length to avoid trivial checks — created file successfully with detailed content", nil
			},
		}
		o := &Orchestrator{
			kernel:          &MockKernel{},
			workspace:       workspace,
			taskExecutor:    executor,
			taskResults:     map[string]string{},
			taskResultOrder: []string{},
			campaign:        &Campaign{ID: "/campaign_ctx_test"},
		}
		depID := "dep_task_1"
		depResult := "FINDINGS from dep: critical nil-deref at internal/world/world.go:88 — must be cited in final report"
		o.taskResults[depID] = depResult
		o.taskResultOrder = append(o.taskResultOrder, depID)

		task := &Task{
			ID:          "task_file_1",
			PhaseID:     "phase_0",
			Type:        TaskTypeFileCreate,
			Description: "Create final audit report at docs/audit_report.md",
			Artifacts:   []TaskArtifact{{Type: "/doc", Path: "docs/audit_report.md"}},
			ContextFrom: []string{depID},
		}

		_, err := o.executeTask(context.Background(), task)
		if err != nil {
			t.Fatalf("executeTask() error = %v", err)
		}
		if captured.Task == "" {
			t.Fatal("executor was not called or TaskRequest.Task empty")
		}
		if !strings.Contains(captured.Task, "=== CONTEXT FROM TASK "+depID+" ===") {
			t.Errorf("shard input should contain context header for dep %q; got %q", depID, captured.Task)
		}
		if !strings.Contains(captured.Task, depResult) {
			t.Errorf("shard input should contain dependency result %q; got %q", depResult, captured.Task)
		}
		if !strings.Contains(captured.Task, "Create final audit report") {
			t.Errorf("shard input should still contain original description; got %q", captured.Task)
		}
		if !strings.Contains(captured.Task, "docs/audit_report.md") {
			t.Errorf("shard input should preserve file routing; got %q", captured.Task)
		}
	})

	t.Run("document task receives context from dependency", func(t *testing.T) {
		workspace := t.TempDir()
		var captured session.TaskRequest
		executor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req
				if idx := strings.Index(req.Task, "file:"); idx != -1 {
					rest := req.Task[idx+len("file:"):]
					fields := strings.Fields(rest)
					if len(fields) > 0 {
						rel := strings.TrimSpace(fields[0])
						full := filepath.Join(workspace, rel)
						_ = os.MkdirAll(filepath.Dir(full), 0o755)
						_ = os.WriteFile(full, []byte("stub"), 0o644)
					}
				}
				return "substantive document result with enough length to be considered real deliverable content for the test", nil
			},
		}
		o := &Orchestrator{
			kernel:          &MockKernel{},
			workspace:       workspace,
			taskExecutor:    executor,
			taskResults:     map[string]string{},
			taskResultOrder: []string{},
			campaign:        &Campaign{ID: "/campaign_ctx_test_doc"},
		}
		depID := "dep_task_doc"
		depResult := "RESEARCH FINDINGS: internal/session executor loop analysis — see span 42"
		o.taskResults[depID] = depResult
		o.taskResultOrder = append(o.taskResultOrder, depID)

		task := &Task{
			ID:          "task_doc_1",
			PhaseID:     "phase_0",
			Type:        TaskTypeDocument,
			Description: "Generate the consolidated audit report",
			Artifacts:   []TaskArtifact{{Type: "/doc", Path: "docs/final_report.md"}},
			ContextFrom: []string{depID},
		}
		_, err := o.executeTask(context.Background(), task)
		if err != nil {
			t.Fatalf("executeTask() error = %v", err)
		}
		if !strings.Contains(captured.Task, "=== CONTEXT FROM TASK "+depID+" ===") {
			t.Errorf("document shard input should contain context header; got %q", captured.Task)
		}
		if !strings.Contains(captured.Task, depResult) {
			t.Errorf("document shard input should contain dependency result; got %q", captured.Task)
		}
	})

	t.Run("file task with empty ContextFrom is unchanged", func(t *testing.T) {
		workspace := t.TempDir()
		var captured session.TaskRequest
		executor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req
				if idx := strings.Index(req.Task, "file:"); idx != -1 {
					rest := req.Task[idx+len("file:"):]
					fields := strings.Fields(rest)
					if len(fields) > 0 {
						rel := strings.TrimSpace(fields[0])
						full := filepath.Join(workspace, rel)
						_ = os.MkdirAll(filepath.Dir(full), 0o755)
						_ = os.WriteFile(full, []byte("stub"), 0o644)
					}
				}
				return "substantive file result with enough length to avoid trivial checks and pass", nil
			},
		}
		o := &Orchestrator{
			kernel:          &MockKernel{},
			workspace:       workspace,
			taskExecutor:    executor,
			taskResults:     map[string]string{},
			taskResultOrder: []string{},
			campaign:        &Campaign{ID: "/campaign_ctx_control"},
		}
		task := &Task{
			ID:          "task_file_control",
			PhaseID:     "phase_0",
			Type:        TaskTypeFileCreate,
			Description: "Create docs/audit_report.md with placeholder",
			Artifacts:   []TaskArtifact{{Type: "/doc", Path: "docs/audit_report.md"}},
			ContextFrom: []string{},
		}
		_, err := o.executeTask(context.Background(), task)
		if err != nil {
			t.Fatalf("executeTask() error = %v", err)
		}
		if strings.Contains(captured.Task, "=== CONTEXT FROM TASK") {
			t.Errorf("shard input with empty ContextFrom should not contain context block; got %q", captured.Task)
		}
		expectedPrefix := "create file:docs/audit_report.md Create docs/audit_report.md with placeholder"
		if captured.Task != expectedPrefix {
			t.Errorf("control: expected shard input %q, got %q", expectedPrefix, captured.Task)
		}
	})
}

func TestTypeRoutedContextInjection_Research(t *testing.T) {
	t.Run("research task receives context from dependency", func(t *testing.T) {
		var captured session.TaskRequest
		executor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req
				return "Substantive research findings: analysis of internal/world shows no nil deref at line 88, with file+symbol anchors and detailed evidence that clears the persistence threshold.", nil
			},
		}
		o := &Orchestrator{
			kernel:          &MockKernel{},
			workspace:       t.TempDir(),
			taskExecutor:    executor,
			taskResults:     map[string]string{},
			taskResultOrder: []string{},
			campaign:        &Campaign{ID: "/campaign_research_ctx"},
		}
		depID := "dep_research_1"
		depResult := "Prior audit: internal/session findings — context cancellation propagated"
		o.taskResults[depID] = depResult
		o.taskResultOrder = append(o.taskResultOrder, depID)

		task := &Task{
			ID:          "task_research_1",
			PhaseID:     "phase_0",
			Type:        TaskTypeResearch,
			Description: "Research internal/world for nil panics and lifecycle issues",
			ContextFrom: []string{depID},
		}
		_, err := o.executeTask(context.Background(), task)
		if err != nil {
			t.Fatalf("executeTask() error = %v", err)
		}
		if !strings.Contains(captured.Task, "=== CONTEXT FROM TASK "+depID+" ===") {
			t.Errorf("research shard input should contain context header; got %q", captured.Task)
		}
		if !strings.Contains(captured.Task, depResult) {
			t.Errorf("research shard input should contain dep result; got %q", captured.Task)
		}
		if !strings.Contains(captured.Task, "Research internal/world") {
			t.Errorf("research shard input should contain original description; got %q", captured.Task)
		}
	})

	t.Run("research task with empty ContextFrom is unchanged", func(t *testing.T) {
		var captured session.TaskRequest
		executor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req
				return "Substantive research findings that are long enough to be persisted and not retried via the hollow guard, with realistic detail.", nil
			},
		}
		o := &Orchestrator{
			kernel:          &MockKernel{},
			workspace:       t.TempDir(),
			taskExecutor:    executor,
			taskResults:     map[string]string{},
			taskResultOrder: []string{},
			campaign:        &Campaign{ID: "/campaign_research_control"},
		}
		task := &Task{
			ID:          "task_research_control",
			PhaseID:     "phase_0",
			Type:        TaskTypeResearch,
			Description: "Research internal/world for nil panics",
			ContextFrom: []string{},
		}
		_, err := o.executeTask(context.Background(), task)
		if err != nil {
			t.Fatalf("executeTask() error = %v", err)
		}
		if strings.Contains(captured.Task, "=== CONTEXT FROM TASK") {
			t.Errorf("empty ContextFrom should not inject context; got %q", captured.Task)
		}
		if captured.Task != "Research internal/world for nil panics" {
			t.Errorf("expected exact description passthrough; got %q", captured.Task)
		}
	})
}

func TestTypeRoutedContextInjection_TestHandlers(t *testing.T) {
	t.Run("test_write receives context", func(t *testing.T) {
		var captured session.TaskRequest
		executor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req
				return "generated tests for package with substantive output", nil
			},
		}
		o := &Orchestrator{
			kernel:          &MockKernel{},
			workspace:       t.TempDir(),
			taskExecutor:    executor,
			taskResults:     map[string]string{},
			taskResultOrder: []string{},
			campaign:        &Campaign{ID: "/campaign_test_write"},
		}
		depID := "dep_test_write"
		depResult := "Prior discovery: internal/campaign has 42 files needing test coverage"
		o.taskResults[depID] = depResult
		o.taskResultOrder = append(o.taskResultOrder, depID)

		task := &Task{
			ID:          "task_test_write_1",
			PhaseID:     "phase_0",
			Type:        TaskTypeTestWrite,
			Description: "Write tests for internal/campaign orchestrator",
			Artifacts:   []TaskArtifact{{Type: "/file", Path: "internal/campaign/foo.go"}},
			ContextFrom: []string{depID},
		}
		_, err := o.executeTask(context.Background(), task)
		if err != nil {
			t.Fatalf("executeTask() error = %v", err)
		}
		if !strings.Contains(captured.Task, "=== CONTEXT FROM TASK "+depID+" ===") {
			t.Errorf("test_write shard input should contain context header; got %q", captured.Task)
		}
		if !strings.Contains(captured.Task, depResult) {
			t.Errorf("test_write shard input should contain dep result; got %q", captured.Task)
		}
		if !strings.Contains(captured.Task, "generate_tests") {
			t.Errorf("test_write shard input should preserve tester prefix; got %q", captured.Task)
		}
	})

	t.Run("test_run receives context", func(t *testing.T) {
		var captured session.TaskRequest
		executor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req
				return "tests passed: 12 passed, 0 failed", nil
			},
		}
		o := &Orchestrator{
			kernel:          &MockKernel{},
			workspace:       t.TempDir(),
			taskExecutor:    executor,
			taskResults:     map[string]string{},
			taskResultOrder: []string{},
			campaign:        &Campaign{ID: "/campaign_test_run"},
		}
		depID := "dep_test_run"
		depResult := "Write tests produced 12 new tests for orchestrator"
		o.taskResults[depID] = depResult
		o.taskResultOrder = append(o.taskResultOrder, depID)

		task := &Task{
			ID:          "task_test_run_1",
			PhaseID:     "phase_0",
			Type:        TaskTypeTestRun,
			Description: "Run tests for ./internal/campaign/...",
			Artifacts:   []TaskArtifact{{Type: "/file", Path: "./internal/campaign/..."}},
			ContextFrom: []string{depID},
		}
		_, err := o.executeTask(context.Background(), task)
		if err != nil {
			t.Fatalf("executeTask() error = %v", err)
		}
		if !strings.Contains(captured.Task, "=== CONTEXT FROM TASK "+depID+" ===") {
			t.Errorf("test_run shard input should contain context header; got %q", captured.Task)
		}
		if !strings.Contains(captured.Task, depResult) {
			t.Errorf("test_run shard input should contain dep result; got %q", captured.Task)
		}
		if !strings.Contains(captured.Task, "run_tests") {
			t.Errorf("test_run shard input should preserve tester prefix; got %q", captured.Task)
		}
	})

	t.Run("test handlers with empty ContextFrom do not inject", func(t *testing.T) {
		var captured session.TaskRequest
		executor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				captured = req
				return "tests passed", nil
			},
		}
		o := &Orchestrator{
			kernel:          &MockKernel{},
			workspace:       t.TempDir(),
			taskExecutor:    executor,
			taskResults:     map[string]string{},
			taskResultOrder: []string{},
			campaign:        &Campaign{ID: "/campaign_test_control"},
		}
		task := &Task{
			ID:          "task_test_control",
			PhaseID:     "phase_0",
			Type:        TaskTypeTestWrite,
			Artifacts:   []TaskArtifact{{Type: "/file", Path: "internal/foo/bar.go"}},
			Description: "Write tests for bar",
		}
		_, err := o.executeTask(context.Background(), task)
		if err != nil {
			t.Fatalf("executeTask() error = %v", err)
		}
		if strings.Contains(captured.Task, "=== CONTEXT FROM TASK") {
			t.Errorf("empty ContextFrom should not inject; got %q", captured.Task)
		}
	})
}

// TestTypeRoutedContextInjection_MissingDep ensures that a ContextFrom entry
// whose result is not in taskResults does not produce an empty header — the
// context block is only emitted when a real result exists.
func TestTypeRoutedContextInjection_MissingDep(t *testing.T) {
	var captured session.TaskRequest
	executor := &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			captured = req
			return "Substantive research findings that are long enough to be persisted and not retried, with realistic detail.", nil
		},
	}
	o := &Orchestrator{
		kernel:          &MockKernel{},
		workspace:       t.TempDir(),
		taskExecutor:    executor,
		taskResults:     map[string]string{},
		taskResultOrder: []string{},
		campaign:        &Campaign{ID: "/campaign_missing_dep"},
	}
	task := &Task{
		ID:          "task_missing_dep",
		PhaseID:     "phase_0",
		Type:        TaskTypeResearch,
		Description: "Research internal/world",
		ContextFrom: []string{"nonexistent_dep"},
	}
	_, err := o.executeTask(context.Background(), task)
	if err != nil {
		t.Fatalf("executeTask() error = %v", err)
	}
	if strings.Contains(captured.Task, "=== CONTEXT FROM TASK") {
		t.Errorf("missing dep should not produce context block; got %q", captured.Task)
	}
	if captured.Task != "Research internal/world" {
		t.Errorf("expected passthrough description when dep missing; got %q", captured.Task)
	}
}
