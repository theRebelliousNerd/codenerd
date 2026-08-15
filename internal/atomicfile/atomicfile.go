// Package atomicfile writes a file so that a reader sees either the previous
// contents or the new ones, never a partial write.
//
// This exists because the same defect was found four separate times in this
// codebase, in four unrelated packages: a truncating os.WriteFile over
// .nerd/usage.json, an os.Remove before a rename retry in the campaign journal,
// a shared <path>.tmp in the fact snapshotter, and a truncating write in the
// init preferences merge. Each destroyed the previous good copy before the
// replacement was guaranteed, and each was found only after it had shipped.
//
// The init case showed why the tail matters more than it looks. That write is a
// read-modify-write over a file three subsystems share, and the merge treats a
// corrupt existing file as a hard error rather than clobbering it — a correct
// choice on its own. Together they mean a single torn write does not just lose
// data, it permanently wedges `nerd init` for that workspace: every later run
// refuses to touch the file it cannot parse. Atomicity is what keeps
// fail-closed from becoming fail-forever.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile atomically replaces path with data.
//
// The temp file is created in the destination directory (a rename across
// filesystems is not atomic and would degrade to copy-then-delete) with a
// unique name (a shared "<path>.tmp" lets two concurrent writers interleave
// into one file and rename the mixture over a good copy). It is fsynced before
// the rename, because a rename can otherwise land while the contents are still
// only in the page cache — which is the crash window the temp file exists to
// close.
//
// The containing directory is fsynced afterwards so the rename itself is
// durable, not just the bytes.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file beside %s: %w", path, err)
	}
	tmpName := tmp.Name()

	// Any failure from here on must not leave the temp file behind.
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	// CreateTemp makes 0600; the caller's mode is what the file should have.
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file onto %s: %w", path, err)
	}

	// Best effort: a filesystem that cannot fsync a directory (or a platform
	// that does not support it) has still completed the rename.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
