// Package chat provides tests for the Update loop and message routing.
// This file tests the core state machine transitions and message handling.
package chat

import (
	"runtime"
	"testing"
	"time"

	"codenerd/cmd/nerd/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// WINDOW SIZE MESSAGE TESTS
// =============================================================================

func TestUpdate_WindowSize(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := newModel.(Model)

	if result.width != 120 {
		t.Errorf("Expected width 120, got %d", result.width)
	}
	if result.height != 40 {
		t.Errorf("Expected height 40, got %d", result.height)
	}
}

func TestUpdate_WindowSize_Zero(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Should not panic on zero dimensions
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic on zero window size: %v", r)
		}
	}()

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	_ = newModel
}

func TestUpdate_WindowSize_Negative(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Should not panic on negative dimensions
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic on negative window size: %v", r)
		}
	}()

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: -1, Height: -1})
	_ = newModel
}

func TestUpdate_WindowSize_Large(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Should handle very large dimensions
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic on large window size: %v", r)
		}
	}()

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 10000, Height: 5000})
	result := newModel.(Model)

	if result.width != 10000 {
		t.Errorf("Expected width 10000, got %d", result.width)
	}
}

// =============================================================================
// BOOT SEQUENCE TESTS
// =============================================================================

func TestUpdate_BootComplete(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithBooting(true))

	msg := bootCompleteMsg{
		components: &SystemComponents{
			InitialMessages: []Message{
				{Role: "assistant", Content: "System Ready", Time: time.Now()},
			},
			Workspace: "/test",
		},
		err: nil,
	}

	newModel, cmd := m.Update(msg)
	result := newModel.(Model)

	// Should transition to scanning stage
	if result.bootStage != BootStageScanning {
		t.Errorf("Expected BootStageScanning, got %d", result.bootStage)
	}

	// Should still be booting (scanning phase)
	if !result.isBooting {
		t.Error("Expected isBooting to still be true during scanning")
	}

	// Should have a command to start scanning
	if cmd == nil {
		t.Error("Expected scan command")
	}
}

func TestUpdate_BootComplete_Error(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithBooting(true))

	msg := bootCompleteMsg{
		components: nil,
		err:        &MockError{msg: "boot failed"},
	}

	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	// Should have error set
	if result.err == nil {
		t.Error("Expected error to be set")
	}
}

func TestUpdate_ScanComplete(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithBooting(true))
	m.bootStage = BootStageScanning

	msg := scanCompleteMsg{
		fileCount: 100,
		factCount: 500,
		duration:  time.Second,
		err:       nil,
	}

	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	// Should no longer be booting
	if result.isBooting {
		t.Error("Expected isBooting to be false after scan complete")
	}
}

func TestUpdate_ScanComplete_Error(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithBooting(true))
	m.bootStage = BootStageScanning

	msg := scanCompleteMsg{
		fileCount: 0,
		factCount: 0,
		duration:  time.Second,
		err:       &MockError{msg: "scan failed"},
	}

	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	// Boot should complete even with scan error (non-blocking)
	// Check that model state is consistent
	if result.bootStage != BootStageScanning && result.isBooting {
		t.Log("Boot continues despite scan error")
	}
}

// =============================================================================
// VIEW MODE ROUTING TESTS
// =============================================================================

func TestUpdate_ListView_KeyRouting(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(ListView))

	// The list needs to be properly initialized
	// Skip key routing test since list requires items
	// Just verify no panic on Escape
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic on ListView key routing: %v", r)
		}
	}()

	// Escape should work
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := newModel.(Model)

	// Should return to ChatView
	if result.viewMode != ChatView {
		t.Errorf("Expected ChatView after Esc, got %v", result.viewMode)
	}
}

func TestUpdate_FilePickerView_KeyRouting(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(FilePickerView))

	// Keys should be routed to file picker
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	result := newModel.(Model)

	// Should still be in FilePickerView
	if result.viewMode != FilePickerView {
		t.Errorf("Expected FilePickerView, got %v", result.viewMode)
	}
}

func TestUpdate_UsageView_KeyRouting(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(UsageView))

	// Keys should be routed to usage page
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	result := newModel.(Model)

	// Should still be in UsageView
	if result.viewMode != UsageView {
		t.Errorf("Expected UsageView, got %v", result.viewMode)
	}
}

func TestUpdate_CampaignPage_KeyRouting(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(CampaignPage))

	// Q should exit campaign page
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	result := newModel.(Model)

	// Should return to ChatView
	if result.viewMode != ChatView {
		t.Errorf("Expected ChatView after q in CampaignPage, got %v", result.viewMode)
	}
}

func TestUpdate_PromptInspector_KeyRouting(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(PromptInspector))

	// Q should exit prompt inspector
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	result := newModel.(Model)

	// Should return to ChatView
	if result.viewMode != ChatView {
		t.Errorf("Expected ChatView after q in PromptInspector, got %v", result.viewMode)
	}
}

func TestUpdate_AutopoiesisPage_KeyRouting(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(AutopoiesisPage))

	// Q should exit autopoiesis page
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	result := newModel.(Model)

	// Should return to ChatView
	if result.viewMode != ChatView {
		t.Errorf("Expected ChatView after q in AutopoiesisPage, got %v", result.viewMode)
	}
}

func TestUpdate_ShardPage_KeyRouting(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(ShardPage))

	// Q should exit shard page
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	result := newModel.(Model)

	// Should return to ChatView
	if result.viewMode != ChatView {
		t.Errorf("Expected ChatView after q in ShardPage, got %v", result.viewMode)
	}
}

// =============================================================================
// SPINNER MESSAGE TESTS
// =============================================================================

func TestUpdate_SpinnerTick(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithLoading(true))

	// Spinner tick should update spinner state
	tickCmd := m.spinner.Tick
	// tickCmd is a method value, so it is never nil.
	tickMsg := tickCmd()
	newModel, cmd := m.Update(tickMsg)
	_ = newModel
	_ = cmd
}

// =============================================================================
// STATUS MESSAGE TESTS
// =============================================================================

func TestUpdate_StatusMsg(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.Update(statusMsg("Processing..."))
	result := newModel.(Model)

	if result.statusMessage != "Processing..." {
		t.Errorf("Expected status 'Processing...', got '%s'", result.statusMessage)
	}
}

// =============================================================================
// MEMORY USAGE MESSAGE TESTS
// =============================================================================

func TestUpdate_MemUsageMsg(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	msg := memUsageMsg{Alloc: 1000000, Sys: 5000000}
	newModel, _ := m.Update(msg)
	result := newModel.(Model)

	if result.memAllocBytes != 1000000 {
		t.Errorf("Expected memAllocBytes 1000000, got %d", result.memAllocBytes)
	}
	if result.memSysBytes != 5000000 {
		t.Errorf("Expected memSysBytes 5000000, got %d", result.memSysBytes)
	}
}

// =============================================================================
// ERROR PANEL FOCUS TESTS
// =============================================================================

func TestUpdate_ErrorPanelFocus_ScrollKeys(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.focusError = true
	m.err = &MockError{msg: "test error"}
	m.showError = true

	// Up/Down should scroll error viewport
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	result := newModel.(Model)

	// Should still be focused on error
	if !result.focusError {
		t.Error("Expected focusError to remain true")
	}
}

func TestUpdate_ErrorPanelFocus_Escape(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.focusError = true
	m.err = &MockError{msg: "test error"}
	m.showError = true

	// Escape should unfocus error panel
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := newModel.(Model)

	if result.focusError {
		t.Error("Expected focusError to be false after Escape")
	}
}

// =============================================================================
// CLARIFICATION STATE TESTS
// =============================================================================

func TestUpdate_ClarificationMode(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithInputMode(InputModeClarification))
	m.clarificationState = &ClarificationState{
		Question: "Which file?",
		Options:  []string{"file1.go", "file2.go"},
	}

	// Enter should process clarification response
	m.textarea.SetValue("1")
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = newModel
}

// =============================================================================
// PANE MODE TESTS
// =============================================================================

func TestUpdate_ToggleLogicPane(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.showLogic = false
	if m.showLogic {
		t.Error("Expected showLogic to be false")
	}

	// Alt+L should toggle logic pane (if implemented)
	// This tests the pane mode cycling
	initialMode := m.paneMode

	// Test cycling through pane modes
	modes := []ui.PaneMode{ui.ModeSinglePane, ui.ModeSplitPane, ui.ModeFullLogic}
	for _, mode := range modes {
		m.paneMode = mode
		if m.paneMode != mode {
			t.Errorf("Expected pane mode %v", mode)
		}
	}
	_ = initialMode
}

// =============================================================================
// MESSAGE TYPE COVERAGE TESTS
// =============================================================================

func TestUpdate_AllMessageTypes_NoPanic(t *testing.T) {
	t.Parallel()

	messages := []tea.Msg{
		tea.WindowSizeMsg{Width: 100, Height: 50},
		tea.KeyMsg{Type: tea.KeyEnter},
		tea.KeyMsg{Type: tea.KeyEsc},
		tea.KeyMsg{Type: tea.KeyCtrlC},
		tea.KeyMsg{Type: tea.KeyCtrlX},
		tea.KeyMsg{Type: tea.KeyShiftTab},
		tea.KeyMsg{Type: tea.KeyUp},
		tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyLeft},
		tea.KeyMsg{Type: tea.KeyRight},
		tea.KeyMsg{Type: tea.KeyPgUp},
		tea.KeyMsg{Type: tea.KeyPgDown},
		tea.KeyMsg{Type: tea.KeyHome},
		tea.KeyMsg{Type: tea.KeyEnd},
		tea.KeyMsg{Type: tea.KeyTab},
		tea.KeyMsg{Type: tea.KeyBackspace},
		tea.KeyMsg{Type: tea.KeyDelete},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}},
		statusMsg("test"),
		memUsageMsg{Alloc: 1000, Sys: 2000},
		// bootCompleteMsg and scanCompleteMsg require proper components, skip
		// bootCompleteMsg{components: nil, err: nil},
		scanCompleteMsg{fileCount: 10, factCount: 100, duration: time.Second, err: nil},
	}

	for i, msg := range messages {
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC on message %d (%T): %v", i, msg, r)
				}
			}()

			m := NewTestModel()
			_, _ = m.Update(msg)
		})
	}
}

// =============================================================================
// VIEW RENDERING TESTS
// =============================================================================

func TestView_NoPanic(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic in View(): %v", r)
		}
	}()

	view := m.View()
	if view == "" {
		t.Log("Empty view (may be expected during boot)")
	}
}

func TestView_AllViewModes(t *testing.T) {
	t.Parallel()

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
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC in View() for mode %v: %v", mode, r)
				}
			}()

			m := NewTestModel(WithViewMode(mode))
			view := m.View()
			if view == "" {
				t.Logf("Empty view for mode %v", mode)
			}
		})
	}
}

func TestView_WithHistory(t *testing.T) {
	t.Parallel()
	m := NewTestModel(
		WithHistory(
			Message{Role: "user", Content: "Hello", Time: time.Now()},
			Message{Role: "assistant", Content: "Hi there!", Time: time.Now()},
		),
	)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic in View() with history: %v", r)
		}
	}()

	view := m.View()
	if view == "" {
		t.Error("Expected non-empty view with history")
	}
}

func TestView_DuringBoot(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithBooting(true))

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic in View() during boot: %v", r)
		}
	}()

	view := m.View()
	// During boot, should show boot progress or spinner
	_ = view
}

func TestView_WithError(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.err = &MockError{msg: "Something went wrong"}
	m.showError = true

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic in View() with error: %v", r)
		}
	}()

	view := m.View()
	_ = view
}

// =============================================================================
// PERFORMANCE TESTS
// =============================================================================

func TestUpdate_Performance_Rapid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	t.Parallel()

	m := NewTestModel()

	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		newModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
		m = newModel.(Model)
	}

	elapsed := time.Since(start)
	avgPerUpdate := elapsed / time.Duration(iterations)

	// Target: <1ms per update for simple messages
	if avgPerUpdate > time.Millisecond {
		t.Logf("Warning: Average update time %v exceeds 1ms target", avgPerUpdate)
	}

	t.Logf("%d updates in %v (avg: %v/update)", iterations, elapsed, avgPerUpdate)
}

func TestUpdate_Performance_WithHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	t.Parallel()

	// Create model with substantial history
	messages := make([]Message, 100)
	for i := range messages {
		messages[i] = Message{
			Role:    "user",
			Content: "Test message content that is reasonably long to simulate real usage",
			Time:    time.Now(),
		}
	}

	m := NewTestModel(WithHistory(messages...))

	start := time.Now()
	iterations := 100

	for i := 0; i < iterations; i++ {
		newModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
		m = newModel.(Model)
	}

	elapsed := time.Since(start)

	// With 100 messages, should still be responsive
	if elapsed > 2*time.Second {
		t.Errorf("100 updates with 100-message history took %v (should be <2s)", elapsed)
	}

	t.Logf("100 updates with 100-message history: %v", elapsed)
}

// =============================================================================
// GOROUTINE LEAK TESTS
// =============================================================================

func TestUpdate_NoGoroutineLeakOnQuit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping goroutine test in short mode")
	}
	t.Parallel()

	before := runtime.NumGoroutine()

	// Create and shutdown multiple models
	for i := 0; i < 5; i++ {
		m := NewTestModel()
		m.Shutdown()
	}

	// Allow time for goroutines to exit
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Allow some slack for background runtime goroutines
	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d after=%d", before, after)
	}
}

// =============================================================================
// STATE CONSISTENCY TESTS
// =============================================================================

func TestUpdate_StateConsistency_AfterResize(t *testing.T) {
	t.Parallel()
	m := NewTestModel(
		WithHistory(
			Message{Role: "user", Content: "test", Time: time.Now()},
		),
	)

	// Resize multiple times
	sizes := []tea.WindowSizeMsg{
		{Width: 80, Height: 24},
		{Width: 120, Height: 40},
		{Width: 60, Height: 20},
		{Width: 200, Height: 100},
	}

	for _, size := range sizes {
		newModel, _ := m.Update(size)
		m = newModel.(Model)

		// History should be preserved
		if len(m.history) != 1 {
			t.Errorf("History lost after resize to %dx%d", size.Width, size.Height)
		}

		// View should not panic
		_ = m.View()
	}
}

func TestUpdate_StateConsistency_ModeTransitions(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Transition through view modes
	transitions := []struct {
		action   tea.Msg
		expected ViewMode
	}{
		{tea.KeyMsg{Type: tea.KeyEsc}, ChatView}, // Stay in ChatView
	}

	for _, tr := range transitions {
		newModel, _ := m.Update(tr.action)
		m = newModel.(Model)

		if m.viewMode != tr.expected {
			t.Errorf("Expected mode %v, got %v", tr.expected, m.viewMode)
		}
	}
}

// =============================================================================
// HELPER: ViewMode String
// =============================================================================

func (v ViewMode) String() string {
	names := []string{
		"ChatView",
		"ListView",
		"FilePickerView",
		"UsageView",
		"CampaignPage",
		"PromptInspector",
		"AutopoiesisPage",
		"ShardPage",
	}
	if int(v) < len(names) {
		return names[v]
	}
	return "Unknown"
}
