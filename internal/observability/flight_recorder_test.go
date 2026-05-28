package observability

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// resetFlightRecorder ensures a clean singleton between tests. The Go
// runtime only allows one active FlightRecorder per process, so tests
// MUST stop the recorder when they're done.
func resetFlightRecorder(t *testing.T) {
	t.Helper()
	if err := StopFlightRecorder(); err != nil {
		t.Fatalf("StopFlightRecorder: %v", err)
	}
}

func TestStartFlightRecorder_StartStop(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if FlightRecorderEnabled() {
		t.Fatal("recorder should not be enabled before Start")
	}
	if err := StartFlightRecorder(2<<20, 250*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}
	if !FlightRecorderEnabled() {
		t.Fatal("recorder should be enabled after Start")
	}

	// Second Start is a no-op (already started).
	if err := StartFlightRecorder(2<<20, 250*time.Millisecond); err != nil {
		t.Fatalf("second StartFlightRecorder: %v", err)
	}
}

func TestDumpFlightRecord_WritesNonEmptyFile(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if err := StartFlightRecorder(4<<20, 100*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}

	// Generate a small amount of trace activity so the ring has data.
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Cheap busywork + sched yields produce trace events.
			for j := 0; j < 1000; j++ {
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()

	tmp := t.TempDir()
	path, err := DumpFlightRecord(tmp)
	if err != nil {
		t.Fatalf("DumpFlightRecord: %v", err)
	}
	if !filepath.IsAbs(path) {
		// path produced by filepath.Join with absolute tmp must be absolute
		t.Fatalf("expected absolute path, got %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat dump file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("dump file %s is empty", path)
	}
	// Trace files have a 16-byte header even when nearly empty; require
	// at least that much.
	if info.Size() < 16 {
		t.Errorf("dump file %s is smaller than trace header (%d bytes)", path, info.Size())
	}

	// Confirm the trace landed in <nerdDir>/.nerd/traces/ as documented.
	want := filepath.Join(tmp, ".nerd", "traces")
	if got := filepath.Dir(path); got != want {
		t.Errorf("dump dir = %q, want %q", got, want)
	}
}

func TestDumpFlightRecord_WithoutStart(t *testing.T) {
	resetFlightRecorder(t)

	tmp := t.TempDir()
	_, err := DumpFlightRecord(tmp)
	if err == nil {
		t.Fatal("expected error when dumping without an active recorder")
	}
}

func TestStopFlightRecorder_Idempotent(t *testing.T) {
	resetFlightRecorder(t)
	// Calling stop with nothing active must not error.
	if err := StopFlightRecorder(); err != nil {
		t.Fatalf("StopFlightRecorder: %v", err)
	}
}

func TestDumpFlightRecord_EmptyNerdDir(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if err := StartFlightRecorder(1<<20, 100*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}
	if _, err := DumpFlightRecord(""); err == nil {
		t.Fatal("expected error for empty nerdDir")
	}
}
