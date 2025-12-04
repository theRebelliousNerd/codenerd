// Package main provides the codeNERD CLI entry point.
// This file implements the interactive chat interface using bubbletea.
package main

import (
	"codenerd/cmd/nerd/config"
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/tactile"
	"codenerd/internal/world"
	"context"
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
	emitter := articulation.NewEmitter()
	scanner := world.NewScanner()

	return chatModel{
		textinput:             ti,
		viewport:              vp,
		spinner:               sp,
		styles:                styles,
		renderer:              renderer,
		history:               []chatMessage{},
		config:                cfg,
		client:                llmClient,
		kernel:                kernel,
		shardMgr:              shardMgr,
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
| /quit, /exit, /q | Exit the CLI |

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

		// Built-in agents
		sb.WriteString("### Built-in (Type 2: Ephemeral)\n")
		sb.WriteString("| Name | Capabilities |\n")
		sb.WriteString("|------|-------------|\n")
		sb.WriteString("| researcher | Deep web research, codebase analysis |\n")
		sb.WriteString("| coder | Code generation, refactoring |\n")
		sb.WriteString("| reviewer | Code review, best practices |\n")
		sb.WriteString("| tester | Test generation, TDD loop |\n\n")

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

	// Input area
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Accent).
		Padding(0, 1)

	inputArea := inputStyle.Render(m.textinput.View())

	// Footer
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
	help := m.styles.Muted.Render("Enter: send • /help: commands • Ctrl+C: exit")
	return lipgloss.NewStyle().
		MarginTop(1).
		Render(help)
}

// runInit performs workspace initialization
func (m chatModel) runInit() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Create initializer with LLM client
		initConfig := struct {
			Workspace string
			LLMClient perception.LLMClient
			Timeout   time.Duration
		}{
			Workspace: m.workspace,
			LLMClient: m.client,
			Timeout:   5 * time.Minute,
		}

		// Simple initialization - create .nerd directory and scan codebase
		nerdDir := m.workspace + "/.nerd"
		if err := createDirIfNotExists(nerdDir); err != nil {
			return errorMsg(fmt.Errorf("failed to create .nerd directory: %w", err))
		}
		if err := createDirIfNotExists(nerdDir + "/shards"); err != nil {
			return errorMsg(fmt.Errorf("failed to create shards directory: %w", err))
		}

		// Scan workspace
		facts, err := m.scanner.ScanWorkspace(m.workspace)
		if err != nil {
			return errorMsg(fmt.Errorf("failed to scan workspace: %w", err))
		}

		// Load facts into kernel
		if err := m.kernel.LoadFacts(facts); err != nil {
			return errorMsg(fmt.Errorf("failed to load facts: %w", err))
		}

		// Detect project type
		projectType := detectProjectType(m.workspace)

		// Generate summary
		summary := fmt.Sprintf(`## Initialization Complete

**Workspace**: %s
**Language**: %s
**Framework**: %s
**Architecture**: %s
**Files Indexed**: %d

The .nerd/ directory has been created with:
- Knowledge database
- Shard storage
- Project profile

You can now use codeNERD with full context awareness.

**Next steps:**
- Define specialist agents: ` + "`/define-agent <name>`" + `
- Query the codebase: Just ask questions!
- Run tasks: ` + "`/spawn <agent> <task>`",
			m.workspace, projectType.Language, projectType.Framework, projectType.Architecture, len(facts))

		_ = initConfig // Suppress unused warning
		_ = ctx

		return responseMsg(summary)
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
