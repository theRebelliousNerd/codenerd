package observability

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime/trace"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// flightMu guards the package-level flight recorder. The Go runtime
// itself allows only one active FlightRecorder per process today, so the
// singleton design mirrors that constraint.
var (
	flightMu sync.Mutex
	flight   *trace.FlightRecorder
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
func StartFlightRecorder(sizeBytes int, period time.Duration) error {
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

	fr := trace.NewFlightRecorder(cfg)
	if err := fr.Start(); err != nil {
		return fmt.Errorf("flight recorder start: %w", err)
	}
	flight = fr

	logging.Get(logging.CategoryBoot).Info(
		"flight recorder started (max_bytes=%d min_age=%s)",
		sizeBytes, period,
	)
	return nil
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
	if flight == nil {
		return nil
	}
	flight.Stop()
	flight = nil
	return nil
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
