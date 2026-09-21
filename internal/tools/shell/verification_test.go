package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
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

// A build checks that the code compiles and changes nothing on disk — not
// even for a single main package, where plain `go build` would drop an
// executable next to the sources. Observed 2026-09-19: run_build on
// ./cmd/nerd left a new nerd.exe in the repository root and rotated the
// running binary aside as nerd.exe~.
func TestTypedVerification_BuildLeavesNothingOnDisk(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module buildclean\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[string]bool {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		for _, e := range entries {
			names[e.Name()] = true
		}
		return names
	}
	for _, pkgs := range []any{nil, []any{"."}} {
		args := map[string]any{"working_dir": root}
		if pkgs != nil {
			args["packages"] = pkgs
		}
		before := snapshot()
		if _, err := executeTypedVerification(shellWsCtx(root), args, false); err != nil {
			t.Fatalf("packages %v: %v", pkgs, err)
		}
		after := snapshot()
		for name := range after {
			if !before[name] {
				t.Fatalf("packages %v: build left %q behind", pkgs, name)
			}
		}
		if len(after) != len(before) {
			t.Fatalf("packages %v: workspace changed: before %v after %v", pkgs, before, after)
		}
	}
}

// A test call with no packages used to run "./...". On a repository whose
// suite outlasts the tool's time limit that call cannot succeed; a
// documentation task held its phase for the whole limit that way (ledger P10,
// 2026-09-21). With a session's write set in the context, the default is the
// packages the turn wrote.
func TestTypedVerification_TestsDefaultToTheWrittenPackages(t *testing.T) {
	t.Parallel()
	root := goWorkspace(t)
	if err := os.MkdirAll(filepath.Join(root, "internal", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := tools.WithTestScope(shellWsCtx(root), func() []string {
		return []string{"./internal/alpha", "./internal/deleted_this_turn"}
	})

	got, scoped := writtenTestScope(ctx, root, true)
	if !scoped || len(got) != 1 || got[0] != "./internal/alpha" {
		t.Fatalf("writtenTestScope = %v, scoped=%v; want [./internal/alpha], true", got, scoped)
	}
	// A build has no written-package default: it compiles the module.
	if _, scoped := writtenTestScope(ctx, root, false); scoped {
		t.Error("a build was scoped to the write set")
	}
	// Below the workspace root "./..." is already the caller's narrowing, and
	// the write set's workspace-relative packages would not resolve there.
	if _, scoped := writtenTestScope(ctx, filepath.Join(root, "internal", "alpha"), true); scoped {
		t.Error("a working_dir below the root was scoped to workspace-relative packages")
	}
	// No session, no write set: the old default stands.
	if _, scoped := writtenTestScope(shellWsCtx(root), root, true); scoped {
		t.Error("a call with no session scope was scoped")
	}
}

// Nothing written means nothing runs, and the result cannot be read as a pass:
// it has no exit_code, and says how to ask for more.
func TestTypedVerification_NothingWrittenRunsNothing(t *testing.T) {
	t.Parallel()
	root := goWorkspace(t)
	ctx := tools.WithTestScope(shellWsCtx(root), func() []string { return nil })

	out, err := executeTypedVerification(ctx, map[string]any{}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"ran":false`) || strings.Contains(out, "exit_code") {
		t.Errorf("result can be mistaken for a test run: %s", out)
	}
	if !strings.Contains(out, `./...`) {
		t.Errorf("result does not say how to run the whole module: %s", out)
	}
}
