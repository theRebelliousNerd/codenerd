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
//
// A later audit found six more, which makes ten and makes this systemic rather
// than a run of bad luck: the northstar vision JSON and its Mangle projection,
// the assault triage latest.json, the transaction manager's apply and rollback,
// autopoiesis agent specs and memory, and the seven paths by which the agent
// edits the user's own source. So the useful thing to record is not the tally
// but the rule that separates the ten from the writes correctly left alone.
//
// A write must be atomic when EITHER holds:
//
//  1. The file is read back and its reader treats corruption as a hard error.
//     Then a torn write is not a lost update, it is a permanent one: every
//     later read fails on the file it cannot parse. northstar.json,
//     agent.json and memory.json are all this shape.
//
//  2. Losing the previous contents is worse than losing the new ones. The
//     transaction manager is the sharp case — it exists to give a multi-file
//     edit all-or-nothing semantics, and it cannot deliver that on top of a
//     write that can leave a file half-formed. The agent's source edits are
//     the same argument at one file's scale.
//
// A write is correctly NOT atomic when the file is written once to a fresh path
// (a timestamped report, a per-run log) or when it is a cache the caller can
// re-derive. Converting those buys nothing and costs a temp file in a directory
// somebody is watching.
//
// Two costs come with the rename, and both are stated on WriteFile rather than
// left to be discovered: it needs write permission on the containing directory
// rather than on the file, and it always creates a new inode and therefore
// always applies the mode — which is why WriteFilePreservingMode exists.
//
// A third cost is Windows, and it is the one that nearly sank this package.
// POSIX lets a rename replace a file that other processes hold open; Windows
// does not, unless every one of those handles was opened with
// FILE_SHARE_DELETE. So atomicity here is a CONTRACT BETWEEN WRITER AND
// READER, not a property of the writer alone: the writer uses WriteFile, and
// anyone reading a file that gets written this way uses Open from this package.
// A reader that reaches for os.Open instead makes every atomic write to that
// path fail for as long as it lives.
//
// That half was built and then not connected, which cost six packages failing
// on the Windows runner while Linux stayed green — including internal/jsonl,
// whose rotation comment explains at length that it uses atomicfile.Replace so
// a reader cannot break rotation, immediately above a reader of its own logs
// calling os.Open. See replace_windows.go for the two further Windows
// behaviours that had to be handled: a destination carrying the read-only
// attribute, and transient sharing violations under contention.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// removeTemp deletes a temp file this package created, retrying briefly.
//
// A plain os.Remove is enough on POSIX and not quite enough on Windows, where
// a failed ReplaceFileW can still have the replacement file open for a moment
// afterwards and deleting an open file is refused outright. Sixteen writers
// racing on one path left temp files behind for exactly that reason — the
// write correctly reported its failure and the debris outlived it, in the
// directory the caller is watching, which is the one place this package
// promises not to leave anything.
//
// Best effort by design: it returns nothing, because every caller is already
// on a failure path and has a better error to report than this one.
func removeTemp(path string) {
	for attempt := 0; attempt < 5; attempt++ {
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(time.Duration(attempt+1) * time.Millisecond)
	}
}

// WriteFile atomically replaces path with data.
//
// One cost is worth stating rather than discovering. Replacing by rename needs
// write permission on the CONTAINING DIRECTORY, where a truncating write needed
// it only on the file. A file that is writable inside a directory that is not
// can be updated by os.WriteFile and cannot be updated by this. That
// combination is unusual -- it means someone made the directory read-only and
// left a file in it writable -- and the failure is loud and names the path,
// which is the right trade against silently leaving a half-written file behind.
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
		removeTemp(tmpName)
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
		removeTemp(tmpName)
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	// CreateTemp makes 0600; the caller's mode is what the file should have.
	if err := os.Chmod(tmpName, perm); err != nil {
		removeTemp(tmpName)
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := replaceExisting(tmpName, path); err != nil {
		removeTemp(tmpName)
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

// Replace atomically moves src onto dst, replacing dst if it exists.
//
// Exported for callers that must stream their own bytes (a digest tee, a
// compressor) and so cannot use WriteFile, but still need the Windows-correct
// replace. See replaceExisting for why os.Rename is not sufficient there.
func Replace(src, dst string) error {
	return replaceExisting(src, dst)
}

// WriteFilePreservingMode atomically replaces path, keeping whatever mode the
// file already has. A file that does not exist yet is created with fallback.
//
// This exists because swapping os.WriteFile for WriteFile is not a
// mode-neutral change, and the difference is easy to miss.
//
// os.WriteFile passes its perm argument to open(2), which applies it only when
// the call CREATES the file. Writing over an existing one leaves the mode
// alone, so every caller that passes a constant 0644 has been preserving the
// executable bit on scripts by accident of the API rather than by intent.
//
// WriteFile always creates a new inode and therefore always chmods it. Feeding
// it the same constant 0644 would silently strip +x off every script it
// touched -- a regression that shows up as "the build script stopped running"
// with nothing pointing back at the write.
func WriteFilePreservingMode(path string, data []byte, fallback os.FileMode) error {
	perm := fallback
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	return WriteFile(path, data, perm)
}
