package autopoiesis

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestKernelListenerSingletonPanicRecoveryAndCancellation(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	var polls atomic.Int32
	observed := make(chan struct{}, 1)
	kernel := &MockKernelInterface{QueryPredicateFunc: func(predicate string) ([]types.Fact, error) {
		if predicate == "delegate_task" {
			if polls.Add(1) == 1 {
				panic("negative-control poll panic")
			}
			select {
			case observed <- struct{}{}:
			default:
			}
		}
		return nil, nil
	}}
	orch.SetKernel(kernel)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := orch.StartKernelListener(ctx, time.Millisecond)
	if duplicate := orch.StartKernelListener(ctx, time.Millisecond); duplicate != done {
		t.Fatal("duplicate listener started")
	}
	select {
	case <-observed:
	case <-time.After(time.Second):
		t.Fatal("listener did not recover from panic")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("listener did not join cancellation")
	}
	count := polls.Load()
	if _, err := orch.processDelegationsSafely(ctx); err == nil {
		t.Fatal("canceled listener still admits work")
	}
	if polls.Load() != count {
		t.Fatal("canceled listener queried kernel")
	}
}
