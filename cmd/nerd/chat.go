// Package main provides the codeNERD CLI entry point.
// This file implements the interactive chat interface using bubbletea.
package main

import (
	"codenerd/cmd/nerd/config"
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/tactile"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

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
	splitPane    *ui.SplitPaneView
	logicPane    *ui.LogicPane
	showLogic    bool
	paneMode     ui.PaneMode

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

	// Session State
	sessionID  string
	turnCount  int

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
}

type chatMessage struct {
	role    string // "user" or "assistant"
	content string
	time    time.Time
}

// Messages for tea updates
type (
	responseMsg        string
	errorMsg           error
	windowSizeMsg      tea.WindowSizeMsg
	clarificationMsg   ClarificationState // Request for user clarification
	clarificationReply string             // User's response to clarification
)

// initChat initializes the interactive chat model
func initChat() chatModel {
	// Load configuration
	cfg, _ := config.Load()

	// Initialize styles
	styles := ui.DefaultStyles()
	if cfg.Theme == "dark" {
		styles = ui.NewStyles(ui.DarkTheme())
	}

	// Initialize textinput for input
	ti := textinput.New()
	ti.Placeholder = "Ask me anything... (Enter to send, Ctrl+C to exit)"
	ti.Focus()
	ti.Prompt = "│ "
	ti.CharLimit = 4096
	ti.Width = 80
	ti.PromptStyle = styles.Prompt
	ti.TextStyle = styles.UserInput

	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	// Initialize viewport for chat history
	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Initialize markdown renderer
	var renderer *glamour.TermRenderer
	if styles.Theme.IsDark {
		renderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(80),
		)
	} else {
		renderer, _ = glamour.NewTermRenderer(
			glamour.WithStylePath("light"),
			glamour.WithWordWrap(80),
		)
	}

	// Resolve workspace
	workspace, _ := os.Getwd()

	// Resolve API key
	apiKey := os.Getenv("ZAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		apiKey = cfg.APIKey
	}

	// Initialize backend components
	llmClient := perception.NewZAIClient(apiKey)
	transducer := perception.NewRealTransducer(llmClient)
	kernel := core.NewRealKernel()
	executor := tactile.NewSafeExecutor()
	shardMgr := core.NewShardManager()
	shardMgr.SetParentKernel(kernel)
	shadowMode := core.NewShadowMode(kernel)
	emitter := articulation.NewEmitter()
	scanner := world.NewScanner()

	// Initialize split-pane view (Glass Box Interface)
	splitPaneView := ui.NewSplitPaneView(styles, 80, 24)
	logicPane := ui.NewLogicPane(styles, 30, 20)

	return chatModel{
		textinput:             ti,
		viewport:              vp,
		spinner:               sp,
		styles:                styles,
		renderer:              renderer,
		splitPane:             &splitPaneView,
		logicPane:             &logicPane,
		showLogic:             false,
		paneMode:              ui.ModeSinglePane,
		history:               []chatMessage{},
		config:                cfg,
		client:                llmClient,
		kernel:                kernel,
		shardMgr:              shardMgr,
		shadowMode:            shadowMode,
		transducer:            transducer,
		executor:              executor,
		emitter:               emitter,
		scanner:               scanner,
		workspace:             workspace,
		sessionID:             fmt.Sprintf("sess_%d", time.Now().UnixNano()),
		turnCount:             0,
		awaitingClarification: false,
		selectedOption:        0,
	}
}

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

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 4
		footerHeight := 3
		inputHeight := 3 // Smaller input height for textinput

		if !m.ready {
			m.viewport = viewport.New(msg.Width-4, msg.Height-headerHeight-footerHeight-inputHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = msg.Height - headerHeight - footerHeight - inputHeight
		}

		m.textinput.Width = msg.Width - 4

		// Update split pane dimensions
		if m.splitPane != nil {
			m.splitPane.SetSize(msg.Width, msg.Height-headerHeight-footerHeight-inputHeight)
		}
		if m.logicPane != nil {
			m.logicPane.SetSize(msg.Width/3, msg.Height-headerHeight-footerHeight-inputHeight)
		}

		// Update renderer word wrap
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(msg.Width-8),
			)
		}

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
	m.isLoading = true

	// Process in background
	return m, tea.Batch(
		m.spinner.Tick,
		m.processInput(input),
	)
}

func (m chatModel) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/quit", "/exit", "/q":
		return m, tea.Quit

	case "/clear":
		m.history = []chatMessage{}
		m.viewport.SetContent("")
		m.textinput.Reset()
		return m, nil

	case "/help":
		help := `## Available Commands

| Command | Description |
|---------|-------------|
| /help | Show this help message |
| /clear | Clear chat history |
| /status | Show system status |
| /init | Initialize codeNERD in the workspace |
| /config set-key <key> | Set API key |
| /config set-theme <theme> | Set theme (light/dark) |
| /spawn <type> <task> | Spawn a shard agent |
| /define-agent <name> | Define a Type 4 specialist agent |
| /agents | List all defined agents |
| /query <predicate> | Query the Mangle kernel |
| /why [predicate] | Explain logic derivation |
| /logic [predicate] | Show derivation trace in Glass Box |
| /shadow <action> | Start Shadow Mode simulation |
| /whatif <action> | Quick counterfactual query |
| /approve | Review pending mutations (Interactive Diff) |
| /quit, /exit, /q | Exit the CLI |

## Glass Box Interface (Split-Pane TUI)
| Keybinding | Description |
|------------|-------------|
| Ctrl+L | Toggle logic pane on/off |
| Ctrl+G | Cycle views: Chat → Split → Logic |
| Ctrl+R | Toggle focus between panes |

## Shard Types
| Type | Lifecycle | Use Case |
|------|-----------|----------|
| Type 1 (System) | Always On | Core functions |
| Type 2 (Ephemeral) | Spawn->Die | Quick tasks |
| Type 3 (Persistent) | LLM-Created | Background monitoring |
| Type 4 (User) | User-Defined | Domain experts |

## Tips
- **Enter** to send a message
- **Ctrl+C** or **Esc** to exit
- Use **↑/↓** to scroll history
`
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: help,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/config":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/config set-key <key>` or `/config set-theme <light|dark>`",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		subCmd := parts[1]
		switch subCmd {
		case "set-key":
			if len(parts) < 3 {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "Usage: `/config set-key <key>`",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}
			key := parts[2]
			m.config.APIKey = key
			if err := config.Save(m.config); err != nil {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: fmt.Sprintf("Error saving config: %v", err),
					time:    time.Now(),
				})
			} else {
				// Re-initialize client
				m.client = perception.NewZAIClient(key)
				m.transducer = perception.NewRealTransducer(m.client)
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "✅ API key saved and client updated.",
					time:    time.Now(),
				})
			}

		case "set-theme":
			if len(parts) < 3 {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "Usage: `/config set-theme <light|dark>`",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}
			theme := parts[2]
			if theme != "light" && theme != "dark" {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "Invalid theme. Use 'light' or 'dark'.",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}
			m.config.Theme = theme
			if err := config.Save(m.config); err != nil {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: fmt.Sprintf("Error saving config: %v", err),
					time:    time.Now(),
				})
			} else {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: fmt.Sprintf("✅ Theme set to '%s'. Restart CLI to apply.", theme),
					time:    time.Now(),
				})
			}
		}

		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/status":
		status := fmt.Sprintf(`## System Status

- **Workspace**: %s
- **Kernel**: Active
- **Shards**: %d active
- **Session**: %s (Turn %d)
- **Time**: %s
- **Config**: %s
`, m.workspace, len(m.shardMgr.GetActiveShards()), m.sessionID[:16], m.turnCount, time.Now().Format(time.RFC3339), func() string {
			path, _ := config.ConfigFile()
			return path
		}())

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: status,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/init":
		// Run comprehensive initialization in background
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: `🚀 **Initializing codeNERD in workspace...**

This comprehensive initialization will:
1. 📁 Create ` + "`.nerd/`" + ` directory structure
2. 📊 Deep scan the codebase for project profile
3. 🔬 Run Researcher shard for analysis
4. 🧠 Generate initial Mangle facts
5. 🤖 Determine & create Type 3 agents
6. 📚 Build knowledge bases for each agent
7. ⚙️ Initialize preferences & session state

_This may take a minute for large codebases..._`,
			time: time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.runInit(),
		)

	case "/define-agent":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/define-agent <name> [--topic <topic>]`\n\nExample:\n```\n/define-agent RustExpert --topic \"Tokio async runtime\"\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		agentName := parts[1]
		topic := ""
		for i, p := range parts {
			if p == "--topic" && i+1 < len(parts) {
				topic = strings.Join(parts[i+1:], " ")
				break
			}
		}

		// Define the agent profile (Type 4: User Configured)
		config := core.DefaultSpecialistConfig(agentName, fmt.Sprintf(".nerd/shards/%s_knowledge.db", agentName))
		m.shardMgr.DefineProfile(agentName, config)

		response := fmt.Sprintf(`## Agent Defined: %s

**Type**: 4 (User Configured - Persistent Specialist)
**Topic**: %s
**Knowledge Path**: .nerd/shards/%s_knowledge.db
**Model**: High Reasoning (glm4)

The agent will undergo deep research on first spawn to build its knowledge base.

**Next steps:**
- Run research: ` + "`/spawn researcher %s research`" + `
- Use the agent: ` + "`/spawn %s <task>`", agentName, topic, agentName, topic, agentName)

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: response,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/agents":
		var sb strings.Builder
		sb.WriteString("## Defined Agents\n\n")

		// Built-in agents (Type 2: Ephemeral)
		sb.WriteString("### Built-in (Type 2: Ephemeral)\n")
		sb.WriteString("| Name | Capabilities |\n")
		sb.WriteString("|------|-------------|\n")
		sb.WriteString("| researcher | Deep web research, codebase analysis |\n")
		sb.WriteString("| coder | Code generation, refactoring |\n")
		sb.WriteString("| reviewer | Code review, best practices |\n")
		sb.WriteString("| tester | Test generation, TDD loop |\n\n")

		// Type 3 agents (LLM-Created Persistent)
		type3Agents := m.loadType3Agents()
		if len(type3Agents) > 0 {
			sb.WriteString("### Auto-Created (Type 3: Persistent)\n")
			sb.WriteString("| Name | KB Size | Status |\n")
			sb.WriteString("|------|---------|--------|\n")
			for _, agent := range type3Agents {
				sb.WriteString(fmt.Sprintf("| %s | %d atoms | %s |\n", agent.Name, agent.KBSize, agent.Status))
			}
			sb.WriteString("\n")
		}

		// User-defined agents (Type 4)
		sb.WriteString("### User-Defined (Type 4: Specialist)\n")
		profiles := m.getDefinedProfiles()
		if len(profiles) == 0 {
			sb.WriteString("_No user-defined agents. Use `/define-agent <name>` to create one._\n")
		} else {
			sb.WriteString("| Name | Knowledge Path |\n")
			sb.WriteString("|------|---------------|\n")
			for name, cfg := range profiles {
				sb.WriteString(fmt.Sprintf("| %s | %s |\n", name, cfg.KnowledgePath))
			}
		}

		sb.WriteString("\n### Commands\n")
		sb.WriteString("- Spawn agent: `/spawn <agent> <task>`\n")
		sb.WriteString("- Define new: `/define-agent <name> --topic <topic>`\n")

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: sb.String(),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/spawn":
		if len(parts) < 3 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/spawn <agent-type> <task>`\n\nExamples:\n```\n/spawn researcher \"analyze auth system\"\n/spawn coder \"implement user login\"\n/spawn RustExpert \"review async code\"\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		shardType := parts[1]
		task := strings.Join(parts[2:], " ")

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔄 Spawning **%s** shard for task: %s", shardType, task),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.spawnShard(shardType, task),
		)

	case "/query":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/query <predicate>`\n\nExamples:\n```\n/query next_action\n/query impacted\n/query block_commit\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		predicate := parts[1]
		facts, err := m.kernel.Query(predicate)

		var response string
		if err != nil {
			response = fmt.Sprintf("Query error: %v", err)
		} else if len(facts) == 0 {
			response = fmt.Sprintf("No facts found for `%s`", predicate)
		} else {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("## Query: %s\n\n", predicate))
			sb.WriteString("```datalog\n")
			for _, fact := range facts {
				sb.WriteString(fact.String() + "\n")
			}
			sb.WriteString("```\n")
			response = sb.String()
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: response,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/why":
		predicate := "next_action"
		if len(parts) >= 2 {
			predicate = parts[1]
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## Explaining: %s\n\n", predicate))

		facts, _ := m.kernel.Query(predicate)
		if len(facts) == 0 {
			sb.WriteString("No facts derived for this predicate.\n\n")
			sb.WriteString("**Possible reasons:**\n")
			sb.WriteString("- Required preconditions not met\n")
			sb.WriteString("- No matching rules triggered\n")
			sb.WriteString("- Workspace not scanned\n")
		} else {
			sb.WriteString("**Derived facts:**\n```datalog\n")
			for _, fact := range facts {
				sb.WriteString(fact.String() + "\n")
			}
			sb.WriteString("```\n\n")

			// Show related rules
			sb.WriteString("**Related policy rules:**\n")
			switch predicate {
			case "next_action":
				sb.WriteString("```datalog\nnext_action(A) :- user_intent(_, V, T, _), action_mapping(V, A).\nnext_action(/ask_user) :- clarification_needed(_).\nnext_action(/interrogative_mode) :- ambiguity_detected(_).\n```")
			case "block_commit":
				sb.WriteString("```datalog\nblock_commit(\"Build Broken\") :- diagnostic(/error, _, _, _, _).\nblock_commit(\"Tests Failing\") :- test_state(/failing).\n```")
			case "impacted":
				sb.WriteString("```datalog\nimpacted(X) :- dependency_link(X, Y, _), modified(Y).\nimpacted(X) :- dependency_link(X, Z, _), impacted(Z). # Transitive\n```")
			case "clarification_needed":
				sb.WriteString("```datalog\nclarification_needed(Ref) :- focus_resolution(Ref, _, _, Score), Score < 0.85.\nclarification_needed(File) :- chesterton_fence_warning(File, _).\n```")
			default:
				sb.WriteString("_(See policy.gl for rule definitions)_")
			}
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: sb.String(),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/logic":
		// Show derivation trace in the Glass Box pane
		predicate := "next_action"
		if len(parts) >= 2 {
			predicate = parts[1]
		}

		// Query the kernel
		facts, _ := m.kernel.Query(predicate)

		// Build derivation trace
		trace := m.buildDerivationTrace(predicate, facts)

		// Update the logic pane
		if m.splitPane != nil && m.splitPane.RightPane != nil {
			m.splitPane.RightPane.SetTrace(trace)
		}
		if m.logicPane != nil {
			m.logicPane.SetTrace(trace)
		}

		// Enable split view if not already enabled
		if !m.showLogic {
			m.showLogic = true
			m.paneMode = ui.ModeSplitPane
			m.splitPane.SetMode(ui.ModeSplitPane)
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔬 Showing derivation trace for `%s` in the Glass Box pane.\n\nUse **Ctrl+L** to toggle the logic view.", predicate),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/shadow":
		// Start Shadow Mode simulation
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/shadow <action-type> <target>`\n\n**Action Types:**\n- `write <file>` - Simulate file modification\n- `delete <file>` - Simulate file deletion\n- `refactor <file>` - Simulate refactoring\n- `commit` - Simulate git commit\n\n**Examples:**\n```\n/shadow write src/auth/handler.go\n/shadow refactor internal/core/kernel.go\n/shadow commit\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		actionType := parts[1]
		target := ""
		if len(parts) >= 3 {
			target = strings.Join(parts[2:], " ")
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🌑 **Starting Shadow Mode Simulation**\n\nAction: `%s`\nTarget: `%s`\n\n_Running counterfactual analysis..._", actionType, target),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.runShadowSimulation(actionType, target),
		)

	case "/whatif":
		// Quick counterfactual query
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/whatif <scenario>`\n\n**Examples:**\n```\n/whatif I delete auth/handler.go\n/whatif I refactor the login function\n/whatif tests fail after this change\n```\n\nThis runs a quick counterfactual analysis without starting a full simulation.",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		scenario := strings.Join(parts[1:], " ")

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔮 **What-If Analysis**\n\nScenario: _\"%s\"_\n\n_Projecting effects..._", scenario),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.runWhatIfQuery(scenario),
		)

	case "/approve":
		// Interactive Diff Approval
		// Query for pending mutations requiring approval
		pendingMutations, _ := m.kernel.Query("pending_mutation")
		requiresApproval, _ := m.kernel.Query("requires_approval")

		var sb strings.Builder
		sb.WriteString("## 📝 Interactive Diff Approval\n\n")

		if len(pendingMutations) == 0 {
			sb.WriteString("✅ **No pending mutations** - All changes have been reviewed or there are no pending changes.\n\n")
			sb.WriteString("Mutations require approval when:\n")
			sb.WriteString("- Chesterton's Fence warning is triggered (recent changes by others)\n")
			sb.WriteString("- Code impacts other files transitively\n")
			sb.WriteString("- Shadow Mode simulation detected potential issues\n")
		} else {
			sb.WriteString(fmt.Sprintf("Found **%d pending mutation(s)** requiring review:\n\n", len(pendingMutations)))

			sb.WriteString("| # | File | Reason |\n")
			sb.WriteString("|---|------|--------|\n")
			for i, mutation := range pendingMutations {
				file := "unknown"
				if len(mutation.Args) > 1 {
					file = fmt.Sprintf("%v", mutation.Args[1])
				}
				reason := "approval_required"
				for _, ra := range requiresApproval {
					if len(ra.Args) > 0 && fmt.Sprintf("%v", ra.Args[0]) == fmt.Sprintf("%v", mutation.Args[0]) {
						reason = "safety_check"
					}
				}
				sb.WriteString(fmt.Sprintf("| %d | %s | %s |\n", i+1, file, reason))
			}

			sb.WriteString("\n### Approval Commands\n\n")
			sb.WriteString("```\n")
			sb.WriteString("/approve accept <id>  - Approve a specific mutation\n")
			sb.WriteString("/approve reject <id>  - Reject a specific mutation\n")
			sb.WriteString("/approve all          - Approve all pending mutations\n")
			sb.WriteString("/approve clear        - Clear all pending mutations\n")
			sb.WriteString("```\n")
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: sb.String(),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	default:
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("Unknown command: `%s`. Type `/help` for available commands.", cmd),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}
}

func (m chatModel) processInput(input string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Parse intent via transducer
		intent, err := m.transducer.ParseIntent(ctx, input)
		if err != nil {
			return errorMsg(fmt.Errorf("perception error: %w", err))
		}

		// Load workspace facts
		fileFacts, err := m.scanner.ScanWorkspace(m.workspace)
		if err != nil {
			// Non-fatal, continue without workspace facts
			fileFacts = nil
		}

		// Load facts into kernel
		if err := m.kernel.LoadFacts([]core.Fact{intent.ToFact()}); err != nil {
			return errorMsg(fmt.Errorf("kernel load error: %w", err))
		}
		if len(fileFacts) > 0 {
			_ = m.kernel.LoadFacts(fileFacts)
		}

		// Load shard facts
		_ = m.kernel.LoadFacts(m.shardMgr.ToFacts())

		// Query for actions
		actions, _ := m.kernel.Query("next_action")

		// Build the dual payload for articulation
		var mangleUpdates []string
		for _, action := range actions {
			mangleUpdates = append(mangleUpdates, action.Predicate)
		}

		// Generate surface response
		var surfaceResponse string
		if len(actions) > 0 {
			surfaceResponse = fmt.Sprintf("Processed: %s → %s\n\n%s", intent.Verb, intent.Target, intent.Response)
		} else {
			// Use the piggybacked response directly
			surfaceResponse = intent.Response
		}

		payload := articulation.PiggybackEnvelope{
			Surface: surfaceResponse,
			Control: articulation.ControlPacket{
				IntentClassification: articulation.IntentClassification{
					Category:   intent.Category,
					Verb:       intent.Verb,
					Target:     intent.Target,
					Constraint: intent.Constraint,
					Confidence: intent.Confidence,
				},
				MangleUpdates: mangleUpdates,
			},
		}

		// Format response
		response := formatResponse(intent, payload)

		return responseMsg(response)
	}
}

func formatResponse(intent perception.Intent, payload articulation.PiggybackEnvelope) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**Intent**: `%s` → `%s`\n\n", intent.Verb, intent.Target))

	// Show reasoning trace
	// Show intent classification
	sb.WriteString(fmt.Sprintf("Confidence: %.2f\n\n", payload.Control.IntentClassification.Confidence))

	// Show mangle updates
	if len(payload.Control.MangleUpdates) > 0 {
		sb.WriteString("**Mangle Facts:**\n```datalog\n")
		for _, update := range payload.Control.MangleUpdates {
			sb.WriteString(update + "\n")
		}
		sb.WriteString("```\n\n")
	}

	sb.WriteString("✅ " + payload.Surface)

	return sb.String()
}

func (m chatModel) renderHistory() string {
	var sb strings.Builder

	for _, msg := range m.history {
		if msg.role == "user" {
			// Render user message
			userStyle := m.styles.Bold.
				Foreground(m.styles.Theme.Primary).
				MarginTop(1)
			sb.WriteString(userStyle.Render("You") + "\n")
			sb.WriteString(m.styles.UserInput.Render(msg.content))
			sb.WriteString("\n\n")
		} else {
			// Render assistant message with markdown
			assistantStyle := m.styles.Bold.
				Foreground(m.styles.Theme.Accent).
				MarginTop(1)
			sb.WriteString(assistantStyle.Render("🧠 codeNERD") + "\n")

			// Render markdown with panic recovery
			rendered := m.safeRenderMarkdown(msg.content)
			sb.WriteString(rendered)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// safeRenderMarkdown renders markdown with panic recovery
func (m chatModel) safeRenderMarkdown(content string) (result string) {
	defer func() {
		if r := recover(); r != nil {
			// If glamour panics, return plain text
			result = content
		}
	}()

	if m.renderer != nil && content != "" {
		rendered, err := m.renderer.Render(content)
		if err == nil {
			return rendered
		}
	}
	return content
}

func (m chatModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Header
	header := m.renderHeader()

	// Chat viewport
	chatView := m.styles.Content.Render(m.viewport.View())

	// Loading indicator
	if m.isLoading {
		chatView += "\n" + m.styles.Spinner.Render(m.spinner.View()) + " Thinking..."
	}

	// Error display
	if m.err != nil {
		chatView += "\n" + m.styles.Error.Render("Error: "+m.err.Error())
	}

	// Apply split-pane view if enabled (Glass Box Interface)
	if m.showLogic && m.splitPane != nil {
		chatView = m.splitPane.Render(chatView)
	}

	// Input area
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Accent).
		Padding(0, 1)

	inputArea := inputStyle.Render(m.textinput.View())

	// Footer (with mode indicator)
	footer := m.renderFooter()

	// Compose full view
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		chatView,
		inputArea,
		footer,
	)
}

func (m chatModel) renderHeader() string {
	// Logo and title
	title := m.styles.Header.Render(" 🧠 codeNERD ")
	version := m.styles.Badge.Render("v1.0")
	workspace := m.styles.Muted.Render(fmt.Sprintf(" 📁 %s", m.workspace))

	// Status indicators
	var status string
	if m.isLoading {
		status = m.styles.Warning.Render("● Processing")
	} else {
		status = m.styles.Success.Render("● Ready")
	}

	headerLine := lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		" ",
		version,
		"  ",
		status,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerLine,
		workspace,
		m.styles.RenderDivider(m.width),
	)
}

func (m chatModel) renderFooter() string {
	// Build mode indicator
	modeIndicator := ""
	switch m.paneMode {
	case ui.ModeSinglePane:
		modeIndicator = "📝 Chat"
	case ui.ModeSplitPane:
		modeIndicator = "🔬 Split (Chat + Logic)"
	case ui.ModeFullLogic:
		modeIndicator = "🔬 Logic View"
	}

	help := m.styles.Muted.Render(fmt.Sprintf("%s • Enter: send • Ctrl+L: toggle logic • Ctrl+G: cycle views • /help: commands • Ctrl+C: exit", modeIndicator))
	return lipgloss.NewStyle().
		MarginTop(1).
		Render(help)
}

// runShadowSimulation runs a full Shadow Mode simulation
func (m chatModel) runShadowSimulation(actionType, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Map action type string to SimActionType
		var at core.SimActionType
		switch actionType {
		case "write":
			at = core.ActionTypeFileWrite
		case "delete":
			at = core.ActionTypeFileDelete
		case "refactor":
			at = core.ActionTypeRefactor
		case "exec":
			at = core.ActionTypeExec
		case "commit":
			at = core.ActionTypeGitCommit
		default:
			return errorMsg(fmt.Errorf("unknown action type: %s", actionType))
		}

		// Start simulation
		sim, err := m.shadowMode.StartSimulation(ctx, fmt.Sprintf("%s %s", actionType, target))
		if err != nil {
			return errorMsg(fmt.Errorf("failed to start simulation: %w", err))
		}

		// Create the action
		action := core.SimulatedAction{
			ID:          fmt.Sprintf("action_%d", time.Now().UnixNano()),
			Type:        at,
			Target:      target,
			Description: fmt.Sprintf("%s on %s", actionType, target),
		}

		// Run simulation
		result, err := m.shadowMode.SimulateAction(ctx, action)
		if err != nil {
			m.shadowMode.AbortSimulation(err.Error())
			return errorMsg(fmt.Errorf("simulation failed: %w", err))
		}

		// Build response
		var sb strings.Builder
		sb.WriteString("## 🌑 Shadow Mode Simulation Complete\n\n")
		sb.WriteString(fmt.Sprintf("**Simulation ID**: `%s`\n", sim.ID))
		sb.WriteString(fmt.Sprintf("**Action**: %s → %s\n\n", actionType, target))

		// Show projected effects
		sb.WriteString("### Projected Effects\n\n")
		if len(result.Effects) == 0 {
			sb.WriteString("_No effects projected._\n\n")
		} else {
			sb.WriteString("```datalog\n")
			for _, effect := range result.Effects {
				op := "+"
				if !effect.IsPositive {
					op = "-"
				}
				sb.WriteString(fmt.Sprintf("%s %s(%v)\n", op, effect.Predicate, effect.Args))
			}
			sb.WriteString("```\n\n")
		}

		// Show violations
		sb.WriteString("### Safety Analysis\n\n")
		if len(result.Violations) == 0 {
			sb.WriteString("✅ **No violations detected** - Action appears safe.\n\n")
		} else {
			for _, v := range result.Violations {
				icon := "⚠️"
				if v.Blocking {
					icon = "🛑"
				}
				sb.WriteString(fmt.Sprintf("%s **%s**: %s\n", icon, v.ViolationType, v.Description))
			}
			sb.WriteString("\n")
		}

		// Overall verdict
		if result.IsSafe {
			sb.WriteString("### ✅ Verdict: SAFE\n\n")
			sb.WriteString("The simulated action passes all safety checks.\n")
		} else {
			sb.WriteString("### 🛑 Verdict: BLOCKED\n\n")
			sb.WriteString("The action would be blocked by safety rules.\n")
		}

		// Abort the simulation (don't apply changes)
		m.shadowMode.AbortSimulation("simulation complete - not applying")

		return responseMsg(sb.String())
	}
}

// runWhatIfQuery runs a quick counterfactual query
func (m chatModel) runWhatIfQuery(scenario string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Parse the scenario to determine action type
		scenarioLower := strings.ToLower(scenario)
		var actionType core.SimActionType
		var target string

		switch {
		case strings.Contains(scenarioLower, "delete"):
			actionType = core.ActionTypeFileDelete
			target = extractTarget(scenario, "delete")
		case strings.Contains(scenarioLower, "refactor"):
			actionType = core.ActionTypeRefactor
			target = extractTarget(scenario, "refactor")
		case strings.Contains(scenarioLower, "modify") || strings.Contains(scenarioLower, "change") || strings.Contains(scenarioLower, "edit"):
			actionType = core.ActionTypeFileWrite
			target = extractTarget(scenario, "modify", "change", "edit")
		case strings.Contains(scenarioLower, "commit"):
			actionType = core.ActionTypeGitCommit
			target = "HEAD"
		case strings.Contains(scenarioLower, "test") && strings.Contains(scenarioLower, "fail"):
			// Simulate test failure scenario
			actionType = core.ActionTypeExec
			target = "test"
		default:
			actionType = core.ActionTypeFileWrite
			target = scenario
		}

		// Create action
		action := core.SimulatedAction{
			ID:          fmt.Sprintf("whatif_%d", time.Now().UnixNano()),
			Type:        actionType,
			Target:      target,
			Description: scenario,
		}

		// Run what-if query
		result, err := m.shadowMode.WhatIf(ctx, action)
		if err != nil {
			return errorMsg(fmt.Errorf("what-if query failed: %w", err))
		}

		// Build response
		var sb strings.Builder
		sb.WriteString("## 🔮 What-If Analysis Results\n\n")
		sb.WriteString(fmt.Sprintf("**Scenario**: _%s_\n\n", scenario))
		sb.WriteString(fmt.Sprintf("**Interpreted as**: `%s` on `%s`\n\n", actionType, target))

		// Effects
		sb.WriteString("### If this happens, then:\n\n")
		if len(result.Effects) == 0 {
			sb.WriteString("- No immediate effects detected\n")
		} else {
			for _, effect := range result.Effects {
				sb.WriteString(fmt.Sprintf("- `%s(%v)` would be asserted\n", effect.Predicate, effect.Args))
			}
		}
		sb.WriteString("\n")

		// Consequences
		sb.WriteString("### Potential Consequences:\n\n")
		if len(result.Violations) == 0 {
			sb.WriteString("✅ No safety violations predicted.\n\n")
		} else {
			for _, v := range result.Violations {
				icon := "⚠️"
				if v.Blocking {
					icon = "🛑"
				}
				sb.WriteString(fmt.Sprintf("%s %s\n", icon, v.Description))
			}
			sb.WriteString("\n")
		}

		// Recommendation
		sb.WriteString("### Recommendation:\n\n")
		if result.IsSafe {
			sb.WriteString("👍 This action appears safe to proceed with.\n")
		} else {
			sb.WriteString("⚠️ Consider addressing the violations before proceeding.\n")
		}

		return responseMsg(sb.String())
	}
}

// extractTarget extracts the target from a scenario description
func extractTarget(scenario string, keywords ...string) string {
	words := strings.Fields(scenario)
	for i, word := range words {
		for _, kw := range keywords {
			if strings.EqualFold(word, kw) && i+1 < len(words) {
				// Return everything after the keyword
				return strings.Join(words[i+1:], " ")
			}
		}
	}
	// Return the whole scenario if no keyword found
	return scenario
}

// buildDerivationTrace constructs a derivation trace from kernel facts
func (m chatModel) buildDerivationTrace(predicate string, facts []core.Fact) *ui.DerivationTrace {
	trace := &ui.DerivationTrace{
		Query:       predicate,
		TotalFacts:  len(facts),
		DerivedTime: 10 * time.Millisecond, // Placeholder, could track actual time
		RootNodes:   make([]*ui.DerivationNode, 0),
	}

	// Build nodes from facts
	for _, fact := range facts {
		args := make([]string, len(fact.Args))
		for i, arg := range fact.Args {
			args[i] = fmt.Sprintf("%v", arg)
		}

		node := &ui.DerivationNode{
			Predicate:  fact.Predicate,
			Args:       args,
			Source:     "idb", // Assume derived unless we can determine otherwise
			Rule:       m.getRuleForPredicate(fact.Predicate),
			Expanded:   true,
			Activation: 0.8 + float64(len(facts)-1)*0.05, // Simulated activation
			Children:   m.getChildNodes(fact),
		}
		trace.RootNodes = append(trace.RootNodes, node)
	}

	// If no facts, create a placeholder
	if len(trace.RootNodes) == 0 {
		trace.RootNodes = append(trace.RootNodes, &ui.DerivationNode{
			Predicate:  predicate,
			Args:       []string{"(no facts derived)"},
			Source:     "edb",
			Expanded:   false,
			Activation: 0.0,
		})
	}

	return trace
}

// getRuleForPredicate returns the Mangle rule that derives a predicate
func (m chatModel) getRuleForPredicate(predicate string) string {
	// Map of predicates to their derivation rules
	ruleMap := map[string]string{
		"next_action":          "next_action(A) :- user_intent(_, V, T, _), action_mapping(V, A).",
		"block_commit":         "block_commit(R) :- diagnostic(/error, _, _, _, _).",
		"permitted":            "permitted(A) :- safe_action(A).",
		"impacted":             "impacted(X) :- dependency_link(X, Y, _), modified(Y).",
		"clarification_needed": "clarification_needed(R) :- focus_resolution(R, _, _, S), S < 0.85.",
		"unsafe_to_refactor":   "unsafe_to_refactor(T) :- impacted(D), not test_coverage(D).",
		"needs_research":       "needs_research(A) :- shard_profile(A, _, T, _), not knowledge_ingested(A).",
	}

	if rule, ok := ruleMap[predicate]; ok {
		return rule
	}
	return ""
}

// getChildNodes finds the supporting facts for a derived fact
func (m chatModel) getChildNodes(fact core.Fact) []*ui.DerivationNode {
	children := make([]*ui.DerivationNode, 0)

	// Based on the predicate, find related facts
	switch fact.Predicate {
	case "next_action":
		// next_action depends on user_intent and action_mapping
		intents, _ := m.kernel.Query("user_intent")
		for _, intent := range intents {
			args := make([]string, len(intent.Args))
			for i, arg := range intent.Args {
				args[i] = fmt.Sprintf("%v", arg)
			}
			children = append(children, &ui.DerivationNode{
				Predicate:  intent.Predicate,
				Args:       args,
				Source:     "edb",
				Expanded:   false,
				Activation: 0.7,
			})
		}

	case "permitted":
		// permitted depends on safe_action or admin_override
		safeActions, _ := m.kernel.Query("safe_action")
		for _, sa := range safeActions {
			args := make([]string, len(sa.Args))
			for i, arg := range sa.Args {
				args[i] = fmt.Sprintf("%v", arg)
			}
			children = append(children, &ui.DerivationNode{
				Predicate:  sa.Predicate,
				Args:       args,
				Source:     "edb",
				Expanded:   false,
				Activation: 0.6,
			})
		}

	case "impacted":
		// impacted depends on dependency_link and modified
		deps, _ := m.kernel.Query("dependency_link")
		for _, dep := range deps {
			args := make([]string, len(dep.Args))
			for i, arg := range dep.Args {
				args[i] = fmt.Sprintf("%v", arg)
			}
			children = append(children, &ui.DerivationNode{
				Predicate:  dep.Predicate,
				Args:       args,
				Source:     "edb",
				Expanded:   false,
				Activation: 0.5,
			})
		}

	case "block_commit":
		// block_commit depends on diagnostic or test_state
		diagnostics, _ := m.kernel.Query("diagnostic")
		for _, diag := range diagnostics {
			args := make([]string, len(diag.Args))
			for i, arg := range diag.Args {
				args[i] = fmt.Sprintf("%v", arg)
			}
			children = append(children, &ui.DerivationNode{
				Predicate:  diag.Predicate,
				Args:       args,
				Source:     "edb",
				Expanded:   false,
				Activation: 0.9, // High activation for blockers
			})
		}
	}

	return children
}

// runInit performs comprehensive workspace initialization
func (m chatModel) runInit() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Create the comprehensive initializer with all components
		initConfig := nerdinit.InitConfig{
			Workspace:    m.workspace,
			LLMClient:    m.client,
			ShardManager: m.shardMgr,
			Timeout:      10 * time.Minute,
			Interactive:  false,        // Non-interactive in chat mode
			SkipResearch: false,        // Do full research
			SkipAgentCreate: false,     // Create Type 3 agents
		}

		initializer := nerdinit.NewInitializer(initConfig)

		// Run the comprehensive initialization
		result, err := initializer.Initialize(ctx)
		if err != nil {
			return errorMsg(fmt.Errorf("initialization failed: %w", err))
		}

		// Load all generated facts into the kernel
		nerdDir := m.workspace + "/.nerd"
		factsPath := nerdDir + "/profile.gl"
		if _, statErr := os.Stat(factsPath); statErr == nil {
			// TODO: Load Mangle facts from file
			// For now, scan workspace to load basic facts
			facts, scanErr := m.scanner.ScanWorkspace(m.workspace)
			if scanErr == nil {
				_ = m.kernel.LoadFacts(facts)
			}
		}

		// Build the summary message
		var sb strings.Builder
		sb.WriteString("## ✅ Initialization Complete\n\n")

		sb.WriteString(fmt.Sprintf("**Project**: %s\n", result.Profile.Name))
		sb.WriteString(fmt.Sprintf("**Language**: %s\n", result.Profile.Language))
		if result.Profile.Framework != "" {
			sb.WriteString(fmt.Sprintf("**Framework**: %s\n", result.Profile.Framework))
		}
		sb.WriteString(fmt.Sprintf("**Architecture**: %s\n", result.Profile.Architecture))
		sb.WriteString(fmt.Sprintf("**Files Analyzed**: %d\n", result.Profile.FileCount))
		sb.WriteString(fmt.Sprintf("**Directories**: %d\n", result.Profile.DirectoryCount))
		sb.WriteString(fmt.Sprintf("**Facts Generated**: %d\n\n", result.FactsGenerated))

		// Show created agents
		if len(result.CreatedAgents) > 0 {
			sb.WriteString("### 🤖 Type 3 Agents Created\n\n")
			sb.WriteString("| Agent | Knowledge Atoms | Status |\n")
			sb.WriteString("|-------|-----------------|--------|\n")
			for _, agent := range result.CreatedAgents {
				sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", agent.Name, agent.KBSize, agent.Status))
			}
			sb.WriteString("\n")
		}

		// Show warnings if any
		if len(result.Warnings) > 0 {
			sb.WriteString("### ⚠️ Warnings\n\n")
			for _, w := range result.Warnings {
				sb.WriteString(fmt.Sprintf("- %s\n", w))
			}
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("**Duration**: %.2fs\n\n", result.Duration.Seconds()))

		sb.WriteString("### 💡 Next Steps\n\n")
		sb.WriteString("- View agents: `/agents`\n")
		sb.WriteString("- Spawn an agent: `/spawn <agent> <task>`\n")
		sb.WriteString("- Define custom agents: `/define-agent <name>`\n")
		sb.WriteString("- Query the codebase: Just ask questions!\n")

		return responseMsg(sb.String())
	}
}

// getDefinedProfiles returns user-defined agent profiles
func (m chatModel) getDefinedProfiles() map[string]core.ShardConfig {
	profiles := make(map[string]core.ShardConfig)

	// Get profiles from shard manager
	// Note: We need to iterate through known profile names
	// For now, we'll check some common ones and any that were defined this session
	knownProfiles := []string{
		"RustExpert", "SecurityAuditor", "K8sArchitect",
		"PythonExpert", "GoExpert", "ReactExpert",
	}

	for _, name := range knownProfiles {
		if cfg, ok := m.shardMgr.GetProfile(name); ok {
			profiles[name] = cfg
		}
	}

	return profiles
}

// loadType3Agents loads Type 3 agents from the agents.json registry
func (m chatModel) loadType3Agents() []nerdinit.CreatedAgent {
	agents := make([]nerdinit.CreatedAgent, 0)

	// Try to load from agents.json registry
	registryPath := m.workspace + "/.nerd/agents.json"
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return agents
	}

	// Parse the registry
	var registry struct {
		Version   string                   `json:"version"`
		CreatedAt string                   `json:"created_at"`
		Agents    []nerdinit.CreatedAgent  `json:"agents"`
	}

	if err := json.Unmarshal(data, &registry); err != nil {
		return agents
	}

	return registry.Agents
}

// spawnShard spawns a shard agent for a task
func (m chatModel) spawnShard(shardType, task string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := m.shardMgr.Spawn(ctx, shardType, task)
		if err != nil {
			return errorMsg(fmt.Errorf("shard spawn failed: %w", err))
		}

		response := fmt.Sprintf(`## Shard Execution Complete

**Agent**: %s
**Task**: %s

### Result
%s`, shardType, task, result)

		return responseMsg(response)
	}
}

// createDirIfNotExists creates a directory if it doesn't exist
func createDirIfNotExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// ProjectTypeInfo holds detected project characteristics
type ProjectTypeInfo struct {
	Language     string
	Framework    string
	Architecture string
}

// detectProjectType analyzes the workspace to determine project type
func detectProjectType(workspace string) ProjectTypeInfo {
	pt := ProjectTypeInfo{
		Language:     "unknown",
		Framework:    "unknown",
		Architecture: "unknown",
	}

	// Check for language markers
	markers := map[string]struct {
		lang  string
		build string
	}{
		"go.mod":           {"go", "go"},
		"Cargo.toml":       {"rust", "cargo"},
		"package.json":     {"javascript", "npm"},
		"requirements.txt": {"python", "pip"},
		"pom.xml":          {"java", "maven"},
	}

	for file, info := range markers {
		if _, err := os.Stat(workspace + "/" + file); err == nil {
			pt.Language = info.lang
			break
		}
	}

	// Detect architecture based on directory structure
	dirs := []string{"cmd", "internal", "pkg", "api", "services"}
	foundDirs := 0
	for _, dir := range dirs {
		if info, err := os.Stat(workspace + "/" + dir); err == nil && info.IsDir() {
			foundDirs++
		}
	}

	if foundDirs >= 3 {
		pt.Architecture = "clean_architecture"
	} else if _, err := os.Stat(workspace + "/docker-compose.yml"); err == nil {
		pt.Architecture = "microservices"
	} else {
		pt.Architecture = "monolith"
	}

	return pt
}

// runInteractiveChat starts the interactive chat interface
func runInteractiveChat() error {
	p := tea.NewProgram(
		initChat(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}
