package chat

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/session"
)

// background_ownership_test.go: a finished background job settles once, in
// the session that started it, and its chain's next step is queued, run or
// canceled -- never dropped because the chat happened to be busy.

func chainJob(id string) backgroundJob {
	return backgroundJob{ID: id, Shard: "reviewer", Task: "diagnose the log", Started: time.Now(), FollowUp: "Fix what the diagnosis found, with tests."}
}

func TestBackground_ABusyChatQueuesTheNextStep(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	job := chainJob("bg-busy")
	m.background.add(job, nil)
	m.isLoading = true
	m, cmd := m.finishBackground(backgroundDoneMsg{Job: job, Surface: "two causes", Result: "two causes"})
	if cmd != nil {
		t.Fatal("the next step started under a running turn")
	}
	if m.turn.queuedCount() != 1 || !strings.Contains(lastAssistant(m), "next step is queued") {
		t.Fatalf("queued=%d reply=%q: the next step was dropped", m.turn.queuedCount(), lastAssistant(m))
	}
	next, _ := m.turn.dequeue()
	if !strings.HasPrefix(next, job.FollowUp) || !strings.Contains(next, "two causes") {
		t.Fatalf("queued step = %q", next)
	}
}

func TestBackground_TwoChainsFinishingTogetherKeepBothNextSteps(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	a, b := chainJob("bg-a"), chainJob("bg-b")
	m.background.add(a, nil)
	m.background.add(b, nil)
	m, first := m.finishBackground(backgroundDoneMsg{Job: a, Surface: "a", Result: "a"})
	m, second := m.finishBackground(backgroundDoneMsg{Job: b, Surface: "b", Result: "b"})
	if first == nil || second != nil || m.turn.queuedCount() != 1 {
		t.Fatalf("first started=%v second started=%v queued=%d", first != nil, second != nil, m.turn.queuedCount())
	}
}

// A stop that came after the work returned: the result stands, the next
// step does not start.
func TestBackground_AStopAfterTheWorkReturnedCancelsTheNextStep(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	job := chainJob("bg-stop-late")
	m.background.add(job, func() {})
	m.background.cancelAll()
	m, cmd := m.finishBackground(backgroundDoneMsg{Job: job, Surface: "two causes", Result: "two causes"})
	if cmd != nil || m.isLoading || m.turn.queuedCount() != 0 {
		t.Fatal("a stopped chain started its next step")
	}
	reply := lastAssistant(m)
	if !strings.Contains(reply, "Stopped background reviewer") || !strings.Contains(reply, "two causes") || !strings.Contains(reply, "next step was not started") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestBackground_ADuplicateDoneSettlesOnce(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	job := chainJob("bg-dup")
	m.background.add(job, nil)
	done := backgroundDoneMsg{Job: job, Surface: "found it", Result: "found it"}
	m, _ = m.finishBackground(done)
	m.isLoading = false
	n := len(m.history)
	m, cmd := m.finishBackground(done)
	if cmd != nil || len(m.history) != n {
		t.Fatal("a second done message was settled again")
	}
}

// A job started in session A finishes while session B is open: A gets the
// result, B gets a notice, and B does not run A's next step.
func TestBackground_AResultSettlesInTheSessionThatStartedIt(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	if err := session.SaveSessionHistory(m.workspace, "sess_a", []session.ChatMessage{{Role: "user", Content: "diagnose yourself"}}); err != nil {
		t.Fatal(err)
	}
	m.sessionID = "sess_a"
	job := chainJob("bg-origin")
	_ = m.startBackground(job, job.Task) // registers the job under sess_a
	job.Session = "sess_a"
	m.sessionID = "sess_b"

	m, cmd := m.finishBackground(backgroundDoneMsg{Job: job, Surface: "A's diagnosis", Result: "A's diagnosis"})
	if cmd != nil || m.isLoading || m.turn.queuedCount() != 0 {
		t.Fatal("session B adopted session A's next step")
	}
	if strings.Contains(lastAssistant(m), "A's diagnosis") || !strings.Contains(lastAssistant(m), "sess_a") {
		t.Fatalf("B shows %q", lastAssistant(m))
	}
	if m.lastShardResult != nil && strings.Contains(m.lastShardResult.RawOutput, "A's diagnosis") {
		t.Fatal("B's follow-up context took A's result")
	}
	saved, err := session.LoadSessionHistory(m.workspace, "sess_a")
	if err != nil {
		t.Fatal(err)
	}
	last := saved.Messages[len(saved.Messages)-1].Content
	if !strings.Contains(last, "A's diagnosis") || !strings.Contains(last, "next step was not started") {
		t.Fatalf("session A's history ends with %q", last)
	}
}

// The completion is saved like any other assistant message.
func TestBackground_CompletionSchedulesASessionSave(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	m.sessionID = "sess_save"
	job := backgroundJob{ID: "bg-save", Shard: "researcher", Task: "survey", Started: time.Now()}
	m.background.add(job, nil)
	_, cmd := m.Update(backgroundDoneMsg{Job: job, Surface: "found", Result: "found"})
	if !runsSave(cmd) {
		t.Fatal("a background completion is not saved")
	}
}

func runsSave(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case sessionStatePersistedMsg:
		return true
	case tea.BatchMsg:
		for _, c := range msg {
			if runsSave(c) {
				return true
			}
		}
	}
	return false
}

// A background result between turns does not shift the turns stored after
// it (SQLite session_history pairing).
func TestSessionTurns_AnUnsolicitedResultDoesNotShiftLaterTurns(t *testing.T) {
	history := []Message{
		{Role: "user", Content: "review auth"},
		{Role: "assistant", Content: "working on it in the background"},
		{Role: "assistant", Content: "Background reviewer finished"},
		{Role: "user", Content: "thanks, now fix it"},
		{Role: "assistant", Content: "fixed"},
	}
	got := sessionTurns(history)
	if len(got) != 2 || got[1].number != 1 || got[1].user != "thanks, now fix it" || got[1].assistant != "fixed" {
		t.Fatalf("turns = %+v", got)
	}
}
