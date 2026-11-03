package logging

import (
	"errors"
	"github.com/google/go-cmp/cmp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotation_WhenSegmentExceedsMaxSize_ShouldStartNewSegment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run_kernel.log")
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	configMu.Lock()
	config.MaxLogFileMB = 0 // exercised through the sink fields below
	configMu.Unlock()

	sink, err := openRotatingFile(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sink.Close()
	sink.maxBytes = 64
	sink.keep = 2

	for i := 0; i < 10; i++ {
		if _, err := sink.WriteString(strings.Repeat("x", 40) + "\n"); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat live segment: %v", err)
	}
	if fi.Size() > 64 {
		t.Errorf("live segment is %d bytes, past the 64 byte budget", fi.Size())
	}

	archived := rotatedSegments(path)
	if len(archived) == 0 {
		t.Fatal("expected archived segments after rotation")
	}
	if len(archived) > sink.keep {
		t.Errorf("kept %d archived segments, want at most %d", len(archived), sink.keep)
	}
	for _, seg := range archived {
		// Archived segments must stay .log and keep the run prefix, or the
		// startup retention sweep (which only looks at *.log) would never
		// expire them.
		if !strings.HasSuffix(seg, ".log") {
			t.Errorf("archived segment %q lost the .log suffix", seg)
		}
		if !strings.HasPrefix(filepath.Base(seg), "run_kernel.") {
			t.Errorf("archived segment %q lost its base name", seg)
		}
	}
}

func TestRotation_WhenSegmentIsOlderThanMaxAge_ShouldRotate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run_api.log")

	sink, err := openRotatingFile(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sink.Close()
	sink.maxBytes = 0 // size rotation off; age alone must trigger
	sink.maxAge = time.Millisecond
	sink.keep = 1

	if _, err := sink.WriteString("first\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := sink.WriteString("second\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	if len(rotatedSegments(path)) != 1 {
		t.Fatalf("expected one archived segment, got %v", rotatedSegments(path))
	}
	live, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	if strings.Contains(string(live), "first") {
		t.Error("old content stayed in the live segment after age rotation")
	}
	if !strings.Contains(string(live), "second") {
		t.Error("new content did not land in the fresh segment")
	}
}

func TestRotation_WhenSegmentIsEmpty_ShouldNotRotate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run_boot.log")

	sink, err := openRotatingFile(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sink.Close()
	sink.maxBytes = 1
	sink.maxAge = time.Nanosecond

	if _, err := sink.WriteString("only line\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := rotatedSegments(path); len(got) != 0 {
		t.Errorf("rotating an empty segment churns names for nothing: %v", got)
	}
}

func TestRotationPolicy_WhenConfigured_ShouldHonourOverridesAndOptOut(t *testing.T) {
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	configMu.Lock()
	config.MaxLogFileMB = 0
	config.MaxLogFileMinutes = 0
	config.MaxRotatedFiles = 0
	configMu.Unlock()
	if bytes, age, keep := rotationPolicy(); bytes != defaultMaxLogFileBytes || age != 0 || keep != defaultMaxRotatedFiles {
		t.Errorf("zero config should mean defaults, got (%d, %v, %d)", bytes, age, keep)
	}

	configMu.Lock()
	config.MaxLogFileMB = 2
	config.MaxLogFileMinutes = 15
	config.MaxRotatedFiles = 5
	configMu.Unlock()
	if bytes, age, keep := rotationPolicy(); bytes != 2<<20 || age != 15*time.Minute || keep != 5 {
		t.Errorf("explicit config not honoured, got (%d, %v, %d)", bytes, age, keep)
	}

	configMu.Lock()
	config.MaxLogFileMB = -1
	config.MaxRotatedFiles = -1
	configMu.Unlock()
	if bytes, _, keep := rotationPolicy(); bytes != 0 || keep != 0 {
		t.Errorf("negative config should disable, got (%d, %d)", bytes, keep)
	}
}

func TestRotation_WhenCategoryLoggerExceedsBudget_ShouldRotateOnDisk(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "max_log_file_mb": 1, "max_rotated_files": 2`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	logger := Get(CategoryKernel)
	if logger.sink == nil {
		t.Fatal("expected a file-backed logger")
	}
	// Shrink the live budget rather than writing a megabyte of test data.
	logger.sink.mu.Lock()
	logger.sink.maxBytes = 1024
	logger.sink.mu.Unlock()

	for i := 0; i < 60; i++ {
		logger.Info("%s", strings.Repeat("y", 100))
	}
	CloseAll()

	if got := rotatedSegments(filepath.Join(ws, ".nerd", "logs", filepath.Base(logger.sink.Path()))); len(got) == 0 {
		t.Error("a category logger past its budget did not rotate")
	}
}

func TestRotation_ReopenedSegmentPreservesRecordsAtSizeBoundary(t *testing.T) {
	for _, tt := range []struct {
		name        string
		initial     string
		incoming    string
		maxBytes    int64
		wantArchive bool
	}{
		{name: "below budget", initial: "seed\n", incoming: "next\n", maxBytes: 11},
		{name: "exact budget", initial: "seed\n", incoming: "next\n", maxBytes: 10},
		{name: "incoming exceeds budget", initial: "seed\n", incoming: "next\n", maxBytes: 9, wantArchive: true},
		{name: "existing segment exceeds budget", initial: "seed\n", incoming: "next\n", maxBytes: 4, wantArchive: true},
		{name: "oversized first record stays whole", incoming: "next\n", maxBytes: 1},
		{name: "size rotation disabled", initial: "seed\n", incoming: "next\n", maxBytes: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "run_kernel.log")
			if err := os.WriteFile(path, []byte(tt.initial), 0o600); err != nil {
				t.Fatalf("seed existing segment: %v", err)
			}
			sink, err := openRotatingFile(path)
			if err != nil {
				t.Fatalf("reopen segment: %v", err)
			}
			t.Cleanup(func() {
				if err := sink.Close(); err != nil {
					t.Errorf("close segment: %v", err)
				}
			})
			sink.maxBytes = tt.maxBytes
			sink.maxAge = 0
			sink.keep = 2

			n, err := sink.WriteString(tt.incoming)
			if err != nil || n != len(tt.incoming) {
				t.Fatalf("WriteString() = (%d, %v), want (%d, nil)", n, err, len(tt.incoming))
			}
			if err := sink.Close(); err != nil {
				t.Fatalf("close before reading: %v", err)
			}

			wantLive := tt.initial + tt.incoming
			wantArchives := 0
			if tt.wantArchive {
				wantLive = tt.incoming
				wantArchives = 1
			}
			live, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read live segment: %v", err)
			}
			if diff := cmp.Diff(wantLive, string(live)); diff != "" {
				t.Errorf("live segment mismatch (-want +got):\n%s", diff)
			}
			archives := rotatedSegments(path)
			if len(archives) != wantArchives {
				t.Fatalf("archive count = %d, want %d", len(archives), wantArchives)
			}
			if tt.wantArchive {
				archived, err := os.ReadFile(archives[0])
				if err != nil {
					t.Fatalf("read archived segment: %v", err)
				}
				if diff := cmp.Diff(tt.initial, string(archived)); diff != "" {
					t.Errorf("archived segment mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestRotation_ClosedSinkRejectsWritesWithoutChangingRecords(t *testing.T) {
	for _, tt := range []struct {
		name  string
		write func(*rotatingFile) (int, error)
	}{
		{name: "bytes", write: func(sink *rotatingFile) (int, error) { return sink.Write([]byte("late\n")) }},
		{name: "string", write: func(sink *rotatingFile) (int, error) { return sink.WriteString("late\n") }},
		{name: "empty bytes", write: func(sink *rotatingFile) (int, error) { return sink.Write(nil) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "run_kernel.log")
			sink, err := openRotatingFile(path)
			if err != nil {
				t.Fatalf("open segment: %v", err)
			}
			t.Cleanup(func() {
				if err := sink.Close(); err != nil {
					t.Errorf("close segment: %v", err)
				}
			})
			sink.maxBytes = 0
			sink.maxAge = 0
			const record = "before close\n"
			if n, err := sink.WriteString(record); err != nil || n != len(record) {
				t.Fatalf("seed WriteString() = (%d, %v), want (%d, nil)", n, err, len(record))
			}
			if err := sink.Close(); err != nil {
				t.Fatalf("close segment: %v", err)
			}
			if err := sink.Close(); err != nil {
				t.Fatalf("repeated Close() = %v, want nil", err)
			}
			if n, err := tt.write(sink); n != 0 || !errors.Is(err, os.ErrClosed) {
				t.Errorf("write after Close() = (%d, %v), want (0, os.ErrClosed)", n, err)
			}
			live, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read closed segment: %v", err)
			}
			if diff := cmp.Diff(record, string(live)); diff != "" {
				t.Errorf("closed segment mismatch (-want +got):\n%s", diff)
			}
			if archives := rotatedSegments(path); len(archives) != 0 {
				t.Errorf("write after Close() created archives: %v", archives)
			}
		})
	}
}
