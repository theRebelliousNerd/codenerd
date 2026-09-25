package gates

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func goModule(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	root := t.TempDir()
	files["go.mod"] = "module example.com/m\n\ngo 1.21\n"
	writeFiles(t, root, files)
	return root
}

// A gate's verdict is what the process said: a failing test fails the gate
// and names the test; the passing package passes.
func TestRun_GoTestVerdictAndFindings(t *testing.T) {
	root := goModule(t, map[string]string{
		"good/good.go":      "package good\n\nfunc One() int { return 1 }\n",
		"good/good_test.go": "package good\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) { if One() != 1 { t.Fatal(\"no\") } }\n",
		"bad/bad.go":        "package bad\n\nfunc Two() int { return 3 }\n",
		"bad/bad_test.go":   "package bad\n\nimport \"testing\"\n\nfunc TestTwo(t *testing.T) { if got := Two(); got != 2 { t.Fatalf(\"Two() = %d\", got) } }\n",
	})
	g := Gate{ID: "go:test", Kind: Test, Argv: []string{"go", "test", "-count=1", "-cover", PkgToken}, Scope: ScopeNode}

	good := Run(context.Background(), root, g, "good")
	if !good.Passed || good.Unverified() || good.ExitCode != 0 {
		t.Fatalf("good package: %+v", good)
	}
	if fs := Findings(root, good); len(fs) != 0 {
		t.Fatalf("a pass has no findings: %+v", fs)
	}
	if cov, ok := Coverage(good.Output); !ok || cov != 10000 {
		t.Fatalf("a -cover run reports the node's coverage: %d, %v\n%s", cov, ok, good.Output)
	}

	bad := Run(context.Background(), root, g, "bad")
	if bad.Passed || bad.Unverified() || bad.ExitCode == 0 {
		t.Fatalf("bad package must fail with a verdict: %+v", bad)
	}
	fs := Findings(root, bad)
	if len(fs) != 1 || fs[0].Target != "bad::TestTwo" || fs[0].Kind != Test || fs[0].Node != "bad" {
		t.Fatalf("want one finding for bad::TestTwo, got %+v\noutput:\n%s", fs, bad.Output)
	}
}

// A build break is a finding per diagnostic, keyed by file and message.
func TestRun_GoBuildBreakNamesTheFile(t *testing.T) {
	root := goModule(t, map[string]string{
		"lib/lib.go": "package lib\n\nfunc F() int { return undefinedThing }\n",
	})
	g := Gate{ID: "go:build", Kind: Build, Argv: []string{"go", "build", "./..."}, Scope: ScopeAll}
	r := Run(context.Background(), root, g, "")
	fs := Findings(root, r)
	if r.Passed || len(fs) != 1 || fs[0].Target != "lib/lib.go" || !strings.Contains(fs[0].Message, "undefinedThing") {
		t.Fatalf("want lib/lib.go undefinedThing, got %+v\noutput:\n%s", fs, r.Output)
	}
}

// A gate that cannot start, or that the loop stopped, is unverified: no
// verdict, no findings, never a pass.
func TestRun_UnstartableOrStoppedIsUnverified(t *testing.T) {
	root := t.TempDir()
	missing := Run(context.Background(), root, Gate{ID: "x", Kind: Test, Argv: []string{"definitely-not-a-program-nerd"}}, "")
	if !missing.Unverified() || missing.Passed || Findings(root, missing) != nil {
		t.Fatalf("missing program: %+v", missing)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root2 := goModule(t, map[string]string{"a/a.go": "package a\n"})
	stopped := Run(ctx, root2, Gate{ID: "go:build", Kind: Build, Argv: []string{"go", "build", "./..."}}, "")
	if !stopped.Unverified() || stopped.Passed {
		t.Fatalf("stopped run: %+v", stopped)
	}
}

// Gates run project code; the environment is an allowlist, so a credential
// the harness holds does not reach a project's tests.
func TestAllowedEnv_KeepsToolchainVarsDropsTheRest(t *testing.T) {
	t.Setenv("NERD_GATE_TEST_API_KEY", "sk-secret")
	t.Setenv("VIRTUAL_ENV", "/venv")
	env := allowedEnv(t.TempDir())
	if slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "NERD_GATE_TEST_API_KEY=") }) {
		t.Fatal("an unlisted variable reached the gate's environment")
	}
	if !slices.Contains(env, "VIRTUAL_ENV=/venv") {
		t.Fatalf("a toolchain variable must pass through: %v", env)
	}
}

func TestBoundedBuffer_KeepsHeadAndTail(t *testing.T) {
	b := &boundedBuffer{limit: 20}
	for _, chunk := range []string{"HEAD-0123", "45", strings.Repeat("m", 100), "tail-END"} {
		if _, err := b.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	got := b.String()
	if !strings.HasPrefix(got, "HEAD-01234") || !strings.HasSuffix(got, "mmtail-END") || !strings.Contains(got, "bytes dropped") {
		t.Fatalf("bounded output = %q", got)
	}
	small := &boundedBuffer{limit: 100}
	if _, err := small.Write([]byte("all of it")); err != nil {
		t.Fatal(err)
	}
	if small.String() != "all of it" {
		t.Fatalf("under the limit nothing is dropped: %q", small.String())
	}
}
