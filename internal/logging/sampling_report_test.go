package logging

import (
	"strings"
	"testing"
	"time"
)

// Sampling drops are counted and reported. Before, a non-slow timing that
// performance_sampling dropped left no trace, so a thinned performance log read
// exactly like a quiet run (WIRING-AND-NOT-BUILT: "Sampling drops data
// silently").
func TestPerformanceSampling_WhenTimingsAreDropped_ShouldCountAndReportThem(t *testing.T) {
	resetAllLoggingState(t)
	t.Cleanup(func() { resetAllLoggingState(t) })
	performanceSampleSeen.Store(0)
	performanceSampleDropped.Store(0)

	ws := newWorkspace(t, `"level": "debug", "debug_mode": true,
		"categories": {"performance": true, "kernel": true},
		"performance_sampling": 0.000001`)
	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	const timings = 200
	for range timings {
		logPerformance(CategoryKernel, "fast_op", time.Microsecond, nil)
	}
	seen, dropped := PerformanceSamplingStats()
	if seen != timings {
		t.Fatalf("the sampler saw %d timings, want %d", seen, timings)
	}
	if dropped < timings*9/10 {
		t.Fatalf("dropped = %d of %d at a sampling rate of 1e-6; the drops are not being counted", dropped, timings)
	}

	CloseAll()
	perf := readLog(t, ws, "performance")
	if !strings.Contains(perf, "performance sampling dropped") {
		t.Fatalf("the performance log does not say what sampling dropped:\n%s", perf)
	}
	if seen, dropped := PerformanceSamplingStats(); seen != 0 || dropped != 0 {
		t.Errorf("the counts were not restarted after the report: seen=%d dropped=%d", seen, dropped)
	}
}
