package chat

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/observation"
	"codenerd/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

// TestSaveSessionStateCmd_NilWhenIncomplete pins the contract that the
// asynchronous persistence Cmd is a no-op when there is no session to
// save. This guarantees that Update() callers can unconditionally batch
// the Cmd without paying for an unnecessary tea.Msg round-trip during
// boot.
func TestSaveSessionStateCmd_NilWhenIncomplete(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		workspace string
		sessionID string
	}{
		{name: "empty workspace and session", workspace: "", sessionID: ""},
		{name: "missing session id", workspace: t.TempDir(), sessionID: ""},
		{name: "missing workspace", workspace: "", sessionID: "sess-1"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := Model{}
			m.workspace = tc.workspace
			m.sessionID = tc.sessionID
			if cmd := m.saveSessionStateCmd(); cmd != nil {
				t.Errorf("expected nil Cmd when persistence is impossible, got %T", cmd)
			}
		})
	}
}

// TestSaveSessionStateCmd_DoesNotBlock proves that returning the Cmd
// from Update() is cheap — the heavy work is deferred into the goroutine
// that Bubbletea schedules for the Cmd, not executed inline. The bug we
// are guarding against here is the previously-synchronous saveSessionState
// call inside Update() that blocked the event loop for ~360ms.
//
// We construct the Cmd against a temp workspace with no compressor or
// localDB wired (the persistence path short-circuits cleanly in that
// case) and measure the time to obtain the Cmd; the goroutine-scheduled
// inner closure may take longer but must run off-thread.
func TestSaveSessionStateCmd_DoesNotBlock(t *testing.T) {
	t.Parallel()

	m := Model{}
	m.workspace = t.TempDir()
	m.sessionID = "sess-test"

	// Obtaining the Cmd must be effectively instant — the regression this
	// guards was ~360ms of synchronous work inside Update(). A single
	// wall-clock sample under a fully loaded parallel test run flakes on
	// scheduler noise alone, so take the minimum of three constructions:
	// noise delays one sample, but only genuinely inline work delays all
	// three. The 100ms bound keeps a >3x margin below the 360ms regression.
	var cmd tea.Cmd
	minElapsed := time.Duration(1<<63 - 1)
	for i := 0; i < 3; i++ {
		start := time.Now()
		cmd = m.saveSessionStateCmd()
		if elapsed := time.Since(start); elapsed < minElapsed {
			minElapsed = elapsed
		}
	}

	if cmd == nil {
		t.Fatal("expected non-nil Cmd when workspace and sessionID are set")
	}

	if minElapsed > 100*time.Millisecond {
		t.Errorf("saveSessionStateCmd() blocked for %v (min of 3); expected <100ms (work should be deferred)", minElapsed)
	}

	// Executing the Cmd must produce the no-op signal message. Run it on
	// a helper goroutine with a generous timeout so a hung implementation
	// fails the test rather than the whole suite.
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg := cmd()
		if _, ok := msg.(sessionStatePersistedMsg); !ok {
			t.Errorf("expected sessionStatePersistedMsg, got %T", msg)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("saveSessionStateCmd() goroutine did not complete within 5s")
	}
	wg.Wait()
}

// TestSaveSessionStateCmd_HistorySnapshotIsolated guards against a
// subtle race: the deferred Cmd captures a snapshot of m.history. If the
// snapshot were a shallow slice reference, a subsequent m.addMessage()
// append in the next Update() tick could mutate the underlying array
// while the persistence goroutine was iterating it.
//
// We mutate the model's history slice after constructing the Cmd and
// verify the Cmd still completes without panic. Combined with -race in
// CI, this catches accidental sharing of the slice backing array.
func TestSaveSessionStateCmd_HistorySnapshotIsolated(t *testing.T) {
	t.Parallel()

	m := Model{}
	m.workspace = t.TempDir()
	m.sessionID = "sess-iso"
	m.history = []Message{{Role: "user", Content: "first"}}

	cmd := m.saveSessionStateCmd()
	if cmd == nil {
		t.Fatal("expected non-nil Cmd")
	}

	// Mutate the source after the snapshot has been captured.
	m.history = append(m.history, Message{Role: "assistant", Content: "second"})

	// Drive the Cmd to completion; failure here means the snapshot
	// wasn't isolated (would also trip -race).
	done := make(chan struct{})
	go func() {
		_ = cmd()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Cmd did not complete after concurrent history mutation")
	}
}

func TestUpdate_SessionStatePersistedIsNoop(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		showError  bool
		focusError bool
	}{
		{name: "successful save does not open an error panel"},
		{name: "successful save preserves an existing error", err: errors.New("delegated turn failed"), showError: true, focusError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTestModel()
			defer m.shutdownCancel()
			m = m.addMessage(Message{Role: "user", Content: "hello"})
			m.err, m.showError, m.focusError = tc.err, tc.showError, tc.focusError
			before := len(m.history)

			updated, cmd := m.Update(sessionStatePersistedMsg{})
			result, ok := updated.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", updated)
			}
			if result.err != tc.err || result.showError != tc.showError || result.focusError != tc.focusError {
				t.Fatalf("successful save changed error presentation: err=%v showError=%v focusError=%v; want err=%v showError=%v focusError=%v", result.err, result.showError, result.focusError, tc.err, tc.showError, tc.focusError)
			}
			if len(result.history) != before {
				t.Fatalf("persisted signal must not touch history: %d -> %d", before, len(result.history))
			}
			if cmd != nil {
				if msg := cmd(); msg != nil {
					t.Fatalf("persisted signal must not chain commands, got %T", msg)
				}
			}
		})
	}
}

// Terminal messages must reach both the live conversation and its on-disk
// history without another user message. Drive the returned commands as Bubble
// Tea does, rather than calling saveSessionState explicitly.
func TestUpdate_DelegatedOutcomeSavedAtTurnEnd(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		msg   tea.Msg
		want  string
		panel bool
	}{
		{name: "done", msg: assistantMsg{Surface: "Done: the requested behavior was verified."}, want: "Done:"},
		{name: "not verified", msg: assistantMsg{Surface: "Not verified: acceptance checks did not run."}, want: "acceptance checks did not run"},
		{name: "verified execution failed", msg: errorMsg(failedVerifiedDelegation("verified execution failed", errors.New("final suite fails: TestExecutiveLiteralBudget"))), want: "TestExecutiveLiteralBudget", panel: true},
		{name: "delegation failed", msg: errorMsg(failedDelegation("shard delegation failed", errors.New("provider unavailable"), observation.Return{})), want: "provider unavailable", panel: true},
		{name: "continuation done", msg: continuationDoneMsg{stepCount: 2, summary: "Both coder passes verified the requested behavior.", outcome: continuationCompleted}, want: "complete and verified"},
		{name: "continuation not verified", msg: continuationDoneMsg{stepCount: 2, summary: "Acceptance checks did not run.", outcome: continuationUnverified}, want: "Acceptance checks did not run"},
		{name: "continuation failed", msg: continuationDoneMsg{stepCount: 2, summary: "Step 2 failed: final suite fails", outcome: continuationFailed}, want: "final suite fails"},
		{name: "continuation stopped", msg: continuationDoneMsg{stepCount: 2, summary: "Stopped by user (Ctrl+X)", outcome: continuationInterrupted}, want: "Stopped by user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := NewTestModel()
			defer m.shutdownCancel()
			m.workspace = t.TempDir()
			m.sessionID = "delegated-outcome"
			m.isLoading = true
			m.isStreaming = true
			m.currentStream = "unfinished attempt"
			m = m.addMessage(Message{Role: "user", Content: "fix the requested behavior", Time: time.Now()})
			request := m.history[0].Request
			updated, cmd := m.Update(answerMsg{Request: request, Msg: tc.msg})
			got := updated.(Model)
			if got.isLoading {
				t.Error("terminal outcome left the chat loading")
			}
			if got.isStreaming || got.currentStream != "" || got.currentThought != "" {
				t.Error("terminal outcome retained an unfinished stream")
			}
			if len(got.history) != 2 {
				t.Fatalf("terminal outcome must add one conversation message, got %+v", got.history)
			}
			last := got.history[1]
			if last.Role != "assistant" || !strings.Contains(last.Content, tc.want) || last.Answers != request {
				t.Fatalf("outcome lost its reason or request identity: %+v", last)
			}
			if tc.panel && (!got.showError || !strings.Contains(last.Content, "Failed")) {
				t.Fatalf("failure must remain in the panel and conversation: panel=%v message=%q", got.showError, last.Content)
			}
			saved := false
			var run func(tea.Cmd)
			run = func(c tea.Cmd) {
				if c == nil {
					return
				}
				switch msg := c().(type) {
				case tea.BatchMsg:
					for _, child := range msg {
						run(child)
					}
				case sessionStatePersistedMsg:
					saved = true
				default:
					t.Fatalf("unexpected terminal command result %T", msg)
				}
			}
			run(cmd)
			if !saved {
				t.Fatal("turn ended without scheduling a session save")
			}
			history, err := session.LoadSessionHistory(got.workspace, got.sessionID)
			if err != nil {
				t.Fatalf("load history before any further user input: %v", err)
			}
			if len(history.Messages) != len(got.history) {
				t.Fatalf("saved %d messages, want %d", len(history.Messages), len(got.history))
			}
			persisted := history.Messages[1]
			if persisted.Content != last.Content || persisted.Role != last.Role || persisted.Answers != request {
				t.Fatalf("saved outcome differs from conversation: %+v, want %+v", persisted, last)
			}
		})
	}
}

// A turn's outcome remains in memory even if the immediate disk save fails;
// the asynchronous failure must be visible without starting another save loop.
func TestSaveSessionStateCmd_ReportsWriteFailure(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	defer m.shutdownCancel()
	m.workspace = t.TempDir()
	m.sessionID = "save-failure"
	m = m.addMessage(Message{Role: "assistant", Content: "Not verified: checks did not run."})
	// A regular file at .nerd prevents persistence without relying on permissions.
	if err := os.WriteFile(filepath.Join(m.workspace, ".nerd"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := m.saveSessionStateCmd()
	if cmd == nil {
		t.Fatal("missing turn-end save")
	}
	updated, follow := m.Update(cmd())
	got := updated.(Model)
	if !got.showError || got.err == nil || !strings.Contains(got.err.Error(), "save session") {
		t.Fatalf("save failure not presented: showError=%v err=%v", got.showError, got.err)
	}
	if len(got.history) != 1 || got.history[0].Content != m.history[0].Content {
		t.Fatalf("save failure replaced the outcome: %+v", got.history)
	}
	if follow != nil {
		t.Fatal("save failure must not schedule a recursive save")
	}
}
