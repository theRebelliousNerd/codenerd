package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// checkGate builds a tiny test gate: it passes when <dir>/value.txt says ok.
// A compiled program so the test runs the same on every platform.
func checkGate(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain to build the gate")
	}
	dir := t.TempDir()
	src := "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"strings\"\n)\n\nfunc main() {\n\tb, _ := os.ReadFile(filepath.Join(os.Args[1], \"value.txt\"))\n\tif strings.TrimSpace(string(b)) != \"ok\" {\n\t\tfmt.Println(os.Args[1] + \"/value.txt:1: want ok\")\n\t\tos.Exit(1)\n\t}\n}\n"
	for name, body := range map[string]string{"go.mod": "module checkgate\n\ngo 1.21\n", "main.go": src} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(dir, "checkgate")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build gate: %v\n%s", err, out)
	}
	return bin
}

func workspaceWithGate(t *testing.T, gateLine string) string {
	t.Helper()
	ws := t.TempDir()
	files := map[string]string{
		"svc/app.py":    "def f():\n    return 1\n",
		"svc/value.txt": "bad\n",
		"nerd.md":       "---\nschema: nerd/v1\ngates:\n" + gateLine + "---\n",
	}
	for rel, body := range files {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

// A non-Go write in a workspace that declares how it tests is settled by the
// workspace's gate, over the directory written -- not by a run the model
// picks -- and the gate's verdict is the /test_run gate's.
func TestWorkspaceTestRun_TheWorkspacesGateDecidesANonGoWrite(t *testing.T) {
	bin := checkGate(t)
	ws := workspaceWithGate(t, "  - id: check\n    kind: test\n    run: >-\n      '"+filepath.ToSlash(bin)+"' {node}\n    scope: node\n")
	e := &Executor{}
	e.SetConfig(ExecutorConfig{WorkspaceRoot: ws})
	result := &ExecutionResult{WrittenPaths: []string{"svc/app.py"}}

	ran, out := e.workspaceTestRun(context.Background(), result)
	if !ran || result.testRunVerdict() != VerifyFailed {
		t.Fatalf("a red workspace gate fails the test run: ran=%v verdict=%v\n%s", ran, result.testRunVerdict(), out)
	}
	if err := os.WriteFile(filepath.Join(ws, "svc", "value.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result.TestRunSinceLastWrite = nil
	if ran, out := e.workspaceTestRun(context.Background(), result); !ran || result.testRunVerdict() != VerifyPassed {
		t.Fatalf("a green workspace gate passes it: ran=%v verdict=%v\n%s", ran, result.testRunVerdict(), out)
	}
}

// Go writes keep the Go gates; a language the workspace has no test gate for,
// or a gate whose program is missing, leaves the run to the model as before.
func TestWorkspaceTestRun_LeavesWhatItCannotDecide(t *testing.T) {
	bin := checkGate(t)
	for name, tc := range map[string]struct {
		gate    string
		written []string
	}{
		"go write": {
			gate:    "  - id: check\n    kind: test\n    run: >-\n      '" + filepath.ToSlash(bin) + "' {node}\n    scope: node\n",
			written: []string{"svc/main.go", "README.md"},
		},
		"no gate for the language": {
			gate:    "  - id: check\n    kind: test\n    run: npx vitest run {node}\n    scope: node\n",
			written: []string{"svc/app.py"},
		},
		"toolchain missing": {
			gate:    "  - id: check\n    kind: test\n    run: definitely-not-a-test-runner {node}\n    scope: node\n",
			written: []string{"svc/app.py"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ws := workspaceWithGate(t, tc.gate)
			e := &Executor{}
			e.SetConfig(ExecutorConfig{WorkspaceRoot: ws})
			result := &ExecutionResult{WrittenPaths: tc.written}
			if ran, _ := e.workspaceTestRun(context.Background(), result); ran || result.TestRunSinceLastWrite != nil {
				t.Fatalf("the executor must not decide this turn's test run: ran=%v run=%+v", ran, result.TestRunSinceLastWrite)
			}
		})
	}
}
