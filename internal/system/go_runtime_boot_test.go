package system

import (
	"os"
	"runtime/debug"
	"testing"

	"codenerd/internal/config"
)

func TestConfigureGoRuntimeAppliesCoreLimits(t *testing.T) {
	unsetenv(t, "GOGC")
	unsetenv(t, "GOMEMLIMIT")
	prevGC := debug.SetGCPercent(-1)
	debug.SetGCPercent(prevGC)
	prevLimit := debug.SetMemoryLimit(-1)
	t.Cleanup(func() {
		debug.SetGCPercent(prevGC)
		debug.SetMemoryLimit(prevLimit)
	})

	configureGoRuntime(config.CoreLimits{GCPercent: 300, MaxTotalMemoryMB: 2048})
	if got := debug.SetGCPercent(-1); got != 300 {
		t.Errorf("GOGC = %d, want 300 from core_limits.gc_percent", got)
	}
	debug.SetGCPercent(300)
	if got := debug.SetMemoryLimit(-1); got != 2048<<20 {
		t.Errorf("memory limit = %d, want %d from core_limits.max_total_memory_mb", got, int64(2048)<<20)
	}
}

func TestConfigureGoRuntimeLeavesEnvironmentOverrides(t *testing.T) {
	t.Setenv("GOGC", "150")
	t.Setenv("GOMEMLIMIT", "1GiB")
	prevGC := debug.SetGCPercent(-1)
	debug.SetGCPercent(prevGC)
	prevLimit := debug.SetMemoryLimit(-1)
	t.Cleanup(func() {
		debug.SetGCPercent(prevGC)
		debug.SetMemoryLimit(prevLimit)
	})
	configureGoRuntime(config.CoreLimits{GCPercent: 300, MaxTotalMemoryMB: 2048})
	if got := debug.SetGCPercent(-1); got != prevGC {
		t.Errorf("GOGC changed to %d although the environment sets it", got)
	}
	debug.SetGCPercent(prevGC)
	if got := debug.SetMemoryLimit(-1); got != prevLimit {
		t.Errorf("memory limit changed to %d although the environment sets it", got)
	}
}

// unsetenv removes key for the test and restores it afterwards.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		}
	})
}
