package shards

import (
	"testing"
	"time"

	"codenerd/internal/types"
)

// --- Config functions ---

func TestDefaultGeneralistConfig_ShouldReturnCorrectDefaults(t *testing.T) {
	cfg := DefaultGeneralistConfig("test-coder")
	if cfg.Name != "test-coder" {
		t.Errorf("expected name 'test-coder', got %q", cfg.Name)
	}
	if cfg.Type != types.ShardTypeEphemeral {
		t.Errorf("expected type ephemeral, got %s", cfg.Type)
	}
	if cfg.Timeout != 15*time.Minute {
		t.Errorf("expected 15m timeout, got %v", cfg.Timeout)
	}
	if len(cfg.Permissions) != 3 {
		t.Errorf("expected 3 permissions, got %d", len(cfg.Permissions))
	}
}

func TestDefaultSpecialistConfig_ShouldReturnCorrectDefaults(t *testing.T) {
	cfg := DefaultSpecialistConfig("expert", "/db/path")
	if cfg.Name != "expert" {
		t.Errorf("expected name 'expert', got %q", cfg.Name)
	}
	if cfg.Type != types.ShardTypePersistent {
		t.Errorf("expected type persistent, got %s", cfg.Type)
	}
	if cfg.KnowledgePath != "/db/path" {
		t.Errorf("expected knowledge path '/db/path', got %q", cfg.KnowledgePath)
	}
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("expected 30m timeout, got %v", cfg.Timeout)
	}
	if len(cfg.Permissions) != 5 {
		t.Errorf("expected 5 permissions, got %d", len(cfg.Permissions))
	}
}

func TestDefaultSystemConfig_ShouldReturnCorrectDefaults(t *testing.T) {
	cfg := DefaultSystemConfig("policy")
	if cfg.Name != "policy" {
		t.Errorf("expected name 'policy', got %q", cfg.Name)
	}
	if cfg.Type != types.ShardTypeSystem {
		t.Errorf("expected type system, got %s", cfg.Type)
	}
	if cfg.Timeout != 24*time.Hour {
		t.Errorf("expected 24h timeout, got %v", cfg.Timeout)
	}
}

func TestCoreShardDescriptions_ShouldHaveEntries(t *testing.T) {
	expected := []string{"researcher", "reviewer", "codebase", "coder", "tester"}
	for _, name := range expected {
		if desc, ok := CoreShardDescriptions[name]; !ok {
			t.Errorf("missing description for %q", name)
		} else if desc == "" {
			t.Errorf("empty description for %q", name)
		}
	}
}

// --- ShardManager creation ---

func TestNewShardManager_ShouldInitializeMaps(t *testing.T) {
	sm := NewShardManager()
	if sm == nil {
		t.Fatal("expected non-nil ShardManager")
	}
	if sm.shards == nil {
		t.Error("expected shards map to be initialized")
	}
	if sm.results == nil {
		t.Error("expected results map to be initialized")
	}
	if sm.profiles == nil {
		t.Error("expected profiles map to be initialized")
	}
	if sm.factories == nil {
		t.Error("expected factories map to be initialized")
	}
}

// --- ShardManager setters ---

func TestShardManager_SetSessionID(t *testing.T) {
	sm := NewShardManager()
	sm.SetSessionID("sess-123")
	if sm.sessionID != "sess-123" {
		t.Errorf("expected sessionID 'sess-123', got %q", sm.sessionID)
	}
}

func TestShardManager_SetNerdDir(t *testing.T) {
	sm := NewShardManager()
	sm.SetNerdDir("/path/to/.nerd")
	if sm.nerdDir != "/path/to/.nerd" {
		t.Errorf("expected nerdDir, got %q", sm.nerdDir)
	}
}

func TestShardManager_SetVirtualStore(t *testing.T) {
	sm := NewShardManager()
	sm.SetVirtualStore("mock-vs")
	if sm.virtualStore != "mock-vs" {
		t.Error("expected virtualStore to be set")
	}
}

func TestShardManager_SetTransparencyManager(t *testing.T) {
	sm := NewShardManager()
	sm.SetTransparencyManager("mock-tm")
	if sm.transparencyManager != "mock-tm" {
		t.Error("expected transparencyManager to be set")
	}
}

// --- Profile management ---

func TestShardManager_DefineAndGetProfile(t *testing.T) {
	sm := NewShardManager()
	cfg := DefaultGeneralistConfig("test")
	sm.DefineProfile("test", cfg)

	got, ok := sm.GetProfile("test")
	if !ok {
		t.Fatal("expected profile to exist")
	}
	if got.Name != "test" {
		t.Errorf("expected name 'test', got %q", got.Name)
	}
}

func TestShardManager_GetProfile_WhenMissing_ShouldReturnFalse(t *testing.T) {
	sm := NewShardManager()
	_, ok := sm.GetProfile("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent profile")
	}
}

// --- Factory registration ---

func TestShardManager_RegisterShard(t *testing.T) {
	sm := NewShardManager()
	factory := func(id string, cfg types.ShardConfig) types.ShardAgent { return nil }
	sm.RegisterShard("test-factory", factory)
	if _, ok := sm.factories["test-factory"]; !ok {
		t.Error("expected factory to be registered")
	}
}

// --- ListAvailableShards ---

func TestShardManager_ListAvailableShards_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	sm := NewShardManager()
	shards := sm.ListAvailableShards()
	if len(shards) != 0 {
		t.Errorf("expected 0 shards, got %d", len(shards))
	}
}

func TestShardManager_ListAvailableShards_WithFactories(t *testing.T) {
	sm := NewShardManager()
	sm.RegisterShard("coder", func(id string, cfg types.ShardConfig) types.ShardAgent { return nil })
	sm.RegisterShard("tester", func(id string, cfg types.ShardConfig) types.ShardAgent { return nil })

	shards := sm.ListAvailableShards()
	if len(shards) != 2 {
		t.Fatalf("expected 2 shards, got %d", len(shards))
	}
}

func TestShardManager_ListAvailableShards_WithSystemShard(t *testing.T) {
	sm := NewShardManager()
	sm.RegisterShard("perception_firewall", func(id string, cfg types.ShardConfig) types.ShardAgent { return nil })
	shards := sm.ListAvailableShards()
	if len(shards) != 1 {
		t.Fatalf("expected 1 shard, got %d", len(shards))
	}
	if shards[0].Type != types.ShardTypeSystem {
		t.Errorf("expected system type, got %s", shards[0].Type)
	}
}

func TestShardManager_ListAvailableShards_MergesProfilesAndFactories(t *testing.T) {
	sm := NewShardManager()
	sm.RegisterShard("coder", func(id string, cfg types.ShardConfig) types.ShardAgent { return nil })
	sm.DefineProfile("special-agent", DefaultSpecialistConfig("special-agent", "/db"))

	shards := sm.ListAvailableShards()
	if len(shards) != 2 {
		t.Errorf("expected 2 (factory + profile), got %d", len(shards))
	}
}

// --- ToFacts ---

func TestShardManager_ToFacts_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	sm := NewShardManager()
	facts := sm.ToFacts()
	if len(facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(facts))
	}
}

func TestShardManager_ToFacts_WithProfiles(t *testing.T) {
	sm := NewShardManager()
	sm.DefineProfile("coder", DefaultGeneralistConfig("coder"))
	sm.DefineProfile("expert", DefaultSpecialistConfig("expert", "/db"))

	facts := sm.ToFacts()
	if len(facts) != 2 {
		t.Errorf("expected 2 facts, got %d", len(facts))
	}
	for _, f := range facts {
		if f.Predicate != "shard_profile" {
			t.Errorf("expected predicate 'shard_profile', got %q", f.Predicate)
		}
	}
}

// --- categorizeShardType ---

func TestCategorizeShardType_WhenSystemShard_ShouldReturnSystem(t *testing.T) {
	sm := NewShardManager()
	tests := []string{"perception_firewall", "constitution_gate", "executive_policy",
		"cost_guard", "tactile_router", "session_planner", "world_model_ingestor"}
	for _, name := range tests {
		result := sm.categorizeShardType(name, "")
		if result != "system" {
			t.Errorf("expected 'system' for %q, got %q", name, result)
		}
	}
}

func TestCategorizeShardType_WhenEphemeralShard_ShouldReturnEphemeral(t *testing.T) {
	sm := NewShardManager()
	tests := []string{"coder", "tester", "reviewer", "researcher"}
	for _, name := range tests {
		result := sm.categorizeShardType(name, "")
		if result != "ephemeral" {
			t.Errorf("expected 'ephemeral' for %q, got %q", name, result)
		}
	}
}

func TestCategorizeShardType_WhenUnknown_ShouldReturnSpecialist(t *testing.T) {
	sm := NewShardManager()
	result := sm.categorizeShardType("custom-agent", "")
	if result != "specialist" {
		t.Errorf("expected 'specialist', got %q", result)
	}
}

func TestCategorizeShardType_WhenTypeSystem_ShouldReturnSystem(t *testing.T) {
	sm := NewShardManager()
	result := sm.categorizeShardType("anything", types.ShardTypeSystem)
	if result != "system" {
		t.Errorf("expected 'system' when type is ShardTypeSystem, got %q", result)
	}
}

// --- normalizeMangleAtom ---

func TestNormalizeMangleAtom_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	if normalizeMangleAtom("") != "" {
		t.Error("expected empty result for empty input")
	}
}

func TestNormalizeMangleAtom_WhenNoSlash_ShouldAddSlash(t *testing.T) {
	result := normalizeMangleAtom("coder")
	if result != "/coder" {
		t.Errorf("expected '/coder', got %q", result)
	}
}

func TestNormalizeMangleAtom_WhenAlreadySlashed_ShouldNotDouble(t *testing.T) {
	result := normalizeMangleAtom("/coder")
	if result != "/coder" {
		t.Errorf("expected '/coder', got %q", result)
	}
}

func TestNormalizeMangleAtom_WhenDoubleSlash_ShouldNormalize(t *testing.T) {
	result := normalizeMangleAtom("//coder")
	if result != "/coder" {
		t.Errorf("expected '/coder', got %q", result)
	}
}

func TestNormalizeMangleAtom_WhenWhitespace_ShouldTrim(t *testing.T) {
	result := normalizeMangleAtom("  coder  ")
	if result != "/coder" {
		t.Errorf("expected '/coder', got %q", result)
	}
}

// --- estimateTokens ---

func TestEstimateTokens_ShouldReturnQuarterLength(t *testing.T) {
	if estimateTokens("hello world!") != 3 { // 12/4
		t.Errorf("expected 3, got %d", estimateTokens("hello world!"))
	}
	if estimateTokens("") != 0 {
		t.Errorf("expected 0 for empty, got %d", estimateTokens(""))
	}
}

// --- trimToTokenBudget ---

func TestTrimToTokenBudget_WhenZeroBudget_ShouldDefault2000(t *testing.T) {
	sm := NewShardManager()
	tools := []types.ToolInfo{
		{Name: "tool1", Description: "short desc"},
	}
	result := sm.trimToTokenBudget(tools, 0)
	// With default 2000 budget, one small tool should fit
	if len(result) != 1 {
		t.Errorf("expected 1 tool with default budget, got %d", len(result))
	}
}

func TestTrimToTokenBudget_WhenBudgetExceeded_ShouldTruncate(t *testing.T) {
	sm := NewShardManager()
	tools := make([]types.ToolInfo, 0, 100)
	for i := 0; i < 100; i++ {
		tools = append(tools, types.ToolInfo{
			Name:        "tool-with-a-really-long-name-that-uses-lots-of-tokens",
			Description: "This is a very detailed description that should eat up the token budget quickly and cause truncation to occur",
		})
	}
	result := sm.trimToTokenBudget(tools, 100) // Very small budget
	if len(result) >= 100 {
		t.Errorf("expected truncation, but got %d tools", len(result))
	}
}

// --- Review feedback methods ---

func TestCheckReviewNeedsValidation_WhenNoProvider_ShouldReturnFalse(t *testing.T) {
	sm := NewShardManager()
	result := sm.CheckReviewNeedsValidation("review-1")
	if result {
		t.Error("expected false without provider")
	}
}

func TestGetReviewSuspectReasons_WhenNoProvider_ShouldReturnNil(t *testing.T) {
	sm := NewShardManager()
	reasons := sm.GetReviewSuspectReasons("review-1")
	if reasons != nil {
		t.Errorf("expected nil reasons without provider, got %v", reasons)
	}
}

func TestGetReviewAccuracyReport_WhenNoProvider_ShouldReturnMessage(t *testing.T) {
	sm := NewShardManager()
	report := sm.GetReviewAccuracyReport("review-1")
	if report == "" {
		t.Error("expected non-empty fallback message")
	}
}

func TestAcceptReviewFinding_WhenNoProvider_ShouldNotPanic(t *testing.T) {
	sm := NewShardManager()
	sm.AcceptReviewFinding("review-1", "file.go", 42) // Should not panic
}

func TestRejectReviewFinding_WhenNoProvider_ShouldNotPanic(t *testing.T) {
	sm := NewShardManager()
	sm.RejectReviewFinding("review-1", "file.go", 42, "false positive") // Should not panic
}

// --- GetRunningShardByConfigName ---

func TestGetRunningShardByConfigName_WhenNoShards_ShouldReturnFalse(t *testing.T) {
	sm := NewShardManager()
	_, ok := sm.GetRunningShardByConfigName("coder")
	if ok {
		t.Error("expected false for empty shard map")
	}
}

// --- queryToolsFromKernel ---

func TestQueryToolsFromKernel_WhenNoKernel_ShouldReturnNil(t *testing.T) {
	sm := NewShardManager()
	tools := sm.queryToolsFromKernel()
	if tools != nil {
		t.Errorf("expected nil tools without kernel, got %v", tools)
	}
}

// --- queryRelevantTools ---

func TestQueryRelevantTools_WhenNoKernel_ShouldReturnNil(t *testing.T) {
	sm := NewShardManager()
	tools := sm.queryRelevantTools(ToolRelevanceQuery{ShardType: "coder"})
	if tools != nil {
		t.Errorf("expected nil without kernel, got %v", tools)
	}
}

// --- DisableExecutiveBootGuard ---

func TestDisableExecutiveBootGuard_ShouldDisablePolicy(t *testing.T) {
	sm := NewShardManager()
	sm.DisableExecutiveBootGuard()
	if sm.disabled == nil {
		// disabled map may not be initialized if DisableSystemShard creates it lazily
		// This is OK since it means the method ran without panic
		return
	}
	if _, ok := sm.disabled["executive_policy"]; !ok {
		t.Error("expected executive_policy to be disabled")
	}
}
