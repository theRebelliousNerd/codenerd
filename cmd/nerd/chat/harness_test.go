package chat

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHarness_Stability verifies the TUI model handles state transitions without panicking.
func TestHarness_Stability(t *testing.T) {
	// 1. Initialize
	cfg := Config{}
	model := InitChat(cfg)

	// Verify initial state
	if !model.isBooting {
		t.Error("Model should be in booting state initially")
	}

	// 2. Test Window Resizing (Common source of layout panics)
	t.Run("WindowResize", func(t *testing.T) {
		newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
		m, ok := newModel.(Model)
		if !ok {
			t.Fatal("Model type assertion failed")
		}
		if m.width != 100 || m.height != 50 {
			t.Errorf("Resize failed: got %dx%d, want 100x50", m.width, m.height)
		}
		// Test View() doesn't panic
		_ = m.View()
	})

	// 3. Test Boot Completion (Simulated)
	t.Run("BootCompletion", func(t *testing.T) {
		// Mock minimal components to prevent nil dereferences
		components := &SystemComponents{
			InitialMessages: []Message{
				{Role: "assistant", Content: "System Ready", Time: time.Now()},
			},
			Workspace: "/tmp/test",
		}

		msg := bootCompleteMsg{
			components: components,
			err:        nil,
		}

		newModel, _ := model.Update(msg)
		m, ok := newModel.(Model)
		if !ok {
			t.Fatal("Model type assertion failed")
		}

		if !m.isBooting {
			t.Error("Model should still be booting (scanning phase) after bootCompleteMsg")
		}
		if m.bootStage != BootStageScanning {
			t.Errorf("Expected BootStageScanning, got %d", m.bootStage)
		}

		// 4. Test Scan Completion
		scanMsg := scanCompleteMsg{
			fileCount: 10,
			factCount: 100,
			duration:  1 * time.Second,
			err:       nil,
		}
		newModel, _ = m.Update(scanMsg)
		m, ok = newModel.(Model)
		if !ok {
			t.Fatal("Model type assertion failed")
		}

		if m.isBooting {
			t.Error("Model should NOT be booting after scanCompleteMsg")
		}

		// Ensure no nil panic on View() after boot
		_ = m.View()
	})
}

// TestHarness_InputValidation verifies input handling.
func TestHarness_InputValidation(t *testing.T) {
	model := InitChat(Config{})
	// Simulate boot completion to enable input
	model.isBooting = false
	model.ready = true

	t.Run("EmptyInput", func(t *testing.T) {
		// Simulate Enter key on empty input
		msg := tea.KeyMsg{Type: tea.KeyEnter}
		newModel, _ := model.Update(msg)
		m := newModel.(Model)

		// Should not add empty message to history (except maybe a newline in textarea)
		// This depends on exact implementation, but mostly ensuring no panic
		_ = m.View()
	})
}

// TestHarness_Shutdown verifies graceful shutdown.
func TestHarness_Shutdown(t *testing.T) {
	model := InitChat(Config{})

	t.Run("GracefulShutdown", func(t *testing.T) {
		// Create a done channel to detect hangs
		done := make(chan struct{})
		go func() {
			model.Shutdown()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Error("Shutdown timed out")
		}
	})

	t.Run("IdempotentShutdown", func(t *testing.T) {
		// Should not panic on second call
		model.Shutdown()
	})
}

// TestHarness_ConfigLoading verifies config loading.
func TestHarness_ConfigLoading(t *testing.T) {
	// Config loading is handled implicitly by InitChat
	model := InitChat(Config{})
	if model.Config == nil {
		t.Error("Config should be initialized")
	}
}
