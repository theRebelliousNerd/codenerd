package usage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAttemptScopeIsolationAndCancellation(t *testing.T) {
	ctx, scope := WithAttemptScope(t.Context())
	defer scope.Close()
	other, independent := WithAttemptScope(t.Context())
	defer independent.Close()
	child, nested := WithAttemptScope(ctx)
	defer nested.Close()
	if err := BeforeAttempt(child); err != nil {
		t.Fatal(err)
	}
	scope.StopUnknown()
	for _, c := range []context.Context{ctx, child, context.WithoutCancel(ctx), context.WithoutCancel(child)} {
		if err := BeforeAttempt(c); !errors.Is(err, ErrUnknownSpend) {
			t.Fatalf("closed scope admitted: %v", err)
		}
	}
	if !errors.Is(context.Cause(ctx), ErrUnknownSpend) || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("cancellation cause lost: %v/%v", ctx.Err(), context.Cause(ctx))
	}
	for _, c := range []context.Context{other, t.Context(), context.Background()} {
		if err := BeforeAttempt(c); err != nil {
			t.Fatalf("unrelated scope stopped: %v", err)
		}
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	owned, owner := WithAttemptScope(canceled)
	defer owner.Close()
	if err := BeforeAttempt(owned); !errors.Is(err, context.Canceled) {
		t.Fatalf("caller cancellation lost: %v", err)
	}
	if err := BeforeAttempt(canceled); err != nil {
		t.Fatalf("ordinary behavior changed: %v", err)
	}
}

func TestAttemptScopeConcurrentStopIsMonotone(t *testing.T) {
	ctx, scope := WithAttemptScope(t.Context())
	defer scope.Close()
	detached := context.WithoutCancel(ctx)
	var workers sync.WaitGroup
	start := make(chan struct{})
	var violations atomic.Int32
	for range 32 {
		workers.Go(func() {
			<-start
			for range 32 {
				_ = BeforeAttempt(detached)
			}
			scope.StopUnknown()
			for range 32 {
				if !errors.Is(BeforeAttempt(detached), ErrUnknownSpend) {
					violations.Add(1)
				}
			}
		})
	}
	close(start)
	workers.Wait()
	if violations.Load() != 0 {
		t.Fatalf("reopened admissions: %d", violations.Load())
	}
}
