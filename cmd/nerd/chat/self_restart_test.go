package chat

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/core"
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
	if err == nil || after.restartFailed == nil {
		t.Fatalf("a failed exec was not reported: %v", err)
	}
	if last := after.history[len(after.history)-1].Content; !strings.Contains(last, "Restart failed") || !strings.Contains(last, "keeps running") {
		t.Fatalf("the session was not told why: %q", last)
	}
	if again, cmd := after.handleRestartCheck(); again.restartRequest != nil || cmd != nil {
		t.Fatal("a failed restart is retried before the next rebuild")
	}
}

func TestSelfRestart_FailureIsScopedToTheAttemptedIdentity(t *testing.T) {
	if !selfrestart.ExecSupported {
		t.Skip("automatic exec is unavailable")
	}
	for _, replaceWatcher := range []bool{false, true} {
		t.Run(fmt.Sprintf("replace-watcher-%t", replaceWatcher), func(t *testing.T) {
			m, _ := journeyModel(t, config.WorkModeYolo)
			m.isBooting = false
			m.restarter = pendingRestarter(t, m, nil)
			b, _ := m.restarter.Pending()
			attempts := 0
			fail := func(string, []string, []string) error { attempts++; return errors.New("exec format error") }
			m, _ = m.handleRestartCheck()
			var err error
			m, err = m.performRestart(fail)
			if err == nil || attempts != 1 {
				t.Fatalf("first failure: attempts=%d err=%v", attempts, err)
			}
			if replaceWatcher {
				// Reopening the TUI can adopt a restarter whose baseline is B.
				m, _ = m.handleRebuildWatch(rebuildWatchMsg{restarter: selfrestart.NewRestarter(b, t.TempDir(), fail)})
			}
			before := len(m.history)
			duplicate := b
			duplicate.Build = "a label change alone"
			m, _ = m.handleBinaryRebuilt(binaryRebuiltMsg{id: duplicate})
			if len(m.history) != before {
				t.Fatal("a duplicate failed build promised another restart")
			}
			if same, cmd := m.handleRestartCheck(); same.restartRequest != nil || cmd != nil {
				t.Fatal("the failed identity was retried")
			}

			// Each new failed generation can recover again at a distinct build.
			for _, size := range []int64{12, 13} {
				c := b
				c.Size = size
				m, _ = m.handleBinaryRebuilt(binaryRebuiltMsg{id: c})
				for _, busy := range []string{"loading", "booting", "approval"} {
					blocked := m
					blocked.isLoading = busy == "loading"
					blocked.isBooting = busy == "booting"
					if busy == "approval" {
						blocked.viewMode = ApprovalView
					}
					if held, cmd := blocked.handleRestartCheck(); held.restartRequest != nil || cmd == nil {
						t.Fatalf("%s did not defer the new build", busy)
					}
				}
				m, _ = m.handleRestartCheck()
				if m.restartRequest == nil {
					t.Fatal("a distinct later build did not reach the idle gate")
				}
				m, err = m.performRestart(fail)
				if err == nil || m.restartFailed == nil || m.restartFailed.Changed(c) {
					t.Fatalf("later failure not bound to its attempted identity: %v", err)
				}
			}
			if attempts != 3 {
				t.Fatalf("exec attempts = %d, want 3", attempts)
			}
		})
	}
}

func TestSelfRestart_RestoreWriteFailureCanRecoverOnAnotherBuild(t *testing.T) {
	if !selfrestart.ExecSupported {
		t.Skip("automatic exec is unavailable")
	}
	m, _ := journeyModel(t, config.WorkModeYolo)
	m.isBooting = false
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(dir, []byte("blocks directory creation"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := selfrestart.Identity{Path: "/bin/nerd", Size: 10}
	m.restarter = selfrestart.NewRestarter(old, dir, nil)
	b := old
	b.Size++
	m, _ = m.handleBinaryRebuilt(binaryRebuiltMsg{id: b})
	m, _ = m.handleRestartCheck()
	attempts := 0
	fail := func(string, []string, []string) error { attempts++; return errors.New("exec failure") }
	m, err := m.performRestart(fail)
	if err == nil || attempts != 0 {
		t.Fatalf("restore failure = %v, exec attempts = %d", err, attempts)
	}
	if again, cmd := m.handleRestartCheck(); again.restartRequest != nil || cmd != nil {
		t.Fatal("restore failure retried unchanged build")
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	b.Inode++
	m, _ = m.handleBinaryRebuilt(binaryRebuiltMsg{id: b})
	m, _ = m.handleRestartCheck()
	if m.restartRequest == nil {
		t.Fatal("later build remained suppressed after restore failure")
	}
	m, err = m.performRestart(fail)
	if err == nil || attempts != 1 {
		t.Fatalf("later attempt = %v, exec attempts = %d", err, attempts)
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

func TestPolicyReload_OnlyARejectedGenerationClaimsUnchanged(t *testing.T) {
	for _, disposition := range []string{"rejected", ""} {
		m, _ := journeyModel(t, config.WorkModeYolo)
		m, _ = m.handlePolicyReload(policyReloadMsg{ev: nerdsystem.PolicyReloadEvent{
			Files: []string{"defaults/policy/x.mg"}, Err: errors.New("evaluation ceiling"),
			Reload: core.ConstitutionReload{Disposition: disposition, Active: "accepted", Candidate: "candidate", Stage: "evaluate"},
		}})
		last := m.history[len(m.history)-1].Content
		if strings.Contains(last, "unchanged") != (disposition == "rejected") {
			t.Fatalf("unsupported disposition claim: %s", last)
		}
		for _, want := range []string{"accepted", "candidate", "evaluate", "evaluation ceiling"} {
			if !strings.Contains(last, want) {
				t.Fatalf("missing %q in %s", want, last)
			}
		}
	}
}
