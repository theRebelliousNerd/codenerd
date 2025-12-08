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
	"context"
	"sync"

	"codenerd/cmd/nerd/config"
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
	"codenerd/internal/browser"
	"codenerd/internal/campaign"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/usage"
	"codenerd/internal/verification"
	"codenerd/internal/world"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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

// ViewMode determines which component is focused/active
type ViewMode int

const (
	ChatView ViewMode = iota
	ListView
	FilePickerView
	UsageView
	CampaignPage
)

// InputMode represents the current input handling state.
// This unifies the scattered awaiting* flags into a single state machine
// to prevent inconsistent states and simplify Update() logic.
type InputMode int

const (
	InputModeNormal       InputMode = iota // Default: process as chat input
	InputModeClarification                  // Awaiting clarification response
	InputModePatch                          // Awaiting patch input (--END-- terminated)
	InputModeAgentWizard                    // Agent definition wizard active
	InputModeConfigWizard                   // Config wizard active
	InputModeCampaignLaunch                 // Campaign launch clarification
)

// sessionItem is a list item for the session list
type sessionItem struct {
	id, date, desc string
}

func (i sessionItem) Title() string       { return i.date }
func (i sessionItem) Description() string { return fmt.Sprintf("[%s] %s", i.id, i.desc) }
func (i sessionItem) FilterValue() string { return i.id + " " + i.desc }

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
	textarea   textarea.Model
	viewport   viewport.Model
	spinner    spinner.Model
	list       list.Model
	filepicker filepicker.Model
	styles     ui.Styles
	renderer   *glamour.TermRenderer

	viewMode ViewMode

	// Split-pane TUI (Glass Box Interface)
	splitPane *ui.SplitPaneView
	logicPane *ui.LogicPane
	showLogic bool
	paneMode  ui.PaneMode

	// Usage Page
	usagePage ui.UsagePageModel

	// Campaign Page
	campaignPage ui.CampaignPageModel

	// Usage Tracking
	usageTracker *usage.Tracker

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
	lastClarifyInput      string // Track last input for clarification dedup

	// Session State
	sessionID string
	turnCount int
	// Agent creation wizard
	awaitingAgentDefinition bool

	// Backend
	client              perception.LLMClient
	kernel              *core.RealKernel
	shardMgr            *core.ShardManager
	shadowMode          *core.ShadowMode
	transducer          *perception.RealTransducer
	executor            *tactile.SafeExecutor
	emitter             *articulation.Emitter
	virtualStore        *core.VirtualStore
	scanner             *world.Scanner
	workspace           string
	DisableSystemShards []string
	browserMgr          *browser.SessionManager // Browser automation manager
	browserCtxCancel    context.CancelFunc      // Cancels browser manager goroutine

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
	autopoiesis           *autopoiesis.Orchestrator
	autopoiesisCancel     context.CancelFunc // Cancels kernel listener goroutine
	autopoiesisListenerCh <-chan struct{}    // Closed when listener stops

	// Verification Loop (Quality-Enforcing)
	verifier *verification.TaskVerifier

	// Agent Wizard State
	agentWizard *AgentWizardState

	// Config Wizard State
	awaitingConfigWizard bool
	configWizard         *ConfigWizardState

	// CLI Config
	CLIConfig Config

	// Status Tracking
	statusMessage string      // Current operation description
	statusChan    chan string // Channel for streaming status updates

	// Boot State
	isBooting bool

	// Input History
	inputHistory []string
	historyIndex int

	// Campaign Launch State
	launchClarifyPending bool
	launchClarifyGoal    string
	launchClarifyAnswers string

	// Context State
	lastShardResult    *ShardResult
	shardResultHistory []*ShardResult

	// Unified Input Mode (replaces scattered awaiting* flags)
	// Use this for new code; legacy flags preserved for compatibility during migration
	inputMode InputMode

	// Shutdown coordination
	shutdownOnce    sync.Once      // Ensures Shutdown() is only called once
	shutdownCtx     context.Context // Root context for all background operations
	shutdownCancel  context.CancelFunc // Cancels shutdownCtx on quit
}

// Shutdown gracefully stops all background goroutines and releases resources.
// Safe to call multiple times - only executes once.
// MUST be called before tea.Quit to prevent goroutine leaks.
func (m *Model) Shutdown() {
	m.shutdownOnce.Do(func() {
		// Cancel all background operations via root context
		if m.shutdownCancel != nil {
			m.shutdownCancel()
		}

		// Cancel autopoiesis listener goroutine
		if m.autopoiesisCancel != nil {
			m.autopoiesisCancel()
			// Wait for listener to stop (with timeout)
			if m.autopoiesisListenerCh != nil {
				select {
				case <-m.autopoiesisListenerCh:
					// Listener stopped cleanly
				case <-time.After(2 * time.Second):
					// Timeout - listener may be stuck, proceed anyway
				}
			}
		}

		// Stop browser manager goroutine
		if m.browserCtxCancel != nil {
			m.browserCtxCancel()
		}
		if m.browserMgr != nil {
			// Give it a moment to stop gracefully
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = m.browserMgr.Shutdown(ctx)
		}

		// Stop campaign orchestrator if running
		if m.campaignOrch != nil {
			m.campaignOrch.Stop()
		}

		// Close status channel to unblock waitForStatus
		if m.statusChan != nil {
			close(m.statusChan)
		}

		// Close local database connection
		if m.localDB != nil {
			m.localDB.Close()
		}

		// Stop all active shards
		if m.shardMgr != nil {
			m.shardMgr.StopAll()
		}
	})
}

// IsKernelReady returns true if the kernel is initialized and ready for queries.
// Use this guard before any kernel operations in commands.
func (m *Model) IsKernelReady() bool {
	return m.kernel != nil && !m.isBooting
}

// performShutdown is a value-receiver wrapper for Shutdown() that can be called
// from Update(). It uses a local copy to call the pointer method.
func (m Model) performShutdown() {
	// Create a temporary pointer to call Shutdown
	// This is safe because Shutdown uses sync.Once internally
	modelPtr := &m
	modelPtr.Shutdown()
}

// statusMsg represents a status update from a background process
type statusMsg string

// waitForStatus listens for status updates
func (m Model) waitForStatus() tea.Cmd {
	return func() tea.Msg {
		return statusMsg(<-m.statusChan)
	}
}

// ReportStatus sends a non-blocking status update
func (m Model) ReportStatus(msg string) {
	if m.statusChan != nil {
		select {
		case m.statusChan <- msg:
		default:
			// Channel full, drop update to prevent blocking
		}
	}
}

// ShardResult stores the full output from a shard execution for follow-up queries.
// This enables conversational follow-ups like "show me more" or "what are the warnings?".
type ShardResult struct {
	ShardType  string           // "reviewer", "coder", "tester", "researcher"
	Task       string           // Original task sent to the shard
	RawOutput  string           // Full untruncated output
	Timestamp  time.Time        // When the shard executed
	TurnNumber int              // Which turn this was
	Findings   []map[string]any // Structured findings (for reviewer)
	Metrics    map[string]any   // Metrics (for reviewer)
	ExtraData  map[string]any   // Any additional structured data
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

	// System Boot messages
	systemBootMsg struct {
		components *SystemComponents
		err        error
	}

	bootCompleteMsg struct {
		components *SystemComponents
		err        error
	}
)

// SystemComponents holds the initialized backend services
type SystemComponents struct {
	Kernel                *core.RealKernel
	ShardMgr              *core.ShardManager
	VirtualStore          *core.VirtualStore
	LLMClient             perception.LLMClient
	LocalDB               *store.LocalStore
	Transducer            *perception.RealTransducer
	Executor              *tactile.SafeExecutor
	Scanner               *world.Scanner
	Autopoiesis           *autopoiesis.Orchestrator
	Verifier              *verification.TaskVerifier
	Compressor            *ctxcompress.Compressor
	ShadowMode            *core.ShadowMode
	InitialMessages       []Message
	Client                perception.LLMClient
	Emitter               *articulation.Emitter
	AutopoiesisCancel     context.CancelFunc
	AutopoiesisListenerCh <-chan struct{}
	SessionID             string
	TurnCount             int
	BrowserManager        *browser.SessionManager
	BrowserCtxCancel      context.CancelFunc // Cancels browser manager goroutine
	Workspace             string
}

// Init initializes the interactive chat model
// Init initializes the interactive chat model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		// m.checkWorkspaceSync(), // DEFERRED until boot complete
		tea.EnableMouseCellMotion,
		m.waitForStatus(), // Start status listener
		performSystemBoot(m.Config, m.DisableSystemShards, m.workspace), // Start heavy system initialization
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
		// Global Keybindings (Ctrl+C, Esc)
		switch msg.Type {
		case tea.KeyCtrlC:
			// Graceful shutdown before quit
			m.performShutdown()
			return m, tea.Quit
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

		// Chat View Handling
		switch msg.Type {

		case tea.KeyEnter:
			// Allow Alt+Enter for newlines
			if msg.Alt {
				// Let textarea handle it
				break
			}

			// Enter sends the message if not loading
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

		calcHeight := msg.Height - headerHeight - footerHeight - inputHeight - paddingHeight
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
		return m, m.processClarificationResponse(string(msg), m.clarificationState.PendingIntent)

	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, spCmd = m.spinner.Update(msg)
			return m, spCmd
		}

	case traceUpdateMsg:
		if m.logicPane != nil {
			m.logicPane.SetTraceMangle(msg.Trace)
			// Auto-open logic pane relative to user preference?
			// For now, keep it manual via Alt+L, but update content.
		}
		return m, nil

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

		// Update Campaign Page
		if m.activeCampaign != nil {
			prog := campaign.Progress(*msg)
			m.campaignPage.UpdateContent(&prog, m.activeCampaign)
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

	case systemBootMsg:
		m.isLoading = false
		m.statusMessage = "" // Clear booting message
		m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"

		if msg.err != nil {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("CRITICAL: System boot failed: %v", msg.err),
				Time:    time.Now(),
			})
			m.err = msg.err
		} else {
			// Hydrate Model with initialized components
			c := msg.components
			m.kernel = c.Kernel
			m.shardMgr = c.ShardMgr
			m.virtualStore = c.VirtualStore
			m.client = c.LLMClient
			m.localDB = c.LocalDB
			m.transducer = c.Transducer
			m.executor = c.Executor
			m.scanner = c.Scanner
			m.autopoiesis = c.Autopoiesis
			m.verifier = c.Verifier
			m.compressor = c.Compressor
			m.shadowMode = c.ShadowMode

			// Wire Autopoiesis listener
			if c.Autopoiesis != nil {
				autopoiesisCtx, autopoiesisCancel := context.WithCancel(context.Background())
				m.autopoiesisCancel = autopoiesisCancel
				m.autopoiesisListenerCh = c.Autopoiesis.StartKernelListener(autopoiesisCtx, 2*time.Second)
			}

			// Append initial messages (e.g. "Embedding engine ready")
			if len(c.InitialMessages) > 0 {
				m.history = append(m.history, c.InitialMessages...)
			}

			// Load previous session state if available (now that kernel is ready)
			loadedSession, _ := hydrateNerdState(m.workspace, m.kernel, m.shardMgr, &m.history)
			m.sessionID = resolveSessionID(loadedSession)
			m.turnCount = resolveTurnCount(loadedSession)
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

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

	case statusMsg:
		m.statusMessage = string(msg)
		return m, m.waitForStatus() // Listen for next update

	case bootCompleteMsg:
		m.isBooting = false
		if msg.err != nil {
			m.err = msg.err
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**System Boot Failed:** %v", msg.err),
				Time:    time.Now(),
			})
		} else {
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
		}

		// Append any initial messages generated during boot
		if msg.components != nil && len(msg.components.InitialMessages) > 0 {
			m.history = append(m.history, msg.components.InitialMessages...)
		}

		// Now trigger the workspace scan (deferred)
		return m, m.checkWorkspaceSync()
	}

	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}

// handleClarificationResponse processes the user's response to a clarification request
func (m Model) handleClarificationResponse() (tea.Model, tea.Cmd) {
	var response string

	// Check if user selected an option or typed custom response
	if m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
		inputText := strings.TrimSpace(m.textarea.Value())
		if inputText == "" {
			// Use selected option
			response = m.clarificationState.Options[m.selectedOption]
		} else {
			// Use custom input
			response = inputText
		}
	} else {
		response = strings.TrimSpace(m.textarea.Value())
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
	// Append to input history
	if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != response {
		m.inputHistory = append(m.inputHistory, response)
	}
	m.historyIndex = len(m.inputHistory)

	// Clear clarification state (Resume)
	pendingIntent := m.clarificationState.PendingIntent
	m.awaitingClarification = false
	m.clarificationState = nil
	m.selectedOption = 0

	// Reset input
	m.textarea.Reset()
	m.textarea.Placeholder = "Ask me anything... (Enter to send, Shift+Enter for newline, Ctrl+C to exit)"

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
		// Guard: kernel must be initialized
		if m.kernel == nil {
			return errorMsg(fmt.Errorf("system not ready: kernel not initialized"))
		}

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
	input := strings.TrimSpace(m.textarea.Value())
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
			m.textarea.Placeholder = "Ask me anything... (Enter to send, Shift+Enter for newline, Ctrl+C to exit)"
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: applyPatchResult(m.workspace, patch),
				Time:    time.Now(),
			})
			m.textarea.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		m.pendingPatchLines = append(m.pendingPatchLines, input)
		m.textarea.Reset()
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
	// Append to input history
	if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != input {
		m.inputHistory = append(m.inputHistory, input)
	}
	m.historyIndex = len(m.inputHistory)

	// Clear input
	m.textarea.Reset()

	// Update viewport
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	// Start loading
	if m.awaitingAgentDefinition {
		return m.handleAgentWizardInput(input)
	}

	// Config wizard mode
	if m.awaitingConfigWizard {
		return m.handleConfigWizardInput(input)
	}

	m.isLoading = true

	// Check for negative feedback auto-trigger
	if isNegativeFeedback(input) {
		return m.triggerLearningLoop(input)
	}

	// Process in background
	return m, tea.Batch(
		m.spinner.Tick,
		m.processInput(input),
	)
}

// isNegativeFeedback checks for common frustration signals
func isNegativeFeedback(input string) bool {
	lower := strings.ToLower(input)
	triggers := []string{
		"bad bot", "wrong", "stop", "no that's not right",
		"you didn't", "fail", "incorrect", "mistake",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// triggerLearningLoop initiates the Ouroboros self-correction process
func (m Model) triggerLearningLoop(userInput string) (tea.Model, tea.Cmd) {
	// Add the user's complaint to history first so the Critic sees it
	m.history = append(m.history, Message{
		Role:    "user",
		Content: userInput,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()

	// Notify user we are paying attention
	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: "I detect dissatisfaction. Invoking Meta-Cognitive Supervisor to analyze our interaction and learn from this mistake...",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	m.isLoading = true

	learningCmd := func() tea.Msg {
		// Convert history to traces
		var traces []perception.ReasoningTrace
		for _, msg := range m.history {
			t := perception.ReasoningTrace{
				UserPrompt: "...",
				Response:   msg.Content,
				Success:    true,
			}
			if msg.Role == "user" {
				t.UserPrompt = msg.Content
			}
			traces = append(traces, t)
		}

		// Execute Learning
		perception.SharedTaxonomy.SetClient(m.client)
		fact, err := perception.SharedTaxonomy.LearnFromInteraction(context.Background(), traces)
		if err != nil {
			return responseMsg(fmt.Sprintf("Auto-learning failed: %v", err))
		}
		if fact == "" {
			return responseMsg("I analyzed the interaction but couldn't identify a clear pattern to generalize yet. I will keep this in mind.")
		}
		return responseMsg(fmt.Sprintf("I have crystallized a new rule from this interaction:\n```\n%s\n```\nI will apply this correction in future turns.", fact))
	}

	return m, tea.Batch(m.spinner.Tick, learningCmd)
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
// Also maintains a sliding window history for cross-shard context (blackboard pattern).
func (m *Model) storeShardResult(shardType, task, result string, facts []core.Fact) {
	sr := &ShardResult{
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
		sr.ExtraData["facts"] = factStrings
	}

	// Set as most recent result
	m.lastShardResult = sr

	// Add to history (sliding window of last 10 results)
	m.shardResultHistory = append(m.shardResultHistory, sr)
	const maxHistorySize = 10
	if len(m.shardResultHistory) > maxHistorySize {
		m.shardResultHistory = m.shardResultHistory[len(m.shardResultHistory)-maxHistorySize:]
	}
}

// buildSessionContext creates a SessionContext for shard injection (Blackboard Pattern).
// This provides shards with comprehensive session context including:
// - Compressed history, recent findings, recent actions (original)
// - World model facts (impacted files, diagnostics, symbols, dependencies)
// - User intent and focus resolutions
// - Campaign context (if active)
// - Git state for Chesterton's Fence
// - Test state for TDD loop awareness
// - Cross-shard execution history
// - Domain knowledge atoms
// - Constitutional constraints
func (m *Model) buildSessionContext(ctx context.Context) *core.SessionContext {
	sessionCtx := &core.SessionContext{
		ExtraContext: make(map[string]string),
	}

	// ==========================================================================
	// CORE CONTEXT (Original)
	// ==========================================================================

	// Get compressed history from compressor
	if m.compressor != nil {
		if ctxStr, err := m.compressor.GetContextString(ctx); err == nil {
			sessionCtx.CompressedHistory = ctxStr
		}
	}

	// Extract recent findings from shard history
	for _, sr := range m.shardResultHistory {
		if sr.ShardType == "reviewer" || sr.ShardType == "tester" {
			for _, f := range sr.Findings {
				if msg, ok := f["raw"].(string); ok {
					sessionCtx.RecentFindings = append(sessionCtx.RecentFindings, msg)
				}
			}
		}
		// Track recent actions
		sessionCtx.RecentActions = append(sessionCtx.RecentActions,
			fmt.Sprintf("[%s] %s", sr.ShardType, truncateForContext(sr.Task, 50)))
	}

	// Limit findings to last 20
	if len(sessionCtx.RecentFindings) > 20 {
		sessionCtx.RecentFindings = sessionCtx.RecentFindings[len(sessionCtx.RecentFindings)-20:]
	}

	// Limit actions to last 10
	if len(sessionCtx.RecentActions) > 10 {
		sessionCtx.RecentActions = sessionCtx.RecentActions[len(sessionCtx.RecentActions)-10:]
	}

	// ==========================================================================
	// WORLD MODEL / EDB FACTS (from kernel)
	// ==========================================================================
	if m.kernel != nil {
		// Get impacted files (transitive impact from modified files)
		sessionCtx.ImpactedFiles = m.queryKernelStrings("impacted")

		// Get current diagnostics (errors/warnings)
		sessionCtx.CurrentDiagnostics = m.queryDiagnostics()

		// Get relevant symbols in scope
		sessionCtx.SymbolContext = m.querySymbolContext()

		// Get 1-hop dependencies for active files
		if len(sessionCtx.ActiveFiles) > 0 {
			sessionCtx.DependencyContext = m.queryDependencyContext(sessionCtx.ActiveFiles)
		}

		// Get focus resolutions
		sessionCtx.FocusResolutions = m.queryFocusResolutions()
	}

	// ==========================================================================
	// CAMPAIGN CONTEXT (if active)
	// ==========================================================================
	if m.activeCampaign != nil {
		sessionCtx.CampaignActive = true
		// Get current phase from progress or derive from phases
		if m.campaignProgress != nil {
			sessionCtx.CampaignPhase = m.campaignProgress.CurrentPhase
		} else {
			sessionCtx.CampaignPhase = m.getCurrentPhaseName()
		}
		sessionCtx.CampaignGoal = m.getCampaignPhaseGoal()
		sessionCtx.TaskDependencies = m.getCampaignTaskDeps()
		sessionCtx.LinkedRequirements = m.getCampaignLinkedReqs()
	}

	// ==========================================================================
	// GIT STATE / CHESTERTON'S FENCE
	// ==========================================================================
	m.populateGitContext(sessionCtx)

	// ==========================================================================
	// TEST STATE (TDD LOOP)
	// ==========================================================================
	m.populateTestState(sessionCtx)

	// ==========================================================================
	// CROSS-SHARD EXECUTION HISTORY
	// ==========================================================================
	sessionCtx.PriorShardOutputs = m.buildPriorShardSummaries()

	// ==========================================================================
	// DOMAIN KNOWLEDGE (Type B Specialists)
	// ==========================================================================
	if m.learningStore != nil {
		sessionCtx.KnowledgeAtoms = m.queryKnowledgeAtoms()
		sessionCtx.SpecialistHints = m.querySpecialistHints()
	}

	// ==========================================================================
	// CONSTITUTIONAL CONSTRAINTS
	// ==========================================================================
	if m.kernel != nil {
		sessionCtx.AllowedActions = m.queryAllowedActions()
		sessionCtx.BlockedActions = m.queryBlockedActions()
		sessionCtx.SafetyWarnings = m.querySafetyWarnings()
	}

	return sessionCtx
}

// =============================================================================
// KERNEL QUERY HELPERS FOR SESSION CONTEXT
// =============================================================================

// queryKernelStrings queries a predicate and returns all first-arg strings.
func (m *Model) queryKernelStrings(predicate string) []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query(predicate)
	if err != nil {
		return nil
	}
	var strs []string
	for _, fact := range results {
		if len(fact.Args) > 0 {
			if s, ok := fact.Args[0].(string); ok {
				strs = append(strs, s)
			}
		}
	}
	return strs
}

// queryDiagnostics extracts current diagnostics from the kernel.
func (m *Model) queryDiagnostics() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("diagnostic")
	if err != nil {
		return nil
	}
	var diagnostics []string
	for _, fact := range results {
		// diagnostic(Severity, FilePath, Line, ErrorCode, Message)
		if len(fact.Args) >= 5 {
			severity, _ := fact.Args[0].(string)
			file, _ := fact.Args[1].(string)
			line, _ := fact.Args[2].(int64)
			msg, _ := fact.Args[4].(string)
			diagnostics = append(diagnostics,
				fmt.Sprintf("[%s] %s:%d: %s", severity, file, line, msg))
		}
	}
	// Limit to most recent 10
	if len(diagnostics) > 10 {
		diagnostics = diagnostics[len(diagnostics)-10:]
	}
	return diagnostics
}

// querySymbolContext gets relevant symbols from symbol_graph.
func (m *Model) querySymbolContext() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("symbol_graph")
	if err != nil {
		return nil
	}
	var symbols []string
	for _, fact := range results {
		// symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
		if len(fact.Args) >= 5 {
			symbolID, _ := fact.Args[0].(string)
			symType, _ := fact.Args[1].(string)
			visibility, _ := fact.Args[2].(string)
			signature, _ := fact.Args[4].(string)
			if visibility == "/public" || visibility == "/exported" {
				symbols = append(symbols,
					fmt.Sprintf("%s %s: %s", symType, symbolID, truncateForContext(signature, 60)))
			}
		}
	}
	// Limit to 15 most relevant
	if len(symbols) > 15 {
		symbols = symbols[:15]
	}
	return symbols
}

// queryDependencyContext gets 1-hop dependencies for target files.
func (m *Model) queryDependencyContext(files []string) []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("dependency_link")
	if err != nil {
		return nil
	}
	var deps []string
	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}
	for _, fact := range results {
		// dependency_link(CallerID, CalleeID, ImportPath)
		if len(fact.Args) >= 3 {
			caller, _ := fact.Args[0].(string)
			callee, _ := fact.Args[1].(string)
			importPath, _ := fact.Args[2].(string)
			// Check if caller or callee is in our active files
			if fileSet[caller] {
				deps = append(deps, fmt.Sprintf("%s imports %s", caller, importPath))
			}
			if fileSet[callee] {
				deps = append(deps, fmt.Sprintf("%s imported by %s", callee, caller))
			}
		}
	}
	// Limit to 10
	if len(deps) > 10 {
		deps = deps[:10]
	}
	return deps
}

// queryFocusResolutions gets resolved paths from fuzzy references.
func (m *Model) queryFocusResolutions() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("focus_resolution")
	if err != nil {
		return nil
	}
	var resolutions []string
	for _, fact := range results {
		// focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence)
		if len(fact.Args) >= 4 {
			rawRef, _ := fact.Args[0].(string)
			resolved, _ := fact.Args[1].(string)
			confidence, _ := fact.Args[3].(float64)
			resolutions = append(resolutions,
				fmt.Sprintf("'%s' -> %s (%.0f%%)", rawRef, resolved, confidence*100))
		}
	}
	return resolutions
}

// getCurrentPhaseName derives the current phase name from campaign phases.
func (m *Model) getCurrentPhaseName() string {
	if m.activeCampaign == nil {
		return ""
	}
	// Find phase with /in_progress status
	for _, phase := range m.activeCampaign.Phases {
		if phase.Status == campaign.PhaseInProgress {
			return phase.Name
		}
	}
	// Fallback: find first pending phase
	for _, phase := range m.activeCampaign.Phases {
		if phase.Status == campaign.PhasePending {
			return phase.Name
		}
	}
	// Fallback: return first phase name
	if len(m.activeCampaign.Phases) > 0 {
		return m.activeCampaign.Phases[0].Name
	}
	return ""
}

// getCampaignPhaseGoal returns the current phase's goal description.
func (m *Model) getCampaignPhaseGoal() string {
	if m.activeCampaign == nil {
		return ""
	}
	currentPhaseName := m.getCurrentPhaseName()
	for _, phase := range m.activeCampaign.Phases {
		if phase.Name == currentPhaseName {
			// Use first objective's description if available
			if len(phase.Objectives) > 0 {
				return phase.Objectives[0].Description
			}
			return phase.Name
		}
	}
	return currentPhaseName
}

// getCampaignTaskDeps returns dependencies for the current task.
func (m *Model) getCampaignTaskDeps() []string {
	if m.activeCampaign == nil || m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("has_blocking_task_dep")
	if err != nil {
		return nil
	}
	var deps []string
	for _, fact := range results {
		if len(fact.Args) >= 1 {
			if dep, ok := fact.Args[0].(string); ok {
				deps = append(deps, dep)
			}
		}
	}
	return deps
}

// getCampaignLinkedReqs returns requirements linked to current task.
func (m *Model) getCampaignLinkedReqs() []string {
	if m.activeCampaign == nil || m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("requirement_task_link")
	if err != nil {
		return nil
	}
	var reqs []string
	for _, fact := range results {
		// requirement_task_link(RequirementID, TaskID, Strength)
		if len(fact.Args) >= 2 {
			if req, ok := fact.Args[0].(string); ok {
				reqs = append(reqs, req)
			}
		}
	}
	return reqs
}

// populateGitContext fills in git state for Chesterton's Fence.
func (m *Model) populateGitContext(sessionCtx *core.SessionContext) {
	if m.kernel == nil {
		return
	}

	// Query git_branch fact
	if results, err := m.kernel.Query("git_branch"); err == nil && len(results) > 0 {
		if len(results[0].Args) >= 1 {
			if branch, ok := results[0].Args[0].(string); ok {
				sessionCtx.GitBranch = branch
			}
		}
	}

	// Query modified files
	sessionCtx.GitModifiedFiles = m.queryKernelStrings("modified")

	// Query recent commits (for Chesterton's Fence)
	if results, err := m.kernel.Query("recent_commit"); err == nil {
		for _, fact := range results {
			// recent_commit(Hash, Message, Author, Timestamp)
			if len(fact.Args) >= 2 {
				msg, _ := fact.Args[1].(string)
				sessionCtx.GitRecentCommits = append(sessionCtx.GitRecentCommits,
					truncateForContext(msg, 80))
			}
		}
	}
	// Limit to 5 commits
	if len(sessionCtx.GitRecentCommits) > 5 {
		sessionCtx.GitRecentCommits = sessionCtx.GitRecentCommits[:5]
	}

	// Count unstaged changes
	sessionCtx.GitUnstagedCount = len(sessionCtx.GitModifiedFiles)
}

// populateTestState fills in TDD loop state.
func (m *Model) populateTestState(sessionCtx *core.SessionContext) {
	if m.kernel == nil {
		sessionCtx.TestState = "unknown"
		return
	}

	// Query test_state fact
	if results, err := m.kernel.Query("test_state"); err == nil && len(results) > 0 {
		if len(results[0].Args) >= 1 {
			if state, ok := results[0].Args[0].(string); ok {
				sessionCtx.TestState = state
			}
		}
	} else {
		sessionCtx.TestState = "unknown"
	}

	// Query failing tests
	if results, err := m.kernel.Query("failing_test"); err == nil {
		for _, fact := range results {
			// failing_test(TestName, ErrorMessage)
			if len(fact.Args) >= 2 {
				testName, _ := fact.Args[0].(string)
				errMsg, _ := fact.Args[1].(string)
				sessionCtx.FailingTests = append(sessionCtx.FailingTests,
					fmt.Sprintf("%s: %s", testName, truncateForContext(errMsg, 60)))
			}
		}
	}

	// Query retry count
	if results, err := m.kernel.Query("retry_count"); err == nil && len(results) > 0 {
		if len(results[0].Args) >= 1 {
			if count, ok := results[0].Args[0].(int64); ok {
				sessionCtx.TDDRetryCount = int(count)
			}
		}
	}
}

// buildPriorShardSummaries creates summaries of recent shard executions.
func (m *Model) buildPriorShardSummaries() []core.ShardSummary {
	var summaries []core.ShardSummary
	for _, sr := range m.shardResultHistory {
		summaries = append(summaries, core.ShardSummary{
			ShardType: sr.ShardType,
			Task:      truncateForContext(sr.Task, 50),
			Summary:   extractShardSummary(sr),
			Timestamp: sr.Timestamp,
			Success:   sr.ExtraData["error"] == nil,
		})
	}
	// Limit to last 5
	if len(summaries) > 5 {
		summaries = summaries[len(summaries)-5:]
	}
	return summaries
}

// extractShardSummary extracts a one-line summary from shard result.
func extractShardSummary(sr *ShardResult) string {
	// Try to find a summary line
	lines := strings.Split(sr.RawOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for status lines
		if strings.Contains(line, "PASSED") || strings.Contains(line, "FAILED") ||
			strings.Contains(line, "complete") || strings.Contains(line, "created") ||
			strings.Contains(line, "modified") || strings.Contains(line, "reviewed") {
			return truncateForContext(line, 80)
		}
	}
	// Fallback: first non-empty line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 {
			return truncateForContext(line, 80)
		}
	}
	return truncateForContext(sr.RawOutput, 80)
}

// queryKnowledgeAtoms gets relevant domain knowledge from learning store.
func (m *Model) queryKnowledgeAtoms() []string {
	if m.learningStore == nil {
		return nil
	}
	// Query knowledge atoms from learning store
	learnings, err := m.learningStore.LoadByPredicate("knowledge", "atom")
	if err != nil {
		return nil
	}
	var atoms []string
	for _, learning := range learnings {
		if len(learning.FactArgs) >= 2 {
			domain, _ := learning.FactArgs[0].(string)
			fact, _ := learning.FactArgs[1].(string)
			atoms = append(atoms, fmt.Sprintf("[%s] %s", domain, fact))
		}
	}
	// Limit to 10
	if len(atoms) > 10 {
		atoms = atoms[:10]
	}
	return atoms
}

// querySpecialistHints gets hints from specialist knowledge base.
func (m *Model) querySpecialistHints() []string {
	if m.learningStore == nil {
		return nil
	}
	learnings, err := m.learningStore.LoadByPredicate("specialist", "hint")
	if err != nil {
		return nil
	}
	var hints []string
	for _, learning := range learnings {
		if len(learning.FactArgs) >= 1 {
			if hint, ok := learning.FactArgs[0].(string); ok {
				hints = append(hints, hint)
			}
		}
	}
	// Limit to 5
	if len(hints) > 5 {
		hints = hints[:5]
	}
	return hints
}

// queryAllowedActions gets permitted actions from constitutional rules.
func (m *Model) queryAllowedActions() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("permitted")
	if err != nil {
		return nil
	}
	var actions []string
	for _, fact := range results {
		if len(fact.Args) >= 1 {
			if action, ok := fact.Args[0].(string); ok {
				actions = append(actions, action)
			}
		}
	}
	return actions
}

// queryBlockedActions gets denied actions from constitutional rules.
func (m *Model) queryBlockedActions() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("blocked_action")
	if err != nil {
		return nil
	}
	var actions []string
	for _, fact := range results {
		if len(fact.Args) >= 1 {
			if action, ok := fact.Args[0].(string); ok {
				actions = append(actions, action)
			}
		}
	}
	return actions
}

// querySafetyWarnings gets active safety concerns.
func (m *Model) querySafetyWarnings() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("safety_warning")
	if err != nil {
		return nil
	}
	var warnings []string
	for _, fact := range results {
		if len(fact.Args) >= 1 {
			if warning, ok := fact.Args[0].(string); ok {
				warnings = append(warnings, warning)
			}
		}
	}
	return warnings
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

// traceUpdateMsg carries a new derivation trace
type traceUpdateMsg struct {
	Trace *mangle.DerivationTrace
}

// fetchTrace queries the kernel for the most recent interesting trace
func (m Model) fetchTrace(query string) tea.Cmd {
	return func() tea.Msg {
		if m.kernel == nil {
			return nil
		}

		// If query is empty, try to get the most recent derived context
		targetQuery := query
		if targetQuery == "" {
			targetQuery = "context_atom(?params)"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		trace, err := m.kernel.TraceQuery(ctx, targetQuery)
		if err != nil {
			// Fail silently or log?
			return nil
		}

		return traceUpdateMsg{Trace: trace}
	}
}
