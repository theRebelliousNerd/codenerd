package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Size and age based rotation for the files this package appends to.
//
// Run-prefix retention (fresh_run.go) bounds growth *across* runs: the oldest
// prefixes are deleted at startup. It does nothing *within* a run, and a run is
// not a short-lived thing here — an interactive chat session or a campaign can
// stay up for hours with the kernel and api categories writing continuously.
// A single .log reaching multiple gigabytes is a real outcome of that, and it
// takes the workspace with it. Rotation closes the open segment when it gets
// too big or too old and starts a new one, keeping at most maxRotated
// historical segments per file.
//
// Rotated segments keep the .log suffix and the leading run prefix
// (<prefix>_kernel.20260815T031500.123Z.log) so the startup retention sweep
// sees them as part of the same run and expires them together. A name that
// dropped the suffix would have been immortal.

// Rotation defaults. They are deliberately generous: rotation is a safety net
// against unbounded growth, not a log-shipping policy, and a segment boundary
// in the middle of a trace makes debugging harder.
const (
	defaultMaxLogFileBytes int64 = 32 << 20 // 32 MiB
	defaultMaxRotatedFiles       = 3
)

// rotatingFile is an io.WriteCloser that appends to path, closing and renaming
// the current segment when it exceeds the size or age budget. Writes are whole
// log lines (log.Logger issues one Write per record), so a rotation boundary
// never splits a line.
type rotatingFile struct {
	mu       sync.Mutex
	f        *os.File
	path     string
	size     int64
	opened   time.Time
	maxBytes int64
	maxAge   time.Duration
	keep     int
}

// openRotatingFile opens (or creates) path for append and captures the current
// size so an existing segment from earlier in the same run counts toward the
// budget instead of restarting it.
func openRotatingFile(path string) (*rotatingFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	size := int64(0)
	if fi, statErr := f.Stat(); statErr == nil {
		size = fi.Size()
	}
	maxBytes, maxAge, keep := rotationPolicy()
	return &rotatingFile{
		f:        f,
		path:     path,
		size:     size,
		opened:   time.Now(),
		maxBytes: maxBytes,
		maxAge:   maxAge,
		keep:     keep,
	}, nil
}

// Write appends p, rotating first when the current segment is already over
// budget. A failed rotation is not fatal: the write still goes to the open
// file, because losing diagnostics is worse than an oversized one.
func (r *rotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return 0, os.ErrClosed
	}
	if r.shouldRotateLocked(len(p)) {
		if err := r.rotateLocked(); err != nil {
			fmt.Fprintf(os.Stderr, "[logging] rotation failed for %s: %v (continuing in place)\n", r.path, err)
		}
	}
	n, err := r.f.Write(p)
	r.size += int64(n)
	return n, err
}

// WriteString exists so audit.go can keep its string-oriented call shape.
func (r *rotatingFile) WriteString(s string) (int, error) {
	return r.Write([]byte(s))
}

func (r *rotatingFile) shouldRotateLocked(incoming int) bool {
	if r.size == 0 {
		return false // never rotate an empty segment; that just churns names
	}
	if r.maxBytes > 0 && r.size+int64(incoming) > r.maxBytes {
		return true
	}
	if r.maxAge > 0 && time.Since(r.opened) > r.maxAge {
		return true
	}
	return false
}

// rotateLocked closes the current segment, renames it with a sortable UTC
// stamp, prunes older segments, and opens a fresh file at the original path.
func (r *rotatingFile) rotateLocked() error {
	if err := r.f.Close(); err != nil {
		// Keep going: the rename is what actually frees the name, and a close
		// error here is usually a double close on a file we are discarding.
		fmt.Fprintf(os.Stderr, "[logging] closing segment %s: %v\n", r.path, err)
	}
	r.f = nil

	archived := rotatedSegmentName(r.path, time.Now())
	if err := os.Rename(r.path, archived); err != nil && !os.IsNotExist(err) {
		// Reopen the original so logging survives a rename failure (Windows
		// sharing violations, read-only dir).
		f, openErr := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if openErr != nil {
			return fmt.Errorf("rename %s: %v; reopen: %w", r.path, err, openErr)
		}
		r.f = f
		r.opened = time.Now()
		return fmt.Errorf("rename %s -> %s: %w", r.path, archived, err)
	}

	pruneRotatedSegments(r.path, r.keep)

	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("reopen %s after rotation: %w", r.path, err)
	}
	r.f = f
	r.size = 0
	r.opened = time.Now()
	return nil
}

// Close releases the underlying file. Safe to call repeatedly.
func (r *rotatingFile) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}

// Path returns the live segment path (the archived segments are derived names).
func (r *rotatingFile) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// rotatedSegmentName turns /logs/<prefix>_kernel.log into
// /logs/<prefix>_kernel.20260815T031500.123Z.log — still a .log, still
// prefixed, and lexically ordered by time.
func rotatedSegmentName(path string, at time.Time) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, ".log")
	stamp := at.UTC().Format("20060102T150405.000Z")
	return filepath.Join(dir, fmt.Sprintf("%s.%s.log", stem, stamp))
}

// rotatedSegments lists archived segments for path, oldest first.
func rotatedSegments(path string) []string {
	dir := filepath.Dir(path)
	stem := strings.TrimSuffix(filepath.Base(path), ".log")
	matches, err := filepath.Glob(filepath.Join(dir, stem+".*.log"))
	if err != nil {
		return nil
	}
	sort.Strings(matches) // timestamp is fixed-width, so lexical == chronological
	return matches
}

// pruneRotatedSegments keeps the newest keep segments and removes the rest.
// keep <= 0 removes every archived segment (rotation as pure truncation).
func pruneRotatedSegments(path string, keep int) {
	segments := rotatedSegments(path)
	if keep < 0 {
		keep = 0
	}
	if len(segments) <= keep {
		return
	}
	for _, old := range segments[:len(segments)-keep] {
		if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[logging] could not prune rotated segment %s: %v\n", old, err)
		}
	}
}

// rotationPolicy reads the configured budgets. A zero/absent setting means the
// default; a negative size or age disables that half of the policy, which is
// how an operator opts out entirely (max_log_file_mb: -1).
func rotationPolicy() (maxBytes int64, maxAge time.Duration, keep int) {
	configMu.RLock()
	defer configMu.RUnlock()

	switch {
	case config.MaxLogFileMB < 0:
		maxBytes = 0
	case config.MaxLogFileMB == 0:
		maxBytes = defaultMaxLogFileBytes
	default:
		maxBytes = config.MaxLogFileMB << 20
	}

	if config.MaxLogFileMinutes > 0 {
		maxAge = time.Duration(config.MaxLogFileMinutes) * time.Minute
	}

	switch {
	case config.MaxRotatedFiles < 0:
		keep = 0
	case config.MaxRotatedFiles == 0:
		keep = defaultMaxRotatedFiles
	default:
		keep = config.MaxRotatedFiles
	}
	return maxBytes, maxAge, keep
}
