package sqlpragmas

import (
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

// EnvMetrics names the environment variable that enables pragma-failure
// counting at process start. Any value strconv.ParseBool accepts as true
// ("1", "t", "true", "TRUE") turns it on.
const EnvMetrics = "NERD_SQL_PRAGMA_METRICS"

// Pragma failures are expected in normal operation — the modernc.org/sqlite
// driver rejects a subset of the pragmas mattn accepts — so they are logged at
// Debug and nothing else. That makes a real regression (a pragma that started
// failing on the *primary* driver, or a profile whose whole preset is being
// rejected) invisible: nobody runs with store Debug on.
//
// These counters close that gap without adding noise. They are off by default;
// an operator or a CI job that wants the signal turns them on via
// SetMetricsEnabled or EnvMetrics and reads PragmaFailureTotal /
// PragmaFailuresByProfile. When disabled, recordPragmaFailure is a single
// atomic load and returns.
var (
	metricsEnabled atomic.Bool

	metricsMu           sync.Mutex
	failuresTotal       uint64
	failuresByProfile   = map[string]uint64{}
	failuresByStatement = map[string]uint64{}
)

func init() {
	if raw, ok := os.LookupEnv(EnvMetrics); ok {
		if on, err := strconv.ParseBool(raw); err == nil {
			metricsEnabled.Store(on)
		}
	}
}

// SetMetricsEnabled turns pragma-failure counting on or off. Like
// SetHostClass, this is the inversion-of-control seam that lets an
// observability config switch the counters on without this leaf importing
// the observability package.
func SetMetricsEnabled(on bool) { metricsEnabled.Store(on) }

// MetricsEnabled reports whether pragma-failure counting is on.
func MetricsEnabled() bool { return metricsEnabled.Load() }

// PragmaFailureTotal returns the number of PRAGMA statements that have failed
// since counting was enabled. Zero when metrics are disabled.
func PragmaFailureTotal() uint64 {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	return failuresTotal
}

// PragmaFailuresByProfile returns a copy of the per-profile failure counts,
// keyed by PragmaProfile.String().
func PragmaFailuresByProfile() map[string]uint64 {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	out := make(map[string]uint64, len(failuresByProfile))
	for k, v := range failuresByProfile {
		out[k] = v
	}
	return out
}

// PragmaFailuresByStatement returns a copy of the per-PRAGMA failure counts.
// This is the view that identifies a driver reject set: the statements a
// pure-Go build refuses show up here with a count per open.
func PragmaFailuresByStatement() map[string]uint64 {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	out := make(map[string]uint64, len(failuresByStatement))
	for k, v := range failuresByStatement {
		out[k] = v
	}
	return out
}

// FailingPragmas returns the distinct PRAGMA statements that have failed,
// sorted, for a one-line operator report.
func FailingPragmas() []string {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	out := make([]string, 0, len(failuresByStatement))
	for k := range failuresByStatement {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ResetPragmaMetrics zeroes the counters. Intended for tests and for an
// operator taking a fresh reading.
func ResetPragmaMetrics() {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	failuresTotal = 0
	failuresByProfile = map[string]uint64{}
	failuresByStatement = map[string]uint64{}
}

// recordPragmaFailure is called from every path that swallows a PRAGMA error.
func recordPragmaFailure(profile PragmaProfile, statement string) {
	if !metricsEnabled.Load() {
		return
	}
	metricsMu.Lock()
	defer metricsMu.Unlock()
	failuresTotal++
	failuresByProfile[profile.String()]++
	failuresByStatement[statement]++
}
