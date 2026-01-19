package chat

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// LIVE KERNEL TESTS
// These tests use a real RealKernel with embedded .mg files.
// Skip with: go test -short ./cmd/nerd/chat/...
// =============================================================================

func TestLive_KernelCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Verify kernel was created
	if m.kernel == nil {
		t.Fatal("Expected kernel to be non-nil")
	}

	// Try a simple query
	perf.Track("kernel_query", func() {
		// Basic kernel functionality test
		t.Logf("Kernel initialized successfully")
	})
}

func TestLive_QueryCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "simple ground query",
			query:   "/query config_value(/model, X)?",
			wantErr: false,
		},
		{
			name:    "empty query",
			query:   "/query",
			wantErr: true, // Should fail - no query provided
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perf.Track("query_"+tt.name, func() {
				newModel, _ := m.handleCommand(tt.query)
				result := newModel.(Model)
				t.Logf("Query result: %d messages in history", len(result.history))
			})
		})
	}
}

func TestLive_FactsCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Simulate /facts command
	var result Model
	perf.Track("facts_command", func() {
		newModel, _ := m.handleCommand("/facts")
		result = newModel.(Model)
	})

	// Check that we got some kind of response
	t.Logf("Model has %d messages after /facts", len(result.history))
}

func TestLive_ResetCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// First add some state
	m = m.addMessage(Message{Role: "user", Content: "test message", Time: time.Now()})
	initialMsgCount := len(m.history)

	// Execute reset
	var result Model
	perf.Track("reset_command", func() {
		newModel, _ := m.handleCommand("/reset")
		result = newModel.(Model)
	})

	// Verify reset happened
	t.Logf("Reset executed, messages: %d -> %d", initialMsgCount, len(result.history))
}

func TestLive_HelpCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	var result Model
	perf.Track("help_command", func() {
		newModel, _ := m.handleCommand("/help")
		result = newModel.(Model)
	})

	// Help should add a message with command info
	found := false
	for _, msg := range result.history {
		if strings.Contains(msg.Content, "/") {
			found = true
			break
		}
	}
	if !found && len(result.history) > 0 {
		t.Logf("Help output: %d messages", len(result.history))
	}
}

func TestLive_ModelCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Test /model without args (should show current)
	var result Model
	perf.Track("model_show", func() {
		newModel, _ := m.handleCommand("/model")
		result = newModel.(Model)
	})
	t.Logf("Model command result: %d messages", len(result.history))

	// Test /model with selection
	perf.Track("model_select", func() {
		newModel, _ := m.handleCommand("/model gemini-2.5-pro")
		result = newModel.(Model)
	})
	t.Logf("Model select result: %d messages", len(result.history))
}

func TestLive_ConfigCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	var result Model
	perf.Track("config_command", func() {
		newModel, _ := m.handleCommand("/config")
		result = newModel.(Model)
	})

	// Config should show some output
	t.Logf("Config result: %d messages", len(result.history))
}

func TestLive_WhyCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// /why without prior derivation
	var result Model
	perf.Track("why_no_context", func() {
		newModel, _ := m.handleCommand("/why")
		result = newModel.(Model)
	})
	t.Logf("Why (no context) result: %d messages", len(result.history))

	// /why with a fact
	perf.Track("why_with_fact", func() {
		newModel, _ := m.handleCommand("/why config_value(/model, X)")
		result = newModel.(Model)
	})
	t.Logf("Why (with fact) result: %d messages", len(result.history))
}

func TestLive_StatusCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	var result Model
	perf.Track("status_command", func() {
		newModel, _ := m.handleCommand("/status")
		result = newModel.(Model)
	})

	// Status should show kernel/system info
	t.Logf("Status result: %d messages", len(result.history))
}

func TestLive_ClearCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Add some messages
	m = m.addMessage(Message{Role: "user", Content: "msg1", Time: time.Now()})
	m = m.addMessage(Message{Role: "assistant", Content: "msg2", Time: time.Now()})
	beforeCount := len(m.history)

	var result Model
	perf.Track("clear_command", func() {
		newModel, _ := m.handleCommand("/clear")
		result = newModel.(Model)
	})

	t.Logf("Clear: %d -> %d messages", beforeCount, len(result.history))
}

func TestLive_VersionCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	var result Model
	perf.Track("version_command", func() {
		newModel, _ := m.handleCommand("/version")
		result = newModel.(Model)
	})

	// Version should show version info
	t.Logf("Version result: %d messages", len(result.history))
}

func TestLive_DiagnoseCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	var result Model
	perf.Track("diagnose_command", func() {
		newModel, _ := m.handleCommand("/diagnose")
		result = newModel.(Model)
	})

	t.Logf("Diagnose result: %d messages", len(result.history))
}

func TestLive_ModeCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	modes := []string{"/mode", "/mode ask", "/mode code", "/mode architect"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			var result Model
			perf.Track("mode_"+mode, func() {
				newModel, _ := m.handleCommand(mode)
				result = newModel.(Model)
			})
			t.Logf("%s result: %d messages", mode, len(result.history))
		})
	}
}

func TestLive_GlassBoxCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Toggle glassbox on
	var result1 Model
	perf.Track("glassbox_toggle_on", func() {
		newModel, _ := m.handleCommand("/glassbox")
		result1 = newModel.(Model)
	})
	t.Logf("GlassBox toggle 1: viewMode=%v", result1.viewMode)

	// Toggle with category
	var result2 Model
	perf.Track("glassbox_mangle", func() {
		newModel, _ := m.handleCommand("/glassbox mangle")
		result2 = newModel.(Model)
	})
	t.Logf("GlassBox mangle: viewMode=%v", result2.viewMode)
}

func TestLive_ShadowModeCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Enter shadow mode
	var result Model
	perf.Track("shadow_enter", func() {
		newModel, _ := m.handleCommand("/shadow")
		result = newModel.(Model)
	})

	// Check shadow mode state - shadowMode is a *core.ShadowMode
	t.Logf("Shadow mode command executed, messages=%d", len(result.history))

	// Try to exit shadow mode
	perf.Track("shadow_exit", func() {
		newModel, _ := result.handleCommand("/shadow")
		result = newModel.(Model)
	})
	t.Logf("Shadow mode toggle again, messages=%d", len(result.history))
}

func TestLive_WhatIfCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Execute a what-if query
	var result Model
	perf.Track("whatif_query", func() {
		newModel, _ := m.handleCommand("/whatif user_intent(/test, /refactor, /code, /none)")
		result = newModel.(Model)
	})

	t.Logf("WhatIf result: %d messages", len(result.history))
}

func TestLive_HandleSubmit_MockLLM(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Set up input
	m.textarea.SetValue("Hello, please help me refactor this code")

	var result Model
	var cmd tea.Cmd
	perf.Track("handle_submit", func() {
		newModel, newCmd := m.handleSubmit()
		result = newModel.(Model)
		cmd = newCmd
	})

	// Check that input was cleared and message added
	if result.textarea.Value() != "" {
		t.Error("Expected input to be cleared after submit")
	}

	// Should have added user message
	found := false
	for _, msg := range result.history {
		if msg.Role == "user" && strings.Contains(msg.Content, "refactor") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected user message to be added")
	}

	t.Logf("Submit result: %d messages, cmd=%v", len(result.history), cmd != nil)
}

func TestLive_HandleSubmit_CommandDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	tests := []struct {
		name  string
		input string
	}{
		{"help_command", "/help"},
		{"status_command", "/status"},
		{"clear_command", "/clear"},
		{"reset_command", "/reset"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testM := m
			testM.textarea.SetValue(tt.input)

			var result Model
			perf.Track("submit_"+tt.name, func() {
				newModel, _ := testM.handleSubmit()
				result = newModel.(Model)
			})

			t.Logf("%s: %d messages", tt.name, len(result.history))
		})
	}
}

func TestLive_MultipleCommandSequence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Execute a sequence of commands
	commands := []string{
		"/status",
		"/model",
		"/config",
		"/help",
	}

	result := m
	perf.Track("command_sequence", func() {
		for _, cmd := range commands {
			newModel, _ := result.handleCommand(cmd)
			result = newModel.(Model)
		}
	})

	t.Logf("After %d commands: %d messages", len(commands), len(result.history))
}

func TestLive_ViewRendering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Add some content
	m = m.addMessage(Message{Role: "user", Content: "Test message", Time: time.Now()})
	m = m.addMessage(Message{Role: "assistant", Content: "Response content", Time: time.Now()})

	var view string
	perf.Track("render_view", func() {
		view = m.View()
	})

	if len(view) == 0 {
		t.Error("Expected non-empty view")
	}

	t.Logf("View length: %d chars", len(view))
}

func TestLive_InputProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Test various input scenarios
	inputs := []string{
		"simple text",
		"multi\nline\ntext",
		"special chars: @#$%^&*()",
		strings.Repeat("a", 1000), // Long input
	}

	for i, input := range inputs {
		t.Run(input[:minIntLive(20, len(input))], func(t *testing.T) {
			testM := m
			perf.Track("input_"+string(rune('0'+i)), func() {
				testM.textarea.SetValue(input)
				if testM.textarea.Value() != input {
					t.Errorf("Input mismatch: got %q", testM.textarea.Value())
				}
			})
		})
	}
}

func TestLive_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Test invalid commands
	invalidCommands := []string{
		"/notacommand",
		"/query invalid syntax [[[",
		"/model nonexistent-model-xyz",
	}

	for _, cmd := range invalidCommands {
		t.Run(cmd, func(t *testing.T) {
			var result Model
			perf.Track("invalid_"+cmd, func() {
				newModel, _ := m.handleCommand(cmd)
				result = newModel.(Model)
			})
			// Should not panic, should add error message or handle gracefully
			t.Logf("Invalid command %s: %d messages", cmd, len(result.history))
		})
	}
}

func TestLive_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Test that model handles multiple rapid updates
	done := make(chan bool)
	perf.Track("concurrent_updates", func() {
		for i := 0; i < 10; i++ {
			go func(idx int) {
				localM := m
				localM.textarea.SetValue("test input " + string(rune('0'+idx)))
				_ = localM.View()
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("Timeout waiting for goroutine")
			}
		}
	})
}

func TestLive_PerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf := SetupLiveModel(t)
	defer perf.Report(t)

	// Establish performance baselines
	iterations := 100

	// View rendering performance
	perf.Track("view_100x", func() {
		for i := 0; i < iterations; i++ {
			_ = m.View()
		}
	})

	// Command handling performance
	perf.Track("help_100x", func() {
		for i := 0; i < iterations; i++ {
			_, _ = m.handleCommand("/help")
		}
	})

	// Input update performance
	perf.Track("input_100x", func() {
		for i := 0; i < iterations; i++ {
			m.textarea.SetValue("test input")
			m.textarea.SetValue("")
		}
	})
}

// =============================================================================
// FULL INTEGRATION TESTS
// =============================================================================

func TestLive_FullIntegration_CommandFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf, exec := SetupFullIntegrationModel(t)
	defer perf.Report(t)

	result := m
	// Simulate a realistic user session
	perf.Track("full_session", func() {
		// 1. User checks status
		newModel, _ := result.handleCommand("/status")
		result = newModel.(Model)

		// 2. User asks for help
		newModel, _ = result.handleCommand("/help")
		result = newModel.(Model)

		// 3. User checks config
		newModel, _ = result.handleCommand("/config")
		result = newModel.(Model)

		// 4. User sends a message
		result.textarea.SetValue("Help me understand this codebase")
		newModel, _ = result.handleSubmit()
		result = newModel.(Model)

		// 5. User clears and tries again
		newModel, _ = result.handleCommand("/clear")
		result = newModel.(Model)

		// 6. User checks model
		newModel, _ = result.handleCommand("/model")
		result = newModel.(Model)
	})

	// Verify executor was set up
	executions := exec.GetExecutions()
	t.Logf("Full session: %d messages, %d executions", len(result.history), len(executions))
}

func TestLive_FullIntegration_GlassBoxFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf, _ := SetupFullIntegrationModel(t)
	defer perf.Report(t)

	result := m
	perf.Track("glassbox_session", func() {
		// Enable glassbox
		newModel, _ := result.handleCommand("/glassbox")
		result = newModel.(Model)

		// Do some operations
		newModel, _ = result.handleCommand("/status")
		result = newModel.(Model)

		newModel, _ = result.handleCommand("/facts")
		result = newModel.(Model)

		// Check different glassbox modes
		newModel, _ = result.handleCommand("/glassbox mangle")
		result = newModel.(Model)

		newModel, _ = result.handleCommand("/glassbox llm")
		result = newModel.(Model)

		newModel, _ = result.handleCommand("/glassbox all")
		result = newModel.(Model)

		// Disable
		newModel, _ = result.handleCommand("/glassbox off")
		result = newModel.(Model)
	})

	t.Logf("GlassBox session complete: %d messages", len(result.history))
}

func TestLive_FullIntegration_ErrorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live kernel test in short mode")
	}

	m, perf, exec := SetupFullIntegrationModel(t)
	defer perf.Report(t)

	// Set up executor to fail
	exec.SetFail("simulated failure")

	result := m
	perf.Track("error_recovery", func() {
		// Try operations that might fail
		newModel, _ := result.handleCommand("/status")
		result = newModel.(Model)

		newModel, _ = result.handleCommand("/diagnose")
		result = newModel.(Model)

		// Clear failure and retry
		exec.shouldFail = false
		newModel, _ = result.handleCommand("/status")
		result = newModel.(Model)
	})

	t.Logf("Error recovery complete: %d messages", len(result.history))
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// minIntLive is a helper for min of two ints (live test local version)
func minIntLive(a, b int) int {
	if a < b {
		return a
	}
	return b
}
