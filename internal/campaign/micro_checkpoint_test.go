package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/tactile"
)

// mockExecutor implements tactile.Executor for testing
type mockExecutor struct {
	tactile.Executor // embed to satisfy interface
	executeFunc      func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error)
}

func (m *mockExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, cmd)
	}
	return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "success"}, nil
}

func TestRunTaskMicroCheckpoint(t *testing.T) {
	workspace := t.TempDir()

	// Create some existing files
	subDir := filepath.Join(workspace, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	existingFile := filepath.Join(subDir, "existing.go")
	if err := os.WriteFile(existingFile, []byte("package sub"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	existingTxt := filepath.Join(subDir, "existing.txt")
	if err := os.WriteFile(existingTxt, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	tests := []struct {
		name        string
		task        *Task
		workspace   string
		setupFiles  func(ws string)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil task",
			task:        nil,
			workspace:   workspace,
			expectError: false,
		},
		{
			name:        "non-mutating task",
			task:        &Task{Type: TaskTypeResearch},
			workspace:   workspace,
			expectError: false,
		},
		{
			name:        "empty write set",
			task:        &Task{Type: TaskTypeFileModify, WriteSet: []string{}},
			workspace:   workspace,
			expectError: false,
		},
		{
			name: "paths do not exist",
			task: &Task{
				Type:     TaskTypeFileModify,
				WriteSet: []string{filepath.Join(workspace, "missing.txt")},
			},
			workspace:   workspace,
			expectError: true,
			errorMsg:    "none of the planned write_set paths exist",
		},
		{
			name: "exact path exists",
			task: &Task{
				Type:     TaskTypeFileModify,
				WriteSet: []string{existingTxt},
			},
			workspace:   workspace,
			expectError: false,
		},
		{
			name: "a same-named file elsewhere is not evidence",
			task: &Task{
				Type:     TaskTypeFileModify,
				WriteSet: []string{filepath.Join(workspace, "wrong_dir", "existing.txt")}, // sub/existing.txt exists; it is not this task's
			},
			workspace:   workspace,
			expectError: true,
			errorMsg:    "none of the planned write_set paths exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFiles != nil {
				tt.setupFiles(tt.workspace)
			}

			// Clean up go.mod after each test to avoid interference
			defer func() {
				_ = os.Remove(filepath.Join(tt.workspace, "go.mod"))
			}()

			o := &Orchestrator{
				workspace: tt.workspace,
			}

			err := o.runTaskMicroCheckpoint(context.Background(), tt.task)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

type mockTactileExecutor struct {
	res *tactile.ExecutionResult
	err error
}

func (m *mockTactileExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	return m.res, m.err
}

func (m *mockTactileExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}

func (m *mockTactileExecutor) Validate(cmd tactile.Command) error {
	return nil
}

func TestMicroCheckpoint_NilTask(t *testing.T) {
	o := &Orchestrator{}
	err := o.runTaskMicroCheckpoint(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil error for nil task, got %v", err)
	}
}

func TestMicroCheckpoint_NonMutatingTask(t *testing.T) {
	o := &Orchestrator{}
	task := &Task{Type: TaskTypeTestRun} // non-mutating
	err := o.runTaskMicroCheckpoint(context.Background(), task)
	if err != nil {
		t.Fatalf("expected nil error for non-mutating task, got %v", err)
	}
}

func TestMicroCheckpoint_EmptyWriteSet(t *testing.T) {
	o := &Orchestrator{workspace: "/tmp"}
	task := &Task{Type: TaskTypeFileModify, WriteSet: []string{}} // mutating, empty write_set
	err := o.runTaskMicroCheckpoint(context.Background(), task)
	if err != nil {
		t.Fatalf("expected nil error for empty write set, got %v", err)
	}
}

func TestMicroCheckpoint_MissingFile(t *testing.T) {
	workspace := t.TempDir()
	o := &Orchestrator{workspace: workspace}
	task := &Task{
		Type:     TaskTypeFileModify,
		WriteSet: []string{"doesnotexist.txt"},
	}
	err := o.runTaskMicroCheckpoint(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestMicroCheckpoint_FileExists(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "exists.txt")
	err := os.WriteFile(file, []byte(""), 0644)
	if err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{workspace: workspace}
	task := &Task{
		Type:     TaskTypeFileModify,
		WriteSet: []string{"exists.txt"},
	}
	err = o.runTaskMicroCheckpoint(context.Background(), task)
	if err != nil {
		t.Fatalf("expected nil error for existing file, got %v", err)
	}
}

// A file of the same name elsewhere in the workspace is not evidence the task
// did anything (sweep finding F12): a task whose write set is
// docs/features/README.md and whose attempt wrote nothing passed because some
// other README.md existed.
func TestMicroCheckpoint_ASameNamedFileElsewhereIsNotEvidence(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("the repo's readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{workspace: workspace}
	task := &Task{ID: "/task_readme", Type: TaskTypeDocument, WriteSet: []string{"docs/features/README.md"}}
	o.beginAttempt(task)
	if err := o.runTaskMicroCheckpoint(context.Background(), task); err == nil {
		t.Fatal("a task that wrote nothing passed because another README.md exists")
	}
}

// Where the attempt wrote is where its change landed, when the planned path
// was a guess (ladder C2): a write the attempt recorded, on disk, passes.
func TestMicroCheckpoint_TheAttemptsOwnWriteIsEvidence(t *testing.T) {
	workspace := t.TempDir()
	written := filepath.Join(workspace, "docs", "features", "INDEX.md")
	if err := os.MkdirAll(filepath.Dir(written), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(written, []byte("# index"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{workspace: workspace}
	task := &Task{ID: "/task_index", Type: TaskTypeDocument, WriteSet: []string{"docs/features/README.md"}}
	o.beginAttempt(task)
	o.recordAttemptWrites(task, []observation.FileWrite{{Path: written}})
	if err := o.runTaskMicroCheckpoint(context.Background(), task); err != nil {
		t.Fatalf("the attempt wrote %s and the gate refused it: %v", written, err)
	}
}
