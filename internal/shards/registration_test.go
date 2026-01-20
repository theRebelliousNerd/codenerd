package shards

import (
	"context"
	"testing"

	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// =============================================================================
// MOCKS
// =============================================================================

type mockLLMClient struct{}

func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "Mock response", nil
}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "Mock response", nil
}

func (m *mockLLMClient) StreamComplete(ctx context.Context, prompt string) (<-chan string, <-chan error) {
	return nil, nil
}

func (m *mockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "Mock response"}, nil
}

// Full Kernel interface implementation
type mockKernel struct{}

func (m *mockKernel) LoadFacts(facts []types.Fact) error                             { return nil }
func (m *mockKernel) Query(predicate string) ([]types.Fact, error)                   { return nil, nil }
func (m *mockKernel) QueryAll() (map[string][]types.Fact, error)                     { return nil, nil }
func (m *mockKernel) Assert(fact types.Fact) error                                   { return nil }
func (m *mockKernel) AssertBatch(facts []types.Fact) error                           { return nil }
func (m *mockKernel) Retract(predicate string) error                                 { return nil }
func (m *mockKernel) RetractFact(fact types.Fact) error                              { return nil }
func (m *mockKernel) UpdateSystemFacts() error                                       { return nil }
func (m *mockKernel) Reset()                                                         {}
func (m *mockKernel) AppendPolicy(policy string)                                     {}
func (m *mockKernel) RetractExactFactsBatch(facts []types.Fact) error                { return nil }
func (m *mockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error { return nil }

// =============================================================================
// REGISTRATION TESTS
// =============================================================================

func TestRegisterAllShardFactories(t *testing.T) {
	// Not safe for parallel because it modifies ShardManager global state if not careful.
	// We'll create a local ShardManager.

	sm := coreshards.NewShardManager()
	ctx := RegistryContext{
		Kernel:    &mockKernel{}, // mockKernel needs to implement types.Kernel
		LLMClient: &mockLLMClient{},
		Workspace: "/tmp/workspace",
		// VirtualStore, JITCompiler can be nil for basic registration
	}

	RegisterAllShardFactories(sm, ctx)

	// Verify expected shards are registered
	expectedShards := []string{
		"requirements_interrogator",
		"perception_firewall",
		"world_model_ingestor",
		"executive_policy",
		"constitution_gate",
		"legislator",
		"mangle_repair",
		"tactile_router",
		"campaign_runner",
		"session_planner",
	}

	for _, name := range expectedShards {
		// Verify factory exists
		// ShardManager doesn't expose factories directly, but we can try to CreateShard
		// However, CreateShard requires config.
		// Best we can do is check if profiles are defined (RegisterSystemShardProfiles does that)
		// Or try to execute one.
		// Since RegisterShard adds to internal map, we can't inspect it directly without internals.
		// But we can check if Profiles are defined.

		profile, ok := sm.GetProfile(name)
		if !ok {
			t.Errorf("Expected profile for %s to be registered", name)
		}
		if profile.Name != name {
			t.Errorf("Profile name mismatch for %s", name)
		}
	}
}

func TestRegisterSystemShardProfiles(t *testing.T) {
	t.Parallel()
	sm := coreshards.NewShardManager()

	RegisterSystemShardProfiles(sm)

	// Check a few key profiles
	checkProfile(t, sm, "perception_firewall", types.ShardTypeSystem)
	checkProfile(t, sm, "campaign_runner", types.ShardTypeSystem)
}

func checkProfile(t *testing.T, sm *coreshards.ShardManager, name string, expectedType types.ShardType) {
	t.Helper()
	profile, ok := sm.GetProfile(name)
	if !ok {
		t.Errorf("Profile %q not found", name)
		return
	}
	if profile.Type != expectedType {
		t.Errorf("Profile %q type = %v, want %v", name, profile.Type, expectedType)
	}
}
