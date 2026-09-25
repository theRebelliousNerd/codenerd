package observability

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Every dump is up to the ring size, and /flightrec makes dumping a thing an
// operator does at will. The traces directory keeps the newest
// maxFlightTraces and drops the rest, oldest first.
func TestDumpFlightRecord_WhenTracesAccumulate_ShouldKeepOnlyTheNewest(t *testing.T) {
	resetFlightRecorder(t)
	t.Cleanup(func() { _ = StopFlightRecorder() })
	if err := StartFlightRecorder(2<<20, 100*time.Millisecond); err != nil {
		t.Fatalf("StartFlightRecorder: %v", err)
	}

	ws := t.TempDir()
	traces := filepath.Join(ws, ".nerd", "traces")
	if err := os.MkdirAll(traces, 0o755); err != nil {
		t.Fatal(err)
	}
	// Older than anything this run can write: year 2000 timestamps.
	for i := 0; i < maxFlightTraces+2; i++ {
		name := filepath.Join(traces, fmt.Sprintf("flight_20000101T0000%02dZ_001.trace", i))
		if err := os.WriteFile(name, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	unrelated := filepath.Join(traces, "notes.txt")
	if err := os.WriteFile(unrelated, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	path, err := DumpFlightRecord(ws)
	if err != nil {
		t.Fatalf("DumpFlightRecord: %v", err)
	}

	left, _ := filepath.Glob(filepath.Join(traces, "flight_*.trace"))
	if len(left) != maxFlightTraces {
		t.Fatalf("%d traces left, want %d: %v", len(left), maxFlightTraces, left)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the dump just written was pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(traces, "flight_20000101T000000Z_001.trace")); !os.IsNotExist(err) {
		t.Error("the oldest trace survived pruning")
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("pruning removed a file that is not a flight trace: %v", err)
	}
}
