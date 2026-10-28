package system

import (
	"os"
	"runtime/debug"

	"codenerd/internal/config"
	"codenerd/internal/logging"
)

// configureGoRuntime applies core_limits to the Go garbage collector: the GC
// target (gc_percent) and max_total_memory_mb as the soft memory limit. A
// boot spent 57% of its CPU in GC under the runtime's default target
// (measured 2026-10-27, 28.6 s wall, 70 s CPU). The soft limit bounds what the
// higher target costs: near it the collector works harder rather than letting
// the heap grow past it. GOGC and GOMEMLIMIT, when set in the environment, win,
// so a run can still be tuned or constrained without editing config.
func configureGoRuntime(limits config.CoreLimits) {
	log := logging.Get(logging.CategoryBoot)
	if _, set := os.LookupEnv("GOGC"); !set && limits.GCPercent > 0 {
		debug.SetGCPercent(limits.GCPercent)
	}
	if _, set := os.LookupEnv("GOMEMLIMIT"); !set && limits.MaxTotalMemoryMB > 0 {
		debug.SetMemoryLimit(int64(limits.MaxTotalMemoryMB) << 20)
	}
	gogc := debug.SetGCPercent(-1)
	debug.SetGCPercent(gogc)
	log.Info("go runtime: GOGC=%d soft memory limit=%d MiB", gogc, debug.SetMemoryLimit(-1)>>20)
}
