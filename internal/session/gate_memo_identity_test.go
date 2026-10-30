package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The memo key names what the gate's obligations follow, not only the bytes
// of the turn's packages: rewriting a sibling test with its own bytes adds a
// written path (and with it a tagged-test obligation and a coverage scope)
// without changing a byte, and a different preimage is a different baseline.
func TestGateKey_NamesWrittenMembershipAndPreimages(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	for rel, body := range map[string]string{
		"pkg/a.go":                "package pkg\n",
		"pkg/integration_test.go": "//go:build integration\n\npackage pkg\n",
	} {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result := mutationResult()
	result.WrittenPaths = []string{"pkg/a.go"}
	result.PreWriteContents = map[string]PreImage{"pkg/a.go": existed("package pkg // before\n")}
	before := gateKey("own+coverage", ws, result)

	// Same bytes, one more written path: the sibling was already hashed.
	result.WrittenPaths = append(result.WrittenPaths, "pkg/integration_test.go")
	result.PreWriteContents["pkg/integration_test.go"] = existed("//go:build integration\n\npackage pkg\n")
	if gateKey("own+coverage", ws, result) == before {
		t.Fatal("a same-byte write that adds a written test left the memo key unchanged")
	}

	other := mutationResult()
	other.WrittenPaths = []string{"pkg/a.go"}
	other.PreWriteContents = map[string]PreImage{"pkg/a.go": existed("package pkg // another past\n")}
	if gateKey("own+coverage", ws, other) == before {
		t.Fatal("a different preimage (a different baseline) kept the memo key")
	}
}

// An importer's files are part of what its run reads: a change to an
// importer's test outside the turn's packages is a new key. A listing that
// failed is not "nothing imports this", and is not kept.
func TestGateTests_ImporterIdentityAndFailedDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := importerModule(t, aBefore)
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}
	launches := countTestLaunches(t)

	gateTests(context.Background(), ws, result, false)
	if result.ImporterCheck.Verdict() != VerifyPassed {
		t.Fatalf("setup: importers = %s (%s)", result.ImporterCheck.Verdict(), result.ImporterCheck.Output)
	}
	measured := *launches

	// An edit to the importer's test, which no write tool recorded.
	failing := "package b\n\nimport \"testing\"\n\nfunc TestOutside(t *testing.T) { t.Fatal(\"changed outside the turn\") }\n"
	if err := os.WriteFile(filepath.Join(ws, "b", "outside_test.go"), []byte(failing), 0o644); err != nil {
		t.Fatal(err)
	}
	gateTests(context.Background(), ws, result, false)
	if *launches == measured {
		t.Fatal("an importer changed outside the turn's packages and its earlier verdict was reused")
	}
	if result.ImporterCheck.Verdict() != VerifyFailed {
		t.Fatalf("importers after the outside edit = %s, want failed", result.ImporterCheck.Verdict())
	}

	// A discovery that does not finish is reported, not remembered as a skip.
	real := verifyBuildRunner
	broken := true
	verifyBuildRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
		if broken && len(args) > 0 && args[0] == "list" {
			return []byte("go: module cache unavailable"), context.DeadlineExceeded
		}
		return real(ctx, dir, env, name, args)
	}
	t.Cleanup(func() { verifyBuildRunner = real })
	if err := os.WriteFile(filepath.Join(ws, "b", "outside_test.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gateTests(context.Background(), ws, result, false)
	if result.ImporterCheck.Verdict() != VerifySkipped || result.ImporterCheck.Reason == "nothing imports what this turn wrote" {
		t.Fatalf("a failed listing = %s %q; want a skip that says the listing failed", result.ImporterCheck.Verdict(), result.ImporterCheck.Reason)
	}
	broken = false
	gateTests(context.Background(), ws, result, false)
	if result.ImporterCheck.Verdict() != VerifyPassed {
		t.Fatalf("after the listing recovered importers = %s (%s); the failed discovery was reused", result.ImporterCheck.Verdict(), result.ImporterCheck.Reason)
	}
}
