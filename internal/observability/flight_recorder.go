package observability

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime/metrics"
	"runtime/trace"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// flightMu guards the package-level flight recorder. The Go runtime
// itself allows only one active FlightRecorder per process today, so the
// singleton design mirrors that constraint.
//
// flightWatchdogStop is the stop channel for the memory watchdog that
// belongs to the currently-active recorder generation. It is closed (and
// niled) by stopFlightLocked so the watchdog goroutine exits promptly and
// so a watchdog can prove it is stopping *its own* generation and not a
// later Start's. nil means no watchdog is running.
var (
	flightMu           sync.Mutex
	flight             *trace.FlightRecorder
	flightWatchdogStop chan struct{}
)

// memClassOther is the runtime/metrics path where the execution tracer's
// region memory (stack/string interning tables, generation backlog, and
// the ring buffer itself) is accounted. Watching its growth is how the
// recorder detects the runaway allocation that ends in the runtime's
// fatal `throw("traceRegion: out of memory")`.
const memClassOther = "/memory/classes/other:bytes"

// Watchdog tunables. These are package vars (not consts) so tests can
// drive the guard deterministically with a tiny cap / short interval.
//
//   - flightWatchdogInterval: how often the watchdog samples trace memory.
//     The failure mode is gradual, GB-scale growth, so a 2 s cadence
//     reacts with enormous margin while costing a single cheap
//     metrics.Read per tick (no stop-the-world).
//   - flightWatchdogGrowthCap: the floor on how much trace-attributed
//     memory may grow past its at-Start baseline before the recorder is
//     auto-stopped. The effective cap scales up with the ring size so a
//     deliberately large ring does not false-trip (see StartFlightRecorder).
var (
	flightWatchdogInterval         = 2 * time.Second
	flightWatchdogGrowthCap uint64 = 256 << 20 // 256 MiB
)

// StartFlightRecorder begins a process-wide ring-buffer trace using
// runtime/trace.FlightRecorder (graduated in Go 1.25).
//
//   - sizeBytes is an upper bound on the in-memory ring window in bytes
//     (recommend 64 MiB). 0 selects the runtime default (~10 MiB).
//   - period is the lower bound on the age of events retained in the
//     window (recommend 30 s). 0 selects the runtime default (~seconds).
//
// Calling StartFlightRecorder twice is a no-op on the second call and
// returns nil; the existing recorder is preserved. The recorder is safe
// to leave running for the lifetime of the process — the runtime stops
// it automatically at exit, so callers do not have to invoke
// DumpFlightRecord or any Stop() to remain correct.
//
// A background memory watchdog is started alongside the recorder. The
// execution tracer keeps interning tables and generation buffers for the
// life of the trace; under heavy trace load (a long campaign spawning
// many subprocesses / goroutines with deep unique stacks) that memory can
// grow without bound until the runtime aborts the whole process with the
// unrecoverable fatal error `throw("traceRegion: out of memory")`. The
// watchdog samples the tracer's memory and stops the recorder before that
// point, degrading gracefully (tracing off) instead of crashing the host.
func StartFlightRecorder(sizeBytes int, period time.Duration) error {
	// Effective cap: the floor, but at least 4× the ring size so a large
	// configured ring (whose bytes also land in memClassOther) does not
	// immediately look like runaway growth.
	capBytes := flightWatchdogGrowthCap
	if r := uint64(sizeBytes) * 4; r > capBytes {
		capBytes = r
	}
	return startFlightRecorder(sizeBytes, period, capBytes, flightWatchdogInterval)
}

// startFlightRecorder is the internal entry point that makes the watchdog
// cap and cadence injectable so tests can exercise the guard
// deterministically without generating gigabytes of trace data. growthCap
// is honoured verbatim (a cap of 0 trips on any growth); the ring-size
// scaling is policy applied by the public StartFlightRecorder wrapper.
func startFlightRecorder(sizeBytes int, period time.Duration, growthCap uint64, interval time.Duration) error {
	flightMu.Lock()
	defer flightMu.Unlock()

	if flight != nil {
		// Already started; treat as success.
		return nil
	}

	cfg := trace.FlightRecorderConfig{}
	if sizeBytes > 0 {
		cfg.MaxBytes = uint64(sizeBytes)
	}
	if period > 0 {
		cfg.MinAge = period
	}

	// Baseline BEFORE Start so the delta the watchdog measures is
	// dominated by the tracer's own allocations rather than whatever the
	// process was already using.
	baseline := flightMemSample()

	fr := trace.NewFlightRecorder(cfg)
	if err := fr.Start(); err != nil {
		return fmt.Errorf("flight recorder start: %w", err)
	}
	flight = fr

	if interval > 0 {
		stopCh := make(chan struct{})
		flightWatchdogStop = stopCh
		go flightMemWatchdog(stopCh, baseline, growthCap, interval)
	}

	logging.Get(logging.CategoryBoot).Info(
		"flight recorder started (max_bytes=%d min_age=%s mem_guard=%d)",
		sizeBytes, period, growthCap,
	)
	return nil
}

// otherMemoryBytes reads the current execution-tracer/other memory class.
// Returns 0 if the runtime does not report the metric as a uint64 (which
// would only happen on an unexpected toolchain change); a 0 baseline is
// safe — it just makes the watchdog slightly more eager.
func otherMemoryBytes() uint64 {
	s := []metrics.Sample{{Name: memClassOther}}
	metrics.Read(s)
	if s[0].Value.Kind() == metrics.KindUint64 {
		return s[0].Value.Uint64()
	}
	return 0
}

// flightMemSample is the watchdog's memory source, indirected through a
// package var so tests can drive the trip decision deterministically.
//
// Whether real goroutine churn grows this metric past a given guard depends on
// the Go toolchain's trace accounting, so a test that churns and waits is
// asserting a property of the runtime, not of the watchdog. Substituting the
// sampler lets the trip logic be tested for what it actually promises: stop the
// recorder once observed growth exceeds the cap.
var flightMemSample = otherMemoryBytes

// flightMemWatchdog stops the recorder once trace-attributed memory grows
// more than capBytes past its baseline. It samples on a ticker and exits
// when (a) its generation's stop channel is closed by StopFlightRecorder,
// or (b) it trips the cap and stops the recorder itself. The
// flightWatchdogStop == stopCh identity check ensures a watchdog only ever
// stops the exact recorder generation it was launched for, even across
// rapid stop/start cycles.
func flightMemWatchdog(stopCh chan struct{}, baseline, capBytes uint64, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			cur := flightMemSample()
			var grew uint64
			if cur > baseline {
				grew = cur - baseline
			}
			if grew <= capBytes {
				continue
			}

			flightMu.Lock()
			if flightWatchdogStop == stopCh && flight != nil {
				logging.Get(logging.CategoryBoot).Warn(
					"flight recorder auto-stopped: trace memory grew %d bytes "+
						"(> %d guard); the execution tracer was about to exhaust "+
						"process memory. Disable it for heavy/long runs via "+
						"NERD_FLIGHTREC=0 or features.flight_recorder=false.",
					grew, capBytes,
				)
				stopFlightLocked()
			}
			flightMu.Unlock()
			return
		}
	}
}

// StopFlightRecorder ends the recording session if one is active. Safe
// to call even when no recorder was ever started; in that case it is a
// no-op. Returns nil on success or when there was nothing to stop.
//
// Most callers do not need to invoke this — the Go runtime will stop the
// recorder cleanly at process exit. It is exposed primarily for tests
// and for wiring into explicit shutdown hooks.
func StopFlightRecorder() error {
	flightMu.Lock()
	defer flightMu.Unlock()
	stopFlightLocked()
	return nil
}

// stopFlightLocked stops the active recorder and tears down its watchdog.
// Callers MUST hold flightMu. Safe to call when nothing is active (no-op),
// which is what makes StopFlightRecorder idempotent and prevents a
// double-close of the watchdog stop channel: the channel is only ever
// closed on the single transition from flight != nil to nil.
func stopFlightLocked() {
	if flight == nil {
		return
	}
	flight.Stop()
	flight = nil
	if flightWatchdogStop != nil {
		close(flightWatchdogStop)
		flightWatchdogStop = nil
	}
}

// FlightRecorderEnabled reports whether a flight recorder is currently
// active. Useful for diagnostics and for tests that want to assert state
// without poking at unexported globals.
func FlightRecorderEnabled() bool {
	flightMu.Lock()
	defer flightMu.Unlock()
	return flight != nil && flight.Enabled()
}

// DumpFlightRecord snapshots the active flight recorder's window into a
// timestamped file under <nerdDir>/.nerd/traces/. nerdDir should be the
// workspace root (NOT the .nerd directory itself); the function will
// create .nerd/traces/ underneath it as needed.
//
// Returns the absolute path of the written file, or an error if no
// recorder is active, if the directory cannot be created, or if the
// write fails. The recorder remains running after a successful dump.
func DumpFlightRecord(nerdDir string) (string, error) {
	flightMu.Lock()
	fr := flight
	flightMu.Unlock()

	if fr == nil {
		return "", fmt.Errorf("flight recorder not started")
	}
	if !fr.Enabled() {
		return "", fmt.Errorf("flight recorder not enabled")
	}

	if nerdDir == "" {
		return "", fmt.Errorf("nerdDir must be non-empty")
	}

	tracesDir := filepath.Join(nerdDir, ".nerd", "traces")
	if err := os.MkdirAll(tracesDir, 0o755); err != nil {
		return "", fmt.Errorf("create traces dir: %w", err)
	}

	filename := fmt.Sprintf("flight_%s.trace", time.Now().UTC().Format("20060102T150405Z"))
	path := filepath.Join(tracesDir, filename)

	// Buffer first so a failed disk write doesn't tear the recorder.
	var buf bytes.Buffer
	if _, err := fr.WriteTo(&buf); err != nil {
		return "", fmt.Errorf("flight recorder snapshot: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("write flight trace: %w", err)
	}

	logging.Get(logging.CategoryBoot).Info(
		"flight recorder dumped: path=%s bytes=%d", path, buf.Len(),
	)
	return path, nil
}
