package observability

import (
	"runtime"
	"runtime/metrics"
	"strings"
	"testing"
)

func TestLogStartupMetrics_NoPanic(t *testing.T) {
	// Should never panic regardless of logging configuration; runs in
	// "production" mode (debug_mode off) by default since no .nerd/
	// config.json exists for this test process.
	LogStartupMetrics()
}

func TestStartupMetricPaths_AllSupported(t *testing.T) {
	// All hard-coded metric paths must be supported by the running
	// runtime; otherwise samples will silently come back as KindBad and
	// downstream dashboards will see "n/a" values.
	descs := metrics.All()
	known := make(map[string]struct{}, len(descs))
	for _, d := range descs {
		known[d.Name] = struct{}{}
	}
	for _, p := range startupMetricPaths {
		if _, ok := known[p]; !ok {
			t.Errorf("metric path %q not present in runtime/metrics.All() — Go %s", p, runtime.Version())
		}
	}
}

func TestGOMAXPROCSReported(t *testing.T) {
	// Sanity check: GOMAXPROCS(0) must report a positive value on every
	// supported platform.
	v := runtime.GOMAXPROCS(0)
	if v < 1 {
		t.Fatalf("GOMAXPROCS(0) = %d, want >=1", v)
	}
}

func TestFormatSample_Uint64(t *testing.T) {
	samples := []metrics.Sample{{Name: "/sched/goroutines:goroutines"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() != metrics.KindUint64 {
		t.Fatalf("expected uint64 sample, got kind %v", samples[0].Value.Kind())
	}
	v, display := formatSample(samples[0])
	if _, ok := v.(uint64); !ok {
		t.Errorf("expected uint64 value, got %T", v)
	}
	if display == "" || display == "n/a" {
		t.Errorf("expected numeric display string, got %q", display)
	}
}

func TestMetricFieldKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/sched/goroutines:goroutines", "sched_goroutines"},
		{"/gc/heap/goal:bytes", "gc_heap_goal"},
		{"/cpu/classes/gc/total:cpu-seconds", "cpu_classes_gc_total"},
		{"/sched/goroutines-created:goroutines", "sched_goroutines_created"},
	}
	for _, c := range cases {
		if got := metricFieldKey(c.in); got != c.want {
			t.Errorf("metricFieldKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestShortMetric(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/sched/goroutines:goroutines", "goroutines"},
		{"/gc/heap/goal:bytes", "heap_goal"},              // generic leaf disambiguated
		{"/cpu/classes/gc/total:cpu-seconds", "gc_total"}, // generic leaf disambiguated
		{"/memory/classes/total:bytes", "classes_total"},  // generic leaf disambiguated
		{"/gc/cycles/total:gc-cycles", "cycles_total"},    // generic leaf disambiguated
		{"/sched/gomaxprocs:threads", "gomaxprocs"},
	}
	for _, c := range cases {
		if got := shortMetric(c.in); got != c.want {
			t.Errorf("shortMetric(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGreenTeaStatus(t *testing.T) {
	// We can't reliably control GOEXPERIMENT for this process, but the
	// returned detail must be non-empty and consistent with the bool.
	enabled, detail := greenTeaStatus()
	if detail == "" {
		t.Fatal("greenTeaStatus returned empty detail")
	}
	if !enabled && !strings.Contains(detail, "nogreenteagc") {
		t.Errorf("disabled status without nogreenteagc in detail: %q", detail)
	}
}
