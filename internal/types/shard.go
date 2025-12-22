package types

import (
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
	CapabilityBalanced      ModelCapability = "balanced"       // e.g. Gemini 2.5 Pro
	CapabilityHighSpeed     ModelCapability = "high_speed"     // e.g. Gemini 2.5 Flash, Haiku
)

// ModelConfig defines the LLM requirements for a shard.
type ModelConfig struct {
	Capability ModelCapability
}

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
