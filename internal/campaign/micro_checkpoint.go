package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/tactile"
)

// runTaskMicroCheckpoint enforces a minimal per-task verification gate.
func (o *Orchestrator) runTaskMicroCheckpoint(ctx context.Context, task *Task) error {
	if task == nil || !isMutatingTaskType(task.Type) {
		return nil
	}

	writeSet := o.resolveTaskWriteSet(task)
	if len(writeSet) == 0 {
		return fmt.Errorf("micro-checkpoint: mutating task %s has empty write_set", task.ID)
	}

	// File existence sanity for create/modify tasks (fail fast before expensive checks).
	for _, p := range writeSet {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("micro-checkpoint missing mutated path %s: %w", p, err)
		}
		if info.IsDir() {
			continue
		}
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
