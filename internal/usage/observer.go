package usage

import "context"

// Observer receives every usage report that passes through TrackFromContext,
// in addition to (not instead of) the Tracker's own aggregation.
//
// This exists so a caller can capture the provider's *actual* reported token
// counts for a single in-flight request without touching the nine provider
// clients that produce them. Every client in internal/perception already calls
// trackUsage with the numbers the provider returned; before this hook those
// numbers reached the cost aggregator and nothing else, which is why every
// budgeting decision in the codebase was made against an estimate while the
// exact figure sat one function call away.
//
// Implementations must be safe for concurrent use: a streaming turn and a tool
// call can report on different goroutines against the same context.
type Observer interface {
	// Observed is called once per provider usage report. It must not block;
	// the caller is on the request's hot path.
	Observed(model, provider string, input, output int, operation string)
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
