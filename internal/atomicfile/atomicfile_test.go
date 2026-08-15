package atomicfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// TestWriteFile_ShouldReplaceTheInodeRatherThanTruncate is the guard that
// actually catches a regression to os.WriteFile.
//
// Content-based assertions do NOT catch it: a truncating write leaves the same
// correct bytes on the happy path, so "the file parses afterwards" passes
// either way. That is how two durability suites in this repo (usage, factsnap)
// came to pass with their atomic write reverted to O_TRUNC.
//
// What distinguishes them is identity. A rename swaps in a NEW inode, so a
// handle opened before the write still sees the whole previous file; a
// truncating write mutates the inode under that handle. os.SameFile is the
// check, and reading through the old handle proves the previous copy survived
// intact — which is exactly what a torn write would destroy.
func TestWriteFile_ShouldReplaceTheInodeRatherThanTruncate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")

	original := []byte(`{"keep":"me","n":1}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}
	oldHandle, err := Open(path)
	if err != nil {
		t.Fatalf("open before: %v", err)
	}
	defer oldHandle.Close()

	// Deliberately larger than the original: a truncate-then-write that fails
	// midway cannot fit what it just destroyed.
	replacement := []byte(`{"keep":"me","n":2,"padding":"` + strings.Repeat("x", 4096) + `"}`)
	if err := WriteFile(path, replacement, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	// ReplaceFileW preserves file identity by design, so the inode proxy does
	// not hold on Windows while the guarantee still does.
	if runtime.GOOS != "windows" {
		if os.SameFile(before, after) {
			t.Error("wrote through the existing file; a partial write would have destroyed the only copy")
		}
	}

	buf := make([]byte, 8192)
	n, _ := oldHandle.Read(buf)
	if string(buf[:n]) != string(original) {
		t.Errorf("old handle did not still yield ORIGINAL bytes in full: got %d bytes want %d bytes", n, len(original))
	}
	var previous map[string]any
	if err := json.Unmarshal(buf[:n], &previous); err != nil {
		t.Errorf("previous contents were mutated mid-write: %v", err)
	}
	if previous["keep"] != "me" {
		t.Errorf("previous contents lost data during the write: %v", previous)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after: %v", err)
	}
	if string(data) != string(replacement) {
		t.Errorf("destination does not hold replacement bytes in full: got %d bytes want %d bytes", len(data), len(replacement))
	}
}

func TestWriteFile_ShouldLeaveNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")
	for i := 0; i < 5; i++ {
		if err := WriteFile(path, []byte(`{"n":1}`), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only the target file, got %v", names)
	}
}

// TestWriteFile_WhenWritersRaceOnOnePath_ShouldNotInterleave uses a UNIQUE temp
// name per call as its defence, so — unlike a mutex-serialised writer — this
// still holds with no lock at all.
func TestWriteFile_WhenWritersRaceOnOnePath_ShouldNotInterleave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload, _ := json.Marshal(map[string]any{
				"writer": i,
				"pad":    strings.Repeat("y", 2048+i),
			})
			_ = WriteFile(path, payload, 0o644)
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("racing writers produced an unparseable file (interleaved): %v", err)
	}
	if _, ok := got["writer"]; !ok {
		t.Errorf("result is not one writer's complete payload: %v", got)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp files left behind after racing writers: %d entries", len(entries))
	}
}

func TestWriteFile_ShouldApplyTheRequestedMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX mode bits; os.Chmod only toggles the read-only attribute")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")
	if err := WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// CreateTemp makes 0600; assert the caller's mode is honoured, not inherited.
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 0600", got)
	}
	if err := WriteFile(path, []byte("y"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	info, _ = os.Stat(path)
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("mode = %o, want 0644", got)
	}
}
