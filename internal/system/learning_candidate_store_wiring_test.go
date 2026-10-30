package system

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// The executive and the perception firewall stage learning candidates for
// verbs they could not map, through SetLearningCandidateStore. Nothing at
// boot called it, so every candidate they staged went nowhere. Both shards,
// as the booted shard manager runs them, hold the workspace's local store.
func TestBoot_SystemShardsStageLearningCandidatesInTheLocalStore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ws := t.TempDir()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	for _, name := range []string{"executive_policy", "perception_firewall"} {
		agent, ok := cortex.ShardManager.GetRunningShardByConfigName(name)
		for deadline := time.Now().Add(15 * time.Second); !ok && time.Now().Before(deadline); {
			time.Sleep(50 * time.Millisecond)
			agent, ok = cortex.ShardManager.GetRunningShardByConfigName(name)
		}
		if !ok {
			t.Fatalf("%s is not running after boot", name)
		}
		field := reflect.ValueOf(agent).Elem().FieldByName("candidateStore")
		if !field.IsValid() {
			t.Fatalf("%s (%T) has no candidateStore", name, agent)
		}
		if field.IsNil() {
			t.Errorf("%s holds no learning candidate store: the candidates it stages are dropped", name)
		}
	}
}
