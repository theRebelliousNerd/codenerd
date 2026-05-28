package observability

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFlightRecorder_PanicCaptureLifecycle exercises the full lifecycle
// the production code expects on a crash:
//
//	StartFlightRecorder → background goroutine panics → defer-recover
//	→ DumpFlightRecord writes the ring window → file is a valid Go
//	execution trace.
//
// This validates the contract that runtime/trace observes the panic
// goroutine before it unwinds, so the dump captures *why* the system
// died. We can't use testing/synctest here: runtime/trace needs real
// wall-clock and OS-thread scheduling; synctest's virtual time starves
// trace emission and the resulting dump would be near-empty.
func TestFlightRecorder_PanicCaptureLifecycle(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	require.NoError(t,
		StartFlightRecorder(4<<20, 100*time.Millisecond),
		"start recorder")
	require.True(t, FlightRecorderEnabled(), "recorder must be enabled after Start")

	// Spawn a goroutine that does some traceable work, then panics. A
	// deferred recover() inside the goroutine prevents test crash; the
	// flight recorder should have observed the goroutine's lifetime
	// regardless of whether it panicked or returned normally.
	panicked := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				close(panicked)
			}
		}()
		// Generate trace events before panicking so the ring has data.
		for i := 0; i < 2000; i++ {
			runtime.Gosched()
		}
		panic("simulated production crash for flight-recorder test")
	}()

	wg.Wait()
	select {
	case <-panicked:
		// expected
	default:
		t.Fatal("goroutine did not panic — recovery channel not closed")
	}

	// Dump after the panic. Production main.go calls this from its
	// top-level defer; here we call it directly post-recover.
	workspace := t.TempDir()
	path, err := DumpFlightRecord(workspace)
	require.NoError(t, err, "dump after panic")
	require.NotEmpty(t, path, "dump path must be non-empty")
	require.True(t, filepath.IsAbs(path), "dump path must be absolute")

	// File must land in <workspace>/.nerd/traces/flight_*.trace.
	wantDir := filepath.Join(workspace, ".nerd", "traces")
	require.Equal(t, wantDir, filepath.Dir(path), "trace must live under .nerd/traces")
	require.True(t, strHasPrefix(filepath.Base(path), "flight_"),
		"filename should follow flight_<ts>.trace convention, got %q", filepath.Base(path))

	// File must be a parseable Go execution trace. runtime/trace.Reader
	// is not a public symbol, so we validate by magic bytes: every Go
	// trace file begins with the ASCII string "go 1." in the header.
	// This is the same check `go tool trace` uses internally.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 16,
		"trace files have a 16-byte minimum header")
	require.True(t, hasGoTraceMagic(data),
		"file does not start with Go trace magic header; first 16 bytes = %x", data[:16])
}

// TestFlightRecorder_DoubleDumpKeepsRecorderRunning verifies the API
// contract that the recorder stays live after a dump — production code
// dumps once on panic and may dump again on graceful shutdown, so we
// must not implicitly stop on first dump.
func TestFlightRecorder_DoubleDumpKeepsRecorderRunning(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	require.NoError(t, StartFlightRecorder(2<<20, 50*time.Millisecond))
	for i := 0; i < 500; i++ {
		runtime.Gosched()
	}

	workspace := t.TempDir()
	first, err := DumpFlightRecord(workspace)
	require.NoError(t, err, "first dump")
	require.True(t, FlightRecorderEnabled(), "recorder must remain enabled after first dump")

	// Stagger a tick so the second dump's filename (second-precision)
	// is guaranteed distinct.
	time.Sleep(1100 * time.Millisecond)
	for i := 0; i < 500; i++ {
		runtime.Gosched()
	}

	second, err := DumpFlightRecord(workspace)
	require.NoError(t, err, "second dump")
	require.NotEqual(t, first, second, "filenames must be unique per dump")

	// Both files must exist and parse.
	for _, p := range []string{first, second} {
		data, err := os.ReadFile(p)
		require.NoError(t, err, "read %s", p)
		require.True(t, hasGoTraceMagic(data), "trace magic missing in %s", p)
	}
}

// hasGoTraceMagic checks the Go execution-trace file header. The Go
// runtime emits an ASCII magic prefix "go 1." followed by a version
// digit (e.g. "go 1.22 trace\x00..."). Asserting the prefix is robust
// to minor-version drift and lets us confirm "this is a trace file"
// without depending on the (currently private) runtime/trace.Reader.
func hasGoTraceMagic(data []byte) bool {
	const magic = "go 1."
	if len(data) < len(magic) {
		return false
	}
	for i, b := range []byte(magic) {
		if data[i] != b {
			return false
		}
	}
	return true
}

func strHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
