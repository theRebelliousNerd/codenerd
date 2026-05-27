package chat

import (
	"fmt"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyMsg processes all keyboard input for the Update() function.
// Returns (model, cmd, handled) where handled=true means the caller should
// return immediately with the given model and cmd, and handled=false means
// the key was not fully consumed (e.g., fell through to textarea update).
func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd, bool) {

	// Global exit keys are ALWAYS honored, even when the error panel is focused
	// or a non-chat view is active. These must never be swallowed by focus state.
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyCtrlD:
		// Graceful shutdown before quit
		m.performShutdown()
		return m, tea.Quit, true
	}

	// If the error panel is focused, capture scroll keys.
	// Global exit keys handled above; Ctrl+X (stop) is also honored here so
	// users can interrupt an in-flight operation without first un-focusing.
	if m.focusError && m.err != nil && m.showError && !msg.Alt {
		switch msg.Type {
		case tea.KeyCtrlX:
			// Fall through to global Ctrl+X handler below.
		case tea.KeyEsc:
			m.focusError = false
			return m, nil, true
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
			var errCmd tea.Cmd
			m.errorVP, errCmd = m.errorVP.Update(msg)
			return m, errCmd, true
		default:
			// Swallow other keys while focused to avoid editing input accidentally.
			return m, nil, true
		}
	}

	// Global Keybindings (Ctrl+X, Shift+Tab)
	// Esc is intentionally NOT handled globally — each view handles its own
	// Esc below so view-specific cleanup (e.g., campaign pause) can run.
	switch msg.Type {
	case tea.KeyCtrlX:
		// Ctrl+X: Stop current activity immediately
		if m.isLoading {
			m.isInterrupted = true
			if m.kernel != nil {
				_ = m.kernel.Assert(core.Fact{Predicate: "interrupt_requested", Args: nil})
			}
			m.isLoading = false
			stepMsg := ""
			if m.continuationTotal > 0 {
				stepMsg = fmt.Sprintf(" at step %d/%d", m.continuationStep, m.continuationTotal)
			}
			m = m.pushAssistantMsg(fmt.Sprintf("⏹️ Stopped%s. Type '/continue' to resume or give new instructions.", stepMsg))
			return m, nil, true
		}

	case tea.KeyShiftTab:
		// Shift+Tab: Cycle continuation mode (A → B → C → A)
		m.continuationMode = (m.continuationMode + 1) % 3
		modeName := m.continuationMode.String()
		modeChar := 'A' + rune(m.continuationMode)
		m.statusMessage = fmt.Sprintf("Mode: [%c] %s", modeChar, modeName)
		// Persist to config
		if m.Config != nil {
			m.Config.ContinuationMode = int(m.continuationMode)
			_ = m.Config.Save(config.DefaultUserConfigPath())
		}
		return m, nil, true
	}

	// Esc in ChatView is a no-op (use Ctrl+C to quit). For non-chat views,
	// fall through to the view-specific Esc handlers below.
	if msg.Type == tea.KeyEsc && m.viewMode == ChatView {
		return m, nil, true
	}

	// List View Handling
	if m.viewMode == ListView {
		// Esc returns to chat (preserve original global-handler behavior)
		if msg.Type == tea.KeyEsc {
			m.viewMode = ChatView
			return m, nil, true
		}
		// Check for Enter to select session
		if msg.Type == tea.KeyEnter {
			if selected, ok := m.list.SelectedItem().(sessionItem); ok {
				model, cmd := m.loadSelectedSession(selected.id)
				return model.(Model), cmd, true
			}
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd, true
	}

	// File Picker View Handling
	if m.viewMode == FilePickerView {
		// Esc returns to chat (preserve original global-handler behavior)
		if msg.Type == tea.KeyEsc {
			m.viewMode = ChatView
			m.filepicker = filepicker.New()
			return m, nil, true
		}
		var cmd tea.Cmd
		m.filepicker, cmd = m.filepicker.Update(msg)

		// Check for selection
		if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
			// File selected!
			m.textarea.SetValue(fmt.Sprintf("/read %s", path))
			m.viewMode = ChatView
			m.filepicker = filepicker.New() // Reset for next time (optional, but good practice)
			return m, cmd, true
		}

		// Check for disabled selection (optional warning)
		if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
			m.err = fmt.Errorf("file %s is disabled", path)
			m.showError = true
			m.focusError = false
			m.refreshErrorViewport()
			m.errorVP.GotoTop()
			return m, cmd, true
		}

		return m, cmd, true
	}

	if m.viewMode == UsageView {
		// Esc returns to chat (preserve original global-handler behavior)
		if msg.Type == tea.KeyEsc {
			m.viewMode = ChatView
			return m, nil, true
		}
		var cmd tea.Cmd
		m.usagePage, cmd = m.usagePage.Update(msg)
		return m, cmd, true
	}

	// Campaign Page Handling
	if m.viewMode == CampaignPage {
		// Direct Control Plane
		switch msg.String() {
		case "esc", "q":
			m.viewMode = ChatView
			return m, nil, true
		case " ":
			// Toggle Pause/Resume
			if m.activeCampaign != nil && m.campaignOrch != nil {
				if m.activeCampaign.Status == campaign.StatusPaused {
					m.campaignOrch.Resume()
				} else {
					m.campaignOrch.Pause()
				}
				// Force status update visibility immediately
				m.campaignPage.UpdateContent(m.campaignProgress, m.activeCampaign)
			}
			return m, nil, true
		}

		// Forward other keys (scrolling) to the page model
		var cmd tea.Cmd
		m.campaignPage, cmd = m.campaignPage.Update(msg)
		return m, cmd, true
	}

	// Prompt Inspector Handling
	if m.viewMode == PromptInspector {
		switch msg.String() {
		case "esc", "q":
			m.viewMode = ChatView
			return m, nil, true
		}
		var cmd tea.Cmd
		m.jitPage, cmd = m.jitPage.Update(msg)
		return m, cmd, true
	}

	// Autopoiesis Dashboard Handling
	if m.viewMode == AutopoiesisPage {
		switch msg.String() {
		case "esc", "q":
			m.viewMode = ChatView
			return m, nil, true
		}
		var cmd tea.Cmd
		m.autoPage, cmd = m.autoPage.Update(msg)
		return m, cmd, true
	}

	// Shard Console Handling
	if m.viewMode == ShardPage {
		switch msg.String() {
		case "esc", "q":
			m.viewMode = ChatView
			return m, nil, true
		}
		// Refresh content on every update tick or keypress to keep it live
		if m.shardMgr != nil {
			m.shardPage.UpdateContent(m.shardMgr.GetActiveShards(), m.shardMgr.GetBackpressureStatus())
		}
		var cmd tea.Cmd
		m.shardPage, cmd = m.shardPage.Update(msg)
		return m, cmd, true
	}

	// Chat View Handling
	switch msg.Type {

	case tea.KeyEnter:
		// Allow Alt+Enter for newlines
		if msg.Alt {
			// Let textarea handle it (fall through)
			break
		}

		// Bracketed paste: don't submit on Enter during paste, insert newline instead
		if msg.Paste {
			break // Let textarea handle it as newline
		}

		// Logic pane: Enter toggles expand/collapse
		if m.splitPane != nil && m.splitPane.FocusRight && m.logicPane != nil {
			m.logicPane.ToggleExpand()
			return m, nil, true
		}

		// Enter sends the message if not loading
		if !m.isLoading {
			if m.inputMode == InputModeClarification {
				model, cmd := m.handleClarificationResponse()
				return model.(Model), cmd, true
			}
			model, cmd := m.handleSubmit()
			return model.(Model), cmd, true
		}

	case tea.KeyUp:
		// Logic pane navigation when focused
		if m.splitPane != nil && m.splitPane.FocusRight && m.logicPane != nil {
			m.logicPane.SelectPrev()
			return m, nil, true
		}

		// Navigate options when in clarification mode
		if m.inputMode == InputModeClarification && m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
			if m.selectedOption > 0 {
				m.selectedOption--
			}
			return m, nil, true
		}

		// History Previous (if at top line)
		if m.textarea.Line() == 0 {
			if m.historyIndex > 0 {
				m.historyIndex--
				m.textarea.SetValue(m.inputHistory[m.historyIndex])
				// Move cursor to end
				m.textarea.CursorEnd()
			}
			return m, nil, true
		}

	case tea.KeyDown:
		// Logic pane navigation when focused
		if m.splitPane != nil && m.splitPane.FocusRight && m.logicPane != nil {
			m.logicPane.SelectNext()
			return m, nil, true
		}

		// Navigate options when in clarification mode
		if m.inputMode == InputModeClarification && m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
			if m.selectedOption < len(m.clarificationState.Options)-1 {
				m.selectedOption++
			}
			return m, nil, true
		}

		// History Next (if at bottom line)
		if m.textarea.Line() == m.textarea.LineCount()-1 {
			if m.historyIndex < len(m.inputHistory) {
				m.historyIndex++
				if m.historyIndex == len(m.inputHistory) {
					m.textarea.SetValue("")
				} else {
					m.textarea.SetValue(m.inputHistory[m.historyIndex])
					m.textarea.CursorEnd()
				}
			}
			return m, nil, true
		}

	case tea.KeyTab:
		// Tab cycles through options
		if m.inputMode == InputModeClarification && m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
			m.selectedOption = (m.selectedOption + 1) % len(m.clarificationState.Options)
			return m, nil, true
		}
	}

	// Handle Alt key bindings
	if msg.Alt && len(msg.Runes) > 0 {
		switch msg.Runes[0] {
		case 'e', 'E':
			// Error panel controls
			// - Alt+E: toggle focus (enables scrolling)
			// - Alt+Shift+E: toggle visibility
			if m.err != nil {
				if msg.Runes[0] == 'E' {
					m.showError = !m.showError
					if !m.showError {
						m.focusError = false
					}
					return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }, true
				}

				if m.showError {
					m.focusError = !m.focusError
				} else {
					m.showError = true
					m.focusError = true
					return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }, true
				}
			}
			return m, nil, true

		case 'l':
			// Toggle logic pane (Alt+L)
			m.showLogic = !m.showLogic
			var cmd tea.Cmd
			if m.showLogic {
				m.paneMode = ui.ModeSplitPane
				m.splitPane.SetMode(ui.ModeSplitPane)
				cmd = m.fetchTrace("") // Fetch default trace
			} else {
				m.paneMode = ui.ModeSinglePane
				m.splitPane.SetMode(ui.ModeSinglePane)
			}
			// Trigger resize to update viewport width
			cmd = tea.Batch(cmd, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			})
			return m, cmd, true

		case 'g':
			// Cycle through pane modes (Alt+G)
			switch m.paneMode {
			case ui.ModeSinglePane:
				m.paneMode = ui.ModeSplitPane
				m.showLogic = true
			case ui.ModeSplitPane:
				m.paneMode = ui.ModeFullLogic
			case ui.ModeFullLogic:
				m.paneMode = ui.ModeSinglePane
				m.showLogic = false
			}
			m.splitPane.SetMode(m.paneMode)
			return m, nil, true

		case 'w':
			// Toggle focus (Alt+W)
			if m.paneMode == ui.ModeSplitPane {
				m.splitPane.ToggleFocus()
			}
			return m, nil, true

		case 'c':
			// Toggle campaign progress panel (Alt+C)
			m.showCampaignPanel = !m.showCampaignPanel
			return m, nil, true

		case 'm':
			// Toggle mouse capture (Alt+M) - disable to allow text selection
			m.mouseEnabled = !m.mouseEnabled
			if m.mouseEnabled {
				return m, tea.EnableMouseCellMotion, true
			}
			return m, tea.DisableMouse, true

		case 'p':
			// Toggle Prompt Inspector (Alt+P)
			if m.viewMode == PromptInspector {
				m.viewMode = ChatView
			} else {
				m.viewMode = PromptInspector
				if m.jitCompiler != nil {
					m.jitPage.UpdateContent(m.jitCompiler.GetLastResult())
				}
			}
			return m, nil, true

		case 'a':
			// Toggle Autopoiesis Dashboard (Alt+A)
			if m.viewMode == AutopoiesisPage {
				m.viewMode = ChatView
			} else {
				m.viewMode = AutopoiesisPage
				if m.autopoiesis != nil {
					m.autoPage.UpdateContent(m.autopoiesis.GetAllPatterns(0.0), m.autopoiesis.GetAllLearnings())
				}
			}
			return m, nil, true

		case 's':
			// Toggle Shard Console (Alt+S)
			if m.viewMode == ShardPage {
				m.viewMode = ChatView
			} else {
				m.viewMode = ShardPage
				if m.shardMgr != nil {
					m.shardPage.UpdateContent(m.shardMgr.GetActiveShards(), m.shardMgr.GetBackpressureStatus())
				}
			}
			return m, nil, true

		case 'd':
			// Toggle Glass Box Debug Mode (Alt+D)
			debugMsg := m.toggleGlassBox()
			m = m.pushAssistantMsg(debugMsg)
			// Start listening for events if enabled
			if m.glassBoxEnabled {
				return m, m.listenGlassBoxEvents(), true
			}
			return m, nil, true

		case 'y':
			// Toggle system action summaries in chat output (Alt+Y)
			m.showSystemActions = !m.showSystemActions
			return m, nil, true
		}
	}

	// Handle regular key input — not fully handled, let Update() continue
	if !m.isLoading {
		var tiCmd tea.Cmd
		m.textarea, tiCmd = m.textarea.Update(msg)
		return m, tiCmd, false
	}

	return m, nil, false
}
