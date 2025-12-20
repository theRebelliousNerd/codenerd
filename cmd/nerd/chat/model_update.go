package chat

import (
	"fmt"
	"strings"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/transparency"
	"codenerd/internal/ux"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		errCmd tea.Cmd
		spCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If the error panel is focused, capture scroll keys first.
		// Keep global keys (Ctrl+C, etc.) handled normally.
		if m.focusError && m.err != nil && m.showError && !msg.Alt {
			switch msg.Type {
			case tea.KeyEsc:
				m.focusError = false
				return m, nil
			case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
				m.errorVP, errCmd = m.errorVP.Update(msg)
				return m, errCmd
			default:
				// Swallow other keys while focused to avoid editing input accidentally.
				return m, nil
			}
		}

		// Global Keybindings (Ctrl+C, Ctrl+X, Shift+Tab, Esc)
		switch msg.Type {
		case tea.KeyCtrlC:
			// Graceful shutdown before quit
			m.performShutdown()
			return m, tea.Quit

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
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("⏹️ Stopped%s. Type '/continue' to resume or give new instructions.", stepMsg),
					Time:    time.Now(),
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
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
			return m, nil

		case tea.KeyEsc:
			if m.viewMode == ListView {
				m.viewMode = ChatView // Escape list view
				return m, nil
			}
			if m.viewMode == UsageView {
				m.viewMode = ChatView // Escape usage view
				return m, nil
			}
			// Only Quit if not in List View
			m.performShutdown()
			return m, tea.Quit
		}

		// List View Handling
		if m.viewMode == ListView {
			// Check for Enter to select session
			if msg.Type == tea.KeyEnter {
				if selected, ok := m.list.SelectedItem().(sessionItem); ok {
					return m.loadSelectedSession(selected.id)
				}
			}
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		// File Picker View Handling
		if m.viewMode == FilePickerView {
			var cmd tea.Cmd
			m.filepicker, cmd = m.filepicker.Update(msg)

			// Check for selection
			if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
				// File selected!
				m.textarea.SetValue(fmt.Sprintf("/read %s", path))
				m.viewMode = ChatView
				m.filepicker = filepicker.New() // Reset for next time (optional, but good practice)
				return m, cmd
			}

			// Check for disabled selection (optional warning)
			if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
				m.err = fmt.Errorf("file %s is disabled", path)
				m.showError = true
				m.focusError = false
				m.refreshErrorViewport()
				m.errorVP.GotoTop()
				return m, cmd
			}

			return m, cmd
		}

		if m.viewMode == UsageView {
			var cmd tea.Cmd
			m.usagePage, cmd = m.usagePage.Update(msg)
			return m, cmd
		}

		// Campaign Page Handling
		if m.viewMode == CampaignPage {
			// Direct Control Plane
			switch msg.String() {
			case "esc", "q":
				m.viewMode = ChatView
				return m, nil
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
				return m, nil
			}

			// Forward other keys (scrolling) to the page model
			var cmd tea.Cmd
			m.campaignPage, cmd = m.campaignPage.Update(msg)
			return m, cmd
		}

		// Prompt Inspector Handling
		if m.viewMode == PromptInspector {
			// Exit on Esc/Q
			if msg.String() == "esc" || msg.String() == "q" {
				m.viewMode = ChatView
				return m, nil
			}
			// Scroll handling (if we use viewport)
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		// Chat View Handling
		switch msg.Type {

		case tea.KeyEnter:
			// Allow Alt+Enter for newlines
			if msg.Alt {
				// Let textarea handle it
				break
			}

			// Bracketed paste: don't submit on Enter during paste, insert newline instead
			if msg.Paste {
				break // Let textarea handle it as newline
			}

			// Logic pane: Enter toggles expand/collapse
			if m.splitPane != nil && m.splitPane.FocusRight && m.logicPane != nil {
				m.logicPane.ToggleExpand()
				return m, nil
			}

			// Enter sends the message if not loading
			if !m.isLoading {
				if m.awaitingClarification {
					return m.handleClarificationResponse()
				}
				return m.handleSubmit()
			}

		case tea.KeyUp:
			// Logic pane navigation when focused
			if m.splitPane != nil && m.splitPane.FocusRight && m.logicPane != nil {
				m.logicPane.SelectPrev()
				return m, nil
			}

			// Navigate options when in clarification mode
			if m.awaitingClarification && m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
				if m.selectedOption > 0 {
					m.selectedOption--
				}
				return m, nil
			}

			// History Previous (if at top line)
			if m.textarea.Line() == 0 {
				if m.historyIndex > 0 {
					m.historyIndex--
					m.textarea.SetValue(m.inputHistory[m.historyIndex])
					// Move cursor to end
					m.textarea.CursorEnd()
				}
				return m, nil
			}

		case tea.KeyDown:
			// Logic pane navigation when focused
			if m.splitPane != nil && m.splitPane.FocusRight && m.logicPane != nil {
				m.logicPane.SelectNext()
				return m, nil
			}

			// Navigate options when in clarification mode
			if m.awaitingClarification && m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
				if m.selectedOption < len(m.clarificationState.Options)-1 {
					m.selectedOption++
				}
				return m, nil
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
				return m, nil
			}

		case tea.KeyTab:
			// Tab cycles through options
			if m.awaitingClarification && m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
				m.selectedOption = (m.selectedOption + 1) % len(m.clarificationState.Options)
				return m, nil
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
						return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }
					}

					if m.showError {
						m.focusError = !m.focusError
					} else {
						m.showError = true
						m.focusError = true
						return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }
					}
				}
				return m, nil

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
				return m, cmd

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
				return m, nil

			case 'w':
				// Toggle focus (Alt+W)
				if m.paneMode == ui.ModeSplitPane {
					m.splitPane.ToggleFocus()
				}
				return m, nil

			case 'c':
				// Toggle campaign progress panel (Alt+C)
				m.showCampaignPanel = !m.showCampaignPanel
				return m, nil

			case 'm':
				// Toggle mouse capture (Alt+M) - disable to allow text selection
				m.mouseEnabled = !m.mouseEnabled
				if m.mouseEnabled {
					return m, tea.EnableMouseCellMotion
				}
				return m, tea.DisableMouse

			case 'p':
				// Toggle Prompt Inspector (Alt+P)
				if m.viewMode == PromptInspector {
					m.viewMode = ChatView
				} else {
					m.viewMode = PromptInspector
					// Trigger content refresh for inspector
					if m.jitCompiler != nil {
						res := m.jitCompiler.GetLastResult()
						if res != nil {
							// Render manifest to viewport
							// We'll need a helper function renderManifest(res)
							// For now, simple textual dump
							content := fmt.Sprintf("# JIT Prompt Inspector\n\nGenerated: %s\nTokens: %d (%.1f%% budget)\n\n## Included Atoms (%d)\n",
								time.Now().Format(time.RFC3339), res.TotalTokens, res.BudgetUsed*100, res.AtomsIncluded)

							for _, atom := range res.IncludedAtoms {
								content += fmt.Sprintf("- [%s] %s (%d tokens)\n", atom.Category, atom.ID, atom.TokenCount)
							}

							content += "\n## Prompt Preview\n\n```markdown\n" + res.Prompt + "\n```"

							// Use existing renderer
							rendered, _ := m.renderer.Render(content)
							m.viewport.SetContent(rendered)
							m.viewport.GotoTop()
						} else {
							m.viewport.SetContent("No compilation result available yet.")
						}
					} else {
						m.viewport.SetContent("JIT Compiler not available.")
					}
				}
				return m, nil

			case 's':
				// Toggle system action summaries in chat output (Alt+S)
				m.showSystemActions = !m.showSystemActions
				return m, nil
			}
		}

		// Handle regular key input
		if !m.isLoading {
			m.textarea, tiCmd = m.textarea.Update(msg)
		}

	case windowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 4
		footerHeight := 3
		inputHeight := 3   // Smaller input height for textinput
		paddingHeight := 2 // Extra padding for safety

		// Calculate layout
		chatWidth := msg.Width - 4
		if m.showLogic {
			logicWidth := msg.Width / 3
			chatWidth = msg.Width - logicWidth - 4 // minus padding/borders
		}
		if chatWidth < 1 {
			chatWidth = 1
		}

		errorPanelHeight := 0
		if m.err != nil && m.showError {
			// 1 header line + viewport height + 2 border lines
			errorPanelHeight = 1 + errorPanelViewportHeight + 2
		}

		calcHeight := msg.Height - headerHeight - footerHeight - inputHeight - paddingHeight - errorPanelHeight
		if calcHeight < 1 {
			calcHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(chatWidth, calcHeight)
			m.ready = true
		} else {
			m.viewport.Width = chatWidth
			m.viewport.Height = calcHeight
		}

		// Error viewport lives inside a bordered box within the content area.
		// Box uses 1-col padding left/right plus 1-col border left/right => total 4 cols.
		m.errorVP.Width = chatWidth - 4
		if m.errorVP.Width < 1 {
			m.errorVP.Width = 1
		}
		m.errorVP.Height = errorPanelViewportHeight
		if m.err != nil {
			m.refreshErrorViewport()
		}

		// Reduce input width to accommodate border (2) + padding (2) + safety margin
		m.textarea.SetWidth(chatWidth - 4)

		// Update split pane dimensions
		if m.splitPane != nil {
			m.list.SetSize(msg.Width, msg.Height)
			m.filepicker.Height = msg.Height - 15
			m.splitPane.SetSize(msg.Width, msg.Height-headerHeight-footerHeight)
			m.usagePage.SetSize(msg.Width, msg.Height-headerHeight)
			m.campaignPage.SetSize(msg.Width, msg.Height-headerHeight)

		}
		if m.logicPane != nil {
			m.logicPane.SetSize(msg.Width/3, msg.Height-headerHeight-footerHeight-inputHeight-paddingHeight)
		}

		// Update renderer word wrap
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(chatWidth-4),
			)
			// Re-render history with new wrapping
			m.viewport.SetContent(m.renderHistory())
		}

	case tea.WindowSizeMsg:
		// Convert to our alias and re-process
		return m.Update(windowSizeMsg(msg))

	case clarificationReply:
		// Handle clarification reply
		return m, m.processClarificationResponse(string(msg), m.clarificationState.PendingIntent, m.clarificationState.Context)

	case spinner.TickMsg:
		if m.isLoading || m.isBooting {
			m.spinner, spCmd = m.spinner.Update(msg)
			return m, spCmd
		}

	case traceUpdateMsg:
		m.isLoading = false

		if m.logicPane != nil {
			m.logicPane.SetTraceMangle(msg.Trace)
		}

		// If ShowInChat is true (from /why command), show explanation in chat
		if msg.ShowInChat {
			explainer := transparency.NewExplainer()
			explanation := explainer.ExplainTrace(msg.Trace)

			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: explanation,
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
		}
		return m, nil

	case assistantMsg:
		m.isLoading = false
		m.turnCount++

		// Apply any state updates carried by the message
		if msg.ClarifyUpdate != nil {
			m.lastClarifyInput = msg.ClarifyUpdate.LastClarifyInput
			m.launchClarifyPending = msg.ClarifyUpdate.LaunchClarifyPending
			m.launchClarifyGoal = msg.ClarifyUpdate.LaunchClarifyGoal
			m.launchClarifyAnswers = msg.ClarifyUpdate.LaunchClarifyAnswers
		}
		if msg.DreamHypothetical != "" {
			m.lastDreamHypothetical = msg.DreamHypothetical
		}
		if msg.ShardResult != nil {
			m.storeShardResult(msg.ShardResult.ShardType, msg.ShardResult.Task, msg.ShardResult.Result, msg.ShardResult.Facts)
		}

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: msg.Surface,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()

	case responseMsg:
		m.isLoading = false
		m.turnCount++
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: string(msg),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		// Persist session after each response
		m.saveSessionState()

	case multiShardReviewMsg:
		// Multi-shard review completed
		m.isLoading = false
		m.turnCount++
		if msg.err != nil {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Multi-shard review failed: %v", msg.err),
				Time:    time.Now(),
			})
		} else if msg.review != nil {
			// Format and display the aggregated review
			content := formatMultiShardResponse(msg.review)
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: content,
				Time:    time.Now(),
			})
			m.storeAggregatedReviewResult(msg.review, content)
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()

	case clarificationMsg:
		// Enter clarification mode (Pause)
		m.isLoading = false
		m.awaitingClarification = true
		m.clarificationState = &ClarificationState{
			Question:      msg.Question,
			Options:       msg.Options,
			DefaultOption: msg.DefaultOption,
			Context:       msg.Context,
			PendingIntent: msg.PendingIntent,
		}
		m.selectedOption = 0
		if msg.Context != "" {
			m.lastClarifyInput = msg.Context
		}

		// Update UI to show clarification request
		m.textarea.Placeholder = "Select option or type your answer..."
		if len(msg.Options) > 0 {
			m.textarea.Placeholder = "Use ↑/↓ to select, Enter to confirm, or type custom answer..."
		}

		// Add clarification question to history
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: m.formatClarificationRequest(ClarificationState(msg)),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	case errorMsg:
		m.isLoading = false
		// Check if this is a clarification request disguised as an error
		if strings.Contains(msg.Error(), "USER_INPUT_REQUIRED") || strings.Contains(msg.Error(), "clarification") {
			// Extract the question from the error message
			question := extractClarificationQuestion(msg.Error())
			return m, func() tea.Msg {
				return clarificationMsg{
					Question: question,
					Options:  []string{},
				}
			}
		}
		m.err = msg
		m.showError = true
		m.focusError = false
		m.refreshErrorViewport()
		m.errorVP.GotoTop()
		logging.Get(logging.CategorySession).Error("TUI error: %v", msg)
		// Trigger resize so the error panel reserves space immediately.
		return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }

	case campaignErrorMsg:
		m.isLoading = false
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("## Campaign Error\n\n%v", msg.err),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	case northstarDocsAnalyzedMsg:
		m.isLoading = false
		if m.northstarWizard != nil {
			if msg.err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("⚠️ Document analysis encountered an error: %v\n\nContinuing without extracted insights.", msg.err),
					Time:    time.Now(),
				})
			} else if len(msg.facts) > 0 {
				m.northstarWizard.ExtractedFacts = msg.facts
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("## Research Analysis Complete\n\nExtracted **%d key insights** from your documents:\n\n", len(msg.facts)))
				for i, fact := range msg.facts {
					if i < 5 { // Show first 5
						sb.WriteString(fmt.Sprintf("- %s\n", fact))
					}
				}
				if len(msg.facts) > 5 {
					sb.WriteString(fmt.Sprintf("\n_...and %d more insights that will inform the process._\n", len(msg.facts)-5))
				}
				sb.WriteString("\n---\n\n## Phase 2: Problem Statement\n\n**What problem does this project solve?**\n\n_Your research insights will help refine this._")
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: sb.String(),
					Time:    time.Now(),
				})
			}
			m.northstarWizard.Phase = NorthstarProblemStatement
			m.textarea.Placeholder = "Describe the problem..."
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	case requirementsGeneratedMsg:
		m.isLoading = false
		if m.northstarWizard != nil {
			if msg.err != nil {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: fmt.Sprintf("⚠️ Requirement generation encountered an error: %v\n\nYou can add requirements manually.", msg.err),
					Time:    time.Now(),
				})
			} else if len(msg.requirements) > 0 {
				// Append generated requirements to wizard state
				m.northstarWizard.Requirements = append(m.northstarWizard.Requirements, msg.requirements...)
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("## Requirements Generated\n\nAdded **%d requirements** from your vision and capabilities:\n\n", len(msg.requirements)))
				for i, req := range msg.requirements {
					if i < 5 { // Show first 5
						sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", req.ID, req.Priority, req.Description))
					}
				}
				if len(msg.requirements) > 5 {
					sb.WriteString(fmt.Sprintf("\n_...and %d more requirements._\n", len(msg.requirements)-5))
				}
				sb.WriteString("\n_Add more requirements manually or type \"done\" to continue._")
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: sb.String(),
					Time:    time.Now(),
				})
			} else {
				m.history = append(m.history, Message{
					Role:    "assistant",
					Content: "No requirements could be auto-generated. Please add requirements manually.",
					Time:    time.Now(),
				})
			}
			m.textarea.Placeholder = "Add requirement or 'done'..."
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	// Campaign message handlers
	case campaignStartedMsg:
		m.isLoading = false
		m.activeCampaign = msg.campaign
		m.campaignOrch = msg.orch
		m.campaignProgressChan = msg.progressChan // Store channels for listening
		m.campaignEventChan = msg.eventChan
		m.showCampaignPanel = true
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: m.renderCampaignStarted(msg.campaign),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

		// Start orchestrator execution in background and return both listeners
		if m.campaignOrch != nil {
			return m, tea.Batch(
				m.runCampaignOrchestrator(),
				m.listenCampaignEvents(), // Listen for events in parallel
			)
		}

	case campaignProgressMsg:
		m.campaignProgress = msg
		// Update campaign panel without adding to history (live update)
		if m.activeCampaign != nil {
			m.activeCampaign.CompletedPhases = msg.CompletedPhases
			m.activeCampaign.CompletedTasks = msg.CompletedTasks
		}

		// Update Campaign Page
		if m.activeCampaign != nil {
			prog := campaign.Progress(*msg)
			m.campaignPage.UpdateContent(&prog, m.activeCampaign)
		}

		// Continue listening for progress updates via channel (not polling)
		if m.campaignProgressChan != nil && m.activeCampaign != nil {
			return m, m.listenCampaignProgress()
		}

	case campaignEventMsg:
		// Handle real-time events from the orchestrator
		// Events are informational - we can log them or show in UI
		// Continue listening for more events
		if m.campaignEventChan != nil && m.activeCampaign != nil {
			return m, m.listenCampaignEvents()
		}

	case campaignCompletedMsg:
		m.isLoading = false
		m.activeCampaign = nil
		m.campaignOrch = nil
		m.campaignProgress = nil
		m.campaignProgressChan = nil // Clear channels to stop listeners
		m.campaignEventChan = nil
		m.showCampaignPanel = false
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: m.renderCampaignCompleted(msg),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	// =========================================================================
	// CONTINUATION PROTOCOL MESSAGE HANDLERS
	// =========================================================================

	case continuationInitMsg:
		// First step already completed; render its surface and initialize counters.
		if msg.firstResult != nil {
			m.storeShardResult(msg.firstResult.ShardType, msg.firstResult.Task, msg.firstResult.Result, msg.firstResult.Facts)
		}

		if msg.totalSteps > 0 {
			m.continuationTotal = msg.totalSteps
		} else if m.continuationTotal == 0 {
			m.continuationTotal = 2
		}
		m.continuationStep = 1

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: msg.completedSurface,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

		// Decide whether to pause before next step.
		shouldPause := false
		switch m.continuationMode {
		case ContinuationModeConfirm:
			shouldPause = true
		case ContinuationModeBreakpoint:
			shouldPause = msg.next.isMutation
		}

		if shouldPause {
			m.isLoading = false
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("?? Next: %s\n\nPress Enter to continue, or type new instructions.", msg.next.description),
				Time:    time.Now(),
			})
			m.pendingSubtasks = append(m.pendingSubtasks, Subtask{
				ID:          msg.next.subtaskID,
				Description: msg.next.description,
				ShardType:   msg.next.shardType,
				IsMutation:  msg.next.isMutation,
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		// Auto-continue immediately.
		m.isLoading = true
		m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, msg.next.description)
		return m, tea.Batch(
			m.spinner.Tick,
			m.executeSubtask(msg.next.subtaskID, msg.next.description, msg.next.shardType),
		)

	case continueMsg:
		// Store the just-completed subtask result for follow-ups.
		if msg.completedShardResult != nil {
			m.storeShardResult(msg.completedShardResult.ShardType, msg.completedShardResult.Task, msg.completedShardResult.Result, msg.completedShardResult.Facts)
		}
		if msg.totalSteps > 0 && msg.totalSteps > m.continuationTotal {
			m.continuationTotal = msg.totalSteps
		}

		m.continuationStep++

		// Show progress for completed step
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("✓ [%d/%d] %s", m.continuationStep-1, m.continuationTotal, m.statusMessage),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

		// Check continuation mode to decide whether to pause
		shouldPause := false
		switch m.continuationMode {
		case ContinuationModeConfirm:
			shouldPause = true // Always pause in Confirm mode
		case ContinuationModeBreakpoint:
			shouldPause = msg.isMutation // Pause only for mutations
		}

		if shouldPause {
			m.isLoading = false
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⏸️ Next: %s\n\nPress Enter to continue, or type new instructions.", msg.description),
				Time:    time.Now(),
			})
			m.pendingSubtasks = append(m.pendingSubtasks, Subtask{
				ID:          msg.subtaskID,
				Description: msg.description,
				ShardType:   msg.shardType,
				IsMutation:  msg.isMutation,
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		// Auto mode: continue immediately
		m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, msg.description)
		return m, tea.Batch(
			m.spinner.Tick,
			m.executeSubtask(msg.subtaskID, msg.description, msg.shardType),
		)

	case confirmContinueMsg:
		// Resume from paused state (Enter pressed in Confirm/Breakpoint mode)
		if len(m.pendingSubtasks) > 0 {
			next := m.pendingSubtasks[0]
			m.pendingSubtasks = m.pendingSubtasks[1:]
			m.isLoading = true
			m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, next.Description)
			return m, tea.Batch(
				m.spinner.Tick,
				m.executeSubtask(next.ID, next.Description, next.ShardType),
			)
		}
		return m, nil

	case continuationDoneMsg:
		if msg.completedShardResult != nil {
			m.storeShardResult(msg.completedShardResult.ShardType, msg.completedShardResult.Task, msg.completedShardResult.Result, msg.completedShardResult.Facts)
		}

		m.isLoading = false
		m.continuationStep = 0
		m.continuationTotal = 0
		m.pendingSubtasks = nil
		m.isInterrupted = false
		// Clear continuation facts from kernel
		if m.kernel != nil {
			_ = m.kernel.Retract("interrupt_requested")
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("✅ All %d steps complete.\n\n%s", msg.stepCount, msg.summary),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	case initCompleteMsg:
		m.isLoading = false
		// Set up learning store and connect to shard manager via adapter
		if msg.learningStore != nil {
			m.learningStore = msg.learningStore
			adapter := &learningStoreAdapter{store: msg.learningStore}
			m.shardMgr.SetLearningStore(adapter)
		}
		// Build summary message from result
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: m.renderInitComplete(msg.result),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		// Persist session after init
		m.saveSessionState()

	case scanCompleteMsg:
		startupScan := m.isBooting && m.bootStage == BootStageScanning
		m.isLoading = false
		if msg.err != nil {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Scan failed:** %v", msg.err),
				Time:    time.Now(),
			})
		} else {
			m.history = append(m.history, Message{
				Role: "assistant",
				Content: fmt.Sprintf(`**Scan complete**

| Metric | Value |
|--------|-------|
| Files indexed | %d |
| Directories | %d |
| Facts generated | %d |
| Duration | %.2fs |

The kernel has been updated with fresh codebase facts.`, msg.fileCount, msg.directoryCount, msg.factCount, msg.duration.Seconds()),
				Time: time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()

		// If this was the startup scan, unlock chat input now that we're green.
		if startupScan {
			m.isBooting = false
			m.textarea.Placeholder = "Ask me anything... (Enter to send, Shift+Enter for newline, Ctrl+C to exit)"
			m.textarea.Focus()

			// Check for first-run onboarding after boot completes
			return m, checkFirstRun(m.workspace)
		}

	case reembedCompleteMsg:
		m.isLoading = false
		if msg.err != nil {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Re-embedding failed:** %v", msg.err),
				Time:    time.Now(),
			})
		} else {
			var sb strings.Builder
			sb.WriteString("**Re-embedding complete**\n\n")
			sb.WriteString(fmt.Sprintf("| Metric | Value |\n|--------|-------|\n| DBs processed | %d |\n| Vectors re-embedded | %d |\n| Prompt atoms re-embedded | %d |\n| Duration | %.2fs |\n",
				msg.dbCount, msg.vectorsDone, msg.atomsDone, msg.duration.Seconds()))
			if len(msg.skipped) > 0 {
				sb.WriteString("\nSkipped/errored DBs:\n")
				for _, s := range msg.skipped {
					sb.WriteString(fmt.Sprintf("- %s\n", s))
				}
			}
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: sb.String(),
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()

	case statusMsg:
		m.statusMessage = string(msg)
		return m, m.waitForStatus() // Listen for next update

	case memUsageMsg:
		m.memAllocBytes = msg.Alloc
		m.memSysBytes = msg.Sys
		return m, m.tickMemory()

	case bootCompleteMsg:
		if msg.err != nil {
			// Boot failed - unlock input so user can fix config.
			m.isBooting = false
			m.bootStage = BootStageBooting
			m.err = msg.err
			m.showError = true
			m.focusError = false
			m.refreshErrorViewport()
			m.errorVP.GotoTop()
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**System Boot Failed:** %v", msg.err),
				Time:    time.Now(),
			})
		} else {
			// Boot succeeded - keep UI in boot screen while we scan/index workspace.
			m.isBooting = true
			m.bootStage = BootStageScanning
			// Populate components from the heavy initialization
			c := msg.components
			m.kernel = c.Kernel
			m.shardMgr = c.ShardMgr
			m.shadowMode = c.ShadowMode
			m.transducer = c.Transducer
			m.executor = c.Executor
			m.emitter = c.Emitter
			m.virtualStore = c.VirtualStore
			m.scanner = c.Scanner
			m.localDB = c.LocalDB
			m.compressor = c.Compressor
			m.autopoiesis = c.Autopoiesis
			m.autopoiesisCancel = c.AutopoiesisCancel
			m.autopoiesisListenerCh = c.AutopoiesisListenerCh
			m.verifier = c.Verifier
			m.client = c.Client

			// Wire browser manager for graceful shutdown
			m.browserMgr = c.BrowserManager
			m.browserCtxCancel = c.BrowserCtxCancel
			m.jitCompiler = c.JITCompiler

			// Wire Mangle file watcher for real-time .mg validation
			m.mangleWatcher = c.MangleWatcher

			// Initialize Dream State learning collector and router (§8.3.1)
			m.dreamCollector = core.NewDreamLearningCollector()
			m.dreamRouter = core.NewDreamRouter(m.kernel, nil, m.localDB)

			// Load previous session state if available (now that kernel is ready)
			loadedSession, _ := hydrateNerdState(m.workspace, m.kernel, m.shardMgr, &m.history)
			m.sessionID = resolveSessionID(loadedSession)
			m.turnCount = resolveTurnCount(loadedSession)

			// Rehydrate semantic compression state for this session (if persisted).
			m.hydrateCompressorForSession(m.sessionID)
		}

		// If boot failed, allow input immediately. If boot succeeded, wait until scan completes.
		if msg.err != nil {
			m.textarea.Placeholder = "System boot failed. Fix config then retry /scan or restart."
			m.textarea.Focus()
		} else {
			m.textarea.Placeholder = "Indexing workspace..."
		}

		// Append any initial messages generated during boot
		if msg.components != nil && len(msg.components.InitialMessages) > 0 {
			m.history = append(m.history, msg.components.InitialMessages...)
		}

		// Now trigger the workspace scan (deferred). This keeps chat input hidden until ready.
		return m, m.runScan(false)

	case onboardingCheckMsg:
		// Handle first-run detection result
		if msg.IsFirstRun {
			// Start onboarding wizard for new users
			return m.startOnboarding()
		}
		// Existing user - run migration silently
		_, _ = ux.MigratePreferences(msg.Workspace)
		return m, nil

	case onboardingCompleteMsg:
		// Onboarding finished - record in preferences
		if !msg.Skipped && msg.ExperienceLevel != "" {
			// Update guidance level based on experience
			if m.Config != nil && m.Config.Guidance != nil {
				switch msg.ExperienceLevel {
				case "beginner":
					m.Config.Guidance.Level = config.GuidanceVerbose
				case "intermediate":
					m.Config.Guidance.Level = config.GuidanceNormal
				case "advanced", "expert":
					m.Config.Guidance.Level = config.GuidanceMinimal
				}
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}
