package campaign

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
)

const (
	pickA      = "package a\n\n// A is 1 when b.\nfunc A(b bool) int {\n\tif b == true {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	pickAFixed = "package a\n\n// A is 1 when b.\nfunc A(b bool) int {\n\tif b {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	pickB      = "package b\n\n// B is 2 when b.\nfunc B(b bool) int {\n\tif b == true {\n\t\treturn 2\n\t}\n\treturn 0\n}\n"
	// The fix for b clears its S1002 and writes two if-return-bool blocks
	// staticcheck simplifies (S1008): staticcheck reports more in b
	// than before, though the witness for S1002 passes.
	pickBWorse = "package b\n\n// B is 2 when b.\nfunc B(b bool) int {\n\tif b {\n\t\treturn 2\n\t}\n\treturn 0\n}\n\n// C is b.\nfunc C(b bool) bool {\n\tif b {\n\t\treturn true\n\t}\n\treturn false\n}\n\n// D is not b.\nfunc D(b bool) bool {\n\tif !b {\n\t\treturn true\n\t}\n\treturn false\n}\n"
)

// Rung R7's loop: the error-free gates' findings are the work. Two packages
// each hold one staticcheck S1002 finding, which none of the workspace's own
// gates (build, vet, test) report. The policy picks a's group; its attempt is
// briefed with the symptom and carries `staticcheck -checks S1002 ./a` as its
// witness; the witness passes and nothing got worse, so it is kept. b's fix
// clears its finding but introduces two others: the witness passes and the
// cycle is reverted because staticcheck got worse. The after-measures are
// scoped to the package each attempt touched.
func TestRecurseCycles_PicksErrorFreeFindingsAndProvesEachWithItsWitness(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("staticcheck"); err != nil {
		t.Skip("staticcheck is not installed")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	root := gitFixture(t, map[string]string{
		"go.mod":     "module example.com/r7\n\ngo 1.22\n",
		".gitignore": ".nerd/\n",
		"a/a.go":     pickA,
		"b/b.go":     pickB,
	}).root

	var mu sync.Mutex
	var scopes [][]string
	measure := func(ctx context.Context, packages []string) gates.Snapshot {
		mu.Lock()
		scopes = append(scopes, packages)
		mu.Unlock()
		return gates.MeasureErrorFree(ctx, root, gates.MeasureOptions{Only: []string{"staticcheck", "gofmt"}, ScopedPackages: packages})
	}
	var fixes []RecurseAttempt
	execute := onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		fixes = append(fixes, a)
		switch a.Node.ID {
		case "a":
			write(t, root, "a/a.go", pickAFixed)
		case "b":
			write(t, root, "b/b.go", pickBWorse)
		}
		return nil
	})
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var progress strings.Builder
	res, err := RunRecurseCycles(context.Background(), RecurseCycleConfig{
		Workspace: root, Kernel: k, Execute: ownWrites(root, execute), Passes: 1, Progress: &progress,
		MeasureGates: measure,
	})
	t.Logf("progress:\n%s", progress.String())
	if err != nil {
		t.Fatal(err)
	}

	if len(fixes) != 2 {
		t.Fatalf("fix attempts = %d (%+v), want one per package's S1002 group", len(fixes), fixes)
	}
	first := fixes[0]
	if first.Node.ID != "a" || !slices.Equal(first.Check, []string{"staticcheck", "-checks", "S1002", "./a"}) {
		t.Fatalf("first attempt = node %s check %v, want a's group with its witness", first.Node.ID, first.Check)
	}
	if !strings.Contains(first.Evidence, "a/a.go:5:") || !strings.Contains(first.Evidence, "What should hold: `staticcheck -checks S1002 ./a` exits 0") {
		t.Fatalf("the brief is not the symptom and what should hold:\n%s", first.Evidence)
	}

	r := ratchets(journalOf(t, root))
	if len(r) != 2 || r[0].Outcome != outcomeKept || r[1].Outcome != outcomeReverted {
		t.Fatalf("ratchets = %+v, want a kept then b reverted", r)
	}
	if !strings.Contains(r[1].Detail, "error-free gates worse: staticcheck") {
		t.Fatalf("b's revert does not name the gate that got worse: %q", r[1].Detail)
	}
	if mustRead(t, root, "a/a.go") != pickAFixed || mustRead(t, root, "b/b.go") != pickB {
		t.Fatal("the tree is not a's fix kept and b's attempt reverted")
	}
	if res.Kept != 1 {
		t.Fatalf("result = %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(scopes) != 3 || scopes[0] != nil || !slices.Equal(scopes[1], []string{"./a"}) || !slices.Equal(scopes[2], []string{"./b"}) {
		t.Fatalf("measurements = %v, want one full, then scoped to ./a and to ./b", scopes)
	}
}
