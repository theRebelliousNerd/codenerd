package chat

import (
	"context"
	"strings"
	"testing"

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

	t.Run("failed spawn step is absent from delivery reports", func(t *testing.T) {
		t.Parallel()
		// Audited failing input: a single-step plan whose spawn fails
		// (nil taskExecutor). Current executeMultiStepTask hits `continue`
		// before appending to ran/reports, so the failed step never reaches
		// record and a plan whose steps all failed skips the delivery judge
		// (len(ran)==0). The fix records the failed return.
		m := NewTestModel()
		m.taskExecutor = nil
		m.shardMgr = coreshards.NewShardManager()
		m.kernel = nil
		m.jev = nil
		steps := []TaskStep{{Verb: "/build", Target: "svc", ShardType: "coder", Task: "build svc"}}
		intent := perception.Intent{Response: "build svc"}
		msg := m.executeMultiStepTask(context.Background(), intent, "build svc", steps)()
		asst, ok := msg.(assistantMsg)
		if !ok {
			t.Fatalf("executeMultiStepTask msg = %T, want assistantMsg", msg)
		}
		if !strings.Contains(asst.Surface, "Failed") {
			t.Fatalf("surface should show Failed for nil executor, got: %q", asst.Surface)
		}
		// Desired: the failed step is recorded for delivery, so the summary
		// counts it as ran. Current: "0/1 steps ran".
		if strings.Contains(asst.Surface, "0/1 steps ran") {
			t.Errorf("BUG PINNED: failed step absent from ran/reports; surface=%q", asst.Surface)
		}
		if !strings.Contains(asst.Surface, "1/1 steps ran") {
			t.Errorf("want failed step counted as ran (1/1), surface=%q", asst.Surface)
		}
	})
}
