package chat

import (
	"sync"
	"testing"
	"time"

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
