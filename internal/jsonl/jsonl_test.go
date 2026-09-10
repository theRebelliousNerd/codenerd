package jsonl

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type rec struct {
	N    int    `json:"n"`
	Text string `json:"text"`
}

func tempLog(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "nested", "log.jsonl")
}

func TestAppendAndReadRoundTrip(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 0; i < 5; i++ {
		a.Append(rec{N: i, Text: fmt.Sprintf("line %d", i)})
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, truncated, err := Read[rec](path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if truncated != 0 {
		t.Fatalf("truncated = %d, want 0", truncated)
	}
	if len(got) != 5 {
		t.Fatalf("records = %d, want 5", len(got))
	}
	for i, r := range got {
		if r.N != i {
			t.Fatalf("record %d = %+v, want N=%d — order must be preserved", i, r, i)
		}
	}
}

func TestOpenCreatesParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "log.jsonl")
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = a.Close() }()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log not created: %v", err)
	}
}

func TestReopenAppendsRatherThanTruncates(t *testing.T) {
	path := tempLog(t)

	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.Append(rec{N: 1})
	_ = a.Close()

	// A second process starting in the same workspace must not erase the first
	// one's evidence. This is the whole reason the log is append-only.
	b, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	b.Append(rec{N: 2})
	_ = b.Close()

	got, _, err := Read[rec](path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 2 || got[0].N != 1 || got[1].N != 2 {
		t.Fatalf("records = %+v, want [1 2]", got)
	}
}

func TestRotationKeepsOneGenerationAndReadsBoth(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.SetMaxBytes(200) // a handful of records

	const total = 60
	for i := 0; i < total; i++ {
		a.Append(rec{N: i, Text: "xxxxxxxxxx"})
	}
	_ = a.Close()

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("no rotated generation: %v", err)
	}

	got, truncated, err := Read[rec](path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if truncated != 0 {
		t.Fatalf("truncated = %d, want 0", truncated)
	}
	if len(got) == 0 {
		t.Fatal("rotation lost everything")
	}
	// Only one generation is kept, so the oldest records are gone by design.
	// What must hold is that whatever survived is in write order: a reader that
	// walked the live file before the rotated one would report a history that
	// jumps backwards in time and segment epochs at cuts that never happened.
	for i := 1; i < len(got); i++ {
		if got[i].N <= got[i-1].N {
			t.Fatalf("records out of order at %d: %d after %d", i, got[i].N, got[i-1].N)
		}
	}
	if got[len(got)-1].N != total-1 {
		t.Fatalf("last record = %d, want %d — the newest record must survive rotation", got[len(got)-1].N, total-1)
	}
}

func TestRotationDropsOnlyTheOldestGeneration(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.SetMaxBytes(120)

	for i := 0; i < 200; i++ {
		a.Append(rec{N: i, Text: "yyyyyyyyyy"})
	}
	_ = a.Close()

	// Exactly two files: the live one and one backup. A third would mean the
	// bound is not a bound.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("log files = %v, want exactly the live log and one backup", names)
	}
}

func TestSetMaxBytesRejectsUnbounded(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = a.Close() }()

	// Zero or negative must restore the default, not disable rotation: an
	// unbounded log in a long agent session is the failure the cap prevents.
	a.SetMaxBytes(0)
	if got := a.maxBytes; got != DefaultMaxBytes {
		t.Fatalf("maxBytes after 0 = %d, want the default %d", got, DefaultMaxBytes)
	}
	a.SetMaxBytes(-1)
	if got := a.maxBytes; got != DefaultMaxBytes {
		t.Fatalf("maxBytes after -1 = %d, want the default %d", got, DefaultMaxBytes)
	}
}

func TestTruncatedTailIsToleratedAndReported(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 0; i < 3; i++ {
		a.Append(rec{N: i})
	}
	_ = a.Close()

	// Simulate a crash mid-write.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("reopen for corruption: %v", err)
	}
	if _, err := f.WriteString(`{"n": 99, "te`); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	_ = f.Close()

	got, truncated, err := Read[rec](path)
	if err != nil {
		t.Fatalf("Read returned an error for a truncated tail: %v", err)
	}
	// Everything before the cut must survive. Refusing to report anything
	// because the last line is short would throw away a whole sample to protect
	// a number that is already approximate.
	if len(got) != 3 {
		t.Fatalf("records = %d, want 3", len(got))
	}
	if truncated != 1 {
		t.Fatalf("truncated = %d, want 1 — a damaged log must say so", truncated)
	}
}

func TestReadMissingLogIsNotAnError(t *testing.T) {
	got, truncated, err := Read[rec](filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil {
		t.Fatalf("Read of a missing log: %v", err)
	}
	if len(got) != 0 || truncated != 0 {
		t.Fatalf("got %d records / %d truncated from a missing log", len(got), truncated)
	}
}

func TestAppendAfterCloseIsInert(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.Append(rec{N: 1})
	_ = a.Close()

	// Shutdown ordering is not something a hot-path writer can rely on, so a
	// late append must be a no-op rather than a panic on a closed file.
	a.Append(rec{N: 2})
	if err := a.Close(); err != nil {
		t.Fatalf("double Close: %v", err)
	}

	got, _, err := Read[rec](path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("records = %d, want 1", len(got))
	}
}

func TestUnmarshallableRecordIsCountedNotDropped(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = a.Close() }()

	a.Append(map[string]any{"bad": make(chan int)})

	n, failErr := a.Failures()
	if failErr == nil || n != 1 {
		t.Fatalf("Failures = (%d, %v), want a recorded marshal failure", n, failErr)
	}

	var typeErr *json.UnsupportedTypeError
	if !errors.As(failErr, &typeErr) {
		t.Fatalf("recorded error = %T (%v), want a json.UnsupportedTypeError", failErr, failErr)
	}

	got, _, err := Read[rec](path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("a record that could not marshal reached the log: %+v", got)
	}
}

func TestConcurrentAppendsAreLineAtomic(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	const workers, each = 8, 100
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				a.Append(rec{N: worker*each + i, Text: "concurrent"})
			}
		}(w)
	}
	wg.Wait()
	_ = a.Close()

	got, truncated, err := Read[rec](path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if truncated != 0 {
		t.Fatalf("truncated = %d — interleaved writes corrupted a line", truncated)
	}
	// Every record must be present and parseable. An interleaved partial write
	// would show up as a decode failure that ends the read early, which is why
	// the whole line is written in one Write call.
	if len(got) != workers*each {
		t.Fatalf("records = %d, want %d", len(got), workers*each)
	}
	seen := make(map[int]bool, len(got))
	for _, r := range got {
		if seen[r.N] {
			t.Fatalf("record %d written twice", r.N)
		}
		seen[r.N] = true
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("Open(\"\") succeeded")
	}
}

func TestNilAppenderFieldsAreSafe(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if a.Path() != path {
		t.Fatalf("Path() = %q, want %q", a.Path(), path)
	}
	_ = a.Close()
}
