package session

import (
	"context"
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

	e.appendEvidenceReport(context.Background(), result)

	if !strings.Contains(result.Response, "Wrote 1 file(s): x.go") {
		t.Fatalf("response lacks written file ledger: %q", result.Response)
	}
	if !strings.Contains(result.Response, "Evidence: ") {
		t.Fatalf("response lost the Evidence sentence: %q", result.Response)
	}
}
