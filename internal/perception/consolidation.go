package perception

import (
	"context"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// ConsolidationWorker manages an asynchronous "sleep cycle" for learning taxonomy patterns
// from recent interactions without blocking the main execution loop.
type ConsolidationWorker struct {
	engine *TaxonomyEngine
	queue  chan []ReasoningTrace
	quit   chan struct{}
	wg     sync.WaitGroup
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
	cw.wg.Add(1)
	go func() {
		defer cw.wg.Done()
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
	}()
	logging.Perception("ConsolidationWorker started in background")
}

// Stop signals the worker to finish processing the queue and shut down.
func (cw *ConsolidationWorker) Stop() {
	close(cw.quit)
	cw.wg.Wait()
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
	// Create a background context with timeout to ensure we don't leak resources
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fact, err := cw.engine.LearnFromInteraction(ctx, traces)
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
