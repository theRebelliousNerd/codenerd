package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tactile"
)

func TestFindWorkspaceFileByBase(t *testing.T) {
	// Setup temporary workspace
	workspace := t.TempDir()

	// Test empty cases
	if alt := findWorkspaceFileByBase("", "foo.txt"); alt != "" {
		t.Errorf("expected empty string for empty workspace, got %s", alt)
	}
	if alt := findWorkspaceFileByBase(workspace, ""); alt != "" {
		t.Errorf("expected empty string for empty base, got %s", alt)
	}

	// Create some files
	subDir := filepath.Join(workspace, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	targetFile := filepath.Join(subDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Test finding the file
	alt := findWorkspaceFileByBase(workspace, "target.txt")
	if alt != targetFile {
		t.Errorf("expected %s, got %s", targetFile, alt)
	}

	// Test skipping ignored directories
	ignoredDir := filepath.Join(workspace, ".git")
	if err := os.Mkdir(ignoredDir, 0755); err != nil {
		t.Fatalf("failed to create ignored subdir: %v", err)
	}
	ignoredFile := filepath.Join(ignoredDir, "ignored.txt")
	if err := os.WriteFile(ignoredFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create ignored file: %v", err)
	}

	if alt := findWorkspaceFileByBase(workspace, "ignored.txt"); alt != "" {
		t.Errorf("expected empty string for ignored file, got %s", alt)
	}
}

func TestHasGoFiles(t *testing.T) {
	tests := []struct {
		paths    []string
		expected bool
	}{
		{[]string{"foo.txt", "bar.js"}, false},
		{[]string{"foo.txt", "bar.go"}, true},
		{[]string{"foo.GO"}, true},
		{[]string{}, false},
	}

	for _, tt := range tests {
		if got := hasGoFiles(tt.paths); got != tt.expected {
			t.Errorf("hasGoFiles(%v) = %v; want %v", tt.paths, got, tt.expected)
		}
	}
}

// mockExecutor implements tactile.Executor for testing
type mockExecutor struct {
	tactile.Executor // embed to satisfy interface
	executeFunc func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error)
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
		executor    tactile.Executor
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
			errorMsg:    "none of planned write_set paths exist",
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
			name: "alternate path exists",
			task: &Task{
				Type:     TaskTypeFileModify,
				WriteSet: []string{filepath.Join(workspace, "wrong_dir", "existing.txt")}, // will find sub/existing.txt
			},
			workspace:   workspace,
			expectError: false,
		},
		{
			name: "go build succeeds",
			task: &Task{
				Type:     TaskTypeFileModify,
				WriteSet: []string{existingFile},
			},
			workspace: workspace,
			executor: &mockExecutor{
				executeFunc: func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
					return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "build ok"}, nil
				},
			},
			setupFiles: func(ws string) {
				goModFile := filepath.Join(ws, "go.mod")
				_ = os.WriteFile(goModFile, []byte("module example.com/test"), 0644)
			},
			expectError: false,
		},
		{
			name: "go build fails with exit code",
			task: &Task{
				Type:     TaskTypeFileModify,
				WriteSet: []string{existingFile},
			},
			workspace: workspace,
			executor: &mockExecutor{
				executeFunc: func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
					return &tactile.ExecutionResult{Success: true, ExitCode: 1, Stdout: "compile error"}, nil
				},
			},
			setupFiles: func(ws string) {
				goModFile := filepath.Join(ws, "go.mod")
				_ = os.WriteFile(goModFile, []byte("module example.com/test"), 0644)
			},
			expectError: true,
			errorMsg:    "go build failed with exit code 1",
		},
		{
			name: "go build fails with error",
			task: &Task{
				Type:     TaskTypeFileModify,
				WriteSet: []string{existingFile},
			},
			workspace: workspace,
			executor: &mockExecutor{
				executeFunc: func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
					return nil, fmt.Errorf("timeout")
				},
			},
			setupFiles: func(ws string) {
				goModFile := filepath.Join(ws, "go.mod")
				_ = os.WriteFile(goModFile, []byte("module example.com/test"), 0644)
			},
			expectError: true,
			errorMsg:    "micro-checkpoint go build failed",
		},
		{
			name: "no executor available for go build",
			task: &Task{
				Type:     TaskTypeFileModify,
				WriteSet: []string{existingFile},
			},
			workspace: workspace,
			executor: nil, // force nil executor
			setupFiles: func(ws string) {
				goModFile := filepath.Join(ws, "go.mod")
				_ = os.WriteFile(goModFile, []byte("module example.com/test"), 0644)
			},
			expectError: true,
			errorMsg:    "executor unavailable",
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
				executor:  tt.executor,
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
