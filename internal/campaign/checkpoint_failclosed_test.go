package campaign

import (
	"context"
	"strings"
	"testing"
)

// A checkpoint that cannot run must not report PASS.
//
// Both of these returned true when taskExecutor was nil, so "we did not check"
// was indistinguishable from "we checked and it was fine". The orchestrator's
// own validation could not catch it either: it accepts a config where
// ShardManager is set and TaskExecutor is nil, because the guard is an OR.
// A campaign built that way reported every phase verified having verified
// nothing.
//
// Found by a source-level sweep for fields set at some construction sites and
// not others: OrchestratorConfig.TaskExecutor was set at 4 of 5 sites, missing
// at internal/shards/system/campaign_runner.go.
func TestCheckpointsFailClosedWithoutTaskExecutor(t *testing.T) {
	cr := NewCheckpointRunner(nil, nil, t.TempDir())
	phase := &Phase{Name: "unverifiable-phase"}

	t.Run("shard validation", func(t *testing.T) {
		passed, details, err := cr.runShardValidationCheckpoint(context.Background(), phase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if passed {
			t.Fatal("checkpoint passed without a task executor: an unrun verification reported success")
		}
		if !strings.Contains(details, "TaskExecutor") {
			t.Errorf("details should name the missing collaborator so the fix is obvious; got %q", details)
		}
	})

	t.Run("nemesis gauntlet", func(t *testing.T) {
		passed, details, err := cr.runNemesisGauntletCheckpoint(context.Background(), phase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if passed {
			t.Fatal("gauntlet passed without a task executor: no adversarial verification ran")
		}
		if !strings.Contains(details, "TaskExecutor") {
			t.Errorf("details should name the missing collaborator; got %q", details)
		}
	})
}
