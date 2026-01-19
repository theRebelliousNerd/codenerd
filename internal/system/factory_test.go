package system

import (
	"context"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

// MockTaskExecutor for testing Cortex routing
type MockTaskExecutor struct {
	LastIntent string
}

func (m *MockTaskExecutor) Execute(ctx context.Context, intent string, task string) (string, error) {
	m.LastIntent = intent
	return "executed", nil
}

func (m *MockTaskExecutor) ExecuteWithContext(ctx context.Context, intent string, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	m.LastIntent = intent
	return "executed_with_context", nil
}

func (m *MockTaskExecutor) ExecuteAsync(ctx context.Context, intent string, task string) (string, error) {
	return "task_id", nil
}

func (m *MockTaskExecutor) GetResult(taskID string) (string, bool, error) {
	return "", false, nil
}

func (m *MockTaskExecutor) WaitForResult(ctx context.Context, taskID string) (string, error) {
	return "result", nil
}

func TestNormalizeShardTypeName(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"coder", "coder"},
		{"/coder", "coder"},
		{" system/scheduler ", "system/scheduler"},
	}

	for _, tc := range cases {
		if got := normalizeShardTypeName(tc.input); got != tc.expected {
			t.Errorf("normalizeShardTypeName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestCortex_SpawnTask_Routing(t *testing.T) {
	mockExec := &MockTaskExecutor{}
	cortex := &Cortex{
		TaskExecutor: mockExec,
	}

	// Test routing to TaskExecutor
	res, err := cortex.SpawnTask(context.Background(), "coder", "do something")
	if err != nil {
		t.Fatalf("SpawnTask failed: %v", err)
	}
	if res != "executed" {
		t.Errorf("Expected 'executed', got %s", res)
	}

	// Verify intent conversion
	// session.LegacyShardNameToIntent("coder") -> "/fix"
	if mockExec.LastIntent != "/fix" {
		t.Errorf("TaskExecutor.Execute called with intent %q, want \"/fix\"", mockExec.LastIntent)
	}
}

func TestCortex_SpawnTaskWithContext_Routing(t *testing.T) {
	mockExec := &MockTaskExecutor{}
	cortex := &Cortex{
		TaskExecutor: mockExec,
	}

	// Test routing
	res, err := cortex.SpawnTaskWithContext(context.Background(), "tester", "test it", nil, types.PriorityHigh)
	if err != nil {
		t.Fatalf("SpawnTaskWithContext failed: %v", err)
	}
	if res != "executed_with_context" {
		t.Errorf("Expected 'executed_with_context', got %s", res)
	}
}

func TestSessionVirtualStoreAdapter(t *testing.T) {
	// Adapter uses os package directly for ReadFile/WriteFile fallback
	// This tests the fallback logic in sessionVirtualStoreAdapter

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	adapter := &sessionVirtualStoreAdapter{vs: nil} // VS can be nil for ReadFile/WriteFile fallback

	// Test WriteFile
	content := []string{"line1", "line2"}
	if err := adapter.WriteFile(path, content); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test ReadFile
	readContent, err := adapter.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(readContent) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(readContent))
	}
	if readContent[0] != "line1" {
		t.Errorf("Expected line1, got %s", readContent[0])
	}
}
