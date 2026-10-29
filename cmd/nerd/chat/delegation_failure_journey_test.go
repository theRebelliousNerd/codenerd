package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// failingObservedExecutor is a worker that fails after doing work: it returns
// the error and the observation of what it had measured by then.
type failingObservedExecutor struct {
	processControlTaskExecutor
	err error
	ret observation.Return
}

var _ session.ObservedTaskExecutor = (*failingObservedExecutor)(nil)

func (e *failingObservedExecutor) ExecuteObserved(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
	return e.ExecuteObservedWithContext(ctx, req, nil, types.PriorityNormal)
}

func (e *failingObservedExecutor) ExecuteObservedWithContext(context.Context, session.TaskRequest, *types.SessionContext, types.SpawnPriority) (observation.Return, error) {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	return e.ret, e.err
}

// A background job whose worker fails after tool work tells the user what it
// had already done, not only that it stopped.
func TestJourney_AFailedBackgroundJobSaysWhatItHadDone(t *testing.T) {
	t.Parallel()
	research := perception.Intent{
		Category: "/query", Verb: "/research", Target: "retry policy", Confidence: 0.9,
		Response: "I'll have the researcher look.",
	}
	m, _ := journeyModel(t, config.WorkModeExplicit, research)
	cause := errors.New("provider transport: connection reset")
	m.taskExecutor = &failingObservedExecutor{
		err: cause,
		ret: observation.Return{
			Changed: []string{"notes/retry.md"},
			Outcome: "/failed",
			Notes:   []string{"tool call failed: read_file: no such file"},
		},
	}

	m, msg, cmd := say(t, m, "where do we build the retry policy?")
	if _, ok := msg.(backgroundStartMsg); !ok {
		t.Fatalf("read-only work answered %T, want a background start", msg)
	}
	done := runBatchFor[backgroundDoneMsg](t, cmd)
	if !errors.Is(done.Err, cause) {
		t.Fatalf("the job's error hides the cause: %v", done.Err)
	}
	next, _ := m.Update(done)
	m = next.(Model)

	surface := lastAssistant(m)
	for _, want := range []string{"failed", "connection reset", "notes/retry.md", "tool call failed: read_file", "nothing here is verified complete"} {
		if !strings.Contains(surface, want) {
			t.Errorf("the failure report lacks %q:\n%s", want, surface)
		}
	}
}
