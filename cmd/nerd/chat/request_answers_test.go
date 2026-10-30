package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

// turnEnd runs a command the way the Bubble Tea runtime does and returns the
// message the turn ended with: spinner ticks and session saves are skipped,
// and a stream is awaited to its result.
func turnEnd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command to run")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if end := turnEndOrNil(t, c); end != nil {
				return end
			}
		}
		t.Fatal("the batch ended no turn")
	}
	return awaitProcessStream(t, msg)
}

func turnEndOrNil(t *testing.T, c tea.Cmd) tea.Msg {
	switch msg := c().(type) {
	case spinner.TickMsg, sessionStatePersistedMsg, nil:
		return nil
	case tea.BatchMsg:
		return turnEnd(t, func() tea.Msg { return msg })
	default:
		return awaitProcessStream(t, msg)
	}
}

// Two requests typed before the first was answered: the SQLite projection
// paired each user message with the first assistant message before the next
// user message, so the first request got no row and the second got its queue
// notice as its answer (audit queued-history-projection). Each request now
// carries an id, and its answer carries the same id, through the queue and
// the turn that runs it.
func TestSessionHistory_QueuedRequestsKeepTheirOwnAnswers(t *testing.T) {
	t.Parallel()
	a := perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, IsQuestion: true, Response: "Two places build the retry policy."}
	b := perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, IsQuestion: true, Response: "Its tests are in retry_test.go."}
	m, _ := journeyModel(t, config.WorkModePropose, a, b)
	db, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m.localDB = db

	m, cmdA := typeAndEnter(t, m, "where is the retry policy built?")
	m, cmdB := typeAndEnter(t, m, "and where are its tests?")
	if cmdB != nil || m.turn.queuedCount() != 1 {
		t.Fatalf("the second request was not queued behind the first (queued %d)", m.turn.queuedCount())
	}

	next, cmd := m.Update(turnEnd(t, cmdA))
	m = next.(Model)
	next, _ = m.Update(turnEnd(t, cmd))
	m = next.(Model)

	m.syncSessionToSQLite()
	rows, err := db.GetSessionHistory(m.sessionID, 20)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[fmt.Sprint(r["user_input"])] = fmt.Sprint(r["response"])
	}
	if !strings.Contains(got["where is the retry policy built?"], "Two places build the retry policy") {
		t.Errorf("the first request's stored answer is %q", got["where is the retry policy built?"])
	}
	if b := got["and where are its tests?"]; !strings.Contains(b, "retry_test.go") || strings.Contains(b, "Queued") {
		t.Errorf("the second request's stored answer is %q, want its own answer, not the queue notice", b)
	}
}

// The background audit's case, by id: an acknowledgement is not the answer;
// the job's result is. A message without a request id (a history saved
// before ids) is stored with no answer rather than a guessed one.
func TestSessionTurns_PairByRequestID(t *testing.T) {
	history := []Message{
		{Role: "user", Content: "review auth", Request: 1},
		{Role: "assistant", Content: "working on it in the background"},
		{Role: "user", Content: "thanks, now fix it", Request: 2},
		{Role: "assistant", Content: "Background reviewer finished", Answers: 1},
		{Role: "assistant", Content: "fixed", Answers: 2},
		{Role: "user", Content: "an old message"},
		{Role: "assistant", Content: "an old reply"},
	}
	got := sessionTurns(history)
	if len(got) != 3 {
		t.Fatalf("turns = %+v", got)
	}
	if got[0].assistant != "Background reviewer finished" || got[1].assistant != "fixed" || got[1].number != 1 {
		t.Fatalf("turns = %+v", got)
	}
	if got[2].assistant != "" {
		t.Fatalf("an unattributed message was given the answer %q", got[2].assistant)
	}
}
