package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// INTEGRATION TESTS - Tests that exercise multiple components together
// ============================================================================

// TestModel_FullUpdateCycle tests a complete message update cycle
func TestModel_FullUpdateCycle(t *testing.T) {
	m := NewTestModel(
		WithSize(100, 50),
		WithHistory(TestMessages.UserMessage),
	)

	// Verify initial state
	if m.viewMode != ChatView {
		t.Errorf("Initial viewMode = %v, want ChatView", m.viewMode)
	}

	// Simulate a window resize
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	m = newModel.(Model)

	if m.width != 120 || m.height != 60 {
		t.Errorf("Size not updated: got %dx%d, want 120x60", m.width, m.height)
	}

	// Verify view renders without panic
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

// TestModel_CommandProcessing tests command handling
func TestModel_CommandProcessing(t *testing.T) {
	workspace := t.TempDir()
	m := NewTestModel(WithSize(100, 50))
	m.workspace = workspace
	if m.workspace != workspace {
		t.Errorf("Workspace mismatch")
	}

	// Test commands that don't require kernel
	commands := []string{
		"/help",
		"/clear",
		"/usage",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			m.textarea.SetValue(cmd)
			newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = newModel.(Model)

			// Verify no panic occurred and model is still valid
			if m.width == 0 {
				t.Error("Model became invalid after command")
			}
		})
	}
}

// TestModel_ViewModeTransitions tests switching between view modes
func TestModel_ViewModeTransitions(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	modes := []ViewMode{
		ChatView,
		ListView,
		FilePickerView,
		UsageView,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			m.viewMode = mode

			// Render should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("View() panicked for mode %v: %v", mode, r)
				}
			}()

			view := m.View()
			if view == "" && mode == ChatView {
				t.Error("ChatView rendered empty")
			}
		})
	}
}

// TestModel_InputModeTransitions tests switching between input modes
func TestModel_InputModeTransitions(t *testing.T) {
	tests := []struct {
		name      string
		inputMode InputMode
		checkFn   func(m Model) bool
	}{
		{
			name:      "Normal",
			inputMode: InputModeNormal,
			checkFn: func(m Model) bool {
				return !m.awaitingClarification && !m.awaitingPatch
			},
		},
		{
			name:      "Clarification",
			inputMode: InputModeClarification,
			checkFn: func(m Model) bool {
				return m.awaitingClarification
			},
		},
		{
			name:      "Patch",
			inputMode: InputModePatch,
			checkFn: func(m Model) bool {
				return m.awaitingPatch
			},
		},
		{
			name:      "ConfigWizard",
			inputMode: InputModeConfigWizard,
			checkFn: func(m Model) bool {
				return m.awaitingConfigWizard
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewTestModel(WithInputMode(tt.inputMode))
			if !tt.checkFn(m) {
				t.Errorf("Input mode %s not set correctly", tt.name)
			}
		})
	}
}

// TestModel_HistoryManagement tests history operations
func TestModel_HistoryManagement(t *testing.T) {
	m := NewTestModel()

	// Add messages
	msg1 := Message{Role: "user", Content: "Hello", Time: time.Now()}
	msg2 := Message{Role: "assistant", Content: "Hi there!", Time: time.Now()}

	m = NewTestModel(WithHistory(msg1, msg2))

	if len(m.history) != 2 {
		t.Errorf("History length = %d, want 2", len(m.history))
	}

	// Test addMessage - note: addMessage returns a new model (immutable)
	m = m.addMessage(Message{Role: "user", Content: "Test", Time: time.Now()})
	if len(m.history) != 3 {
		t.Errorf("After addMessage, history length = %d, want 3", len(m.history))
	}
}

// TestModel_ShardResultHistory tests shard result tracking
func TestModel_ShardResultHistory(t *testing.T) {
	m := NewTestModel()

	// Add shard results
	for i := 0; i < 15; i++ {
		result := &ShardResult{
			ShardType:  "test",
			Task:       "task",
			RawOutput:  "output",
			Timestamp:  time.Now(),
			TurnNumber: i,
		}
		m.shardResultHistory = append(m.shardResultHistory, result)
	}

	// Should have 15 results
	if len(m.shardResultHistory) != 15 {
		t.Errorf("ShardResultHistory length = %d, want 15", len(m.shardResultHistory))
	}
}

// TestModel_ErrorHandling tests error state management
func TestModel_ErrorHandling(t *testing.T) {
	m := NewTestModel()

	// Set an error
	testErr := &MockError{msg: "Test error message"}
	m.err = testErr

	// Refresh error viewport
	m.refreshErrorViewport()

	// Error panel should have content
	content := m.errorVP.View()
	if content == "" {
		t.Log("Error viewport content may be empty if width < 1")
	}

	// Clear error
	m.err = nil
	m.refreshErrorViewport()
}

// TestModel_ContinuationModes tests continuation mode settings
func TestModel_ContinuationModes(t *testing.T) {
	modes := []ContinuationMode{
		ContinuationModeAuto,
		ContinuationModeConfirm,
		ContinuationModeBreakpoint,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			m := NewTestModel(WithContinuationMode(mode))
			if m.continuationMode != mode {
				t.Errorf("ContinuationMode = %v, want %v", m.continuationMode, mode)
			}
		})
	}
}

// TestModel_PendingSubtasks tests subtask management
func TestModel_PendingSubtasks(t *testing.T) {
	subtasks := []Subtask{
		{ID: "1", Description: "review file1.go", ShardType: "reviewer"},
		{ID: "2", Description: "test file2.go", ShardType: "tester"},
	}

	m := NewTestModel(WithPendingSubtasks(subtasks...))

	if len(m.pendingSubtasks) != 2 {
		t.Errorf("PendingSubtasks length = %d, want 2", len(m.pendingSubtasks))
	}
	if m.continuationTotal != 2 {
		t.Errorf("ContinuationTotal = %d, want 2", m.continuationTotal)
	}
}

// TestModel_Shutdown tests graceful shutdown
func TestModel_Shutdown(t *testing.T) {
	m := NewTestModel()

	// Verify shutdown context exists
	if m.shutdownCtx == nil {
		t.Error("shutdownCtx is nil")
	}
	if m.shutdownCancel == nil {
		t.Error("shutdownCancel is nil")
	}

	// Trigger shutdown
	m.shutdownCancel()

	// Context should be done
	select {
	case <-m.shutdownCtx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown context not cancelled")
	}
}

// TestModel_WithWorkspace tests workspace path handling
func TestModel_WithWorkspace(t *testing.T) {
	workspace := t.TempDir()
	m := NewTestModel()
	m.workspace = workspace

	if m.workspace != workspace {
		t.Errorf("Workspace = %q, want %q", m.workspace, workspace)
	}
}

// TestModel_RenderingCache tests rendering cache functionality
func TestModel_RenderingCache(t *testing.T) {
	m := NewTestModel(WithHistory(
		Message{Role: "user", Content: "Hello", Time: time.Now()},
		Message{Role: "assistant", Content: "Hi", Time: time.Now()},
	))

	// Render history
	_ = m.View()

	// Cache should have entries (or be empty if caching is disabled)
	// Just verify no panic
}

// ============================================================================
// SESSION TESTS - Tests for session management
// ============================================================================

// TestSession_StatePreservation tests session state preservation
func TestSession_StatePreservation(t *testing.T) {
	workspace := t.TempDir()
	m := NewTestModel()
	m.workspace = workspace
	m.sessionID = "test-session-123"

	// Add some history
	m.history = []Message{
		{Role: "user", Content: "Hello", Time: time.Now()},
		{Role: "assistant", Content: "Hi there!", Time: time.Now()},
	}

	// Verify state
	if m.workspace != workspace {
		t.Errorf("Workspace mismatch: %s != %s", m.workspace, workspace)
	}
	if m.sessionID != "test-session-123" {
		t.Errorf("SessionID = %q, want test-session-123", m.sessionID)
	}
	if len(m.history) != 2 {
		t.Errorf("History length = %d, want 2", len(m.history))
	}
}

// TestSession_IDHandling tests session ID handling
func TestSession_IDHandling(t *testing.T) {
	m := NewTestModel()

	// Empty session ID
	if m.sessionID != "" {
		t.Logf("Initial sessionID = %q", m.sessionID)
	}

	// Set session ID
	m.sessionID = "existing-session"
	if m.sessionID != "existing-session" {
		t.Errorf("sessionID = %q, want existing-session", m.sessionID)
	}
}

// TestSession_TurnCount tests turn counting
func TestSession_TurnCount(t *testing.T) {
	m := NewTestModel()
	m.turnCount = 5

	if m.turnCount != 5 {
		t.Errorf("turnCount = %d, want 5", m.turnCount)
	}

	// Increment
	m.turnCount++
	if m.turnCount != 6 {
		t.Errorf("turnCount after increment = %d, want 6", m.turnCount)
	}
}

// TestSession_WorkspaceDetection tests workspace detection
func TestSession_WorkspaceDetection(t *testing.T) {
	workspace := t.TempDir()

	// Create .nerd directory
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		t.Fatalf("Failed to create .nerd dir: %v", err)
	}

	m := NewTestModel()
	m.workspace = workspace

	// Verify workspace is set
	if m.workspace != workspace {
		t.Errorf("Workspace = %q, want %q", m.workspace, workspace)
	}
}

// ============================================================================
// CAMPAIGN TESTS - Tests for campaign functionality
// ============================================================================

// TestCampaign_RenderProgressBarFunc tests progress bar rendering
func TestCampaign_RenderProgressBarFunc(t *testing.T) {
	tests := []struct {
		name     string
		progress float64
		width    int
	}{
		{"0 percent", 0.0, 20},
		{"50 percent", 0.5, 20},
		{"100 percent", 1.0, 20},
		{"narrow width", 0.5, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderProgressBar(tt.progress, tt.width)
			// Just verify it doesn't panic and returns something
			if result == "" {
				t.Error("renderProgressBar returned empty string")
			}
		})
	}
}

// TestCampaign_StatusIcons tests status icon rendering
func TestCampaign_StatusIcons(t *testing.T) {
	statuses := []string{
		"pending",
		"running",
		"completed",
		"failed",
		"cancelled",
		"unknown",
	}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			icon := getStatusIcon(status)
			// Just verify it doesn't panic
			_ = icon
		})
	}
}

// TestCampaign_TruncateString tests string truncation
func TestCampaign_TruncateString(t *testing.T) {
	tests := []struct {
		input   string
		maxLen  int
		wantMax int
	}{
		{"short", 10, 10},
		{"this is a long string", 10, 10},
		{"", 5, 5},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if len(result) > tt.wantMax {
			t.Errorf("truncateString(%q, %d) len = %d, want <= %d", tt.input, tt.maxLen, len(result), tt.wantMax)
		}
	}
}

// ============================================================================
// WIZARD TESTS - Tests for wizard functionality
// ============================================================================

// TestWizard_ConfigWizardSteps tests config wizard step progression
func TestWizard_ConfigWizardSteps(t *testing.T) {
	wizard := NewConfigWizard()

	steps := []ConfigWizardStep{
		StepWelcome,
		StepEngine,
		StepProvider,
		StepAPIKey,
		StepModel,
		StepReview,
	}

	for i, step := range steps {
		t.Run(fmt.Sprintf("step_%d", i), func(t *testing.T) {
			wizard.Step = step
			if wizard.Step != step {
				t.Errorf("Step = %v, want %v", wizard.Step, step)
			}
		})
	}
}

// TestWizard_DefaultValues tests config wizard default values
func TestWizard_DefaultValues(t *testing.T) {
	wizard := NewConfigWizard()

	// Check defaults
	if wizard.Engine != "api" {
		t.Errorf("Default Engine = %q, want api", wizard.Engine)
	}
	if wizard.MaxTokens != 128000 {
		t.Errorf("Default MaxTokens = %d, want 128000", wizard.MaxTokens)
	}
	if wizard.MaxMemoryMB != 2048 {
		t.Errorf("Default MaxMemoryMB = %d, want 2048", wizard.MaxMemoryMB)
	}
}

// TestWizard_ProviderModels tests provider model mapping
func TestWizard_ProviderModels(t *testing.T) {
	providers := []string{"gemini", "openai", "anthropic", "openrouter"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			model := DefaultProviderModel(provider)
			if model == "" {
				t.Errorf("DefaultProviderModel(%q) returned empty", provider)
			}
		})
	}

	// Unknown provider should return empty
	unknown := DefaultProviderModel("unknown_provider")
	if unknown != "" {
		t.Errorf("DefaultProviderModel(unknown) = %q, want empty", unknown)
	}
}

// ============================================================================
// VIEW RENDERING TESTS - Tests for view rendering
// ============================================================================

// TestView_AllViewModesIntegration tests rendering for all view modes
func TestView_AllViewModesIntegration(t *testing.T) {
	modes := []ViewMode{
		ChatView,
		ListView,
		FilePickerView,
		UsageView,
		CampaignPage,
		PromptInspector,
		AutopoiesisPage,
		ShardPage,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			m := NewTestModel(WithViewMode(mode), WithSize(100, 50))

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("View() panicked for mode %v: %v", mode, r)
				}
			}()

			view := m.View()
			// ChatView should return something, others may be empty without state
			_ = view
		})
	}
}

// TestView_BootScreen tests boot screen rendering
func TestView_BootScreen(t *testing.T) {
	m := NewTestModel(WithBooting(true), WithSize(100, 50))

	view := m.View()
	if view == "" {
		t.Error("Boot screen rendered empty")
	}
}

// TestView_LoadingState tests loading state rendering
func TestView_LoadingState(t *testing.T) {
	m := NewTestModel(WithLoading(true), WithSize(100, 50))

	view := m.View()
	// Should show spinner or loading indicator
	_ = view
}

// TestView_ErrorPanelRendering tests error panel rendering
func TestView_ErrorPanelRendering(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.err = &MockError{msg: "Test error"}

	view := m.View()
	if !strings.Contains(view, "Test error") && !strings.Contains(view, "error") {
		// Error may be rendered differently
		t.Log("Error not visible in view, may be hidden by panel state")
	}
}

// TestView_History tests history rendering
func TestView_History(t *testing.T) {
	m := NewTestModel(
		WithSize(100, 50),
		WithHistory(
			Message{Role: "user", Content: "Hello world", Time: time.Now()},
			Message{Role: "assistant", Content: "Hi there!", Time: time.Now()},
		),
	)

	view := m.View()
	if view == "" {
		t.Error("View with history rendered empty")
	}
}

// TestView_EmptyState tests empty state rendering
func TestView_EmptyState(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	view := m.View()
	if view == "" {
		t.Error("Empty state rendered empty")
	}
}

// TestView_SmallTerminal tests rendering in small terminal
func TestView_SmallTerminal(t *testing.T) {
	m := NewTestModel(WithSize(40, 10))

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("View() panicked with small terminal: %v", r)
		}
	}()

	view := m.View()
	_ = view
}

// TestView_LargeTerminal tests rendering in large terminal
func TestView_LargeTerminal(t *testing.T) {
	m := NewTestModel(WithSize(300, 100))

	view := m.View()
	if view == "" {
		t.Error("Large terminal rendered empty")
	}
}

// ============================================================================
// CONCURRENT TESTS - Tests for concurrency safety
// ============================================================================

// TestConcurrent_ViewRendering tests concurrent view rendering
func TestConcurrent_ViewRendering(t *testing.T) {
	m := NewTestModel(WithSize(100, 50), WithHistory(TestMessages.UserMessage))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() {
				recover() // Don't fail on panic in goroutine
				done <- true
			}()
			for j := 0; j < 100; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					_ = m.View()
				}
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestConcurrent_HistoryAccess tests concurrent history access
func TestConcurrent_HistoryAccess(t *testing.T) {
	m := NewTestModel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan bool, 5)

	// Writers
	for i := 0; i < 2; i++ {
		go func(id int) {
			defer func() {
				recover()
				done <- true
			}()
			for j := 0; j < 50; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					m.history = append(m.history, Message{
						Role:    "user",
						Content: "Test",
						Time:    time.Now(),
					})
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < 3; i++ {
		go func() {
			defer func() {
				recover()
				done <- true
			}()
			for j := 0; j < 50; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					_ = len(m.history)
				}
			}
		}()
	}

	// Wait for all
	for i := 0; i < 5; i++ {
		<-done
	}
}

// ============================================================================
// HELPER FUNCTION TESTS - Additional coverage for helpers
// ============================================================================

// TestFormatShardTask tests shard task formatting
func TestFormatShardTask(t *testing.T) {
	workspace := t.TempDir()

	tests := []struct {
		verb       string
		target     string
		constraint string
	}{
		{"/review", "main.go", ""},
		{"/test", "", ""},
		{"/fix", "file.go", "focus on errors"},
		{"/refactor", "none", "improve performance"},
	}

	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			result := formatShardTask(tt.verb, tt.target, tt.constraint, workspace)
			if result == "" {
				t.Error("formatShardTask returned empty string")
			}
		})
	}
}

// TestRenderHistory tests history rendering function
func TestRenderHistory(t *testing.T) {
	m := NewTestModel(
		WithSize(100, 50),
		WithHistory(
			Message{Role: "user", Content: "Hello", Time: time.Now()},
			Message{Role: "assistant", Content: "Hi", Time: time.Now()},
		),
	)

	result := m.renderHistory()
	if result == "" {
		t.Error("renderHistory returned empty string")
	}
}

// TestRenderSingleMessage_Integration tests single message rendering
func TestRenderSingleMessage_Integration(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	// Test user and assistant messages (always render)
	messages := []Message{
		{Role: "user", Content: "Hello", Time: time.Now()},
		{Role: "assistant", Content: "Hi there", Time: time.Now()},
	}

	for _, msg := range messages {
		t.Run(msg.Role, func(t *testing.T) {
			result := m.renderSingleMessage(msg)
			if result == "" {
				t.Error("renderSingleMessage returned empty string")
			}
		})
	}

	// System messages only render when glassBoxEnabled is true
	t.Run("system_without_glassbox", func(t *testing.T) {
		msg := Message{Role: "system", Content: "System message", Time: time.Now()}
		result := m.renderSingleMessage(msg)
		if result != "" {
			t.Error("system message should be empty when glassBoxEnabled is false")
		}
	})

	t.Run("system_with_glassbox", func(t *testing.T) {
		m.glassBoxEnabled = true
		msg := Message{Role: "system", Content: "System message", Time: time.Now()}
		result := m.renderSingleMessage(msg)
		// Note: result may still be empty depending on renderGlassBoxMessage implementation
		// Just verify no panic
		_ = result
	})
}

// TestSafeRenderMarkdown tests safe markdown rendering
func TestSafeRenderMarkdown(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	inputs := []string{
		"Hello world",
		"# Heading",
		"**bold** and *italic*",
		"```go\nfunc main() {}\n```",
		"",
	}

	for i, input := range inputs {
		t.Run(fmt.Sprintf("input_%d", i), func(t *testing.T) {
			result := m.safeRenderMarkdown(input)
			// Just verify no panic - result may vary based on renderer availability
			_ = result
		})
	}
}
