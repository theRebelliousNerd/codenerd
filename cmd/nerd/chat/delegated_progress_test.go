package chat

import (
	"codenerd/internal/sessionlog"
	"context"
	"fmt"
	"github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"time"
)

func TestDelegatedProgress_ReachesChatBeforeAgentFinishes(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	defer m.shutdownCancel()
	m.sessionID = "live-chat"
	m.glassBoxEnabled = false
	m.isLoading = true
	m.delegatedEvents = make(chan sessionlog.LiveEvent, 128)
	ctx, cancel := m.sessionOperationContext()
	defer cancel()
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = sessionlog.RunAgent(ctx, "coder", "1", "fix loader", func(ctx context.Context) (string, error) {
			tr := sessionlog.TranscriptFrom(ctx)
			tr.Assistant(ctx, "model", &sessionlog.Response{Text: []string{"Plan: read loader, fix it, run tests"}})
			sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "step", Text: "Step 1/2: fix loader"})
			sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "tool_started", Tool: "read_file", CallID: "r", Text: "internal/loader.go"})
			select {
			case <-release:
			case <-ctx.Done():
			}
			return "done", nil
		})
	}()
	defer func() { close(release); <-done }()
	for i := 0; i < 4; i++ {
		cmd := m.listenDelegatedProgress()
		received := make(chan tea.Msg, 1)
		go func() { received <- cmd() }()
		select {
		case msg := <-received:
			next, nextCmd := m.Update(msg)
			m = next.(Model)
			if nextCmd == nil {
				t.Fatal("listener was not rearmed")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("progress hidden while agent still running")
		}
	}
	select {
	case <-done:
		t.Fatal("agent must still be working")
	default:
	}
	for _, want := range []string{"Plan: read loader", "Step 1/2", "RUN read_file", "internal/loader.go"} {
		if got := m.renderHistory(); !strings.Contains(got, want) {
			t.Errorf("live scrollback lacks %q: %s", want, got)
		}
	}
	if got := m.renderActivityLine(); !strings.Contains(got, "read_file") || !strings.Contains(got, "Step 1/2") {
		t.Fatalf("current action missing: %s", got)
	}
}

func TestDelegatedProgress_StepAndToolClocksAndResults(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed=%v", failed), func(t *testing.T) {
			m := NewTestModel(WithSize(100, 50))
			defer m.shutdownCancel()
			m.sessionID = "clock"
			m.wrapWidth = 96
			at := time.Unix(100, 0)
			m.handleDelegatedProgress(sessionlog.LiveEvent{Session: "clock", Agent: "coder-1", Kind: "step", Text: "Step 2/3: tests", At: at})
			m.handleDelegatedProgress(sessionlog.LiveEvent{Session: "clock", Agent: "coder-1", Kind: "tool_started", Tool: "run_tests", Text: "./internal/session", CallID: "t", At: at.Add(10 * time.Second)})
			line := m.renderDelegatedProgress(at.Add(25 * time.Second))
			for _, want := range []string{"Step 2/3", "25.0s", "run_tests", "15.0s"} {
				if !strings.Contains(line, want) {
					t.Errorf("clock line lacks %s: %s", want, line)
				}
			}
			m.handleDelegatedProgress(sessionlog.LiveEvent{Session: "clock", Agent: "coder-1", Kind: sessionlog.EventToolResult, Status: "PASS", Tool: "run_tests", Text: "./internal/session", CallID: "t", Failed: failed, At: at.Add(30 * time.Second)})
			status := "PASS"
			if failed {
				status = "FAIL"
			}
			last := m.history[len(m.history)-1].Content
			if !strings.Contains(last, status+" run_tests ./internal/session (20.0s)") {
				t.Fatalf("result missing: %s", last)
			}
			m.handleDelegatedProgress(sessionlog.LiveEvent{Session: "clock", Agent: "coder-1", Kind: sessionlog.EventFinal, At: at.Add(31 * time.Second)})
			if got := m.renderDelegatedProgress(at.Add(time.Minute)); got != "" {
				t.Fatalf("finished agent still active: %s", got)
			}
		})
	}
}

func TestDelegatedProgress_IgnoresOtherSessionsAndUnblocksOnShutdown(t *testing.T) {
	m := NewTestModel()
	m.sessionID = "current"
	m.delegatedEvents = make(chan sessionlog.LiveEvent, 1)
	m.handleDelegatedProgress(sessionlog.LiveEvent{Session: "elsewhere", Agent: "coder", Kind: "step", Text: "must not leak"})
	if len(m.history) != 0 || len(m.delegatedActive) != 0 {
		t.Fatal("foreign session reached this chat")
	}
	m.delegatedEvents <- sessionlog.LiveEvent{}
	done := make(chan struct{})
	go func() { defer close(done); m.sendDelegatedProgress(context.Background(), sessionlog.LiveEvent{}) }()
	m.shutdownCancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("full progress queue blocked shutdown")
	}
	if got := m.listenDelegatedProgress(); got == nil {
		t.Fatal("listener missing")
	}
}

func TestDelegatedProgress_CancelledAgentStillRetiresItsClock(t *testing.T) {
	m := NewTestModel()
	defer m.shutdownCancel()
	m.sessionID = "cancel"
	m.delegatedEvents = make(chan sessionlog.LiveEvent, 1)
	ctx, cancel := m.sessionOperationContext()
	ctx = sessionlog.WithAgent(ctx, "coder", "cancelled")
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "step", Text: "waiting"})
	cancel()
	done := make(chan struct{})
	go func() { defer close(done); sessionlog.TranscriptFrom(ctx).Final(ctx, "", ctx.Err()) }()
	for i := 0; i < 2; i++ {
		select {
		case ev := <-m.delegatedEvents:
			m.handleDelegatedProgress(ev)
		case <-time.After(time.Second):
			t.Fatal("cancelled run lost its terminal update")
		}
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("final progress blocked cancellation")
	}
	if len(m.delegatedActive) != 0 {
		t.Fatal("cancelled agent kept a running clock")
	}
}

func TestDelegatedProgress_PinnedPanelStaysShort(t *testing.T) {
	m := NewTestModel(WithSize(80, 40))
	defer m.shutdownCancel()
	m.sessionID, m.wrapWidth = "scan", 72
	for i := 0; i < 5; i++ {
		m.handleDelegatedProgress(sessionlog.LiveEvent{Session: "scan", Agent: fmt.Sprintf("coder-%d", i), Kind: "step", Text: "Step 1/3: " + strings.Repeat("long plan ", 60), At: time.Now().Add(-time.Minute)})
	}
	panel := m.renderDelegatedProgress(time.Now())
	if strings.Count(panel, "\n") > 6 || !strings.Contains(panel, "+2 agents active") {
		t.Fatalf("unscannable panel: %s", panel)
	}
	if !strings.Contains(panel, "1m00s") || !strings.Contains(panel, "Step 1/3") {
		t.Fatalf("step/duration hidden: %s", panel)
	}
	if len(m.history) != 5 || !strings.Contains(m.history[4].Content, strings.Repeat("long plan ", 40)) {
		t.Fatal("compact pinned panel discarded full chat observations")
	}
}

func TestDelegatedProgress_PanelUsesActivityBudget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		agents  int
		visible int
	}{
		{name: "empty"},
		{name: "below capacity", agents: maxActivityTrail - 1, visible: maxActivityTrail - 1},
		{name: "at capacity", agents: maxActivityTrail, visible: maxActivityTrail},
		{name: "above capacity", agents: maxActivityTrail + 1, visible: maxActivityTrail - 1},
		{name: "several overflow agents", agents: maxActivityTrail + 3, visible: maxActivityTrail - 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTestModel(WithSize(80, 40))
			defer m.shutdownCancel()
			m.sessionID, m.wrapWidth = "budget", 72
			at := time.Unix(100, 0)
			for i := 0; i < tc.agents; i++ {
				m.handleDelegatedProgress(sessionlog.LiveEvent{Session: "budget", Agent: fmt.Sprintf("coder-%02d", i), Kind: "step", Text: "Step 1/2: inspect loader", At: at})
			}
			panel := m.renderDelegatedProgress(at.Add(25 * time.Second))
			if tc.agents == 0 && panel != "" {
				t.Fatalf("empty panel = %q", panel)
			}
			if got := strings.Count(panel, "Step 1/2: inspect loader"); got != tc.visible {
				t.Errorf("visible agent blocks = %d, want %d: %s", got, tc.visible, panel)
			}
			if tc.agents > tc.visible {
				want := fmt.Sprintf("+%d agents active; activity in chat", tc.agents-tc.visible)
				if !strings.Contains(panel, want) {
					t.Errorf("missing overflow summary %q: %s", want, panel)
				}
			} else if strings.Contains(panel, "agents active") {
				t.Errorf("unnecessary overflow summary: %s", panel)
			}
			for i := 0; i < tc.agents; i++ {
				agent := fmt.Sprintf("coder-%02d", i)
				if got := strings.Contains(panel, agent); got != (i < tc.visible) {
					t.Errorf("agent %s visibility = %v: %s", agent, got, panel)
				}
				if !strings.Contains(m.history[i].Content, "["+agent+"] Step 1/2: inspect loader") {
					t.Errorf("full observation for %s missing from history", agent)
				}
			}
			if len(m.history) != tc.agents || len(m.delegatedActive) != tc.agents {
				t.Errorf("panel bound discarded observations or active agents: history=%d active=%d", len(m.history), len(m.delegatedActive))
			}
			if tc.visible > 0 && strings.Count(panel, "25.0s") != tc.visible*2 {
				t.Errorf("step and action durations not visible: %s", panel)
			}
		})
	}
}

func TestDelegatedProgress_UpdateReservesAndReleasesPanelSpace(t *testing.T) {
	for _, size := range []struct{ width, height int }{{100, 50}, {80, 40}, {20, 5}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			m := NewTestModel(WithSize(size.width, size.height))
			defer m.shutdownCancel()
			m.sessionID = "layout"
			m.glassBoxEnabled = false
			m.delegatedEvents = make(chan sessionlog.LiveEvent, maxActivityTrail)
			m = layoutResize(t, m, size.width, size.height)
			idleHeight := m.viewport.Height
			at := time.Unix(100, 0)
			for i := 0; i < maxActivityTrail; i++ {
				next, cmd := m.Update(delegatedProgressMsg(sessionlog.LiveEvent{Session: "layout", Agent: fmt.Sprintf("coder-%d", i), Kind: "step", Text: "Step 1/2: inspect loader", At: at}))
				m = next.(Model)
				if cmd == nil {
					t.Fatal("Update did not rearm live listener")
				}
			}
			m = layoutResize(t, m, size.width+10, size.height+10)
			m = layoutResize(t, m, size.width, size.height)
			panel := m.renderActivityLine()
			if strings.Contains(panel, "agents active") || !strings.Contains(panel, "coder-3") {
				t.Fatalf("at-capacity Update hid an agent: %s", panel)
			}
			panelHeight := strings.Count(panel, "\n") + 1
			if want := max(idleHeight-panelHeight, 1); m.viewport.Height != want {
				t.Errorf("viewport height with panel = %d, want %d", m.viewport.Height, want)
			}
			for i := 0; i < maxActivityTrail; i++ {
				next, _ := m.Update(delegatedProgressMsg(sessionlog.LiveEvent{Session: "layout", Agent: fmt.Sprintf("coder-%d", i), Kind: sessionlog.EventFinal, Text: "done", At: at.Add(time.Minute)}))
				m = next.(Model)
			}
			if panel := m.renderActivityLine(); panel != "" {
				t.Errorf("finished panel = %q", panel)
			}
			if m.viewport.Height != idleHeight {
				t.Errorf("finished agents did not release panel space: height=%d, want %d", m.viewport.Height, idleHeight)
			}
			if len(m.history) != maxActivityTrail*2 {
				t.Errorf("Update discarded progress observations: history=%d", len(m.history))
			}
		})
	}
}

func TestDelegatedProgress_BackpressuredFinalSurvivesCancellationAndUnblocksOnShutdown(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(fmt.Sprintf("shutdown=%v", shutdown), func(t *testing.T) {
			m := NewTestModel()
			m.delegatedEvents = make(chan sessionlog.LiveEvent, 1)
			queued := sessionlog.LiveEvent{Kind: "step", Text: "already queued"}
			m.delegatedEvents <- queued
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			terminal := sessionlog.LiveEvent{Kind: sessionlog.EventFinal, Agent: "coder-cancelled", Text: "cancelled", Failed: true}
			started := make(chan struct{})
			done := make(chan struct{})
			t.Cleanup(func() {
				m.shutdownCancel()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("terminal sender did not stop during cleanup")
				}
			})
			go func() {
				defer close(done)
				close(started)
				m.sendDelegatedProgress(ctx, terminal)
			}()
			<-started
			// Keep the channel full while the cancelled sender runs. Draining it
			// immediately could exercise only the initial nonblocking send.
			select {
			case <-done:
				t.Fatal("cancelled terminal sender returned before delivery or shutdown")
			case <-time.After(50 * time.Millisecond):
			}
			if shutdown {
				m.shutdownCancel()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("shutdown did not unblock the backpressured terminal sender")
				}
				if got := <-m.delegatedEvents; got != queued {
					t.Fatalf("shutdown changed the queued event: %+v", got)
				}
				return
			}
			if got := <-m.delegatedEvents; got != queued {
				t.Fatalf("queued event changed: %+v", got)
			}
			select {
			case got := <-m.delegatedEvents:
				if got != terminal {
					t.Fatalf("terminal event changed: %+v", got)
				}
			case <-time.After(time.Second):
				t.Fatal("terminal event was lost to agent cancellation")
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("terminal sender remained blocked after delivery")
			}
		})
	}
}

// Init must schedule the live listener, not merely expose a helper that tests
// can call themselves. Execute its UI startup batch; the final command is the
// system boot command, which this offline listener test deliberately does not run.
func TestModelInit_SchedulesDelegatedProgressListener(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	defer m.shutdownCancel()
	m.sessionID = "init-listener"
	m.glassBoxEnabled = false
	m.delegatedEvents = make(chan sessionlog.LiveEvent, 1)
	m.statusChan = make(chan string, 1)
	m.statusChan <- "ready"
	ctx, cancel := m.sessionOperationContext()
	defer cancel()
	ctx = sessionlog.WithAgent(ctx, "coder", "startup")
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventAssistant, Text: "Inspect loader before editing"})

	startup := m.Init()
	if startup == nil {
		t.Fatal("Init returned no startup commands")
	}
	batch, ok := startup().(tea.BatchMsg)
	if !ok || len(batch) < 2 {
		t.Fatal("Init did not return its UI/boot startup batch")
	}
	var progress tea.Msg
	for _, cmd := range batch[:len(batch)-1] {
		if cmd == nil {
			continue
		}
		msg := cmd()
		if _, ok := msg.(delegatedProgressMsg); ok {
			if progress != nil {
				t.Fatal("Init scheduled more than one delegated listener")
			}
			progress = msg
		}
	}
	if progress == nil {
		t.Fatal("Init did not schedule a listener that delivers delegated activity")
	}
	next, rearm := m.Update(progress)
	m = next.(Model)
	if rearm == nil {
		t.Fatal("startup listener did not rearm after delivering progress")
	}
	if got := m.renderHistory(); !strings.Contains(got, "Inspect loader before editing") {
		t.Fatalf("Init's listener did not deliver the agent message to chat: %s", got)
	}
}

// Use the production constructor without installing a queue in the test. If
// InitChat stops allocating the live transport, scoped agent work cannot reach
// the UI even though listenDelegatedProgress itself still works in isolation.
func TestInitChat_CreatesWorkingDelegatedProgressTransport(t *testing.T) {
	t.Chdir(t.TempDir())
	m := InitChat(Config{})
	defer m.shutdownCancel()
	t.Cleanup(func() {
		if m.usageTracker != nil {
			if err := m.usageTracker.Close(); err != nil {
				t.Errorf("close usage tracker: %v", err)
			}
		}
	})
	m.sessionID = "constructed-chat"
	ctx, cancel := m.sessionOperationContext()
	defer cancel()
	ctx = sessionlog.WithAgent(ctx, "coder", "constructed")
	if !sessionlog.HasLiveProgress(ctx) {
		t.Fatal("InitChat did not create the transport needed by delegated operation contexts")
	}
	if cap(m.delegatedEvents) == 0 {
		t.Fatal("InitChat's progress transport must buffer worker events until the UI receives them")
	}
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventAssistant, Text: "Read parser, then test it"})
	listen := m.listenDelegatedProgress()
	if listen == nil {
		t.Fatal("constructed chat has no live listener")
	}
	msg, ok := listen().(delegatedProgressMsg)
	if !ok {
		t.Fatal("constructed transport did not deliver a delegated progress message")
	}
	ev := sessionlog.LiveEvent(msg)
	if ev.Session != m.sessionID || ev.Agent != "coder-constructed" || ev.Text != "Read parser, then test it" {
		t.Fatalf("constructed transport lost agent activity or ownership: %+v", ev)
	}
	m.handleDelegatedProgress(ev)
	if len(m.history) != 1 || !strings.Contains(m.history[0].Content, "Read parser, then test it") {
		t.Fatalf("constructed chat did not retain live agent activity: %+v", m.history)
	}
}
