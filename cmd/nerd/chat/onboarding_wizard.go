package chat

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/ux"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// ONBOARDING WIZARD
// =============================================================================
// The onboarding wizard provides a guided first-run experience for new users.
// It follows a 5-phase flow: Welcome → Experience → API Check → Wow → Complete
//
// Design principles:
// - Skip-able at any step (type "skip" or press Ctrl+C)
// - Non-blocking: doesn't prevent CLI usage
// - Respects existing users: auto-skips if .nerd/ directory exists

// startOnboarding initializes the onboarding wizard.
func (m Model) startOnboarding() (tea.Model, tea.Cmd) {
	m.awaitingOnboarding = true
	m.inputMode = InputModeOnboarding
	m.onboardingWizard = &OnboardingWizardState{
		Step: OnboardingStepWelcome,
	}

	welcomeMsg := `
# Welcome to codeNERD

**The Logic-First CLI Coding Agent**

codeNERD combines the creative power of LLMs with deterministic logic reasoning.
Unlike traditional AI assistants, codeNERD:

- **Remembers** across sessions (no more re-explaining your codebase)
- **Reasons** about code structure through a logic kernel
- **Learns** from your corrections and preferences
- **Explains** why it makes decisions (type /why anytime)

Let's get you set up in under 2 minutes.

**How would you describe your experience level?**
1. **Beginner** - New to AI coding tools
2. **Intermediate** - Used tools like Copilot or ChatGPT for coding
3. **Advanced** - Experienced with AI agents, familiar with prompting
4. **Expert** - I want minimal guidance, show me the power features

Type a number (1-4) or "skip" to use defaults:
`

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: welcomeMsg,
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Enter 1-4 or skip..."

	return m, nil
}

// handleOnboardingInput processes input during the onboarding wizard.
func (m Model) handleOnboardingInput(input string) (tea.Model, tea.Cmd) {
	if m.onboardingWizard == nil {
		m.onboardingWizard = &OnboardingWizardState{}
	}

	// Handle skip at any step
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "skip" || trimmed == "s" {
		return m.skipOnboarding()
	}

	switch m.onboardingWizard.Step {
	case OnboardingStepWelcome, OnboardingStepExperience:
		return m.handleExperienceSelection(trimmed)

	case OnboardingStepAPICheck:
		return m.handleAPICheckResponse(trimmed)

	case OnboardingStepWow:
		return m.handleWowResponse(trimmed)

	case OnboardingStepComplete:
		return m.completeOnboarding()
	}

	return m, nil
}

// handleExperienceSelection processes the experience level selection.
func (m Model) handleExperienceSelection(input string) (tea.Model, tea.Cmd) {
	var level string
	var levelName string

	switch input {
	case "1", "beginner":
		level = string(config.ExperienceBeginner)
		levelName = "Beginner"
	case "2", "intermediate":
		level = string(config.ExperienceIntermediate)
		levelName = "Intermediate"
	case "3", "advanced":
		level = string(config.ExperienceAdvanced)
		levelName = "Advanced"
	case "4", "expert":
		level = string(config.ExperienceExpert)
		levelName = "Expert"
	default:
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Please enter a number (1-4) or type \"skip\" to use defaults.",
			Time:    time.Now(),
		})
		return m, nil
	}

	m.onboardingWizard.ExperienceLevel = level
	m.onboardingWizard.Step = OnboardingStepAPICheck

	// Check if API is already configured
	if m.client != nil {
		providerCfg, _ := detectCurrentProvider()
		if providerCfg != "" {
			m.onboardingWizard.APIConfigured = true
			m.onboardingWizard.Step = OnboardingStepWow
			return m.showWowMoment(levelName)
		}
	}

	apiMsg := fmt.Sprintf(`
Great choice, %s! I'll tailor my guidance accordingly.

## API Configuration

I need an LLM backend to work. Choose one:

1. **API Key** - Use OpenAI, Anthropic, Google, or other providers
2. **Claude CLI** - Use Anthropic's Claude Code CLI (requires installation)
3. **Codex CLI** - Use OpenAI's Codex CLI (requires installation)

Type a number (1-3) or "skip" to configure later:
`, levelName)

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: apiMsg,
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Enter 1-3 or skip..."

	return m, nil
}

// handleAPICheckResponse processes the API configuration choice.
func (m Model) handleAPICheckResponse(input string) (tea.Model, tea.Cmd) {
	switch input {
	case "1", "api", "api key":
		// Launch config wizard for API setup
		m.awaitingOnboarding = false
		m.inputMode = InputModeConfigWizard
		m.awaitingConfigWizard = true
		m.configWizard = &ConfigWizardState{
			Step:   StepEngine,
			Engine: "api",
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Let's set up your API key. Starting configuration wizard...\n\n" + getAPIProviderPrompt(),
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Enter provider name..."
		return m, nil

	case "2", "claude", "claude cli":
		m.awaitingOnboarding = false
		m.inputMode = InputModeConfigWizard
		m.awaitingConfigWizard = true
		m.configWizard = &ConfigWizardState{
			Step:   StepClaudeCLIConfig,
			Engine: "claude-cli",
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Setting up Claude CLI integration...\n\nWhich model tier? (sonnet/opus/haiku)",
			Time:    time.Now(),
		})
		return m, nil

	case "3", "codex", "codex cli":
		m.awaitingOnboarding = false
		m.inputMode = InputModeConfigWizard
		m.awaitingConfigWizard = true
		m.configWizard = &ConfigWizardState{
			Step:   StepCodexCLIConfig,
			Engine: "codex-cli",
		}
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Setting up Codex CLI integration...\n\nWhich model? (gpt-5/o4-mini/codex-mini-latest)",
			Time:    time.Now(),
		})
		return m, nil

	case "skip", "s", "later":
		m.onboardingWizard.Step = OnboardingStepWow
		return m.showWowMoment("")

	default:
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Please enter a number (1-3) or type \"skip\" to configure later.",
			Time:    time.Now(),
		})
		return m, nil
	}
}

// showWowMoment demonstrates codeNERD's unique capabilities.
func (m Model) showWowMoment(levelName string) (tea.Model, tea.Cmd) {
	m.onboardingWizard.Step = OnboardingStepWow
	m.onboardingWizard.ShowedWow = true

	// Personalize based on level if provided
	intro := ""
	if levelName != "" {
		intro = fmt.Sprintf("As a **%s** user, you'll appreciate how codeNERD works under the hood:\n\n", levelName)
	}

	wowMsg := intro + `
## What Makes codeNERD Different

**Logic Kernel** - A Mangle (Datalog) engine tracks facts about your code:
` + "```" + `
/query file_topology  # See all files
/query symbol_graph   # See functions/classes
/why next_action      # See reasoning chain
` + "```" + `

**Specialized Shards** - Purpose-built agents for different tasks:
` + "```" + `
/review       # Security + code review
/test         # Run and analyze tests
/research X   # Deep-dive into topic X
` + "```" + `

**Memory That Persists** - Your corrections become learned preferences:
` + "```" + `
/learn "always use tabs"     # Record a preference
/dream "what if we refactor" # Explore hypotheticals
` + "```" + `

**Press Enter to complete setup**, or type any command to jump right in!
`

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: wowMsg,
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Press Enter or type a command..."

	return m, nil
}

// handleWowResponse processes the response after showing the wow moment.
func (m Model) handleWowResponse(input string) (tea.Model, tea.Cmd) {
	if input != "" {
		logging.Boot("User completed wow moment with input: %q", input)
	}
	// Any input completes onboarding - if it's a command, we'll process it
	return m.completeOnboarding()
}

// skipOnboarding skips the remaining onboarding steps.
func (m Model) skipOnboarding() (tea.Model, tea.Cmd) {
	m.onboardingWizard.SkipRequested = true
	m.awaitingOnboarding = false
	m.inputMode = InputModeNormal

	// Save skip state to preferences
	pm := ux.NewPreferencesManager(m.workspace)
	if err := pm.Load(); err == nil {
		_ = pm.SkipOnboarding()
		_ = pm.Save()
	}

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: "Onboarding skipped. Type `/help` anytime to see available commands.\n\nReady when you are!",
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Enter a command or chat..."

	return m, func() tea.Msg {
		return onboardingCompleteMsg{
			Skipped: true,
		}
	}
}

// completeOnboarding finalizes the onboarding wizard.
func (m Model) completeOnboarding() (tea.Model, tea.Cmd) {
	m.awaitingOnboarding = false
	m.inputMode = InputModeNormal

	// Save preferences
	pm := ux.NewPreferencesManager(m.workspace)
	if err := pm.Load(); err == nil {
		_ = pm.MarkOnboardingComplete()

		// Set experience level if selected
		if m.onboardingWizard != nil && m.onboardingWizard.ExperienceLevel != "" {
			switch m.onboardingWizard.ExperienceLevel {
			case string(config.ExperienceBeginner):
				_ = pm.SetGuidanceLevel(config.GuidanceVerbose)
			case string(config.ExperienceIntermediate):
				_ = pm.SetGuidanceLevel(config.GuidanceNormal)
			case string(config.ExperienceAdvanced), string(config.ExperienceExpert):
				_ = pm.SetGuidanceLevel(config.GuidanceMinimal)
			}
		}
		_ = pm.Save()
	}

	completeMsg := `
## Setup Complete!

You're all set. Here are the essentials:

| Command | What it does |
|---------|-------------|
| **/scan** | Index your codebase |
| **/review** | Get a code review |
| **/test** | Run and analyze tests |
| **/help** | See all commands |
| **/why** | Understand my reasoning |

Just type naturally - I'll figure out the rest. Happy coding!
`

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: completeMsg,
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Enter a command or chat..."

	expLevel := ""
	if m.onboardingWizard != nil {
		expLevel = m.onboardingWizard.ExperienceLevel
	}

	return m, func() tea.Msg {
		return onboardingCompleteMsg{
			ExperienceLevel: expLevel,
			Skipped:         false,
		}
	}
}

// detectCurrentProvider returns the currently configured provider name.
func detectCurrentProvider() (string, error) {
	cfg, err := config.GlobalConfig()
	if err != nil {
		return "", err
	}
	if cfg.Provider != "" {
		return cfg.Provider, nil
	}
	return "", nil
}

// getAPIProviderPrompt returns the prompt for API provider selection.
func getAPIProviderPrompt() string {
	return `Which LLM provider would you like to use?

1. **anthropic** - Claude models (recommended)
2. **openai** - GPT-4, GPT-4o
3. **gemini** - Google's Gemini models
4. **xai** - Grok models
5. **zai** - Z.AI models
6. **openrouter** - Multiple providers via OpenRouter

Enter provider name or number:`
}

// checkFirstRun checks if this is a first run and triggers onboarding if needed.
func checkFirstRun(workspace string) tea.Cmd {
	return func() tea.Msg {
		isFirst := ux.ShouldShowOnboarding(workspace)
		return onboardingCheckMsg{
			IsFirstRun: isFirst,
			Workspace:  workspace,
		}
	}
}
