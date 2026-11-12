package problemlog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"codenerd/internal/types"
)

// Operation timings: the evidence the optimize lane ranks (self_timing,
// policy/self_repair.mg). Every logging.StartTimer stop writes a perf_metric
// (or, over its threshold, perf_slow) event to the run's audit log with the
// operation's system, name and duration. The watcher reads the log
// incrementally and keeps one aggregate per operation; the kernel picks the
// costliest. Nothing here is estimated: an operation the run did not time has
// no row.

// TimingPredicate is self_timing(Target, System, Operation, Count, TotalMs, MeanMs).
const TimingPredicate = "self_timing"

// TimingTargetPrefix starts every optimize-lane target.
const TimingTargetPrefix = "timing:"

// Timing is one operation's aggregate over the run.
type Timing struct {
	System    string
	Operation string
	Count     int64
	TotalMs   int64
	MaxMs     int64
}

// Target is the optimize lane's name for the operation.
func (t Timing) Target() string { return TimingTarget(t.System, t.Operation) }

// TimingTarget names an operation as an optimize-lane target.
func TimingTarget(system, operation string) string {
	return TimingTargetPrefix + system + "." + operation
}

// MeanMs is the operation's average duration, rounded down.
func (t Timing) MeanMs() int64 {
	if t.Count == 0 {
		return 0
	}
	return t.TotalMs / t.Count
}

// auditPerf is the part of an audit event (internal/logging AuditEvent) a
// timing needs.
type auditPerf struct {
	Event      string `json:"event"`
	Category   string `json:"cat"`
	Action     string `json:"action"`
	DurationMs int64  `json:"dur_ms"`
}

var perfNeedles = [][]byte{[]byte(`"event":"perf_metric"`), []byte(`"event":"perf_slow"`)}

// TimingWatcher aggregates an audit log's timings as it grows. It is safe for
// concurrent use.
type TimingWatcher struct {
	mu      sync.Mutex
	path    string
	offset  int64
	timings map[[2]string]*Timing
	// changed is set when a read added a timing since the last Facts.
	changed bool
}

// NewTimingWatcher starts with no log.
func NewTimingWatcher() *TimingWatcher {
	return &TimingWatcher{timings: map[[2]string]*Timing{}}
}

// Read consumes the complete lines appended to path since the last read. A
// new path (a new run's log) starts the aggregate over: timings belong to the
// process that measured them.
func (w *TimingWatcher) Read(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if path == "" {
		return nil
	}
	if path != w.path {
		w.path, w.offset, w.timings, w.changed = path, 0, map[[2]string]*Timing{}, true
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat audit log: %w", err)
	}
	if fi.Size() < w.offset {
		// Rotated in place: the new file is a new stretch of the same run.
		w.offset = 0
	}
	if _, err := f.Seek(w.offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek audit log: %w", err)
	}
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 && line[len(line)-1] == '\n' {
			w.offset += int64(len(line))
			w.add(line)
		}
		if err == io.EOF {
			return nil // a partial last line waits for its newline
		}
		if err != nil {
			return fmt.Errorf("read audit log: %w", err)
		}
	}
}

func (w *TimingWatcher) add(line []byte) {
	matched := false
	for _, n := range perfNeedles {
		if bytes.Contains(line, n) {
			matched = true
			break
		}
	}
	if !matched {
		return
	}
	var ev auditPerf
	if err := json.Unmarshal(line, &ev); err != nil || ev.Action == "" {
		return
	}
	key := [2]string{ev.Category, ev.Action}
	t, ok := w.timings[key]
	if !ok {
		t = &Timing{System: ev.Category, Operation: ev.Action}
		w.timings[key] = t
	}
	t.Count++
	t.TotalMs += ev.DurationMs
	if ev.DurationMs > t.MaxMs {
		t.MaxMs = ev.DurationMs
	}
	w.changed = true
}

// Timings is every operation's aggregate, costliest first.
func (w *TimingWatcher) Timings() []Timing {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.listLocked()
}

func (w *TimingWatcher) listLocked() []Timing {
	out := make([]Timing, 0, len(w.timings))
	for _, t := range w.timings {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].TotalMs != out[k].TotalMs {
			return out[i].TotalMs > out[k].TotalMs
		}
		return out[i].Target() < out[k].Target()
	})
	return out
}

// Lookup returns target's aggregate.
func (w *TimingWatcher) Lookup(target string) (Timing, bool) {
	for _, t := range w.Timings() {
		if t.Target() == target {
			return t, true
		}
	}
	return Timing{}, false
}

// Changed reports, and clears, whether a read moved the aggregate since the
// last call.
func (w *TimingWatcher) Changed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.changed
	w.changed = false
	return c
}

// TimingFacts renders timings as self_timing rows.
func TimingFacts(timings []Timing) []types.Fact {
	out := make([]types.Fact, 0, len(timings))
	for _, t := range timings {
		out = append(out, types.Fact{Predicate: TimingPredicate, Args: []any{
			types.MangleString(t.Target()), types.MangleString(t.System), types.MangleString(t.Operation),
			t.Count, t.TotalMs, t.MeanMs(),
		}})
	}
	return out
}

// ReplaceTimings swaps the kernel's self_timing rows for timings.
func ReplaceTimings(k Kernel, timings []Timing) error {
	if k == nil {
		return errors.New("problemlog: nil Kernel")
	}
	var errs []error
	if err := k.Retract(TimingPredicate); err != nil {
		errs = append(errs, fmt.Errorf("problemlog: retract %s: %w", TimingPredicate, err))
	}
	if facts := TimingFacts(timings); len(facts) > 0 {
		if err := k.AssertBatch(facts); err != nil {
			errs = append(errs, fmt.Errorf("problemlog: assert %d timings: %w", len(facts), err))
		}
	}
	return errors.Join(errs...)
}
