// Package main provides the codeNERD CLI entry point.
// This file implements the core interactive chat interface using bubbletea.
// The chat functionality is split across multiple files for maintainability:
//   - chat_core.go: Types, Init, Update loop (this file)
//   - chat_commands.go: /command handling
//   - chat_process.go: Natural language input processing
//   - chat_view.go: Rendering functions
//   - chat_session.go: Session management
//   - chat_campaign.go: Campaign orchestration
//   - chat_delegation.go: Shard spawning
//   - chat_shadow.go: Shadow mode
//   - chat_helpers.go: Utility functions
package main

import (
	"codenerd/cmd/nerd/config"
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
	"codenerd/internal/campaign"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/shards"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// CORE TYPES
// =============================================================================

// ClarificationState represents a pending clarification request
type ClarificationState struct {
	Question      string
	Options       []string
	DefaultOption string
	Context       string // Serialized kernel state
	PendingIntent *perception.Intent
}

// chatModel is the main model for the interactive chat interface
type chatModel struct {
	// UI Components
	textinput textinput.Model
	viewport  viewport.Model
	spinner   spinner.Model
	styles    ui.Styles
	renderer  *glamour.TermRenderer

	// Split-pane TUI (Glass Box Interface)
	splitPane *ui.SplitPaneView
	logicPane *ui.LogicPane
	showLogic bool
	paneMode  ui.PaneMode

	// State
	history   []chatMessage
	isLoading bool
	err       error
	width     int
	height    int
	ready     bool
	config    config.Config

	// Clarification Loop State (Pause/Resume Protocol)
	awaitingClarification bool
	clarificationState    *ClarificationState
	selectedOption        int // For option picker
	awaitingPatch         bool
	pendingPatchLines     []string

	// Session State
	sessionID string
	turnCount int
	// Agent creation wizard
	awaitingAgentDefinition bool

	// Backend
	client     perception.LLMClient
	kernel     *core.RealKernel
	shardMgr   *core.ShardManager
	shadowMode *core.ShadowMode
	transducer *perception.RealTransducer
	executor   *tactile.SafeExecutor
	emitter    *articulation.Emitter
	scanner    *world.Scanner
	workspace  string

	// Campaign Orchestration
	activeCampaign    *campaign.Campaign
	campaignOrch      *campaign.Orchestrator
	campaignProgress  *campaign.Progress
	showCampaignPanel bool

	// Learning Store for Autopoiesis (§8.3)
	learningStore *store.LearningStore

	// Local knowledge database for research persistence
	localDB *store.LocalStore

	// Semantic Compression (§8.2) - Infinite Context
	compressor *ctxcompress.Compressor

	// Autopoiesis (§8.3) - Self-Modification
	autopoiesis *autopoiesis.Orchestrator
}

type chatMessage struct {
	role    string // "user" or "assistant"
	content string
	time    time.Time
}

type nerdAgent struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	KnowledgePath string `json:"knowledge_path"`
	KBSize        int    `json:"kb_size"`
	Status        string `json:"status"`
}

type nerdRegistry struct {
	Version   string      `json:"version"`
	CreatedAt string      `json:"created_at"`
	Agents    []nerdAgent `json:"agents"`
}

type nerdPreferences struct {
	RequireTests     bool   `json:"require_tests"`
	RequireReview    bool   `json:"require_review"`
	Verbosity        string `json:"verbosity"`
	ExplanationLevel string `json:"explanation_level"`
}

type nerdSession struct {
	SessionID    string `json:"session_id"`
	StartedAt    string `json:"started_at"`
	LastActiveAt string `json:"last_active_at"`
	TurnCount    int    `json:"turn_count"`
	Suspended    bool   `json:"suspended"`
}

// Messages for tea updates
type (
	responseMsg        string
	errorMsg           error
	windowSizeMsg      tea.WindowSizeMsg
	clarificationMsg   ClarificationState // Request for user clarification
	clarificationReply string             // User's response to clarification

	// Campaign messages
	campaignStartedMsg   *campaign.Campaign
	campaignProgressMsg  *campaign.Progress
	campaignCompletedMsg *campaign.Campaign
	campaignErrorMsg     error

	// Init messages
	initCompleteMsg struct {
		result        *nerdinit.InitResult
		learningStore *store.LearningStore
	}
)

// initChat initializes the interactive chat model
func (m chatModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyCtrlL:
			// Toggle logic pane (Glass Box Interface)
			m.showLogic = !m.showLogic
			if m.showLogic {
				m.paneMode = ui.ModeSplitPane
				m.splitPane.SetMode(ui.ModeSplitPane)
			} else {
				m.paneMode = ui.ModeSinglePane
				m.splitPane.SetMode(ui.ModeSinglePane)
			}
			return m, nil

		case tea.KeyCtrlG:
			// Cycle through pane modes: Single -> Split -> Full Logic -> Single
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

		case tea.KeyCtrlR:
			// Toggle focus between chat and logic pane (when in split mode)
			if m.paneMode == ui.ModeSplitPane {
				m.splitPane.ToggleFocus()
			}
			return m, nil

		case tea.KeyCtrlP:
			// Toggle campaign progress panel
			m.showCampaignPanel = !m.showCampaignPanel
			return m, nil

		case tea.KeyEnter:
			// Enter sends the message
			if !m.isLoading {
				if m.awaitingClarification {
					return m.handleClarificationResponse()
				}
				return m.handleSubmit()
			}

		case tea.KeyUp:
			// Navigate options when in clarification mode
			if m.awaitingClarification && m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
				if m.selectedOption > 0 {
					m.selectedOption--
				}
				return m, nil
			}

		case tea.KeyDown:
			// Navigate options when in clarification mode
			if m.awaitingClarification && m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
				if m.selectedOption < len(m.clarificationState.Options)-1 {
					m.selectedOption++
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

		// Handle regular key input
		if !m.isLoading {
			m.textinput, tiCmd = m.textinput.Update(msg)
		}

	case windowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 4
		footerHeight := 3
		inputHeight := 3   // Smaller input height for textinput
		paddingHeight := 2 // Extra padding for safety

		if !m.ready {
			m.viewport = viewport.New(msg.Width-4, msg.Height-headerHeight-footerHeight-inputHeight-paddingHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = msg.Height - headerHeight - footerHeight - inputHeight - paddingHeight
		}

		m.textinput.Width = msg.Width - 4

		// Update split pane dimensions
		if m.splitPane != nil {
			m.splitPane.SetSize(msg.Width, msg.Height-headerHeight-footerHeight-inputHeight-paddingHeight)
		}
		if m.logicPane != nil {
			m.logicPane.SetSize(msg.Width/3, msg.Height-headerHeight-footerHeight-inputHeight-paddingHeight)
		}

		// Update renderer word wrap
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(msg.Width-8),
			)
		}

	case tea.WindowSizeMsg:
		// Convert to our alias and re-process
		return m.Update(windowSizeMsg(msg))

	case clarificationReply:
		// Handle clarification reply
		return m, m.processClarificationResponse(string(msg), m.clarificationState.PendingIntent)

	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, spCmd = m.spinner.Update(msg)
			return m, spCmd
		}

	case responseMsg:
		m.isLoading = false
		m.turnCount++
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: string(msg),
			time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		// Persist session after each response
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

		// Update UI to show clarification request
		m.textinput.Placeholder = "Select option or type your answer..."
		if len(msg.Options) > 0 {
			m.textinput.Placeholder = "Use ↑/↓ to select, Enter to confirm, or type custom answer..."
		}

		// Add clarification question to history
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: m.formatClarificationRequest(ClarificationState(msg)),
			time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	case campaignErrorMsg:
		m.isLoading = false
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("## ❌ Campaign Error\n\n%v", msg),
			time:    time.Now(),
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

	// Campaign message handlers
	case campaignStartedMsg:
		m.isLoading = false
		m.activeCampaign = msg
		m.showCampaignPanel = true
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: m.renderCampaignStarted(msg),
			time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	case campaignProgressMsg:
		m.campaignProgress = msg
		// Update campaign panel without adding to history (live update)
		if m.activeCampaign != nil {
			m.activeCampaign.CompletedPhases = msg.CompletedPhases
			m.activeCampaign.CompletedTasks = msg.CompletedTasks
		}

	case campaignCompletedMsg:
		m.isLoading = false
		m.activeCampaign = nil
		m.campaignOrch = nil
		m.campaignProgress = nil
		m.showCampaignPanel = false
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: m.renderCampaignCompleted(msg),
			time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

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
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: m.renderInitComplete(msg.result),
			time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		// Persist session after init
		m.saveSessionState()

	case scanCompleteMsg:
		m.isLoading = false
		if msg.err != nil {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("❌ **Scan failed:** %v", msg.err),
				time:    time.Now(),
			})
		} else {
			m.history = append(m.history, chatMessage{
				role: "assistant",
				content: fmt.Sprintf(`✅ **Scan complete**

| Metric | Value |
|--------|-------|
| Files indexed | %d |
| Directories | %d |
| Facts generated | %d |
| Duration | %.2fs |

The kernel has been updated with fresh codebase facts.`, msg.fileCount, msg.directoryCount, msg.factCount, msg.duration.Seconds()),
				time: time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()
	}

	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}

// handleClarificationResponse processes the user's response to a clarification request
func (m chatModel) handleClarificationResponse() (tea.Model, tea.Cmd) {
	var response string

	// Check if user selected an option or typed custom response
	if m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
		inputText := strings.TrimSpace(m.textinput.Value())
		if inputText == "" {
			// Use selected option
			response = m.clarificationState.Options[m.selectedOption]
		} else {
			// Use custom input
			response = inputText
		}
	} else {
		response = strings.TrimSpace(m.textinput.Value())
		if response == "" {
			return m, nil
		}
	}

	// Add user response to history
	m.history = append(m.history, chatMessage{
		role:    "user",
		content: response,
		time:    time.Now(),
	})

	// Clear clarification state (Resume)
	pendingIntent := m.clarificationState.PendingIntent
	m.awaitingClarification = false
	m.clarificationState = nil
	m.selectedOption = 0

	// Reset input
	m.textinput.Reset()
	m.textinput.Placeholder = "Ask me anything... (Enter to send, Ctrl+C to exit)"

	// Update viewport
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	// Start loading
	m.isLoading = true

	// Resume processing with clarification response
	return m, tea.Batch(
		m.spinner.Tick,
		m.processClarificationResponse(response, pendingIntent),
	)
}

// processClarificationResponse continues processing after user provides clarification
func (m chatModel) processClarificationResponse(response string, pendingIntent *perception.Intent) tea.Cmd {
	return func() tea.Msg {
		// Inject the clarification fact into the kernel
		clarificationFact := core.Fact{
			Predicate: "focus_clarification",
			Args:      []interface{}{response},
		}
		if err := m.kernel.Assert(clarificationFact); err != nil {
			return errorMsg(fmt.Errorf("failed to inject clarification: %w", err))
		}

		// If we have a pending intent, re-process with the clarification
		if pendingIntent != nil {
			// Update intent with clarification
			pendingIntent.Target = response

			// Continue processing
			actions, _ := m.kernel.Query("next_action")

			var surfaceResponse string
			if len(actions) > 0 {
				surfaceResponse = fmt.Sprintf("Clarified: %s\n\nProceeding with: %s", response, pendingIntent.Target)
			} else {
				surfaceResponse = fmt.Sprintf("Thank you for clarifying: %s\n\nI'll proceed with your request.", response)
			}

			return responseMsg(surfaceResponse)
		}

		return responseMsg(fmt.Sprintf("Got it: %s", response))
	}
}

// formatClarificationRequest formats a clarification request for display
func (m chatModel) formatClarificationRequest(state ClarificationState) string {
	var sb strings.Builder

	sb.WriteString("🤔 **I need some clarification:**\n\n")
	sb.WriteString(state.Question)
	sb.WriteString("\n\n")

	if len(state.Options) > 0 {
		sb.WriteString("**Options:**\n")
		for i, opt := range state.Options {
			if i == m.selectedOption {
				sb.WriteString(fmt.Sprintf("  → **%d. %s** ←\n", i+1, opt))
			} else {
				sb.WriteString(fmt.Sprintf("    %d. %s\n", i+1, opt))
			}
		}
		sb.WriteString("\n_Use ↑/↓ to select, Enter to confirm, or type a custom answer_")
	}

	return sb.String()
}

// extractClarificationQuestion extracts the question from an error message
func extractClarificationQuestion(errMsg string) string {
	// Try to extract a meaningful question
	if strings.Contains(errMsg, ":") {
		parts := strings.SplitN(errMsg, ":", 2)
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}
	return "Could you please provide more details?"
}

func (m chatModel) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textinput.Value())
	if input == "" {
		return m, nil
	}

	// Patch ingestion mode
	if m.awaitingPatch {
		// Accumulate lines until user types --END--
		if input == "--END--" {
			patch := strings.Join(m.pendingPatchLines, "\n")
			m.pendingPatchLines = nil
			m.awaitingPatch = false
			m.textinput.Placeholder = "Ask me anything... (Enter to send, Ctrl+C to exit)"
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: applyPatchResult(m.workspace, patch),
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		m.pendingPatchLines = append(m.pendingPatchLines, input)
		m.textinput.Reset()
		return m, nil
	}

	// Check for special commands
	if strings.HasPrefix(input, "/") {
		return m.handleCommand(input)
	}

	// Add user message to history
	m.history = append(m.history, chatMessage{
		role:    "user",
		content: input,
		time:    time.Now(),
	})

	// Clear input
	m.textinput.Reset()

	// Update viewport
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	// Start loading
	if m.awaitingAgentDefinition {
		m.awaitingAgentDefinition = false
		m.textinput.Placeholder = "Ask me anything... (Enter to send, Ctrl+C to exit)"
		m.isLoading = true
		return m, tea.Batch(
			m.spinner.Tick,
			m.createAgentFromPrompt(input),
		)
	}

	m.isLoading = true

	// Process in background
	return m, tea.Batch(
		m.spinner.Tick,
		m.processInput(input),
	)
}

