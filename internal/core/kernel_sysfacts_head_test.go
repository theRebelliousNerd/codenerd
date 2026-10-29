package core

import (
	"os/exec"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// UpdateSystemFacts publishes the full HEAD SHA as git_state(/head, Sha): the
// revision that revision-bound evidence (northstar mitigation evidence in
// prompt_northstar.mg) is compared against.
func TestUpdateSystemFacts_AssertsHeadSHA(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "seed"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("git %v: %v: %s", args, err, out)
		}
	}
	want, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}

	k, err := NewRealKernelWithWorkspace(dir)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	if err := k.UpdateSystemFacts(); err != nil {
		t.Fatalf("UpdateSystemFacts: %v", err)
	}
	rows, err := k.Query("git_state")
	if err != nil {
		t.Fatalf("Query git_state: %v", err)
	}
	for _, f := range rows {
		if len(f.Args) == 2 && types.ExtractName(f.Args[0]) == "/head" {
			if got := types.ExtractString(f.Args[1]); got != strings.TrimSpace(string(want)) {
				t.Fatalf("git_state(/head) = %q, want %q", got, strings.TrimSpace(string(want)))
			}
			return
		}
	}
	t.Fatalf("no git_state(/head, _) fact among %v", rows)
}
