package core

import (
	"strings"
	"testing"
)

// The run_tests and build_project handlers ran with getAllowedEnv alone, which
// reads only OS environment variables named in the execution allowlist. The
// project's own build settings are config, not environment, so they never
// arrived.
//
// On this repo that meant those two actions could never work. Observed live
// 2026-08-08, run_tests executing its default `go test ./...`:
//
//	./sqlite-vec.h:7:10: fatal error: 'sqlite3.h' file not found
//
// codeNERD's own test action could not test codeNERD, and reported the compile
// failure as a test failure.
func TestBuildToolEnvCarriesTheProjectBuildSettings(t *testing.T) {
	v := &VirtualStore{workingDir: workspaceRootForTest(t)}

	env := v.buildToolEnv()
	if len(env) == 0 {
		t.Fatal("buildToolEnv returned nothing; the test and build actions would run bare")
	}

	var hasCGO bool
	for _, e := range env {
		if strings.HasPrefix(e, "CGO_CFLAGS=") {
			hasCGO = true
			if !strings.Contains(e, "sqlite_headers") {
				t.Errorf("CGO_CFLAGS does not point at the repo's headers: %q", e)
			}
		}
	}
	if !hasCGO {
		t.Error("no CGO_CFLAGS in the build environment, so `go test ./...` cannot compile this repo")
	}
}

// The widening must not become an environment leak: buildToolEnv is a union of
// two allowlist-respecting sources, and neither may pull in the whole
// environment.
func TestBuildToolEnvDoesNotLeakArbitraryEnvironment(t *testing.T) {
	t.Setenv("NERD_SECRET_CANARY", "must-not-appear")

	v := &VirtualStore{workingDir: workspaceRootForTest(t)}
	for _, e := range v.buildToolEnv() {
		if strings.HasPrefix(e, "NERD_SECRET_CANARY=") {
			t.Fatal("an un-allowlisted environment variable reached the build environment")
		}
	}
}

// workspaceRootForTest returns the repository root, which is two levels up from
// internal/core.
func workspaceRootForTest(t *testing.T) string {
	t.Helper()
	return "../.."
}
