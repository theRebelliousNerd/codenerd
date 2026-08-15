package types

import (
	"context"
	"time"
)

// =============================================================================
// SHARD TYPES AND CONSTANTS
// =============================================================================

// ShardType defines the lifecycle model of a shard.
type ShardType string

const (
	ShardTypeEphemeral  ShardType = "ephemeral"  // Type A: Created for a task, dies after
	ShardTypePersistent ShardType = "persistent" // Type B: Persistent, user-defined specialist
	ShardTypeUser       ShardType = "user"       // Alias for Persistent
	ShardTypeSystem     ShardType = "system"     // Type S: Long-running system service
)

// ShardState defines the execution state of a shard.
type ShardState string

const (
	ShardStateIdle      ShardState = "idle"
	ShardStateRunning   ShardState = "running"
	ShardStateCompleted ShardState = "completed"
	ShardStateFailed    ShardState = "failed"
)

// ShardPermission defines what a shard is allowed to do.
type ShardPermission string

const (
	PermissionReadFile  ShardPermission = "read_file"
	PermissionWriteFile ShardPermission = "write_file"
	PermissionExecCmd   ShardPermission = "exec_cmd"
	PermissionNetwork   ShardPermission = "network"
	PermissionBrowser   ShardPermission = "browser"
	PermissionCodeGraph ShardPermission = "code_graph"
	PermissionAskUser   ShardPermission = "ask_user"
	PermissionResearch  ShardPermission = "research"
)

// ModelCapability defines the class of LLM reasoning required.
type ModelCapability string

const (
	CapabilityHighReasoning ModelCapability = "high_reasoning" // e.g. Claude 3.5 Sonnet, GPT-4o
	CapabilityBalanced      ModelCapability = "balanced"       // e.g. Gemini 3 Pro
	CapabilityHighSpeed     ModelCapability = "high_speed"     // e.g. Gemini 3 Flash, Haiku
)

// ModelConfig defines the LLM requirements for a shard.
type ModelConfig struct {
	Name       string
	Capability ModelCapability
}

// StartupMode determines when a system shard starts (only applicable to Type S shards).
type StartupMode string

const (
	// StartupAuto starts the shard when the application initializes.
	StartupAuto StartupMode = "auto"
	// StartupOnDemand starts the shard only when explicitly requested.
	StartupOnDemand StartupMode = "on_demand"
)

// ShardConfig holds configuration for a shard.
type ShardConfig struct {
	Name string
	Type ShardType
	// BaseType selects the underlying factory to use when Name doesn't match a registered factory.
	// Intended for Type B (persistent) and Type U (user-defined) specialists.
	BaseType      string
	Permissions   []ShardPermission // Allowed capabilities
	Timeout       time.Duration     // Default execution timeout
	MemoryLimit   int               // Abstract memory unit limit
	Model         ModelConfig       // LLM requirements
	KnowledgePath string            // Path to local knowledge DB (Type B only)
	// StartupMode controls when system shards (Type S) start (auto=on boot, on_demand=only when requested)
	StartupMode StartupMode

	// Tool associations (for specialist shards)
	Tools           []string          // List of tool names this shard can use
	ToolPreferences map[string]string // Action -> preferred tool mapping

	// Shard-specific Mangle policy (POWER-USER-FEATURE)
	// When set, these rules are appended to the kernel before shard execution.
	// Use for specialist shards that need domain-specific permissions or constraints.
	Policy string

	// Session context (Blackboard Pattern)
	SessionContext *SessionContext // Compressed session context for LLM injection
}

// ShardResult represents the outcome of a shard execution.
type ShardResult struct {
	ShardID   string
	Result    string
	Error     error
	Timestamp time.Time
}

// ShardInfo contains information about an available shard for selection.
type ShardInfo struct {
	Name         string    `json:"name"`
	Type         ShardType `json:"type"`
	Description  string    `json:"description,omitempty"`
	HasKnowledge bool      `json:"has_knowledge"`
}

// SpawnPriority defines the scheduling priority for spawn requests.
type SpawnPriority int

const (
	// PriorityLow is for background tasks, speculation, and learning.
	PriorityLow SpawnPriority = 0

	// PriorityNormal is for campaign tasks and regular operations.
	PriorityNormal SpawnPriority = 1

	// PriorityHigh is for user-requested commands (/review, /test, /fix).
	PriorityHigh SpawnPriority = 2

	// PriorityCritical is for system shards and safety-critical operations.
	PriorityCritical SpawnPriority = 3
)

// String returns the priority name.
func (p SpawnPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// CtxKeyPriority is the legacy string context key for spawn priority.
//
// Deprecated: use WithSpawnPriority / SpawnPriorityFromContext (ctxkeys.go).
// Kept because existing readers still call ctx.Value(CtxKeyPriority) directly;
// the setters dual-write this key so those readers keep working.
const CtxKeyPriority = "spawn_priority"

// CtxKeyModelCapability is the legacy string context key for the per-call
// reasoning-class hint that lets one shared LLM client serve shards of
// different model tiers.
//
// Deprecated: use WithModelCapability / ModelCapabilityFromContext.
const CtxKeyModelCapability = "model_capability"

// CtxKeyModelName is the legacy string context key for a concrete per-shard
// model override.
//
// Deprecated: use WithModelName / ModelNameFromContext.
const CtxKeyModelName = "model_name"

// CtxKeyStructuredOutputOnly tells an LLM client that this call's reply is
// parsed directly into a Go struct and must NOT be wrapped in the Piggyback
// envelope, so no envelope response_format schema may be attached.
//
// It exists because clients otherwise decide by sniffing the prompt for the
// substring "control_packet" (perception.isPiggybackPrompt). That heuristic
// cannot distinguish a prompt that teaches the envelope from one that forbids
// it: writing "Do NOT wrap your reply in a control_packet envelope" made the
// client attach the envelope schema, and a strict schema is not advice — the
// model then emitted a fully-formed envelope with every field empty
// (overall_usefulness: 0, missing_context: "") because that is what the schema
// required. Four live campaign decompositions returned no phases this way and
// silently ran a generic placeholder plan.
//
// The contract belongs to the caller, which knows its own role, not to a
// substring search over text written for a different audience. Set it with
// WithStructuredOutputOnly wherever prompt.IsStructuredOutputOnly(shardType)
// holds; leaving it unset preserves the sniffing behaviour for conversational
// shards, which genuinely do speak the envelope.
const CtxKeyStructuredOutputOnly = "structured_output_only"

// WithStructuredOutputOnly marks ctx as carrying a call whose reply is parsed
// directly as JSON, so no Piggyback envelope schema may be attached.
func WithStructuredOutputOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey(CtxKeyStructuredOutputOnly), true)
}

// IsStructuredOutputOnlyCtx reports whether ctx was marked by
// WithStructuredOutputOnly. It also accepts the bare string key, because
// existing call sites in this repo set model_capability and model_name that way.
func IsStructuredOutputOnlyCtx(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if v, ok := ctx.Value(ctxKey(CtxKeyStructuredOutputOnly)).(bool); ok {
		return v
	}
	if v, ok := ctx.Value(CtxKeyStructuredOutputOnly).(bool); ok { //nolint:staticcheck // back-compat with string-keyed call sites
		return v
	}
	return false
}

// ctxKey is a private type so WithStructuredOutputOnly cannot collide with
// another package's context value.
type ctxKey string
