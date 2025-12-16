package core

import (
	"codenerd/internal/types"
)

// =============================================================================
// TYPE ALIASES
// =============================================================================
// These aliases maintain backward compatibility for packages that import
// core but expect types that have been moved to internal/types.

type ShardType = types.ShardType

const (
	ShardTypeEphemeral  = types.ShardTypeEphemeral
	ShardTypePersistent = types.ShardTypePersistent
	ShardTypeUser       = types.ShardTypeUser
	ShardTypeSystem     = types.ShardTypeSystem
)

type ShardState = types.ShardState

const (
	ShardStateIdle      = types.ShardStateIdle
	ShardStateRunning   = types.ShardStateRunning
	ShardStateCompleted = types.ShardStateCompleted
	ShardStateFailed    = types.ShardStateFailed
)

type ShardPermission = types.ShardPermission

const (
	PermissionReadFile  = types.PermissionReadFile
	PermissionWriteFile = types.PermissionWriteFile
	PermissionExecCmd   = types.PermissionExecCmd
	PermissionNetwork   = types.PermissionNetwork
	PermissionBrowser   = types.PermissionBrowser
	PermissionCodeGraph = types.PermissionCodeGraph
	PermissionAskUser   = types.PermissionAskUser
	PermissionResearch  = types.PermissionResearch
)

type ModelCapability = types.ModelCapability

const (
	CapabilityHighReasoning = types.CapabilityHighReasoning
	CapabilityBalanced      = types.CapabilityBalanced
	CapabilityHighSpeed     = types.CapabilityHighSpeed
)

type ModelConfig = types.ModelConfig
type ShardConfig = types.ShardConfig
type ShardResult = types.ShardResult
type SpawnPriority = types.SpawnPriority

const (
	PriorityLow      = types.PriorityLow
	PriorityNormal   = types.PriorityNormal
	PriorityHigh     = types.PriorityHigh
	PriorityCritical = types.PriorityCritical
)
