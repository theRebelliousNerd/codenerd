// Package ownerstop carries an operation owner's stop past a soft deadline.
//
// A command's --timeout is a deadline on the work's context; the owner (the
// process's signal handling, the operator) is its parent. Once the deadline
// fires, the work's context reports DeadlineExceeded for good -- a context's
// first error is permanent -- so a later SIGTERM or operator cancel can no
// longer be seen through it. Verification deliberately outlives a soft
// deadline, and so needs the owner's stop as its own channel: With records the
// owner on the context, and the session's verification and diagnostics watch
// Done (audit verification-cancellation-after-deadline).
//
// The package is a dependency-free leaf on purpose: the CLI's process-level
// signal-lifetime test builds a -race fixture around the real With, and that
// build stays small only while this package imports nothing heavy.
package ownerstop

import "context"

type key struct{}

// With records owner as the operation owner of ctx: when owner is done, work
// that outlives ctx's own deadline stops too.
func With(ctx, owner context.Context) context.Context {
	if owner == nil {
		return ctx
	}
	return context.WithValue(ctx, key{}, owner)
}

// Done returns the owner's done channel, nil when ctx records none.
func Done(ctx context.Context) <-chan struct{} {
	if owner, ok := ctx.Value(key{}).(context.Context); ok {
		return owner.Done()
	}
	return nil
}
