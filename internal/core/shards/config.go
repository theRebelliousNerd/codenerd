package shards

import (
	"time"

	"codenerd/internal/types"
)

// DefaultGeneralistConfig returns config for a Type A generalist.
func DefaultGeneralistConfig(name string) types.ShardConfig {
	return types.ShardConfig{
		Name:    name,
		Type:    types.ShardTypeEphemeral,
		Timeout: 15 * time.Minute,
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
			types.PermissionWriteFile,
			types.PermissionNetwork,
		},
		Model: types.ModelConfig{
			Capability: types.CapabilityBalanced,
		},
	}
}

// DefaultSpecialistConfig returns config for a Type B specialist.
func DefaultSpecialistConfig(name, knowledgePath string) types.ShardConfig {
	return types.ShardConfig{
		Name:          name,
		Type:          types.ShardTypePersistent,
		BaseType:      "researcher",
		KnowledgePath: knowledgePath,
		Timeout:       30 * time.Minute,
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
			types.PermissionWriteFile,
			types.PermissionNetwork,
			types.PermissionBrowser,
			types.PermissionResearch,
		},
		Model: types.ModelConfig{
			Capability: types.CapabilityHighReasoning,
		},
	}
}

// DefaultSystemConfig returns config for a Type S system shard.
func DefaultSystemConfig(name string) types.ShardConfig {
	return types.ShardConfig{
		Name:    name,
		Type:    types.ShardTypeSystem,
		Timeout: 24 * time.Hour, // Long running
		Permissions: []types.ShardPermission{
			types.PermissionReadFile,
			types.PermissionWriteFile,
			types.PermissionExecCmd,
			types.PermissionNetwork,
		},
		Model: types.ModelConfig{
			Capability: types.CapabilityBalanced,
		},
	}
}
