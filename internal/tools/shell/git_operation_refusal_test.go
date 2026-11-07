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
	if desc := GitOperationTool().Description; !strings.Contains(desc, "fetch") || !strings.Contains(desc, "reset") || !strings.Contains(desc, "gh") {
		t.Errorf("description %q does not list every operation that runs", desc)
	}
}

func TestGitOperation_BuildsGitHubCLICommands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		operation string
		args      map[string]any
		wantCmd   string
		wantGH    bool
		wantErr   bool
	}{
		{
			name:      "gh with args",
			operation: "gh",
			args:      map[string]any{"args": "pr list --state open"},
			wantCmd:   "gh pr list --state open",
			wantGH:    true,
		},
		{
			name:      "gh prefix in operation",
			operation: "gh issue view",
			args:      map[string]any{"args": "42 --comments"},
			wantCmd:   "gh issue view 42 --comments",
			wantGH:    true,
		},
		{
			name:      "direct gh subcommand",
			operation: "pr",
			args:      map[string]any{"args": `create --title "Fix bug" --body "Details; here"`},
			wantCmd:   "gh pr create --title 'Fix bug' --body 'Details; here'",
			wantGH:    true,
		},
		{
			name:      "gh without args fails closed",
			operation: "gh",
			args:      map[string]any{},
			wantGH:    true,
			wantErr:   true,
		},
		{
			name:      "native git status is not treated as gh",
			operation: "status",
			args:      map[string]any{},
			wantGH:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, isGH, err := buildGitHubCLICommand(tc.operation, tc.args)
			if isGH != tc.wantGH {
				t.Fatalf("isGH = %v, want %v", isGH, tc.wantGH)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && cmd != tc.wantCmd {
				t.Fatalf("cmd = %q, want %q", cmd, tc.wantCmd)
			}
		})
	}
}
