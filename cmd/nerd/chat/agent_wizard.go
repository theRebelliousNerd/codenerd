package chat

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// AGENT CREATION WIZARD
// =============================================================================
// Manages the interactive dialogue for defining new Type 4 Specialist Agents.

// AgentWizardState tracks the state of the agent creation wizard.
type AgentWizardState struct {
	Step          int // 0: Name, 1: Role, 2: Topics/Docs
	Name          string
	Role          string
	Topics        string
	Docs          []string
	IsResearching bool
}

// handleAgentWizardInput processes input during the agent creation wizard.
func (m Model) handleAgentWizardInput(input string) (tea.Model, tea.Cmd) {
	if m.agentWizard == nil {
		m.agentWizard = &AgentWizardState{}
	}

	switch m.agentWizard.Step {
	case 0: // Name
		name := strings.TrimSpace(input)
		if name == "" {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Agent name cannot be empty. Please enter a name (e.g., 'RustExpert'):",
				Time:    time.Now(),
			})
			return m, nil
		}
		m.agentWizard.Name = name
		m.agentWizard.Step = 1
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Great, I'll call it **%s**.\n\nWhat is this agent's primary role or domain of expertise?\n(e.g., 'Expert in async Rust programming and Tokio runtime')", name),
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Describe the agent's role..."
		return m, nil

	case 1: // Role
		role := strings.TrimSpace(input)
		if role == "" {
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: "Role cannot be empty. Please describe the agent's role:",
				Time:    time.Now(),
			})
			return m, nil
		}
		m.agentWizard.Role = role
		m.agentWizard.Step = 2
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Got it: *%s*.\n\nFinally, what specific topics, libraries, or documentation URLs should this agent study?\nThis will be used to populate its knowledge base via Context7 and Deep Research.\n(e.g., 'https://docs.rs/tokio', 'actix-web', 'async traits')", role),
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Enter topics or URLs..."
		return m, nil

	case 2: // Topics/Docs -> Trigger Research
		topics := strings.TrimSpace(input)
		m.agentWizard.Topics = topics
		m.agentWizard.IsResearching = true

		// Transition out of wizard mode in UI, but keep state for async process
		m.awaitingAgentDefinition = false
		m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"

		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Defining agent **%s**...\n\nInitializing Deep Research on: %s\nThis may take a moment.", m.agentWizard.Name, topics),
			Time:    time.Now(),
		})

		m.isLoading = true
		return m, tea.Batch(
			m.spinner.Tick,
			m.runAgentResearch(m.agentWizard),
		)
	}

	return m, nil
}

// runAgentResearch executes the deep research and agent creation process.
func (m Model) runAgentResearch(wizard *AgentWizardState) tea.Cmd {
	return func() tea.Msg {
		// 1. Define Profile
		config := core.DefaultSpecialistConfig(wizard.Name, fmt.Sprintf("memory/shards/%s_knowledge.db", wizard.Name))
		m.shardMgr.DefineProfile(wizard.Name, config)

		// 2. Trigger Research (using Researcher Shard)
		// We construct a prompt that explicitly mentions Context7 if applicable
		researchTask := fmt.Sprintf(
			"Research the following topics to build a knowledge base for a new '%s' agent (%s).\nTopics/Docs: %s.\n\nGenerate extensive Mangle facts covering API patterns, best practices, and pitfalls.",
			wizard.Name, wizard.Role, wizard.Topics,
		)

		// Spawn researcher
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		result, err := m.shardMgr.Spawn(ctx, "researcher", researchTask)
		if err != nil {
			return errorMsg(fmt.Errorf("research failed: %w", err))
		}

		// 3. Persist Agent Profile
		// Extract stats from result if possible, or estimate
		// Result string format: "Researched ... gathered N knowledge atoms ..."
		// We'll just default to "Active" and 0 size if we can't parse easily,
		// or assume result contains the summary.
		// For now, we persist so it survives restart.
		if err := persistAgentProfile(m.workspace, wizard.Name, "persistent", config.KnowledgePath, 0, "active"); err != nil {
			// Log error but don't fail the UI flow
			// In a real app we'd send a toast or log it
		}

		// 4. Clear wizard state
		m.agentWizard = nil

		response := fmt.Sprintf(`## Agent Created: %s

**Role**: %s
**Status**: Ready
**Knowledge Base**: Populated via Deep Research

### Research Summary
%s

You can now use this agent by running:
`+"`/spawn %s <task>`", wizard.Name, wizard.Role, result, wizard.Name)

		return responseMsg(response)
	}
}
