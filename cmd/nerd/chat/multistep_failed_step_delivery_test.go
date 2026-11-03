package chat

import (
	"strings"
	"testing"

	"codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
)

func TestExecuteMultiStepTask_RecordsFailedStepForDelivery(t *testing.T) {
	t.Parallel()
	// deliveryChain.record must preserve failure evidence even when Output is empty.
	t.Run("record preserves failure evidence", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name        string
			ret         observation.Return
			wantSubstr  string
			wantChanged int
		}{
			{"happy output recorded", observation.Return{Output: "did work", Changed: []string{"a.go"}}, "did work", 1},
			{"edge empty output no failure", observation.Return{Output: "", Changed: nil}, "", 0},
			{"nil changed deduped", observation.Return{Output: "ok", Changed: []string{"a.go", "a.go"}}, "ok", 1},
			{"empty task output", observation.Return{Output: "   ", Changed: nil}, "", 0},
			{"audited empty output with failure", observation.Return{Output: "", Failure: "boom: spawn failed", Changed: []string{"b.go"}}, "The step failed: boom", 1},
			{"audited output plus failure keeps output", observation.Return{Output: "partial", Failure: "boom", Changed: nil}, "partial", 0},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				dc := &deliveryChain{}
				orig := len(dc.reports)
				t.Cleanup(func() { _ = orig })
				dc.record(tt.ret)
				if len(dc.reports) != 1 {
					t.Fatalf("reports len = %d, want 1", len(dc.reports))
				}
				if tt.wantSubstr != "" && !strings.Contains(dc.reports[0], tt.wantSubstr) {
					t.Errorf("report = %q, want substr %q", dc.reports[0], tt.wantSubstr)
				}
				if len(dc.changed) != tt.wantChanged {
					t.Errorf("changed len = %d, want %d (%v)", len(dc.changed), tt.wantChanged, dc.changed)
				}
				// nil chain must not panic.
				var nilDC *deliveryChain
				nilDC.record(tt.ret)
			})
		}
	})

	tests := []struct {
		name     string
		steps    []TaskStep
		wantSkip bool
	}{
		{
			name:  "failed only step",
			steps: []TaskStep{{Verb: "/fix", Target: "cmd/nerd/cmd_status.go", ShardType: "coder", Task: "add a --json flag"}},
		},
		{
			name: "failed step blocks its dependent",
			steps: []TaskStep{
				{Verb: "/fix", Target: "cmd/nerd/cmd_status.go", ShardType: "coder", Task: "add a --json flag"},
				{Verb: "/test", Target: "cmd/nerd/cmd_status.go", ShardType: "tester", Task: "test the flag", DependsOn: []int{0}},
			},
			wantSkip: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, _ := journeyModel(t, config.WorkModeYolo)
			m.shardMgr = coreshards.NewShardManager()
			m.taskExecutor = nil
			m = withJev(t, m, &fakeJevServer{probs: deliveryProbs(0.02, 0.96, 0.04)})
			msg := m.executeMultiStepTask(m.shutdownCtx, perception.Intent{Response: "Two steps."}, jsonFlagRequest, tt.steps)()
			init, ok := msg.(continuationInitMsg)
			if !ok {
				t.Fatalf("failed step did not reach delivery judgment: got %T, want continuationInitMsg", msg)
			}
			if init.delivery == nil {
				t.Fatal("continuation has no delivery chain")
			}
			if len(init.delivery.reports) != 1 || !strings.Contains(init.delivery.reports[0], "taskExecutor not initialized") {
				t.Errorf("delivery reports = %q, want the failed step's error", init.delivery.reports)
			}
			if !strings.Contains(init.completedSurface, "**Status**: Failed") || !strings.Contains(init.completedSurface, "0 verified") {
				t.Errorf("surface does not retain failure and unverified status: %q", init.completedSurface)
			}
			if tt.wantSkip && !strings.Contains(init.completedSurface, "SKIPPED (dependencies not met)") {
				t.Errorf("failed step's dependent was not skipped: %q", init.completedSurface)
			}
		})
	}
}
