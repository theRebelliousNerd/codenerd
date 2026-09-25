package build

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A test subcommand runs under the test env (GOTRACEBACK=all, -count=1 folded
// into GOFLAGS); a build does not. Both carry the workspace's go_flags right
// after the subcommand, where package patterns cannot shadow them.
func TestGoInvocation_WhenTestOrBuild_ShouldPickTheMatchingEnvAndInjectConfiguredFlags(t *testing.T) {
	ws := t.TempDir()
	mustMkdir(t, filepath.Join(ws, ".git"))
	mustMkdir(t, filepath.Join(ws, ".nerd"))
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"),
		[]byte(`{"build":{"go_flags":["-mod=mod"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	testEnv, testArgv := GoInvocation(ws, ws, []string{"test", "-count=3", "./..."})
	if got := envValue(testEnv, "GOTRACEBACK"); got != "all" {
		t.Errorf("test env GOTRACEBACK = %q, want all", got)
	}
	if want := []string{"test", "-mod=mod", "-count=3", "./..."}; !slices.Equal(testArgv, want) {
		t.Errorf("test argv = %v, want %v", testArgv, want)
	}

	buildEnv, buildArgv := GoInvocation(ws, ws, []string{"build", "./..."})
	if hasEnvKey(buildEnv, "GOTRACEBACK") {
		t.Errorf("build env carries the test-only GOTRACEBACK: %v", SummarizeEnv(buildEnv))
	}
	if want := []string{"build", "-mod=mod", "./..."}; !slices.Equal(buildArgv, want) {
		t.Errorf("build argv = %v, want %v", buildArgv, want)
	}
}

// The header walk stops at the workspace. A sqlite_headers directory above the
// workspace root belongs to some other tree; adopting it would compile the
// workspace against headers it never declared.
func TestGoInvocation_WhenHeadersSitAboveTheWorkspace_ShouldNotAdoptThem(t *testing.T) {
	outer := t.TempDir()
	mustMkdir(t, filepath.Join(outer, "sqlite_headers"))
	ws := filepath.Join(outer, "ws")
	sub := filepath.Join(ws, "services", "indexer")
	mustMkdir(t, sub)

	// Premise: the unbounded walk from the module does climb out to outer.
	if got := DetectionRootFor(sub); got != outer {
		t.Fatalf("premise: DetectionRootFor(%q) = %q, want %q", sub, got, outer)
	}

	env, _ := GoInvocation(ws, sub, []string{"build", "./..."})
	if got := envValue(env, "CGO_CFLAGS"); strings.Contains(got, filepath.Join(outer, "sqlite_headers")) {
		t.Errorf("CGO_CFLAGS = %q adopts headers above the workspace root", got)
	}
}

// A config that cannot be loaded does not stop the command; it runs under the
// unconfigured build env.
func TestGoInvocation_WhenConfigIsInvalid_ShouldStillReturnTheBuildEnv(t *testing.T) {
	ws := t.TempDir()
	mustMkdir(t, filepath.Join(ws, ".nerd"))
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(`{not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if cfg := WorkspaceUserConfig(ws); cfg != nil {
		t.Fatalf("WorkspaceUserConfig on invalid JSON = %+v, want nil", cfg)
	}
	env, argv := GoInvocation(ws, ws, []string{"test", "./..."})
	if !hasEnvKey(env, "PATH") {
		t.Errorf("no PATH in the fallback env: %v", SummarizeEnv(env))
	}
	if want := []string{"test", "./..."}; !slices.Equal(argv, want) {
		t.Errorf("argv = %v, want %v unchanged", argv, want)
	}
}
