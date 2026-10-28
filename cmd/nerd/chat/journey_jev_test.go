package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// journey_jev_test.go: Jev's turn judgments enhance routing without replacing
// perception (jev_turn.go, policy/jev_evidence.mg). A fake Jev answers each
// question by name; perception is scripted; the kernel is real.

// fakeJevServer answers each noul from probs (0.5 when unlisted). Setting
// fail makes every call a 500.
type fakeJevServer struct {
	mu    sync.Mutex
	probs map[string]float64
	fail  bool
	calls int
}

func (f *fakeJevServer) set(probs map[string]float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probs = probs
}

func (f *fakeJevServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls++
		if f.fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		answers := map[string]any{}
		for name := range req.Questions {
			p, ok := f.probs[name]
			if !ok {
				p = 0.5
			}
			answers[name] = map[string]any{"type": "noul", "noul": p}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-fake", "answers": answers, "usage": map[string]any{"input_tokens": 100, "output_tokens": 1}})
	}
}

// withJev attaches a fake Jev to the model.
func withJev(t *testing.T, m Model, f *fakeJevServer) Model {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "jev-fake", BaseURL: srv.URL, APIKey: "k", Retry: &typesafe.RetryPolicy{}})
	if err != nil {
		t.Fatal(err)
	}
	m.jev = c
	m.Config.TypeSafe = &config.TypeSafeConfig{Model: "jev-fake", BaseURL: srv.URL, APIKey: "k"}
	return m
}

func fixReadmeTurn(response string, brief *perception.Brief) perception.Intent {
	return perception.Intent{Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.95, Response: response, Brief: brief}
}

// "ship it" that perception reads as another fix still approves the held
// work, because Jev says it approves it and adds nothing.
func TestJevJourney_MisreadGoAheadStillApprovesTheHeldWork(t *testing.T) {
	ready := &perception.Brief{Goal: "fix the README typo", Ready: true}
	m, _ := journeyModel(t, config.WorkModePropose, fixReadmeTurn("I can fix it.", ready), fixReadmeTurn("Fixing it.", ready))
	jev := &fakeJevServer{probs: map[string]float64{"changes_files": 0.9, "names_target": 0.9}}
	m = withJev(t, m, jev)

	m, msg, _ := say(t, m, "fix the typo in README.md")
	if _, ok := msg.(holdMsg); !ok {
		t.Fatalf("propose answered %T, want a hold", msg)
	}

	jev.set(map[string]float64{"changes_files": 0.9, "names_target": 0.9, "approves_held_work": 0.93, "adds_to_held_work": 0.04})
	_, msg, _ = say(t, m, "looks good, ship it")
	if _, ok := msg.(approveWorkMsg); !ok {
		t.Fatalf("Jev's go-ahead did not take the approve lane: %T", msg)
	}
}

// A confident mutation that Jev reads as not asking for any change asks
// first instead of running, even in yolo.
func TestJevJourney_DisputedMutationAsksFirst(t *testing.T) {
	m, exec := journeyModel(t, config.WorkModeYolo, fixReadmeTurn("Fixing it.", nil))
	m = withJev(t, m, &fakeJevServer{probs: map[string]float64{"changes_files": 0.08, "names_target": 0.9, "hypothetical": 0.1}})
	intent := fixReadmeTurn("", nil)
	assertRouteIntent(t, m, intent)
	m.recordTurnJudgments(m.shutdownCtx, m.startTurnJudgments(m.shutdownCtx, "is the README typo still there?", nil))
	if route := m.decideRoute("is the README typo still there?", intent, "coder"); route.Kind != RouteClarify {
		t.Fatalf("route = %s, want clarify when Jev disputes the mutation", route.Kind)
	}
	if exec.callCount() != 0 {
		t.Fatal("work ran on a disputed mutation")
	}
}

// Perception kept an open question that the user's message answers: Jev's
// judgment closes it, and yolo starts the work instead of holding.
func TestJevJourney_AnsweredOpenQuestionUnblocksYolo(t *testing.T) {
	open := &perception.Brief{Goal: "fix a README typo", OpenQuestions: []string{"which typo?"}, Ready: true}
	m, _ := journeyModel(t, config.WorkModeYolo, fixReadmeTurn("Which typo?", open), fixReadmeTurn("On it.", open))
	jev := &fakeJevServer{probs: map[string]float64{"changes_files": 0.9, "names_target": 0.9}}
	m = withJev(t, m, jev)

	m, msg, _ := say(t, m, "fix the readme typo")
	if _, ok := msg.(holdMsg); !ok {
		t.Fatalf("an open question did not hold: %T", msg)
	}
	jev.set(map[string]float64{"changes_files": 0.9, "names_target": 0.9, "answered_0": 0.94})
	_, msg, _ = say(t, m, "the 'teh' on line 2")
	am, ok := msg.(assistantMsg)
	if !ok || !strings.Contains(am.Surface, "no task verifier is configured") {
		t.Fatalf("Jev's answered question did not release the work: %T %+v", msg, msg)
	}
}

// "Stop" while a background job runs cancels it, and the job reports back as
// stopped.
func TestJevJourney_StopCancelsBackgroundWork(t *testing.T) {
	chat := perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, Response: "Stopping it."}
	m, _ := journeyModel(t, config.WorkModePropose, chat)
	m = withJev(t, m, &fakeJevServer{probs: map[string]float64{"wants_to_stop": 0.96, "changes_files": 0.05}})

	block := make(chan struct{})
	m.taskExecutor = &blockingExecutor{release: block}
	job := backgroundJob{ID: "bg-stop", Shard: "researcher", Task: "survey the retry code", Started: time.Now()}
	run := m.startBackground(job, job.Task)
	doneCh := make(chan backgroundDoneMsg, 1)
	go func() { doneCh <- run().(backgroundDoneMsg) }()

	m, _, _ = say(t, m, "stop, never mind that")
	select {
	case done := <-doneCh:
		next, _ := m.Update(done)
		m = next.(Model)
		if !strings.Contains(lastAssistant(m), "Stopped background researcher") {
			t.Fatalf("the stopped job did not report as stopped:\n%s", lastAssistant(m))
		}
	case <-time.After(10 * time.Second):
		close(block)
		t.Fatal("stop did not cancel the running job")
	}
}

// A failed Jev call leaves no evidence: the turn routes exactly as it would
// have without Jev.
func TestJevJourney_FailedCallChangesNothing(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo, fixReadmeTurn("Fixing it.", &perception.Brief{Goal: "fix the typo", Ready: true}))
	jev := &fakeJevServer{fail: true}
	m = withJev(t, m, jev)
	_, msg, _ := say(t, m, "fix the typo in README.md")
	am, ok := msg.(assistantMsg)
	if !ok || !strings.Contains(am.Surface, "no task verifier is configured") {
		t.Fatalf("a failed Jev call changed the route: %T %+v", msg, msg)
	}
	if jev.calls == 0 {
		t.Fatal("Jev was never asked")
	}
	if rows, _ := m.kernel.Query("jev_judgment"); len(rows) != 0 {
		t.Fatalf("a failed call left evidence: %v", rows)
	}
}

// blockingExecutor holds a task until its context ends or release closes.
type blockingExecutor struct{ release chan struct{} }

func (e *blockingExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	return e.ExecuteWithContext(ctx, req, nil, types.PriorityNormal)
}

func (e *blockingExecutor) ExecuteWithContext(ctx context.Context, _ session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-e.release:
		return "released", nil
	}
}

func (e *blockingExecutor) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", fmt.Errorf("unexpected async execution")
}

func (e *blockingExecutor) GetResult(string) (string, bool, error) {
	return "", false, fmt.Errorf("unexpected result lookup")
}

func (e *blockingExecutor) WaitForResult(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected wait")
}
