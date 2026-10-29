// Package observability provides startup runtime diagnostics and trace
// capture utilities. It is a leaf package: it depends only on the standard
// library and codenerd/internal/logging.
//
// The two main entry points are:
//
//   - LogStartupMetrics() — samples Go scheduler / GC / memory counters
//     once at boot, plus toolchain and Green Tea GC status, and emits the
//     results to the boot logger.
//   - StartFlightRecorder() / DumpFlightRecord() — wrap Go 1.25+
//     runtime/trace.FlightRecorder so a rolling window of execution traces
//     can be persisted on panic.
package observability

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"strings"

	"codenerd/internal/logging"
)

// startupMetricPaths is the canonical set of runtime/metrics samples taken
// at boot. The list intentionally targets Go 1.26 scheduler, GC and memory
// counters that are stable and inexpensive to read. Only paths present in
// runtime/metrics.All() are listed; the package's tests verify each one
// against the live runtime so a Go upgrade that retires a path will
// surface immediately.
var startupMetricPaths = []string{
	"/sched/goroutines:goroutines",
	"/sched/gomaxprocs:threads",
	"/gc/heap/goal:bytes",
	"/gc/heap/live:bytes",
	"/gc/cycles/total:gc-cycles",
	"/cpu/classes/gc/total:cpu-seconds",
	"/memory/classes/total:bytes",
}

// LogStartupMetrics samples runtime metrics once at boot and writes a
// single human-readable INFO line plus a structured event to the boot
// logger. It is safe to call before metric values stabilize; the sample
// is a one-shot snapshot intended for diagnostics, not monitoring.
//
// Should be invoked from main() after logging.Initialize() has run.
func LogStartupMetrics() {
	samples := make([]metrics.Sample, len(startupMetricPaths))
	for i, p := range startupMetricPaths {
		samples[i].Name = p
	}
	metrics.Read(samples)

	gomaxprocs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	goVer := runtime.Version()

	// Build a flat map for structured logging plus a compact one-line
	// summary for humans tailing the boot log.
	fields := map[string]any{
		"go_version": goVer,
		"gomaxprocs": gomaxprocs,
		"num_cpu":    numCPU,
	}
	summaryParts := []string{
		fmt.Sprintf("go=%s", goVer),
		fmt.Sprintf("GOMAXPROCS=%d/%d", gomaxprocs, numCPU),
	}

	for _, s := range samples {
		key := metricFieldKey(s.Name)
		val, display := formatSample(s)
		fields[key] = val
		if display != "" {
			summaryParts = append(summaryParts, fmt.Sprintf("%s=%s", shortMetric(s.Name), display))
		}
	}

	// Detect Green Tea GC status from build info / environment. Green Tea
	// is the Go 1.26 default but can be disabled via
	// GOEXPERIMENT=nogreenteagc at build time.
	gcEnabled, gcDetail := greenTeaStatus()
	fields["greentea_gc"] = gcEnabled
	fields["greentea_detail"] = gcDetail
	summaryParts = append(summaryParts, fmt.Sprintf("greentea=%v", gcEnabled))

	logger := logging.Get(logging.CategoryBoot)
	logger.Info("runtime metrics snapshot: %s", strings.Join(summaryParts, " "))
	logger.StructuredLog("info", "runtime_metrics_startup", fields)

	if !gcEnabled {
		logger.Warn("Green Tea GC appears disabled (%s); Go 1.26 default is Green Tea ON. "+
			"Check GOEXPERIMENT for 'nogreenteagc'.", gcDetail)
	}
}

// metricFieldKey converts a /sched/goroutines:goroutines metric path into
// a stable snake-case field key suitable for structured logs.
func metricFieldKey(name string) string {
	// Drop the unit suffix after ':' and slashes become underscores.
	if idx := strings.IndexByte(name, ':'); idx >= 0 {
		name = name[:idx]
	}
	name = strings.TrimPrefix(name, "/")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

// shortMetric returns a compact identifier for a metric path. Most paths
// reduce to their trailing segment, but generic leaves like "total" pull
// in the preceding segment so two distinct counters don't collide in the
// human-readable summary line.
func shortMetric(name string) string {
	if idx := strings.IndexByte(name, ':'); idx >= 0 {
		name = name[:idx]
	}
	parts := strings.Split(strings.TrimPrefix(name, "/"), "/")
	if len(parts) == 0 {
		return name
	}
	leaf := parts[len(parts)-1]
	generic := map[string]bool{"total": true, "goal": true, "live": true}
	if generic[leaf] && len(parts) >= 2 {
		return parts[len(parts)-2] + "_" + leaf
	}
	return leaf
}

// formatSample extracts a comparable value and a display string from a
// metrics.Sample. Unsupported kinds are reported as the literal string
// "n/a" so downstream parsers can still find the key.
func formatSample(s metrics.Sample) (value any, display string) {
	switch s.Value.Kind() {
	case metrics.KindUint64:
		v := s.Value.Uint64()
		return v, fmt.Sprintf("%d", v)
	case metrics.KindFloat64:
		v := s.Value.Float64()
		return v, fmt.Sprintf("%.6f", v)
	case metrics.KindFloat64Histogram:
		// Histograms aren't useful in a single boot log line; just record
		// the bucket count so downstream tooling can detect presence.
		h := s.Value.Float64Histogram()
		return len(h.Buckets), fmt.Sprintf("hist[%d]", len(h.Buckets))
	case metrics.KindBad:
		return nil, "n/a"
	default:
		return nil, "n/a"
	}
}

// greenTeaStatus reports whether Green Tea GC is enabled in this binary.
// It treats the presence of "nogreenteagc" in either the runtime
// GOEXPERIMENT env or the build-info GOEXPERIMENT setting as the disabled
// signal. Returns the boolean status and a short human-readable detail
// string for logging.
func greenTeaStatus() (enabled bool, detail string) {
	envExp := os.Getenv("GOEXPERIMENT")
	buildExp := ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "GOEXPERIMENT" {
				buildExp = s.Value
				break
			}
		}
	}

	combined := envExp
	if buildExp != "" {
		if combined != "" {
			combined += ","
		}
		combined += buildExp
	}

	// nogreenteagc disables Green Tea; +greenteagc forces it on for older
	// toolchains. Default in Go 1.26 is on.
	if strings.Contains(combined, "nogreenteagc") {
		return false, fmt.Sprintf("GOEXPERIMENT=%q (env=%q build=%q)", combined, envExp, buildExp)
	}
	if combined == "" {
		return true, "default (GOEXPERIMENT unset)"
	}
	return true, fmt.Sprintf("GOEXPERIMENT=%q (env=%q build=%q)", combined, envExp, buildExp)
}
