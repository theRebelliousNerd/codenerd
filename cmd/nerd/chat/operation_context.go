package chat

import (
	"context"

	"codenerd/internal/sessionlog"
	"codenerd/internal/usage"
)

// sessionOperationContext is what a chat operation that outlives one message
// runs under -- a delegated shard, a scan, an ingestion, a tool run: the
// session's shutdown context, so quitting or Ctrl+X cancels it, and no clock
// of its own. Until 2026-09-19 these ran under 10-30 minute ceilings from
// llm_timeouts, most of them parented on context.Background(), so the ceiling
// was the only thing that could ever stop them; an hours-long task was cut
// and a stuck one could not be cancelled. A stalled task is stopped by the
// working policy, and each model request by its client's own timeout.
//
// It also carries the session's usage tracker and session id. LLM clients
// meter through the context (perception.trackUsage), so an operation
// context without them spends invisibly: until 2026-10-27 every /init
// planner call ran under a context built here and recorded nothing in
// usage.json, and chat turns recorded their session as "unknown".
func (m Model) sessionOperationContext() (context.Context, context.CancelFunc) {
	parent := m.shutdownCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	if m.usageTracker != nil {
		ctx = usage.NewContext(ctx, m.usageTracker)
	}
	ctx = usage.WithSessionID(ctx, m.sessionID)
	ctx = sessionlog.WithChat(ctx)
	if m.delegatedEvents != nil {
		ctx = sessionlog.WithLiveProgress(ctx, m.sendDelegatedProgress)
	}
	return ctx, cancel
}
