package campaign

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRecurseLock_OneLoopPerWorkspace(t *testing.T) {
	root := t.TempDir()
	release, err := acquireRecurseLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if !recurseRunning(root) {
		t.Fatal("a held lock reads as running")
	}
	if _, err := acquireRecurseLock(root); !errors.Is(err, ErrRecurseRunning) {
		t.Fatalf("a second loop on the workspace must be refused: %v", err)
	}
	release()
	if recurseRunning(root) {
		t.Fatal("a released lock reads as not running")
	}
	again, err := acquireRecurseLock(root)
	if err != nil {
		t.Fatalf("the lock is takeable again after release: %v", err)
	}
	again()
}

func TestRunRecurseCycles_RefusesWhileAnotherLoopRuns(t *testing.T) {
	t.Parallel()
	root := recurseFixture(t, nil)
	release, err := acquireRecurseLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, err = runRecurse(t, context.Background(), root, 1, func(ctx context.Context, a RecurseAttempt) error {
		t.Fatal("no attempt may run beside another loop")
		return nil
	})
	if !errors.Is(err, ErrRecurseRunning) {
		t.Fatalf("err = %v, want ErrRecurseRunning", err)
	}
}

// A stop requested mid-attempt (from another shell) lets the attempt be
// judged, then ends the loop cleanly; the next run resumes, and status reads
// the ledger.
func TestRunRecurseCycles_StopRequestEndsAfterTheAttemptIsJudged(t *testing.T) {
	t.Parallel()
	root := recurseFixture(t, map[string]string{
		"lib/lib.go":      "package lib\n\nfunc L() int { return 1 }\n",
		"lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestL(t *testing.T) {\n\tif L() != 0 {\n\t\tt.Fatal(\"L() != 0\")\n\t}\n}\n",
	})
	var nodes []string
	res, err := runRecurse(t, context.Background(), root, 0, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		nodes = append(nodes, a.Node.ID)
		if err := RequestRecurseStop(root); err != nil {
			t.Fatal(err)
		}
		write(t, root, "lib/lib.go", "package lib\n\nfunc L() int { return 0 }\n")
		return nil
	}))
	if !errors.Is(err, ErrRecurseStopped) {
		t.Fatalf("err = %v, want ErrRecurseStopped", err)
	}
	if len(nodes) != 1 || nodes[0] != "lib" || res.Kept != 1 {
		t.Fatalf("the attempt in flight is judged before the stop: nodes %v, result %+v", nodes, res)
	}
	if recurseStopRequested(root) {
		t.Fatal("the stop request is consumed")
	}

	st, err := ReadRecurseStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Running || st.Kept != 1 || len(st.ThisPass) != 1 || st.ThisPass[0].Node != "lib" || st.ThisPass[0].Angle != "" {
		t.Fatalf("status = %+v", st)
	}
	if !strings.Contains(st.String(), "not running") || !strings.Contains(st.String(), "1 kept") {
		t.Fatalf("status text = %q", st.String())
	}

	// The next run resumes the pass: store is still red and gets its turn.
	nodes = nil
	if _, err := runRecurse(t, context.Background(), root, 1, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		nodes = append(nodes, a.Node.ID)
		write(t, root, "store/store.go", fixedStore)
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0] != "store" {
		t.Fatalf("resumed run attempts = %v", nodes)
	}
	st, err = ReadRecurseStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Kept != 2 || len(st.OpenByPass) != 1 || st.OpenByPass[0] != 0 {
		t.Fatalf("status after the pass = %+v", st)
	}
}

func TestReadRecurseStatus_NeverRun(t *testing.T) {
	st, err := ReadRecurseStatus(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if st.Running || st.Cycles != 0 || !strings.Contains(st.String(), "not running") {
		t.Fatalf("status = %+v", st)
	}
	if _, err := os.Stat(recurseLockPath(t.TempDir())); !os.IsNotExist(err) {
		t.Fatal("reading status creates nothing")
	}
}

func TestReadRecurseStatus_JSONMappingPinsToJSON(t *testing.T) {
	t.Parallel()
	st := RecurseStatus{
		Running: true, StopRequested: true, Pass: 2, Node: "lib",
		Cycles: 3, Kept: 1, Reverted: 1, Unverified: 1,
		OpenByPass: []int{2, 1},
		Base: RecurseBaseStatus{
			Branch: "main", TipAtStart: "tipcommit1234567890",
			LastAbsorbed: "abcdef1234567890xyz", Absorbs: 2,
			Conflicts:   []RecurseConflict{{BaseCommit: "conflict1234567890", Detail: "merge conflicted"}},
			Generations: []RecurseGeneration{{Commit: "feedface1234567890", Binary: "/tmp/nerd-new"}, {Commit: "deadbeef1234567890", Failure: "build failed"}},
		},
		ThisPass: []recurseRecord{
			{Cycle: 7, Outcome: "/kept", Node: "lib", Commit: "abcdef1234567890xyz", Signature: "sig", Finding: "F1"},
			{Cycle: 8, Outcome: "/unverified", Node: "app", Commit: "", Why: "needs /changed_code_unexecuted", Detail: "gate /check_not_green"},
		},
	}
	got := st.ToRecurseStatusJSON()
	if got.State != "running, stop requested" {
		t.Fatalf("State = %q, want %q", got.State, "running, stop requested")
	}
	if got.Pass != 2 || got.Node != "lib" || got.Cycles != 3 || got.Kept != 1 || got.Reverted != 1 || got.Unverified != 1 {
		t.Fatalf("counts = %+v, want pass 2 lib 3/1/1/1", got)
	}
	if len(got.OpenByPass) != 2 || got.OpenByPass[0] != 2 || got.OpenByPass[1] != 1 {
		t.Fatalf("OpenByPass = %v, want [2 1]", got.OpenByPass)
	}
	if got.Base.Branch != "main" || got.Base.Tip != "tipcommit1234567890" || got.Base.TipShort != "tipcommit" {
		t.Fatalf("base tip = %q/%q, want tipcommit1234567890/tipcommit", got.Base.Tip, got.Base.TipShort)
	}
	if got.Base.LastAbsorbed != "abcdef1234567890xyz" || got.Base.LastAbsorbedShort != "abcdef123" || got.Base.Absorbs != 2 {
		t.Fatalf("absorbed = %q/%q absorbs %d, want abcdef1234567890xyz/abcdef123/2", got.Base.LastAbsorbed, got.Base.LastAbsorbedShort, got.Base.Absorbs)
	}
	if len(got.Base.Conflicts) != 1 || got.Base.Conflicts[0].ShortCommit != "conflict1" || got.Base.Conflicts[0].Detail != "merge conflicted" {
		t.Fatalf("conflicts = %+v, want conflict1", got.Base.Conflicts)
	}
	if len(got.Base.Generations) != 2 || got.Base.Generations[0].ShortCommit != "feedface1" || got.Base.Generations[0].Binary != "/tmp/nerd-new" {
		t.Fatalf("generations[0] = %+v, want feedface1", got.Base.Generations[0])
	}
	if got.Base.Generations[1].Failure != "build failed" || got.Base.Generations[1].ShortCommit != "deadbeef1" {
		t.Fatalf("generations[1] = %+v, want failure", got.Base.Generations[1])
	}
	if len(got.Attempts) != 2 {
		t.Fatalf("Attempts len = %d, want 2", len(got.Attempts))
	}
	if got.Attempts[0].Cycle != 7 || got.Attempts[0].Outcome != "/kept" || got.Attempts[0].ShortCommit != "abcdef123" {
		t.Fatalf("attempt 0 = %+v, want 7 /kept abcdef123", got.Attempts[0])
	}
	if len(got.Attempts[0].MissingEvidence) != 0 {
		t.Fatalf("attempt 0 missing = %v, want empty", got.Attempts[0].MissingEvidence)
	}
	if got.Attempts[1].ShortCommit != "" {
		t.Fatalf("attempt 1 ShortCommit = %q, want empty", got.Attempts[1].ShortCommit)
	}
	if len(got.Attempts[1].MissingEvidence) != 2 {
		t.Fatalf("attempt 1 missing = %v, want 2", got.Attempts[1].MissingEvidence)
	}
	if got2 := (RecurseStatus{Running: false, StopRequested: true}).ToRecurseStatusJSON(); got2.State != "not running, stop requested" {
		t.Fatalf("State = %q, want not running, stop requested", got2.State)
	}
}
