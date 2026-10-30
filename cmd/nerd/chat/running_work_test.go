package chat

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// progressModel is a chat whose coder turn has been running: an agent under
// the turn's task id has read a file and is waiting on a model, and README.md
// was written after the turn began.
func progressModel(t *testing.T, session string, intents ...perception.Intent) (Model, context.Context, func()) {
	t.Helper()
	sessionlog.Install(sessionlog.NewStore(sessionlog.Options{Workspace: t.TempDir(), Session: session, Enabled: func() bool { return false }}))
	t.Cleanup(func() { sessionlog.Install(nil) })
	m, ctx := busyModel(t, intents...)
	m.sessionID = session
	owner := m.turn.current()

	reached, release := make(chan struct{}), make(chan struct{})
	go func() {
		actx := sessionlog.WithTask(usage.WithSessionID(context.Background(), session), foregroundTask(owner))
		_, _ = sessionlog.RunAgent(actx, "coder", "7", "fix README.md", func(ctx context.Context) (string, error) {
			tr := sessionlog.TranscriptFrom(ctx)
			tr.ToolResult(ctx, "read_file", "c1", "# README", false)
			tr.CallStarted(ctx, "model-under-test")
			close(reached)
			<-release
			return "", nil
		})
	}()
	<-reached
	if err := m.kernel.Assert(core.Fact{Predicate: "file_written", Args: []any{"README.md", "h1", session, time.Now().Unix()}}); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	return m, ctx, func() { once.Do(func() { close(release) }) }
}

// While a coder turn ran, "how is progress?" got "I'll check current progress
// on ..." and then "The brief changed while I read this, so this message's
// change to it was not applied": nothing was checked, and a status question
// was read as an edit to the brief. A question about how the running work is
// going is now answered from what the work has done, and changes nothing.
func TestSideTurn_ProgressQuestionIsAnsweredFromTheWork(t *testing.T) {
	promise := perception.Intent{
		Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, IsQuestion: true,
		Response: "I'll check current progress on attributing model usage to active shards.",
		Brief:    &perception.Brief{Goal: "attribute model usage to active shards"},
	}
	m, ctx, release := progressModel(t, "session-progress", promise)
	defer release()
	m = withJev(t, m, &fakeJevServer{probs: map[string]float64{"asks_about_running_work": 0.96, "wants_to_stop": 0.02, "changes_files": 0.02}})
	before := m.currentBrief().Render()

	m, cmd := typeAndEnter(t, m, "how is progress?")
	m = runSideTurn(t, m, cmd)

	reply := lastAssistant(m)
	for _, want := range []string{"Running: coder: fix README.md", "coder-7", "1 tool result(s)", "last tool read_file", "waiting on model-under-test", "Written since it began: README.md"} {
		if !strings.Contains(reply, want) {
			t.Errorf("the progress answer does not say %q:\n%s", want, reply)
		}
	}
	if strings.Contains(reply, "I'll check") || strings.Contains(reply, "brief") {
		t.Errorf("the progress answer promised a check or talked about the brief:\n%s", reply)
	}
	if got := m.currentBrief().Render(); got != before {
		t.Errorf("a status question changed the brief:\n%s\nwant\n%s", got, before)
	}
	if ctx.Err() != nil || m.turn.queuedCount() != 0 {
		t.Error("a status question stopped or queued the work")
	}
}

// briefMovingTransducer is perception that, while it reads, sees the brief
// change under it (the running work updating it), and records the context
// it was given.
type briefMovingTransducer struct {
	scriptedTransducer
	m       *Model
	moved   *perception.Brief
	running string
}

func (b *briefMovingTransducer) ParseIntentWithContext(ctx context.Context, input string, h []perception.ConversationTurn) (perception.Intent, error) {
	if sc := types.GetSessionContext(ctx); sc != nil {
		b.running = sc.RunningWork
	}
	b.m.writeBrief(b.moved)
	return b.scriptedTransducer.ParseIntentWithContext(ctx, input, h)
}

// Without Jev, perception reads the message with the running work's progress,
// and a question never produces a brief change or a note about one, even when
// perception returned a brief and the brief moved meanwhile.
func TestSideTurn_AQuestionNeverTouchesTheBrief(t *testing.T) {
	question := perception.Intent{
		Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, IsQuestion: true,
		Response: "It has read the README and is waiting on the model.",
		Brief:    &perception.Brief{Goal: "something perception made up"},
	}
	m, _, release := progressModel(t, "session-question")
	defer release()
	moved := &perception.Brief{Goal: "fix the README typo", Constraints: []string{"keep the badge"}}
	tr := &briefMovingTransducer{scriptedTransducer: scriptedTransducer{intents: []perception.Intent{question}}, moved: moved}
	m.transducer = tr
	tr.m = &m

	m, cmd := typeAndEnter(t, m, "how far along is it?")
	m = runSideTurn(t, m, cmd)

	if strings.Contains(lastAssistant(m), "brief") {
		t.Errorf("a question got a note about the brief:\n%s", lastAssistant(m))
	}
	if got := m.currentBrief().Render(); got != moved.Render() {
		t.Errorf("a question replaced the brief:\n%s", got)
	}
	if !strings.Contains(tr.running, "Running: coder: fix README.md") || !strings.Contains(tr.running, "last tool read_file") {
		t.Errorf("perception did not read the running work's progress: %q", tr.running)
	}
}

// A real mid-run change still updates the brief; one made while the brief
// moved is refused with the note.
func TestSideTurn_ARealChangeStillUpdatesTheBrief(t *testing.T) {
	t.Parallel()
	change := perception.Intent{
		Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9,
		Response: "Noted: the badge line stays.",
		Brief:    &perception.Brief{Goal: "fix the README typo", Constraints: []string{"keep the badge line as is"}},
	}
	m, _ := busyModel(t, change)
	m, cmd := typeAndEnter(t, m, "keep the badge line as is")
	m = runSideTurn(t, m, cmd)
	if got := m.currentBrief(); got == nil || len(got.Constraints) != 1 || got.Constraints[0] != "keep the badge line as is" {
		t.Fatalf("the change did not reach the brief: %+v", got)
	}

	m2, _ := busyModel(t, change)
	tr := &briefMovingTransducer{scriptedTransducer: scriptedTransducer{intents: []perception.Intent{change}}, moved: &perception.Brief{Goal: "fix the README typo", Decisions: []string{"use the short form"}}}
	m2.transducer = tr
	tr.m = &m2
	m2, cmd = typeAndEnter(t, m2, "keep the badge line as is")
	m2 = runSideTurn(t, m2, cmd)
	if !strings.Contains(lastAssistant(m2), "The brief changed while I read this") {
		t.Fatalf("a change made while the brief moved was not refused with the note:\n%s", lastAssistant(m2))
	}
}
