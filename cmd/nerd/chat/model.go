// Package chat provides the interactive TUI chat interface for codeNERD.
// The chat functionality is split across multiple files for maintainability:
//   - model.go: Types, Init, Update loop (this file)
//   - commands.go: /command handling
//   - process.go: Natural language input processing
//   - view.go: Rendering functions
//   - session.go: Session management
//   - campaign.go: Campaign orchestration
//   - delegation.go: Shard spawning
//   - shadow.go: Shadow mode
//   - helpers.go: Utility functions
package chat

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
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/verification"
	"codenerd/internal/world"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// =============================================================================
// CONFIGURATION
// =============================================================================

// Config holds configuration for initializing the chat interface.
type Config struct {
	// DisableSystemShards is a list of system shard names to disable.
	DisableSystemShards []string
}

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

// Model is the main model for the interactive chat interface
type Model struct {
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
	history   []Message
	isLoading bool
	err       error
	width     int
	height    int
	ready     bool
	Config    config.Config

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

	// Verification Loop (Quality-Enforcing)
	verifier *verification.TaskVerifier

	// Agent Wizard State
	agentWizard *AgentWizardState

	// ==========================================================================
	// CONVERSATIONAL CONTEXT (Fix for follow-up questions)
	// ==========================================================================
	// Stores the last shard result so follow-up questions can reference it.
	// This enables "what are the other suggestions?" after a review.
	lastShardResult *ShardResult
}

// ShardResult stores the full output from a shard execution for follow-up queries.
// This enables conversational follow-ups like "show me more" or "what are the warnings?".
type ShardResult struct {
	ShardType   string                 // "reviewer", "coder", "tester", "researcher"
	Task        string                 // Original task sent to the shard
	RawOutput   string                 // Full untruncated output
	Timestamp   time.Time              // When the shard executed
	TurnNumber  int                    // Which turn this was
	Findings    []map[string]any       // Structured findings (for reviewer)
	Metrics     map[string]any         // Metrics (for reviewer)
	ExtraData   map[string]any         // Any additional structured data
}

// Message represents a single message in the chat history
type Message struct {
	Role    string // "user" or "assistant"
	Content string
	Time    time.Time
}

// Agent represents a defined agent in the registry
type Agent struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	KnowledgePath string `json:"knowledge_path"`
	KBSize        int    `json:"kb_size"`
	Status        string `json:"status"`
}

// Registry holds the list of defined agents
type Registry struct {
	Version   string  `json:"version"`
	CreatedAt string  `json:"created_at"`
	Agents    []Agent `json:"agents"`
}

// Preferences holds user preferences
type Preferences struct {
	RequireTests     bool   `json:"require_tests"`
	RequireReview    bool   `json:"require_review"`
	Verbosity        string `json:"verbosity"`
	ExplanationLevel string `json:"explanation_level"`
}

// Session holds session state
type Session struct {
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
	campaignErrorMsg     struct{ err error }

	// Init messages
	initCompleteMsg struct {
		result        *nerdinit.InitResult
		learningStore *store.LearningStore
	}
)

// Init initializes the interactive chat model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		// Handle Alt key bindings
		if msg.Alt && len(msg.Runes) > 0 {
			switch msg.Runes[0] {
			case 'l':
				// Toggle logic pane (Alt+L)
				m.showLogic = !m.showLogic
				if m.showLogic {
					m.paneMode = ui.ModeSplitPane
					m.splitPane.SetMode(ui.ModeSplitPane)
				} else {
					m.paneMode = ui.ModeSinglePane
					m.splitPane.SetMode(ui.ModeSinglePane)
				}
				return m, nil

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

		// Reduce input width to accommodate border (2) + padding (2) + safety margin
		m.textinput.Width = msg.Width - 8

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
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: string(msg),
			Time:    time.Now(),
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

	case campaignErrorMsg:
		m.isLoading = false
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("## Campaign Error\n\n%v", msg.err),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	// Campaign message handlers
	case campaignStartedMsg:
		m.isLoading = false
		m.activeCampaign = msg
		m.showCampaignPanel = true
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: m.renderCampaignStarted(msg),
			Time:    time.Now(),
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
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: m.renderCampaignCompleted(msg),
			Time:    time.Now(),
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
	}

	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}

// handleClarificationResponse processes the user's response to a clarification request
func (m Model) handleClarificationResponse() (tea.Model, tea.Cmd) {
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
	m.history = append(m.history, Message{
		Role:    "user",
		Content: response,
		Time:    time.Now(),
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
func (m Model) processClarificationResponse(response string, pendingIntent *perception.Intent) tea.Cmd {
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
func (m Model) formatClarificationRequest(state ClarificationState) string {
	var sb strings.Builder

	sb.WriteString("**I need some clarification:**\n\n")
	sb.WriteString(state.Question)
	sb.WriteString("\n\n")

	if len(state.Options) > 0 {
		sb.WriteString("**Options:**\n")
		for i, opt := range state.Options {
			if i == m.selectedOption {
				sb.WriteString(fmt.Sprintf("  -> **%d. %s** <-\n", i+1, opt))
			} else {
				sb.WriteString(fmt.Sprintf("    %d. %s\n", i+1, opt))
			}
		}
		sb.WriteString("\n_Use arrow keys to select, Enter to confirm, or type a custom answer_")
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

func (m Model) handleSubmit() (tea.Model, tea.Cmd) {
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
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: applyPatchResult(m.workspace, patch),
				Time:    time.Now(),
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
	m.history = append(m.history, Message{
		Role:    "user",
		Content: input,
		Time:    time.Now(),
	})

	// Clear input
	m.textinput.Reset()

	// Update viewport
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	// Start loading
	if m.awaitingAgentDefinition {
		return m.handleAgentWizardInput(input)
	}

	m.isLoading = true

	// Process in background
	return m, tea.Batch(
		m.spinner.Tick,
		m.processInput(input),
	)
}

// RunInteractiveChat starts the interactive chat session
func RunInteractiveChat(cfg Config) error {
	model := InitChat(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// storeShardResult saves shard execution results for follow-up queries.
// This enables conversational follow-ups like "show me more" or "what are the warnings?".
func (m *Model) storeShardResult(shardType, task, result string, facts []core.Fact) {
	m.lastShardResult = &ShardResult{
		ShardType:  shardType,
		Task:       task,
		RawOutput:  result,
		Timestamp:  time.Now(),
		TurnNumber: m.turnCount,
		Findings:   extractFindings(result),
		Metrics:    extractMetrics(result),
		ExtraData:  make(map[string]any),
	}

	// Store facts for later reference
	if len(facts) > 0 {
		factStrings := make([]string, len(facts))
		for i, f := range facts {
			factStrings[i] = f.String()
		}
		m.lastShardResult.ExtraData["facts"] = factStrings
	}
}

// extractFindings parses structured findings from reviewer output.
func extractFindings(result string) []map[string]any {
	var findings []map[string]any
	// Simple line-based extraction - look for patterns like "- [ERROR] file:line: message"
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- [") || strings.HasPrefix(line, "• [") ||
			strings.Contains(line, "[WARN]") || strings.Contains(line, "[INFO]") ||
			strings.Contains(line, "[CRIT]") || strings.Contains(line, "[ERR]") {
			finding := map[string]any{
				"raw": line,
			}
			// Extract severity
			if strings.Contains(line, "[CRIT]") || strings.Contains(line, "[CRITICAL]") {
				finding["severity"] = "critical"
			} else if strings.Contains(line, "[ERR]") || strings.Contains(line, "[ERROR]") {
				finding["severity"] = "error"
			} else if strings.Contains(line, "[WARN]") || strings.Contains(line, "[WARNING]") {
				finding["severity"] = "warning"
			} else if strings.Contains(line, "[INFO]") {
				finding["severity"] = "info"
			}
			findings = append(findings, finding)
		}
	}
	return findings
}

// extractMetrics parses metrics section from output.
func extractMetrics(result string) map[string]any {
	metrics := make(map[string]any)
	// Look for common metric patterns
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "lines") || strings.Contains(line, "functions") ||
			strings.Contains(line, "complexity") || strings.Contains(line, "nesting") {
			// Parse "Key: Value" or "Key = Value" patterns
			for _, sep := range []string{": ", "= ", "="} {
				if strings.Contains(line, sep) {
					parts := strings.SplitN(line, sep, 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						metrics[key] = value
					}
					break
				}
			}
		}
	}
	return metrics
}
