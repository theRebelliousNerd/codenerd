package observability

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"runtime/trace"
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

func TestStartFlightRecorder_WatchdogKeepsStartingSampler(t *testing.T) {
	resetFlightRecorder(t)
	original := flightMemSample
	t.Cleanup(func() {
		if err := StopFlightRecorder(); err != nil {
			t.Errorf("StopFlightRecorder: %v", err)
		}
		flightMemSample = original
	})

	// An unbuffered, nonblocking notification cannot retain samples from
	// before the source changes. At most one old-source call can be in flight.
	sampled := make(chan struct{})
	flightMemSample = func() uint64 {
		select {
		case sampled <- struct{}{}:
		default:
		}
		return 100
	}
	if err := startFlightRecorder(2<<20, 30*time.Millisecond, 8, time.Millisecond); err != nil {
		t.Fatalf("startFlightRecorder: %v", err)
	}

	// Changing the source for future starts must not affect this generation.
	// Reading this replacement would exceed the guard and stop the recorder.
	flightMemSample = func() uint64 { return 200 }
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for range 2 {
		select {
		case <-sampled:
		case <-timer.C:
			t.Fatal("watchdog stopped sampling the source captured at start")
		}
	}
	if !FlightRecorderEnabled() {
		t.Fatal("replacing the source for future starts stopped the active recorder")
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

func TestStartFlightRecorder_StartError(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	// Start a conflicting flight recorder to make our start fail
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{})
	if err := fr.Start(); err != nil {
		t.Fatalf("fr.Start: %v", err)
	}
	defer fr.Stop()

	err := StartFlightRecorder(0, 0)
	if err == nil {
		t.Fatal("expected error starting flight recorder when another is already running")
	}
}

func TestDumpFlightRecord_MkdirError(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if err := StartFlightRecorder(1<<20, 100*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}

	tmp := t.TempDir()
	// Create a file named .nerd where the directory should be
	nerdFile := filepath.Join(tmp, ".nerd")
	if err := os.WriteFile(nerdFile, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := DumpFlightRecord(tmp)
	if err == nil {
		t.Fatal("expected error when .nerd is a file instead of a directory")
	}
}

func TestDumpFlightRecord_WriteError(t *testing.T) {
	// Windows ACLs do not honor Unix-style chmod 0500 for write denial reliably.
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based write denial is not reliable on Windows")
	}
	// Root bypasses the permission bits entirely, so a 0500 directory is still
	// writable and DumpFlightRecord correctly returns no error. The assertion
	// is about the error path, not about privilege, so skip rather than assert
	// something untrue of the environment. Containers and CI images routinely
	// run as UID 0.
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod-based write denial does not apply")
	}

	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if err := StartFlightRecorder(1<<20, 100*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}

	tmp := t.TempDir()
	tracesDir := filepath.Join(tmp, ".nerd", "traces")
	if err := os.MkdirAll(tracesDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Ensure it's not writable.
	if err := os.Chmod(tracesDir, 0500); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	_, err := DumpFlightRecord(tmp)
	if err == nil {
		t.Fatal("expected error when traces directory is not writable")
	}

	// Restore permissions so t.TempDir() cleanup doesn't fail
	_ = os.Chmod(tracesDir, 0755)
}

func TestDumpFlightRecord_NotEnabled(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if err := StartFlightRecorder(1<<20, 100*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}

	// Reach into the global state and disable it (for coverage of the defensive check)
	flightMu.Lock()
	fr := flight
	flightMu.Unlock()
	fr.Stop() // this makes fr.Enabled() false but leaves flight != nil

	tmp := t.TempDir()
	_, err := DumpFlightRecord(tmp)
	if err == nil {
		t.Fatal("expected error when dumping after recorder is stopped manually")
	}
}

func TestStopFlightRecorder_Active(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if err := StartFlightRecorder(2<<20, 250*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}
	if !FlightRecorderEnabled() {
		t.Fatal("recorder should be enabled after Start")
	}

	if err := StopFlightRecorder(); err != nil {
		t.Fatalf("StopFlightRecorder: %v", err)
	}

	if FlightRecorderEnabled() {
		t.Fatal("recorder should not be enabled after Stop")
	}
}

func TestStartFlightRecorder_ZeroSelectsDefaults(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	// 0/0 must select runtime defaults, not error.
	if err := StartFlightRecorder(0, 0); err != nil {
		t.Fatalf("StartFlightRecorder(0,0): %v", err)
	}
	if !FlightRecorderEnabled() {
		t.Fatal("recorder should be enabled after StartFlightRecorder(0,0)")
	}
}

func TestStartFlightRecorder_DoubleStartPreservesRecorder(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if err := StartFlightRecorder(2<<20, 250*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}
	flightMu.Lock()
	first := flight
	flightMu.Unlock()
	if first == nil {
		t.Fatal("flight must be non-nil after Start")
	}
	if err := StartFlightRecorder(4<<20, 100*time.Millisecond); err != nil {
		t.Fatalf("second StartFlightRecorder: %v", err)
	}
	flightMu.Lock()
	second := flight
	flightMu.Unlock()
	if second != first {
		t.Fatal("second Start must preserve the existing recorder, not replace it")
	}
	if !FlightRecorderEnabled() {
		t.Fatal("recorder should stay enabled across double Start")
	}
}

func TestStartFlightRecorder_NoWatchdogWhenIntervalNonPositive(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	// Interval <= 0 starts the ring without a watchdog generation.
	if err := startFlightRecorder(1<<20, 50*time.Millisecond, 1<<20, 0); err != nil {
		t.Fatalf("startFlightRecorder with interval 0: %v", err)
	}
	if !FlightRecorderEnabled() {
		t.Fatal("recorder should be enabled even without a watchdog")
	}
	flightMu.Lock()
	stopCh := flightWatchdogStop
	flightMu.Unlock()
	if stopCh != nil {
		t.Fatalf("interval<=0 must not create a watchdog channel, got %v", stopCh)
	}
}

func TestDumpFlightRecord_SameSecondDumpsAreDistinct(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	if err := StartFlightRecorder(2<<20, 50*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}
	ws := t.TempDir()
	// No sleep: the per-process sequence must disambiguate dumps that land
	// in the same wall-clock second. This pins flightDumpSeq without the
	// 1.1s wall-clock dependency of the lifecycle double-dump test.
	first, err := DumpFlightRecord(ws)
	if err != nil {
		t.Fatalf("first DumpFlightRecord: %v", err)
	}
	second, err := DumpFlightRecord(ws)
	if err != nil {
		t.Fatalf("second DumpFlightRecord: %v", err)
	}
	if first == second {
		t.Fatalf("same-second dumps must be distinct via sequence, both=%q", first)
	}
}

func TestMemClassOther_Supported(t *testing.T) {
	t.Parallel()
	descs := metrics.All()
	for _, d := range descs {
		if d.Name == memClassOther {
			return
		}
	}
	t.Fatalf("watchdog metric path %q not present in runtime/metrics.All() — Go %s", memClassOther, runtime.Version())
}

func TestOtherMemoryBytes_DoesNotPanic(t *testing.T) {
	t.Parallel()
	// otherMemoryBytes backs the watchdog baseline; it must never panic even
	// on an unexpected toolchain where the metric kind changes (returns 0).
	_ = otherMemoryBytes()
	_ = otherMemoryBytes()
}

func TestPruneFlightTraces_EmptyDirIsNoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No flight_*.trace files: must not error or panic.
	pruneFlightTraces(dir, maxFlightTraces)
	// Missing directory entirely: Glob finds nothing, must be a no-op.
	pruneFlightTraces(filepath.Join(dir, "does-not-exist"), maxFlightTraces)
}

func TestPruneFlightTraces_KeepLargerThanMatchesIsNoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"flight_20000101T000001Z_001.trace", "flight_20000101T000002Z_001.trace"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pruneFlightTraces(dir, 10)
	left, _ := filepath.Glob(filepath.Join(dir, "flight_*.trace"))
	if len(left) != 2 {
		t.Fatalf("prune with keep > matches must keep everything, left=%v", left)
	}
}

func TestPruneFlightTraces_KeepZeroRemovesAllFlightTraces(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"flight_20000101T000001Z_001.trace", "flight_20000101T000002Z_001.trace"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	unrelated := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	pruneFlightTraces(dir, 0)
	left, _ := filepath.Glob(filepath.Join(dir, "flight_*.trace"))
	if len(left) != 0 {
		t.Fatalf("prune with keep=0 must remove all flight traces, left=%v", left)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("pruning must not remove non-flight files: %v", err)
	}
}

func TestPruneFlightTraces_KeepsNewestLexicalOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	names := []string{
		"flight_20000101T000001Z_001.trace",
		"flight_20000101T000002Z_001.trace",
		"flight_20000101T000003Z_001.trace",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pruneFlightTraces(dir, 2)
	if _, err := os.Stat(filepath.Join(dir, names[0])); !os.IsNotExist(err) {
		t.Errorf("oldest trace should have been pruned: %v", err)
	}
	for _, name := range names[1:] {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("newest trace %s should survive pruning: %v", name, err)
		}
	}
}

func TestFlightWatchdog_StaleGenerationDoesNotStopNewRecorder(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })

	// Real generation with an unreachable guard so only our stale
	// watchdog's tripping sample is in play.
	if err := startFlightRecorder(2<<20, 50*time.Millisecond, 1<<50, 10*time.Millisecond); err != nil {
		t.Fatalf("startFlightRecorder: %v", err)
	}
	// A watchdog from an older generation (different stop channel) must not
	// stop the current recorder even when its sample exceeds its cap. This
	// pins the flightWatchdogStop == stopCh identity guard across rapid
	// stop/start cycles.
	staleStop := make(chan struct{})
	flightMemWatchdog(staleStop, 0, 0, 10*time.Millisecond, func() uint64 { return 1 << 20 })
	if !FlightRecorderEnabled() {
		t.Fatal("stale watchdog generation stopped the current recorder; identity guard failed")
	}
}
