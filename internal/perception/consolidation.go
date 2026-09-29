package perception

import (
	"context"
	"sync"

	"codenerd/internal/logging"
)

// ConsolidationWorker manages an asynchronous "sleep cycle" for learning taxonomy patterns
// from recent interactions without blocking the main execution loop.
type ConsolidationWorker struct {
	engine   *TaxonomyEngine
	queue    chan []ReasoningTrace
	quit     chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once // Bug #17: guards close(quit) against double-close panic.
}

// NewConsolidationWorker initializes a ConsolidationWorker with the given TaxonomyEngine.
// By default, it uses a buffered channel to hold pending traces.
func NewConsolidationWorker(engine *TaxonomyEngine) *ConsolidationWorker {
	return &ConsolidationWorker{
		engine: engine,
		queue:  make(chan []ReasoningTrace, 100), // Buffer capacity for 100 interaction histories
		quit:   make(chan struct{}),
	}
}

// Start begins the background worker goroutine for processing learning evaluations.
func (cw *ConsolidationWorker) Start() {
	cw.wg.Go(func() {
		for {
			select {
			case <-cw.quit:
				logging.Perception("ConsolidationWorker: shutting down, processing remaining items...")
				// Drain the queue if possible before fully shutting down
				cw.drain()
				return
			case traces := <-cw.queue:
				cw.process(traces)
			}
		}
	})
	logging.Perception("ConsolidationWorker started in background")
}

// Stop signals the worker to finish processing the queue and shut down.
// Stop is idempotent: subsequent calls are no-ops. This matters because both
// the chat-session Shutdown path and ad-hoc test cleanup paths can call
// StopWorker; double-closing the quit channel would panic.
func (cw *ConsolidationWorker) Stop() {
	cw.stopOnce.Do(func() {
		close(cw.quit)
		cw.wg.Wait()
	})
}

// Enqueue submits a history of reasoning traces to be evaluated asynchronously.
// If the queue is full, it drops the trace to prevent blocking the caller.
func (cw *ConsolidationWorker) Enqueue(traces []ReasoningTrace) {
	select {
	case cw.queue <- traces:
		logging.PerceptionDebug("ConsolidationWorker: enqueued history of %d traces for asynchronous evaluation", len(traces))
	default:
		logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker queue full, dropping learning opportunity")
	}
}

func (cw *ConsolidationWorker) process(traces []ReasoningTrace) {
	if cw.engine == nil {
		return // misconstructed worker; nothing to learn into
	}
	// No clock around the batch. It is a critic call plus local persistence
	// (and, when a pattern is found, one embed). The worker stops when Stop
	// closes quit, after drain. The critic call applies
	// llm_timeouts.per_call_timeout itself; the embed applies
	// embedding.request_timeout. A cancelled context here would abort the
	// drain Stop is waiting on, so this batch is not parented on one.
	fact, err := cw.engine.LearnFromInteraction(context.Background(), traces)
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker: learning failed: %v", err)
		return
	}

	if fact != "" {
		logging.Perception("ConsolidationWorker: successfully extracted new pattern: %s", fact)
		if err := cw.engine.PersistLearnedFact(fact); err != nil {
			logging.Get(logging.CategoryPerception).Warn("ConsolidationWorker: failed to persist learned pattern: %v", err)
		}
	}
}

func (cw *ConsolidationWorker) drain() {
	for {
		select {
		case traces := <-cw.queue:
			cw.process(traces)
		default:
			return
		}
	}
}
