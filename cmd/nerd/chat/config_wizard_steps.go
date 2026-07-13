package chat

import (
	internalconfig "codenerd/internal/config"
	"fmt"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/logging"

	tea "github.com/charmbracelet/bubbletea"
)

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
	models := m.configWizard.availableModels()

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
	models := m.configWizard.availableModels()

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
		sb.WriteString(fmt.Sprintf("- **Repo Skill Enabled**: %t\n", w.CodexCLISkillEnabled))
		sb.WriteString(fmt.Sprintf("- **Repo Skill**: %s\n", w.CodexCLISkillName))
		sb.WriteString("- **Schema Mode**: enabled\n")
		sb.WriteString(fmt.Sprintf("- **Max Concurrent Calls**: %d\n", w.CodexCLIMaxConcurrentCalls))
		sb.WriteString("- **Auth**: Codex CLI (subscription-based)\n")

	case "xai-oauth":
		model := w.Model
		if model == "" {
			model = "grok-4.5"
		}
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", model))
		sb.WriteString("- **Auth**: SuperGrok OAuth (run `nerd auth grok`)\n")

	default: // "api"
		sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", w.Provider))
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.Model))
		if w.APIKey != "" {
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
			m.setInputMode(InputModeNormal)
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
		}
	} else {
		// Cancel
		m.setInputMode(InputModeNormal)
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
	configPath := m.userConfigPath()
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
		if cliCfg.SkillEnabled != nil {
			sb.WriteString(fmt.Sprintf("- **Repo Skill Enabled**: %t\n", *cliCfg.SkillEnabled))
		}
		sb.WriteString(fmt.Sprintf("- **Repo Skill**: %s\n", cliCfg.SkillName))
		if cliCfg.EnableOutputSchema != nil {
			sb.WriteString(fmt.Sprintf("- **Schema Mode**: %t\n", *cliCfg.EnableOutputSchema))
		}
		sb.WriteString(fmt.Sprintf("- **Max Concurrent Calls**: %d\n", cliCfg.MaxConcurrentCalls))
		sb.WriteString(fmt.Sprintf("- **Effective Scheduler Ceiling**: %d\n", userCfg.GetEffectiveMaxConcurrentAPICalls()))
		sb.WriteString("- **Auth**: Codex CLI (subscription-based)\n")

	case "xai-oauth":
		oauthCfg := userCfg.GetXAIOAuthConfig()
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", oauthCfg.Model))
		sb.WriteString(fmt.Sprintf("- **Timeout**: %ds\n", oauthCfg.Timeout))
		sb.WriteString(fmt.Sprintf("- **Max Concurrent Calls**: %d\n", oauthCfg.MaxConcurrentCalls))
		sb.WriteString(fmt.Sprintf("- **Effective Scheduler Ceiling**: %d\n", userCfg.GetEffectiveMaxConcurrentAPICalls()))
		sb.WriteString("- **Auth**: SuperGrok OAuth (subscription-based)\n")

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
	configPath := m.userConfigPath()
	userCfg, err := internalconfig.LoadUserConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load existing user config: %w", err)
	}
	if userCfg == nil {
		userCfg = &internalconfig.UserConfig{}
	}

	// The wizard owns provider, context-window, and embedding choices. Merge those
	// fields into the canonical config so logging, execution, integrations, feature
	// flags, and other subsystem settings survive reconfiguration.
	userCfg.Engine = w.Engine

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
		skillEnabled := w.CodexCLISkillEnabled
		disableShell := true
		enableSchema := true
		userCfg.CodexCLI = &internalconfig.CodexCLIConfig{
			Model:              w.CodexCLIModel,
			Sandbox:            "read-only",
			Timeout:            w.CodexCLITimeout,
			SkillEnabled:       &skillEnabled,
			SkillName:          w.CodexCLISkillName,
			MaxConcurrentCalls: w.CodexCLIMaxConcurrentCalls,
			DisableShellTool:   &disableShell,
			EnableOutputSchema: &enableSchema,
		}

	case "xai-oauth":
		// SuperGrok OAuth — no API key; authenticate with `nerd auth grok`
		importGrok := true
		model := w.Model
		if model == "" {
			model = "grok-4.5"
		}
		userCfg.XAIOAuth = &internalconfig.XAIOAuthConfig{
			Model:              model,
			Timeout:            300,
			ImportGrokAuth:     &importGrok,
			MaxConcurrentCalls: internalconfig.DefaultXAIOAuthMaxConcurrentCalls,
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
	if err := userCfg.Save(configPath); err != nil {
		return fmt.Errorf("failed to save user config: %w", err)
	}

	return nil
}
