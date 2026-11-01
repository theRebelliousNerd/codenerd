package prompt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Pause timeout construction after the selection has observed its memo miss.
// The first Deadline query is the vector-search timeout, the second relevance.
// This schedules the lookup/publication race without sleeps or production hooks.
type relevanceDeadlineBarrier struct {
	context.Context
	queries    atomic.Int32
	at         int32
	reached    chan struct{}
	resume     chan struct{}
	resumeOnce sync.Once
}

func (c *relevanceDeadlineBarrier) Deadline() (time.Time, bool) {
	if c.queries.Add(1) == c.at {
		close(c.reached)
		select {
		case <-c.resume:
		case <-c.Context.Done():
		}
	}
	return c.Context.Deadline()
}

func (c *relevanceDeadlineBarrier) release() { c.resumeOnce.Do(func() { close(c.resume) }) }

func relevanceAwait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("relevance fixture did not reach its barrier")
	}
}

type relevanceFlightJudge struct {
	mu       sync.Mutex
	requests []string
	work     [][]string
	entered  chan struct{}
	release  chan struct{}
	stop     func()
}

func (j *relevanceFlightJudge) JudgeAtomRelevance(ctx context.Context, req AtomRelevanceRequest, atoms []*PromptAtom) (map[string]float64, error) {
	j.mu.Lock()
	first := len(j.work) == 0
	var keys []string
	for _, a := range atoms {
		keys = append(keys, a.ID+"/"+a.ContentHash)
	}
	j.requests = append(j.requests, req.Request)
	j.work = append(j.work, keys)
	j.mu.Unlock()
	if first {
		close(j.entered)
		select {
		case <-j.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(atoms))
	for _, a := range atoms {
		out[a.ID] = .8
	}
	return out, nil
}

func newRelevanceFlightJudge(t *testing.T) *relevanceFlightJudge {
	t.Helper()
	j := &relevanceFlightJudge{entered: make(chan struct{}), release: make(chan struct{})}
	var once sync.Once
	j.stop = func() { once.Do(func() { close(j.release) }) }
	t.Cleanup(j.stop)
	return j
}

func relevanceSelectionAsync(t *testing.T, s *AtomSelector, ctx context.Context, atoms []*PromptAtom, cc *CompilationContext) (*recordingKernel, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	k, done := &recordingKernel{}, make(chan struct{})
	go func() {
		defer close(done)
		if _, err := s.runSelectionFor(ctx, atoms, cc, k); err != nil {
			t.Errorf("selection failed: %v", err)
		}
	}()
	// Caller releases its barrier before test cleanup joins admitted selections.
	t.Cleanup(func() { cancel(); relevanceAwait(t, done) })
	return k, done
}

func TestAtomRelevance_InFlightMemoMiss(t *testing.T) {
	for _, variant := range []string{"identical", "overlapping", "changed-content"} {
		t.Run(variant, func(t *testing.T) {
			atoms, _ := relevanceCorpus()
			j := newRelevanceFlightJudge(t)
			s := relevanceSelector(j, 6, time.Hour)
			first := relevanceContext()
			second := first.Clone()
			second.TokenBudget++ // Distinct compile identity, same relevance request.
			if first.Hash() == second.Hash() || relevanceRequestFor(first).key() != relevanceRequestFor(second).key() {
				t.Fatal("fixture does not distinguish compile and relevance identity")
			}
			k1, done1 := relevanceSelectionAsync(t, s, context.Background(), atoms, first)
			relevanceAwait(t, j.entered)
			secondAtoms := atoms
			wantCalls, wantAtoms, wantRows := 1, 6, 6
			if variant == "overlapping" {
				s.SetAtomRelevance(AtomRelevance{Judge: j, Pool: 8, Timeout: time.Hour, YesPercent: 70, NoPercent: 30})
				wantCalls, wantAtoms, wantRows = 2, 8, 8
			}
			if variant == "changed-content" {
				secondAtoms = append([]*PromptAtom(nil), atoms...)
				changed := *atoms[1]
				changed.Content += " changed guidance"
				changed.ContentHash = HashContent(changed.Content)
				secondAtoms[1] = &changed
				wantCalls, wantAtoms = 2, 7
			}
			barrier := &relevanceDeadlineBarrier{Context: context.Background(), at: 2, reached: make(chan struct{}), resume: make(chan struct{})}
			k2, done2 := relevanceSelectionAsync(t, s, barrier, secondAtoms, second)
			t.Cleanup(barrier.release)
			relevanceAwait(t, barrier.reached)
			j.stop()
			relevanceAwait(t, done1)
			barrier.release()
			relevanceAwait(t, done2)
			if len(k1.with("atom_relevance(")) != 6 || len(k2.with("atom_relevance(")) != wantRows {
				t.Fatal("a complete admitted judgment did not reach both selection fact sets")
			}
			j.mu.Lock()
			defer j.mu.Unlock()
			seen, n := map[string]bool{}, 0
			for _, work := range j.work {
				for _, key := range work {
					if seen[key] {
						t.Errorf("duplicate relevance work: %s", key)
					}
					seen[key] = true
					n++
				}
			}
			if len(j.work) != wantCalls || n != wantAtoms {
				t.Errorf("dispatched %d judgments / %d atoms; want %d / %d", len(j.work), n, wantCalls, wantAtoms)
			}
		})
	}
}

func TestAtomRelevance_InFlightCancellationOwnership(t *testing.T) {
	for _, cancelOwner := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel_owner_%v", cancelOwner), func(t *testing.T) {
			atoms, _ := relevanceCorpus()
			j := newRelevanceFlightJudge(t)
			s := relevanceSelector(j, 6, time.Hour)
			owner, stopOwner := context.WithCancel(context.Background())
			defer stopOwner()
			k1, done1 := relevanceSelectionAsync(t, s, owner, atoms, relevanceContext())
			relevanceAwait(t, j.entered)
			waiter, stopWaiter := context.WithCancel(context.Background())
			defer stopWaiter()
			barrier := &relevanceDeadlineBarrier{Context: waiter, at: 2, reached: make(chan struct{}), resume: make(chan struct{})}
			k2, done2 := relevanceSelectionAsync(t, s, barrier, atoms, relevanceContext())
			t.Cleanup(barrier.release)
			relevanceAwait(t, barrier.reached)
			if cancelOwner {
				stopOwner()
				relevanceAwait(t, done1)
				barrier.release()
				relevanceAwait(t, done2)
			} else {
				stopWaiter()
				barrier.release()
				relevanceAwait(t, done2)
				j.stop()
				relevanceAwait(t, done1)
			}
			j.mu.Lock()
			calls := len(j.work)
			j.mu.Unlock()
			wantCalls, firstRows, secondRows := 1, 6, 0
			if cancelOwner {
				wantCalls, firstRows, secondRows = 2, 0, 6
			}
			if calls != wantCalls || len(k1.with("atom_relevance(")) != firstRows || len(k2.with("atom_relevance(")) != secondRows {
				t.Errorf("cancellation ownership: calls=%d, rows=%d/%d; want %d, %d/%d", calls, len(k1.with("atom_relevance(")), len(k2.with("atom_relevance(")), wantCalls, firstRows, secondRows)
			}
		})
	}
}

func TestAtomRelevance_InFlightUnrelatedRequestProgresses(t *testing.T) {
	atoms, _ := relevanceCorpus()
	j := newRelevanceFlightJudge(t)
	s := relevanceSelector(j, 6, time.Hour)
	_, done1 := relevanceSelectionAsync(t, s, context.Background(), atoms, relevanceContext())
	relevanceAwait(t, j.entered)
	other := relevanceContext()
	other.SemanticQuery = "independent request"
	k2, done2 := relevanceSelectionAsync(t, s, context.Background(), atoms, other)
	relevanceAwait(t, done2) // Must finish while the first request is blocked.
	if len(k2.with("atom_relevance(")) != 6 {
		t.Fatal("unrelated request did not publish its independent judgment")
	}
	j.mu.Lock()
	if len(j.requests) != 2 || j.requests[0] == j.requests[1] {
		t.Error("unrelated requests were not independently dispatched")
	}
	j.mu.Unlock()
	j.stop()
	relevanceAwait(t, done1)
}

func TestAtomRelevance_InFlightWaitFitsJudgmentTimeout(t *testing.T) {
	atoms, _ := relevanceCorpus()
	j := newRelevanceFlightJudge(t)
	s := relevanceSelector(j, 6, time.Hour)
	k1, done1 := relevanceSelectionAsync(t, s, context.Background(), atoms, relevanceContext())
	relevanceAwait(t, j.entered)
	s.SetAtomRelevance(AtomRelevance{Judge: j, Pool: 6, Timeout: 30 * time.Millisecond, YesPercent: 70, NoPercent: 30})
	k2, done2 := relevanceSelectionAsync(t, s, context.Background(), atoms, relevanceContext())
	relevanceAwait(t, done2) // The owner stays blocked throughout the waiter's timeout.
	if len(k2.with("atom_relevance")) != 0 || len(k2.with("vector_hit(")) != len(k2.with("vector_slot(")) {
		t.Fatal("timed-out waiter admitted evidence or widened vector recall")
	}
	j.mu.Lock()
	calls := len(j.work)
	j.mu.Unlock()
	if calls != 1 {
		t.Errorf("timed-out waiter dispatched judgment work: %d calls", calls)
	}
	j.stop()
	relevanceAwait(t, done1)
	if len(k1.with("atom_relevance(")) != 6 {
		t.Fatal("waiter's timeout canceled the independent owner")
	}
}

type relevanceJudgeFunc func(context.Context, AtomRelevanceRequest, []*PromptAtom) (map[string]float64, error)

func (f relevanceJudgeFunc) JudgeAtomRelevance(ctx context.Context, req AtomRelevanceRequest, atoms []*PromptAtom) (map[string]float64, error) {
	return f(ctx, req, atoms)
}

func TestAtomRelevance_InFlightFailuresPublishNothingAndRelease(t *testing.T) {
	for _, failure := range []string{"error", "incomplete", "late", "panic"} {
		t.Run(failure, func(t *testing.T) {
			atoms, _ := relevanceCorpus()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			j := relevanceJudgeFunc(func(_ context.Context, _ AtomRelevanceRequest, ask []*PromptAtom) (map[string]float64, error) {
				out := make(map[string]float64, len(ask))
				for _, a := range ask {
					out[a.ID] = .1
				}
				switch failure {
				case "error":
					return out, errors.New("offline judgment failure")
				case "incomplete":
					delete(out, ask[len(ask)-1].ID)
				case "late":
					cancel() // Even a judge ignoring cancellation cannot publish late answers.
				case "panic":
					panic("offline judgment panic")
				}
				return out, nil
			})
			s := relevanceSelector(j, 6, time.Second)
			cc := relevanceContext()
			key := relevanceCacheKey(relevanceRequestFor(cc).key(), atoms[1])
			s.relevanceCache.putAll(map[string]float64{key: .8})
			k := &recordingKernel{}
			if _, err := s.runSelectionFor(ctx, atoms, cc, k); err != nil {
				t.Fatal(err)
			}
			if len(k.with("atom_relevance")) != 0 {
				t.Fatal("failed judgment retained partial or cached-prefix evidence")
			}
			if len(k.with("vector_hit(")) != len(k.with("vector_slot(")) {
				t.Fatal("failed judgment widened the vector-only fallback")
			}
			for _, atom := range atoms[2:7] {
				if _, ok := s.relevanceCache.get(relevanceCacheKey(relevanceRequestFor(cc).key(), atom)); ok {
					t.Errorf("%s cached a partial answer for %s", failure, atom.ID)
				}
			}
			healthy := &countingJudge{answers: map[string]float64{}}
			for _, a := range atoms {
				healthy.answers[a.ID] = .8
			}
			s.SetAtomRelevance(AtomRelevance{Judge: healthy, Pool: 6, Timeout: time.Second, YesPercent: 70, NoPercent: 30})
			k2, done := relevanceSelectionAsync(t, s, context.Background(), atoms, cc)
			relevanceAwait(t, done)
			if len(k2.with("atom_relevance(")) != 6 || healthy.calls != 1 || len(healthy.asked) != 5 {
				t.Fatalf("admission or memo poisoned after failure: calls=%d asked=%v rows=%v", healthy.calls, healthy.asked, k2.with("atom_relevance("))
			}
		})
	}
}
