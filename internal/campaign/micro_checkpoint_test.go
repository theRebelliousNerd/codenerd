package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"codenerd/internal/tactile"
)

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
        Type: TaskTypeFileModify,
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
        Type: TaskTypeFileModify,
        WriteSet: []string{"exists.txt"},
    }
	err = o.runTaskMicroCheckpoint(context.Background(), task)
	if err != nil {
		t.Fatalf("expected nil error for existing file, got %v", err)
	}
}

func TestMicroCheckpoint_AlternateFileExists(t *testing.T) {
    workspace := t.TempDir()
    // create under a subdirectory
    err := os.Mkdir(filepath.Join(workspace, "sub"), 0755)
    if err != nil {
        t.Fatal(err)
    }
    file := filepath.Join(workspace, "sub", "exists.txt")
    err = os.WriteFile(file, []byte(""), 0644)
    if err != nil {
        t.Fatal(err)
    }
	o := &Orchestrator{workspace: workspace}
	task := &Task{
        Type: TaskTypeFileModify,
        // asking for top level, but it exists in sub
        WriteSet: []string{"exists.txt"},
    }
	err = o.runTaskMicroCheckpoint(context.Background(), task)
	if err != nil {
		t.Fatalf("expected nil error for existing alternate file, got %v", err)
	}
}

func TestMicroCheckpoint_GoBuild(t *testing.T) {
    workspace := t.TempDir()
    file := filepath.Join(workspace, "exists.go")
    err := os.WriteFile(file, []byte(""), 0644)
    if err != nil {
        t.Fatal(err)
    }
    modFile := filepath.Join(workspace, "go.mod")
    err = os.WriteFile(modFile, []byte("module test"), 0644)
    if err != nil {
        t.Fatal(err)
    }

    mockExec := &mockTactileExecutor{
        res: &tactile.ExecutionResult{ExitCode: 0},
    }

	o := &Orchestrator{workspace: workspace, executor: mockExec}
	task := &Task{
        Type: TaskTypeFileModify,
        WriteSet: []string{"exists.go"},
    }
	err = o.runTaskMicroCheckpoint(context.Background(), task)
	if err != nil {
		t.Fatalf("expected nil error for successful go build, got %v", err)
	}
}

func TestMicroCheckpoint_GoBuild_Failed(t *testing.T) {
    workspace := t.TempDir()
    file := filepath.Join(workspace, "exists.go")
    err := os.WriteFile(file, []byte(""), 0644)
    if err != nil {
        t.Fatal(err)
    }
    modFile := filepath.Join(workspace, "go.mod")
    err = os.WriteFile(modFile, []byte("module test"), 0644)
    if err != nil {
        t.Fatal(err)
    }

    mockExec := &mockTactileExecutor{
        res: &tactile.ExecutionResult{ExitCode: 1, Stdout: "build failed"},
    }

	o := &Orchestrator{workspace: workspace, executor: mockExec}
	task := &Task{
        Type: TaskTypeFileModify,
        WriteSet: []string{"exists.go"},
    }
	err = o.runTaskMicroCheckpoint(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for failed go build, got nil")
	}
    if err != nil && err.Error() != "micro-checkpoint go build failed with exit code 1: build failed" {
        t.Fatalf("unexpected error message: %v", err)
    }
}

func TestMicroCheckpoint_GoBuild_ExecError(t *testing.T) {
    workspace := t.TempDir()
    file := filepath.Join(workspace, "exists.go")
    err := os.WriteFile(file, []byte(""), 0644)
    if err != nil {
        t.Fatal(err)
    }
    modFile := filepath.Join(workspace, "go.mod")
    err = os.WriteFile(modFile, []byte("module test"), 0644)
    if err != nil {
        t.Fatal(err)
    }

    mockExec := &mockTactileExecutor{
        err: fmt.Errorf("executor error"),
    }

	o := &Orchestrator{workspace: workspace, executor: mockExec}
	task := &Task{
        Type: TaskTypeFileModify,
        WriteSet: []string{"exists.go"},
    }
	err = o.runTaskMicroCheckpoint(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for executor error, got nil")
	}
}

func TestMicroCheckpoint_GoBuild_NoExecutor(t *testing.T) {
    workspace := t.TempDir()
    file := filepath.Join(workspace, "exists.go")
    err := os.WriteFile(file, []byte(""), 0644)
    if err != nil {
        t.Fatal(err)
    }
    modFile := filepath.Join(workspace, "go.mod")
    err = os.WriteFile(modFile, []byte("module test"), 0644)
    if err != nil {
        t.Fatal(err)
    }

	o := &Orchestrator{workspace: workspace}
	task := &Task{
        Type: TaskTypeFileModify,
        WriteSet: []string{"exists.go"},
    }
	err = o.runTaskMicroCheckpoint(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for missing executor, got nil")
	}
}
