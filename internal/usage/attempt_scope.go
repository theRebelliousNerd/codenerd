package usage

import (
	"context"
	"errors"
	"sync"
)

// ErrUnknownSpend identifies a closed caller-owned provider attempt scope.
// It is not a price estimate, a provider error or a billing reservation.
var ErrUnknownSpend = errors.New("provider attempts stopped: spend is unknown")

type attemptScopeKey struct{}

// AttemptScope is a monotone cancellation/interlock owned by one caller.
// No scope is installed for ordinary requests. Already admitted work can still
// have reached the provider; cancellation does not establish a zero bill.
type AttemptScope struct {
	mu      sync.Mutex
	stopped bool
	cancel  context.CancelCauseFunc
	parent  *AttemptScope
}

// WithAttemptScope adds an independent stop scope, retaining any enclosing
// interlock. Context values preserve it across nested usage captures and even
// context.WithoutCancel; detaching cancellation cannot reopen admission.
func WithAttemptScope(ctx context.Context) (context.Context, *AttemptScope) {
	parent, _ := ctx.Value(attemptScopeKey{}).(*AttemptScope)
	ctx, cancel := context.WithCancelCause(ctx)
	s := &AttemptScope{cancel: cancel, parent: parent}
	return context.WithValue(ctx, attemptScopeKey{}, s), s
}

// StopUnknown closes admission before signaling cancellation. It is idempotent,
// non-blocking on provider work, and safe alongside reports and admissions.
func (s *AttemptScope) StopUnknown() {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.cancel(ErrUnknownSpend)
}

// Close releases the owned context without manufacturing usage uncertainty.
func (s *AttemptScope) Close() { s.cancel(context.Canceled) }

// BeforeAttempt is called immediately before provider dispatch. It is a no-op
// without an explicit scope, preserving ordinary client retry behavior. Its
// mutex check linearizes admission against StopUnknown: work already admitted
// before closure is in flight, not a reservation or proof of unbilled work.
func BeforeAttempt(ctx context.Context) error {
	s, _ := ctx.Value(attemptScopeKey{}).(*AttemptScope)
	if s == nil {
		return nil
	}
	for current := s; current != nil; current = current.parent {
		current.mu.Lock()
		stopped := current.stopped
		current.mu.Unlock()
		if stopped {
			return ErrUnknownSpend
		}
	}
	return ctx.Err()
}

// HasAttemptScope distinguishes explicitly owned requests from ordinary calls.
func HasAttemptScope(ctx context.Context) bool {
	s, _ := ctx.Value(attemptScopeKey{}).(*AttemptScope)
	return s != nil
}
