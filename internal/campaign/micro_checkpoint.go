package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

// findWorkspaceFileByBase walks workspace for a file with the given basename
// (skips .nerd, node_modules, target). Used when planner paths don't match layout.
func findWorkspaceFileByBase(workspace, base string) string {
	if workspace == "" || base == "" || base == "." || base == string(filepath.Separator) {
		return ""
	}
	var found string
	_ = filepath.WalkDir(workspace, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".nerd" || name == "node_modules" || name == "target" || name == ".git" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(d.Name(), base) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// runTaskMicroCheckpoint enforces a minimal per-task verification gate.
func (o *Orchestrator) runTaskMicroCheckpoint(ctx context.Context, task *Task) error {
	if task == nil || !isMutatingTaskType(task.Type) {
		return nil
	}

	writeSet := o.resolveTaskWriteSet(task)
	if len(writeSet) == 0 {
		// Mutating tasks may omit write_set (see acquireWriteSetLease soft path).
		// Skip micro-checkpoint rather than hard-fail the whole campaign task.
		logging.Get(logging.CategoryCampaign).Warn(
			"micro-checkpoint: task %s has empty write_set; skipping file gate", task.ID,
		)
		return nil
	}

	// File existence sanity for create/modify tasks (fail fast before expensive checks).
	// Planner paths are often wrong (e.g. cmd/server/main.go when code is backend/main.go).
	// Accept any write_set path that exists OR a same-basename file under the workspace
	// so checkpoints do not hard-fail layout mismatches after successful nearby writes.
	for _, p := range writeSet {
		info, err := os.Stat(p)
		if err == nil {
			if info.IsDir() {
				continue
			}
			continue
		}
		if alt := findWorkspaceFileByBase(o.workspace, filepath.Base(p)); alt != "" {
			logging.Get(logging.CategoryCampaign).Warn(
				"micro-checkpoint: planned path %s missing; found alternate %s", p, alt,
			)
			continue
		}
		// Soft-skip missing planned paths when ANY other write_set entry exists
		// or any alternate was already accepted — planners invent extra files
		// (server.py + main.py). Fail only if zero planned paths resolved.
		logging.Get(logging.CategoryCampaign).Warn(
			"micro-checkpoint: skipping missing planned path %s", p,
		)
		continue
	}
	// If write set was non-empty but nothing existed, fail.
	anyExists := false
	for _, p := range writeSet {
		if _, err := os.Stat(p); err == nil {
			anyExists = true
			break
		}
		if findWorkspaceFileByBase(o.workspace, filepath.Base(p)) != "" {
			anyExists = true
			break
		}
	}
	if !anyExists {
		return fmt.Errorf("micro-checkpoint: none of planned write_set paths exist: %v", writeSet)
	}

	if hasGoFiles(writeSet) && fileExists(o.workspace, "go.mod") {
		if o.executor == nil {
			return fmt.Errorf("micro-checkpoint executor unavailable for go build verification")
		}
		cmd := tactile.Command{
			Binary:           "go",
			Arguments:        []string{"build", "./..."},
			WorkingDirectory: o.workspace,
			Limits: &tactile.ResourceLimits{
				TimeoutMs: 20000,
			},
		}
		res, err := o.executor.Execute(ctx, cmd)
		if err != nil {
			out := ""
			if res != nil {
				out = res.Output()
			}
			return fmt.Errorf("micro-checkpoint go build failed: %w: %s", err, out)
		}
		if res != nil && res.ExitCode != 0 {
			return fmt.Errorf("micro-checkpoint go build failed with exit code %d: %s", res.ExitCode, res.Output())
		}
	}
	return nil
}

func hasGoFiles(paths []string) bool {
	for _, p := range paths {
		if strings.EqualFold(filepath.Ext(p), ".go") {
			return true
		}
	}
	return false
}
