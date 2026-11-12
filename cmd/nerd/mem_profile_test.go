package main

import (
	"os"
	"path/filepath"
	"testing"
)

// --memprofile writes a gzip-compressed pprof heap profile when the command
// ends, and nothing when the flag is unset.
func TestWriteMemProfile(t *testing.T) {
	saved := memProfilePath
	t.Cleanup(func() { memProfilePath = saved })

	memProfilePath = ""
	if err := writeMemProfile(); err != nil {
		t.Fatalf("unset --memprofile: %v", err)
	}

	memProfilePath = filepath.Join(t.TempDir(), "heap.prof")
	if err := writeMemProfile(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(memProfilePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		t.Fatalf("heap profile is not a gzip pprof profile (%d bytes)", len(data))
	}

	memProfilePath = filepath.Join(t.TempDir(), "missing", "heap.prof")
	if err := writeMemProfile(); err == nil {
		t.Fatal("an unwritable --memprofile path was not reported")
	}
}
