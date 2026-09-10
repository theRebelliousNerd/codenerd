// Package jsonl appends records to a size-capped, line-delimited JSON log and
// reads them back.
//
// It exists because two subsystems need the same thing for the same reason.
// Broker receipts and prompt-atom selections are both measurement streams whose
// questions span processes -- "what is the distribution of calls per epoch
// across real sessions", "which atoms are selected together" -- so both have to
// reach disk, and both are written on a hot path where blocking is not
// acceptable. One implementation of that, not two that drift.
package jsonl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// DefaultMaxBytes caps a live log before rotation.
const DefaultMaxBytes int64 = 20 << 20 // 20 MiB

// Appender writes JSON records one per line.
//
// Writes are unbuffered single Write calls under O_APPEND -- atomic for small
// writes on every platform this runs on -- with no fsync. That makes two
// processes in one workspace safe to append concurrently without coordinating,
// and keeps the write off the latency path of whatever produced the record.
// Losing the last few lines to a crash is the accepted cost: this is
// measurement data, not a ledger of record.
type Appender struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	written  int64
	maxBytes int64

	// firstErr and errCount are recorded rather than returned because the
	// callers write from paths that cannot fail. An operator needs to be able
	// to tell "no data because nothing happened" from "no data because the
	// writer has been broken since boot".
	firstErr error
	errCount int
}

// Open creates or opens a log at path, creating parent directories.
func Open(path string) (*Appender, error) {
	if path == "" {
		return nil, errors.New("jsonl: path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("jsonl: create dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("jsonl: open %s: %w", path, err)
	}

	var size int64
	if info, statErr := f.Stat(); statErr == nil {
		size = info.Size()
	}
	return &Appender{path: path, file: f, written: size, maxBytes: DefaultMaxBytes}, nil
}

// SetMaxBytes overrides the rotation threshold. A non-positive value restores
// the default rather than disabling rotation: an unbounded log in a long agent
// session is the failure the cap exists to prevent.
func (a *Appender) SetMaxBytes(n int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n <= 0 {
		n = DefaultMaxBytes
	}
	a.maxBytes = n
}

// Append marshals v and writes it as one line.
func (a *Appender) Append(v any) {
	line, err := json.Marshal(v)
	if err != nil {
		a.noteErr(err)
		return
	}
	line = append(line, '\n')

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.file == nil {
		return
	}
	if a.written+int64(len(line)) > a.maxBytes {
		if rotErr := a.rotateLocked(); rotErr != nil {
			a.recordErrLocked(rotErr)
			return
		}
	}

	n, werr := a.file.Write(line)
	a.written += int64(n)
	if werr != nil {
		a.recordErrLocked(werr)
	}
}

// rotateLocked moves the live log aside and starts a fresh one. Exactly one
// generation is kept: a readout wants recent history, and keeping more turns a
// bounded cost into an unbounded one.
func (a *Appender) rotateLocked() error {
	if err := a.file.Close(); err != nil {
		return err
	}
	// Rename rather than truncate, so a reader holding the old file keeps a
	// consistent view instead of watching its content vanish mid-scan.
	if err := os.Rename(a.path, a.path+".1"); err != nil && !os.IsNotExist(err) {
		return err
	}

	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		a.file = nil
		return err
	}
	a.file = f
	a.written = 0
	return nil
}

func (a *Appender) noteErr(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.recordErrLocked(err)
}

func (a *Appender) recordErrLocked(err error) {
	if a.firstErr == nil {
		a.firstErr = err
	}
	a.errCount++
}

// Err returns the first write failure and how many have occurred.
func (a *Appender) Err() (error, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.firstErr, a.errCount
}

// Path returns the live log path.
func (a *Appender) Path() string { return a.path }

// Close closes the log. Safe to call more than once.
func (a *Appender) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil {
		return nil
	}
	err := a.file.Close()
	a.file = nil
	return err
}

// Read decodes every record from a log and its rotated generation, oldest
// first. It returns the records, how many generations ended on a malformed
// line, and any error other than the log simply not existing.
//
// A truncated tail is tolerated rather than fatal. These logs are appended to
// by several processes and can be cut mid-line by a crash; refusing to report
// anything because the last line is short would throw away a whole sample to
// protect a number that is already approximate.
func Read[T any](path string) ([]T, int, error) {
	var (
		out       []T
		truncated int
	)

	// Oldest generation first, so the combined slice stays in write order.
	for _, p := range []string{path + ".1", path} {
		recs, cut, err := readOne[T](p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return out, truncated, err
		}
		out = append(out, recs...)
		if cut {
			truncated++
		}
	}
	return out, truncated, nil
}

func readOne[T any](path string) ([]T, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()

	var out []T
	dec := json.NewDecoder(f)
	for {
		var rec T
		if err := dec.Decode(&rec); err != nil {
			if errors.Is(err, io.EOF) {
				return out, false, nil
			}
			// A stream decoder cannot resynchronize after a malformed object,
			// so a bad line ends this generation. Everything before it stands.
			return out, true, nil
		}
		out = append(out, rec)
	}
}
