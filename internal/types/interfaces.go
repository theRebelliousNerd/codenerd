package types

import (
	"context"
)

// Kernel defines the interface for the logic core.
type Kernel interface {
	LoadFacts(facts []Fact) error
	Query(predicate string) ([]Fact, error)
	QueryAll() (map[string][]Fact, error)
	Assert(fact Fact) error
	AssertBatch(facts []Fact) error
	Retract(predicate string) error
	RetractFact(fact Fact) error
	// UpdateSystemFacts updates system facts (time, etc.)
	UpdateSystemFacts() error

	// Power-user features for advanced kernel control
	// Reset clears all facts while keeping schemas and policies
	Reset()
	// AppendPolicy adds shard-specific policy rules to the kernel
	AppendPolicy(policy string)

	// Optimized batch operations (required by world scanner)
	RetractExactFactsBatch(facts []Fact) error
	RemoveFactsByPredicateSet(predicates map[string]struct{}) error
}

// LLMClient defines the interface for LLM interactions.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	// CompleteWithTools sends a prompt with tool definitions and returns response with tool calls.
	// This enables agentic behavior where the LLM can invoke tools to complete tasks.
	CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error)
}

// ToolDefinition describes a tool that the LLM can invoke.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"` // JSON Schema for parameters
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID    string                 `json:"id"`    // Unique ID for this tool use
	Name  string                 `json:"name"`  // Tool name to invoke
	Input map[string]interface{} `json:"input"` // Tool arguments
}

// UsageMetadata captures token usage metrics from the LLM.
type UsageMetadata struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	TotalTokens         int `json:"total_tokens"`
	ThinkingTokens      int `json:"thinking_tokens,omitempty"`       // Subset of OutputTokens used for thinking
	CachedContentTokens int `json:"cached_content_tokens,omitempty"` // Tokens read from context cache
}

// LLMToolResponse contains both text response and tool calls from the LLM.
type LLMToolResponse struct {
	Text       string        `json:"text"`        // Text response (may be empty if only tool calls)
	ToolCalls  []ToolCall    `json:"tool_calls"`  // Tool invocations requested by LLM
	StopReason string        `json:"stop_reason"` // "end_turn", "tool_use", etc.
	Usage      UsageMetadata `json:"usage"`       // Token usage metrics

	// Gemini Thinking Mode metadata (for learning and improvement)
	// ThoughtSummary captures the model's reasoning process for post-hoc analysis
	ThoughtSummary string `json:"thought_summary,omitempty"`
	// ThoughtSignature is an encrypted blob for multi-turn function calling (Gemini 3)
	// Must be passed back in subsequent turns for reasoning continuity
	ThoughtSignature string `json:"thought_signature,omitempty"`
	// ThinkingTokens tracks tokens used for reasoning (for budget monitoring)
	// Deprecated: Use Usage.ThinkingTokens instead
	ThinkingTokens int `json:"thinking_tokens,omitempty"`

	// Grounding metadata (from Google Search / URL Context)
	// GroundingSources lists URLs used to ground the response
	GroundingSources []string `json:"grounding_sources,omitempty"`
}

// ShardAgent defines the interface for all agents.
// Renamed from 'Shard' to match usage in registration.go.
type ShardAgent interface {
	Execute(ctx context.Context, task string) (string, error)
	GetID() string
	GetState() ShardState
	GetConfig() ShardConfig
	Stop() error

	// Dependency Injection methods
	SetParentKernel(k Kernel)
	SetLLMClient(client LLMClient)
	SetSessionContext(ctx *SessionContext) // For dream mode and session state
}

// ShardFactory is a function that creates a new shard instance.
type ShardFactory func(id string, config ShardConfig) ShardAgent

// PromptLoaderFunc is a callback for loading agent prompts from YAML files.
type PromptLoaderFunc func(context.Context, string, string) (int, error)

// JITDBRegistrar is a callback for registering agent knowledge DBs with the JIT prompt compiler.
type JITDBRegistrar func(agentName string, dbPath string) error

// JITDBUnregistrar is a callback for unregistering agent knowledge DBs from the JIT prompt compiler.
type JITDBUnregistrar func(agentName string)

// ReviewerFeedbackProvider defines the interface for reviewer validation.
type ReviewerFeedbackProvider interface {
	NeedsValidation(reviewID string) bool
	GetSuspectReasons(reviewID string) []string
	AcceptFinding(reviewID, file string, line int)
	RejectFinding(reviewID, file string, line int, reason string)
	GetAccuracyReport(reviewID string) string
}

// LimitsEnforcer defines the interface for resource limits.
type LimitsEnforcer interface {
	CheckShardLimit(activeCount int) error
	CheckMemory() error
	GetAvailableShardSlots(activeCount int) int
}

// ShardLearning represents a learned pattern or preference.
type ShardLearning struct {
	FactPredicate string  `json:"fact_predicate"`
	FactArgs      []any   `json:"fact_args"`
	Confidence    float64 `json:"confidence"`
	Timestamp     int64   `json:"timestamp"`
}

// LearningStore defines the interface for persisting learned patterns.
// Used by autopoiesis.
type LearningStore interface {
	Save(shardID, predicate string, args []any, source string) error
	LoadByPredicate(shardID, predicate string) ([]ShardLearning, error)
}

// VirtualStore defines the interface for the virtual filesystem and execution environment.
// This is a marker interface to break import cycles; implementation is *core.VirtualStore.
type VirtualStore interface {
	// Methods required by shards (expand as needed)
	ReadFile(path string) ([]string, error)
	WriteFile(path string, content []string) error
	Exec(ctx context.Context, cmd string, env []string) (string, string, error)
	// ReadRaw reads a file and returns its raw bytes (for YAML/JSON parsing)
	ReadRaw(path string) ([]byte, error)
}

// GraphQuery defines the interface for querying the World Model graph.
// This interface allows Mangle policies (via Virtual Predicates) to access
// the dependency graph, AST, and file topology without direct coupling.
// Moved from world package to break import cycles (core <-> world).
type GraphQuery interface {
	// QueryGraph performs a query against the world graph.
	// queryType: e.g., "dependencies", "symbols", "callers"
	// params: query-specific parameters
	// Returns: structured result (e.g., []string, []Symbol, etc.)
	QueryGraph(queryType string, params map[string]interface{}) (interface{}, error)
}

// GroundingProvider is an optional interface for LLM clients that support
// grounding (Google Search, URL Context, etc.). Use type assertion to check
// if a client supports grounding:
//
//	if gp, ok := client.(types.GroundingProvider); ok {
//	    sources := gp.GetLastGroundingSources()
//	}
type GroundingProvider interface {
	// GetLastGroundingSources returns URLs used to ground the last response.
	// Returns nil if no grounding was used or client doesn't support grounding.
	GetLastGroundingSources() []string

	// IsGoogleSearchEnabled returns whether Google Search grounding is enabled.
	IsGoogleSearchEnabled() bool

	// IsURLContextEnabled returns whether URL Context grounding is enabled.
	IsURLContextEnabled() bool
}

// GroundingController extends GroundingProvider with control methods.
// Use this interface when you need to enable/disable grounding features:
//
//	if gc, ok := client.(types.GroundingController); ok {
//	    gc.SetEnableGoogleSearch(true)
//	    gc.SetURLContextURLs([]string{"https://docs.example.com"})
//	}
type GroundingController interface {
	GroundingProvider

	// SetEnableGoogleSearch enables or disables Google Search grounding.
	SetEnableGoogleSearch(enable bool)

	// SetEnableURLContext enables or disables URL Context grounding.
	SetEnableURLContext(enable bool)

	// SetURLContextURLs sets the URLs for URL Context grounding.
	// Max 20 URLs, 34MB each per Gemini API limits.
	SetURLContextURLs(urls []string)
}

// PiggybackToolProvider is an optional interface for LLM clients that should
// use Piggyback Protocol for tool invocation instead of native function calling.
// This is required for Gemini clients because native function calling conflicts
// with built-in tools (Google Search, URL Context).
//
// Usage:
//
//	if ptp, ok := client.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
//	    // Use structured output with tool_requests in control_packet
//	    // instead of native function calling
//	}
type PiggybackToolProvider interface {
	// ShouldUsePiggybackTools returns true if this client should use
	// Piggyback Protocol for tool invocation instead of native function calling.
	// Typically true for Gemini clients when grounding (Google Search/URL Context) is enabled.
	ShouldUsePiggybackTools() bool
}

// ThinkingProvider is an optional interface for LLM clients that support
// explicit thinking/reasoning mode (Gemini 3 Thinking Mode, Claude extended thinking, etc.).
// Use type assertion to check if a client supports thinking metadata:
//
//	if tp, ok := client.(types.ThinkingProvider); ok {
//	    summary := tp.GetLastThoughtSummary()
//	    tokens := tp.GetLastThinkingTokens()
//	}
//
// This metadata is used by the System Prompt Learning (SPL) system to:
// 1. Evaluate WHY a task succeeded or failed (reasoning quality)
// 2. Learn from the model's decision-making process
// 3. Generate better prompt atoms based on reasoning patterns
type ThinkingProvider interface {
	// GetLastThoughtSummary returns the model's reasoning process from the last call.
	// Returns empty string if thinking mode is disabled or client doesn't support it.
	GetLastThoughtSummary() string

	// GetLastThinkingTokens returns the number of tokens used for reasoning.
	// Returns 0 if thinking mode is disabled or client doesn't support it.
	GetLastThinkingTokens() int

	// IsThinkingEnabled returns whether thinking mode is currently enabled.
	IsThinkingEnabled() bool

	// GetThinkingLevel returns the current thinking level (e.g., "minimal", "low", "medium", "high").
	// Returns empty string if thinking mode uses token budget instead of levels.
	GetThinkingLevel() string
}

// ThoughtSignatureProvider is an optional interface for LLM clients that support
// multi-turn function calling with thought signatures (Gemini 3).
//
// When using function calling with thinking mode enabled, Gemini returns an encrypted
// "thought signature" that must be passed back in subsequent turns to maintain
// reasoning continuity. Without this, the model loses context about WHY it made
// specific function calls.
//
// Usage pattern for multi-turn function calling:
//
//	// First turn: LLM returns tool calls + thought signature
//	resp, _ := client.CompleteWithTools(ctx, system, user, tools)
//
//	// Capture signature after tool response
//	var signature string
//	if tsp, ok := client.(types.ThoughtSignatureProvider); ok {
//	    signature = tsp.GetLastThoughtSignature()
//	}
//
//	// Execute tools, then send results back with signature
//	// (signature should be included in the function response content)
//
// This is critical for agentic workflows where:
// 1. The LLM makes multiple tool calls
// 2. Each tool's result informs subsequent reasoning
// 3. The thinking chain must be preserved across turns
type ThoughtSignatureProvider interface {
	// GetLastThoughtSignature returns the encrypted thought signature from the last response.
	// This signature must be passed back in subsequent function calling turns.
	// Returns empty string if:
	// - Thinking mode is disabled
	// - The last response didn't include tool calls
	// - The client doesn't support thought signatures
	GetLastThoughtSignature() string
}

// FileProvider is an optional interface for LLM clients that support
// file uploads (Gemini Files API, OpenAI Files, etc.).
type FileProvider interface {
	// UploadFile uploads a file to the provider's storage.
	// mimeType is optional (detected if empty).
	// returns file ID (URI) and error.
	UploadFile(ctx context.Context, path string, mimeType string) (string, error)

	// DeleteFile deletes a file from the provider's storage.
	DeleteFile(ctx context.Context, fileID string) error

	// ListFiles lists uploaded files.
	// returns list of file IDs and error.
	ListFiles(ctx context.Context) ([]string, error)

	// GetFile retrieves metadata for an uploaded file.
	GetFile(ctx context.Context, fileID string) (interface{}, error)
}

// CacheProvider is an optional interface for LLM clients that support
// context caching (Gemini Context Caching, Anthropic Message Caching).
type CacheProvider interface {
	// CreateCachedContent creates a cache for the given files/content.
	// files is a list of file IDs/URIs.
	// ttl is the time-to-live seconds.
	// returns cache name (ID) and error.
	CreateCachedContent(ctx context.Context, files []string, ttl int) (string, error)

	// GetCachedContent retrieves metadata for a cached content.
	GetCachedContent(ctx context.Context, cacheName string) (interface{}, error)

	// DeleteCachedContent deletes a context cache.
	DeleteCachedContent(ctx context.Context, cacheName string) error

	// ListCachedContent lists active context caches.
	ListCachedContent(ctx context.Context) ([]string, error)

	// SetCachedContent sets the active cached context for subsequent requests.
	// Pass empty string to disable.
	SetCachedContent(name string)
}
