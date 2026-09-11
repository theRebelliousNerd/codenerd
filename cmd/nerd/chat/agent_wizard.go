package chat

import (
	nerdconfig "codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	coresys "codenerd/internal/system"
	"context"
	"fmt"
	"os"
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
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Agent name cannot be empty. Please enter a name (e.g., 'RustExpert'):",
				Time:    time.Now(),
			})
			return m, nil
		}
		// The name becomes a directory under .nerd/agents and a knowledge
		// database path, so it is validated where it is typed rather than
		// failing later inside the research goroutine.
		if err := coresys.ValidateAgentName(name); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("%v. Please enter a name (e.g., 'RustExpert'):", err),
				Time:    time.Now(),
			})
			return m, nil
		}
		m.agentWizard.Name = name
		m.agentWizard.Step = 1
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Great, I'll call it **%s**.\n\nWhat is this agent's primary role or domain of expertise?\n(e.g., 'Expert in async Rust programming and Tokio runtime')", name),
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Describe the agent's role..."
		return m, nil

	case 1: // Role
		role := strings.TrimSpace(input)
		if role == "" {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Role cannot be empty. Please describe the agent's role:",
				Time:    time.Now(),
			})
			return m, nil
		}
		m.agentWizard.Role = role
		m.agentWizard.Step = 2
		m = m.addMessage(Message{
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

		// Transition out of wizard mode in UI, but keep state for async process.
		// Note: agentWizard pointer is retained because the async research goroutine
		// still references it; setInputMode would nil it, so we set inputMode directly.
		m.inputMode = InputModeNormal
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
		config := coreshards.DefaultSpecialistConfig(wizard.Name, fmt.Sprintf("memory/shards/%s_knowledge.db", wizard.Name))
		m.shardMgr.DefineProfile(wizard.Name, config)

		// 2. An existing definition is never clobbered, so decide up front
		// whether the research can land in prompts.yaml at all.
		promptsPath := coresys.AgentPromptsPath(m.workspace, wizard.Name)
		_, statErr := os.Stat(promptsPath)
		alreadyDefined := statErr == nil

		// 3. Research. The result IS the agent's domain atom, so it is asked
		// for as prompt text rather than as Mangle facts.
		researchTask := fmt.Sprintf(
			"Research the following topics for a new '%s' agent (%s).\nTopics/Docs: %s.\n\n"+
				"Write the domain-knowledge section of this agent's system prompt in markdown: "+
				"key concepts, APIs and patterns, common pitfalls, best practices, and references with URLs. "+
				"No preamble; the text is used verbatim.",
			wizard.Name, wizard.Role, wizard.Topics,
		)

		ctx, cancel := context.WithTimeout(context.Background(), nerdconfig.GetLLMTimeouts().ShardExecutionTimeout)
		defer cancel()

		result, err := m.spawnTask(ctx, "researcher", researchTask)
		if err != nil {
			// The agent still exists on disk, with a domain atom that says
			// nothing has been researched, so the research can be re-run.
			if !alreadyDefined {
				if _, werr := coresys.WriteAgentDefinition(m.workspace, wizard.Name, wizard.Role, wizard.Topics, ""); werr != nil {
					logging.Session("agent wizard: prompts.yaml for %q was not written: %v", wizard.Name, werr)
				}
			}
			return errorMsg(fmt.Errorf("research failed: %w", err))
		}

		// 4. Write the definition with the researched knowledge as its domain
		// atom. Before this the result was shown once and discarded, and the
		// atom shipped "[Add specific concepts...]" placeholders instead.
		knowledgeNote := "Populated via Deep Research; written into the domain atom of prompts.yaml"
		if alreadyDefined {
			knowledgeNote = "Existing prompts.yaml kept as is; the research summary below was NOT written into it"
		} else if _, werr := coresys.WriteAgentDefinition(m.workspace, wizard.Name, wizard.Role, wizard.Topics, result); werr != nil {
			logging.Session("agent wizard: prompts.yaml for %q was not written: %v", wizard.Name, werr)
			knowledgeNote = fmt.Sprintf("prompts.yaml was not written (%v); the research summary is below", werr)
		}

		// 5. Persist the agent profile so it survives restart.
		if err := persistAgentProfile(m.workspace, wizard.Name, "persistent", config.KnowledgePath, 0, "active"); err != nil {
			// Deliberately non-fatal: the agent is usable this session even if
			// its profile did not persist. Silently discarding the error meant
			// the agent simply vanished on restart with nothing to explain it.
			logging.Session("agent wizard: profile for %q was not persisted and will not survive restart: %v",
				wizard.Name, err)
		}

		// 6. Clear wizard state
		m.agentWizard = nil

		spawnCmd := fmt.Sprintf("/spawn %s <task>", wizard.Name)

		response := fmt.Sprintf("## Agent Created: %s\n\n**Role**: %s\n**Status**: Ready\n**Knowledge Base**: %s\n**Prompts**: %s\n\n### Research Summary\n%s\n\nYou can now use this agent by running:\n`%s`\n\nTo customize this agent's behavior, edit the prompts.yaml file.",
			wizard.Name, wizard.Role, knowledgeNote, promptsPath, result, spawnCmd)

		return responseMsg(response)
	}
}
