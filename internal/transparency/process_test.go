package transparency

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// withProcessHandles installs manager/bus as the process-wide handles for the
// duration of a test and restores the previous values afterwards.
func withProcessHandles(t *testing.T, tm *TransparencyManager, bus *GlassBoxEventBus) {
	t.Helper()
	prevMgr := SetProcessManager(tm)
	prevBus := SetProcessBus(bus)
	t.Cleanup(func() {
		SetProcessManager(prevMgr)
		SetProcessBus(prevBus)
	})
}

func enabledManager() *TransparencyManager {
	return NewTransparencyManager(&config.TransparencyConfig{
		Enabled:            true,
		ShardPhases:        true,
		SafetyExplanations: true,
		JITExplain:         true,
		OperationSummaries: true,
		VerboseErrors:      true,
	})
}

func TestReportDeny_WhenProcessManagerEnabled_ShouldRecordViolation(t *testing.T) {
	tm := enabledManager()
	withProcessHandles(t, tm, nil)

	v := ReportDeny("/delete_file", "/etc/passwd", "permitted")
	if v == nil {
		t.Fatal("expected a violation from the process manager")
	}
	if v.Rule != "permitted" || v.Target != "/etc/passwd" {
		t.Fatalf("violation lost its context: %+v", v)
	}

	recent := tm.SafetyReporter().GetRecentViolations(5)
	if len(recent) != 1 {
		t.Fatalf("expected the denial in violation history, got %d entries", len(recent))
	}
	if !strings.Contains(tm.GetStatus(), "Recent Safety Blocks") {
		t.Error("expected /transparency status to surface the auto-reported block")
	}
}

func TestReportDeny_WhenNoProcessManager_ShouldBeNoop(t *testing.T) {
	withProcessHandles(t, nil, nil)

	if v := ReportDeny("/exec_cmd", "rm -rf /", "constitution"); v != nil {
		t.Fatalf("expected nil violation with no manager, got %+v", v)
	}
}

func TestReportDeny_WhenBusRegistered_ShouldEmitControlEvent(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	ch := bus.Subscribe()
	defer bus.Close()
	withProcessHandles(t, enabledManager(), bus)

	ReportDeny("/write_file", ".env", "permitted")

	select {
	case evt := <-ch:
		if evt.Category != CategoryControl {
			t.Fatalf("expected control category, got %s", evt.Category)
		}
		if !strings.Contains(evt.Summary, "DENIED") || !strings.Contains(evt.Summary, ".env") {
			t.Fatalf("unexpected summary: %s", evt.Summary)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a Glass Box event for the denial")
	}
}

func TestEmitJIT_WhenJITExplainOn_ShouldEmitJITEvent(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	ch := bus.Subscribe()
	defer bus.Close()
	withProcessHandles(t, enabledManager(), bus)

	if !JITExplainEnabled() {
		t.Fatal("expected JIT explain to be reported as enabled")
	}
	EmitJIT("42 atoms", "details", "coder-1", 5*time.Millisecond)

	select {
	case evt := <-ch:
		if evt.Category != CategoryJIT {
			t.Fatalf("expected jit category, got %s", evt.Category)
		}
		if evt.Source != "coder-1" {
			t.Fatalf("expected source to survive, got %q", evt.Source)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a JIT Glass Box event")
	}
}

func TestEmitJIT_WhenJITExplainOff_ShouldNotEmit(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	ch := bus.Subscribe()
	defer bus.Close()

	tm := NewTransparencyManager(&config.TransparencyConfig{Enabled: true, JITExplain: false})
	withProcessHandles(t, tm, bus)

	if JITExplainEnabled() {
		t.Fatal("JIT explain should be off")
	}
	EmitJIT("42 atoms", "details", "coder-1", 0)

	select {
	case evt := <-ch:
		t.Fatalf("expected no event when JIT explain is off, got %+v", evt)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEmitJIT_WhenTransparencyDisabled_ShouldNotEmit(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	ch := bus.Subscribe()
	defer bus.Close()

	// JITExplain set but master toggle off: the master toggle wins.
	tm := NewTransparencyManager(&config.TransparencyConfig{Enabled: false, JITExplain: true})
	withProcessHandles(t, tm, bus)

	EmitJIT("42 atoms", "details", "coder-1", 0)

	select {
	case evt := <-ch:
		t.Fatalf("expected no event while transparency is disabled, got %+v", evt)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRecordOperation_WhenProcessManagerSet_ShouldReachStatus(t *testing.T) {
	tm := enabledManager()
	withProcessHandles(t, tm, nil)

	RecordOperation(types.OperationRecord{
		Operation: "coder shard",
		Outcome:   "Success",
		Duration:  1500 * time.Millisecond,
		Source:    "coder-1",
	})

	status := tm.GetStatus()
	if !strings.Contains(status, "Recent Operations") || !strings.Contains(status, "coder shard") {
		t.Fatalf("expected the operation in status, got:\n%s", status)
	}
}

func TestNewTransparencyManager_ShouldAdoptProcessManagerWhenUnset(t *testing.T) {
	prev := SetProcessManager(nil)
	t.Cleanup(func() { SetProcessManager(prev) })

	tm := NewTransparencyManager(nil)
	if ProcessManager() != tm {
		t.Fatal("expected the first constructed manager to become the process manager")
	}

	// A second manager must not steal the registration.
	other := NewTransparencyManager(nil)
	if ProcessManager() == other {
		t.Fatal("expected first-writer-wins registration")
	}
}

func TestNewGlassBoxEventBus_ShouldAdoptProcessBusWhenUnset(t *testing.T) {
	prev := SetProcessBus(nil)
	t.Cleanup(func() { SetProcessBus(prev) })

	bus := NewGlassBoxEventBus()
	defer bus.Close()
	if ProcessBus() != bus {
		t.Fatal("expected the first constructed bus to become the process bus")
	}
}
