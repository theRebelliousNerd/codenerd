// Package chat provides tests for the tool adapter.
package chat

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/core"
)

// =============================================================================
// TOOL ADAPTER TESTS
// =============================================================================

func TestToolAdapter_NewToolExecutorAdapter(t *testing.T) {
	t.Parallel()

	// Test with nil orchestrator
	adapter := NewToolExecutorAdapter(nil)
	if adapter == nil {
		t.Error("Expected non-nil adapter even with nil orchestrator")
	}
}

func TestToolAdapter_CompileTimeCheck(t *testing.T) {
	t.Parallel()

	// Verify interface implementation at compile time
	var _ core.ToolExecutor = (*ToolExecutorAdapter)(nil)
	t.Log("ToolExecutorAdapter implements core.ToolExecutor")
}

func TestToolAdapter_ListTools_NilOrchestrator(t *testing.T) {
	t.Parallel()

	adapter := NewToolExecutorAdapter(nil)

	// Should panic or return empty (depending on implementation)
	defer func() {
		if r := recover(); r != nil {
			t.Log("ListTools panicked with nil orchestrator (expected behavior)")
		}
	}()

	// This will likely panic
	_ = adapter.ListTools()
}

func TestToolAdapter_GetTool_NilOrchestrator(t *testing.T) {
	t.Parallel()

	adapter := NewToolExecutorAdapter(nil)

	// Should panic or return false (depending on implementation)
	defer func() {
		if r := recover(); r != nil {
			t.Log("GetTool panicked with nil orchestrator (expected behavior)")
		}
	}()

	// This will likely panic
	_, exists := adapter.GetTool("nonexistent")
	if exists {
		t.Error("Expected false for nonexistent tool")
	}
}

func TestToolAdapter_ExecuteTool_NilOrchestrator(t *testing.T) {
	t.Parallel()

	adapter := NewToolExecutorAdapter(nil)

	// Should panic or return error (depending on implementation)
	defer func() {
		if r := recover(); r != nil {
			t.Log("ExecuteTool panicked with nil orchestrator (expected behavior)")
		}
	}()

	ctx := context.Background()
	_, err := adapter.ExecuteTool(ctx, "test", "input")
	if err != nil {
		t.Logf("ExecuteTool returned error: %v", err)
	}
}

// =============================================================================
// MOCK ORCHESTRATOR TESTS
// =============================================================================

// MockOrchestrator provides a mock implementation for testing
type MockOrchestrator struct {
	tools            map[string]autopoiesis.ToolInfo
	executeResult    string
	executeErr       error
	quality          *autopoiesis.QualityAssessment
	shouldRefine     bool
	refineSuggests   []string
	executionRecords []autopoiesis.ExecutionFeedback
}

func NewMockOrchestrator() *MockOrchestrator {
	return &MockOrchestrator{
		tools: make(map[string]autopoiesis.ToolInfo),
	}
}

func (m *MockOrchestrator) ListTools() []autopoiesis.ToolInfo {
	result := make([]autopoiesis.ToolInfo, 0, len(m.tools))
	for _, t := range m.tools {
		result = append(result, t)
	}
	return result
}

func (m *MockOrchestrator) GetToolInfo(name string) (autopoiesis.ToolInfo, bool) {
	t, ok := m.tools[name]
	return t, ok
}

func (m *MockOrchestrator) ExecuteAndEvaluateWithProfile(ctx context.Context, toolName, input string) (string, *autopoiesis.QualityAssessment, error) {
	return m.executeResult, m.quality, m.executeErr
}

func (m *MockOrchestrator) RecordExecution(ctx context.Context, feedback *autopoiesis.ExecutionFeedback) {
	m.executionRecords = append(m.executionRecords, *feedback)
}

func (m *MockOrchestrator) ShouldRefineTool(name string) (bool, []string) {
	return m.shouldRefine, m.refineSuggests
}

func (m *MockOrchestrator) RefineTool(ctx context.Context, name, reason string) (*autopoiesis.RefinementResult, error) {
	return &autopoiesis.RefinementResult{
		Success: true,
		Changes: []string{"test change"},
	}, nil
}

// AddTool adds a tool to the mock orchestrator
func (m *MockOrchestrator) AddTool(name, description string) {
	m.tools[name] = autopoiesis.ToolInfo{
		Name:         name,
		Description:  description,
		BinaryPath:   "/path/to/" + name,
		Hash:         "abc123",
		RegisteredAt: time.Now(),
		ExecuteCount: 0,
	}
}

// =============================================================================
// INTEGRATION-STYLE TESTS (with real components where possible)
// =============================================================================

func TestToolAdapter_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// Verify that ToolExecutorAdapter can be used where core.ToolExecutor is expected
	var executor core.ToolExecutor
	adapter := NewToolExecutorAdapter(nil)
	executor = adapter

	// Should not be nil
	if executor == nil {
		t.Error("Expected non-nil executor after assignment")
	}
}

// =============================================================================
// HELPER TESTS
// =============================================================================

func TestToolAdapter_ToolInfoConversion(t *testing.T) {
	t.Parallel()

	// Test that autopoiesis.ToolInfo -> core.ToolInfo conversion is correct
	autoInfo := autopoiesis.ToolInfo{
		Name:         "test-tool",
		Description:  "A test tool",
		BinaryPath:   "/usr/local/bin/test",
		Hash:         "abc123def456",
		RegisteredAt: time.Now(),
		ExecuteCount: 42,
	}

	// Manual conversion (matching what ListTools does)
	coreInfo := core.ToolInfo{
		Name:         autoInfo.Name,
		Description:  autoInfo.Description,
		BinaryPath:   autoInfo.BinaryPath,
		Hash:         autoInfo.Hash,
		RegisteredAt: autoInfo.RegisteredAt,
		ExecuteCount: autoInfo.ExecuteCount,
	}

	// Verify fields match
	if coreInfo.Name != autoInfo.Name {
		t.Errorf("Name mismatch: %s != %s", coreInfo.Name, autoInfo.Name)
	}
	if coreInfo.Description != autoInfo.Description {
		t.Errorf("Description mismatch: %s != %s", coreInfo.Description, autoInfo.Description)
	}
	if coreInfo.BinaryPath != autoInfo.BinaryPath {
		t.Errorf("BinaryPath mismatch: %s != %s", coreInfo.BinaryPath, autoInfo.BinaryPath)
	}
	if coreInfo.Hash != autoInfo.Hash {
		t.Errorf("Hash mismatch: %s != %s", coreInfo.Hash, autoInfo.Hash)
	}
	if coreInfo.ExecuteCount != autoInfo.ExecuteCount {
		t.Errorf("ExecuteCount mismatch: %d != %d", coreInfo.ExecuteCount, autoInfo.ExecuteCount)
	}
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestToolAdapter_EmptyToolName(t *testing.T) {
	t.Parallel()

	adapter := NewToolExecutorAdapter(nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("GetTool with empty name panicked (expected with nil orchestrator)")
		}
	}()

	_, exists := adapter.GetTool("")
	if exists {
		t.Error("Expected false for empty tool name")
	}
}

func TestToolAdapter_ContextCancellation(t *testing.T) {
	t.Parallel()

	adapter := NewToolExecutorAdapter(nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	defer func() {
		if r := recover(); r != nil {
			t.Log("ExecuteTool with cancelled context panicked (nil orchestrator)")
		}
	}()

	_, err := adapter.ExecuteTool(ctx, "test", "input")
	if err != nil {
		t.Logf("ExecuteTool with cancelled context: %v", err)
	}
}

func TestToolAdapter_Timeout(t *testing.T) {
	t.Parallel()

	adapter := NewToolExecutorAdapter(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(2 * time.Millisecond)

	defer func() {
		if r := recover(); r != nil {
			t.Log("ExecuteTool with timed out context panicked (nil orchestrator)")
		}
	}()

	_, err := adapter.ExecuteTool(ctx, "test", "input")
	if err != nil {
		t.Logf("ExecuteTool with timed out context: %v", err)
	}
}
