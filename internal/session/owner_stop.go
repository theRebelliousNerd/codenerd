package session

import "context"

// The operation owner's stop, carried past a soft deadline.
//
// A command's --timeout is a deadline on the work's context; the owner (the
// process's signal handling, the operator) is its parent. Once the deadline
// fires, the work's context reports DeadlineExceeded for good -- a context's
// first error is permanent -- so a later SIGTERM or operator cancel can no
// longer be seen through it. Verification deliberately outlives a soft
// deadline, and so needs the owner's stop as its own channel: WithOwnerStop
// records the owner on the context, and runVerificationCommand watches it
// (audit verification-cancellation-after-deadline).

type ownerStopKey struct{}

// WithOwnerStop records owner as the operation owner of ctx: when owner is
// done, work that outlives ctx's own deadline stops too.
func WithOwnerStop(ctx, owner context.Context) context.Context {
	if owner == nil {
		return ctx
	}
	return context.WithValue(ctx, ownerStopKey{}, owner)
}

// ownerStop returns the owner's done channel, nil when ctx records none.
func ownerStop(ctx context.Context) <-chan struct{} {
	if owner, ok := ctx.Value(ownerStopKey{}).(context.Context); ok {
		return owner.Done()
	}
	return nil
}
