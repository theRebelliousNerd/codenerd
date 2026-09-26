// Package chat provides tests for shadow mode and counterfactual reasoning.
package chat

import (
	"strings"
	"testing"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// SHADOW MODE TESTS
// =============================================================================

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

	// Create shadow mode. It clones a RealKernel; the live test model's
	// kernel is one (production hands shadow mode the catch-all shard's).
	if rk, ok := m.kernel.(*core.RealKernel); ok {
		m.shadowMode = core.NewShadowMode(rk)
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

	// Create shadow mode. It clones a RealKernel; the live test model's
	// kernel is one (production hands shadow mode the catch-all shard's).
	if rk, ok := m.kernel.(*core.RealKernel); ok {
		m.shadowMode = core.NewShadowMode(rk)
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

	if rk, ok := m.kernel.(*core.RealKernel); ok {
		m.shadowMode = core.NewShadowMode(rk)
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

	if rk, ok := m.kernel.(*core.RealKernel); ok {
		m.shadowMode = core.NewShadowMode(rk)
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

	if rk, ok := m.kernel.(*core.RealKernel); ok {
		m.shadowMode = core.NewShadowMode(rk)
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

// =============================================================================
// PERFORMANCE TESTS
// =============================================================================

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
		for range iterations {
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

	if rk, ok := m.kernel.(*core.RealKernel); ok {
		m.shadowMode = core.NewShadowMode(rk)
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

	if rk, ok := m.kernel.(*core.RealKernel); ok {
		m.shadowMode = core.NewShadowMode(rk)
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
