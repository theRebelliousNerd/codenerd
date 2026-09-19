package campaign

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/observation"
	"codenerd/internal/session"
)

// An attempt's own writes (ladder C4, external audit F4). A mutating task's
// lease and snapshot cover the write set it declared, and a coder turn can
// write outside it. R1-2's failed create had its new file removed by the
// rollback and its edit to cmd_mangle_check.go -- outside the set, and
// importing the removed file -- kept, so the tree stopped building; and two
// tasks running side by side could interleave writes to a file neither
// declared.
//
// So an open attempt keeps a record. spawnTask puts a write guard on each
// turn: a write to a path the task does not hold takes that path's lease for
// the task at the moment of the write (writeGuard), held until the attempt
// ends, and a path another task holds is refused. Each turn's writes come back
// on the observed return and are recorded. When the attempt fails, those
// writes are undone with the snapshot (undoFailedAttempt); either way its
// leases are released last.
//
// Records exist only while an attempt is open (beginAttempt .. endAttempt), so
// a task that runs without a mutation snapshot leaves nothing behind.
type attemptRecord struct {
	writes []observation.FileWrite
	leases []*writeSetLockLease
}

func (o *Orchestrator) beginAttempt(task *Task) {
	if task == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.attempts == nil {
		o.attempts = map[string]*attemptRecord{}
	}
	o.attempts[task.ID] = &attemptRecord{}
}

func (o *Orchestrator) recordAttemptWrites(task *Task, writes []observation.FileWrite) {
	if task == nil || len(writes) == 0 {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if rec := o.attempts[task.ID]; rec != nil {
		rec.writes = append(rec.writes, writes...)
	}
}

// attemptWrites returns the writes the task's open attempt has recorded so far.
func (o *Orchestrator) attemptWrites(task *Task) []observation.FileWrite {
	if task == nil {
		return nil
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	if rec := o.attempts[task.ID]; rec != nil {
		return append([]observation.FileWrite(nil), rec.writes...)
	}
	return nil
}

// holdForAttempt keeps a lease the write guard took until the task's attempt
// ends. With no attempt open there is nothing to hold it for: the write was
// checked, and the lease goes back at once.
func (o *Orchestrator) holdForAttempt(task *Task, lease *writeSetLockLease) {
	if lease == nil {
		return
	}
	o.mu.Lock()
	rec := o.attempts[task.ID]
	if rec != nil {
		rec.leases = append(rec.leases, lease)
	}
	o.mu.Unlock()
	if rec == nil {
		lease.release()
	}
}

// endAttempt closes the task's record and returns it; the caller undoes its
// writes if it failed, then releases its leases.
func (o *Orchestrator) endAttempt(task *Task) attemptRecord {
	if task == nil {
		return attemptRecord{}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	rec := o.attempts[task.ID]
	delete(o.attempts, task.ID)
	if rec == nil {
		return attemptRecord{}
	}
	return *rec
}

func (rec attemptRecord) release() {
	for _, lease := range rec.leases {
		lease.release()
	}
}

// writeGuard is the guard spawnTask puts on each turn it runs for task.
func (o *Orchestrator) writeGuard(task *Task) session.WriteGuard {
	return func(_ context.Context, paths []string) error {
		if o.writeSetLocks == nil {
			return nil
		}
		lease, heldBy, err := o.writeSetLocks.tryAcquire(task.ID, paths)
		if err != nil {
			return fmt.Errorf("campaign task %s may not write %s: %w", task.ID, strings.Join(paths, ", "), err)
		}
		if heldBy != "" {
			return fmt.Errorf("%s is held by campaign task %s, which may be writing it now; leave it to that task, or come back to it once that task has finished", strings.Join(paths, ", "), heldBy)
		}
		o.holdForAttempt(task, lease)
		return nil
	}
}

// undoFailedAttempt puts the workspace back after a failed attempt: the
// attempt's own writes outside what the snapshot captured, each only while it
// still holds what the attempt left, then the snapshot, then the attempt's
// leases -- last, so no other task writes a path in between. It returns cause
// with whatever could not be put back, which the retry reads.
func (o *Orchestrator) undoFailedAttempt(task *Task, snapshot taskExecutionSnapshot, cause error) error {
	rec := o.endAttempt(task)
	defer rec.release()
	var outside []observation.FileWrite
	for _, w := range rec.writes {
		if !snapshot.covers(w.Path) {
			outside = append(outside, w)
		}
	}
	err := cause
	left, undoErr := undoAttemptWrites(outside)
	if len(left) > 0 {
		err = fmt.Errorf("%w; the attempt's writes left in place: %s", err, strings.Join(left, ", "))
	}
	if undoErr != nil {
		err = fmt.Errorf("%w (undo failed: %v)", err, undoErr)
	}
	if rollbackErr := o.rollbackTaskExecutionSnapshot(snapshot); rollbackErr != nil {
		err = fmt.Errorf("%w (rollback failed: %v)", err, rollbackErr)
	}
	return err
}

// undoAttemptWrites puts each path the attempt wrote back as it was before
// the attempt -- when it still holds what the attempt left. A path another
// writer has changed since is left as it is and named, rather than undone
// over a sibling's work; so is a path whose earlier content was never read.
// It returns the paths it left.
func undoAttemptWrites(writes []observation.FileWrite) (left []string, err error) {
	// One span per path: the first record's Before is the state before the
	// attempt, the last record's After is what the attempt left.
	type span struct{ before, after observation.FileState }
	var order []string
	spans := map[string]*span{}
	for _, w := range writes {
		if s, seen := spans[w.Path]; seen {
			s.after = w.After
			continue
		}
		spans[w.Path] = &span{before: w.Before, after: w.After}
		order = append(order, w.Path)
	}
	var errs []error
	for _, path := range order {
		s := spans[path]
		if !s.before.Known {
			left = append(left, path+" (what it held before the attempt is unknown)")
			continue
		}
		if !s.after.Known || !holds(path, s.after) {
			left = append(left, path+" (changed since the attempt wrote it)")
			continue
		}
		if s.before.Exists {
			if werr := os.WriteFile(path, []byte(s.before.Content), 0o644); werr != nil {
				errs = append(errs, werr)
				left = append(left, path+" (could not be restored)")
			}
			continue
		}
		if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
			errs = append(errs, rerr)
			left = append(left, path+" (could not be removed)")
		}
	}
	return left, errors.Join(errs...)
}

// holds reports whether path is in state now.
func holds(path string, state observation.FileState) bool {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return !state.Exists
	}
	return err == nil && state.Exists && string(data) == state.Content
}

// covers reports whether the snapshot restores path itself: a file it
// captured, or anything under a root of the declared write set. Those are the
// snapshot's to put back -- another writer in the same attempt (the document
// fallback) may have written them after the turn did.
func (s taskExecutionSnapshot) covers(path string) bool {
	within := func(root string) bool {
		rel, err := filepath.Rel(root, path)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
	}
	for _, m := range s.fileMutations {
		if rel, err := filepath.Rel(m.Path, path); err == nil && rel == "." {
			return true
		}
	}
	for _, root := range s.snapshotRoots {
		if within(root) {
			return true
		}
	}
	return false
}
