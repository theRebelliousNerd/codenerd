package transparency

import (
	"strings"
	"testing"
	"time"
)

type captureObserver struct {
	updates []PhaseUpdate
}

func (c *captureObserver) OnPhaseChange(update PhaseUpdate) {
	c.updates = append(c.updates, update)
}

func TestShardObserverLifecycle(t *testing.T) {
	observer := NewShardObserver()
	if observer.IsEnabled() {
		t.Fatalf("expected observer to be disabled by default")
	}

	capture := &captureObserver{}
	observer.AddObserver(capture)
	observer.Enable()

	observer.StartExecution("shard-1", "coder", "task")
	if exec := observer.GetExecution("shard-1"); exec == nil {
		t.Fatalf("expected execution to exist")
	}

	observer.UpdatePhase("shard-1", PhaseAnalyzing, "analysis")
	active := observer.GetActiveExecutions()
	if len(active) != 1 {
		t.Fatalf("expected one active execution")
	}
	if !strings.Contains(observer.FormatExecutionSummary(), "Analyzing") {
		t.Fatalf("expected summary to include phase")
	}

	observer.EndExecution("shard-1", false)
	active = observer.GetActiveExecutions()
	if len(active) != 0 {
		t.Fatalf("expected no active executions after completion")
	}

	history := observer.GetPhaseHistory(10)
	if len(history) < 1 {
		t.Fatalf("expected phase history entries")
	}
	if len(capture.updates) == 0 {
		t.Fatalf("expected observer updates")
	}
}

func TestShardPhaseStringUnknown(t *testing.T) {
	if got := ShardPhase(99).String(); got != "Unknown" {
		t.Fatalf("expected unknown phase string, got %s", got)
	}
}

func TestShardExecutionDurations(t *testing.T) {
	exec := &ShardExecution{
		StartTime: time.Now().Add(-2 * time.Second),
		PhaseTime: time.Now().Add(-1 * time.Second),
	}
	if exec.Duration() <= 0 {
		t.Fatalf("expected duration to be positive")
	}
	if exec.PhaseDuration() <= 0 {
		t.Fatalf("expected phase duration to be positive")
	}
}
