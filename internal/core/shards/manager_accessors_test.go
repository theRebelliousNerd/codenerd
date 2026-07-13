package shards

import (
	"context"
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
	sm.SetImageLLMClient(nil)
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

// stubLLM is a marker client used only to prove clientForShardType identity.
type stubLLM struct{ name string }

func (s *stubLLM) Complete(context.Context, string) (string, error) { return s.name, nil }
func (s *stubLLM) CompleteWithSystem(context.Context, string, string) (string, error) {
	return s.name, nil
}
func (s *stubLLM) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	errCh := make(chan error)
	close(ch)
	close(errCh)
	return ch, errCh
}
func (s *stubLLM) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: s.name}, nil
}

func TestShardManager_clientForShardType_ImageIsolation(t *testing.T) {
	sm := NewShardManager()
	worker := &stubLLM{name: "worker-ollama"}
	image := &stubLLM{name: "gemini-nano-banana-2"}
	sm.SetLLMClient(worker)
	sm.SetImageLLMClient(image)

	got := sm.clientForShardType("image_generator")
	if got != types.LLMClient(image) {
		t.Fatalf("image_generator client = %v, want image client", got)
	}
	for _, typ := range []string{"image-generator", "imagen", "nano_banana", "image"} {
		if sm.clientForShardType(typ) != types.LLMClient(image) {
			t.Fatalf("%s did not route to image client", typ)
		}
	}

	got = sm.clientForShardType("coder")
	if got != types.LLMClient(worker) {
		t.Fatalf("coder client = %v, want worker", got)
	}

	// No silent worker fallback when image client is missing (FM15).
	sm.SetImageLLMClient(nil)
	if sm.clientForShardType("image_generator") != nil {
		t.Fatal("image shard must not fall back to worker when image client unset")
	}
	if sm.clientForShardType("coder") != types.LLMClient(worker) {
		t.Fatal("non-image shards still use worker when image client unset")
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
