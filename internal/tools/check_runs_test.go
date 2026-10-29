package tools

import (
	"context"
	"testing"
)

func TestRecordAcceptanceRun_RecordsOnlyIntoALog(t *testing.T) {
	// Without a log there is nowhere to record, and nothing panics.
	RecordAcceptanceRun(context.Background(), AcceptanceRun{Argv: []string{"go", "version"}})

	ctx, runs := WithAcceptanceRunLog(context.Background())
	if got := runs(); len(got) != 0 {
		t.Fatalf("a fresh log holds %v, want nothing", got)
	}
	RecordAcceptanceRun(ctx, AcceptanceRun{Argv: []string{"go", "version"}, ExitCode: 0})
	got := runs()
	if len(got) != 1 || got[0].ExitCode != 0 || got[0].Argv[1] != "version" {
		t.Fatalf("runs = %+v, want the one recorded run", got)
	}
	// What the log returns is a copy: a caller cannot rewrite the record.
	got[0].ExitCode = 1
	if runs()[0].ExitCode != 0 {
		t.Fatal("the log handed out its own storage")
	}
}

// An acceptance receipt must never read as test evidence: the /test_run gate
// reads only TestRun, and the two logs share nothing.
func TestAcceptanceRun_DoesNotSatisfyTestRunLog(t *testing.T) {
	ctx, testRuns := WithTestRunLog(context.Background())
	ctx, acceptanceRuns := WithAcceptanceRunLog(ctx)
	RecordAcceptanceRun(ctx, AcceptanceRun{Argv: []string{"checker"}, ExitCode: 0})
	if got := testRuns(); len(got) != 0 {
		t.Fatalf("an acceptance run leaked into the test log: %+v", got)
	}
	if got := acceptanceRuns(); len(got) != 1 {
		t.Fatalf("acceptance runs = %+v, want the one recorded run", got)
	}
}
