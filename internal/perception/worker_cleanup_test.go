package perception

import "context"

// Stop is Shutdown with no budget, for test cleanup. Production stops the
// worker through ShutdownWorker, bounded (Cortex.Close); an unbounded stop
// has no production caller.
func (cw *ConsolidationWorker) Stop() {
	_ = cw.Shutdown(context.Background())
}

// StopWorker is ShutdownWorker with no budget, for test cleanup.
func (t *TaxonomyEngine) StopWorker() {
	_ = t.ShutdownWorker(context.Background())
}
