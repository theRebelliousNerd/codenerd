package atomjudge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

func relevanceAtoms(n int) []*prompt.PromptAtom {
	atoms := make([]*prompt.PromptAtom, n)
	for i := range atoms {
		atoms[i] = atom(fmt.Sprintf("a%03d", i), "check errors")
	}
	return atoms
}

func yesAsker() *fakeAsker {
	return &fakeAsker{answer: func(string, typesafe.Question) string { return `{"type":"noul","noul":0.9}` }}
}

func TestRelevanceBudget_PreflightRejectsWholePoolWithoutCalls(t *testing.T) {
	atoms := relevanceAtoms(41) // Two requests: the state is charged twice.
	req := prompt.AtomRelevanceRequest{Request: "fix errors"}
	base := RelevanceJudge{Asker: yesAsker(), MaxAtoms: 100, MaxInputTokens: 100000}
	plan, err := base.planRelevance(context.Background(), req, atoms)
	if err != nil || len(plan.batches) != 2 {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	// A tiny body still carries full metadata and question criteria.
	if plan.inputTokens <= 41*prompt.EstimateTokens(atoms[0].Content) {
		t.Fatal("input estimate only charged atom bodies")
	}
	longReq := req
	longReq.Request += strings.Repeat("x", 4000)
	longPlan, err := base.planRelevance(context.Background(), longReq, atoms)
	if err != nil || longPlan.inputTokens-plan.inputTokens != 2000 {
		t.Fatalf("repeated state not charged per batch: %d -> %d, %v", plan.inputTokens, longPlan.inputTokens, err)
	}
	for _, tc := range []struct {
		name               string
		maxAtoms, maxInput int
		req                prompt.AtomRelevanceRequest
		resource           string
	}{
		{"atom count", 40, 100000, req, "atoms"},
		{"aggregate input", 100, plan.inputTokens - 1, req, "estimated input tokens"},
		{"request state", 100, plan.inputTokens, longReq, "estimated input tokens"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := yesAsker()
			j := RelevanceJudge{Asker: f, MaxAtoms: tc.maxAtoms, MaxInputTokens: tc.maxInput}
			got, err := j.JudgeAtomRelevance(context.Background(), tc.req, atoms)
			var budget *RelevanceBudgetError
			if got != nil || !errors.As(err, &budget) || budget.Resource != tc.resource || len(f.bodies) != 0 {
				t.Fatalf("got %v, error %v, calls %d", got, err, len(f.bodies))
			}
		})
	}
	f := yesAsker()
	got, err := (RelevanceJudge{Asker: f, MaxAtoms: len(atoms), MaxInputTokens: plan.inputTokens}).JudgeAtomRelevance(context.Background(), req, atoms)
	if err != nil || len(got) != len(atoms) || len(f.bodies) != 2 {
		t.Fatalf("exact budget boundary refused: %d answers, %d calls, %v", len(got), len(f.bodies), err)
	}
}

type gatedRelevanceAsker struct {
	started             chan struct{}
	release             chan struct{}
	active, peak, calls atomic.Int32
}

func (f *gatedRelevanceAsker) Ask(ctx context.Context, state any, qs map[string]typesafe.Question) (*typesafe.Response, error) {
	f.calls.Add(1)
	n := f.active.Add(1)
	defer f.active.Add(-1)
	for old := f.peak.Load(); n > old; old = f.peak.Load() {
		if f.peak.CompareAndSwap(old, n) {
			break
		}
	}
	f.started <- struct{}{}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.release:
		return yesAsker().Ask(ctx, state, qs)
	}
}

func awaitStart(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("batch did not start")
	}
}

func TestRelevanceBudget_BoundsWorkersAndCancelsQueuedBatches(t *testing.T) {
	for _, parallel := range []int{1, 2} {
		t.Run(fmt.Sprint(parallel), func(t *testing.T) {
			f := &gatedRelevanceAsker{started: make(chan struct{}, 3), release: make(chan struct{}, 3)}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				got, err := (RelevanceJudge{Asker: f, MaxAtoms: 100, MaxConcurrentBatches: parallel}).JudgeAtomRelevance(ctx, prompt.AtomRelevanceRequest{Request: "r"}, relevanceAtoms(81))
				if got != nil {
					done <- errors.New("cancellation returned partial evidence")
					return
				}
				done <- err
			}()
			for range parallel {
				awaitStart(t, f.started)
			}
			select {
			case <-f.started:
				t.Fatal("queued batch exceeded concurrency limit")
			case <-time.After(30 * time.Millisecond):
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("canceled workers did not finish")
			}
			if f.calls.Load() != int32(parallel) || f.peak.Load() != int32(parallel) {
				t.Fatalf("calls %d peak %d", f.calls.Load(), f.peak.Load())
			}
		})
	}
}

func TestRelevanceBudget_AllBatchesCompleteWithinWorkerLimit(t *testing.T) {
	f := &gatedRelevanceAsker{started: make(chan struct{}, 3), release: make(chan struct{}, 3)}
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		got, err := (RelevanceJudge{Asker: f, MaxAtoms: 100, MaxConcurrentBatches: 2}).JudgeAtomRelevance(ctx, prompt.AtomRelevanceRequest{Request: "r"}, relevanceAtoms(81))
		if err == nil && len(got) != 81 {
			err = fmt.Errorf("only %d answers", len(got))
		}
		done <- err
	}()
	awaitStart(t, f.started)
	awaitStart(t, f.started)
	f.release <- struct{}{}
	awaitStart(t, f.started)
	f.release <- struct{}{}
	f.release <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if f.calls.Load() != 3 || f.peak.Load() != 2 {
		t.Fatalf("calls %d peak %d", f.calls.Load(), f.peak.Load())
	}
}

func TestRelevanceBudget_FailureStopsQueuedBatches(t *testing.T) {
	f := &fakeAsker{err: errors.New("503")}
	got, err := (RelevanceJudge{Asker: f, MaxAtoms: 100, MaxConcurrentBatches: 1}).JudgeAtomRelevance(context.Background(), prompt.AtomRelevanceRequest{Request: "r"}, relevanceAtoms(81))
	if got != nil || err == nil || len(f.bodies) != 1 {
		t.Fatalf("got %v, error %v, calls %d", got, err, len(f.bodies))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = (RelevanceJudge{Asker: f}).JudgeAtomRelevance(ctx, prompt.AtomRelevanceRequest{}, relevanceAtoms(1))
	if !errors.Is(err, context.Canceled) || len(f.bodies) != 1 {
		t.Fatalf("canceled preflight: %v, calls %d", err, len(f.bodies))
	}
}

type secondBatchBadAsker struct{ calls int }

func (f *secondBatchBadAsker) Ask(ctx context.Context, state any, qs map[string]typesafe.Question) (*typesafe.Response, error) {
	f.calls++ // This fixture uses one worker.
	if f.calls == 1 {
		return yesAsker().Ask(ctx, state, qs)
	}
	return (&fakeAsker{answer: func(string, typesafe.Question) string {
		return `{"type":"score","score":1}`
	}}).Ask(ctx, state, qs)
}

func TestRelevanceBudget_LaterBadBatchDiscardsEarlierAnswers(t *testing.T) {
	f := &secondBatchBadAsker{}
	got, err := (RelevanceJudge{Asker: f, MaxAtoms: 100, MaxConcurrentBatches: 1}).JudgeAtomRelevance(context.Background(), prompt.AtomRelevanceRequest{Request: "r"}, relevanceAtoms(81))
	if got != nil || err == nil || f.calls != 2 {
		t.Fatalf("partial evidence escaped or queued batch started: %v, %v, calls %d", got, err, f.calls)
	}
}
