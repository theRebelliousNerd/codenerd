package chat

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/selfrestart"
	nerdsystem "codenerd/internal/system"
)

func pendingRestarter(t *testing.T, m Model, exec selfrestart.ExecFunc) *selfrestart.Restarter {
	t.Helper()
	old := selfrestart.Identity{Path: "/bin/nerd", Size: 10, ModTime: time.Unix(1700000000, 0), Inode: 1, Build: "aaaa"}
	r := selfrestart.NewRestarter(old, t.TempDir(), exec)
	r.BinaryChanged(selfrestart.Identity{Path: "/bin/nerd", Size: 11, ModTime: time.Unix(1700000100, 0), Inode: 2, Build: "bbbb"})
	return r
}

// A rebuild never interrupts a running turn: the session asks again while it
// is busy, and quits the TUI for the restart once it is idle.
func TestSelfRestart_WaitsForTheTurnToSettle(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	m.isBooting = false
	m.restarter = pendingRestarter(t, m, func(string, []string, []string) error { return nil })

	m.isLoading = true
	busy, cmd := m.handleRestartCheck()
	if busy.restartRequest != nil || cmd == nil {
		t.Fatal("a busy session was restarted, or stopped asking")
	}
	busy.isLoading = false
	idle, cmd := busy.handleRestartCheck()
	if idle.restartRequest == nil || cmd == nil {
		t.Fatal("an idle session did not ask for the restart")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Fatal("the restart does not quit the TUI (which restores the terminal) before the exec")
	}
}

// The session crosses the restart through --restore-session: conversation,
// yolo, brief, held work and draft come back, and the notice names the builds
// and what the restart interrupted.
func TestSelfRestart_TheSessionRoundTripsThroughTheRestoreFile(t *testing.T) {
	old, _ := journeyModel(t, config.WorkModePropose)
	old.isBooting = false
	old.sessionID = "sess-restart"
	old.yoloFlag = true
	old = old.pushUserMsg("fix the retry loop")
	old = old.pushAssistantMsg("Fixed the retry loop; the tests pass.")
	old.writeBrief(&perception.Brief{Goal: "make the retry loop stop on cancellation"})
	old.pendingWork = &pendingWork{Input: "then refactor the backoff", At: time.Now()}
	old.offeredSubtask = &Subtask{Description: "run the integration tests"}
	old.textarea.SetValue("half a thought")
	if err := old.saveSessionState(); err != nil {
		t.Fatalf("saveSessionState: %v", err)
	}
	var argv []string
	old.restarter = pendingRestarter(t, old, func(_ string, a []string, _ []string) error { argv = a; return nil })
	st := old.restartState()
	old.restartRequest = &st
	if _, err := old.performRestart(nil); err != nil {
		t.Fatalf("performRestart: %v", err)
	}
	if len(argv) < 2 || argv[len(argv)-2] != selfrestart.RestoreFlag {
		t.Fatalf("the exec does not carry the restore flag: %q", argv)
	}

	restore, err := readRestore(argv[len(argv)-1])
	if err != nil {
		t.Fatal(err)
	}
	fresh, _ := journeyModel(t, config.WorkModePropose)
	fresh.workspace = old.workspace
	fresh.sessionID = restore.SessionID
	fresh.restore = restore
	fresh = fresh.applyRestore()

	text := strings.Join(contents(fresh.history), "\n")
	for _, want := range []string{"fix the retry loop", "Fixed the retry loop", "codeNERD updated (aaaa → bbbb build), session restored", "the offered next step: run the integration tests", "then refactor the backoff"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the restored session lacks %q:\n%s", want, text)
		}
	}
	if fresh.workMode() != config.WorkModeYolo {
		t.Fatalf("work mode = %s, want the session's yolo", fresh.workMode())
	}
	if b := fresh.currentBrief(); b == nil || b.Goal != "make the retry loop stop on cancellation" {
		t.Fatalf("brief = %+v", b)
	}
	if fresh.pendingWork == nil || fresh.pendingWork.Input != "then refactor the backoff" || fresh.textarea.Value() != "half a thought" {
		t.Fatalf("held work %+v, draft %q", fresh.pendingWork, fresh.textarea.Value())
	}
}

func contents(history []Message) []string {
	out := make([]string, 0, len(history))
	for _, m := range history {
		out = append(out, m.Content)
	}
	return out
}

// An exec that fails keeps this process and session, says why, and stops
// retrying until the next rebuild.
func TestSelfRestart_AFailedExecKeepsTheSession(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	m.isBooting = false
	m.sessionID = "sess-keep"
	m.restarter = pendingRestarter(t, m, nil)
	st := m.restartState()
	m.restartRequest = &st
	after, err := m.performRestart(func(string, []string, []string) error { return errors.New("exec format error") })
	if err == nil || !after.restartFailed {
		t.Fatalf("a failed exec was not reported: %v", err)
	}
	if last := after.history[len(after.history)-1].Content; !strings.Contains(last, "Restart failed") || !strings.Contains(last, "keeps running") {
		t.Fatalf("the session was not told why: %q", last)
	}
	if again, cmd := after.handleRestartCheck(); again.restartRequest != nil || cmd != nil {
		t.Fatal("a failed restart is retried before the next rebuild")
	}
}

// A dev-mode policy reload is shown: live, or failed with the error and the
// running policy kept.
func TestPolicyReload_IsShownToTheSession(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	m, _ = m.handlePolicyReload(policyReloadMsg{ev: nerdsystem.PolicyReloadEvent{Files: []string{"defaults/policy/x.mg"}}})
	if last := m.history[len(m.history)-1].Content; !strings.Contains(last, "defaults/policy/x.mg is live") {
		t.Fatalf("a live reload was not shown: %q", last)
	}
	m, _ = m.handlePolicyReload(policyReloadMsg{ev: nerdsystem.PolicyReloadEvent{Files: []string{"defaults/policy/x.mg"}, Err: errors.New("parse error at line 3")}})
	if last := m.history[len(m.history)-1].Content; !strings.Contains(last, "Policy reload failed") || !strings.Contains(last, "parse error at line 3") {
		t.Fatalf("a failed reload was not shown: %q", last)
	}
}
