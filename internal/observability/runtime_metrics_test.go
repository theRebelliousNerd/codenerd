package observability

import (
	"runtime"
	"runtime/metrics"
	"strconv"
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

func TestFormatSample_ValueKinds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		metric   string
		kind     metrics.ValueKind
		expected func(metrics.Value) (any, string)
	}{
		{
			name:   "float64 preserves numeric value and six decimal places",
			metric: "/cpu/classes/total:cpu-seconds",
			kind:   metrics.KindFloat64,
			expected: func(v metrics.Value) (any, string) {
				return v.Float64(), strconv.FormatFloat(v.Float64(), 'f', 6, 64)
			},
		},
		{
			name:   "histogram reports bucket boundaries rather than counts",
			metric: "/gc/heap/allocs-by-size:bytes",
			kind:   metrics.KindFloat64Histogram,
			expected: func(v metrics.Value) (any, string) {
				buckets := len(v.Float64Histogram().Buckets)
				return buckets, "hist[" + strconv.Itoa(buckets) + "]"
			},
		},
		{
			name:   "unsupported metric remains unavailable",
			metric: "/codenerd/unsupported:metric",
			kind:   metrics.KindBad,
			expected: func(metrics.Value) (any, string) {
				return nil, "n/a"
			},
		},
		{
			name: "zero sample remains unavailable",
			kind: metrics.KindBad,
			expected: func(metrics.Value) (any, string) {
				return nil, "n/a"
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			samples := []metrics.Sample{{Name: tc.metric}}
			if tc.metric != "" {
				metrics.Read(samples)
			}
			sample := samples[0]
			if got := sample.Value.Kind(); got != tc.kind {
				t.Fatalf("metric %q kind = %v, want %v", tc.metric, got, tc.kind)
			}
			wantValue, wantDisplay := tc.expected(sample.Value)
			gotValue, gotDisplay := formatSample(sample)
			if gotValue != wantValue {
				t.Errorf("formatSample() value = %v (%T), want %v (%T)", gotValue, gotValue, wantValue, wantValue)
			}
			if gotDisplay != wantDisplay {
				t.Errorf("formatSample() display = %q, want %q", gotDisplay, wantDisplay)
			}
		})
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

func TestStartupMetricPaths_NonEmpty(t *testing.T) {
	t.Parallel()
	if len(startupMetricPaths) == 0 {
		t.Fatal("startupMetricPaths must not be empty: boot snapshot must sample something")
	}
}

func TestMetricFieldKey_EdgeCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"/a/b", "a_b"},
		{"a/b:c", "a_b"},
		{"foo", "foo"},
		{"/a-b/c-d:e-f", "a_b_c_d"},
		{"", ""},
		{"/sched/goroutines:goroutines", "sched_goroutines"},
	}
	for _, c := range cases {
		if got := metricFieldKey(c.in); got != c.want {
			t.Errorf("metricFieldKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestShortMetric_EdgeCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"total", "total"},
		{"/a/total:x", "a_total"},
		{"/a/b", "b"},
		{"a/b:c", "b"},
		{"", ""},
		{"/a/live:bytes", "a_live"},
		{"/sched/goroutines:goroutines", "goroutines"},
	}
	for _, c := range cases {
		if got := shortMetric(c.in); got != c.want {
			t.Errorf("shortMetric(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGreenTeaStatus_NogreenteagcDisables(t *testing.T) {
	t.Setenv("GOEXPERIMENT", "nogreenteagc")
	enabled, detail := greenTeaStatus()
	if enabled {
		t.Fatalf("greenTeaStatus with GOEXPERIMENT=nogreenteagc must report disabled, detail=%q", detail)
	}
	if detail == "" {
		t.Fatal("greenTeaStatus returned empty detail")
	}
}

func TestGreenTeaStatus_UnrelatedExperimentStaysEnabled(t *testing.T) {
	t.Setenv("GOEXPERIMENT", "greenteaon")
	enabled, detail := greenTeaStatus()
	if detail == "" {
		t.Fatal("greenTeaStatus returned empty detail")
	}
	// A build baked with nogreenteagc stays disabled regardless of env;
	// otherwise an unrelated experiment flag must keep Green Tea enabled.
	if !enabled {
		if strings.Contains(detail, "nogreenteagc") {
			t.Skip("binary built with GOEXPERIMENT=nogreenteagc, env cannot re-enable")
		}
		t.Fatalf("greenTeaStatus with unrelated GOEXPERIMENT must stay enabled, detail=%q", detail)
	}
}
