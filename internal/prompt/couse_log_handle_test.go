package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/jsonl"
)

// Replacing the log must close the one it replaces.
//
// CoUse() is a process-wide singleton and every boot installs a log into it, so
// a setter that dropped the old value on the floor leaked one open file per
// boot. On Linux that is invisible — an unlinked-but-open file just goes away.
// On Windows the file cannot be deleted while a handle is open, which is how it
// finally surfaced: sixteen internal/system tests failing in CI on t.TempDir
// cleanup with "the process cannot access the file because it is being used by
// another process", none of them about anything those tests were testing.
//
// The cross-platform observable is that a closed Appender silently stops
// writing, so the old log's file stays exactly as it was.
func TestSetLogClosesTheLogItReplaces(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.jsonl")
	newPath := filepath.Join(dir, "new.jsonl")

	oldLog, err := jsonl.Open(oldPath)
	if err != nil {
		t.Fatalf("open old: %v", err)
	}
	newLog, err := jsonl.Open(newPath)
	if err != nil {
		t.Fatalf("open new: %v", err)
	}
	// The replacement is never swapped out, so nothing else closes it. A test
	// about leaked handles that leaks one fails its own TempDir cleanup on
	// Windows, which is how this was caught.
	t.Cleanup(func() { _ = newLog.Close() })

	rec := NewCoUseRecorder()
	if err := rec.SetLog(oldLog); err != nil {
		t.Fatalf("SetLog(old): %v", err)
	}
	oldLog.Append(map[string]string{"before": "swap"})

	sizeBeforeSwap := fileSize(t, oldPath)
	if sizeBeforeSwap == 0 {
		t.Fatal("nothing was written to the first log, so this test cannot detect a leak")
	}

	if err := rec.SetLog(newLog); err != nil {
		t.Fatalf("SetLog(new): %v", err)
	}

	// If the swap left the handle open, this lands and the file grows.
	oldLog.Append(map[string]string{"after": "swap"})
	if got := fileSize(t, oldPath); got != sizeBeforeSwap {
		t.Errorf("the replaced log is still writable (%d bytes, was %d): "+
			"its handle was not closed, and on Windows that file can never be deleted",
			got, sizeBeforeSwap)
	}
}

// Detaching with nil is how a clean shutdown gives the handle back, and it is
// what Cortex.Close does.
func TestSetLogNilClosesTheCurrentLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	log, err := jsonl.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	rec := NewCoUseRecorder()
	if err := rec.SetLog(log); err != nil {
		t.Fatalf("SetLog: %v", err)
	}
	log.Append(map[string]string{"a": "b"})
	before := fileSize(t, path)

	if err := rec.SetLog(nil); err != nil {
		t.Fatalf("SetLog(nil): %v", err)
	}
	log.Append(map[string]string{"c": "d"})
	if got := fileSize(t, path); got != before {
		t.Errorf("detaching did not close the log: %d bytes, was %d", got, before)
	}
}

// Installing the same log twice must not close it out from under the recorder.
func TestSetLogIsIdempotentForTheSameLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "same.jsonl")
	log, err := jsonl.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = log.Close() })
	rec := NewCoUseRecorder()
	if err := rec.SetLog(log); err != nil {
		t.Fatalf("SetLog: %v", err)
	}
	if err := rec.SetLog(log); err != nil {
		t.Fatalf("SetLog again: %v", err)
	}
	log.Append(map[string]string{"still": "open"})
	if fileSize(t, path) == 0 {
		t.Error("re-installing the same log closed it; the recorder now writes nowhere")
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}
