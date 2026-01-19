// Package chat provides tests for shadow mode and counterfactual reasoning.
package chat

import (
	"strings"
	"testing"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// SHADOW MODE TESTS
// =============================================================================

func TestShadow_BuildDerivationTrace_FactNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	result := m.buildDerivationTrace("nonexistent_predicate")

	if !strings.Contains(result, "Fact not found") {
		t.Errorf("Expected 'Fact not found' message, got: %s", result)
	}
	if !strings.Contains(result, "Derivation Trace") {
		t.Error("Expected header in trace")
	}
}

func TestShadow_BuildDerivationTrace_WithFact(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Query for facts that might exist
	result := m.buildDerivationTrace("config_value")

	// Should have proper formatting
	if !strings.Contains(result, "Derivation Trace") {
		t.Error("Expected Derivation Trace header")
	}
	// Note: Derivation Tree only appears if fact exists
	// The fact may not exist in a fresh test kernel, so we just check the header
	t.Logf("Derivation trace result length: %d chars", len(result))
}

func TestShadow_GetRuleForPredicate_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	result := getRuleForPredicate(m.kernel, "nonexistent_predicate")
	if result != "" {
		t.Errorf("Expected empty string for nonexistent predicate, got: %s", result)
	}
}

func TestShadow_GetChildNodes_NextAction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	fact := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"test"},
	}

	children := getChildNodes(m.kernel, fact)
	// Should return user_intent facts (may be empty in test kernel)
	t.Logf("Got %d children for next_action", len(children))
}

func TestShadow_GetChildNodes_Impacted(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	fact := core.Fact{
		Predicate: "impacted",
		Args:      []interface{}{"test"},
	}

	children := getChildNodes(m.kernel, fact)
	// Should look for dependency_link and modified facts
	t.Logf("Got %d children for impacted", len(children))
}

func TestShadow_GetChildNodes_ClarificationNeeded(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	fact := core.Fact{
		Predicate: "clarification_needed",
		Args:      []interface{}{"test"},
	}

	children := getChildNodes(m.kernel, fact)
	// Should look for focus_resolution facts
	t.Logf("Got %d children for clarification_needed", len(children))
}

func TestShadow_GetChildNodes_Limit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Test with a predicate that might have many children
	fact := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"test"},
	}

	children := getChildNodes(m.kernel, fact)
	// Should be limited to 5
	if len(children) > 5 {
		t.Errorf("Expected at most 5 children, got %d", len(children))
	}
}

func TestShadow_RenderLogicPane_NilPane(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	m.logicPane = nil

	result := m.renderLogicPane()
	if result != "" {
		t.Errorf("Expected empty string for nil logicPane, got: %s", result)
	}
}

func TestShadow_RenderLogicPane_WithPane(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Create a logic pane - NewLogicPane(styles, width, height) returns LogicPane (not pointer)
	logicPane := ui.NewLogicPane(m.styles, 80, 24)
	m.logicPane = &logicPane

	result := m.renderLogicPane()

	// Should have header
	if !strings.Contains(result, "Logic State") {
		t.Error("Expected 'Logic State' header in rendered pane")
	}
}

func TestShadow_UpdateLogicPane_NilPane(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	m.logicPane = nil

	// Should not panic
	m.UpdateLogicPane()
}

func TestShadow_UpdateLogicPane_WithPane(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Create a logic pane
	logicPane := ui.NewLogicPane(m.styles, 80, 24)
	m.logicPane = &logicPane

	// Should not panic and update content
	m.UpdateLogicPane()

	// The pane should have content now
	content := m.logicPane.Viewport.View()
	if len(content) == 0 {
		t.Log("Logic pane content is empty after update")
	}
}

func TestShadow_GetStyles(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	styles := m.getStyles()

	// Should return valid styles (just verify it doesn't panic)
	_ = styles
	t.Log("getStyles returned successfully")
}

// =============================================================================
// SHADOW SIMULATION TESTS (require shadowMode)
// =============================================================================

func TestShadow_RunShadowSimulation_NilShadowMode(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.shadowMode = nil

	// Just verify that runShadowSimulation returns a command
	// (The command itself would panic if executed with nil shadowMode,
	// but that's expected behavior - we just want to ensure the method doesn't panic)
	cmd := m.runShadowSimulation("test action")

	// The command should be non-nil
	if cmd == nil {
		t.Error("Expected command to be returned")
	}
	// Note: We don't execute the command because it would panic with nil shadowMode
	// This is expected behavior - the system should ensure shadowMode is initialized
}

func TestShadow_RunShadowSimulation_WithShadowMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow simulation test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Create shadow mode
	if m.kernel != nil {
		m.shadowMode = core.NewShadowMode(m.kernel)
	}

	if m.shadowMode == nil {
		t.Skip("Shadow mode not available")
	}

	var result tea.Msg
	perf.Track("shadow_simulation", func() {
		cmd := m.runShadowSimulation("refactor the handler function")
		if cmd != nil {
			result = cmd()
		}
	})

	// Check result type
	switch msg := result.(type) {
	case responseMsg:
		if !strings.Contains(string(msg), "Shadow Mode Simulation") {
			t.Error("Expected Shadow Mode Simulation header in response")
		}
		t.Logf("Shadow simulation result length: %d", len(string(msg)))
	case errorMsg:
		t.Logf("Shadow simulation returned error (may be expected): %v", msg)
	default:
		if result != nil {
			t.Errorf("Unexpected message type: %T", result)
		}
	}
}

func TestShadow_RunWhatIfQuery_NilShadowMode(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.shadowMode = nil

	// Just verify that runWhatIfQuery returns a command
	cmd := m.runWhatIfQuery("test change")

	// The command should be non-nil
	if cmd == nil {
		t.Error("Expected command to be returned")
	}
	// Note: We don't execute the command because it would panic with nil shadowMode
}

func TestShadow_RunWhatIfQuery_WithShadowMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping what-if query test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Create shadow mode
	if m.kernel != nil {
		m.shadowMode = core.NewShadowMode(m.kernel)
	}

	if m.shadowMode == nil {
		t.Skip("Shadow mode not available")
	}

	var result tea.Msg
	perf.Track("whatif_query", func() {
		cmd := m.runWhatIfQuery("modify config.go")
		if cmd != nil {
			result = cmd()
		}
	})

	// Check result type
	switch msg := result.(type) {
	case responseMsg:
		if !strings.Contains(string(msg), "What-If Analysis") {
			t.Error("Expected What-If Analysis header in response")
		}
		if !strings.Contains(string(msg), "Recommendations") {
			t.Error("Expected Recommendations section in response")
		}
		t.Logf("What-if query result length: %d", len(string(msg)))
	case errorMsg:
		t.Logf("What-if query returned error (may be expected): %v", msg)
	default:
		if result != nil {
			t.Errorf("Unexpected message type: %T", result)
		}
	}
}

// =============================================================================
// SHADOW COMMAND INTEGRATION TESTS
// =============================================================================

func TestShadow_CommandIntegration_Shadow(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, cmd := m.handleCommand("/shadow test simulation")
	result := newModel.(Model)

	// Should handle the command
	t.Logf("Shadow command result: %d messages, cmd=%v", len(result.history), cmd != nil)
}

func TestShadow_CommandIntegration_WhatIf(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, cmd := m.handleCommand("/whatif modify the handler")
	result := newModel.(Model)

	// Should handle the command
	t.Logf("WhatIf command result: %d messages, cmd=%v", len(result.history), cmd != nil)
}

func TestShadow_CommandIntegration_Why(t *testing.T) {
	t.Parallel()
	m := NewTestModel()

	newModel, _ := m.handleCommand("/why config_value")
	result := newModel.(Model)

	// Should handle the command and show derivation
	t.Logf("Why command result: %d messages", len(result.history))
}

// =============================================================================
// EDGE CASES AND ERROR HANDLING
// =============================================================================

func TestShadow_EmptyAction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	if m.kernel != nil {
		m.shadowMode = core.NewShadowMode(m.kernel)
	}

	if m.shadowMode == nil {
		t.Skip("Shadow mode not available")
	}

	cmd := m.runShadowSimulation("")
	if cmd != nil {
		result := cmd()
		t.Logf("Empty action result type: %T", result)
	}
}

func TestShadow_LongAction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	if m.kernel != nil {
		m.shadowMode = core.NewShadowMode(m.kernel)
	}

	if m.shadowMode == nil {
		t.Skip("Shadow mode not available")
	}

	longAction := strings.Repeat("refactor code ", 100)
	cmd := m.runShadowSimulation(longAction)
	if cmd != nil {
		result := cmd()
		t.Logf("Long action result type: %T", result)
	}
}

func TestShadow_SpecialCharactersInAction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	if m.kernel != nil {
		m.shadowMode = core.NewShadowMode(m.kernel)
	}

	if m.shadowMode == nil {
		t.Skip("Shadow mode not available")
	}

	specialAction := "test action with special chars: <>&\"'`$()[]{}|\\!@#%^*"
	cmd := m.runShadowSimulation(specialAction)
	if cmd != nil {
		result := cmd()
		t.Logf("Special chars action result type: %T", result)
	}
}

// =============================================================================
// DERIVATION TRACE EDGE CASES
// =============================================================================

func TestShadow_BuildDerivationTrace_EmptyFact(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	result := m.buildDerivationTrace("")
	if !strings.Contains(result, "Derivation Trace") {
		t.Error("Expected header even for empty fact")
	}
}

func TestShadow_BuildDerivationTrace_SpecialCharacters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	result := m.buildDerivationTrace("test:with:colons")
	if !strings.Contains(result, "Derivation Trace") {
		t.Error("Expected header even for fact with special chars")
	}
}

func TestShadow_BuildDerivationTrace_LongFactName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow test requiring kernel in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	longFact := strings.Repeat("a", 1000)
	result := m.buildDerivationTrace(longFact)
	if !strings.Contains(result, "Derivation Trace") {
		t.Error("Expected header even for very long fact name")
	}
}

// =============================================================================
// PERFORMANCE TESTS
// =============================================================================

func TestShadow_Performance_BuildTrace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	iterations := 100
	perf.Track("build_trace_100x", func() {
		for i := 0; i < iterations; i++ {
			_ = m.buildDerivationTrace("config_value")
		}
	})
}

func TestShadow_Performance_RenderLogicPane(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	logicPane := ui.NewLogicPane(m.styles, 80, 24)
	m.logicPane = &logicPane

	iterations := 100
	perf.Track("render_logic_pane_100x", func() {
		for i := 0; i < iterations; i++ {
			_ = m.renderLogicPane()
		}
	})
}

// =============================================================================
// RESULT FORMAT TESTS
// =============================================================================

func TestShadow_SimulationResultFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shadow result format test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	if m.kernel != nil {
		m.shadowMode = core.NewShadowMode(m.kernel)
	}

	if m.shadowMode == nil {
		t.Skip("Shadow mode not available")
	}

	cmd := m.runShadowSimulation("test action")
	if cmd == nil {
		t.Fatal("Expected command, got nil")
	}

	result := cmd()
	if resp, ok := result.(responseMsg); ok {
		content := string(resp)

		// Check expected sections
		sections := []string{
			"Shadow Mode Simulation",
			"Hypothetical",
		}

		for _, section := range sections {
			if !strings.Contains(content, section) {
				t.Errorf("Expected section %q in response", section)
			}
		}
	}
}

func TestShadow_WhatIfResultFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping what-if result format test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	if m.kernel != nil {
		m.shadowMode = core.NewShadowMode(m.kernel)
	}

	if m.shadowMode == nil {
		t.Skip("Shadow mode not available")
	}

	cmd := m.runWhatIfQuery("test change")
	if cmd == nil {
		t.Fatal("Expected command, got nil")
	}

	result := cmd()
	if resp, ok := result.(responseMsg); ok {
		content := string(resp)

		// Check expected sections
		sections := []string{
			"What-If Analysis",
			"Change",
			"Recommendations",
		}

		for _, section := range sections {
			if !strings.Contains(content, section) {
				t.Errorf("Expected section %q in response", section)
			}
		}

		// Should always have "Run tests" recommendation
		if !strings.Contains(content, "Run tests") {
			t.Error("Expected 'Run tests' recommendation")
		}
	}
}

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestShadow_ConcurrentTraceBuilding(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	done := make(chan bool)
	perf.Track("concurrent_traces", func() {
		for i := 0; i < 5; i++ {
			go func(idx int) {
				_ = m.buildDerivationTrace("config_value")
				done <- true
			}(i)
		}

		for i := 0; i < 5; i++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("Timeout waiting for goroutine")
			}
		}
	})
}
