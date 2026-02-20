package chat

import (
	"codenerd/internal/auth/antigravity"
	internalconfig "codenerd/internal/config"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/logging"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// CONFIGURATION WIZARD
// =============================================================================
// Full interactive configuration dialogue for codeNERD settings.
// Covers: LLM provider, API keys, per-shard models, embedding, context window.

// ConfigWizardStep represents the current step in the wizard.
type ConfigWizardStep int

const (
	StepWelcome         ConfigWizardStep = iota
	StepEngine                           // NEW: Select engine (api, claude-cli, codex-cli)
	StepClaudeCLIConfig                  // NEW: Claude CLI model/timeout config
	StepCodexCLIConfig                   // NEW: Codex CLI model/sandbox/timeout config
	StepProvider
	StepAPIKey
	StepAntigravityAccounts // NEW: Show existing accounts and manage them
	StepAntigravityAddMore  // NEW: Add more accounts loop
	StepAntigravityWaiting  // NEW: Waiting for OAuth callback
	StepModel
	StepShardConfig
	StepShardModel
	StepShardTemperature
	StepShardContext
	StepNextShard
	StepEmbeddingProvider
	StepEmbeddingConfig
	StepContextWindow
	StepContextBudget
	StepCoreLimits
	StepReview
	StepComplete
)

// ConfigWizardState tracks the state of the configuration wizard.
type ConfigWizardState struct {
	Step ConfigWizardStep

	// Engine configuration (api, claude-cli, codex-cli)
	Engine string

	// Claude CLI configuration (when Engine="claude-cli")
	ClaudeCLIModel   string // sonnet, opus, haiku
	ClaudeCLITimeout int    // seconds

	// Codex CLI configuration (when Engine="codex-cli")
	CodexCLIModel   string // gpt-5, o4-mini, codex-mini-latest
	CodexCLISandbox string // read-only, workspace-write
	CodexCLITimeout int    // seconds

	// Provider configuration (when Engine="api")
	Provider string // zai, anthropic, openai, gemini, xai
	APIKey   string
	Model    string

	// Antigravity multi-account state
	AntigravityAccounts  []*antigravity.Account      // Cached list of accounts
	AntigravityAuthState *antigravity.AuthFlowResult // Current OAuth flow state

	// Per-shard configuration
	CurrentShard    string
	ShardIndex      int
	ShardProfiles   map[string]*ShardProfileConfig
	ConfigureShards bool // Whether user wants to configure individual shards

	// Embedding configuration
	EmbeddingProvider string // ollama, genai
	OllamaEndpoint    string
	OllamaModel       string
	GenAIAPIKey       string
	GenAIModel        string

	// Context window configuration
	MaxTokens             int
	CoreReservePercent    int
	AtomReservePercent    int
	WorkingReservePercent int
	RecentTurnWindow      int

	// Core limits
	MaxConcurrentShards int
	MaxFactsInKernel    int
	MaxMemoryMB         int
}

// ShardProfileConfig holds per-shard configuration.
type ShardProfileConfig struct {
	Model            string
	Temperature      float64
	MaxContextTokens int
	MaxOutputTokens  int
	EnableLearning   bool
}

// intentTypes lists the primary intent types for the configuration wizard UI.
// Used for iterating through persona-specific settings in the wizard flow.
// Runtime config is handled by ConfigFactory; this is purely for wizard UX.
var intentTypes = []string{"coder", "tester", "reviewer", "researcher"}

// ProviderModels maps providers to their available models.
var ProviderModels = map[string][]string{
	"zai":       {"glm-4.6", "glm-4", "glm-4-air"},
	"anthropic": {"claude-sonnet-4-5-20250514", "claude-opus-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"},
	"openai":    {"gpt-5.1-codex-max", "gpt-5.1-codex-mini", "gpt-5-codex", "gpt-4o", "gpt-4o-mini"},
	"gemini":    {"gemini-3-flash-preview", "gemini-3-pro-preview"},
	"antigravity": {
		// Gemini 3 Flash variants (supports minimal/low/medium/high thinking levels)
		"gemini-3-flash",
		"gemini-3-flash-low",
		"gemini-3-flash-medium",
		"gemini-3-flash-high",
		// Gemini 3 Pro variants (supports low/high thinking levels only)
		"gemini-3-pro-low",
		"gemini-3-pro-high",
		// Claude via Antigravity (with thinking support)
		"claude-sonnet-4-5-thinking",
		"claude-opus-4-5-thinking",
	},
	"xai": {"grok-4-1-fast-reasoning", "grok-2-latest", "grok-2", "grok-beta"},
	"openrouter": {
		// Anthropic via OpenRouter
		"anthropic/claude-3.5-sonnet",
		"anthropic/claude-3.5-haiku",
		"anthropic/claude-3-opus",
		// OpenAI via OpenRouter
		"openai/gpt-4o",
		"openai/gpt-4o-mini",
		"openai/o1-preview",
		"openai/o1-mini",
		// Google via OpenRouter
		"google/gemini-3-flash-preview",
		"google/gemini-3-pro-preview",
		// Meta Llama
		"meta-llama/llama-3.1-405b-instruct",
		"meta-llama/llama-3.1-70b-instruct",
		// Mistral
		"mistralai/mistral-large",
		"mistralai/codestral-latest",
		// DeepSeek
		"deepseek/deepseek-chat",
		"deepseek/deepseek-coder",
		// Qwen
		"qwen/qwen-2.5-72b-instruct",
		"qwen/qwen-2.5-coder-32b-instruct",
	},
}

// DefaultProviderModel returns the default model for a provider.
func DefaultProviderModel(provider string) string {
	models, ok := ProviderModels[provider]
	if ok && len(models) > 0 {
		return models[0]
	}
	return ""
}

// NewConfigWizard creates a new configuration wizard state.
func NewConfigWizard() *ConfigWizardState {
	return &ConfigWizardState{
		Step:          StepWelcome,
		ShardProfiles: make(map[string]*ShardProfileConfig),
		// Engine defaults
		Engine:           "api", // Default to HTTP API mode
		ClaudeCLIModel:   "sonnet",
		ClaudeCLITimeout: 300,
		CodexCLIModel:    "gpt-5.1-codex-max",
		CodexCLISandbox:  "read-only",
		CodexCLITimeout:  300,
		// Embedding defaults
		OllamaEndpoint:        "http://localhost:11434",
		OllamaModel:           "embeddinggemma",
		GenAIModel:            "gemini-embedding-001",
		MaxTokens:             128000,
		CoreReservePercent:    5,
		AtomReservePercent:    30,
		WorkingReservePercent: 50,
		RecentTurnWindow:      5,
		MaxConcurrentShards:   4,
		MaxFactsInKernel:      100000,
		MaxMemoryMB:           2048,
	}
}

// handleConfigWizardInput processes input during the configuration wizard.
func (m Model) handleConfigWizardInput(input string) (tea.Model, tea.Cmd) {
	if m.configWizard == nil {
		m.configWizard = NewConfigWizard()
	}

	input = strings.TrimSpace(input)

	switch m.configWizard.Step {
	case StepWelcome:
		return m.configWizardWelcome(input)
	case StepEngine:
		return m.configWizardEngine(input)
	case StepClaudeCLIConfig:
		return m.configWizardClaudeCLI(input)
	case StepCodexCLIConfig:
		return m.configWizardCodexCLI(input)
	case StepProvider:
		return m.configWizardProvider(input)
	case StepAPIKey:
		return m.configWizardAPIKey(input)
	case StepAntigravityAccounts:
		return m.configWizardAntigravityAccounts(input)
	case StepAntigravityAddMore:
		return m.configWizardAntigravityAddMore(input)
	case StepAntigravityWaiting:
		return m.configWizardAntigravityWaiting(input)
	case StepModel:
		return m.configWizardModel(input)
	case StepShardConfig:
		return m.configWizardShardConfig(input)
	case StepShardModel:
		return m.configWizardShardModel(input)
	case StepShardTemperature:
		return m.configWizardShardTemperature(input)
	case StepShardContext:
		return m.configWizardShardContext(input)
	case StepNextShard:
		return m.configWizardNextShard(input)
	case StepEmbeddingProvider:
		return m.configWizardEmbeddingProvider(input)
	case StepEmbeddingConfig:
		return m.configWizardEmbeddingConfig(input)
	case StepContextWindow:
		return m.configWizardContextWindow(input)
	case StepCoreLimits:
		return m.configWizardCoreLimits(input)
	case StepReview:
		return m.configWizardReview(input)
	}

	return m, nil
}

// configWizardWelcome handles the welcome step.
func (m Model) configWizardWelcome(input string) (tea.Model, tea.Cmd) {
	if input != "" {
		logging.SessionDebug("configWizardWelcome: input=%q", input)
	}
	// User pressed enter to start, move to engine selection
	m.configWizard.Step = StepEngine
	m = m.addMessage(Message{
		Role: "assistant",
		Content: `## Step 1: LLM Engine

How would you like to connect to the LLM?

| # | Engine | Description |
|---|--------|-------------|
| 1 | api | HTTP API with API key (pay-per-token) |
| 2 | claude-cli | Claude Code CLI (Claude Pro/Max subscription) |
| 3 | codex-cli | OpenAI Codex CLI (ChatGPT Plus/Pro subscription) |

**Recommendation:**
- Use **api** if you have API credits and want fine-grained control
- Use **claude-cli** if you have Claude Pro/Max subscription
- Use **codex-cli** if you have ChatGPT Plus/Pro subscription

Enter a number (1-3) or engine name:`,
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Enter engine (1-3 or name)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardEngine handles engine selection.
func (m Model) configWizardEngine(input string) (tea.Model, tea.Cmd) {
	engines := map[string]string{
		"1": "api", "api": "api",
		"2": "claude-cli", "claude-cli": "claude-cli", "claude": "claude-cli",
		"3": "codex-cli", "codex-cli": "codex-cli", "codex": "codex-cli",
	}

	engine, ok := engines[strings.ToLower(input)]
	if !ok {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Invalid selection. Please enter 1-3 or an engine name (api, claude-cli, codex-cli):",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	m.configWizard.Engine = engine

	switch engine {
	case "claude-cli":
		m.configWizard.Step = StepClaudeCLIConfig
		m = m.addMessage(Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## Step 2: Claude Code CLI Configuration

You selected **Claude Code CLI** engine.

### Model Selection

| # | Model | Subscription |
|---|-------|--------------|
| 1 | sonnet | Claude Pro, Max (default) |
| 2 | opus | Claude Max only |
| 3 | haiku | Claude Pro, Max (fast) |

Current: **%s**

Enter model number/name (Enter for default):`, m.configWizard.ClaudeCLIModel),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Claude CLI model (Enter for sonnet)..."

	case "codex-cli":
		m.configWizard.Step = StepCodexCLIConfig
		m = m.addMessage(Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## Step 2: Codex CLI Configuration

You selected **Codex CLI** engine.

### Model Selection

| # | Model | Description |
|---|-------|-------------|
| 1 | gpt-5.1-codex-max | **Recommended** - Best for agentic coding |
| 2 | gpt-5.1-codex-mini | Cost-effective, faster |
| 3 | gpt-5.1 | General coding and reasoning |
| 4 | gpt-5-codex | Legacy agentic model |
| 5 | gpt-5 | Legacy general model |
| 6 | o4-mini | Fast reasoning (legacy) |
| 7 | codex-mini-latest | Low-latency code Q&A |

Current: **%s**

Enter model number/name (Enter for default):`, m.configWizard.CodexCLIModel),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Codex CLI model (Enter for gpt-5.1-codex-max)..."

	default: // "api"
		m.configWizard.Step = StepProvider
		m = m.addMessage(Message{
			Role: "assistant",
			Content: `## Step 2: LLM Provider

Which LLM provider would you like to use?

| # | Provider | Description |
|---|----------|-------------|
| 1 | zai | Z.AI GLM-4.6 (default) |
| 2 | anthropic | Anthropic Claude |
| 3 | openai | OpenAI GPT/Codex |
| 4 | gemini | Google Gemini |
| 5 | antigravity | Google Cloud Code (Internal) |
| 6 | xai | xAI Grok |
| 7 | openrouter | OpenRouter (multi-provider gateway) |
344: 
345: Enter a number (1-7) or provider name:`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Enter provider (1-6 or name)..."
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardClaudeCLI handles Claude CLI configuration.
func (m Model) configWizardClaudeCLI(input string) (tea.Model, tea.Cmd) {
	claudeModels := map[string]string{
		"1": "sonnet", "sonnet": "sonnet",
		"2": "opus", "opus": "opus",
		"3": "haiku", "haiku": "haiku",
	}

	if input != "" {
		if model, ok := claudeModels[strings.ToLower(input)]; ok {
			m.configWizard.ClaudeCLIModel = model
		} else {
			// Allow custom model names
			m.configWizard.ClaudeCLIModel = input
		}
	}

	// Skip to shard configuration (no API key needed for CLI)
	m.configWizard.Step = StepShardConfig
	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`## Step 3: Per-Shard Configuration

Claude CLI model: **%s**

Would you like to configure individual shard settings?
(model, temperature, context limits per shard type)

| Shard | Purpose |
|-------|---------|
| coder | Code generation, edits |
| tester | Test creation, execution |
| reviewer | Code review, analysis |
| researcher | Knowledge gathering |

**y** = Configure each shard
**n** = Use defaults for all shards (recommended for quick setup)`, m.configWizard.ClaudeCLIModel),
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Configure shards? (y/n)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardCodexCLI handles Codex CLI configuration.
func (m Model) configWizardCodexCLI(input string) (tea.Model, tea.Cmd) {
	codexModels := map[string]string{
		"1": "gpt-5.1-codex-max", "gpt-5.1-codex-max": "gpt-5.1-codex-max",
		"2": "gpt-5.1-codex-mini", "gpt-5.1-codex-mini": "gpt-5.1-codex-mini",
		"3": "gpt-5.1", "gpt-5.1": "gpt-5.1",
		"4": "gpt-5-codex", "gpt-5-codex": "gpt-5-codex",
		"5": "gpt-5", "gpt-5": "gpt-5",
		"6": "o4-mini", "o4-mini": "o4-mini",
		"7": "codex-mini-latest", "codex-mini-latest": "codex-mini-latest",
	}

	if input != "" {
		if model, ok := codexModels[strings.ToLower(input)]; ok {
			m.configWizard.CodexCLIModel = model
		} else {
			// Allow custom model names
			m.configWizard.CodexCLIModel = input
		}
	}

	// Skip to shard configuration (no API key needed for CLI)
	m.configWizard.Step = StepShardConfig
	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`## Step 3: Per-Shard Configuration

Codex CLI model: **%s**

Would you like to configure individual shard settings?
(model, temperature, context limits per shard type)

| Shard | Purpose |
|-------|---------|
| coder | Code generation, edits |
| tester | Test creation, execution |
| reviewer | Code review, analysis |
| researcher | Knowledge gathering |

**y** = Configure each shard
**n** = Use defaults for all shards (recommended for quick setup)`, m.configWizard.CodexCLIModel),
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Configure shards? (y/n)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardProvider handles provider selection.
func (m Model) configWizardProvider(input string) (tea.Model, tea.Cmd) {
	providers := map[string]string{
		"1": "zai", "zai": "zai",
		"2": "anthropic", "anthropic": "anthropic",
		"3": "openai", "openai": "openai",
		"4": "gemini", "gemini": "gemini",
		"5": "antigravity", "antigravity": "antigravity",
		"6": "xai", "xai": "xai",
		"7": "openrouter", "openrouter": "openrouter",
	}

	provider, ok := providers[strings.ToLower(input)]
	if !ok {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Invalid selection. Please enter 1-7 or a provider name (zai, anthropic, openai, gemini, antigravity, xai, openrouter):",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	m.configWizard.Provider = provider

	// Route antigravity to special OAuth flow (no API key)
	if provider == "antigravity" {
		return m.showAntigravityAccountsPrompt()
	}

	m.configWizard.Step = StepAPIKey

	envVar := map[string]string{
		"zai":        "ZAI_API_KEY",
		"anthropic":  "ANTHROPIC_API_KEY",
		"openai":     "OPENAI_API_KEY",
		"gemini":     "GEMINI_API_KEY",
		"xai":        "XAI_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}[provider]

	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`## Step 2: API Key

You selected **%s**.

Enter your API key for %s:
(Or set the %s environment variable and press Enter to skip)`, provider, provider, envVar),
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Enter API key (or Enter to use env var)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// =============================================================================
// ANTIGRAVITY MULTI-ACCOUNT WIZARD
// =============================================================================

// showAntigravityAccountsPrompt shows existing accounts and options.
func (m Model) showAntigravityAccountsPrompt() (tea.Model, tea.Cmd) {
	m.configWizard.Step = StepAntigravityAccounts

	// Load existing accounts
	store, err := antigravity.NewAccountStore()
	if err != nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("**Error loading accounts:** %v\n\nPress Enter to continue with OAuth flow.", err),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	accounts := store.ListAccounts()
	m.configWizard.AntigravityAccounts = accounts

	var sb strings.Builder
	sb.WriteString(`## Step 2: Google Antigravity Accounts

**Antigravity** uses Google OAuth for authentication (no API key needed).
You can add **multiple Google accounts** for load balancing and rate limit rotation.

`)

	if len(accounts) > 0 {
		sb.WriteString("### Existing Accounts\n\n")
		sb.WriteString("| # | Email | Health | Project | Status |\n")
		sb.WriteString("|---|-------|--------|---------|--------|\n")
		for i, acc := range accounts {
			health := store.GetEffectiveScore(acc)
			status := "healthy"
			if health < 30 {
				status = "exhausted"
			} else if health < 50 {
				status = "degraded"
			}
			projectID := acc.ProjectID
			if len(projectID) > 12 {
				projectID = projectID[:12] + "..."
			}
			sb.WriteString(fmt.Sprintf("| %d | %s | %d/100 | %s | %s |\n",
				i+1, acc.Email, health, projectID, status))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("*No accounts configured yet.*\n\n")
	}

	sb.WriteString(`### Benefits of Multiple Accounts
- **Rate limit rotation**: When one account hits 429, automatically switch to another
- **Higher throughput**: Distribute requests across accounts
- **Fault tolerance**: Continue working if one account has issues

### Options
- **a** = Add a new Google account (opens browser for OAuth)
- **d** = Done, proceed with current accounts
`)

	if len(accounts) > 0 {
		sb.WriteString("- **r <#>** = Remove account by number\n")
	}

	sb.WriteString("\nEnter your choice:")

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Enter choice (a=add, d=done)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardAntigravityAccounts handles account management choices.
func (m Model) configWizardAntigravityAccounts(input string) (tea.Model, tea.Cmd) {
	input = strings.ToLower(strings.TrimSpace(input))

	switch {
	case input == "a" || input == "add":
		// Start OAuth flow
		return m.startAntigravityOAuth()

	case input == "d" || input == "done" || input == "":
		// Check if we have at least one account
		if len(m.configWizard.AntigravityAccounts) == 0 {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "**Warning:** No accounts configured. You need at least one account to use Antigravity.\n\nEnter **a** to add an account, or **q** to go back and choose a different provider.",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		// Proceed to model selection
		return m.showAntigravityModelPrompt()

	case strings.HasPrefix(input, "r ") || strings.HasPrefix(input, "remove "):
		// Remove account by number
		parts := strings.Fields(input)
		if len(parts) < 2 {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: "Please specify account number to remove, e.g., `r 1`",
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		num, err := strconv.Atoi(parts[1])
		if err != nil || num < 1 || num > len(m.configWizard.AntigravityAccounts) {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Invalid account number. Enter 1-%d.", len(m.configWizard.AntigravityAccounts)),
				Time:    time.Now(),
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		// Remove the account
		store, _ := antigravity.NewAccountStore()
		email := m.configWizard.AntigravityAccounts[num-1].Email
		if err := store.DeleteAccount(email); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Error removing account:** %v", err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Removed account: **%s**", email),
				Time:    time.Now(),
			})
		}
		// Refresh the account list
		return m.showAntigravityAccountsPrompt()

	case input == "q" || input == "back":
		// Go back to provider selection
		m.configWizard.Step = StepProvider
		m = m.addMessage(Message{
			Role: "assistant",
			Content: `## Step 2: LLM Provider

Which LLM provider would you like to use?

| # | Provider | Description |
|---|----------|-------------|
| 1 | zai | Z.AI GLM-4.6 (default) |
| 2 | anthropic | Anthropic Claude |
| 3 | openai | OpenAI GPT/Codex |
| 4 | gemini | Google Gemini |
| 5 | antigravity | Google Cloud Code (Internal) |
| 6 | xai | xAI Grok |
| 7 | openrouter | OpenRouter (multi-provider gateway) |

Enter a number (1-7) or provider name:`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Enter provider (1-7 or name)..."
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	default:
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Invalid choice. Enter **a** to add account, **d** when done, or **r <#>** to remove.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}
}

// startAntigravityOAuth initiates the OAuth flow.
func (m Model) startAntigravityOAuth() (tea.Model, tea.Cmd) {
	// Start OAuth flow
	authResult, err := antigravity.StartAuth()
	if err != nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("**Error starting OAuth:** %v\n\nPress Enter to try again.", err),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	m.configWizard.AntigravityAuthState = authResult
	m.configWizard.Step = StepAntigravityWaiting

	// Open browser
	openBrowserURL(authResult.AuthURL)

	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`## Adding Google Account

Opening browser for Google OAuth...

If the browser doesn't open automatically, visit:
%s

**Waiting for authentication...**

After signing in, the wizard will continue automatically.
Press **c** to cancel and go back.`, authResult.AuthURL),
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Waiting for OAuth... (c=cancel)"
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	// Return a command that will wait for the OAuth callback
	return m, waitForAntigravityOAuth(authResult)
}

// waitForAntigravityOAuth returns a command that waits for OAuth callback.
func waitForAntigravityOAuth(authResult *antigravity.AuthFlowResult) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		code, err := antigravity.WaitForCallback(ctx, authResult.State)
		if err != nil {
			return antigravityOAuthResultMsg{err: err}
		}

		// Exchange code for tokens
		tm, err := antigravity.NewTokenManager()
		if err != nil {
			return antigravityOAuthResultMsg{err: err}
		}

		token, err := tm.ExchangeCode(ctx, code, authResult.Verifier)
		if err != nil {
			return antigravityOAuthResultMsg{err: err}
		}

		// Create account
		account := &antigravity.Account{
			Email:        token.Email,
			RefreshToken: token.RefreshToken,
			AccessToken:  token.AccessToken,
			AccessExpiry: token.Expiry,
			ProjectID:    token.ProjectID,
		}

		// Resolve project ID if not set
		if account.ProjectID == "" {
			resolver := antigravity.NewProjectResolver(token.AccessToken)
			if pid, err := resolver.ResolveProjectID(); err == nil {
				account.ProjectID = pid
			}
		}

		// Add to store
		store, err := antigravity.NewAccountStore()
		if err != nil {
			return antigravityOAuthResultMsg{err: err}
		}

		if err := store.AddAccount(account); err != nil {
			return antigravityOAuthResultMsg{err: err}
		}

		return antigravityOAuthResultMsg{account: account}
	}
}

// antigravityOAuthResultMsg is the message returned when OAuth completes.
type antigravityOAuthResultMsg struct {
	account *antigravity.Account
	err     error
}

// configWizardAntigravityWaiting handles input while waiting for OAuth.
func (m Model) configWizardAntigravityWaiting(input string) (tea.Model, tea.Cmd) {
	if strings.ToLower(strings.TrimSpace(input)) == "c" {
		// Cancel and go back
		m.configWizard.AntigravityAuthState = nil
		return m.showAntigravityAccountsPrompt()
	}
	// Ignore other input, still waiting
	return m, nil
}

// configWizardAntigravityAddMore handles the "add more accounts" prompt.
func (m Model) configWizardAntigravityAddMore(input string) (tea.Model, tea.Cmd) {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "y" || input == "yes" {
		return m.startAntigravityOAuth()
	}
	// Proceed to model selection
	return m.showAntigravityModelPrompt()
}

// showAntigravityModelPrompt shows model selection for Antigravity.
func (m Model) showAntigravityModelPrompt() (tea.Model, tea.Cmd) {
	m.configWizard.Step = StepModel

	models := ProviderModels["antigravity"]
	var sb strings.Builder
	sb.WriteString("## Step 3: Model Selection\n\nAvailable models for **antigravity**:\n\n")
	sb.WriteString("| # | Model | Description |\n")
	sb.WriteString("|---|-------|-------------|\n")
	for i, model := range models {
		defaultMark := ""
		if i == 0 {
			defaultMark = " (default)"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s |\n", i+1, model, defaultMark))
	}
	sb.WriteString("\nEnter a number or model name (Enter for default):")

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Enter model (or Enter for default)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// openBrowserURL opens a URL in the default browser.
func openBrowserURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// configWizardAPIKey handles API key input.
func (m Model) configWizardAPIKey(input string) (tea.Model, tea.Cmd) {
	if input != "" {
		m.configWizard.APIKey = input
	}
	// If empty, will rely on environment variable

	m.configWizard.Step = StepModel

	// Show available models for the selected provider
	models := ProviderModels[m.configWizard.Provider]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Step 3: Model Selection\n\nAvailable models for **%s**:\n\n", m.configWizard.Provider))
	sb.WriteString("| # | Model | Description |\n")
	sb.WriteString("|---|-------|-------------|\n")
	for i, model := range models {
		defaultMark := ""
		if i == 0 {
			defaultMark = " (default)"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s |\n", i+1, model, defaultMark))
	}
	sb.WriteString("\nEnter a number or model name (Enter for default):")

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Enter model (or Enter for default)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardModel handles model selection.
func (m Model) configWizardModel(input string) (tea.Model, tea.Cmd) {
	models := ProviderModels[m.configWizard.Provider]

	if input == "" {
		// Use default
		m.configWizard.Model = DefaultProviderModel(m.configWizard.Provider)
	} else if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(models) {
		m.configWizard.Model = models[num-1]
	} else {
		// Try to match model name directly
		found := false
		for _, model := range models {
			if strings.EqualFold(model, input) {
				m.configWizard.Model = model
				found = true
				break
			}
		}
		if !found {
			m.configWizard.Model = input // Allow custom model names
		}
	}

	m.configWizard.Step = StepShardConfig

	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`## Step 4: Per-Shard Configuration

Selected model: **%s**

Would you like to configure individual shard settings?
(model, temperature, context limits per shard type)

| Shard | Purpose |
|-------|---------|
| coder | Code generation, edits |
| tester | Test creation, execution |
| reviewer | Code review, analysis |
| researcher | Knowledge gathering |

**y** = Configure each shard
**n** = Use defaults for all shards (recommended for quick setup)`, m.configWizard.Model),
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Configure shards? (y/n)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardShardConfig handles the decision to configure shards.
func (m Model) configWizardShardConfig(input string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(strings.ToLower(input), "y") {
		m.configWizard.ConfigureShards = true
		m.configWizard.ShardIndex = 0
		m.configWizard.CurrentShard = intentTypes[0]
		m.configWizard.Step = StepShardModel
		return m.showShardModelPrompt()
	}

	// Skip to embedding configuration
	m.configWizard.ConfigureShards = false
	m.configWizard.Step = StepEmbeddingProvider
	return m.showEmbeddingProviderPrompt()
}

// showShardModelPrompt shows the model selection prompt for current shard.
func (m Model) showShardModelPrompt() (tea.Model, tea.Cmd) {
	shard := m.configWizard.CurrentShard
	models := ProviderModels[m.configWizard.Provider]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Configuring: %s shard\n\n", shard))
	sb.WriteString("### Model Selection\n\n")
	sb.WriteString("| # | Model |\n|---|-------|\n")
	for i, model := range models {
		defaultMark := ""
		if model == m.configWizard.Model {
			defaultMark = " (current default)"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s%s |\n", i+1, model, defaultMark))
	}
	sb.WriteString("\nEnter model number/name (Enter to use default):")

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.textarea.Placeholder = fmt.Sprintf("%s model (Enter for default)...", shard)
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardShardModel handles shard model selection.
func (m Model) configWizardShardModel(input string) (tea.Model, tea.Cmd) {
	shard := m.configWizard.CurrentShard
	models := ProviderModels[m.configWizard.Provider]

	// Initialize shard profile if needed
	if m.configWizard.ShardProfiles[shard] == nil {
		m.configWizard.ShardProfiles[shard] = &ShardProfileConfig{
			Model:            m.configWizard.Model, // Default to main model
			Temperature:      0.7,
			MaxContextTokens: 30000,
			MaxOutputTokens:  4000,
			EnableLearning:   true,
		}
	}

	if input == "" {
		// Keep default
	} else if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(models) {
		m.configWizard.ShardProfiles[shard].Model = models[num-1]
	} else {
		m.configWizard.ShardProfiles[shard].Model = input
	}

	m.configWizard.Step = StepShardTemperature

	// Suggest different temperatures for different shards
	defaultTemp := map[string]float64{
		"coder":      0.7,
		"tester":     0.5,
		"reviewer":   0.3,
		"researcher": 0.6,
	}[shard]

	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`### Temperature for %s

Temperature controls randomness in responses:
- **0.0-0.3**: Very focused, deterministic (good for review/analysis)
- **0.4-0.7**: Balanced creativity (good for coding)
- **0.8-1.0**: More creative, varied (good for brainstorming)

Suggested for %s: **%.1f**

Enter temperature (0.0-1.0) or Enter for suggested:`, shard, shard, defaultTemp),
		Time: time.Now(),
	})
	m.textarea.Placeholder = fmt.Sprintf("Temperature (%.1f)...", defaultTemp)
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardShardTemperature handles shard temperature.
func (m Model) configWizardShardTemperature(input string) (tea.Model, tea.Cmd) {
	shard := m.configWizard.CurrentShard

	defaultTemp := map[string]float64{
		"coder":      0.7,
		"tester":     0.5,
		"reviewer":   0.3,
		"researcher": 0.6,
	}[shard]

	if input != "" {
		if temp, err := strconv.ParseFloat(input, 64); err == nil && temp >= 0 && temp <= 1 {
			m.configWizard.ShardProfiles[shard].Temperature = temp
		}
	} else {
		m.configWizard.ShardProfiles[shard].Temperature = defaultTemp
	}

	m.configWizard.Step = StepShardContext

	defaultContext := map[string]int{
		"coder":      30000,
		"tester":     20000,
		"reviewer":   40000,
		"researcher": 25000,
	}[shard]

	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`### Context Tokens for %s

Maximum tokens for input context:
- **20000**: Standard tasks
- **30000**: Complex code generation
- **40000**: Full codebase analysis

Suggested for %s: **%d**

Enter max context tokens or Enter for suggested:`, shard, shard, defaultContext),
		Time: time.Now(),
	})
	m.textarea.Placeholder = fmt.Sprintf("Max context tokens (%d)...", defaultContext)
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardShardContext handles shard context tokens.
func (m Model) configWizardShardContext(input string) (tea.Model, tea.Cmd) {
	shard := m.configWizard.CurrentShard

	defaultContext := map[string]int{
		"coder":      30000,
		"tester":     20000,
		"reviewer":   40000,
		"researcher": 25000,
	}[shard]

	if input != "" {
		if ctx, err := strconv.Atoi(input); err == nil && ctx > 0 {
			m.configWizard.ShardProfiles[shard].MaxContextTokens = ctx
		}
	} else {
		m.configWizard.ShardProfiles[shard].MaxContextTokens = defaultContext
	}

	m.configWizard.Step = StepNextShard
	return m.configWizardNextShard("")
}

// configWizardNextShard moves to the next shard or embedding config.
func (m Model) configWizardNextShard(input string) (tea.Model, tea.Cmd) {
	if input != "" {
		logging.SessionDebug("configWizardNextShard: transition with input=%q", input)
	}
	m.configWizard.ShardIndex++

	if m.configWizard.ShardIndex < len(intentTypes) {
		m.configWizard.CurrentShard = intentTypes[m.configWizard.ShardIndex]
		m.configWizard.Step = StepShardModel
		return m.showShardModelPrompt()
	}

	// All shards configured, move to embedding
	m.configWizard.Step = StepEmbeddingProvider
	return m.showEmbeddingProviderPrompt()
}

// showEmbeddingProviderPrompt shows embedding provider selection.
func (m Model) showEmbeddingProviderPrompt() (tea.Model, tea.Cmd) {
	m = m.addMessage(Message{
		Role: "assistant",
		Content: `## Step 5: Embedding Engine

Embeddings are used for semantic search in the knowledge base.

| # | Provider | Description |
|---|----------|-------------|
| 1 | ollama | Local Ollama server (fast, free, private) |
| 2 | genai | Google GenAI cloud (requires API key) |
| 3 | skip | Skip embedding configuration |

Enter selection (1-3):`,
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Embedding provider (1-3)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardEmbeddingProvider handles embedding provider selection.
func (m Model) configWizardEmbeddingProvider(input string) (tea.Model, tea.Cmd) {
	switch strings.ToLower(input) {
	case "1", "ollama":
		m.configWizard.EmbeddingProvider = "ollama"
		m.configWizard.Step = StepEmbeddingConfig
		m = m.addMessage(Message{
			Role: "assistant",
			Content: fmt.Sprintf(`### Ollama Configuration

Default endpoint: **%s**
Default model: **%s**

Enter Ollama endpoint (or Enter for default):`, m.configWizard.OllamaEndpoint, m.configWizard.OllamaModel),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Ollama endpoint (Enter for default)..."

	case "2", "genai":
		m.configWizard.EmbeddingProvider = "genai"
		m.configWizard.Step = StepEmbeddingConfig
		m = m.addMessage(Message{
			Role: "assistant",
			Content: `### GenAI Configuration

Enter your Google GenAI API key for embeddings:
(Or set GENAI_API_KEY environment variable)`,
			Time: time.Now(),
		})
		m.textarea.Placeholder = "GenAI API key..."

	default:
		// Skip embedding config
		m.configWizard.Step = StepContextWindow
		return m.showContextWindowPrompt()
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardEmbeddingConfig handles embedding config details.
func (m Model) configWizardEmbeddingConfig(input string) (tea.Model, tea.Cmd) {
	switch m.configWizard.EmbeddingProvider {
	case "ollama":
		if input != "" {
			m.configWizard.OllamaEndpoint = input
		}
	case "genai":
		if input != "" {
			m.configWizard.GenAIAPIKey = input
		}
	}

	m.configWizard.Step = StepContextWindow
	return m.showContextWindowPrompt()
}

// showContextWindowPrompt shows context window configuration.
func (m Model) showContextWindowPrompt() (tea.Model, tea.Cmd) {
	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`## Step 6: Context Window

Maximum tokens for the context window.
Larger = more context but slower/more expensive.

Default: **%d** tokens (128K)

Common values:
- 32000 (small, fast)
- 64000 (medium)
- 128000 (large, default)
- 200000 (extra large)

Enter max tokens or Enter for default:`, m.configWizard.MaxTokens),
		Time: time.Now(),
	})
	m.textarea.Placeholder = fmt.Sprintf("Max tokens (%d)...", m.configWizard.MaxTokens)
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardContextWindow handles context window max tokens.
func (m Model) configWizardContextWindow(input string) (tea.Model, tea.Cmd) {
	if input != "" {
		if tokens, err := strconv.Atoi(input); err == nil && tokens > 0 {
			m.configWizard.MaxTokens = tokens
		}
	}

	m.configWizard.Step = StepCoreLimits

	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`## Step 7: Resource Limits

### Max Concurrent Shards
How many shard agents can run in parallel?
Default: **%d**

Enter max concurrent shards or Enter for default:`, m.configWizard.MaxConcurrentShards),
		Time: time.Now(),
	})
	m.textarea.Placeholder = fmt.Sprintf("Max shards (%d)...", m.configWizard.MaxConcurrentShards)
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardCoreLimits handles core limits.
func (m Model) configWizardCoreLimits(input string) (tea.Model, tea.Cmd) {
	if input != "" {
		if shards, err := strconv.Atoi(input); err == nil && shards > 0 {
			m.configWizard.MaxConcurrentShards = shards
		}
	}

	m.configWizard.Step = StepReview
	return m.showConfigReview()
}

// showConfigReview shows the full configuration for review.
func (m Model) showConfigReview() (tea.Model, tea.Cmd) {
	w := m.configWizard
	var sb strings.Builder

	sb.WriteString("## Configuration Review\n\n")

	// Engine configuration
	sb.WriteString("### LLM Engine\n")
	sb.WriteString(fmt.Sprintf("- **Engine**: %s\n", w.Engine))

	switch w.Engine {
	case "claude-cli":
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.ClaudeCLIModel))
		sb.WriteString(fmt.Sprintf("- **Timeout**: %ds\n", w.ClaudeCLITimeout))
		sb.WriteString("- **Auth**: Claude Code CLI (subscription-based)\n")

	case "codex-cli":
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.CodexCLIModel))
		sb.WriteString(fmt.Sprintf("- **Sandbox**: %s\n", w.CodexCLISandbox))
		sb.WriteString(fmt.Sprintf("- **Timeout**: %ds\n", w.CodexCLITimeout))
		sb.WriteString("- **Auth**: Codex CLI (subscription-based)\n")

	default: // "api"
		sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", w.Provider))
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.Model))
		if w.Provider == "antigravity" {
			// Show Antigravity accounts instead of API key
			sb.WriteString(fmt.Sprintf("- **Auth**: Google OAuth (%d account(s))\n", len(w.AntigravityAccounts)))
			if len(w.AntigravityAccounts) > 0 {
				sb.WriteString("- **Accounts**:\n")
				for _, acc := range w.AntigravityAccounts {
					sb.WriteString(fmt.Sprintf("  - %s\n", acc.Email))
				}
			}
		} else if w.APIKey != "" {
			sb.WriteString("- **API Key**: ******* (set)\n")
		} else {
			sb.WriteString("- **API Key**: (using environment variable)\n")
		}
	}

	if w.ConfigureShards && len(w.ShardProfiles) > 0 {
		sb.WriteString("\n### Per-Shard Configuration\n\n")
		sb.WriteString("| Shard | Model | Temp | Context |\n")
		sb.WriteString("|-------|-------|------|-------:|\n")
		for _, shard := range intentTypes {
			if profile, ok := w.ShardProfiles[shard]; ok {
				sb.WriteString(fmt.Sprintf("| %s | %s | %.1f | %d |\n",
					shard, profile.Model, profile.Temperature, profile.MaxContextTokens))
			}
		}
	}

	if w.EmbeddingProvider != "" {
		sb.WriteString("\n### Embedding Engine\n")
		sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", w.EmbeddingProvider))
		switch w.EmbeddingProvider {
		case "ollama":
			sb.WriteString(fmt.Sprintf("- **Endpoint**: %s\n", w.OllamaEndpoint))
			sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.OllamaModel))
		case "genai":
			sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.GenAIModel))
		}
	}

	sb.WriteString("\n### Resource Limits\n")
	sb.WriteString(fmt.Sprintf("- **Max Context Tokens**: %d\n", w.MaxTokens))
	sb.WriteString(fmt.Sprintf("- **Max Concurrent Shards**: %d\n", w.MaxConcurrentShards))

	sb.WriteString("\n---\n\n")
	sb.WriteString("**Save this configuration?** (y/n)")

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.textarea.Placeholder = "Save? (y/n)..."
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// configWizardReview handles the review confirmation.
func (m Model) configWizardReview(input string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(strings.ToLower(input), "y") {
		// Save configuration
		if err := m.saveConfigWizard(); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Error saving configuration:** %v\n\nPlease try again.", err),
				Time:    time.Now(),
			})
		} else {
			m.configWizard.Step = StepComplete
			m.awaitingConfigWizard = false
			m = m.addMessage(Message{
				Role: "assistant",
				Content: `## Configuration Saved!

Your configuration has been saved to:
- ` + "`" + `.nerd/config.json` + "`" + ` (project config)
- ` + "`" + `internal/config/config.go` + "`" + ` defaults (if applicable)

**Changes take effect on next startup.**

You can edit the config manually or run ` + "`" + `/config wizard` + "`" + ` again to reconfigure.`,
				Time: time.Now(),
			})
			m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"
			m.configWizard = nil
		}
	} else {
		// Cancel
		m.awaitingConfigWizard = false
		m.configWizard = nil
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Configuration cancelled. No changes were saved.",
			Time:    time.Now(),
		})
		m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"
	}

	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// renderCurrentConfig shows the current configuration.
func (m Model) renderCurrentConfig() string {
	var sb strings.Builder
	sb.WriteString("## Current Configuration\n\n")

	// Try to load user config
	configPath := internalconfig.DefaultUserConfigPath()
	userCfg, err := internalconfig.LoadUserConfig(configPath)
	if err != nil {
		sb.WriteString(fmt.Sprintf("*No configuration file found at %s*\n\n", configPath))
		sb.WriteString("Run `/config wizard` to create one.\n")
		return sb.String()
	}

	// Engine configuration
	sb.WriteString("### LLM Engine\n")
	engine := userCfg.GetEngine()
	sb.WriteString(fmt.Sprintf("- **Engine**: %s\n", engine))

	switch engine {
	case "claude-cli":
		cliCfg := userCfg.GetClaudeCLIConfig()
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", cliCfg.Model))
		sb.WriteString(fmt.Sprintf("- **Timeout**: %ds\n", cliCfg.Timeout))
		sb.WriteString("- **Auth**: Claude Code CLI (subscription-based)\n")

	case "codex-cli":
		cliCfg := userCfg.GetCodexCLIConfig()
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", cliCfg.Model))
		sb.WriteString(fmt.Sprintf("- **Sandbox**: %s\n", cliCfg.Sandbox))
		sb.WriteString(fmt.Sprintf("- **Timeout**: %ds\n", cliCfg.Timeout))
		sb.WriteString("- **Auth**: Codex CLI (subscription-based)\n")

	default: // "api"
		provider, apiKey := userCfg.GetActiveProvider()
		if provider != "" {
			sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", provider))
		} else {
			sb.WriteString("- **Provider**: (not set)\n")
		}
		if userCfg.Model != "" {
			sb.WriteString(fmt.Sprintf("- **Model**: %s\n", userCfg.Model))
		}
		if apiKey != "" {
			sb.WriteString("- **API Key**: ******* (configured)\n")
		} else {
			sb.WriteString("- **API Key**: (not set - check environment variables)\n")
		}
	}

	// Embedding config
	if userCfg.Embedding != nil {
		sb.WriteString("\n### Embedding Engine\n")
		sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", userCfg.Embedding.Provider))
		switch userCfg.Embedding.Provider {
		case "ollama":
			sb.WriteString(fmt.Sprintf("- **Endpoint**: %s\n", userCfg.Embedding.OllamaEndpoint))
			sb.WriteString(fmt.Sprintf("- **Model**: %s\n", userCfg.Embedding.OllamaModel))
		case "genai":
			sb.WriteString(fmt.Sprintf("- **Model**: %s\n", userCfg.Embedding.GenAIModel))
		}
	}

	// Context window
	if userCfg.ContextWindow != nil {
		sb.WriteString("\n### Context Window\n")
		sb.WriteString(fmt.Sprintf("- **Max Tokens**: %d\n", userCfg.ContextWindow.MaxTokens))
		sb.WriteString(fmt.Sprintf("- **Recent Turn Window**: %d\n", userCfg.ContextWindow.RecentTurnWindow))
	}

	sb.WriteString("\n---\n")
	sb.WriteString(fmt.Sprintf("\n*Config file: %s*\n", configPath))
	sb.WriteString("\nRun `/config wizard` to reconfigure.\n")

	return sb.String()
}

// saveConfigWizard saves the wizard configuration to disk.
func (m Model) saveConfigWizard() error {
	w := m.configWizard

	// Build UserConfig for .nerd/config.json
	// Note: We write directly to internal/config UserConfig which is the canonical config
	userCfg := &internalconfig.UserConfig{
		Engine: w.Engine,
	}

	// Configure based on engine type
	switch w.Engine {
	case "claude-cli":
		// Claude CLI configuration - no API key needed
		userCfg.ClaudeCLI = &internalconfig.ClaudeCLIConfig{
			Model:   w.ClaudeCLIModel,
			Timeout: w.ClaudeCLITimeout,
		}

	case "codex-cli":
		// Codex CLI configuration - no API key needed
		userCfg.CodexCLI = &internalconfig.CodexCLIConfig{
			Model:   w.CodexCLIModel,
			Sandbox: w.CodexCLISandbox,
			Timeout: w.CodexCLITimeout,
		}

	default: // "api"
		// HTTP API configuration - needs provider and API key
		userCfg.Provider = w.Provider
		userCfg.Model = w.Model

		// Set API key based on provider
		switch w.Provider {
		case "zai":
			userCfg.ZAIAPIKey = w.APIKey
		case "anthropic":
			userCfg.AnthropicAPIKey = w.APIKey
		case "openai":
			userCfg.OpenAIAPIKey = w.APIKey
		case "gemini":
			userCfg.GeminiAPIKey = w.APIKey
		case "xai":
			userCfg.XAIAPIKey = w.APIKey
		case "openrouter":
			userCfg.OpenRouterAPIKey = w.APIKey
		case "antigravity":
			// Antigravity uses OAuth, not API keys
			// Accounts are stored separately in ~/.nerd/antigravity_accounts.json
			// Just set the Antigravity config with thinking enabled
			userCfg.Antigravity = &internalconfig.AntigravityProviderConfig{
				EnableThinking: true,
				ThinkingLevel:  "high",
			}
		}
	}

	// Context window config
	userCfg.ContextWindow = &internalconfig.ContextWindowConfig{
		MaxTokens:              w.MaxTokens,
		CoreReservePercent:     w.CoreReservePercent,
		AtomReservePercent:     w.AtomReservePercent,
		HistoryReservePercent:  15,
		WorkingReservePercent:  w.WorkingReservePercent,
		RecentTurnWindow:       w.RecentTurnWindow,
		CompressionThreshold:   0.80,
		TargetCompressionRatio: 100.0,
		ActivationThreshold:    30.0,
	}

	// Embedding config
	if w.EmbeddingProvider != "" {
		userCfg.Embedding = &internalconfig.EmbeddingConfig{
			Provider:       w.EmbeddingProvider,
			OllamaEndpoint: w.OllamaEndpoint,
			OllamaModel:    w.OllamaModel,
			GenAIAPIKey:    w.GenAIAPIKey,
			GenAIModel:     w.GenAIModel,
			TaskType:       "SEMANTIC_SIMILARITY",
		}
	}

	// Save to .nerd/config.json
	// The perception package reads provider-specific API keys directly from
	// this file via DetectProvider(). All config is consolidated here.
	configPath := internalconfig.DefaultUserConfigPath()
	if err := userCfg.Save(configPath); err != nil {
		return fmt.Errorf("failed to save user config: %w", err)
	}

	return nil
}
