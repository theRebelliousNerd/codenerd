package codedom

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
)

// commitMu serializes every codedom commit: apply_edits, the element verbs,
// create_file and repoint. The preflight (read, stage, validate) runs
// concurrently; the optimistic check, the writes and any rollback do not.
var commitMu sync.Mutex

// commitWriteFile is the write seam. Tests replace it to inject a failure
// mid-commit and exercise rollback.
var commitWriteFile = atomicfile.WriteFile

// commitBeforeHook is a test seam run inside commitMu immediately before the
// optimistic conflict check, so a test can change a file after its snapshot
// and before the commit verifies it, deterministically.
var commitBeforeHook func()

// plannedWrite is one file a commit writes.
type plannedWrite struct {
	abs, rel string
	// orig is what the file held when it was read. A create has none, and
	// commits only while the file still does not exist.
	orig   []byte
	create bool
	data   []byte
	mode   os.FileMode
}

// commitFiles writes every planned file or none. Under commitMu it re-checks
// that each file still holds exactly what was read (a concurrent writer is an
// optimistic conflict, not something to overwrite), writes in order, and on a
// failure restores what it wrote -- in reverse, and only files still holding
// what this commit wrote, so a restore never clobbers someone else's write.
func commitFiles(ctx context.Context, planned []plannedWrite) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	commitMu.Lock()
	defer commitMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if commitBeforeHook != nil {
		commitBeforeHook()
	}
	unchanged := func(p plannedWrite) error {
		if p.create {
			if _, err := os.Stat(p.abs); err == nil {
				return fmt.Errorf("optimistic conflict: %s now exists", p.rel)
			}
			return nil
		}
		cur, err := projectdoc.ReadFileForTool(p.abs)
		if err != nil {
			return fmt.Errorf("optimistic conflict: failed to re-read %s: %w", p.rel, err)
		}
		if !bytes.Equal(cur, p.orig) {
			return fmt.Errorf("optimistic conflict: file %s changed since snapshot", p.rel)
		}
		return nil
	}
	for _, p := range planned {
		if err := unchanged(p); err != nil {
			return err
		}
	}

	var succeeded []plannedWrite
	var commitErr error
	failedIdx := -1
	for idx, p := range planned {
		if err := ctx.Err(); err != nil {
			commitErr = err
			break
		}
		if err := unchanged(p); err != nil {
			commitErr = fmt.Errorf("%w immediately before write", err)
			break
		}
		if p.create {
			if err := os.MkdirAll(filepath.Dir(p.abs), 0o755); err != nil {
				commitErr = fmt.Errorf("failed to create the directory of %s: %w", p.rel, err)
				break
			}
		}
		if err := commitWriteFile(p.abs, p.data, p.mode); err != nil {
			commitErr = fmt.Errorf("failed to write %s: %w", p.rel, err)
			failedIdx = idx
			break
		}
		logging.Audit().FileOp(logging.AuditFileWrite, p.abs, int64(len(p.data)), true, "")
		succeeded = append(succeeded, p)
	}
	if commitErr == nil {
		return nil
	}

	var conflicts []string
	restore := func(p plannedWrite) {
		if p.create {
			if err := os.Remove(p.abs); err != nil && !os.IsNotExist(err) {
				conflicts = append(conflicts, p.rel+": restore failed: "+err.Error())
			}
			return
		}
		if err := commitWriteFile(p.abs, p.orig, p.mode); err != nil {
			conflicts = append(conflicts, p.rel+": restore failed: "+err.Error())
			return
		}
		after, err := projectdoc.ReadFileForTool(p.abs)
		switch {
		case err != nil:
			conflicts = append(conflicts, p.rel+": restore failed: "+err.Error())
		case !bytes.Equal(after, p.orig):
			conflicts = append(conflicts, p.rel)
		}
	}
	if failedIdx >= 0 {
		// A write that failed part-way may have left partial bytes.
		p := planned[failedIdx]
		cur, err := projectdoc.ReadFileForTool(p.abs)
		switch {
		case p.create && err != nil:
		case err != nil:
			conflicts = append(conflicts, p.rel+": restore failed: "+err.Error())
		case p.create || !bytes.Equal(cur, p.orig):
			restore(p)
		}
	}
	for j := len(succeeded) - 1; j >= 0; j-- {
		p := succeeded[j]
		cur, err := projectdoc.ReadFileForTool(p.abs)
		if err != nil || !bytes.Equal(cur, p.data) {
			conflicts = append(conflicts, p.rel)
			continue
		}
		if p.create {
			restore(p)
			continue
		}
		if err := commitWriteFile(p.abs, p.orig, p.mode); err != nil {
			conflicts = append(conflicts, p.rel+": restore failed: "+err.Error())
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%w; rollback conflicts on: %s", commitErr, strings.Join(conflicts, ", "))
	}
	return commitErr
}
