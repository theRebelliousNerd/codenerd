package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// leaveBuildableTree runs when a repair episode gives up. A turn may end with
// its tests red and its edits in place: the tree still runs, and the edits are
// a starting point. It may not end with the tree uncompilable. codeNERD works
// in the tree it is built from, a campaign's next task inherits whatever this
// one left, and a build that does not compile stops every one of them.
//
// Observed 2026-09-21 (nerd fix, refinement duplicates): three repair attempts,
// the third left internal/campaign/replan.go calling a function it had deleted,
// the run exited, and the repository did not build until it was put back by
// hand.
//
// So: if the workspace does not build, the turn's attempt is saved as a patch
// under .nerd/attempts/ and the files it wrote are put back as the turn found
// them. Nothing is lost and nothing is guessed: a file whose earlier state is
// unknown stops the restore before it touches anything (turnFiles.restore).
// It returns a sentence for the turn's error, empty when it changed nothing.
func (e *Executor) leaveBuildableTree(ctx context.Context, result *ExecutionResult) string {
	if result == nil || len(result.WrittenPaths) == 0 {
		return ""
	}
	workspace := e.workspaceForVerification()
	if strings.TrimSpace(workspace) == "" {
		return ""
	}
	// "Does not build" only means something in a workspace that is a Go module.
	// `go build ./...` also fails in a directory with no go.mod, and a turn
	// that wrote two text files there did not break anything: undoing it on
	// that evidence deleted the turn's work (TestJourney_PlannedSteps).
	if !isGoModuleRoot(workspace) {
		return ""
	}
	build := verifyBuild(ctx, workspace, nil)
	if build.Verdict() != VerifyFailed {
		return ""
	}
	patch := turnDiffSection(workspace, result.WrittenPaths, result.PreWriteContents)
	saved := saveAttemptPatch(workspace, patch)
	restored, err := turnFiles{pre: map[string]PreImage{}}.restore(workspace, result)
	if err != nil {
		logging.Get(logging.CategorySession).Error(
			"The turn gave up with the build broken and its files could not be put back (%v); the workspace does not compile", err)
		return fmt.Sprintf("The workspace does NOT build and the turn's files could not be restored (%v).", err)
	}
	logging.Get(logging.CategorySession).Warn(
		"The turn gave up with the build broken; restored %s as the turn found them; the attempt is saved at %s",
		strings.Join(restored, ", "), saved)
	if saved == "" {
		return fmt.Sprintf("The attempt left the workspace uncompilable, so %s were restored as the turn found them.", strings.Join(restored, ", "))
	}
	return fmt.Sprintf("The attempt left the workspace uncompilable, so %s were restored as the turn found them; the attempt is saved as a patch at %s.",
		strings.Join(restored, ", "), saved)
}

// saveAttemptPatch writes the attempt under .nerd/attempts and returns the
// workspace-relative path, or "" when there was nothing to save or no place to
// save it. Failing to save never blocks the restore: a buildable tree comes
// first.
func saveAttemptPatch(workspace, patch string) string {
	if strings.TrimSpace(patch) == "" {
		return ""
	}
	dir := filepath.Join(workspace, ".nerd", "attempts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	name := "attempt_" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".patch.md"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(patch), 0o644); err != nil {
		return ""
	}
	return filepath.ToSlash(filepath.Join(".nerd", "attempts", name))
}

// isGoModuleRoot reports whether the workspace root holds a go.mod or go.work.
func isGoModuleRoot(workspace string) bool {
	for _, name := range []string{"go.mod", "go.work"} {
		if info, err := os.Stat(filepath.Join(workspace, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
