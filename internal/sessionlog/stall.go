package sessionlog

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
)

// Stall watch. A turn reports what it does through EmitLiveProgress: a model
// call starting and ending, a tool, a verification run. A turn that emits
// nothing for minutes is stalled or wedged, and a TUI shows only "preparing
// next action" while it is (a coder sat ten minutes on no model call and no
// tool, 2026-11-07). In debug-self mode the watch writes every goroutine's
// stack the first time a turn has been silent past the threshold, and logs
// where, so the stall is diagnosed while it is happening.

var stall struct {
	active atomic.Int64 // turns in progress (BeginActivity)
	last   atomic.Int64 // unix nanos of the last live event or activity start
	mu     sync.Mutex
	dumped int64 // the last value a dump was taken for; one dump per stall
}

// BeginActivity marks a turn in progress; the returned func ends it.
func BeginActivity() func() {
	stall.active.Add(1)
	stall.last.Store(time.Now().UnixNano())
	var once sync.Once
	return func() { once.Do(func() { stall.active.Add(-1) }) }
}

// beat records that a turn made progress.
func beat() { stall.last.Store(time.Now().UnixNano()) }

// StartStallWatch checks every interval whether a turn has been silent for
// threshold and, once per stall, calls dump with the reason and logs the
// file it wrote. It returns when ctx ends.
func StartStallWatch(ctx context.Context, threshold, interval time.Duration, dump func(reason string) (string, error)) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				checkStall(time.Now(), threshold, dump)
			}
		}
	}()
}

// checkStall is one watch tick, separate so a test drives the clock.
func checkStall(now time.Time, threshold time.Duration, dump func(reason string) (string, error)) bool {
	if stall.active.Load() == 0 {
		return false
	}
	last := stall.last.Load()
	silent := now.Sub(time.Unix(0, last))
	if silent < threshold {
		return false
	}
	stall.mu.Lock()
	defer stall.mu.Unlock()
	if stall.dumped == last {
		return false
	}
	stall.dumped = last
	reason := "a turn has emitted no progress for " + silent.Round(time.Second).String()
	path, err := dump(reason)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("stall watch: %s; goroutine dump failed: %v", reason, err)
		return true
	}
	logging.Get(logging.CategorySession).Warn("stall watch: %s; goroutines written to %s", reason, path)
	return true
}
