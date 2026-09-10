package broker

import (
	"os"
	"path/filepath"
	"testing"
)

// The process meter's extra sink is a file, and the meter is a singleton, so
// nothing but the swap can own that handle.
//
// Every boot installed a FileSink over the previous one and the previous one
// stayed open for the life of the process. Invisible on Linux, fatal on
// Windows, where an open handle blocks deleting the file at all — sixteen
// internal/system tests failed on t.TempDir cleanup for exactly this, one per
// boot.
//
// A closed FileSink silently stops writing, which is the cross-platform
// observable: the replaced sink's file stops growing.
func TestSetExtraSinkClosesWhatItReplaces(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.jsonl")
	secondPath := filepath.Join(dir, "second.jsonl")

	first, err := NewFileSink(firstPath)
	if err != nil {
		t.Fatalf("NewFileSink(first): %v", err)
	}
	second, err := NewFileSink(secondPath)
	if err != nil {
		t.Fatalf("NewFileSink(second): %v", err)
	}
	t.Cleanup(func() { _ = SetExtraSink(nil) })

	if err := SetExtraSink(first); err != nil {
		t.Fatalf("SetExtraSink(first): %v", err)
	}
	first.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	before := sinkFileSize(t, firstPath)
	if before == 0 {
		t.Fatal("nothing reached the first sink, so this test cannot detect a leak")
	}

	if err := SetExtraSink(second); err != nil {
		t.Fatalf("SetExtraSink(second): %v", err)
	}

	first.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	if got := sinkFileSize(t, firstPath); got != before {
		t.Errorf("the replaced sink is still writable (%d bytes, was %d): its handle "+
			"was not closed, and on Windows that file can never be deleted", got, before)
	}
}

// Detaching with nil is how a clean shutdown gives the handle back, and it must
// leave the meter with a working sink rather than a hole where one was.
func TestSetExtraSinkNilDetachesAndKeepsTheMeterUsable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	if err := SetExtraSink(sink); err != nil {
		t.Fatalf("SetExtraSink: %v", err)
	}
	sink.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	before := sinkFileSize(t, path)

	if err := SetExtraSink(nil); err != nil {
		t.Fatalf("SetExtraSink(nil): %v", err)
	}
	sink.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	if got := sinkFileSize(t, path); got != before {
		t.Errorf("detaching did not close the sink: %d bytes, was %d", got, before)
	}

	// The meter must still record into its ring after the file sink is gone;
	// shutting the log off is not the same as shutting metering off.
	m := Default()
	m.mu.RLock()
	sinkAfter := m.sink
	m.mu.RUnlock()
	if sinkAfter == nil {
		t.Error("detaching the extra sink left the meter with no sink at all")
	}
}

func sinkFileSize(t *testing.T, path string) int64 {
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
