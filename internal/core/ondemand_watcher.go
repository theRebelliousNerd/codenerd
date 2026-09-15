package core

import (
	"context"
	"sync"
	"time"

	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
)

// On-demand activation tuning. The debounce window collapses trigger bursts
// (a file save asserts several modified/1 facts; every turn asserts
// user_intent) into one activate_shard re-query. The fallback ticker follows
// the FactEventBus contract: publish drops events when a subscriber's buffer
// is full, so a periodic sweep catches anything the event path missed.
const (
	onDemandDebounceWindow  = time.Second
	onDemandFallbackSweep   = 30 * time.Second
)

// StartOnDemandWatcher subscribes to the on-demand trigger predicates and
// invokes ensure after every trigger burst and every fallback sweep, so
// kernel-derived activate_shard/1 facts actually start shards at runtime.
// The zero value to pass for ensure is
// ShardManager.EnsureOnDemandShards, bound to the cortex shard manager.
//
// The watcher stops when ctx is done or when the returned stop func runs;
// both paths unsubscribe from the bus, so neither leaks a subscription.
// stop is idempotent and waits for the watcher goroutine to exit.
//
// Exactly one watcher should run per kernel: Ensure skips already-active
// shards, but two watchers racing the same trigger can still double-spawn
// before either spawn lands.
func StartOnDemandWatcher(ctx context.Context, bus *FactEventBus, ensure func(context.Context) []string) (stop func()) {
	if bus == nil || ensure == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer cancel()
		events := bus.Subscribe(append([]string(nil), coreshards.OnDemandTriggerPredicates...))
		defer bus.Unsubscribe(events)

		sweep := time.NewTicker(onDemandFallbackSweep)
		defer sweep.Stop()
		var debounce <-chan time.Time

		run := func(reason string) {
			if spawned := ensure(ctx); len(spawned) > 0 {
				logging.Shards("OnDemandWatcher(%s): started %v", reason, spawned)
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-events:
				if !ok {
					return
				}
				debounce = time.After(onDemandDebounceWindow)
			case <-debounce:
				debounce = nil
				run("trigger")
			case <-sweep.C:
				run("sweep")
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}
