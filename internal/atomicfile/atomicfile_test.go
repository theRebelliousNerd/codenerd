package atomicfile

import (
	"encoding/json"
	"errors"
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

// TestWriteFile_WhenWritersRaceOnOnePath_ShouldNotInterleave pins two things.
// The unique temp name per call is what stops interleaving, and would do so
// with no lock at all. The per-path stripe lock is what makes every writer
// SUCCEED: without it, on Windows, sixteen concurrent ReplaceFileW calls on one
// path race for the destination and, once the machine is loaded, the losers'
// retry windows close before the winner's handles do — a correct write is
// reported as failed and its temp file outlives the report. So no writer may
// fail here, and nothing may be left in the directory.
func TestWriteFile_WhenWritersRaceOnOnePath_ShouldNotInterleave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload, _ := json.Marshal(map[string]any{
				"writer": i,
				"pad":    strings.Repeat("y", 2048+i),
			})
			if err := WriteFile(path, payload, 0o644); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("a racing writer failed a write that had nothing wrong with it: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// A MISSING file here is the worst outcome this package can produce and
		// the least likely to be noticed, because the next writer just creates
		// it again. It caught a real one on Windows: replaceExisting used to
		// pick between MoveFileEx and ReplaceFileW by stat'ing the destination,
		// which is stale the instant it returns, so racing writers ran both
		// mechanisms against one path at once -- and ReplaceFileW is
		// delete-then-rename inside, so one interleaving removed the file and
		// could not put anything back.
		t.Fatalf("no file at all after %d racing writers: %v", 16, err)
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

// The failure paths are the point of the package and were untested.
//
// Every property below is about what survives a failed write. WriteFile's
// happy path being correct is necessary but not sufficient: the four defects
// this package replaced all destroyed the previous good copy before the
// replacement was guaranteed, and every one of them worked fine when nothing
// went wrong.

func TestWriteFile_WhenTheRenameFails_ShouldKeepThePreviousContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")

	if err := os.WriteFile(path, []byte("the good copy"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A rename onto an existing directory fails, which exercises the branch
	// after the temp file is fully written and synced -- the last point at
	// which the old contents could still be destroyed.
	blocker := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blocker, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := WriteFile(blocker, []byte("replacement"), 0o644); err == nil {
		t.Fatal("WriteFile onto a directory succeeded")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after failure: %v", err)
	}
	if string(got) != "the good copy" {
		t.Errorf("previous contents = %q, want them untouched", got)
	}
}

func TestWriteFile_WhenTheRenameFails_ShouldLeaveNoTempFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blocker, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := WriteFile(blocker, []byte("replacement"), 0o644); err == nil {
		t.Fatal("WriteFile onto a directory succeeded")
	}

	// A temp file left behind after a failure accumulates silently, and in a
	// directory three subsystems share it also looks like real state.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temp file %q survived a failed write", e.Name())
		}
	}
}

func TestWriteFile_WhenTheDirectoryIsMissing_ShouldFailWithoutCreatingAnything(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "absent", "target")

	err := WriteFile(path, []byte("data"), 0o644)
	if err == nil {
		t.Fatal("WriteFile into a missing directory succeeded")
	}
	// The message has to name the path, because this failure surfaces from
	// deep inside subsystems that write several files.
	if !strings.Contains(err.Error(), "target") {
		t.Errorf("error does not name the target path: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("a file was created despite the failure")
	}
}

func TestWriteFile_WhenTheFileIsUnwritable_ShouldStillReplaceIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: mode bits do not restrict access")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "readonly")
	if err := os.WriteFile(path, []byte("old"), 0o400); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Replacing by rename does not need write permission on the file itself,
	// only on the directory. That is a property of the approach worth pinning:
	// a read-only config a subsystem must still be able to update would fail
	// under a truncating write and succeeds here.
	if err := WriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile over a read-only file: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("contents = %q, want new", got)
	}
}

func TestWriteFile_EmptyData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := os.WriteFile(path, []byte("previous"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Truncating to empty is a legitimate write, not a no-op. Treating it as
	// one would leave stale contents behind for a caller that meant to clear
	// the file.
	if err := WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("WriteFile(nil): %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("contents = %q, want empty", got)
	}
}

func TestReplace_MovesOntoAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	if err := os.WriteFile(src, []byte("streamed bytes"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("previous"), 0o644); err != nil {
		t.Fatalf("seed dst: %v", err)
	}

	if err := Replace(src, dst); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "streamed bytes" {
		t.Errorf("dst = %q, want the source contents", got)
	}
	// Replace is a move: leaving the source behind would have a caller that
	// streamed to a temp file quietly accumulating them.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("the source survived the replace")
	}
}

func TestReplace_CreatesMissingDestination(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty source"},
		{name: "binary source", data: []byte{0x00, 0xff, '\n', 0x80, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			src := filepath.Join(dir, "src")
			dst := filepath.Join(dir, "dst")
			if err := os.WriteFile(src, tt.data, 0o600); err != nil {
				t.Fatalf("seed source: %v", err)
			}

			if err := Replace(src, dst); err != nil {
				t.Fatalf("Replace onto missing destination: %v", err)
			}
			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read destination: %v", err)
			}
			if string(got) != string(tt.data) {
				t.Errorf("destination contents = %q, want %q", got, tt.data)
			}
			if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("stat moved source = %v, want os.ErrNotExist", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read directory: %v", err)
			}
			if len(entries) != 1 || entries[0].Name() != "dst" {
				t.Errorf("directory entries = %v, want only dst", entries)
			}
		})
	}
}

func TestWriteFile_RejectsInvalidPathsWithoutArtifacts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
	}{
		{name: "embedded NUL", path: "invalid\x00path"},
		{name: "non-directory ancestor", path: filepath.Join("blocker", "target")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			blocker := filepath.Join(dir, "blocker")
			if err := os.WriteFile(blocker, []byte("retained"), 0o600); err != nil {
				t.Fatalf("seed blocker: %v", err)
			}
			path := filepath.Join(dir, tt.path)
			_, statErr := os.Stat(path)
			var cause *os.PathError
			if !errors.As(statErr, &cause) {
				t.Fatalf("stat invalid path = %v, want *os.PathError", statErr)
			}

			err := WriteFile(path, []byte("replacement"), 0o644)
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("WriteFile error = %v, want wrapped *os.PathError", err)
			}
			if !errors.Is(err, cause.Err) {
				t.Errorf("WriteFile error = %v, want cause %v", err, cause.Err)
			}
			got, err := os.ReadFile(blocker)
			if err != nil {
				t.Fatalf("read blocker: %v", err)
			}
			if string(got) != "retained" {
				t.Errorf("blocker contents = %q, want retained", got)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read directory: %v", err)
			}
			if len(entries) != 1 || entries[0].Name() != "blocker" {
				t.Errorf("directory entries = %v, want only blocker", entries)
			}
		})
	}
}

func TestReplace_ReportsAMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := Replace(filepath.Join(dir, "absent"), filepath.Join(dir, "dst")); err == nil {
		t.Fatal("Replace of a missing source succeeded")
	}
}

func TestOpen_ReadsTheCurrentContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	f, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = f.Close() }()

	// A reader holding the file open must keep seeing the contents it opened,
	// even after a replacement lands. That is the guarantee the whole
	// rename-based approach buys, and it is what makes a torn read impossible
	// rather than merely unlikely.
	if err := WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}

	buf := make([]byte, 8)
	n, err := f.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "v1" {
		t.Errorf("an open reader saw %q after a replacement, want the contents it opened", got)
	}
}

func TestWriteFilePreservingMode_KeepsAnExecutableBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX mode bits")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "build.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// This is the whole reason the helper exists. os.WriteFile applies its perm
	// only when it CREATES the file, so every caller passing a constant 0644
	// has been preserving +x by accident of the API. WriteFile always creates a
	// new inode and always chmods, so the same constant would strip it -- a
	// regression that surfaces as "the build script stopped running", with
	// nothing pointing back at the write.
	if err := WriteFilePreservingMode(script, []byte("#!/bin/sh\necho new\n"), 0o644); err != nil {
		t.Fatalf("WriteFilePreservingMode: %v", err)
	}

	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("mode = %o, want 0755 — the executable bit was stripped", got)
	}
}

func TestWriteFilePreservingMode_UsesTheFallbackForANewFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX mode bits")
	}
	dir := t.TempDir()
	created := filepath.Join(dir, "new.go")

	if err := WriteFilePreservingMode(created, []byte("package x\n"), 0o600); err != nil {
		t.Fatalf("WriteFilePreservingMode: %v", err)
	}
	info, err := os.Stat(created)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o for a new file, want the fallback 0600", got)
	}
}

func TestWriteFilePreservingMode_IsStillAtomic(t *testing.T) {
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

	// Preserving the mode must not have cost the atomicity: the point of
	// routing the agent's file edits through here is that an interrupted write
	// leaves the previous file intact rather than a truncated one.
	if err := WriteFilePreservingMode(path, []byte("package replacement\n"+strings.Repeat("x", 4096)), 0o644); err != nil {
		t.Fatalf("WriteFilePreservingMode: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if runtime.GOOS != "windows" && os.SameFile(before, after) {
		t.Error("the write went through the existing inode")
	}
}

func TestWriteFilePreservingMode_RejectsStatErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
	}{
		{name: "embedded NUL", path: "invalid\x00path"},
		{name: "non-directory ancestor", path: filepath.Join("blocker", "target")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			blocker := filepath.Join(dir, "blocker")
			if err := os.WriteFile(blocker, []byte("retained"), 0o600); err != nil {
				t.Fatalf("seed blocker: %v", err)
			}
			path := filepath.Join(dir, tt.path)
			_, statErr := os.Stat(path)
			if statErr == nil {
				t.Fatal("fixture must fail stat")
			}
			var cause *os.PathError
			if !errors.As(statErr, &cause) {
				t.Fatalf("stat error = %T, want *os.PathError", statErr)
			}

			err := WriteFilePreservingMode(path, []byte("replacement"), 0o644)
			var got *os.PathError
			if !errors.As(err, &got) {
				t.Fatalf("WriteFilePreservingMode error = %v, want wrapped stat error", err)
			}
			if !errors.Is(err, cause.Err) {
				t.Errorf("WriteFilePreservingMode error = %v, want cause %v", err, cause.Err)
			}
			// Some platforms classify a non-directory ancestor as not existing.
			// Only other stat errors must stop the write before temp creation.
			if !errors.Is(statErr, os.ErrNotExist) && (got.Op != cause.Op || got.Path != path) {
				t.Errorf("WriteFilePreservingMode error = %v, want original stat failure %v", err, statErr)
			}
			contents, err := os.ReadFile(blocker)
			if err != nil {
				t.Fatalf("read blocker: %v", err)
			}
			if string(contents) != "retained" {
				t.Errorf("blocker contents = %q, want retained", contents)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read directory: %v", err)
			}
			if len(entries) != 1 || entries[0].Name() != "blocker" {
				t.Errorf("unexpected files after rejected write: %v", entries)
			}
		})
	}
}

func TestWriteFilePreservingMode_PreservesSymlinkOnStatFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "loop")
	if err := os.Symlink("loop", path); err != nil {
		if runtime.GOOS == "windows" && errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation requires privileges: %v", err)
		}
		t.Fatalf("create symlink loop: %v", err)
	}
	_, statErr := os.Stat(path)
	if statErr == nil || errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("symlink loop must fail stat without indicating absence: %v", statErr)
	}
	var cause *os.PathError
	if !errors.As(statErr, &cause) {
		t.Fatalf("stat error = %T, want *os.PathError", statErr)
	}

	err := WriteFilePreservingMode(path, []byte("replacement"), 0o600)
	if err == nil {
		t.Error("WriteFilePreservingMode succeeded despite failing to determine the existing mode")
	} else if !errors.Is(err, cause.Err) {
		t.Errorf("WriteFilePreservingMode error = %v, want wrapped stat cause %v", err, cause.Err)
	}
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("symlink was not preserved: %v", err)
	}
	if target != "loop" {
		t.Errorf("symlink target = %q, want loop", target)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "loop" {
		t.Errorf("unexpected files after rejected write: %v", entries)
	}
}

func TestWriteFile_WhenTheDirectoryIsReadOnly_ShouldFailLoudly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows directory permissions do not gate file creation this way")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory mode does not restrict access")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := os.WriteFile(path, []byte("old"), 0o666); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	// The documented cost of replacing by rename: this needs write permission
	// on the directory, where a truncating write needed it only on the file.
	// The point of the test is that it fails LOUDLY and names the path, rather
	// than falling back to a non-atomic write, which would defeat the purpose
	// exactly where durability was hardest to get.
	err := WriteFile(path, []byte("new"), 0o644)
	if err == nil {
		t.Fatal("WriteFile into a read-only directory succeeded")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error does not name the target path: %v", err)
	}

	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if string(got) != "old" {
		t.Errorf("contents = %q, want the previous copy untouched", got)
	}
}

func TestOpen_RejectsMissingAndNULPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{name: "missing file", path: filepath.Join(dir, "missing"), wantErr: os.ErrNotExist},
		{name: "embedded NUL", path: filepath.Join(dir, "invalid\x00path")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Open(tt.path)
			if f != nil {
				if closeErr := f.Close(); closeErr != nil {
					t.Errorf("close unexpected file: %v", closeErr)
				}
				t.Error("Open returned a file for an invalid path")
			}
			if err == nil {
				t.Fatal("Open succeeded for an invalid path")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Open error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

func TestReplace_FailurePreservesSourceAndDestination(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		missingSource bool
	}{
		{name: "missing source preserves existing destination", missingSource: true},
		{name: "nonempty destination directory preserves source"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "src")
			dst := filepath.Join(dir, "dst")
			preserved := dst
			if !tt.missingSource {
				if err := os.WriteFile(src, []byte("source"), 0o600); err != nil {
					t.Fatalf("seed source: %v", err)
				}
				if err := os.Mkdir(dst, 0o700); err != nil {
					t.Fatalf("create destination directory: %v", err)
				}
				preserved = filepath.Join(dst, "child")
			}
			if err := os.WriteFile(preserved, []byte("previous"), 0o600); err != nil {
				t.Fatalf("seed destination: %v", err)
			}

			err := Replace(src, dst)
			if err == nil {
				t.Fatal("Replace succeeded despite an invalid source or destination")
			}
			if tt.missingSource && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("Replace error = %v, want os.ErrNotExist", err)
			}
			got, err := os.ReadFile(preserved)
			if err != nil {
				t.Fatalf("read preserved destination: %v", err)
			}
			if string(got) != "previous" {
				t.Errorf("destination contents = %q, want previous", got)
			}
			if !tt.missingSource {
				got, err := os.ReadFile(src)
				if err != nil {
					t.Fatalf("read preserved source: %v", err)
				}
				if string(got) != "source" {
					t.Errorf("source contents = %q, want source", got)
				}
			}
		})
	}
}

func TestRemoveTemp_PreservesNonEmptyDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "child")
	if err := os.WriteFile(path, []byte("retained"), 0o600); err != nil {
		t.Fatalf("seed child: %v", err)
	}

	// A persistent removal error must exhaust the retry budget and return,
	// without recursively deleting anything to force cleanup to succeed.
	removeTemp(dir)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read retained child: %v", err)
	}
	if string(got) != "retained" {
		t.Errorf("child contents = %q, want retained", got)
	}
}
