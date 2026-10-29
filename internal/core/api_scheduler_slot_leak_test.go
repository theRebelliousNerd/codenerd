package core

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestAPIScheduler_CancelVsGrant_NoSlotLeak reproduces the TOCTOU where
// ReleaseAPISlot/forceReleaseSlot grants (pop waiter + currentlyExecuting++ +
// close ch) while AcquireAPISlot's cancel path used to removeWaiter no-op and
// return context.Canceled without reclaiming — permanent slot leak.
//
// Skeptic repro: ~5/2000 iterations left currentlyExecuting=1 with waitErr=canceled.
func TestAPIScheduler_CancelVsGrant_NoSlotLeak(t *testing.T) {
	const iterations = 2000

	for i := 0; i < iterations; i++ {
		s := NewAPIScheduler(APISchedulerConfig{
			MaxConcurrentAPICalls: 1,
			SlotAcquireTimeout:    5 * time.Second,
			MinCallSpacing:        0, // isolate grant/cancel race from spacing
		})
		s.RegisterShard("holder", "test")
		s.RegisterShard("racer", "test")

		ctx := context.Background()
		if err := s.AcquireAPISlot(ctx, "holder"); err != nil {
			t.Fatalf("iter %d: holder acquire: %v", i, err)
		}

		cancelCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			done <- s.AcquireAPISlot(cancelCtx, "racer")
		}()

		// Let racer enter the wait queue.
		time.Sleep(time.Duration(i%3) * time.Millisecond)

		// Fire cancel and grant in either order to stress both race windows.
		if i%2 == 0 {
			cancel()
			s.ReleaseAPISlot("holder")
		} else {
			s.ReleaseAPISlot("holder")
			cancel()
		}

		waitErr := <-done

		// Normalize: whoever holds the slot must release so metrics settle.
		// - waitErr == nil: racer was granted (possibly while cancel raced) → racer holds slot
		// - waitErr != nil: racer did not keep the slot → holder release already happened;
		//   if grant was honored then cancelled during finishGranted, slot is free.
		if waitErr == nil {
			s.ReleaseAPISlot("racer")
		}

		m := s.GetMetrics()
		if m.ActiveSlots != 0 {
			t.Fatalf("iter %d: slot leak: ActiveSlots=%d waitErr=%v (want ActiveSlots=0)",
				i, m.ActiveSlots, waitErr)
		}
		if len(s.waiters) != 0 {
			t.Fatalf("iter %d: waiter leak: %d waiters remain waitErr=%v", i, len(s.waiters), waitErr)
		}
	}
}

// TestAPIScheduler_CancelVsGrant_SpacingAlsoReclaims covers forceReleaseSlot's
// grant path (spacing cancel) under the same cancel/grant race.
func TestAPIScheduler_CancelVsGrant_SpacingAlsoReclaims(t *testing.T) {
	const iterations = 500

	for i := 0; i < iterations; i++ {
		s := NewAPIScheduler(APISchedulerConfig{
			MaxConcurrentAPICalls: 1,
			SlotAcquireTimeout:    5 * time.Second,
			// Non-zero spacing: finishGranted holds the slot then may forceRelease
			// when ctx is already canceled — that grants the next waiter.
			MinCallSpacing: 5 * time.Millisecond,
		})
		s.RegisterShard("holder", "test")
		s.RegisterShard("racer", "test")
		s.RegisterShard("racer2", "test")

		ctx := context.Background()
		if err := s.AcquireAPISlot(ctx, "holder"); err != nil {
			t.Fatalf("iter %d: holder: %v", i, err)
		}

		var wg sync.WaitGroup
		errs := make([]error, 2)
		for idx, id := range []string{"racer", "racer2"} {
			wg.Add(1)
			go func(j int, shardID string) {
				defer wg.Done()
				cctx, ccancel := context.WithCancel(ctx)
				// Cancel shortly after starting — races grant from forceRelease/Release.
				go func() {
					time.Sleep(time.Duration(j+1) * time.Millisecond)
					ccancel()
				}()
				errs[j] = s.AcquireAPISlot(cctx, shardID)
				if errs[j] == nil {
					s.ReleaseAPISlot(shardID)
				}
			}(idx, id)
		}

		time.Sleep(2 * time.Millisecond)
		s.ReleaseAPISlot("holder")
		wg.Wait()

		m := s.GetMetrics()
		if m.ActiveSlots != 0 {
			t.Fatalf("iter %d: ActiveSlots=%d after spacing cancel races (errs=%v)",
				i, m.ActiveSlots, errs)
		}
	}
}

// TestAPIScheduler_GrantedOnCancelHonorsOrReclaims is a single-shot deterministic
// check: after simultaneous cancel+release, either racer holds (and can release)
// or ActiveSlots is 0 — never stuck at 1 with canceled error.
func TestAPIScheduler_GrantedOnCancelHonorsOrReclaims(t *testing.T) {
	s := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    time.Second,
		MinCallSpacing:        0,
	})
	s.RegisterShard("holder", "test")
	s.RegisterShard("racer", "test")

	ctx := context.Background()
	if err := s.AcquireAPISlot(ctx, "holder"); err != nil {
		t.Fatal(err)
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- s.AcquireAPISlot(cancelCtx, "racer") }()
	time.Sleep(10 * time.Millisecond)

	cancel()
	s.ReleaseAPISlot("holder")
	waitErr := <-done

	active := s.GetMetrics().ActiveSlots
	switch {
	case waitErr == nil && active == 1:
		s.ReleaseAPISlot("racer")
		if s.GetMetrics().ActiveSlots != 0 {
			t.Fatalf("release after grant left ActiveSlots=%d", s.GetMetrics().ActiveSlots)
		}
	case waitErr != nil && active == 0:
		// pure cancel — correct
	case waitErr != nil && active == 1:
		t.Fatalf("SLOT LEAK: waitErr=%v ActiveSlots=1 (granted but returned cancel without reclaim)", waitErr)
	default:
		t.Fatalf("unexpected state waitErr=%v ActiveSlots=%d", waitErr, active)
	}
}
