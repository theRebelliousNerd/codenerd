package shell

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// run_tests used to spawn `go test` with the process environment untouched,
// while the session's own verification gate ran the same tests under
// internal/build's environment. The model and the gate could then disagree
// about one tree -- a project needing CGO headers failed for the model and
// passed for the gate -- and the test binary, which is project code, inherited
// every variable the process held, API keys included.
//
// The probe module's own tests are the assertion: they fail inside the child
// process when it sees the parent's secret, when it lacks the test env's
// GOTRACEBACK=all, or when the workspace's build.go_flags were not applied.
// Not parallel: it sets process environment.
func TestTypedVerification_GoTestRunsUnderTheBuildEnv(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no Go toolchain on PATH")
	}
	root := goWorkspace(t)
	probe := filepath.Join(root, "probe")
	if err := os.MkdirAll(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProbe := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(probe, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeProbe("env_test.go", `package probe

import (
	"os"
	"testing"
)

func TestEnvironment(t *testing.T) {
	if v := os.Getenv("NERD_PROBE_SECRET_API_KEY"); v != "" {
		t.Fatalf("the parent's secret reached the test binary: %q", v)
	}
	if got := os.Getenv("GOTRACEBACK"); got != "all" {
		t.Fatalf("GOTRACEBACK = %q, want all (internal/build's test env)", got)
	}
}
`)
	writeProbe("flags_test.go", `//go:build !nerdprobe

package probe

import "testing"

func TestConfiguredGoFlags(t *testing.T) {
	t.Fatal("build.go_flags from .nerd/config.json were not applied (no -tags=nerdprobe)")
}
`)
	nerd := filepath.Join(root, ".nerd")
	if err := os.MkdirAll(nerd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nerd, "config.json"),
		[]byte(`{"build":{"go_flags":["-tags=nerdprobe"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("NERD_PROBE_SECRET_API_KEY", "sk-must-not-leak")

	out, err := executeTypedVerification(shellWsCtx(root), map[string]any{
		"packages": []any{"./probe"},
	}, true)
	if err != nil {
		t.Fatalf("run_tests failed: %v\n%s", err, out)
	}
	var result struct {
		Argv     []string `json:"argv"`
		ExitCode int      `json:"exit_code"`
		Output   string   `json:"output"`
	}
	if jerr := json.Unmarshal([]byte(out), &result); jerr != nil {
		t.Fatalf("result is not JSON: %v\n%s", jerr, out)
	}
	if result.ExitCode != 0 {
		t.Fatalf("probe tests failed under run_tests (exit %d), argv %v:\n%s", result.ExitCode, result.Argv, result.Output)
	}
}
