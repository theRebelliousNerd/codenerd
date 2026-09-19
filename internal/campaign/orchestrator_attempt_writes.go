package campaign

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/observation"
)

// The attempt's own writes (ladder C4). A mutating task's snapshot restores
// the write set it declared, and a coder turn can write outside it: R1-2's
// failed create had its new file removed by the rollback and its edit to
// cmd_mangle_check.go -- outside the set, and importing the removed file --
// kept, so the tree stopped building. spawnTask records every path each of
// the attempt's turns wrote (observation.Return.Writes); when the attempt
// fails, those writes are undone with the snapshot.
//
// Records are kept only while an attempt is open (beginAttemptWrites ..
// takeAttemptWrites), so a task that runs without a mutation snapshot leaves
// nothing behind.

func (o *Orchestrator) beginAttemptWrites(task *Task) {
	if task == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.attemptWrites == nil {
		o.attemptWrites = map[string][]observation.FileWrite{}
	}
	o.attemptWrites[task.ID] = nil
}

func (o *Orchestrator) recordAttemptWrites(task *Task, writes []observation.FileWrite) {
	if task == nil || len(writes) == 0 {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, open := o.attemptWrites[task.ID]; !open {
		return
	}
	o.attemptWrites[task.ID] = append(o.attemptWrites[task.ID], writes...)
}

func (o *Orchestrator) takeAttemptWrites(task *Task) []observation.FileWrite {
	if task == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	writes := o.attemptWrites[task.ID]
	delete(o.attemptWrites, task.ID)
	return writes
}

// undoAttemptWrites puts each path the attempt wrote back as it was before
// the attempt -- when it still holds what the attempt left. The campaign runs
// tasks side by side, so a path another writer has changed since is left as
// it is and named, rather than undone over a sibling's work; so is a path
// whose earlier content was never read. It returns the paths it left.
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

// undoFailedAttempt puts the workspace back after a failed attempt: the
// attempt's own writes outside what the snapshot captured, each only while it
// still holds what the attempt left, then the snapshot. It returns cause with
// whatever could not be put back, which the retry reads.
func (o *Orchestrator) undoFailedAttempt(task *Task, snapshot taskExecutionSnapshot, cause error) error {
	var outside []observation.FileWrite
	for _, w := range o.takeAttemptWrites(task) {
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
