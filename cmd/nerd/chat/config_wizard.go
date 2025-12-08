package chat

import (
	"codenerd/cmd/nerd/config"
	internalconfig "codenerd/internal/config"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	StepWelcome ConfigWizardStep = iota
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

	// Provider configuration
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

// ShardTypes for iteration
var shardTypes = []string{"coder", "tester", "reviewer", "researcher"}

// ProviderModels maps providers to their available models.
var ProviderModels = map[string][]string{
	"zai":       {"glm-4.6", "glm-4", "glm-4-air"},
	"anthropic": {"claude-sonnet-4-5-20250514", "claude-opus-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"},
	"openai":    {"gpt-5.1-codex-max", "gpt-5.1-codex-mini", "gpt-5-codex", "gpt-4o", "gpt-4o-mini"},
	"gemini":    {"gemini-3-pro-preview", "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"},
	"xai":       {"grok-2-latest", "grok-2", "grok-beta"},
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
		"google/gemini-2.0-flash-exp:free",
		"google/gemini-pro-1.5",
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
		// Defaults
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
	// User pressed enter to start, move to provider selection
	m.configWizard.Step = StepProvider
	m.history = append(m.history, Message{
		Role: "assistant",
		Content: `## Step 1: LLM Provider

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
		m.history = append(m.history, Message{
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

	m.history = append(m.history, Message{
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

	m.history = append(m.history, Message{
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

	m.history = append(m.history, Message{
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
		m.configWizard.CurrentShard = shardTypes[0]
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

	m.history = append(m.history, Message{
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

	m.history = append(m.history, Message{
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

	m.history = append(m.history, Message{
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
	m.configWizard.ShardIndex++

	if m.configWizard.ShardIndex < len(shardTypes) {
		m.configWizard.CurrentShard = shardTypes[m.configWizard.ShardIndex]
		m.configWizard.Step = StepShardModel
		return m.showShardModelPrompt()
	}

	// All shards configured, move to embedding
	m.configWizard.Step = StepEmbeddingProvider
	return m.showEmbeddingProviderPrompt()
}

// showEmbeddingProviderPrompt shows embedding provider selection.
func (m Model) showEmbeddingProviderPrompt() (tea.Model, tea.Cmd) {
	m.history = append(m.history, Message{
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
		m.history = append(m.history, Message{
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
		m.history = append(m.history, Message{
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
	if m.configWizard.EmbeddingProvider == "ollama" {
		if input != "" {
			m.configWizard.OllamaEndpoint = input
		}
	} else if m.configWizard.EmbeddingProvider == "genai" {
		if input != "" {
			m.configWizard.GenAIAPIKey = input
		}
	}

	m.configWizard.Step = StepContextWindow
	return m.showContextWindowPrompt()
}

// showContextWindowPrompt shows context window configuration.
func (m Model) showContextWindowPrompt() (tea.Model, tea.Cmd) {
	m.history = append(m.history, Message{
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

	m.history = append(m.history, Message{
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
	sb.WriteString("### LLM Provider\n")
	sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", w.Provider))
	sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.Model))
	if w.APIKey != "" {
		sb.WriteString("- **API Key**: ******* (set)\n")
	} else {
		sb.WriteString("- **API Key**: (using environment variable)\n")
	}

	if w.ConfigureShards && len(w.ShardProfiles) > 0 {
		sb.WriteString("\n### Per-Shard Configuration\n\n")
		sb.WriteString("| Shard | Model | Temp | Context |\n")
		sb.WriteString("|-------|-------|------|-------:|\n")
		for _, shard := range shardTypes {
			if profile, ok := w.ShardProfiles[shard]; ok {
				sb.WriteString(fmt.Sprintf("| %s | %s | %.1f | %d |\n",
					shard, profile.Model, profile.Temperature, profile.MaxContextTokens))
			}
		}
	}

	if w.EmbeddingProvider != "" {
		sb.WriteString("\n### Embedding Engine\n")
		sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", w.EmbeddingProvider))
		if w.EmbeddingProvider == "ollama" {
			sb.WriteString(fmt.Sprintf("- **Endpoint**: %s\n", w.OllamaEndpoint))
			sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.OllamaModel))
		} else if w.EmbeddingProvider == "genai" {
			sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.GenAIModel))
		}
	}

	sb.WriteString("\n### Resource Limits\n")
	sb.WriteString(fmt.Sprintf("- **Max Context Tokens**: %d\n", w.MaxTokens))
	sb.WriteString(fmt.Sprintf("- **Max Concurrent Shards**: %d\n", w.MaxConcurrentShards))

	sb.WriteString("\n---\n\n")
	sb.WriteString("**Save this configuration?** (y/n)")

	m.history = append(m.history, Message{
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
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Error saving configuration:** %v\n\nPlease try again.", err),
				Time:    time.Now(),
			})
		} else {
			m.configWizard.Step = StepComplete
			m.awaitingConfigWizard = false
			m.history = append(m.history, Message{
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
		m.history = append(m.history, Message{
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

	provider, apiKey := userCfg.GetActiveProvider()
	sb.WriteString("### LLM Provider\n")
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

	// Embedding config
	if userCfg.Embedding != nil {
		sb.WriteString("\n### Embedding Engine\n")
		sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", userCfg.Embedding.Provider))
		if userCfg.Embedding.Provider == "ollama" {
			sb.WriteString(fmt.Sprintf("- **Endpoint**: %s\n", userCfg.Embedding.OllamaEndpoint))
			sb.WriteString(fmt.Sprintf("- **Model**: %s\n", userCfg.Embedding.OllamaModel))
		} else if userCfg.Embedding.Provider == "genai" {
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

	// Load existing config or create new
	cfg, _ := config.Load()

	// Build UserConfig for .nerd/config.json
	userCfg := &internalconfig.UserConfig{
		Provider: w.Provider,
		Model:    w.Model,
	}

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
	configPath := internalconfig.DefaultUserConfigPath()
	if err := userCfg.Save(configPath); err != nil {
		return fmt.Errorf("failed to save user config: %w", err)
	}

	// Also update the simple config for backward compatibility
	cfg.APIKey = w.APIKey
	if err := config.Save(cfg); err != nil {
		// Non-fatal, user config is primary
	}

	return nil
}
