// Package logging — fresh-run ordinary log reset and run-prefix isolation.
//
// On each codeNERD process initialization, ordinary top-level log files
// under .nerd/logs/*.log are trimmed to a retention window: the most
// recent DefaultLogRetentionRuns distinct run prefixes are kept and only
// logs belonging to older prefixes are cleared. This preserves recent
// history for transparency while preventing unbounded growth — including
// multiple runs on the same calendar day.
// Browser flight/evidence traces, non-log files, and nested directories
// are preserved. All mutations are workspace-contained, symlink-safe, and
// best-effort robust when a prior process still holds a file open.
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
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

// DefaultLogRetentionRuns is the number of most recent distinct run
// prefixes to keep on disk. clearOrdinaryLogs deletes only files
// belonging to older prefixes, preserving recent history for
// transparency while bounding growth.
const DefaultLogRetentionRuns = 10

// DefaultSubstantiveRetentionRuns is the number of most recent
// substantive run prefixes to keep on disk in addition to the ordinary
// retention window. A run is substantive when its <prefix>_audit.log
// contains at least one of "event":"shard_, "event":"action_,
// "event":"tool_, "event":"safety_. The substantive budget guarantees
// that diagnostic runs (perf_metric / kernel_query only) never evict a
// substantive run, while growth remains bounded because the substantive
// window itself is finite.
const DefaultSubstantiveRetentionRuns = 10

// maxSubstantiveScanBytes caps how much of any single audit log is
// examined during classification so a single enormous log cannot stall
// startup. Classification is streaming and stops at the first marker.
const maxSubstantiveScanBytes int64 = 1 << 20 // 1 MiB

// substantiveEventMarkers are the substrings that classify a run as
// substantive. Must match the spec exactly.
var substantiveEventMarkers = [][]byte{
	[]byte(`"event":"shard_`),
	[]byte(`"event":"action_`),
	[]byte(`"event":"tool_`),
	[]byte(`"event":"safety_`),
}

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

// runPrefixFromLogName extracts the run prefix from a log filename if it
// begins with a valid run prefix produced by generateRunPrefix. The prefix
// format is 20060102_150405.000000000_<pid>_<seq>_<rand> (46 chars):
//  8 digits date, '_' , 6 digits time, '.' , 9 digits nanos, '_' ,
//  6 digits pid, '_' , 6 digits seq, '_' , 6 hex chars. Lexical order of
// this prefix equals chronological order. Returns the prefix string on
// success or "" if the name does not start with a valid prefix.
func runPrefixFromLogName(name string) string {
	const prefixLen = 46
	if len(name) < prefixLen {
		return ""
	}
	candidate := name[:prefixLen]
	// 0-7: YYYYMMDD digits
	for i := 0; i < 8; i++ {
		if candidate[i] < '0' || candidate[i] > '9' {
			return ""
		}
	}
	if candidate[8] != '_' {
		return ""
	}
	// 9-14: HHMMSS digits
	for i := 9; i < 15; i++ {
		if candidate[i] < '0' || candidate[i] > '9' {
			return ""
		}
	}
	if candidate[15] != '.' {
		return ""
	}
	// 16-24: nanoseconds 9 digits
	for i := 16; i < 25; i++ {
		if candidate[i] < '0' || candidate[i] > '9' {
			return ""
		}
	}
	if candidate[25] != '_' {
		return ""
	}
	// 26-31: pid 6 digits
	for i := 26; i < 32; i++ {
		if candidate[i] < '0' || candidate[i] > '9' {
			return ""
		}
	}
	if candidate[32] != '_' {
		return ""
	}
	// 33-38: seq 6 digits
	for i := 33; i < 39; i++ {
		if candidate[i] < '0' || candidate[i] > '9' {
			return ""
		}
	}
	if candidate[39] != '_' {
		return ""
	}
	// 40-45: rand 6 hex chars
	for i := 40; i < 46; i++ {
		c := candidate[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return ""
		}
	}
	return candidate
}
// isSubstantiveAuditLog reports whether the audit log at path is
// substantive. It streams the file up to maxSubstantiveScanBytes and
// stops at the first occurrence of any substantiveEventMarkers substring.
// A file that cannot be opened or read is treated as diagnostic (false)
// and never causes pruning to fail — pruning is best-effort.
func isSubstantiveAuditLog(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	limited := io.LimitReader(f, maxSubstantiveScanBytes)
	buf := make([]byte, 32*1024)
	const overlap = 32 // > longest marker (16)
	var carry []byte
	for {
		n, err := limited.Read(buf)
		if n > 0 {
			var data []byte
			if len(carry) > 0 {
				data = make([]byte, len(carry)+n)
				copy(data, carry)
				copy(data[len(carry):], buf[:n])
			} else {
				// copy to avoid aliasing buf on next read when we retain tail
				data = make([]byte, n)
				copy(data, buf[:n])
			}
			for _, marker := range substantiveEventMarkers {
				if bytes.Contains(data, marker) {
					return true
				}
			}
			if len(data) > overlap {
				newCarry := make([]byte, overlap)
				copy(newCarry, data[len(data)-overlap:])
				carry = newCarry
			} else {
				newCarry := make([]byte, len(data))
				copy(newCarry, data)
				carry = newCarry
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		if n == 0 && err == nil {
			// No progress, avoid infinite loop
			break
		}
	}
	return false
}


// clearOrdinaryLogs removes/truncates previous top-level *.log files
// under logsDir using a dual retention window: the union of
//   - the most recent DefaultLogRetentionRuns distinct run prefixes, and
//   - the most recent DefaultSubstantiveRetentionRuns substantive prefixes
// (where substantive means <prefix>_audit.log contains at least one of
// "event":"shard_, "event":"action_, "event":"tool_, "event":"safety_).
// Only files belonging to prefixes in neither set are deleted. Legacy or
// unprefixed *.log files that do not start with a valid run prefix are
// treated as expired and removed.
//
// Invariants:
//   - Only direct children of logsDir with suffix ".log" are considered.
//   - Directories (including nested log archives) are preserved.
//   - Non-log files are preserved.
//   - Symlinks at the top level are preserved and never followed.
//   - Symlinked logs directory or symlinked parent (.nerd) is rejected entirely.
//   - Paths are verified to remain inside logsDir (workspace-contained).
//   - Only <prefix>_audit.log files are opened, streaming up to
//     maxSubstantiveScanBytes and stopping at first substantive marker.
//   - A file that cannot be opened is treated as diagnostic, never as a
//     reason to fail startup (best-effort).
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
	// Classify candidates by run prefix. Files that do not start with a
	// valid run prefix are collected as unprefixed (legacy) and deleted
	// regardless of the retention window to preserve prior cleanup
	// behaviour for stale non-prefixed logs (e.g., a.log, stale.log).
	prefixToFiles := make(map[string][]string)
	var unprefixedFiles []string
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

		prefix := runPrefixFromLogName(name)
		if prefix == "" {
			unprefixedFiles = append(unprefixedFiles, cleanFull)
		} else {
			prefixToFiles[prefix] = append(prefixToFiles[prefix], cleanFull)
		}
	}

	// Determine which prefixes to keep: the union of
	//   - the newest DefaultLogRetentionRuns distinct prefixes, and
	//   - the newest DefaultSubstantiveRetentionRuns substantive prefixes.
	// Lexically descending == chronologically newest first because the
	// prefix itself is sortable.
	if len(prefixToFiles) == 0 && len(unprefixedFiles) == 0 {
		return
	}
	distinct := make([]string, 0, len(prefixToFiles))
	for p := range prefixToFiles {
		distinct = append(distinct, p)
	}
	// Sort descending so newest (largest lexical) comes first.
	sort.Sort(sort.Reverse(sort.StringSlice(distinct)))
	keep := make(map[string]struct{}, DefaultLogRetentionRuns+DefaultSubstantiveRetentionRuns)
	for i, p := range distinct {
		if i < DefaultLogRetentionRuns {
			keep[p] = struct{}{}
		} else {
			break
		}
	}
	// Classify substantive prefixes. Only audit logs are opened, streaming
	// up to maxSubstantiveScanBytes and stopping at first marker.
	substantivePrefixes := make([]string, 0, len(distinct))
	for _, p := range distinct {
		auditNameLower := strings.ToLower(p + "_audit.log")
		auditPath := ""
		for _, fp := range prefixToFiles[p] {
			if strings.ToLower(filepath.Base(fp)) == auditNameLower {
				auditPath = fp
				break
			}
		}
		if auditPath == "" {
			continue
		}
		if isSubstantiveAuditLog(auditPath) {
			substantivePrefixes = append(substantivePrefixes, p)
		}
	}
	for i, p := range substantivePrefixes {
		if i < DefaultSubstantiveRetentionRuns {
			keep[p] = struct{}{}
		} else {
			break
		}
	}

	// Delete legacy/unprefixed files (best-effort).
	for _, path := range unprefixedFiles {
		if err := truncateOrRemove(path); err != nil {
			fmt.Fprintf(os.Stderr, "[logging] fresh-run: could not clear %s: %v (continuing)\n", path, err)
		}
	}
	// Delete files belonging to prefixes outside the retention window.
	for prefix, files := range prefixToFiles {
		if _, ok := keep[prefix]; ok {
			continue
		}
		for _, path := range files {
			if err := truncateOrRemove(path); err != nil {
				fmt.Fprintf(os.Stderr, "[logging] fresh-run: could not clear %s: %v (continuing)\n", path, err)
			}
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
