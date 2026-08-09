package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/features"
	"codenerd/internal/logging"
)

// UserConfig holds ALL codeNERD configuration from .nerd/config.json.
// This is the single source of truth for configuration.
//
// Supported models by provider:
//   - anthropic:   claude-sonnet-4-5-20250514, claude-opus-4-20250514, claude-3-5-sonnet-20241022
//   - openai:      gpt-5.4 (default), gpt-5.3-codex, gpt-5.3-codex-spark, gpt-5.2-codex, gpt-5.1-codex-max, gpt-5-codex, gpt-4o
//   - gemini:      gemini-3.5-flash (default), gemini-3.1-flash-lite, gemini-3-pro-preview
//   - xai:         grok-2-latest (default), grok-2, grok-beta
//   - zai:         GLM-4.6 (default)
//   - openrouter:  anthropic/claude-3.5-sonnet, openai/gpt-4o, google/gemini-pro, etc.
//   - ollama:      any local tag (e.g. gemma4:12b) — no API key required
type UserConfig struct {
	// =========================================================================
	// LLM PROVIDER CONFIGURATION
	// =========================================================================

	// Provider selection (anthropic, openai, gemini, xai, zai, openrouter, ollama)
	Provider string `json:"provider,omitempty"`

	// API keys for each provider
	APIKey           string `json:"api_key,omitempty"`            // Legacy: single key
	AnthropicAPIKey  string `json:"anthropic_api_key,omitempty"`  // Anthropic/Claude
	OpenAIAPIKey     string `json:"openai_api_key,omitempty"`     // OpenAI/Codex
	GeminiAPIKey     string `json:"gemini_api_key,omitempty"`     // Google Gemini
	XAIAPIKey        string `json:"xai_api_key,omitempty"`        // xAI/Grok
	ZAIAPIKey        string `json:"zai_api_key,omitempty"`        // Z.AI
	OpenRouterAPIKey string `json:"openrouter_api_key,omitempty"` // OpenRouter (multi-provider)

	// OpenAI-compatible direct vendors. Each speaks the OpenAI Chat Completions
	// wire format at its own base URL, so all three share one client
	// implementation (client_openai_compat.go) with thin per-vendor request hooks.
	DashScopeAPIKey string `json:"dashscope_api_key,omitempty"` // Alibaba Model Studio (Qwen)
	MetaAPIKey      string `json:"meta_api_key,omitempty"`      // Meta Model API (Muse Spark)
	MoonshotAPIKey  string `json:"moonshot_api_key,omitempty"`  // Moonshot AI (Kimi)

	// BaseURL overrides the endpoint for OpenAI-compatible providers. Each known
	// vendor ships a default, so this is only needed for a proxy, a regional
	// endpoint, or an OpenAI-compatible vendor codeNERD has no name for yet.
	BaseURL string `json:"base_url,omitempty"`
	// Ollama needs no API key; presence of provider=ollama or worker.provider=ollama is enough.

	// Optional model override (see supported models above)
	Model string `json:"model,omitempty"`

	// MaxOutputTokens caps the completion length for the MAIN client. Zero means
	// the client's own default, which for OpenAI-compatible vendors is 16384 —
	// far below what a large model can emit, and previously unreachable from
	// config, so a long answer was silently truncated with no knob to raise it.
	// Per-slot equivalents live on worker/planner (SecondaryLLMConfig); Gemini
	// keeps its own gemini.max_output_tokens because thinking and visible output
	// share that budget.
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`

	// ClassificationModel is the fast/cheap model used for perception intent
	// classification (the transducer's Understand call). Every interactive turn
	// pays this call before anything else happens, so it should be the
	// provider's fastest tier, NOT the main model. When empty, codeNERD picks a
	// per-provider default (Anthropic: claude-haiku-4-5, Gemini:
	// gemini-3.1-flash-lite, OpenAI: gpt-4o-mini); providers without a known
	// fast tier (zai, xai, openrouter) fall back to the main client unless this
	// is set explicitly. The main Model setting never applies to classification.
	ClassificationModel string `json:"classification_model,omitempty"`

	// =========================================================================
	// CLI ENGINE CONFIGURATION
	// =========================================================================

	// Engine selection: "api" (default), "claude-cli", "codex-cli", "xai-oauth"
	// When set to a CLI/OAuth engine, uses that backend instead of HTTP API keys.
	Engine string `json:"engine,omitempty"`

	// Gemini-specific configuration (thinking mode, grounding tools)
	Gemini *GeminiProviderConfig `json:"gemini,omitempty"`

	// Claude Code CLI configuration (used when Engine="claude-cli")
	ClaudeCLI *ClaudeCLIConfig `json:"claude_cli,omitempty"`

	// Codex CLI configuration (used when Engine="codex-cli")
	CodexCLI *CodexCLIConfig `json:"codex_cli,omitempty"`

	// XAIOAuth SuperGrok / X Premium+ OAuth configuration (used when Engine="xai-oauth")
	XAIOAuth *XAIOAuthConfig `json:"xai_oauth,omitempty"`

	// Ollama chat (not embedding) configuration when provider=ollama is the main client.
	// Embeddings still use embedding.ollama_*; this block is for LLM chat completions.
	Ollama *OllamaLLMConfig `json:"ollama,omitempty"`

	// Worker is an optional secondary LLM for shards / spawn / create / classification
	// while the main TUI agent stays on Engine/Provider/Model. When unset, shards use
	// the main client (e.g. SuperGrok via engine=xai-oauth). Optional cheap testing:
	// main xai-oauth + worker.provider=ollama worker.model=gemma4:12b.
	// Image generation shards are excluded — see Image.
	Worker *WorkerLLMConfig `json:"worker,omitempty"`

	// Planner is an optional high-reasoning LLM for the work whose quality
	// dominates its token cost: planning, adversarial review, static analysis,
	// policy authorship. When nil, that work stays on the worker (or main)
	// client. This exists so a two-tier stack — an expensive reasoning model
	// plus a cheap bulk model — is expressible: without it, pointing Worker at
	// a cheap model demotes /review and /audit along with everything else,
	// which is backwards. See SecondaryLLMConfig.
	Planner *PlannerLLMConfig `json:"planner,omitempty"`

	// Image is the dedicated image-generation LLM (Nano Banana 2 / Gemini Image).
	// Not routed through worker=ollama — image models are Gemini-only.
	// API id: gemini-3.1-flash-image (Nano Banana 2).
	Image *ImageLLMConfig `json:"image,omitempty"`

	// =========================================================================
	// UI SETTINGS
	// =========================================================================

	// Theme for the TUI ("light" or "dark")
	Theme string `json:"theme,omitempty"`

	// ContinuationMode controls multi-step task execution behavior
	// 0 = Auto (fully automatic), 1 = Confirm (pause after each step), 2 = Breakpoint (pause before mutations)
	ContinuationMode int `json:"continuation_mode,omitempty"`

	// =========================================================================
	// EXTERNAL SERVICE KEYS
	// =========================================================================

	// Context7 API key for research shards
	Context7APIKey string `json:"context7_api_key,omitempty"`

	// =========================================================================
	// CONTEXT & MEMORY CONFIGURATION
	// =========================================================================

	// Context Window Configuration (§8.2 Semantic Compression)
	// This controls the token budget for context compression
	ContextWindow *ContextWindowConfig `json:"context_window,omitempty"`

	// Embedding engine configuration for semantic vector search
	Embedding *EmbeddingConfig `json:"embedding,omitempty"`

	// Reflection (System 2 memory) configuration
	Reflection *ReflectionConfig `json:"reflection,omitempty"`

	// =========================================================================
	// SHARD CONFIGURATION (Per-Shard Settings)
	// =========================================================================

	// Per-shard profiles: coder, tester, reviewer, researcher
	// Each shard type can have custom model, temperature, context limits
	ShardProfiles map[string]ShardProfile `json:"shard_profiles,omitempty"`

	// Default shard settings (fallback for undefined shard types)
	DefaultShard *ShardProfile `json:"default_shard,omitempty"`

	// =========================================================================
	// RESOURCE LIMITS
	// =========================================================================

	// Core resource limits enforced system-wide
	CoreLimits *CoreLimits `json:"core_limits,omitempty"`

	// APIScheduler controls LLM slot queuing, priority grants, call spacing,
	// and adaptive concurrency on provider rate limits. See APISchedulerPolicy.
	APIScheduler *APISchedulerPolicy `json:"api_scheduler,omitempty"`

	// LLMTimeouts overrides the per-call, per-operation, and per-campaign
	// timeouts. Absent means the "default" profile, whose values are documented
	// as calibrated for Z.AI/GLM-4.7 — generous for that vendor and usually far
	// too generous for anything else. Set `{"profile": "fast"}` or override
	// individual durations. See LLMTimeoutsConfig.
	LLMTimeouts *LLMTimeoutsConfig `json:"llm_timeouts,omitempty"`

	// World model scanning/AST parsing configuration
	World *WorldConfig `json:"world,omitempty"`

	// =========================================================================
	// INTEGRATIONS
	// =========================================================================

	// Integration service configuration
	Integrations *IntegrationsConfig `json:"integrations,omitempty"`

	// Native browser automation configuration. This is intentionally separate
	// from integrations.servers.browser, which describes an external MCP server.
	Browser *BrowserAutomationConfig `json:"browser,omitempty"`

	// =========================================================================
	// TOOL GENERATION (Ouroboros)
	// =========================================================================

	// Tool Generation settings for Ouroboros self-generating tools
	ToolGeneration *ToolGenerationConfig `json:"tool_generation,omitempty"`

	// =========================================================================
	// BUILD ENVIRONMENT
	// =========================================================================

	// Build environment configuration for go build/test commands
	// Ensures consistent CGO_CFLAGS etc. across all components
	Build *BuildConfig `json:"build,omitempty"`

	// =========================================================================
	// EXECUTION SETTINGS
	// =========================================================================

	// Execution configuration for tactile interface
	Execution *ExecutionConfig `json:"execution,omitempty"`

	// =========================================================================
	// LOGGING
	// =========================================================================

	// Logging configuration
	Logging *LoggingConfig `json:"logging,omitempty"`

	// =========================================================================
	// JIT PROMPT COMPILER
	// =========================================================================

	// JIT Prompt Compiler configuration
	JIT *JITConfig `json:"jit,omitempty"`

	// =====================================================================
	// LEARNING CANDIDATES
	// =====================================================================

	// Repeats required before proposing a learning candidate (default: 3)
	LearningCandidateThreshold int `json:"learning_candidate_threshold,omitempty"`
	// Require explicit confirmation before promotion (default: false)
	LearningCandidateAutoPromote bool `json:"learning_candidate_auto_promote,omitempty"`

	// =========================================================================
	// USER EXPERIENCE
	// =========================================================================

	// Onboarding state tracking for progressive disclosure
	Onboarding *OnboardingState `json:"onboarding,omitempty"`

	// Transparency configuration for operation visibility
	Transparency *TransparencyConfig `json:"transparency,omitempty"`

	// Guidance configuration for contextual help
	Guidance *GuidanceConfig `json:"guidance,omitempty"`

	// =========================================================================
	// FEATURE FLAGS
	// =========================================================================
	//
	// Modernization toggles: DifferentialEngine, FlightRecorder, Provenance,
	// system shards, dark mode, onboarding, taxonomy-fast, etc. Each field
	// is a pointer so we can distinguish "user wrote `false`" from "key
	// absent → use default". After LoadUserConfig parses the file it
	// installs this block into the `internal/features` package's
	// process-wide active pointer so low-level call sites (kernel, world
	// scanner, main.go boot) can consult it without re-reading config.json.
	Features *features.FeaturesConfig `json:"features,omitempty"`
}

// GetContextWindowConfig returns the context window config with defaults.
func (c *UserConfig) GetContextWindowConfig() ContextWindowConfig {
	def := DefaultContextWindowConfig()
	if c.ContextWindow != nil {
		cfg := *c.ContextWindow
		// Apply defaults for zero values
		if cfg.MaxTokens == 0 {
			cfg.MaxTokens = def.MaxTokens
		}
		if cfg.CoreReservePercent == 0 {
			cfg.CoreReservePercent = def.CoreReservePercent
		}
		if cfg.AtomReservePercent == 0 {
			cfg.AtomReservePercent = def.AtomReservePercent
		}
		if cfg.HistoryReservePercent == 0 {
			cfg.HistoryReservePercent = def.HistoryReservePercent
		}
		if cfg.WorkingReservePercent == 0 {
			cfg.WorkingReservePercent = def.WorkingReservePercent
		}
		if cfg.OutputReserve == 0 {
			cfg.OutputReserve = def.OutputReserve
		}
		// Note: ThinkingReserve can be 0 (disabled) - only set if explicitly negative
		if cfg.ThinkingReserve < 0 {
			cfg.ThinkingReserve = 0
		}
		if cfg.ToolUseBuffer == 0 {
			cfg.ToolUseBuffer = def.ToolUseBuffer
		}
		if cfg.RecentTurnWindow == 0 {
			cfg.RecentTurnWindow = def.RecentTurnWindow
		}
		if cfg.CompressionThreshold == 0 {
			cfg.CompressionThreshold = def.CompressionThreshold
		}
		if cfg.TargetCompressionRatio == 0 {
			cfg.TargetCompressionRatio = def.TargetCompressionRatio
		}
		if cfg.ActivationThreshold == 0 {
			cfg.ActivationThreshold = def.ActivationThreshold
		}
		return cfg
	}
	return def
}

// GetEmbeddingConfig returns the embedding config from .nerd/config.json
// (UserConfig.Embedding) with defaults applied for empty fields ONLY.
//
// RULE: ollama_model / provider / endpoint MUST be driven by config.json.
// Callers must use this helper (or LoadUserConfig + this) — never invent a
// model name at the call site. Boot, /init, reembed, and perception all go
// through here so a single config.json change is authoritative.
func (c *UserConfig) GetEmbeddingConfig() EmbeddingConfig {
	if c != nil && c.Embedding != nil {
		cfg := *c.Embedding
		// Apply defaults for zero values only — never overwrite a non-empty
		// explicit config.json value (except bare "embeddinggemma", which is
		// not a real Ollama tag on most installs).
		if cfg.Provider == "" {
			cfg.Provider = "ollama"
		}
		if cfg.OllamaEndpoint == "" {
			cfg.OllamaEndpoint = "http://localhost:11434"
		}
		if cfg.OllamaModel == "" || cfg.OllamaModel == "embeddinggemma" {
			// Bare tag 404s without :latest; canonical config.json value is :300m.
			cfg.OllamaModel = "embeddinggemma:300m"
		}
		if cfg.GenAIModel == "" {
			cfg.GenAIModel = "gemini-embedding-001"
		}
		if cfg.TaskType == "" {
			cfg.TaskType = "SEMANTIC_SIMILARITY"
		}
		return cfg
	}
	// No embedding block in config.json — return defaults that match the
	// canonical config.json shape so first-run and missing-block behave the same.
	return EmbeddingConfig{
		Provider:       "ollama",
		OllamaEndpoint: "http://localhost:11434",
		OllamaModel:    "embeddinggemma:300m",
		GenAIModel:     "gemini-embedding-001",
		TaskType:       "SEMANTIC_SIMILARITY",
	}
}

// GetReflectionConfig returns reflection config with defaults applied.
func (c *UserConfig) GetReflectionConfig() ReflectionConfig {
	def := DefaultReflectionConfig()
	if c.Reflection != nil {
		cfg := *c.Reflection
		if !cfg.enabledSet {
			cfg.Enabled = def.Enabled
		}
		if cfg.TopK == 0 {
			cfg.TopK = def.TopK
		}
		if !cfg.minScoreSet {
			cfg.MinScore = def.MinScore
		}
		if cfg.RecencyHalfLifeDays == 0 {
			cfg.RecencyHalfLifeDays = def.RecencyHalfLifeDays
		}
		if cfg.BacklogWatermark == 0 {
			cfg.BacklogWatermark = def.BacklogWatermark
		}
		return cfg
	}
	return def
}

// GetToolGenerationConfig returns tool generation settings with defaults applied.
func (c *UserConfig) GetToolGenerationConfig() ToolGenerationConfig {
	cfg := DefaultToolGenerationConfig()
	if c != nil && c.ToolGeneration != nil {
		if c.ToolGeneration.TargetOS != "" {
			cfg.TargetOS = c.ToolGeneration.TargetOS
		}
		if c.ToolGeneration.TargetArch != "" {
			cfg.TargetArch = c.ToolGeneration.TargetArch
		}
	}
	return cfg
}

// GetBuildConfig returns the build configuration with defaults.
func (c *UserConfig) GetBuildConfig() BuildConfig {
	cfg := DefaultBuildConfig()
	if c.Build != nil {
		if len(c.Build.EnvVars) > 0 {
			cfg.EnvVars = c.Build.EnvVars
		}
		if len(c.Build.GoFlags) > 0 {
			cfg.GoFlags = c.Build.GoFlags
		}
		if len(c.Build.CGOPackages) > 0 {
			cfg.CGOPackages = c.Build.CGOPackages
		}
	}
	return cfg
}

// DefaultUserConfigPath returns the default path to .nerd/config.json.
func DefaultUserConfigPath() string {
	root, err := FindWorkspaceRoot()
	if err != nil {
		return ".nerd/config.json"
	}
	return filepath.Join(root, ".nerd", "config.json")
}

// FindWorkspaceRoot finds the project root by walking upward from cwd. It
// returns the deepest ancestor containing a go.mod (the canonical Go module
// root), or — only if no go.mod is found anywhere in the chain — the
// deepest ancestor containing a .nerd directory.
//
// Why go.mod takes precedence over .nerd:
//
// The previous behavior was "stop at the first .nerd OR go.mod going
// upward." That broke in two distinct ways:
//
//  1. Stray nested .nerd trap. If a .nerd/ ever materialized inside a
//     subpackage (e.g. cmd/nerd/chat/.nerd from a test run with cwd
//     pointing there), every subsequent run from inside that subtree
//     would stop at the stray and write all state into it, compounding
//     the pollution. Walking past stray .nerd toward the real module
//     root (go.mod) fixes this.
//
//  2. Personal home .nerd false positive. A "topmost .nerd wins"
//     implementation walks past the actual project root to find a global
//     ~/.nerd, mixing every project's state into one personal directory.
//     Stopping at the project's go.mod (always inside the repo, never in
//     ~) prevents this.
//
// Algorithm:
//
//   - Walk upward from cwd.
//   - At each level, return immediately if the dir has a go.mod.
//   - Otherwise remember the deepest .nerd we see as a fallback.
//   - If we reach the filesystem root without finding go.mod, return
//     the deepest .nerd. If neither exists, return cwd.
//
// This makes go.mod the authoritative project boundary for Go projects
// (codeNERD is always Go) while still supporting fresh non-Go workspaces
// via the .nerd fallback.
func FindWorkspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	originalDir := dir
	var deepestNerd string

	for {
		// go.mod is the authoritative module-root marker — return immediately.
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, nil
		}
		// Track the deepest .nerd seen (walk goes deepest -> shallowest, so
		// the first assignment is the deepest).
		if deepestNerd == "" {
			if info, err := os.Stat(filepath.Join(dir, ".nerd")); err == nil && info.IsDir() {
				deepestNerd = dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if deepestNerd != "" {
		return deepestNerd, nil
	}
	return originalDir, nil
}

// LoadUserConfig loads configuration from .nerd/config.json.
func LoadUserConfig(path string) (*UserConfig, error) {
	cfg := &UserConfig{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return empty config if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read user config: %w", err)
	}

	if err := decodeStrictJSON(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	// Make feature toggles visible to leaf packages (internal/core,
	// internal/observability, internal/world, ...) that cannot import
	// internal/config. Nil is fine: SetActive(nil) resets the registry
	// so accessors fall back to compile-time defaults.
	features.SetActive(cfg.Features)
	// Log the post-install snapshot so triage can tell at a glance which
	// flags are live for this run (features.SetActive itself stays
	// log-free to keep the leaf package dependency-free).
	logging.Get(logging.CategoryBoot).Info("%s", features.Summary())

	// Install the timeout profile into the process-wide singleton the ~25
	// GetLLMTimeouts() call sites read. Same install-on-load pattern as
	// features above, and for the same reason: those call sites live in
	// packages that cannot import internal/config's UserConfig.
	timeouts, terr := cfg.LLMTimeouts.Resolve()
	if terr != nil {
		// A malformed timeout is a config error the user can fix, and running
		// on a silently-wrong 30-minute default is the failure this replaced.
		return nil, fmt.Errorf("failed to parse user config: %w", terr)
	}
	SetLLMTimeouts(timeouts)
	if cfg.LLMTimeouts != nil {
		logging.Get(logging.CategoryBoot).Info(
			"LLM timeouts: profile=%q ooda=%s shard=%s per_call=%s max_retries=%d",
			cfg.LLMTimeouts.Profile, timeouts.OODALoopTimeout, timeouts.ShardExecutionTimeout,
			timeouts.PerCallTimeout, timeouts.MaxRetries)
	}

	return cfg, nil
}

// SaveUserConfig saves configuration to .nerd/config.json.
func (c *UserConfig) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user config: %w", err)
	}
	data = append(data, '\n')
	if err := writePrivateFileAtomically(path, data); err != nil {
		return fmt.Errorf("failed to write user config: %w", err)
	}

	return nil
}

// OllamaLLMConfig is chat configuration for local Ollama (separate from
// EmbeddingConfig which drives embeddinggemma).
type OllamaLLMConfig struct {
	// Endpoint is the Ollama HTTP base (default http://127.0.0.1:11434).
	// Chat uses {Endpoint}/v1/chat/completions.
	Endpoint string `json:"endpoint,omitempty"`
	// Model is the Ollama tag for chat (e.g. gemma4:12b).
	Model string `json:"model,omitempty"`
}

// SecondaryLLMConfig selects an LLM for a slot other than the main client.
// Both the worker slot (bulk work) and the planner slot (reasoning work) use
// this shape, so a slot is fully described by provider + model + optional
// endpoint override.
type SecondaryLLMConfig struct {
	// Provider: any provider the main client supports, e.g. "meta",
	// "dashscope", or "ollama" for local testing.
	Provider string `json:"provider,omitempty"`
	// Model: e.g. gemma4:12b
	Model string `json:"model,omitempty"`
	// Endpoint: base-URL override. For ollama it defaults to ollama.endpoint
	// or localhost; for OpenAI-compatible vendors it overrides the base URL so
	// two slots can sit behind different gateways.
	Endpoint string `json:"endpoint,omitempty"`
	// MaxOutputTokens caps the completion length for this slot. Zero means the
	// client's own default (16384 for OpenAI-compatible vendors), which is well
	// below what large models can emit — a planner asked for a long plan was
	// simply truncated, with no way to raise the ceiling from config. Slots
	// differ in what they need: a bulk worker wants a small budget for cost, a
	// planner wants a large one, so this belongs per-slot rather than global.
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`
}

// WorkerLLMConfig selects a secondary LLM for non-main work (shards, spawn,
// create, classification). When nil, workers share the main provider client.
// Does NOT apply to image generation (see ImageLLMConfig).
type WorkerLLMConfig = SecondaryLLMConfig

// PlannerLLMConfig selects the high-reasoning LLM for planning and analysis
// verbs. When nil, that work falls back to the worker client, then the main
// client.
type PlannerLLMConfig = SecondaryLLMConfig

// ImageLLMConfig is the dedicated image-generation path (Gemini Nano Banana 2).
// Official API model id: gemini-3.1-flash-image (alias: Nano Banana 2).
// Lite variant: gemini-3.1-flash-lite-image (Nano Banana 2 Lite).
type ImageLLMConfig struct {
	// Provider must be "gemini" today (Gemini Image / Nano Banana family).
	Provider string `json:"provider,omitempty"`
	// Model API id, default gemini-3.1-flash-image.
	Model string `json:"model,omitempty"`
}

// DefaultImageModel is Nano Banana 2 (Gemini 3.1 Flash Image).
const DefaultImageModel = "gemini-3.1-flash-image"

// IsImageGenerationModel reports whether model is a Gemini image / Nano Banana model.
func IsImageGenerationModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case m == "":
		return false
	case strings.Contains(m, "flash-image"):
		return true
	case strings.Contains(m, "flash-lite-image"):
		return true
	case strings.Contains(m, "image-preview") && strings.Contains(m, "gemini"):
		return true
	case m == "nano-banana" || m == "nano-banana-2" || m == "nano_banana" || m == "nano_banana_2":
		return true
	default:
		return false
	}
}

// IsImageShardType reports shard type names reserved for image generation.
func IsImageShardType(typeName string) bool {
	t := strings.ToLower(strings.TrimSpace(typeName))
	t = strings.TrimPrefix(t, "/")
	switch t {
	case "image_generator", "image-generator", "imagegenerator",
		"imagen", "image", "nano_banana", "nanobanana":
		return true
	default:
		return false
	}
}

// GetImageLLMConfig returns image-generation settings with Nano Banana 2 defaults.
func (c *UserConfig) GetImageLLMConfig() ImageLLMConfig {
	def := ImageLLMConfig{
		Provider: "gemini",
		Model:    DefaultImageModel,
	}
	if c == nil || c.Image == nil {
		return def
	}
	out := *c.Image
	if out.Provider == "" {
		out.Provider = def.Provider
	}
	if out.Model == "" {
		out.Model = def.Model
	}
	// Normalize friendly aliases to API ids.
	switch strings.ToLower(out.Model) {
	case "nano-banana-2", "nano_banana_2", "nano-banana", "nanobanana2", "gemini-image":
		out.Model = DefaultImageModel
	case "nano-banana-2-lite", "nano_banana_2_lite", "nanobanana2-lite":
		out.Model = "gemini-3.1-flash-lite-image"
	}
	return out
}

// GetOllamaLLMConfig returns Ollama chat settings with defaults.
func (c *UserConfig) GetOllamaLLMConfig() OllamaLLMConfig {
	def := OllamaLLMConfig{
		Endpoint: "http://127.0.0.1:11434",
		Model:    "gemma4:12b",
	}
	if c == nil || c.Ollama == nil {
		// Fall back to embedding endpoint if present so one Ollama host is shared.
		if c != nil && c.Embedding != nil && c.Embedding.OllamaEndpoint != "" {
			def.Endpoint = c.Embedding.OllamaEndpoint
		}
		return def
	}
	out := *c.Ollama
	if out.Endpoint == "" {
		if c.Embedding != nil && c.Embedding.OllamaEndpoint != "" {
			out.Endpoint = c.Embedding.OllamaEndpoint
		} else {
			out.Endpoint = def.Endpoint
		}
	}
	if out.Model == "" {
		out.Model = def.Model
	}
	return out
}

// GetWorkerLLMConfig returns the secondary worker LLM config, or nil if unset.
func (c *UserConfig) GetWorkerLLMConfig() *WorkerLLMConfig {
	if c == nil {
		return nil
	}
	return c.resolveSecondarySlot(c.Worker)
}

// GetPlannerLLMConfig returns the high-reasoning planner LLM config, or nil if
// unset. A planner slot pointing at the same provider+model as the worker is
// treated as unset, since a distinct client would buy nothing but a second
// connection pool.
func (c *UserConfig) GetPlannerLLMConfig() *PlannerLLMConfig {
	if c == nil {
		return nil
	}
	p := c.resolveSecondarySlot(c.Planner)
	if p == nil {
		return nil
	}
	if w := c.resolveSecondarySlot(c.Worker); w != nil && *w == *p {
		return nil
	}
	return p
}

// resolveSecondarySlot applies Ollama's endpoint/model defaults to a slot and
// treats a provider-less slot as absent.
func (c *UserConfig) resolveSecondarySlot(slot *SecondaryLLMConfig) *SecondaryLLMConfig {
	if slot == nil || slot.Provider == "" {
		return nil
	}
	out := *slot
	if out.Provider == "ollama" {
		ollama := c.GetOllamaLLMConfig()
		if out.Endpoint == "" {
			out.Endpoint = ollama.Endpoint
		}
		if out.Model == "" {
			out.Model = ollama.Model
		}
	}
	return &out
}

// APIKeyForProvider returns the configured key for a named provider, without
// consulting c.Provider. Use this when resolving a secondary slot (worker,
// image, classification) whose provider may differ from the main one.
//
// Ollama is keyless and returns a non-empty sentinel so callers that gate on
// "key present" behave uniformly across providers.
func (c *UserConfig) APIKeyForProvider(provider string) string {
	if c == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		return c.AnthropicAPIKey
	case "openai":
		return c.OpenAIAPIKey
	case "gemini":
		return c.GeminiAPIKey
	case "xai":
		return c.XAIAPIKey
	case "zai":
		return c.ZAIAPIKey
	case "openrouter":
		return c.OpenRouterAPIKey
	case "dashscope":
		return c.DashScopeAPIKey
	case "meta":
		return c.MetaAPIKey
	case "moonshot":
		return c.MoonshotAPIKey
	case "ollama":
		return "ollama"
	default:
		return ""
	}
}

// SetAPIKeyForProvider updates the root-level key used by a named main
// provider. Secondary slots such as worker and planner remain independent:
// their provider selection must not be silently changed by a main-client CLI
// override.
func (c *UserConfig) SetAPIKeyForProvider(provider, apiKey string) error {
	if c == nil {
		return fmt.Errorf("set API key: nil user config")
	}

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "":
		// Preserve the legacy no-provider behavior. GetActiveProvider interprets
		// APIKey as Z.AI only when no explicit provider is configured.
		c.APIKey = apiKey
	case "anthropic":
		c.AnthropicAPIKey = apiKey
	case "openai":
		c.OpenAIAPIKey = apiKey
	case "gemini":
		c.GeminiAPIKey = apiKey
	case "xai":
		c.XAIAPIKey = apiKey
	case "zai":
		c.ZAIAPIKey = apiKey
	case "openrouter":
		c.OpenRouterAPIKey = apiKey
	case "dashscope":
		c.DashScopeAPIKey = apiKey
	case "meta":
		c.MetaAPIKey = apiKey
	case "moonshot":
		c.MoonshotAPIKey = apiKey
	case "ollama":
		return fmt.Errorf("provider %q is keyless", provider)
	default:
		return fmt.Errorf("unsupported provider %q", provider)
	}
	return nil
}

// GetActiveProvider returns the provider and API key to use.
//
// Config is boss: if c.Provider is explicitly set, ONLY that provider's key is
// considered. If the matching key is missing, this returns (c.Provider, "")
// so callers can fail loudly. Silent fallback to a different provider would
// violate the user's explicit configuration.
//
// Priority when c.Provider is unset: first available key in priority order.
// Ollama is keyless — GetActiveProvider returns ("ollama", "ollama") when
// provider is explicitly set to ollama.
func (c *UserConfig) GetActiveProvider() (provider string, apiKey string) {
	// If provider is explicitly set, ONLY that provider's key is considered.
	// No silent fallback to another provider — that would violate config-is-boss.
	if c.Provider != "" {
		switch c.Provider {
		case "anthropic":
			return "anthropic", c.AnthropicAPIKey
		case "openai":
			return "openai", c.OpenAIAPIKey
		case "gemini":
			return "gemini", c.GeminiAPIKey
		case "xai":
			return "xai", c.XAIAPIKey
		case "zai":
			return "zai", c.ZAIAPIKey
		case "openrouter":
			return "openrouter", c.OpenRouterAPIKey
		case "dashscope":
			return "dashscope", c.DashScopeAPIKey
		case "meta":
			return "meta", c.MetaAPIKey
		case "moonshot":
			return "moonshot", c.MoonshotAPIKey
		case "ollama":
			// Local Ollama: no cloud key; non-empty sentinel satisfies callers
			// that check apiKey != "" before NewClientFromConfig.
			return "ollama", "ollama"
		default:
			return c.Provider, ""
		}
	}

	// Check for provider-specific keys in priority order

	if c.AnthropicAPIKey != "" {
		return "anthropic", c.AnthropicAPIKey
	}
	if c.OpenAIAPIKey != "" {
		return "openai", c.OpenAIAPIKey
	}
	if c.GeminiAPIKey != "" {
		return "gemini", c.GeminiAPIKey
	}
	if c.XAIAPIKey != "" {
		return "xai", c.XAIAPIKey
	}
	if c.ZAIAPIKey != "" {
		return "zai", c.ZAIAPIKey
	}
	if c.OpenRouterAPIKey != "" {
		return "openrouter", c.OpenRouterAPIKey
	}
	if c.DashScopeAPIKey != "" {
		return "dashscope", c.DashScopeAPIKey
	}
	if c.MetaAPIKey != "" {
		return "meta", c.MetaAPIKey
	}
	if c.MoonshotAPIKey != "" {
		return "moonshot", c.MoonshotAPIKey
	}

	// Legacy: single api_key field (assume zai for backward compatibility)
	if c.APIKey != "" {
		return "zai", c.APIKey
	}

	return "", ""
}

// GetEngine returns the configured engine, defaulting to "api".
func (c *UserConfig) GetEngine() string {
	if c.Engine == "" {
		return "api"
	}
	return c.Engine
}

// HasExplicitLLMSelection reports whether config.json expresses an LLM routing
// choice. When true, client construction errors must not fall through to an
// unrelated ambient provider key.
func (c *UserConfig) HasExplicitLLMSelection() bool {
	if c == nil {
		return false
	}
	return c.Engine != "" || c.Provider != "" || c.APIKey != "" ||
		c.AnthropicAPIKey != "" || c.OpenAIAPIKey != "" || c.GeminiAPIKey != "" ||
		c.XAIAPIKey != "" || c.ZAIAPIKey != "" || c.OpenRouterAPIKey != "" ||
		c.DashScopeAPIKey != "" || c.MetaAPIKey != "" || c.MoonshotAPIKey != "" ||
		c.ClaudeCLI != nil || c.CodexCLI != nil || c.XAIOAuth != nil || c.Ollama != nil
}

// SetEngine updates the engine setting.
func (c *UserConfig) SetEngine(engine string) error {
	validEngines := map[string]bool{
		"api":        true,
		"claude-cli": true,
		"codex-cli":  true,
		"xai-oauth":  true,
	}
	if !validEngines[engine] {
		return fmt.Errorf("invalid engine: %s (valid: api, claude-cli, codex-cli, xai-oauth)", engine)
	}
	c.Engine = engine
	return nil
}

// GetClaudeCLIConfig returns Claude CLI config with defaults applied.
func (c *UserConfig) GetClaudeCLIConfig() *ClaudeCLIConfig {
	if c.ClaudeCLI == nil {
		return &ClaudeCLIConfig{
			Model:   "sonnet",
			Timeout: 300,
		}
	}
	cfg := *c.ClaudeCLI
	if cfg.Model == "" {
		cfg.Model = "sonnet"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300
	}
	return &cfg
}

// GetCodexCLIConfig returns Codex CLI config with defaults applied.
func (c *UserConfig) GetCodexCLIConfig() *CodexCLIConfig {
	if c.CodexCLI == nil {
		disableShell := true
		enableSchema := true
		skillEnabled := true
		return &CodexCLIConfig{
			Model:              "gpt-5.4",
			Sandbox:            "read-only",
			Timeout:            300,
			SkillEnabled:       &skillEnabled,
			SkillName:          DefaultCodexExecSkillName,
			MaxConcurrentCalls: DefaultCodexMaxConcurrentCalls,
			DisableShellTool:   &disableShell,
			EnableOutputSchema: &enableSchema,
		}
	}
	cfg := *c.CodexCLI
	if cfg.Model == "" {
		cfg.Model = "gpt-5.4"
	}
	// Codex CLI is a completion backend, never an effect executor. Force both
	// controls even when an older or hand-edited config requests otherwise.
	cfg.Sandbox = "read-only"
	if cfg.Timeout == 0 {
		cfg.Timeout = 300
	}
	if cfg.SkillEnabled == nil {
		skillEnabled := true
		cfg.SkillEnabled = &skillEnabled
	}
	if cfg.SkillName == "" {
		cfg.SkillName = DefaultCodexExecSkillName
	}
	if cfg.MaxConcurrentCalls == 0 {
		cfg.MaxConcurrentCalls = DefaultCodexMaxConcurrentCalls
	}
	disableShell := true
	cfg.DisableShellTool = &disableShell
	if cfg.EnableOutputSchema == nil {
		enableSchema := true
		cfg.EnableOutputSchema = &enableSchema
	}
	return &cfg
}

// GetXAIOAuthConfig returns SuperGrok OAuth config with defaults applied.
func (c *UserConfig) GetXAIOAuthConfig() *XAIOAuthConfig {
	importGrok := true
	fallbackAPI := true
	if c.XAIOAuth == nil {
		return &XAIOAuthConfig{
			Model:              "grok-4.5",
			Timeout:            300,
			ImportGrokAuth:     &importGrok,
			FallbackToAPIKey:   &fallbackAPI,
			MaxConcurrentCalls: DefaultXAIOAuthMaxConcurrentCalls,
		}
	}
	cfg := *c.XAIOAuth
	if cfg.Model == "" {
		cfg.Model = "grok-4.5"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300
	}
	if cfg.ImportGrokAuth == nil {
		cfg.ImportGrokAuth = &importGrok
	}
	if cfg.FallbackToAPIKey == nil {
		cfg.FallbackToAPIKey = &fallbackAPI
	}
	if cfg.MaxConcurrentCalls == 0 {
		cfg.MaxConcurrentCalls = DefaultXAIOAuthMaxConcurrentCalls
	}
	return &cfg
}

// GetEffectiveMaxConcurrentAPICalls returns the scheduler ceiling after applying
// engine-specific concurrency overrides.
func (c *UserConfig) GetEffectiveMaxConcurrentAPICalls() int {
	coreLimits := c.GetCoreLimits()
	effective := coreLimits.MaxConcurrentAPICalls

	if c.GetEngine() == "codex-cli" {
		codexCfg := c.GetCodexCLIConfig()
		if codexCfg.MaxConcurrentCalls > 0 && codexCfg.MaxConcurrentCalls < effective {
			effective = codexCfg.MaxConcurrentCalls
		}
	}
	if c.GetEngine() == "xai-oauth" {
		oauthCfg := c.GetXAIOAuthConfig()
		if oauthCfg.MaxConcurrentCalls > 0 && oauthCfg.MaxConcurrentCalls < effective {
			effective = oauthCfg.MaxConcurrentCalls
		}
	}
	// Optional Claude CLI ceiling (same pattern as Codex/SuperGrok).
	if c.GetEngine() == "claude-cli" && c.ClaudeCLI != nil && c.ClaudeCLI.MaxConcurrentCalls > 0 {
		if c.ClaudeCLI.MaxConcurrentCalls < effective {
			effective = c.ClaudeCLI.MaxConcurrentCalls
		}
	}

	return effective
}

// subscriptionEngine reports engines that default to polite SuperGrok-style
// scheduling (spacing + adaptive concurrency).
func (c *UserConfig) subscriptionEngine() bool {
	switch c.GetEngine() {
	case "xai-oauth", "codex-cli", "claude-cli":
		return true
	default:
		return false
	}
}

// GetEffectiveAPISchedulerPolicy resolves api_scheduler config with engine
// defaults. Explicit config.json values always win over defaults.
func (c *UserConfig) GetEffectiveAPISchedulerPolicy() EffectiveAPISchedulerPolicy {
	pol := EffectiveAPISchedulerPolicy{
		MaxConcurrentAPICalls: c.GetEffectiveMaxConcurrentAPICalls(),
		AdaptiveFloor:         DefaultAdaptiveFloor,
		AdaptiveRecoverAfter:  time.Duration(DefaultAdaptiveRecoverAfterSec) * time.Second,
		SlotAcquireTimeout:    GetLLMTimeouts().SlotAcquisitionTimeout,
	}
	if pol.SlotAcquireTimeout <= 0 {
		pol.SlotAcquireTimeout = time.Duration(DefaultSlotAcquireTimeoutSec) * time.Second
	}

	// Engine defaults: subscription engines are polite; API is aggressive.
	if c.subscriptionEngine() {
		pol.MinCallSpacing = time.Duration(DefaultSubscriptionMinCallSpacingMs) * time.Millisecond
		pol.AdaptiveConcurrency = true
	} else {
		pol.MinCallSpacing = 0
		pol.AdaptiveConcurrency = false
	}

	if c == nil || c.APIScheduler == nil {
		return pol
	}
	p := c.APIScheduler

	if p.MinCallSpacingMs != nil {
		if *p.MinCallSpacingMs <= 0 {
			pol.MinCallSpacing = 0
		} else {
			pol.MinCallSpacing = time.Duration(*p.MinCallSpacingMs) * time.Millisecond
		}
	}
	if p.AdaptiveConcurrency != nil {
		pol.AdaptiveConcurrency = *p.AdaptiveConcurrency
	}
	if p.AdaptiveFloor != nil && *p.AdaptiveFloor > 0 {
		pol.AdaptiveFloor = *p.AdaptiveFloor
	}
	if p.AdaptiveRecoverAfterSec != nil {
		if *p.AdaptiveRecoverAfterSec <= 0 {
			pol.AdaptiveRecoverAfter = 0
		} else {
			pol.AdaptiveRecoverAfter = time.Duration(*p.AdaptiveRecoverAfterSec) * time.Second
		}
	}
	if p.SlotAcquireTimeoutSec != nil && *p.SlotAcquireTimeoutSec > 0 {
		pol.SlotAcquireTimeout = time.Duration(*p.SlotAcquireTimeoutSec) * time.Second
	}

	return pol
}

// GetShardProfile returns the profile for a specific shard type, falling back to defaults.
func (c *UserConfig) GetShardProfile(shardType string) ShardProfile {
	// Check for explicit profile
	if c.ShardProfiles != nil {
		if profile, ok := c.ShardProfiles[shardType]; ok {
			return applyShardDefaults(profile)
		}
	}

	// Use default shard settings if available
	if c.DefaultShard != nil {
		return applyShardDefaults(*c.DefaultShard)
	}

	// Ultimate fallback - sensible defaults
	return ShardProfile{
		Model:                 "glm-4.7",
		Temperature:           0.7,
		TopP:                  0.9,
		MaxContextTokens:      20000,
		MaxOutputTokens:       4000,
		MaxExecutionTimeSec:   300,
		MaxRetries:            3,
		MaxFactsInShardKernel: 20000,
		EnableLearning:        true,
	}
}

// GetCoreLimits returns core resource limits with defaults applied.
func (c *UserConfig) GetCoreLimits() CoreLimits {
	if c.CoreLimits != nil {
		limits := *c.CoreLimits
		// Apply defaults for zero values
		if limits.MaxTotalMemoryMB == 0 {
			limits.MaxTotalMemoryMB = 12288
		}
		if limits.MaxConcurrentShards == 0 {
			limits.MaxConcurrentShards = 12
		}
		if limits.MaxConcurrentAPICalls == 0 {
			limits.MaxConcurrentAPICalls = 5
		}
		if limits.MaxSessionDurationMin == 0 {
			limits.MaxSessionDurationMin = 120
		}
		if limits.MaxFactsInKernel == 0 {
			limits.MaxFactsInKernel = 250000
		}
		if limits.MaxDerivedFactsLimit == 0 {
			limits.MaxDerivedFactsLimit = 100000
		}
		return limits
	}
	// Return defaults
	return CoreLimits{
		MaxTotalMemoryMB:      12288,
		MaxConcurrentShards:   12,
		MaxConcurrentAPICalls: 5,
		MaxSessionDurationMin: 120,
		MaxFactsInKernel:      250000,
		MaxDerivedFactsLimit:  100000,
	}
}

// GetWorldConfig returns world-model scanning settings with defaults.
func (c *UserConfig) GetWorldConfig() WorldConfig {
	def := DefaultWorldConfig()
	if c != nil && c.World != nil {
		cfg := *c.World
		if cfg.FastWorkers <= 0 {
			cfg.FastWorkers = def.FastWorkers
		}
		if cfg.DeepWorkers <= 0 {
			cfg.DeepWorkers = def.DeepWorkers
		}
		if len(cfg.IgnorePatterns) == 0 {
			cfg.IgnorePatterns = def.IgnorePatterns
		}
		if cfg.MaxFastASTBytes <= 0 {
			cfg.MaxFastASTBytes = def.MaxFastASTBytes
		}
		return cfg
	}
	return def
}

// GetIntegrations returns integration settings with defaults.
// By default, no external MCP servers are configured.
// Internal capabilities (code analysis, browser automation) use internal packages directly.
func (c *UserConfig) GetIntegrations() IntegrationsConfig {
	if c.Integrations != nil {
		return *c.Integrations
	}
	// Return empty - no default MCP servers. User configures external servers as needed.
	return IntegrationsConfig{
		Servers: make(map[string]MCPServerIntegration),
	}
}

// BrowserAutomationConfig controls codeNERD's native Rod browser manager.
type BrowserAutomationConfig struct {
	DebuggerURL          string   `json:"debugger_url,omitempty"`
	Launch               []string `json:"launch,omitempty"`
	Headless             bool     `json:"headless,omitempty"`
	ViewportWidth        int      `json:"viewport_width,omitempty"`
	ViewportHeight       int      `json:"viewport_height,omitempty"`
	NavigationTimeoutMs  int      `json:"navigation_timeout_ms,omitempty"`
	MultiTabDefault      *bool    `json:"multi_tab_default,omitempty"`
	MaxTabs              int      `json:"max_tabs,omitempty"`
	MaxBrowsers          int      `json:"max_browsers,omitempty"`
	IdleTabTimeoutMs     int      `json:"idle_tab_timeout_ms,omitempty"`
	ExtraSensitiveKeys   []string `json:"extra_sensitive_keys,omitempty"`
	WritableRoots        []string `json:"writable_roots,omitempty"`
	EvidenceEnabled      *bool    `json:"evidence_enabled,omitempty"`
	EvidenceDir          string   `json:"evidence_dir,omitempty"`
	MaxEvidenceFiles     int      `json:"max_evidence_files,omitempty"`
	MaxEvidenceFileBytes int64    `json:"max_evidence_file_bytes,omitempty"`
}

// DefaultBrowserAutomationConfig returns BrowserNERD-compatible lifecycle defaults.
func DefaultBrowserAutomationConfig() BrowserAutomationConfig {
	sharedTabs := true
	return BrowserAutomationConfig{
		ViewportWidth:        1920,
		ViewportHeight:       1080,
		NavigationTimeoutMs:  30000,
		MultiTabDefault:      &sharedTabs,
		MaxTabs:              32,
		MaxBrowsers:          4,
		EvidenceEnabled:      boolConfigPointer(true),
		MaxEvidenceFiles:     16,
		MaxEvidenceFileBytes: 4 << 20,
	}
}

func boolConfigPointer(value bool) *bool { return &value }

// GetBrowserConfig returns native browser settings with limits normalized.
func (c *UserConfig) GetBrowserConfig() BrowserAutomationConfig {
	defaults := DefaultBrowserAutomationConfig()
	if c == nil || c.Browser == nil {
		return defaults
	}
	cfg := *c.Browser
	if cfg.ViewportWidth <= 0 {
		cfg.ViewportWidth = defaults.ViewportWidth
	}
	if cfg.ViewportHeight <= 0 {
		cfg.ViewportHeight = defaults.ViewportHeight
	}
	if cfg.NavigationTimeoutMs <= 0 {
		cfg.NavigationTimeoutMs = defaults.NavigationTimeoutMs
	}
	if cfg.MultiTabDefault == nil {
		cfg.MultiTabDefault = defaults.MultiTabDefault
	}
	if cfg.MaxTabs <= 0 {
		cfg.MaxTabs = defaults.MaxTabs
	}
	if cfg.MaxBrowsers <= 0 {
		cfg.MaxBrowsers = defaults.MaxBrowsers
	}
	if cfg.IdleTabTimeoutMs < 0 {
		cfg.IdleTabTimeoutMs = 0
	}
	if cfg.EvidenceEnabled == nil {
		cfg.EvidenceEnabled = defaults.EvidenceEnabled
	}
	if cfg.MaxEvidenceFiles <= 0 {
		cfg.MaxEvidenceFiles = defaults.MaxEvidenceFiles
	} else if cfg.MaxEvidenceFiles > 256 {
		cfg.MaxEvidenceFiles = 256
	}
	if cfg.MaxEvidenceFileBytes <= 0 {
		cfg.MaxEvidenceFileBytes = defaults.MaxEvidenceFileBytes
	} else if cfg.MaxEvidenceFileBytes > 64<<20 {
		cfg.MaxEvidenceFileBytes = 64 << 20
	}
	return cfg
}

// GetExecution returns execution settings with defaults.
func (c *UserConfig) GetExecution() ExecutionConfig {
	if c.Execution != nil {
		cfg := *c.Execution
		if cfg.DefaultTimeout == "" {
			cfg.DefaultTimeout = "30s"
		}
		if cfg.WorkingDirectory == "" {
			cfg.WorkingDirectory = "."
		}
		if len(cfg.AllowedBinaries) == 0 {
			cfg.AllowedBinaries = []string{
				"go", "git", "grep", "ls", "mkdir", "cp", "mv",
				"npm", "npx", "node", "python", "python3", "pip",
				"cargo", "rustc", "make", "cmake",
			}
		}
		if len(cfg.AllowedEnvVars) == 0 {
			cfg.AllowedEnvVars = []string{"PATH", "HOME", "GOPATH", "GOROOT"}
		}
		return cfg
	}
	return ExecutionConfig{
		AllowedBinaries: []string{
			"go", "git", "grep", "ls", "mkdir", "cp", "mv",
			"npm", "npx", "node", "python", "python3", "pip",
			"cargo", "rustc", "make", "cmake",
		},
		DefaultTimeout:   "30s",
		WorkingDirectory: ".",
		AllowedEnvVars:   []string{"PATH", "HOME", "GOPATH", "GOROOT"},
	}
}

// GetLogging returns logging settings with defaults.
func (c *UserConfig) GetLogging() LoggingConfig {
	if c.Logging != nil {
		cfg := *c.Logging
		if cfg.Level == "" {
			cfg.Level = "info"
		}
		if cfg.Format == "" {
			cfg.Format = "text"
		}
		// Note: DebugMode defaults to false (production mode) unless explicitly set
		return cfg
	}
	return LoggingConfig{
		Level:      "info",
		Format:     "text",
		File:       "codenerd.log",
		DebugMode:  false, // Production mode by default
		TraceLLMIO: false,
	}
}

// GetLearningCandidateThreshold returns the candidate threshold with defaults applied.
func (c *UserConfig) GetLearningCandidateThreshold() int {
	if c != nil && c.LearningCandidateThreshold > 0 {
		return c.LearningCandidateThreshold
	}
	return 3
}

// GetLearningCandidateAutoPromote returns whether candidates auto-promote.
func (c *UserConfig) GetLearningCandidateAutoPromote() bool {
	if c != nil {
		return c.LearningCandidateAutoPromote
	}
	return false
}

// DefaultUserConfig returns a UserConfig with sensible defaults.
func DefaultUserConfig() *UserConfig {
	featuresCfg := features.FullyEnabledFeaturesConfig()
	w := DefaultWorldConfig()
	jit := DefaultJITConfig()
	build := DefaultBuildConfig()
	toolGen := DefaultToolGenerationConfig()
	ctxWin := DefaultContextWindowConfig()
	reflection := DefaultReflectionConfig()
	browserCfg := DefaultBrowserAutomationConfig()

	return &UserConfig{
		Provider:                     "zai",
		Model:                        "glm-4.7",
		Engine:                       "api",
		Theme:                        "light",
		ContinuationMode:             1,
		Gemini:                       DefaultGeminiProviderConfig(),
		ClaudeCLI:                    DefaultClaudeCLIConfig(),
		CodexCLI:                     DefaultCodexCLIConfig(),
		ContextWindow:                &ctxWin,
		Embedding:                    DefaultEmbeddingConfig(),
		Reflection:                   &reflection,
		ShardProfiles:                DefaultShardProfiles(),
		DefaultShard:                 DefaultShardProfile(),
		CoreLimits:                   DefaultCoreLimits(),
		World:                        &w,
		Integrations:                 DefaultIntegrationsConfig(),
		Browser:                      &browserCfg,
		ToolGeneration:               &toolGen,
		Build:                        &build,
		Execution:                    DefaultExecutionConfig(),
		Logging:                      DefaultLoggingConfig(),
		JIT:                          &jit,
		LearningCandidateThreshold:   3,
		LearningCandidateAutoPromote: false,
		Onboarding:                   DefaultOnboardingState(),
		Transparency:                 DefaultTransparencyConfig(),
		Guidance:                     DefaultGuidanceConfig(),
		Features:                     &featuresCfg,
	}
}

// GlobalConfig is a convenience function to load config from the default path.
// Returns an empty config (with defaults available via Get* methods) if file doesn't exist.
func GlobalConfig() (*UserConfig, error) {
	return LoadUserConfig(DefaultUserConfigPath())
}

// GetContext7APIKey returns the Context7 API key with auto-detection.
// Priority order:
//  1. CONTEXT7_API_KEY environment variable
//  2. UserConfig.Context7APIKey from .nerd/config.json
//
// Returns empty string if not configured.
func (c *UserConfig) GetContext7APIKey() string {
	// Priority 1: Environment variable
	if key := os.Getenv("CONTEXT7_API_KEY"); key != "" {
		return key
	}

	// Priority 2: Config file value
	if c != nil && c.Context7APIKey != "" {
		return c.Context7APIKey
	}

	return ""
}

// AutoDetectContext7APIKey is a convenience function to get the Context7 API key
// from environment variables or the default config file.
// This is useful for initializing with auto-detection when UserConfig may not be loaded.
func AutoDetectContext7APIKey() string {
	// Priority 1: Environment variable
	if key := os.Getenv("CONTEXT7_API_KEY"); key != "" {
		return key
	}

	// Priority 2: Load from config file
	cfg, err := GlobalConfig()
	if err == nil && cfg != nil && cfg.Context7APIKey != "" {
		return cfg.Context7APIKey
	}

	return ""
}

// GetJITConfig returns JIT Prompt Compiler config with defaults applied.
func (c *UserConfig) GetJITConfig() JITConfig {
	cfg := DefaultJITConfig()
	if c.JIT != nil {
		if c.JIT.enabledSet {
			cfg.Enabled = c.JIT.Enabled
		}
		if c.JIT.fallbackEnabledSet {
			cfg.FallbackEnabled = c.JIT.FallbackEnabled
		}
		if c.JIT.TokenBudget != 0 {
			cfg.TokenBudget = c.JIT.TokenBudget
		}
		if c.JIT.ReservedTokens != 0 {
			cfg.ReservedTokens = c.JIT.ReservedTokens
		}
		if c.JIT.ReservedTokensFallbackRatio != 0 {
			cfg.ReservedTokensFallbackRatio = c.JIT.ReservedTokensFallbackRatio
		}
		cfg.DebugMode = c.JIT.DebugMode
		cfg.TraceLLMIO = c.JIT.TraceLLMIO
		if c.JIT.SemanticTopK != 0 {
			cfg.SemanticTopK = c.JIT.SemanticTopK
		}
	}
	return cfg
}

// GetEffectiveJITConfig returns JIT config clamped to the context window.
func (c *UserConfig) GetEffectiveJITConfig() JITConfig {
	cfg := c.GetJITConfig()
	ctxCfg := c.GetContextWindowConfig()

	if ctxCfg.MaxTokens > 0 && cfg.TokenBudget > ctxCfg.MaxTokens {
		cfg.TokenBudget = ctxCfg.MaxTokens
	}

	if cfg.TokenBudget > 0 && cfg.ReservedTokens >= cfg.TokenBudget {
		fallbackRatio := cfg.ReservedTokensFallbackRatio
		if fallbackRatio <= 0 {
			fallbackRatio = 10
		}
		cfg.ReservedTokens = cfg.TokenBudget / fallbackRatio
		if cfg.ReservedTokens >= cfg.TokenBudget {
			cfg.ReservedTokens = 0
		}
	}

	return cfg
}

// GetOnboardingState returns the onboarding state with defaults applied.
func (c *UserConfig) GetOnboardingState() *OnboardingState {
	if c.Onboarding != nil {
		return c.Onboarding
	}
	return DefaultOnboardingState()
}

// GetTransparencyConfig returns the transparency config with defaults applied.
func (c *UserConfig) GetTransparencyConfig() *TransparencyConfig {
	if c.Transparency != nil {
		return c.Transparency
	}
	return DefaultTransparencyConfig()
}

// GetGuidanceConfig returns the guidance config with defaults applied.
func (c *UserConfig) GetGuidanceConfig() *GuidanceConfig {
	if c.Guidance != nil {
		return c.Guidance
	}
	return DefaultGuidanceConfig()
}

// GetGeminiConfig returns the Gemini provider config with defaults applied.
// Defaults to "high" thinking level (dynamic reasoning) with Google Search and URL Context enabled.
// Thinking levels: "minimal", "low", "medium", "high"
func (c *UserConfig) GetGeminiConfig() *GeminiProviderConfig {
	if c.Gemini != nil {
		cfg := *c.Gemini
		// Apply defaults for thinking level if thinking is enabled but level not set
		if cfg.EnableThinking && cfg.ThinkingLevel == "" {
			cfg.ThinkingLevel = "high" // Dynamic reasoning - maximizes reasoning depth
		}
		return &cfg
	}
	return DefaultGeminiProviderConfig()
}

// IsOnboardingComplete returns true if the user has completed onboarding.
func (c *UserConfig) IsOnboardingComplete() bool {
	if c.Onboarding == nil {
		return false
	}
	return c.Onboarding.SetupComplete
}

// GetExperienceLevel returns the user's experience level.
func (c *UserConfig) GetExperienceLevel() ExperienceLevel {
	if c.Onboarding != nil && c.Onboarding.ExperienceLevel != "" {
		return c.Onboarding.ExperienceLevel
	}
	return ExperienceBeginner
}

// ShouldShowTransparency returns true if any transparency feature is enabled.
func (c *UserConfig) ShouldShowTransparency() bool {
	if c.Transparency == nil {
		return false
	}
	return c.Transparency.Enabled
}
