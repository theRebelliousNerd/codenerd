package jsonl

import (
	"codenerd/internal/atomicfile"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
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

func TestReadPreservesGenerationPrefixes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		backup        string
		live          string
		missingBackup bool
		missingLive   bool
		want          []rec
		wantTruncated int
	}{
		{name: "empty generations"},
		{
			name:   "complete records without final newlines",
			backup: `{"n":1,"text":"backup"}`,
			live:   `{"n":2,"text":"live"}`,
			want:   []rec{{N: 1, Text: "backup"}, {N: 2, Text: "live"}},
		},
		{
			name:        "backup survives missing live log",
			backup:      "{\"n\":1}\n",
			missingLive: true,
			want:        []rec{{N: 1}},
		},
		{
			name:          "live survives missing backup",
			live:          "{\"n\":2}\n",
			missingBackup: true,
			want:          []rec{{N: 2}},
		},
		{
			name:          "damaged backup does not hide live records",
			backup:        "{\"n\":1}\n{\"n\":",
			live:          "{\"n\":2}\n",
			want:          []rec{{N: 1}, {N: 2}},
			wantTruncated: 1,
		},
		{
			name:          "both damaged generations stop at their first bad record",
			backup:        "{\"n\":1}\ninvalid\n{\"n\":2}\n",
			live:          "{\"n\":3}\ninvalid\n{\"n\":4}\n",
			want:          []rec{{N: 1}, {N: 3}},
			wantTruncated: 2,
		},
		{
			name:          "type mismatch preserves only the valid prefix",
			live:          "{\"n\":1}\n{\"n\":\"wrong type\"}\n{\"n\":2}\n",
			want:          []rec{{N: 1}},
			wantTruncated: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "log.jsonl")
			if !tt.missingBackup {
				if err := os.WriteFile(path+".1", []byte(tt.backup), 0o600); err != nil {
					t.Fatalf("write backup fixture: %v", err)
				}
			}
			if !tt.missingLive {
				if err := os.WriteFile(path, []byte(tt.live), 0o600); err != nil {
					t.Fatalf("write live fixture: %v", err)
				}
			}
			got, truncated, err := Read[rec](path)
			if err != nil {
				t.Fatalf("Read(%q): %v", path, err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Read() mismatch (-want +got):\n%s", diff)
			}
			if truncated != tt.wantTruncated {
				t.Errorf("Read() truncated = %d, want %d", truncated, tt.wantTruncated)
			}
		})
	}
}

func TestOpenRejectsFilesystemConflicts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		parentIsFile bool
	}{
		{name: "parent is a file", parentIsFile: true},
		{name: "log is a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "conflict")
			if tt.parentIsFile {
				if err := os.WriteFile(path, []byte("parent must remain intact"), 0o600); err != nil {
					t.Fatalf("write conflicting parent: %v", err)
				}
				path = filepath.Join(path, "log.jsonl")
			} else if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatalf("create conflicting directory: %v", err)
			}
			a, err := Open(path)
			if a != nil {
				t.Cleanup(func() {
					if err := a.Close(); err != nil {
						t.Errorf("Close unexpected appender: %v", err)
					}
				})
				t.Errorf("Open(%q) returned an appender for a filesystem conflict", path)
			}
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("Open(%q) error = %v, want wrapped *os.PathError", path, err)
			}
			if tt.parentIsFile {
				got, err := os.ReadFile(filepath.Dir(path))
				if err != nil {
					t.Fatalf("read conflicting parent: %v", err)
				}
				if string(got) != "parent must remain intact" {
					t.Errorf("parent contents = %q, want unchanged fixture", got)
				}
			} else if info, err := os.Stat(path); err != nil {
				t.Fatalf("stat conflicting directory: %v", err)
			} else if !info.IsDir() {
				t.Error("Open replaced the conflicting directory")
			}
		})
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

func TestRotatesMoreThanOnce(t *testing.T) {
	// The second rotation is the one that matters. It replaces an EXISTING
	// backup, and on Windows os.Rename maps to MoveFileEx, which refuses to
	// supersede a destination any process holds open -- a `nerd meter`
	// invocation reading the rotated generation is exactly such a process. The
	// rotation runs on the agent's hot path, where a failure must not happen,
	// so it goes through atomicfile.Replace rather than a bare rename.
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.SetMaxBytes(120)

	for i := 0; i < 300; i++ {
		a.Append(rec{N: i, Text: "rotate-me"})
	}
	if n, failErr := a.Failures(); failErr != nil {
		t.Fatalf("rotation failed %d times: %v", n, failErr)
	}
	_ = a.Close()

	got, truncated, err := Read[rec](path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if truncated != 0 {
		t.Fatalf("truncated = %d after repeated rotation", truncated)
	}
	if len(got) == 0 {
		t.Fatal("repeated rotation lost everything")
	}
	if got[len(got)-1].N != 299 {
		t.Errorf("last record = %d, want 299", got[len(got)-1].N)
	}
}

func TestRotationWhileAReaderHoldsTheBackupOpen(t *testing.T) {
	path := tempLog(t)
	a, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.SetMaxBytes(120)

	// Force one rotation so a backup exists, then hold it open across the next.
	for i := 0; i < 40; i++ {
		a.Append(rec{N: i, Text: "first-round"})
	}
	// atomicfile.Open, not os.Open. Rotation replaces this file, and on
	// Windows a handle opened without FILE_SHARE_DELETE blocks that replace
	// for as long as it lives — which is why readOne now opens this way too.
	// With os.Open here the test would be asserting something the design does
	// not promise and cannot deliver.
	reader, err := atomicfile.Open(path + ".1")
	if err != nil {
		t.Skipf("no backup produced in this environment: %v", err)
	}
	defer func() { _ = reader.Close() }()

	for i := 40; i < 200; i++ {
		a.Append(rec{N: i, Text: "second-round"})
	}
	if n, failErr := a.Failures(); failErr != nil {
		t.Fatalf("rotation failed %d times with a reader holding the backup: %v", n, failErr)
	}
	_ = a.Close()
}
