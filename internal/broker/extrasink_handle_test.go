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
func TestConfigureClosesTheSinkItReplaces(t *testing.T) {
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
	// DetachExtraSink closes whichever of these is still installed; the
	// other one has to be closed here or the test leaks the handle it is
	// about, and fails its own TempDir cleanup on Windows.
	t.Cleanup(func() {
		_, _ = DetachExtraSink(first)
		_, _ = DetachExtraSink(second)
		_ = first.Close()
		_ = second.Close()
	})

	Configure(MeterConfig{ExtraSink: first})
	first.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	before := sinkFileSize(t, firstPath)
	if before == 0 {
		t.Fatal("nothing reached the first sink, so this test cannot detect a leak")
	}

	Configure(MeterConfig{ExtraSink: second})

	first.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	if got := sinkFileSize(t, firstPath); got != before {
		t.Errorf("the replaced sink is still writable (%d bytes, was %d): its handle "+
			"was not closed, and on Windows that file can never be deleted", got, before)
	}
}

// Detaching is how a clean shutdown gives the handle back, and it must leave the
// meter with a working sink rather than a hole where one was: shutting the log
// off is not the same as shutting metering off.
func TestDetachExtraSinkKeepsTheMeterUsable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	Configure(MeterConfig{ExtraSink: sink})
	sink.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	before := sinkFileSize(t, path)

	if _, err := DetachExtraSink(sink); err != nil {
		t.Fatalf("DetachExtraSink: %v", err)
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

// Detaching must not close a sink somebody else installed.
//
// The meter is a process singleton and Cortex instances are cached per
// workspace and provider, so more than one can be live at once. An
// unconditional detach on shutdown meant closing one agent let it close the
// receipt log a DIFFERENT, still-running agent was writing to — and a closed
// FileSink drops records silently, so that agent would carry on with its
// metering switched off and nothing anywhere to say so.
func TestDetachExtraSinkOnlyClosesWhatYouInstalled(t *testing.T) {
	dir := t.TempDir()
	minePath := filepath.Join(dir, "mine.jsonl")
	theirsPath := filepath.Join(dir, "theirs.jsonl")

	mine, err := NewFileSink(minePath)
	if err != nil {
		t.Fatalf("NewFileSink(mine): %v", err)
	}
	theirs, err := NewFileSink(theirsPath)
	if err != nil {
		t.Fatalf("NewFileSink(theirs): %v", err)
	}
	t.Cleanup(func() {
		_, _ = DetachExtraSink(mine)
		_, _ = DetachExtraSink(theirs)
		_ = mine.Close()
		_ = theirs.Close()
	})

	// I install mine; a later boot replaces it with theirs.
	Configure(MeterConfig{ExtraSink: mine})
	Configure(MeterConfig{ExtraSink: theirs})

	// Now I shut down and try to detach. The installed sink is not mine.
	detached, err := DetachExtraSink(mine)
	if err != nil {
		t.Fatalf("DetachExtraSink: %v", err)
	}
	if detached {
		t.Error("detached a sink this caller did not install")
	}

	theirs.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	if sinkFileSize(t, theirsPath) == 0 {
		t.Error("the other agent's sink was closed by a shutdown that did not own it; " +
			"its metering is now off with nothing to say so")
	}
}

// And it must close what you DID install, or the handle leaks again.
func TestDetachExtraSinkClosesYourOwn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "own.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	t.Cleanup(func() { _, _ = DetachExtraSink(sink); _ = sink.Close() })

	Configure(MeterConfig{ExtraSink: sink})
	sink.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	before := sinkFileSize(t, path)

	detached, err := DetachExtraSink(sink)
	if err != nil {
		t.Fatalf("DetachExtraSink: %v", err)
	}
	if !detached {
		t.Fatal("did not detach the sink this caller installed")
	}
	sink.Record(Receipt{Purpose: "/test", Provider: "p", Model: "m"})
	if got := sinkFileSize(t, path); got != before {
		t.Errorf("detach did not close the sink: %d bytes, was %d", got, before)
	}
}
