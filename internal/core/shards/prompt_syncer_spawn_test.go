package shards

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"codenerd/internal/types"
)

func spawnWithPromptsYAML(t *testing.T, typeName string, withYAML bool, syncer types.AgentPromptSyncFunc) (string, error) {
	t.Helper()
	nerdDir := t.TempDir()
	if withYAML {
		dir := filepath.Join(nerdDir, "agents", typeName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte("- id: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sm := NewShardManager()
	sm.SetTaskDelegator(&recordingDelegator{result: "done"})
	sm.SetNerdDir(nerdDir)
	sm.SetPromptSyncer(syncer)
	sm.DefineProfile(typeName, types.ShardConfig{Name: typeName, Type: types.ShardTypeUser})
	return sm.Spawn(context.Background(), typeName, "task")
}

// A user or persistent agent's spawn re-syncs that one agent through the
// prompt synchronizer, by name, before the shard runs.
func TestSpawnSyncsTheSpawnedAgentsPrompts(t *testing.T) {
	var mu sync.Mutex
	var synced []string
	result, err := spawnWithPromptsYAML(t, "syncme", true, func(_ context.Context, name string) error {
		mu.Lock()
		defer mu.Unlock()
		synced = append(synced, name)
		return nil
	})
	if err != nil || result != "done" {
		t.Fatalf("spawn = %q, %v", result, err)
	}
	if len(synced) != 1 || synced[0] != "syncme" {
		t.Errorf("prompt syncer calls = %v, want exactly [syncme]", synced)
	}
}

// A failed sync leaves the KB's current atoms in place, so the spawn still runs.
func TestSpawnProceedsWhenThePromptSyncFails(t *testing.T) {
	result, err := spawnWithPromptsYAML(t, "failsync", true, func(context.Context, string) error {
		return errors.New("kb locked")
	})
	if err != nil || result != "done" {
		t.Fatalf("a failed prompt sync aborted the spawn: %q, %v", result, err)
	}
}

// No prompts.yaml, nothing to sync.
func TestSpawnWithoutPromptsYAMLDoesNotSync(t *testing.T) {
	called := false
	if _, err := spawnWithPromptsYAML(t, "noyaml", false, func(context.Context, string) error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("prompt syncer ran for an agent with no prompts.yaml")
	}
}
