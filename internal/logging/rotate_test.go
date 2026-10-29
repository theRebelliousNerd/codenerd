package logging

import (
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
