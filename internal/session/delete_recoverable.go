package session

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"codenerd/internal/processutil"
	"codenerd/internal/types"
)

// A delete the repository can undo is not the act the permission gate guards.
//
// requires_permission(/delete_file) exists because deletion is irreversible,
// and the gate's refusal says so: "requires human approval and cannot be
// self-authorized". For a file git tracks with no uncommitted change that
// premise is simply false -- `git checkout -- <path>` restores the exact bytes
// from the index. So this narrows the gate to the case where the premise
// holds, rather than removing it: an untracked file, a file with uncommitted
// work, a path outside the workspace, or anything this cannot determine stays
// behind human approval.
//
// Measured 2026-09-20 (R5): asked to remove 189 forwarding stubs -- a standing
// NO-SHIMS violation, every one of them a file whose whole body is a pointer
// elsewhere -- codeNERD was refused by the gate on the first file, and its
// workaround was to empty the files to zero lines with delete_lines. An empty
// tracked file is not a deleted file, so the end state was worse than the stub
// and the run reported honestly that a human had to finish it. The whole class
// of removal work -- dead code, superseded docs, the shim rule itself -- was
// unreachable.
//
// Scope deliberately kept small: this grants nothing except /delete_file, and
// only for a target git can restore. It is hand-built and not routed through
// codeNERD, because a model must not widen the rule that constrains it.

// gitRecoverableRunner runs a git command for the recoverability check. A
// package-level seam so tests can drive it without a real repository; the
// production value shells out non-interactively, like the verification
// runners in verify_outcome.go.
var gitRecoverableRunner = func(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	return processutil.NonInteractive(cmd).CombinedOutput()
}

// deleteRecoverablePredicate is the fact the constitution reads. Declared in
// schemas_safety.mg and joined by the permitted(/delete_file, ...) rule.
const deleteRecoverablePredicate = "file_recoverable"

// gitRestoresExactly reports whether git can restore path byte-for-byte.
//
// Two conditions, both required, and every failure answers no:
//
//   - `git ls-files -- <path>` names the file, so it is tracked.
//   - `git status --porcelain -- <path>` is empty, so there is nothing staged
//     and nothing modified in the worktree. Porcelain prints a line for any
//     difference at all, including "??" for untracked, so emptiness is exactly
//     the condition wanted and needs no column parsing.
//
// A git that is missing, a directory that is not a repository, or a path that
// escapes the workspace all return false, which leaves the deletion requiring
// human approval.
func gitRestoresExactly(ctx context.Context, workspace, path string) bool {
	workspace = strings.TrimSpace(workspace)
	path = strings.TrimSpace(path)
	if workspace == "" || path == "" {
		return false
	}

	rel, err := workspaceRelative(workspace, path)
	if err != nil {
		return false
	}

	out, err := gitRecoverableRunner(ctx, workspace, []string{"ls-files", "--", rel})
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return false
	}

	out, err = gitRecoverableRunner(ctx, workspace, []string{"status", "--porcelain", "--", rel})
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == ""
}

// workspaceRelative resolves path against workspace and refuses anything that
// escapes it. A delete gate that can be pointed outside the workspace by a
// crafted "../" target is not a gate.
func workspaceRelative(workspace, path string) (string, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workspace, path)
	}
	abs = filepath.Clean(abs)

	rel, err := filepath.Rel(filepath.Clean(workspace), abs)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", errOutsideWorkspace
	}
	return rel, nil
}

type outsideWorkspaceError struct{}

func (outsideWorkspaceError) Error() string { return "path is outside the workspace" }

var errOutsideWorkspace = outsideWorkspaceError{}

// recoverableDeleteFact returns the fact to assert for this tool call, and
// whether there is one. It answers false for every action but /delete_file.
func (e *Executor) recoverableDeleteFact(ctx context.Context, actionAtom types.MangleAtom, target string) (types.Fact, bool) {
	if string(actionAtom) != "/delete_file" {
		return types.Fact{}, false
	}
	if !gitRestoresExactly(ctx, e.workspaceForVerification(), target) {
		return types.Fact{}, false
	}
	return types.Fact{
		Predicate: deleteRecoverablePredicate,
		Args:      []any{target},
	}, true
}
