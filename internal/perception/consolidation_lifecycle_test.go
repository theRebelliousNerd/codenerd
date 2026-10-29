package perception

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// gatedCritic is a critic whose calls the test controls: each call announces
// itself on entered, then returns when released or when its context ends.
type gatedCritic struct {
	baseMockLLMClient
	entered   chan struct{}
	release   chan struct{}
	finished  atomic.Int32
	cancelled atomic.Int32
}

func newGatedCritic() *gatedCritic {
	return &gatedCritic{entered: make(chan struct{}, 16), release: make(chan struct{})}
}

func (c *gatedCritic) CompleteWithSystem(ctx context.Context, _, _ string) (string, error) {
	c.entered <- struct{}{}
	select {
	case <-c.release:
		c.finished.Add(1)
		return "", nil
	case <-ctx.Done():
		c.cancelled.Add(1)
		return "", ctx.Err()
	}
}

func oneHistory(text string) []ReasoningTrace {
	return []ReasoningTrace{{UserPrompt: text, Response: "ok"}, {UserPrompt: "next " + text, Response: "ok"}}
}

// A history offered after Stop must be refused, not parked in a queue with no
// reader. The old Enqueue only looked at channel capacity.
func TestConsolidationWorker_RejectsAfterStop(t *testing.T) {
	cw := NewConsolidationWorker(nil)
	cw.Start()
	cw.Stop()
	if got := cw.Enqueue(oneHistory("late")); got != AdmissionRejectedClosed {
		t.Fatalf("Enqueue after Stop = %q, want %q", got, AdmissionRejectedClosed)
	}
	st := cw.Stats()
	if st.RejectedClosed != 1 || st.QueuedJobs != 0 || st.Accepted != 0 {
		t.Fatalf("stats after rejected enqueue = %+v, want one closed rejection and an empty queue", st)
	}
}

func TestConsolidationWorker_FullQueueIsRejectedNotSilent(t *testing.T) {
	cw := NewConsolidationWorker(nil) // not started: nothing drains
	for i := range consolidationQueueJobs {
		if got := cw.Enqueue(oneHistory("x")); got != AdmissionAccepted {
			t.Fatalf("enqueue %d = %q, want accepted", i, got)
		}
	}
	if got := cw.Enqueue(oneHistory("overflow")); got != AdmissionRejectedFull {
		t.Fatalf("overflow enqueue = %q, want %q", got, AdmissionRejectedFull)
	}
	if st := cw.Stats(); st.RejectedFull != 1 || st.Accepted != consolidationQueueJobs {
		t.Fatalf("stats = %+v, want 100 accepted and 1 rejected_full", st)
	}
}

// Job count is not a memory bound: a few huge histories must hit the byte
// budget, while an empty queue still admits one oversized history.
func TestConsolidationWorker_QueueByteBudget(t *testing.T) {
	cw := NewConsolidationWorker(nil)
	cw.byteBudget = 1000
	big := make([]byte, 5000)
	for i := range big {
		big[i] = 'a'
	}
	if got := cw.Enqueue(oneHistory(string(big))); got != AdmissionAccepted {
		t.Fatalf("first oversized history = %q, want accepted on an empty queue", got)
	}
	if got := cw.Enqueue(oneHistory("small")); got != AdmissionRejectedFull {
		t.Fatalf("history over the byte budget = %q, want %q", got, AdmissionRejectedFull)
	}
	if st := cw.Stats(); st.QueuedBytes < 5000 {
		t.Fatalf("QueuedBytes = %d, want the retained text counted", st.QueuedBytes)
	}
}

// Enqueue racing Stop: every submission gets exactly one answer and every
// accepted history reaches a terminal outcome. Run under -race.
func TestConsolidationWorker_EnqueueRacingStopAccountsEveryJob(t *testing.T) {
	cw := NewConsolidationWorker(nil)
	cw.Start()

	var accepted, rejected atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 8 {
		wg.Go(func() {
			<-start
			for range 40 {
				switch cw.Enqueue(oneHistory("race")) {
				case AdmissionAccepted:
					accepted.Add(1)
				case AdmissionRejectedClosed, AdmissionRejectedFull:
					rejected.Add(1)
				default:
					t.Error("Enqueue returned an unknown admission")
				}
			}
		})
	}
	close(start)
	cw.Stop()
	wg.Wait()

	st := cw.Stats()
	if got := accepted.Load() + rejected.Load(); got != 8*40 {
		t.Fatalf("answers = %d, want one per submission (320)", got)
	}
	if int64(st.Accepted) != accepted.Load() {
		t.Fatalf("stats accepted %d != caller-observed %d", st.Accepted, accepted.Load())
	}
	if st.Processed+st.Abandoned != st.Accepted || st.QueuedJobs != 0 {
		t.Fatalf("accepted jobs not all terminal: %+v", st)
	}
}

// Stop must not return while an accepted job is still using the critic and
// stores: it is the join the shutdown ordering depends on.
func TestConsolidationWorker_StopJoinsInFlightJob(t *testing.T) {
	eng := testTaxonomyEngine(t)
	critic := newGatedCritic()
	eng.SetClient(critic)
	cw := NewConsolidationWorker(eng)
	cw.Start()
	if got := cw.Enqueue(oneHistory("a")); got != AdmissionAccepted {
		t.Fatalf("enqueue = %q", got)
	}
	<-critic.entered

	stopped := make(chan struct{})
	go func() { cw.Stop(); close(stopped) }()
	// Admission closing is Stop's first step, so once it is visible Stop is
	// waiting on the in-flight job.
	for {
		cw.mu.Lock()
		closed := cw.closed
		cw.mu.Unlock()
		if closed {
			break
		}
		runtime.Gosched()
	}
	select {
	case <-stopped:
		t.Fatal("Stop returned while the critic call was still in flight")
	default:
	}
	close(critic.release)
	<-stopped
	if critic.finished.Load() != 1 {
		t.Fatalf("critic finished %d calls before Stop returned, want 1", critic.finished.Load())
	}
	if st := cw.Stats(); st.Processed != 1 || st.Abandoned != 0 {
		t.Fatalf("stats = %+v, want the job processed", st)
	}
}

// A bounded Shutdown that runs out of budget cancels the in-flight critic,
// abandons the queued rest, counts them, and has joined before it returns.
func TestConsolidationWorker_BoundedShutdownCancelsAbandonsAndJoins(t *testing.T) {
	eng := testTaxonomyEngine(t)
	critic := newGatedCritic()
	eng.SetClient(critic)
	cw := NewConsolidationWorker(eng)
	cw.Start()
	for _, name := range []string{"a", "b", "c"} {
		if got := cw.Enqueue(oneHistory(name)); got != AdmissionAccepted {
			t.Fatalf("enqueue %s = %q", name, got)
		}
	}
	<-critic.entered // job a is in flight, b and c are queued

	budget, expire := context.WithCancel(context.Background())
	expire()
	err := cw.Shutdown(budget)

	var incomplete *ShutdownIncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("Shutdown err = %v, want *ShutdownIncompleteError", err)
	}
	if incomplete.Abandoned != 3 || !errors.Is(err, context.Canceled) {
		t.Fatalf("incomplete = %+v, want 3 abandoned with the budget as cause", incomplete)
	}
	if critic.cancelled.Load() != 1 || critic.finished.Load() != 0 {
		t.Fatalf("in-flight critic cancelled=%d finished=%d, want it cancelled", critic.cancelled.Load(), critic.finished.Load())
	}
	// Joined: the worker goroutine is gone and admission stays closed.
	if got := cw.Enqueue(oneHistory("after")); got != AdmissionRejectedClosed {
		t.Fatalf("enqueue after bounded Shutdown = %q, want rejected_closed", got)
	}
	if st := cw.Stats(); st.Abandoned != 3 || st.Processed != 0 {
		t.Fatalf("stats = %+v, want three abandoned", st)
	}
}

func TestConsolidationWorker_ShutdownWithinBudgetDrainsEverything(t *testing.T) {
	eng := testTaxonomyEngine(t)
	critic := newGatedCritic()
	close(critic.release)
	eng.SetClient(critic)
	cw := NewConsolidationWorker(eng)
	for _, name := range []string{"a", "b", "c"} {
		cw.Enqueue(oneHistory(name))
	}
	cw.Start()
	if err := cw.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if st := cw.Stats(); st.Processed != 3 || st.Abandoned != 0 {
		t.Fatalf("stats = %+v, want all three processed", st)
	}
}

// The package singleton is stopped by the shutdown path and a late learning
// submission is a visible rejection, not a silent drop into a dead queue.
func TestTaxonomyEngine_QueueForLearningAfterShutdownIsRejected(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatal(err)
	}
	if got := eng.QueueForLearning(oneHistory("live")); got != AdmissionAccepted {
		t.Fatalf("QueueForLearning on a running engine = %q", got)
	}
	if err := eng.ShutdownWorker(context.Background()); err != nil {
		t.Fatalf("ShutdownWorker: %v", err)
	}
	if got := eng.QueueForLearning(oneHistory("late")); got != AdmissionRejectedClosed {
		t.Fatalf("QueueForLearning after shutdown = %q, want rejected_closed", got)
	}
}
