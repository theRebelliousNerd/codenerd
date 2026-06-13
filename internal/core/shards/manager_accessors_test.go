package shards

import (
	"testing"

	"codenerd/internal/types"
)

func TestShardManager_EmptyAccessors(t *testing.T) {
	sm := NewShardManager()

	if sm.GetActiveShardCount() != 0 {
		t.Errorf("fresh manager GetActiveShardCount=%d, want 0", sm.GetActiveShardCount())
	}
	if got := sm.GetActiveShards(); len(got) != 0 {
		t.Errorf("fresh manager GetActiveShards=%d, want empty", len(got))
	}
	if sm.GetActiveNonSystemShardCount() != 0 {
		t.Errorf("fresh manager GetActiveNonSystemShardCount=%d, want 0", sm.GetActiveNonSystemShardCount())
	}

	// StopAll on an empty manager must be a safe no-op that resets the map.
	sm.StopAll()
	if sm.GetActiveShardCount() != 0 {
		t.Error("GetActiveShardCount should remain 0 after StopAll on an empty manager")
	}
}

func TestShardManager_Setters(t *testing.T) {
	sm := NewShardManager()

	// Each setter should store its value without panicking; verify via the
	// white-box field (same package).
	sm.SetGlassBoxBus(nil)

	sm.SetSessionID("sess-1")
	if sm.sessionID != "sess-1" {
		t.Errorf("SetSessionID not applied: %q", sm.sessionID)
	}

	sm.SetNerdDir("/tmp/.nerd")
	if sm.nerdDir != "/tmp/.nerd" {
		t.Errorf("SetNerdDir not applied: %q", sm.nerdDir)
	}

	sm.SetParentKernel(nil)
	sm.SetLLMClient(nil)
	sm.SetPromptLoader(nil)
	sm.SetJITRegistrar(nil)
	sm.SetJITUnregistrar(nil)
	sm.SetReviewerFeedbackProvider(nil)
	sm.SetLearningStore(nil)

	called := false
	hook := func(types.ShardAgent) { called = true }
	sm.SetPostSpawnHook(hook)
	if sm.postSpawnHook == nil {
		t.Fatal("SetPostSpawnHook did not store the hook")
	}
	// Invoke the stored hook to confirm identity.
	sm.postSpawnHook(nil)
	if !called {
		t.Error("stored post-spawn hook was not the one provided")
	}
}

func TestShardManager_SortToolsByPriority_NilKernel(t *testing.T) {
	sm := NewShardManager()
	// With no kernel attached, sorting is a documented no-op and must not panic.
	tools := []types.ToolInfo{{Name: "a"}, {Name: "b"}}
	sm.sortToolsByPriority(tools, "coder")
	if len(tools) != 2 {
		t.Errorf("tool slice mutated unexpectedly: %v", tools)
	}
}
