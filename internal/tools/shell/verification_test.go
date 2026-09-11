package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goWorkspace returns a workspace root holding a minimal Go module so the
// typed runner resolves to `go build` / `go test` without running anything
// the assertions below do not reach.
func goWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module verification_probe\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// The typed runners exist so the model never hands a string to a shell.
// A `command` argument is refused before the working directory is even
// resolved, so the refusal cannot be bypassed by a valid directory.
func TestTypedVerification_RefusesCustomCommand(t *testing.T) {
	t.Parallel()
	root := goWorkspace(t)
	for _, tests := range []bool{false, true} {
		_, err := executeTypedVerification(shellWsCtx(root), map[string]any{
			"working_dir": root,
			"command":     "echo pwned",
		}, tests)
		if err == nil || !strings.Contains(err.Error(), "custom commands are not accepted") {
			t.Fatalf("tests=%v: want custom-command refusal, got %v", tests, err)
		}
	}
}

// Packages are workspace-relative Go import paths, never absolute paths,
// parent escapes, or shell metacharacters.
func TestTypedVerification_RejectsPackagesOutsideWorkspace(t *testing.T) {
	t.Parallel()
	root := goWorkspace(t)
	for _, pkg := range []string{"/abs/pkg", "../escape", "./../escape", "internal/x", "./ok\x00bad"} {
		_, err := executeTypedVerification(shellWsCtx(root), map[string]any{
			"working_dir": root,
			"packages":    []any{pkg},
		}, true)
		if err == nil {
			t.Fatalf("package %q must be refused", pkg)
		}
	}
	_, err := executeTypedVerification(shellWsCtx(root), map[string]any{
		"working_dir": root,
		"packages":    []any{"./ok", 7},
	}, true)
	if err == nil || !strings.Contains(err.Error(), "packages must contain strings") {
		t.Fatalf("non-string package entry must be refused, got %v", err)
	}
}

// A pattern is data: it is bounded and may not carry a NUL, and a timeout
// outside the sane window is refused rather than clamped silently.
func TestTypedVerification_ValidatesPatternAndTimeout(t *testing.T) {
	t.Parallel()
	root := goWorkspace(t)
	if _, err := executeTypedVerification(shellWsCtx(root), map[string]any{
		"working_dir": root,
		"pattern":     "Test\x00Bad",
	}, true); err == nil || !strings.Contains(err.Error(), "invalid test pattern") {
		t.Fatalf("NUL in pattern must be refused, got %v", err)
	}
	if _, err := executeTypedVerification(shellWsCtx(root), map[string]any{
		"working_dir": root,
		"pattern":     strings.Repeat("a", 4097),
	}, true); err == nil || !strings.Contains(err.Error(), "invalid test pattern") {
		t.Fatalf("oversize pattern must be refused, got %v", err)
	}
	for _, timeout := range []any{0, -5, 86401, "soon"} {
		if _, err := executeTypedVerification(shellWsCtx(root), map[string]any{
			"working_dir":     root,
			"timeout_seconds": timeout,
		}, false); err == nil || !strings.Contains(err.Error(), "invalid timeout_seconds") {
			t.Fatalf("timeout %v must be refused, got %v", timeout, err)
		}
	}
}

// A count is bounded test repetition: it must be an integer in range.
func TestTypedVerification_ValidatesCount(t *testing.T) {
	t.Parallel()
	root := goWorkspace(t)
	for _, count := range []any{0, -1, 1001, "many"} {
		if _, err := executeTypedVerification(shellWsCtx(root), map[string]any{
			"working_dir": root,
			"count":       count,
		}, true); err == nil || !strings.Contains(err.Error(), "invalid count") {
			t.Fatalf("count %v must be refused, got %v", count, err)
		}
	}
}

// A directory with no recognised build system has no runner; the tool says
// so instead of guessing a command.
func TestTypedVerification_NoRunnerIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := executeTypedVerification(shellWsCtx(root), map[string]any{"working_dir": root}, false)
	if err == nil || !strings.Contains(err.Error(), "no supported project verification runner") {
		t.Fatalf("want no-runner error, got %v", err)
	}
}
