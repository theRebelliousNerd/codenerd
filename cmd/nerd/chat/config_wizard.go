package chat

import (
	internalconfig "codenerd/internal/config"
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
	CodexCLIModel              string // gpt-5, o4-mini, codex-mini-latest
	CodexCLISandbox            string // always read-only; execution belongs to Tactile
	CodexCLITimeout            int    // seconds
	CodexCLISkillEnabled       bool
	CodexCLISkillName          string
	CodexCLIMaxConcurrentCalls int

	// Provider configuration (when Engine="api")
	Provider string // zai, anthropic, openai, gemini, xai
	APIKey   string
	Model    string

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
	"openai":    {"gpt-5.4", "gpt-5.3-codex", "gpt-5.3-codex-spark", "gpt-5.2-codex", "gpt-5.1-codex-max", "gpt-5-codex", "gpt-4o", "gpt-4o-mini"},
	"gemini":    {"gemini-3.1-flash-lite", "gemini-3-flash-preview", "gemini-3.1-pro-preview", "gemini-3-pro-preview"},
	"xai":       {"grok-4-1-fast-reasoning", "grok-2-latest", "grok-2", "grok-beta"},
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
		"google/gemini-3.1-flash-lite",
		"google/gemini-3-flash-preview",
		"google/gemini-3.1-pro-preview",
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

var claudeCLIWizardModels = []string{"sonnet", "opus", "haiku"}

var codexCLIWizardModels = []string{
	"gpt-5.4",
	"gpt-5.3-codex",
	"gpt-5.3-codex-spark",
	"gpt-5.2-codex",
	"gpt-5.2",
	"gpt-5.1-codex-max",
	"gpt-5.1",
	"gpt-5.1-codex",
	"gpt-5-codex",
	"gpt-5",
}

// DefaultProviderModel returns the default model for a provider.
func DefaultProviderModel(provider string) string {
	models, ok := ProviderModels[provider]
	if ok && len(models) > 0 {
		return models[0]
	}
	return ""
}

func (w *ConfigWizardState) availableModels() []string {
	switch w.Engine {
	case "claude-cli":
		return claudeCLIWizardModels
	case "codex-cli":
		return codexCLIWizardModels
	default:
		return ProviderModels[w.Provider]
	}
}

// NewConfigWizard creates a new configuration wizard state.
func NewConfigWizard() *ConfigWizardState {
	return &ConfigWizardState{
		Step:          StepWelcome,
		ShardProfiles: make(map[string]*ShardProfileConfig),
		// Engine defaults
		Engine:                     "api", // Default to HTTP API mode
		ClaudeCLIModel:             "sonnet",
		ClaudeCLITimeout:           300,
		CodexCLIModel:              "gpt-5.4",
		CodexCLISandbox:            "read-only",
		CodexCLITimeout:            300,
		CodexCLISkillEnabled:       true,
		CodexCLISkillName:          internalconfig.DefaultCodexExecSkillName,
		CodexCLIMaxConcurrentCalls: internalconfig.DefaultCodexMaxConcurrentCalls,
		// Embedding defaults
		OllamaEndpoint:        "http://localhost:11434",
		OllamaModel:           "embeddinggemma:300m",
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
| 4 | xai-oauth | SuperGrok OAuth (SuperGrok / X Premium+ subscription) |

**Recommendation:**
- Use **api** if you have API credits and want fine-grained control
- Use **claude-cli** if you have Claude Pro/Max subscription
- Use **codex-cli** if you have ChatGPT Plus/Pro subscription
- Use **xai-oauth** if you have SuperGrok (run nerd auth grok)

Enter a number (1-4) or engine name:`,
		Time: time.Now(),
	})
	m.textarea.Placeholder = "Enter engine (1-4 or name)..."
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
		"4": "xai-oauth", "xai-oauth": "xai-oauth", "grok": "xai-oauth", "supergrok": "xai-oauth",
	}

	engine, ok := engines[strings.ToLower(input)]
	if !ok {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Invalid selection. Please enter 1-4 or an engine name (api, claude-cli, codex-cli, xai-oauth):",
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
| 1 | gpt-5.4 | **Recommended** - Best current GPT-5 Codex-capable model |
| 2 | gpt-5.3-codex | Current Codex-tuned model |
| 3 | gpt-5.3-codex-spark | Fast Codex Spark variant |
| 4 | gpt-5.2-codex | Previous Codex-tuned model |
| 5 | gpt-5.2 | Previous general GPT-5 model |
| 6 | gpt-5.1-codex-max | Older top-end Codex model |
| 7 | gpt-5.1 | Older general GPT-5 model |
| 8 | gpt-5.1-codex | Older Codex model |
| 9 | gpt-5-codex | Legacy agentic model |
| 10 | gpt-5 | Legacy general model |

Current: **%s**

Enter model number/name (Enter for default):`, m.configWizard.CodexCLIModel),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Codex CLI model (Enter for gpt-5.4)..."

	case "xai-oauth":
		// SuperGrok OAuth: no API key; auth via `nerd auth grok`. Use defaults and continue.
		if m.configWizard.Model == "" {
			m.configWizard.Model = "grok-4.5"
		}
		m.configWizard.Step = StepShardConfig
		m = m.addMessage(Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## Step 2: SuperGrok OAuth

You selected **xai-oauth** (SuperGrok / X Premium+ subscription).

- Default model: **%s**
- Auth: run "nerd auth grok" (or import from Grok CLI ~/.grok/auth.json)
- No xai_api_key required

Would you like to configure individual shard settings?

**y** = Configure each shard
**n** = Use defaults for all shards (recommended)`, m.configWizard.Model),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "y/n (Enter for n)..."

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
| 5 | xai | xAI Grok |
| 6 | openrouter | OpenRouter (multi-provider gateway) |

Enter a number (1-6) or provider name:`,
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
		"1": "gpt-5.4", "gpt-5.4": "gpt-5.4",
		"2": "gpt-5.3-codex", "gpt-5.3-codex": "gpt-5.3-codex",
		"3": "gpt-5.3-codex-spark", "gpt-5.3-codex-spark": "gpt-5.3-codex-spark",
		"4": "gpt-5.2-codex", "gpt-5.2-codex": "gpt-5.2-codex",
		"5": "gpt-5.2", "gpt-5.2": "gpt-5.2",
		"6": "gpt-5.1-codex-max", "gpt-5.1-codex-max": "gpt-5.1-codex-max",
		"7": "gpt-5.1", "gpt-5.1": "gpt-5.1",
		"8": "gpt-5.1-codex", "gpt-5.1-codex": "gpt-5.1-codex",
		"9": "gpt-5-codex", "gpt-5-codex": "gpt-5-codex",
		"10": "gpt-5", "gpt-5": "gpt-5",
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
Repo skill: **%s**
Schema mode: **enabled**
Codex max concurrent calls: **%d**

Would you like to configure individual shard settings?
(model, temperature, context limits per shard type)

| Shard | Purpose |
|-------|---------|
| coder | Code generation, edits |
| tester | Test creation, execution |
| reviewer | Code review, analysis |
| researcher | Knowledge gathering |

**y** = Configure each shard
**n** = Use defaults for all shards (recommended for quick setup)`, m.configWizard.CodexCLIModel, m.configWizard.CodexCLISkillName, m.configWizard.CodexCLIMaxConcurrentCalls),
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
		"5": "xai", "xai": "xai",
		"6": "openrouter", "openrouter": "openrouter",
	}

	provider, ok := providers[strings.ToLower(input)]
	if !ok {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Invalid selection. Please enter 1-6 or a provider name (zai, anthropic, openai, gemini, xai, openrouter):",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	m.configWizard.Provider = provider

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
