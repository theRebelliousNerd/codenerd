package chat

import (
	nerdconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

		// 2. Generate prompts.yaml template
		if err := generateAgentPromptsTemplate(m.workspace, wizard.Name, wizard.Role, wizard.Topics); err != nil {
			// Log error but don't fail the UI flow - prompts can be added manually
			fmt.Printf("[AgentWizard] Warning: Failed to generate prompts.yaml: %v\n", err)
		}

		// 3. Trigger Research (using Researcher Shard)
		// We construct a prompt that explicitly mentions Context7 if applicable
		researchTask := fmt.Sprintf(
			"Research the following topics to build a knowledge base for a new '%s' agent (%s).\nTopics/Docs: %s.\n\nGenerate extensive Mangle facts covering API patterns, best practices, and pitfalls.",
			wizard.Name, wizard.Role, wizard.Topics,
		)

		// Spawn researcher
		ctx, cancel := context.WithTimeout(context.Background(), nerdconfig.GetLLMTimeouts().ShardExecutionTimeout)
		defer cancel()

		result, err := m.shardMgr.Spawn(ctx, "researcher", researchTask)
		if err != nil {
			return errorMsg(fmt.Errorf("research failed: %w", err))
		}

		// 4. Persist Agent Profile
		// Extract stats from result if possible, or estimate
		// Result string format: "Researched ... gathered N knowledge atoms ..."
		// We'll just default to "Active" and 0 size if we can't parse easily,
		// or assume result contains the summary.
		// For now, we persist so it survives restart.
		if err := persistAgentProfile(m.workspace, wizard.Name, "persistent", config.KnowledgePath, 0, "active"); err != nil {
			// Log error but don't fail the UI flow
			// In a real app we'd send a toast or log it
		}

		// 5. Clear wizard state
		m.agentWizard = nil

		promptsPath := fmt.Sprintf(".nerd/agents/%s/prompts.yaml", wizard.Name)
		spawnCmd := fmt.Sprintf("/spawn %s <task>", wizard.Name)

		response := fmt.Sprintf("## Agent Created: %s\n\n**Role**: %s\n**Status**: Ready\n**Knowledge Base**: Populated via Deep Research\n**Prompts**: Template generated at %s\n\n### Research Summary\n%s\n\nYou can now use this agent by running:\n`%s`\n\nTo customize this agent's behavior, edit the prompts.yaml file.",
			wizard.Name, wizard.Role, promptsPath, result, spawnCmd)

		return responseMsg(response)
	}
}

// generateAgentPromptsTemplate generates a starter prompts.yaml template for a new agent.
// Creates .nerd/agents/{name}/prompts.yaml with identity, methodology, and domain knowledge atoms.
func generateAgentPromptsTemplate(workspace, agentName, role, topics string) error {
	// Create agent directory
	agentDir := filepath.Join(workspace, ".nerd", "agents", agentName)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent directory: %w", err)
	}

	// Generate prompts.yaml template
	promptsPath := filepath.Join(agentDir, "prompts.yaml")

	// Build the YAML template
	template := fmt.Sprintf(`# Prompt atoms for %[1]s
# These are loaded into the JIT prompt compiler when the agent is spawned.
# Edit this file to customize the agent's identity, methodology, and domain knowledge.

- id: "%[1]s/identity"
  category: "identity"
  subcategory: "%[1]s"
  priority: 100
  is_mandatory: true
  description: "Identity and mission for %[1]s"
  content_concise: |
    You are %[1]s, a specialist agent in the codeNERD ecosystem.
    Domain: %[2]s
    Topics: %[3]s
  content_min: |
    You are %[1]s (%[2]s). Operate under the codeNERD kernel.
  content: |
    You are %[1]s, a specialist agent in the codeNERD ecosystem.

    ## Domain
    %[2]s

    ## Research Topics
    %[3]s

    ## Core Responsibilities
    - Provide expert guidance in your domain
    - Follow best practices and established patterns
    - Maintain high code quality standards
    - Integrate seamlessly with the codeNERD architecture

    ## Execution Mode
    You operate under the control of the codeNERD kernel. You receive structured tasks
    with clear objectives, focus patterns, and success criteria. Execute precisely.

- id: "%[1]s/methodology"
  category: "methodology"
  subcategory: "%[1]s"
  priority: 80
  is_mandatory: false
  depends_on: ["%[1]s/identity"]
  description: "Methodology and quality bar for %[1]s"
  content_concise: |
    - Understand context before acting
    - Consider edge cases and failure modes
    - Write clear, maintainable code
    - Verify with tests when feasible
  content_min: |
    Be precise, verify assumptions, and preserve correctness.
  content: |
    ## Methodology

    ### Analysis Approach
    - Understand the full context before acting
    - Consider edge cases and failure modes
    - Think through implications of changes

    ### Implementation Standards
    - Follow language idioms and conventions
    - Write clear, maintainable code
    - Include comprehensive error handling
    - Document non-obvious decisions

    ### Quality Assurance
    - Verify assumptions before proceeding
    - Test critical paths
    - Consider performance implications
    - Ensure backward compatibility when applicable

- id: "%[1]s/domain"
  category: "domain"
  subcategory: "%[1]s"
  priority: 70
  is_mandatory: false
  depends_on: ["%[1]s/identity", "%[1]s/methodology"]
  description: "Domain knowledge, pitfalls, and references for %[1]s"
  content_concise: |
    Domain: %[2]s
    Topics: %[3]s
  content_min: |
    Apply domain best practices for: %[3]s
  content: |
    ## Domain-Specific Knowledge

    ### Key Concepts
    [Add specific concepts, patterns, or frameworks relevant to this domain]

    ### Common Pitfalls
    [Add known issues, gotchas, or anti-patterns to avoid]

    ### Best Practices
    [Add domain-specific best practices and guidelines]

    ### Resources
    Research Topics: %[3]s

    [Add additional references, documentation links, or learning resources]
`,
		agentName, // 1: stable id prefix
		role,      // 2: domain/role
		topics,    // 3: topics
	)

	// Write the template
	if err := os.WriteFile(promptsPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write prompts.yaml: %w", err)
	}

	fmt.Printf("[AgentWizard] ✓ Generated prompts.yaml template at %s\n", promptsPath)
	return nil
}
