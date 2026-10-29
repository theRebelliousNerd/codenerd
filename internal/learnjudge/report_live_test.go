//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"testing"
)

// TestLiveFinalReport probes the final-report questions over the reports that
// motivated the /finish round and over done reports with caveats.
func TestLiveFinalReport(t *testing.T) {
	c, _ := liveClient(t, "final_report")
	const atoms = "fix whatever system produced the oversized prompt atoms, and then fix the existing files in .nerd/agents too. add tests."
	cases := []struct {
		name, task, report  string
		unfinished, blocked bool
	}{
		{"not-done-mistaken", atoms, "Done: generator now enforces small single-concern atoms in code with atomicity tests added. No .nerd/agents files are checked into the repo, so existing-file remediation runs through the re-atomize path on the next init in a workspace that has them.", true, false},
		{"commit-denied", atoms + " then commit it.", "Fixed oversized prompt atoms in place. 21 files now capped. Tests pass. Commit blocked: git add was denied by shell-effect gate (mutating without task-scope auth) despite brief authorizing commit. Please approve/commit with: git add ... && git commit", true, true},
		{"n41-budget", "delete the 188 dead feature flags", "I removed 11 dead flags and their tests. I cannot execute the remaining 177 deletions in this turn. Requesting re-invocation.", true, false},
		{"done-caveat", "make Max return 0 for an empty slice", "Done. Max returns 0 for an empty slice and TestMaxEmpty covers it. Note: I could not run the race detector here; you may want to run go test -race.", false, false},
		{"done-followup", "add a /stats command", "Added /stats with tests; all pass. A follow-up could add a JSON output flag.", false, false},
		{"missing-input", "deploy the staging config", "I prepared the staging manifest, but the deploy needs the STAGING_TOKEN credential, which is not available to me. Nothing was deployed.", true, true},
		{"decision-needed", "pick a database and wire it in", "There are two good options, Postgres and SQLite, with different tradeoffs. Which one do you want? I have not wired either yet.", true, true},
		{"skipped-part", "fix the parser bug and update the docs", "Fixed the parser bug in parse.go with a regression test. I did not get to the docs.", true, false},
	}
	tally := map[string][3]int{}
	var misses []string
	for _, tc := range cases {
		got, err := JudgeReport(probeLabel(context.Background(), tc.name), c, ReportInput{Task: tc.task, Report: tc.report})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		scoreLine(tally, &misses, tc.name, ReportAdmitsUnfinished, tc.report, got.Unfinished, tc.unfinished)
		scoreLine(tally, &misses, tc.name, ReportNamesBlocker, tc.report, got.Blocked, tc.blocked)
		t.Logf("%-18s unfinished=%.2f blocked=%.2f", tc.name, got.Unfinished, got.Blocked)
	}
	lines := append([]string{fmt.Sprintf("final-report probe: %d cases, thresholds yes>=0.70 no<=0.30", len(cases))}, tallyLines(tally)...)
	probeSummary(t, "final_report", append(append(lines, "misses (x wrong, ~ unsettled):"), misses...))
}
