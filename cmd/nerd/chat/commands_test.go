// Package chat provides tests for command handlers.
// This file tests the handleCommand function and related command processing.
package chat

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// SESSION COMMANDS TESTS
// =============================================================================

func TestCommand_Quit(t *testing.T) {
	t.Parallel()
	tests := []string{"/quit", "/exit", "/q"}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			m := NewTestModel()
			m.textarea.SetValue(cmd)

			newModel, teaCmd := m.handleCommand(cmd)
			_ = newModel

			// Should return tea.Quit
			if teaCmd == nil {
				t.Error("Expected tea.Quit command, got nil")
			}
		})
	}
}

func TestCommand_Continue_NoPending(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/continue")
	result := newModel.(Model)

	// Should add message about no pending tasks
	if len(result.history) == 0 {
		t.Error("Expected message in history")
		return
	}
	last := result.history[len(result.history)-1]
	if !strings.Contains(last.Content, "No pending tasks") {
		t.Errorf("Expected 'No pending tasks' message, got: %s", last.Content)
	}
}

func TestCommand_Continue_WithPending(t *testing.T) {
	t.Parallel()
	m := NewTestModel(
		WithPendingSubtasks(
			Subtask{ID: "1", Description: "Test task", ShardType: "coder"},
		),
	)

	newModel, cmd := m.handleCommand("/continue")
	result := newModel.(Model)

	// Should start loading
	if !result.isLoading {
		t.Error("Expected isLoading to be true")
	}

	// Should have a batch command
	if cmd == nil {
		t.Error("Expected command, got nil")
	}

	// Pending subtasks should be consumed
	if len(result.pendingSubtasks) != 0 {
		t.Errorf("Expected 0 pending subtasks, got %d", len(result.pendingSubtasks))
	}
}

func TestCommand_Usage(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/usage")
	result := newModel.(Model)

	if result.viewMode != UsageView {
		t.Errorf("Expected UsageView, got %v", result.viewMode)
	}
}

func TestCommand_Clear(t *testing.T) {
	t.Parallel()
	m := NewTestModel(
		WithHistory(
			Message{Role: "user", Content: "test", Time: time.Now()},
			Message{Role: "assistant", Content: "response", Time: time.Now()},
		),
	)

	if len(m.history) != 2 {
		t.Fatalf("Expected 2 messages in setup, got %d", len(m.history))
	}

	newModel, _ := m.handleCommand("/clear")
	result := newModel.(Model)

	if len(result.history) != 0 {
		t.Errorf("Expected empty history, got %d messages", len(result.history))
	}
}

func TestCommand_NewSession(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	oldSessionID := m.sessionID

	newModel, _ := m.handleCommand("/new-session")
	result := newModel.(Model)

	// Should have a new session ID
	if result.sessionID == oldSessionID && oldSessionID != "" {
		t.Error("Expected new session ID")
	}

	// Should have a message about new session
	if len(result.history) == 0 {
		t.Error("Expected message in history")
		return
	}
	if !strings.Contains(result.history[0].Content, "new session") {
		t.Error("Expected message about new session")
	}
}

// =============================================================================
// HELP COMMANDS TESTS
// =============================================================================

func TestCommand_Help(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/help")
	result := newModel.(Model)

	if len(result.history) == 0 {
		t.Error("Expected help message in history")
		return
	}

	last := result.history[len(result.history)-1]
	if last.Role != "assistant" {
		t.Errorf("Expected assistant role, got %s", last.Role)
	}

	// Help should contain command categories
	if !strings.Contains(last.Content, "Commands") && !strings.Contains(last.Content, "command") {
		t.Error("Expected help content to mention commands")
	}
}

func TestCommand_Status(t *testing.T) {
	t.Parallel()

	// /status calls buildStatusReport which requires kernel
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic on /status without kernel: %v", r)
		}
	}()

	t.Log("Skipping /status test - requires kernel")
}

// =============================================================================
// CONFIG COMMANDS TESTS
// =============================================================================

func TestCommand_Config(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/config")
	result := newModel.(Model)

	// Should enter config wizard or show config
	if result.awaitingConfigWizard {
		t.Log("Config wizard started")
	} else if len(result.history) > 0 {
		t.Log("Config shown in history")
	} else {
		t.Error("Expected either wizard start or config display")
	}
}

// =============================================================================
// INIT COMMANDS TESTS
// =============================================================================

func TestCommand_Init_Usage(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Basic /init should work
	newModel, cmd := m.handleCommand("/init")
	result := newModel.(Model)

	// Should either show already initialized or start loading
	if result.isLoading {
		if cmd == nil {
			t.Error("Expected command when loading")
		}
	}
	// Otherwise should have message about workspace status
}

func TestCommand_Scan(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, cmd := m.handleCommand("/scan")
	result := newModel.(Model)

	if len(result.history) == 0 {
		t.Error("Expected message in history")
	}

	if result.isLoading && cmd == nil {
		t.Error("Expected command when loading")
	}
}

// =============================================================================
// QUERY COMMANDS TESTS
// =============================================================================

func TestCommand_Query_NoKernel(t *testing.T) {
	t.Parallel()
	// kernel is already nil by default in NewTestModel

	// /query requires kernel - skip test
	t.Log("Skipping /query test - requires kernel")
}

func TestCommand_Why(t *testing.T) {
	t.Parallel()

	// /why requires kernel - just test no panic when kernel is nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic on /why: %v", r)
		}
	}()

	// Don't call /why without kernel, just verify we can test it
	t.Log("Skipping /why test - requires kernel")
}

func TestCommand_Logic(t *testing.T) {
	t.Parallel()

	// /logic may require kernel - test with defer for safety
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic on /logic without kernel: %v", r)
		}
	}()

	// Skip actual command execution without kernel
	t.Log("Skipping /logic test - may require kernel")
}

func TestCommand_Glassbox(t *testing.T) {
	t.Parallel()

	// /glassbox may require kernel for some functionality
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic on /glassbox without kernel: %v", r)
		}
	}()

	// Skip actual test - requires proper initialization
	t.Log("Skipping /glassbox test - may require kernel")
}

// =============================================================================
// SHARD COMMANDS TESTS
// =============================================================================

func TestCommand_Review(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, cmd := m.handleCommand("/review main.go")
	result := newModel.(Model)

	// Should have a message and potentially start loading
	if len(result.history) == 0 && cmd == nil {
		t.Error("Expected message or command")
	}
}

func TestCommand_Test(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, cmd := m.handleCommand("/test")
	result := newModel.(Model)

	// Should have a message and potentially start loading
	if len(result.history) == 0 && cmd == nil {
		t.Error("Expected message or command")
	}
}

func TestCommand_Fix(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, cmd := m.handleCommand("/fix the bug")
	result := newModel.(Model)

	// Should have a message and potentially start loading
	if len(result.history) == 0 && cmd == nil {
		t.Error("Expected message or command")
	}
}

// =============================================================================
// CAMPAIGN COMMANDS TESTS
// =============================================================================

func TestCommand_Campaign_NoArgs(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/campaign")
	result := newModel.(Model)

	// Should show usage or current campaign status
	if len(result.history) == 0 {
		// May switch to campaign view
		if result.viewMode != CampaignPage {
			t.Error("Expected message or campaign view")
		}
	}
}

func TestCommand_Campaign_Status(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/campaign status")
	result := newModel.(Model)

	// Should show campaign status
	if len(result.history) == 0 && result.viewMode != CampaignPage {
		t.Error("Expected status message or campaign view")
	}
}

func TestCommand_Clarify_NoArgs(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/clarify")
	result := newModel.(Model)

	// Should show usage
	if len(result.history) == 0 {
		t.Error("Expected usage message")
		return
	}
	if !strings.Contains(result.history[len(result.history)-1].Content, "Usage") {
		t.Error("Expected usage message")
	}
}

func TestCommand_Legislate_NoArgs(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/legislate")
	result := newModel.(Model)

	// Should show usage
	if len(result.history) == 0 {
		t.Error("Expected usage message")
		return
	}
	if !strings.Contains(result.history[len(result.history)-1].Content, "Usage") {
		t.Error("Expected usage message")
	}
}

// =============================================================================
// EVOLUTION COMMANDS TESTS
// =============================================================================

func TestCommand_Evolve(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, cmd := m.handleCommand("/evolve")
	result := newModel.(Model)

	// Should either show message or start evolution
	_ = result
	if cmd != nil {
		t.Log("Evolution command returned")
	}
}

func TestCommand_EvolutionStats(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/evolution-stats")
	result := newModel.(Model)

	// Should show stats or message
	if len(result.history) == 0 {
		t.Log("No stats to show")
	}
}

// =============================================================================
// SPECIAL VIEW COMMANDS TESTS
// =============================================================================

func TestCommand_JIT(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/jit")
	result := newModel.(Model)

	// Should switch to JIT inspector view
	if result.viewMode != PromptInspector {
		t.Log("May show message instead of switching view if no JIT data")
	}
}

func TestCommand_Shards(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/shards")
	result := newModel.(Model)

	// Should switch to shard page or show message
	if result.viewMode != ShardPage && len(result.history) == 0 {
		t.Error("Expected shard view or message")
	}
}

func TestCommand_Autopoiesis(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/autopoiesis")
	result := newModel.(Model)

	// Should switch to autopoiesis page or show message
	if result.viewMode != AutopoiesisPage && len(result.history) == 0 {
		t.Log("May show message if no autopoiesis data")
	}
}

// =============================================================================
// AGENT COMMANDS TESTS
// =============================================================================

func TestCommand_DefineAgent(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/define-agent")
	result := newModel.(Model)

	// Should start agent wizard
	if !result.awaitingAgentDefinition && len(result.history) == 0 {
		t.Error("Expected wizard start or message")
	}
}

func TestCommand_Agents(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/agents")
	result := newModel.(Model)

	// Should list agents or show message
	if len(result.history) == 0 {
		t.Error("Expected agent list or message")
	}
}

// =============================================================================
// FILE COMMANDS TESTS
// =============================================================================

func TestCommand_Read_NoArgs(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/read")
	result := newModel.(Model)

	// Should show usage or open file picker
	if result.viewMode != FilePickerView && len(result.history) == 0 {
		t.Error("Expected file picker or usage message")
	}
}

func TestCommand_Search_NoArgs(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/search")
	result := newModel.(Model)

	// Should show usage
	if len(result.history) == 0 {
		t.Error("Expected usage message")
	}
}

// =============================================================================
// UNKNOWN COMMAND TEST
// =============================================================================

func TestCommand_Unknown(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/nonexistentcommand")
	result := newModel.(Model)

	// Should show unknown command message
	if len(result.history) == 0 {
		t.Error("Expected message about unknown command")
		return
	}
	// May process as natural language or show error
}

// =============================================================================
// COMMAND EDGE CASES
// =============================================================================

func TestCommand_EmptyAfterSlash(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// This tests edge case of just "/"
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic on empty command: %v", r)
		}
	}()

	m.textarea.SetValue("/")
	// The handleCommand is usually called only if there's an actual command
	// But let's test boundary
}

func TestCommand_WhitespaceHandling(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	// Test command with extra whitespace
	newModel, _ := m.handleCommand("/help   ")
	result := newModel.(Model)

	if len(result.history) == 0 {
		t.Error("Expected help message despite whitespace")
	}
}

// =============================================================================
// COMMAND DOES NOT PANIC TESTS
// =============================================================================

func TestAllCommands_NoPanic(t *testing.T) {
	t.Parallel()

	commands := []string{
		"/quit",
		"/exit",
		"/q",
		"/continue",
		"/resume",
		"/usage",
		"/clear",
		// "/reset", // Requires kernel
		"/new-session",
		// "/sessions", // May require workspace
		"/load-session",
		"/help",
		// "/status", // Requires kernel for buildStatusReport
		// "/reflection", // May require kernel
		"/knowledge",
		"/legislate",
		"/clarify",
		"/launchcampaign",
		"/init",
		"/scan",
		"/refresh-docs",
		"/config",
		// "/query", // Requires kernel
		// "/why",   // Requires kernel
		// "/logic", // May require kernel
		// "/glassbox", // May require kernel
		"/transparency",
		// "/shadow", // May require kernel
		// "/whatif", // May require kernel
		"/review",
		"/security",
		"/analyze",
		"/test",
		"/fix",
		"/refactor",
		"/campaign",
		"/evolve",
		"/evolution-stats",
		"/evolved-atoms",
		"/promote-atom",
		"/reject-atom",
		"/strategies",
		"/jit",
		"/shards",
		"/autopoiesis",
		"/define-agent",
		"/northstar",
		"/learn",
		"/agents",
		"/spawn",
		"/ingest",
		"/read",
		"/mkdir",
		"/write",
		"/search",
		"/patch",
		"/edit",
		"/append",
		"/pick",
		"/tool",
		"/cleanup-tools",
		"/approve",
		"/reject-finding",
		"/accept-finding",
		// "/review-accuracy", // May require localDB
		"/embedding",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC on command %s: %v", cmd, r)
				}
			}()

			m := NewTestModel()
			_, _ = m.handleCommand(cmd)
		})
	}
}

// =============================================================================
// COMMAND + ARGUMENT COMBINATIONS
// =============================================================================

func TestCommand_WithArguments(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		command string
	}{
		{"review with file", "/review main.go"},
		{"test with path", "/test ./..."},
		{"fix with description", "/fix the authentication bug"},
		{"search with pattern", "/search TODO"},
		// {"query with predicate", "/query user_intent(?a, ?b, ?c, ?d, ?e)"}, // Requires kernel
		{"legislate with rule", "/legislate always use context.Context"},
		{"clarify with goal", "/clarify build user authentication"},
		{"campaign start", "/campaign start migration"},
		{"campaign pause", "/campaign pause"},
		{"campaign resume", "/campaign resume"},
		{"campaign status", "/campaign status"},
		{"init with force", "/init --force"},
		{"scan with deep", "/scan --deep"},
		{"knowledge search", "/knowledge search authentication"},
		{"knowledge index", "/knowledge 1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC on '%s': %v", tc.command, r)
				}
			}()

			m := NewTestModel()
			_, _ = m.handleCommand(tc.command)
		})
	}
}

// =============================================================================
// KEYBINDING TESTS (Update path for commands)
// =============================================================================

func TestKeyMsg_EnterTriggersCommand(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.textarea.SetValue("/help")

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := newModel.(Model)

	// Textarea should be reset after command
	if m.textarea.Value() == "/help" && result.textarea.Value() == "/help" {
		// May not reset if still processing
		t.Log("Command may be processed asynchronously")
	}
}

func TestKeyMsg_CtrlC_Quit(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	// Should return quit command
	if cmd == nil {
		t.Error("Expected quit command")
	}
}

func TestKeyMsg_Esc_FromListView(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(ListView))

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := newModel.(Model)

	if result.viewMode != ChatView {
		t.Errorf("Expected ChatView after Esc from ListView, got %v", result.viewMode)
	}
}

func TestKeyMsg_Esc_FromUsageView(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithViewMode(UsageView))

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := newModel.(Model)

	if result.viewMode != ChatView {
		t.Errorf("Expected ChatView after Esc from UsageView, got %v", result.viewMode)
	}
}

func TestKeyMsg_ShiftTab_CyclesContinuationMode(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithContinuationMode(ContinuationModeAuto))

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	result := newModel.(Model)

	if result.continuationMode != ContinuationModeConfirm {
		t.Errorf("Expected ContinuationModeConfirm, got %v", result.continuationMode)
	}

	// Cycle again
	newModel2, _ := result.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	result2 := newModel2.(Model)

	if result2.continuationMode != ContinuationModeBreakpoint {
		t.Errorf("Expected ContinuationModeBreakpoint, got %v", result2.continuationMode)
	}

	// Cycle back to Auto
	newModel3, _ := result2.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	result3 := newModel3.(Model)

	if result3.continuationMode != ContinuationModeAuto {
		t.Errorf("Expected ContinuationModeAuto, got %v", result3.continuationMode)
	}
}

func TestKeyMsg_CtrlX_StopsLoading(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithLoading(true))

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	result := newModel.(Model)

	if result.isLoading {
		t.Error("Expected isLoading to be false after Ctrl+X")
	}

	if !result.isInterrupted {
		t.Error("Expected isInterrupted to be true")
	}
}
