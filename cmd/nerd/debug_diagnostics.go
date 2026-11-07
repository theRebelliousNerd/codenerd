package main

import (
	"context"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/sessionlog"
)

// stallThreshold is how long an active turn may emit no progress before the
// debug-self stall watch dumps every goroutine. A model call or a gate that
// runs longer still reports its start, so silence this long is a stall.
const stallThreshold = 10 * time.Minute

var diagnosticsOnce sync.Once

// startDebugSelfDiagnostics installs the on-demand goroutine dump (SIGUSR1
// where the platform has it) and, in debug-self mode, the stall watch. It runs
// once per process, after logging knows its directory and config.
func startDebugSelfDiagnostics() {
	diagnosticsOnce.Do(func() {
		installGoroutineDumpSignal()
		if logging.IsDebugSelf() {
			sessionlog.StartStallWatch(context.Background(), stallThreshold, time.Minute, logging.WriteGoroutineDump)
			logging.Boot("debug-self: stall watch on (dumps goroutines after %s without progress in a turn)", stallThreshold)
		}
	})
}
