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

// DefaultImageGeneratorConfig returns config for Gemini Nano Banana 2 image shards.
// Timeout is intentionally tight so missing/slow Gemini cannot hold the CLI.
func DefaultImageGeneratorConfig(name string) types.ShardConfig {
	return types.ShardConfig{
		Name:    name,
		Type:    types.ShardTypeEphemeral,
		Timeout: 2 * time.Minute,
		Permissions: []types.ShardPermission{
			types.PermissionWriteFile,
			types.PermissionNetwork,
		},
		Model: types.ModelConfig{
			Name:       "gemini-3.1-flash-image",
			Capability: types.CapabilityBalanced,
		},
	}
}

// CoreShardDescriptions provides canonical descriptions for built-in shards.
// These are the official descriptions used by the JIT compiler, UI, and documentation.
// Keep these in sync with actual shard capabilities.
var CoreShardDescriptions = map[string]string{
	"researcher":      "Deep web research and documentation gathering (Context7, GitHub, web search)",
	"reviewer":        "Code review, hypothesis verification, and security analysis",
	"codebase":        "Search within project files for patterns and implementations",
	"coder":           "Write and modify code files based on requirements",
	"tester":          "Run tests and validate implementations",
	"image_generator": "Image generation via Gemini Nano Banana 2 (gemini-3.1-flash-image; never Ollama)",
}
