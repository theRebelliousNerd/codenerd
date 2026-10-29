package usage

import (
	"context"

	"codenerd/internal/types"
)

// Report is one provider usage report in normalized form: which engine billed
// it, for which operation, and the counts (see types.UsageMetadata for the
// category semantics). An adapter builds it once per billed attempt, and the
// same value reaches both consumers — the tracker's aggregates and any
// in-flight Observer — so neither can know something the other was denied.
type Report struct {
	Model     string
	Provider  string
	Operation string
	Usage     types.UsageMetadata
}

// Observer receives every usage report that passes through RecordFromContext,
// in addition to (not instead of) the Tracker's own aggregation.
//
// This exists so a caller can capture the provider's *actual* reported counts
// for a single in-flight request without touching the provider clients that
// produce them. The broker installs one per call and builds its receipt from
// what it observes.
//
// Implementations must be safe for concurrent use: a streaming turn and a tool
// call can report on different goroutines against the same context.
type Observer interface {
	// Observed is called once per provider usage report. It must not block;
	// the caller is on the request's hot path.
	Observed(r Report)
}

type observerKey struct{}

// WithObserver returns a context that also delivers usage reports to obs.
// A nil observer returns ctx unchanged so callers can pass through optional
// wiring without branching.
func WithObserver(ctx context.Context, obs Observer) context.Context {
	if ctx == nil || obs == nil {
		return ctx
	}
	return context.WithValue(ctx, observerKey{}, obs)
}

// ObserverFromContext retrieves the observer installed by WithObserver, or nil.
func ObserverFromContext(ctx context.Context) Observer {
	if ctx == nil {
		return nil
	}
	obs, _ := ctx.Value(observerKey{}).(Observer)
	return obs
}
