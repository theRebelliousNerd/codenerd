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

// LLMToolResponse contains both text response and tool calls from the LLM.
type LLMToolResponse struct {
	Text       string     `json:"text"`        // Text response (may be empty if only tool calls)
	ToolCalls  []ToolCall `json:"tool_calls"`  // Tool invocations requested by LLM
	StopReason string     `json:"stop_reason"` // "end_turn", "tool_use", etc.
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
