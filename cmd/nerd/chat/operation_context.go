package chat

import "context"

// sessionOperationContext is what a chat operation that outlives one message
// runs under -- a delegated shard, a scan, an ingestion, a tool run: the
// session's shutdown context, so quitting or Ctrl+X cancels it, and no clock
// of its own. Until 2026-09-19 these ran under 10-30 minute ceilings from
// llm_timeouts, most of them parented on context.Background(), so the ceiling
// was the only thing that could ever stop them; an hours-long task was cut
// and a stuck one could not be cancelled. A stalled task is stopped by the
// working policy, and each model request by its client's own timeout.
func (m Model) sessionOperationContext() (context.Context, context.CancelFunc) {
	parent := m.shutdownCtx
	if parent == nil {
		parent = context.Background()
	}
	return context.WithCancel(parent)
}
