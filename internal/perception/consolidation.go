package perception

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"codenerd/internal/logging"
)

// Admission is the answer Enqueue gives for one submitted history. Every
// submission gets exactly one; a dropped history is visible to the caller
// instead of only a log line.
type Admission string

const (
	// AdmissionAccepted: the history is queued and will reach a terminal
	// outcome (processed, or accounted as abandoned by a bounded Shutdown).
	AdmissionAccepted Admission = "accepted"
	// AdmissionRejectedFull: the queue is at its job-count or byte budget.
	AdmissionRejectedFull Admission = "rejected_full"
	// AdmissionRejectedClosed: Shutdown has closed admission. A stopped worker
	// is never reopened; a later boot constructs a new one.
	AdmissionRejectedClosed Admission = "rejected_closed"
)

const (
	// consolidationQueueJobs bounds the queued histories by count.
	consolidationQueueJobs = 100
	// consolidationQueueBytes bounds them by retained text. Count alone is not
	// a memory bound: one history carries whole prompts and replies. An empty
	// queue always admits one history, so a single large exchange is learnable.
	consolidationQueueBytes = 8 << 20
)

// ConsolidationStats is the worker's loss and completion accounting.
type ConsolidationStats struct {
	Accepted       uint64
	RejectedFull   uint64
	RejectedClosed uint64
	Processed      uint64
	// Abandoned counts accepted histories a bounded Shutdown discarded
	// instead of processing.
	Abandoned   uint64
	QueuedJobs  int
	QueuedBytes int
}

// ShutdownIncompleteError reports a bounded Shutdown that ran out of budget
// before the accepted queue drained. Queued histories were abandoned, the
// in-flight one was cancelled, and the worker has joined: nothing still uses
// the dependencies when it is returned.
type ShutdownIncompleteError struct {
	Abandoned uint64
	Cause     error
}

func (e *ShutdownIncompleteError) Error() string {
	return fmt.Sprintf("consolidation shutdown incomplete: %d accepted histories abandoned: %v", e.Abandoned, e.Cause)
}

func (e *ShutdownIncompleteError) Unwrap() error { return e.Cause }

type consolidationJob struct {
	traces []ReasoningTrace
	bytes  int
}

// ConsolidationWorker manages an asynchronous "sleep cycle" for learning taxonomy patterns
// from recent interactions without blocking the main execution loop.
//
// Lifecycle: running -> admission closed -> draining -> joined. Admission and
// closure linearize on mu, so after Shutdown closes admission no history can
// enter a queue nobody reads, and the set to drain is fixed.
type ConsolidationWorker struct {
	engine *TaxonomyEngine
	queue  chan consolidationJob
	quit   chan struct{}
	wg     sync.WaitGroup

	mu          sync.Mutex
	closed      bool
	queuedBytes int
	byteBudget  int
	stats       ConsolidationStats

	// workCtx parents every job. A bounded Shutdown cancels it so an in-flight
	// critic call or embedding stops instead of outliving its dependencies.
	workCtx    context.Context
	cancelWork context.CancelFunc
	abandon    atomic.Bool
	stopOnce   sync.Once // guards close(quit) against double-close panic.
}

// NewConsolidationWorker initializes a ConsolidationWorker with the given TaxonomyEngine.
func NewConsolidationWorker(engine *TaxonomyEngine) *ConsolidationWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConsolidationWorker{
		engine:     engine,
		queue:      make(chan consolidationJob, consolidationQueueJobs),
		quit:       make(chan struct{}),
		byteBudget: consolidationQueueBytes,
		workCtx:    ctx,
		cancelWork: cancel,
	}
}

// Start begins the background worker goroutine for processing learning evaluations.
func (cw *ConsolidationWorker) Start() {
	cw.wg.Go(func() {
		for {
			select {
			case <-cw.quit:
				logging.Perception("ConsolidationWorker: shutting down, processing remaining items...")
				cw.drain()
				return
			case job := <-cw.queue:
				cw.run(job)
			}
		}
	})
	logging.Perception("ConsolidationWorker started in background")
}

// Stop closes admission, drains every accepted history and joins the worker,
// with no budget. Idempotent: the chat shutdown path, the perception close and
// test cleanup can all call it.
func (cw *ConsolidationWorker) Stop() {
	_ = cw.Shutdown(context.Background())
}

// Shutdown closes admission and drains the accepted queue until ctx ends. When
// ctx ends first, the in-flight job is cancelled, queued jobs are abandoned
// and counted, and the worker is joined before the error returns, so the
// caller may close the stores the worker used.
func (cw *ConsolidationWorker) Shutdown(ctx context.Context) error {
	cw.mu.Lock()
	cw.closed = true
	cw.mu.Unlock()
	cw.stopOnce.Do(func() { close(cw.quit) })

	joined := make(chan struct{})
	go func() {
		cw.wg.Wait()
		close(joined)
	}()
	select {
	case <-joined:
		cw.cancelWork()
		return nil
	case <-ctx.Done():
	}
	cw.abandon.Store(true)
	cw.cancelWork()
	<-joined
	st := cw.Stats()
	if st.Abandoned == 0 {
		// The budget ended while the last job was finishing; nothing was lost.
		return nil
	}
	return &ShutdownIncompleteError{Abandoned: st.Abandoned, Cause: ctx.Err()}
}

// Stats returns a snapshot of the admission and completion counters.
func (cw *ConsolidationWorker) Stats() ConsolidationStats {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	st := cw.stats
	st.QueuedJobs = len(cw.queue)
	st.QueuedBytes = cw.queuedBytes
	return st
}

// Enqueue submits a history of reasoning traces to be evaluated asynchronously.
// It never blocks: a full queue or a closed worker rejects the history, and
// the returned Admission says which.
func (cw *ConsolidationWorker) Enqueue(traces []ReasoningTrace) Admission {
	job := consolidationJob{traces: traces, bytes: traceBytes(traces)}

	cw.mu.Lock()
	defer cw.mu.Unlock()
	if cw.closed {
		cw.stats.RejectedClosed++
		logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker closed, rejecting learning opportunity")
		return AdmissionRejectedClosed
	}
	if len(cw.queue) > 0 && cw.queuedBytes+job.bytes > cw.byteBudget {
		cw.stats.RejectedFull++
		logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker queue over its byte budget (%d queued, %d more), dropping learning opportunity", cw.queuedBytes, job.bytes)
		return AdmissionRejectedFull
	}
	select {
	case cw.queue <- job:
		cw.queuedBytes += job.bytes
		cw.stats.Accepted++
		logging.PerceptionDebug("ConsolidationWorker: enqueued history of %d traces for asynchronous evaluation", len(traces))
		return AdmissionAccepted
	default:
		cw.stats.RejectedFull++
		logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker queue full, dropping learning opportunity")
		return AdmissionRejectedFull
	}
}

// traceBytes is the retained text of a history: every string the queue keeps
// alive until a job runs.
func traceBytes(traces []ReasoningTrace) int {
	n := 0
	for i := range traces {
		t := &traces[i]
		n += len(t.ID) + len(t.ShardID) + len(t.ShardType) + len(t.ShardCategory) + len(t.SessionID) +
			len(t.TaskContext) + len(t.SystemPrompt) + len(t.UserPrompt) + len(t.Response) +
			len(t.Model) + len(t.ErrorMessage)
		for _, note := range t.LearningNotes {
			n += len(note)
		}
	}
	return n
}

// run takes one accepted job to a terminal outcome: processed, or abandoned
// when a bounded Shutdown ran out of budget.
func (cw *ConsolidationWorker) run(job consolidationJob) {
	cw.mu.Lock()
	cw.queuedBytes -= job.bytes
	cw.mu.Unlock()

	if cw.abandon.Load() {
		cw.mu.Lock()
		cw.stats.Abandoned++
		cw.mu.Unlock()
		return
	}
	cw.process(job.traces)
	cw.mu.Lock()
	if cw.workCtx.Err() != nil {
		// Cancelled mid-flight by a bounded Shutdown: the job did not finish.
		cw.stats.Abandoned++
	} else {
		cw.stats.Processed++
	}
	cw.mu.Unlock()
}

func (cw *ConsolidationWorker) process(traces []ReasoningTrace) {
	if cw.engine == nil {
		return // misconstructed worker; nothing to learn into
	}
	// The batch is a critic call plus local persistence (and, when a pattern
	// is found, one embed). The critic call applies llm_timeouts.per_call_timeout
	// itself and the embed applies embedding.request_timeout; this batch has no
	// clock of its own. It is parented on workCtx, which only a bounded
	// Shutdown that ran out of budget cancels.
	fact, err := cw.engine.LearnFromInteraction(cw.workCtx, traces)
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker: learning failed: %v", err)
		return
	}

	if fact != "" {
		logging.Perception("ConsolidationWorker: successfully extracted new pattern: %s", fact)
		outcome, err := cw.engine.PersistLearnedFact(cw.workCtx, fact)
		if err != nil {
			logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker: failed to persist learned pattern (%s): %v", outcome, err)
			return
		}
		if !outcome.Complete() {
			logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker: learned pattern only partly projected (%s): %v", outcome, outcome.Err())
		}
	}
}

func (cw *ConsolidationWorker) drain() {
	for {
		select {
		case job := <-cw.queue:
			cw.run(job)
		default:
			return
		}
	}
}
