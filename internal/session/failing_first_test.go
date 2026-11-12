package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/testfacts"
	"codenerd/internal/types"
)

// While the test a repair round is clearing still fails, the round's recheck
// reruns that test alone and goes back to the model: the rest of the suite
// does not run again until it passes, and then it does. Live 2026-11-12: each
// round of a self-repair fix turn reran 6089 importer tests for >10 minutes
// while its one failing test failed the same way.
func TestRepairLoop_RecheckRunsTheFailingTestsFirst(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 3 })
	marker := filepath.Join(t.TempDir(), "other_runs")
	otherTest := fmt.Sprintf("package main\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestOther(t *testing.T) {\n\tf, err := os.OpenFile(%q, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\t_, _ = f.WriteString(\"x\")\n\t_ = f.Close()\n}\n", marker)
	if err := os.WriteFile(filepath.Join(h.ws, "other_test.go"), []byte(otherTest), 0o644); err != nil {
		t.Fatal(err)
	}
	otherRuns := func() int {
		data, _ := os.ReadFile(marker)
		return len(data)
	}
	probe := filepath.Join(h.ws, "main_test.go")
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", probe, repairTestBroken),
		}}
	}
	var atRepair []int
	h.onRepair = func(int, []types.Message) *types.LLMToolResponse {
		atRepair = append(atRepair, otherRuns())
		content := strings.Replace(repairTestBroken, "broken", fmt.Sprintf("still broken %d", len(atRepair)), 1)
		if len(atRepair) >= 3 {
			content = repairTestFixed
		}
		return &types.LLMToolResponse{Text: "trying", ToolCalls: []types.ToolCall{
			h.writeCall(fmt.Sprintf("r%d", len(atRepair)), "write_file", probe, content),
		}}
	}
	result, err := h.drive(t, "fix the probe test")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if rec := result.TestCheck.Repair; rec == nil || !rec.Passed || rec.Cost.Attempts != 3 {
		t.Fatalf("repair record = %+v, want convergence on the third attempt", rec)
	}
	if len(atRepair) < 3 || atRepair[0] == 0 {
		t.Fatalf("TestOther runs at each repair call = %v, want the first gate to have run it", atRepair)
	}
	if atRepair[1] != atRepair[0] || atRepair[2] != atRepair[0] {
		t.Fatalf("TestOther runs at each repair call = %v: a recheck reran the whole suite while TestProbe still failed", atRepair)
	}
	if otherRuns() <= atRepair[2] {
		t.Fatalf("TestOther never ran once TestProbe passed (%d runs): the full gate was skipped", otherRuns())
	}
}

// Only the failures the turn has to clear are rerun first: one that failed
// before the turn, or did not reproduce, would hold the full gate back
// forever. Subtests rerun their parent, one -run per package.
func TestFailingFirstRuns_TheTurnsOwnFailuresPerPackage(t *testing.T) {
	t.Parallel()
	res := &testfacts.Result{Packages: []*testfacts.Package{
		{Name: "m/a", Tests: []*testfacts.Test{
			{Name: "TestNew", Status: testfacts.StatusFail},
			{Name: "TestNew/case_one", Status: testfacts.StatusFail},
			{Name: "TestOld", Status: testfacts.StatusFail},
			{Name: "TestPass", Status: testfacts.StatusPass},
		}},
		{Name: "m/b", Tests: []*testfacts.Test{
			{Name: "TestFlaky", Status: testfacts.StatusFail},
			{Name: "TestX.Y", Status: testfacts.StatusFail},
		}},
	}}
	v := TestVerification{Ran: true, Outcome: VerifyFailed, Result: res,
		BaselineRan:      true,
		BaselineFailures: []TestKey{{Pkg: "m/a", Name: "TestOld"}},
		Unreproduced:     []TestKey{{Pkg: "m/b", Name: "TestFlaky"}},
	}
	runs := failingFirstRuns(turnFailures(v))
	if len(runs) != 2 {
		t.Fatalf("runs = %+v, want one per package", runs)
	}
	if got := strings.Join(runs[0].Packages, ",") + " " + strings.Join(runs[0].Args, " "); got != "m/a -run ^(TestNew)$" {
		t.Fatalf("m/a run = %q", got)
	}
	if got := strings.Join(runs[1].Packages, ",") + " " + strings.Join(runs[1].Args, " "); got != `m/b -run ^(TestX\.Y)$` {
		t.Fatalf("m/b run = %q", got)
	}
}
