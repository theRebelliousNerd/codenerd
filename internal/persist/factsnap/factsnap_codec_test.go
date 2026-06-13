package factsnap

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

// TestWriteCodec_Auto verifies CodecAuto falls back to gzip and produces a file
// with the canonical .sc.gz extension that round-trips.
func TestWriteCodec_Auto(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap") // no extension; should gain .sc.gz
	facts := []types.Fact{
		{Predicate: "user_intent", Args: []any{"build"}},
		{Predicate: "focus", Args: []any{"main.go"}},
	}
	if err := WriteCodec(path, facts, CodecAuto); err != nil {
		t.Fatalf("WriteCodec(Auto): %v", err)
	}

	gzPath := path + ExtGzip
	if _, err := os.Stat(gzPath); err != nil {
		t.Fatalf("CodecAuto should produce a %s file: %v", ExtGzip, err)
	}

	got, err := Read(gzPath)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != len(facts) {
		t.Errorf("round-tripped %d facts, want %d", len(got), len(facts))
	}
}

// TestWriteCodec_UnknownCodec exercises the default branch: an out-of-range
// codec value must error and leave no snapshot behind.
func TestWriteCodec_UnknownCodec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.sc.gz")
	err := WriteCodec(path, []types.Fact{{Predicate: "p", Args: []any{"a"}}}, Codec(999))
	if err == nil {
		t.Fatal("expected an error for an unknown codec")
	}
	// The temp file must be cleaned up (no partial snapshot left).
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("no snapshot file should remain after an unknown-codec failure")
	}
	if _, statErr := os.Stat(path + ".tmp"); statErr == nil {
		t.Error("temp file should be cleaned up after failure")
	}
}
