package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// verdictExecutor is a worker whose turns end as scripted: each dispatch gets
// the next return, and the task text it was sent is recorded.
type verdictExecutor struct {
	processControlTaskExecutor
	vmu     sync.Mutex
	returns []observation.Return
	sent    []string
}

var _ session.ObservedTaskExecutor = (*verdictExecutor)(nil)

func (e *verdictExecutor) ExecuteObserved(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
	return e.ExecuteObservedWithContext(ctx, req, nil, types.PriorityNormal)
}

func (e *verdictExecutor) ExecuteObservedWithContext(_ context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (observation.Return, error) {
	e.vmu.Lock()
	defer e.vmu.Unlock()
	i := len(e.sent)
	e.sent = append(e.sent, req.TaskText())
	if i >= len(e.returns) {
		return observation.Return{}, errors.New("verdictExecutor: out of returns")
	}
	return e.returns[i], nil
}

func researchJob(t *testing.T, returns ...observation.Return) (Model, *verdictExecutor, backgroundDoneMsg) {
	t.Helper()
	research := perception.Intent{
		Category: "/query", Verb: "/research", Target: "retry policy", Confidence: 0.9,
		Response: "I'll have the researcher look.",
	}
	m, _ := journeyModel(t, config.WorkModeExplicit, research)
	exec := &verdictExecutor{returns: returns}
	m.taskExecutor = exec
	m, msg, cmd := say(t, m, "where do we build the retry policy?")
	if _, ok := msg.(backgroundStartMsg); !ok {
		t.Fatalf("read-only work answered %T, want a background start", msg)
	}
	return m, exec, runBatchFor[backgroundDoneMsg](t, cmd)
}

// A background job whose turn did not end /done is sent back with what it left
// missing, and reported finished only when the turn is verified. It used to be
// reported "finished" whatever the verdict said.
func TestBackground_AnUnverifiedResultIsSentBackWithTheGap(t *testing.T) {
	m, exec, done := researchJob(t,
		observation.Return{Output: "I read half of it.", Outcome: "/unverified", Missing: []string{"/self_reported_incomplete"}},
		observation.Return{Output: "Found the retry policy in pool.go.", Outcome: "/done"},
	)
	if done.Err != nil {
		t.Fatalf("the job failed: %v", done.Err)
	}
	if len(exec.sent) != 2 {
		t.Fatalf("the worker was sent the task %d time(s), want 2", len(exec.sent))
	}
	for _, want := range []string{"NOT VERIFIED", "the turn's own report says part of the requested work was not done", "I read half of it."} {
		if !strings.Contains(exec.sent[1], want) {
			t.Errorf("the second dispatch lacks %q:\n%s", want, exec.sent[1])
		}
	}
	next, _ := m.Update(done)
	m = next.(Model)
	if surface := lastAssistant(m); !strings.Contains(surface, "finished") || !strings.Contains(surface, "Found the retry policy in pool.go.") {
		t.Fatalf("the verified result is not reported:\n%s", surface)
	}
}

// A job that stays unverified and makes no progress is a failure naming the gap
// that shows what the worker last said; it is not "finished".
func TestBackground_AStalledUnverifiedResultFailsNamingTheGap(t *testing.T) {
	stuck := observation.Return{Output: "I could not find it.", Outcome: "/unverified", Missing: []string{"/self_reported_incomplete"}}
	m, exec, done := researchJob(t, stuck, stuck, stuck)
	if !errors.Is(done.Err, session.ErrNotVerified) {
		t.Fatalf("the job's error = %v, want ErrNotVerified", done.Err)
	}
	if len(exec.sent) != 2 {
		t.Fatalf("the worker was sent the task %d time(s), want the stall to end it at 2", len(exec.sent))
	}
	next, _ := m.Update(done)
	m = next.(Model)
	surface := lastAssistant(m)
	if strings.Contains(surface, "finished") {
		t.Fatalf("an unverified job is reported finished:\n%s", surface)
	}
	for _, want := range []string{"did not finish verified", "the turn's own report says part of the requested work was not done", "I could not find it."} {
		if !strings.Contains(surface, want) {
			t.Errorf("the report lacks %q:\n%s", want, surface)
		}
	}
}
