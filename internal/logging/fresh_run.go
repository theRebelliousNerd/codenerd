// Package logging — fresh-run ordinary log reset and run-prefix isolation.
//
// On each codeNERD process initialization, ordinary top-level log files
// under .nerd/logs/*.log are cleared so that no prior run's data is
// reused or appended — including multiple runs on the same calendar day.
// Browser flight/evidence traces, non-log files, and nested directories
// are preserved. All mutations are workspace-contained, symlink-safe, and
// best-effort robust when a prior process still holds a file open.
//
// Filenames for the current run are prefixed with a process-unique,
// lexically sortable run prefix computed once per Initialize call
// (UTC timestamp with nanosecond precision plus PID, monotonic counter
// and random suffix). Using a fresh prefix guarantees that a locked or
// still-writing prior process can never contaminate the current run's
// category, problems, audit, and llm_io logs.
package logging

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	runPrefix    string
	runPrefixMu  sync.RWMutex
	runPrefixSeq uint64
)

// generateRunPrefix returns a process-unique, lexically sortable prefix.
// Format: 20060102_150405.000000000_<pid>_<seq>_<rand> where the timestamp
// is UTC with nanosecond precision (fixed width, sortable), pid is a
// zero-padded fixed-width process ID (os.Getpid) for cross-process
// uniqueness without relying solely on crypto/rand, seq is a zero-padded
// monotonic counter guaranteeing ordering and uniqueness even within the same
// nanosecond, and rand is 3 random bytes hex-encoded for additional
// cross-process uniqueness. The leading timestamp ensures lexical order equals
// chronological order; the counter ensures two rapid Initialize calls in the
// same process yield different prefixes. Including pid as a fixed-width
// numeric field preserves same-process lexical ordering (pid is constant
// within a process).
func generateRunPrefix() string {
	ts := time.Now().UTC().Format("20060102_150405.000000000")
	seq := atomic.AddUint64(&runPrefixSeq, 1)
	pid := os.Getpid()
	var b [3]byte
	_, _ = rand.Read(b[:])
	rnd := fmt.Sprintf("%06x", int(b[0])<<16|int(b[1])<<8|int(b[2]))
	return fmt.Sprintf("%s_%06d_%06d_%s", ts, pid, seq%1000000, rnd)
}

// currentRunPrefix returns the run prefix computed for this process run.
// Empty if Initialize has not yet been called; callers should fall back to a
// date string in that case.
func currentRunPrefix() string {
	runPrefixMu.RLock()
	defer runPrefixMu.RUnlock()
	return runPrefix
}

// isSymlink reports whether path exists and is a symlink (Lstat, no follow).
func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

// logsDirSymlinkRejected reports whether logsDir or its immediate parent
// (.nerd) is a symlink. If either is a symlink the directory must be
// rejected and never followed.
func logsDirSymlinkRejected(logsDir string) bool {
	if strings.TrimSpace(logsDir) == "" {
		return false
	}
	clean := filepath.Clean(logsDir)
	if isSymlink(clean) {
		return true
	}
	parent := filepath.Dir(clean)
	if isSymlink(parent) {
		return true
	}
	return false
}

// clearOrdinaryLogs removes/truncates previous top-level *.log files
// under logsDir. It is intentionally best-effort: any single entry that
// cannot be cleared (e.g., locked on Windows) is warned to stderr and
// skipped so that process startup is never blocked by stale logs.
//
// Invariants:
//   - Only direct children of logsDir with suffix ".log" are considered.
//   - Directories (including nested log archives) are preserved.
//   - Non-log files are preserved.
//   - Symlinks at the top level are preserved and never followed.
//   - Symlinked logs directory or symlinked parent (.nerd) is rejected entirely.
//   - Paths are verified to remain inside logsDir (workspace-contained).
func clearOrdinaryLogs(logsDir string) {
	if strings.TrimSpace(logsDir) == "" {
		return
	}
	cleanLogsDir := filepath.Clean(logsDir)
	// Reject symlinked logs directory or symlinked parent before any mutation.
	if logsDirSymlinkRejected(cleanLogsDir) {
		fmt.Fprintf(os.Stderr, "[logging] fresh-run: refusing symlinked logs directory %s (skipping cleanup)\n", cleanLogsDir)
		return
	}
	entries, err := os.ReadDir(cleanLogsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		fmt.Fprintf(os.Stderr, "[logging] fresh-run: could not list logs directory %s: %v\n", cleanLogsDir, err)
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		// Only ordinary log files.
		if !strings.HasSuffix(strings.ToLower(name), ".log") {
			continue
		}
		// Nested directories are preserved; only top-level is in scope.
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(cleanLogsDir, name)

		// Workspace containment: the joined path must stay inside logsDir.
		// This guards against malicious or surprising names like "../x.log"
		// (ReadDir itself shouldn't produce those, but we enforce anyway).
		cleanFull := filepath.Clean(fullPath)
		rel, relErr := filepath.Rel(cleanLogsDir, cleanFull)
		if relErr != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
			fmt.Fprintf(os.Stderr, "[logging] fresh-run: skipping out-of-scope entry %q (rel=%q)\n", fullPath, rel)
			continue
		}
		// Symlink-safe: never follow or remove a symlink, and never
		// operate on a file reachable only via a symlinked parent — Lstat
		// on the leaf catches leaf symlinks; containment above plus the
		// leaf check covers the requirement without expensive walk-time
		// parent resolution on every boot.
		info, lerr := os.Lstat(cleanFull)
		if lerr != nil {
			if !os.IsNotExist(lerr) {
				fmt.Fprintf(os.Stderr, "[logging] fresh-run: lstat %s: %v\n", cleanFull, lerr)
			}
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Preserve symlink and whatever it points to (often outside
			// the workspace or to evidence). Do not follow.
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}

		// Robust removal/truncation: try Remove first so unlocked old
		// ordinary logs disappear; if Remove fails for a non-not-exist
		// error (e.g., sharing violation on Windows due to a locked file),
		// fall back to truncating in place with O_WRONLY|O_TRUNC as best
		// effort, preserving private permissions. Warn rather than fail
		// the boot.
		if err := truncateOrRemove(cleanFull); err != nil {
			fmt.Fprintf(os.Stderr, "[logging] fresh-run: could not clear %s: %v (continuing)\n", cleanFull, err)
		}
	}
}

// truncateOrRemove tries to leave path empty without failing when the file
// is still open by another process. It first tries os.Remove so unlocked
// old ordinary logs disappear; if removal fails for a non-not-exist error,
// it falls back to opening with O_WRONLY|O_TRUNC for locked-file best
// effort, preserving private permissions. Combined error context is returned
// only when both strategies fail.
func truncateOrRemove(path string) error {
	// First try Remove — for unlocked files this deletes the entry
	// entirely (desired). On POSIX Remove of an open file succeeds
	// (unlinks, open fd keeps inode); on Windows a locked file will
	// fail with sharing violation and we fall back to truncation.
	if err := os.Remove(path); err == nil {
		return nil
	} else if os.IsNotExist(err) {
		// Path disappeared between ReadDir and Remove — nothing to do.
		return nil
	} else {
		// Remove failed for a reason other than not-exist. Try truncation
		// in place as best effort for a locked file — this is more
		// cooperative with an open handle on both POSIX (truncate succeeds,
		// readers see EOF) and Windows (often succeeds when Remove would get
		// ERROR_SHARING_VIOLATION).
		f, ferr := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
		if ferr == nil {
			_ = f.Close()
			// Ensure permissions remain private after truncation.
			_ = os.Chmod(path, 0o600)
			return nil
		}
		if os.IsNotExist(ferr) {
			return nil
		}
		// Both strategies failed; return combined context.
		return fmt.Errorf("remove: %v; truncate: %v", err, ferr)
	}
}
