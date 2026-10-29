package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codenerd/internal/atomicfile"
)

// The transaction manager exists to give a multi-file edit all-or-nothing
// semantics. A truncating write cannot deliver that: an apply interrupted
// partway leaves the file half-written, and if the rollback's write is also
// interrupted the file is destroyed with neither version surviving -- the
// original is only in memory, and the rollback error is logged while execution
// continues.

func TestTransactionWritePreservesMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX mode bits; os.Chmod only toggles the read-only attribute")
	}
	if os.Geteuid() == 0 {
		// Root can write anything, but the mode bits themselves are still
		// readable, so only the executable-bit assertion below matters here.
		t.Log("running as root; mode bits are still asserted")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "build.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := atomicfile.WriteFilePreservingMode(script, []byte("#!/bin/sh\necho new\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// os.WriteFile applies its perm only when it CREATES the file, so editing
	// an executable script left the mode alone. atomicfile always creates a new
	// inode and chmods it, so a constant 0644 here would silently strip +x off
	// every script this manager edits.
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("mode = %o after an edit, want 0755 — the executable bit was stripped", got)
	}
	data, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "echo new") {
		t.Errorf("contents = %q, want the replacement", data)
	}
}

func TestTransactionWriteDefaultsModeForANewFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX mode bits")
	}
	dir := t.TempDir()
	created := filepath.Join(dir, "new.go")

	if err := atomicfile.WriteFilePreservingMode(created, []byte("package x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(created)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("mode = %o for a newly created file, want 0644", got)
	}
}

func TestTransactionWriteReplacesTheInodeRatherThanTruncating(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.go")

	original := []byte("package original\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// atomicfile.Open, not os.Open: this stands in for a reader holding the
	// file across the write, and on Windows a handle without FILE_SHARE_DELETE
	// blocks the replace outright. That is the contract this package exists to
	// provide, so the test states it rather than working around it.
	held, err := atomicfile.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = held.Close() }()

	// Deliberately larger than the original: a truncate-then-write interrupted
	// midway cannot fit back what it has already destroyed.
	replacement := []byte("package replacement\n// " + strings.Repeat("x", 8192) + "\n")
	if err := atomicfile.WriteFilePreservingMode(path, replacement, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if runtime.GOOS != "windows" && os.SameFile(before, after) {
		t.Error("the write went through the existing inode; an interrupted apply would leave the " +
			"user's source half-written, and an interrupted rollback would destroy it outright")
	}

	buf := make([]byte, 64)
	n, _ := held.Read(buf)
	if string(buf[:n]) != string(original) {
		t.Errorf("a reader holding the file saw %q, want the contents it opened", buf[:n])
	}
}

func TestTransactionWriteLeavesNoTempFilesBeside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.go")

	for i := 0; i < 5; i++ {
		if err := atomicfile.WriteFilePreservingMode(path, []byte("package x\n"), 0o644); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	// A temp file left in a source tree is worse than in a state directory: it
	// gets committed, or it gets compiled.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only the target file", names)
	}
}
