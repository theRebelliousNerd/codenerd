package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The executor's ledger names what the turn wrote, so a final answer that
// denies it contradicts the response itself.
func TestCloseChangeEvidence_ResponseNamesWrittenFiles(t *testing.T) {
	e, result := verifyGateExecutor(t)
	result.SuccessfulWriteTools = 1
	result.WrittenPaths = []string{"x.go"}
	result.Response = "did stuff"

	e.closeAcceptanceEvidence(context.Background(), result)
	e.appendEvidenceSummary(result)

	if !strings.Contains(result.Response, "Wrote 1 file(s): x.go") {
		t.Fatalf("response lacks written file ledger: %q", result.Response)
	}
	if !strings.Contains(result.Response, "Evidence: ") {
		t.Fatalf("response lost the Evidence sentence: %q", result.Response)
	}
}

func TestCloseChangeEvidence_TagGatedPackageIsNotAFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("needs go toolchain")
	}
	e, result := verifyGateExecutor(t)
	ws := e.config.WorkspaceRoot
	before, err := snapshotForTest(ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "e2e"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module example.com/fixture\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "ok.go"), []byte("package fixture\n\nfunc OK() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "e2e", "gated_test.go"), []byte("//go:build integration\n\npackage e2e\n\nimport \"testing\"\n\nfunc TestGated(t *testing.T) { t.Log(\"gated\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result.SuccessfulWriteTools = 1
	result.WrittenPaths = []string{"e2e/gated_test.go"}
	result.Response = "did stuff"

	// Must stay on the same helper the post-edit gate uses, or a tag-gated
	// package fails the turn twice over.
	if err := e.closeChangeEvidence(context.Background(), result, before); err != nil {
		t.Fatalf("closeChangeEvidence failed for tag-gated package: %v", err)
	}
	if result.TestCheck.Verdict() == VerifyFailed {
		t.Fatalf("tag-gated package must not be VerifyFailed: %+v", result.TestCheck)
	}
}
