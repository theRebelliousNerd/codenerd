package shell

import (
	"context"
	"strings"
	"testing"
)

// An operation git_operation does not run is refused with what it does run,
// and a read of git is pointed at the read tool that owns it, so the model's
// next call is the right one instead of the same one again.
func TestGitOperation_AnUnsupportedOperationNamesWhatRuns(t *testing.T) {
	t.Parallel()
	_, err := executeGitOperation(context.Background(), map[string]any{"operation": "diff", "args": "-- a.go"})
	if err == nil {
		t.Fatal("git_operation ran a diff; reading git is git_diff's")
	}
	for _, want := range []string{"unsupported git operation: diff", "status, add, commit, checkout, branch", "git_diff"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not say %q", err, want)
		}
	}
	_, err = executeGitOperation(context.Background(), map[string]any{"operation": "rebase"})
	if err == nil || !strings.Contains(err.Error(), "push") || strings.Contains(err.Error(), "git_diff") {
		t.Errorf("rebase refusal = %v, want the operations that run and no read tool", err)
	}
	if desc := GitOperationTool().Description; !strings.Contains(desc, "fetch") || !strings.Contains(desc, "reset") {
		t.Errorf("description %q does not list every operation that runs", desc)
	}
}
