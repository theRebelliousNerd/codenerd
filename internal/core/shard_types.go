package core

import (
	"context"
	"time"

	"codenerd/internal/types"
)

// =============================================================================
// TYPES AND CONSTANTS
// =============================================================================

// Type aliases to internal/types to avoid import cycles
type ShardType = types.ShardType
type ShardState = types.ShardState
type ShardPermission = types.ShardPermission
type ModelCapability = types.ModelCapability
type StructuredIntent = types.StructuredIntent
type ShardSummary = types.ShardSummary
type SessionContext = types.SessionContext

// Re-export constants from internal/types
const (
	ShardTypeEphemeral  = types.ShardTypeEphemeral
	ShardTypePersistent = types.ShardTypePersistent
	ShardTypeUser       = types.ShardTypeUser
	ShardTypeSystem     = types.ShardTypeSystem

	ShardStateIdle      = types.ShardStateIdle
	ShardStateRunning   = types.ShardStateRunning
	ShardStateCompleted = types.ShardStateCompleted
	ShardStateFailed    = types.ShardStateFailed

	PermissionReadFile  = types.PermissionReadFile
	PermissionWriteFile = types.PermissionWriteFile
	PermissionExecCmd   = types.PermissionExecCmd
	PermissionNetwork   = types.PermissionNetwork
	PermissionBrowser   = types.PermissionBrowser
	PermissionCodeGraph = types.PermissionCodeGraph
	PermissionAskUser   = types.PermissionAskUser
	PermissionResearch  = types.PermissionResearch

	CapabilityHighReasoning = types.CapabilityHighReasoning
	CapabilityBalanced      = types.CapabilityBalanced
	CapabilityHighSpeed     = types.CapabilityHighSpeed
)

// ModelConfig defines the LLM requirements for a shard.
type ModelConfig struct {
	Capability ModelCapability
}

// ShardConfig holds configuration for a shard.
type ShardConfig struct {
	Name    string
	Type    ShardType
	BaseType      string
	Permissions   []ShardPermission
	Timeout       time.Duration
	MemoryLimit   int
	Model         ModelConfig
	KnowledgePath string

	Tools           []string
	ToolPreferences map[string]string

	SessionContext *SessionContext
}

// DefaultGeneralistConfig returns config for a Type A generalist.
func DefaultGeneralistConfig(name string) ShardConfig {
	return ShardConfig{
		Name:    name,
		Type:    ShardTypeEphemeral,
		Timeout: 15 * time.Minute,
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionWriteFile,
			PermissionNetwork,
		},
		Model: ModelConfig{
			Capability: CapabilityBalanced,
		},
	}
}

// DefaultSpecialistConfig returns config for a Type B specialist.
func DefaultSpecialistConfig(name, knowledgePath string) ShardConfig {
	return ShardConfig{
		Name:          name,
		Type:          ShardTypePersistent,
		BaseType:      "researcher",
		KnowledgePath: knowledgePath,
		Timeout:       30 * time.Minute,
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionWriteFile,
			PermissionNetwork,
			PermissionBrowser,
			PermissionResearch,
		},
		Model: ModelConfig{
			Capability: CapabilityHighReasoning,
		},
	}
}

// DefaultSystemConfig returns config for a Type S system shard.
func DefaultSystemConfig(name string) ShardConfig {
	return ShardConfig{
		Name:    name,
		Type:    ShardTypeSystem,
		Timeout: 24 * time.Hour,
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionWriteFile,
			PermissionExecCmd,
			PermissionNetwork,
		},
		Model: ModelConfig{
			Capability: CapabilityBalanced,
		},
	}
}

// ShardResult represents the outcome of a shard execution.
type ShardResult struct {
	ShardID   string
	Result    string
	Error     error
	Timestamp time.Time
}

// ShardAgent defines the interface for all agents.
type ShardAgent interface {
	Execute(ctx context.Context, task string) (string, error)
	GetID() string
	GetState() ShardState
	GetConfig() ShardConfig
	Stop() error

	SetParentKernel(k Kernel)
	SetLLMClient(client LLMClient)
	SetSessionContext(ctx *SessionContext)
}

// ShardFactory is a function that creates a new shard instance.
type ShardFactory func(id string, config ShardConfig) ShardAgent

// PromptLoaderFunc is a callback for loading agent prompts from YAML files.
type PromptLoaderFunc func(context.Context, string, string) (int, error)

// JITDBRegistrar is a callback for registering agent knowledge DBs with the JIT prompt compiler.
type JITDBRegistrar func(agentName string, dbPath string) error

// JITDBUnregistrar is a callback for unregistering agent knowledge DBs from the JIT prompt compiler.
type JITDBUnregistrar func(agentName string)

// ShardInfo contains information about an available shard for selection.
type ShardInfo struct {
	Name         string    `json:"name"`
	Type         ShardType `json:"type"`
	Description  string    `json:"description,omitempty"`
	HasKnowledge bool      `json:"has_knowledge"`
}

// ReviewerFeedbackProvider defines the interface for reviewer validation.
type ReviewerFeedbackProvider interface {
	NeedsValidation(reviewID string) bool
	GetSuspectReasons(reviewID string) []string
	AcceptFinding(reviewID, file string, line int)
	RejectFinding(reviewID, file string, line int, reason string)
	GetAccuracyReport(reviewID string) string
}
