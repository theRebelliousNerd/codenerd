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
	root := recurseFixture(t, map[string]string{
		"lib/lib.go":      "package lib\n\nfunc L() int { return 1 }\n",
		"lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestL(t *testing.T) {\n\tif L() != 0 {\n\t\tt.Fatal(\"L() != 0\")\n\t}\n}\n",
	})
	var nodes []string
	res, err := runRecurse(t, context.Background(), root, 0, func(ctx context.Context, a RecurseAttempt) error {
		nodes = append(nodes, a.Node.ID)
		if err := RequestRecurseStop(root); err != nil {
			t.Fatal(err)
		}
		write(t, root, "lib/lib.go", "package lib\n\nfunc L() int { return 0 }\n")
		return nil
	})
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
	if st.Running || st.Kept != 1 || len(st.ThisPass) != 1 || st.ThisPass[0].Node != "lib" {
		t.Fatalf("status = %+v", st)
	}
	if !strings.Contains(st.String(), "not running") || !strings.Contains(st.String(), "1 kept") {
		t.Fatalf("status text = %q", st.String())
	}

	// The next run resumes the pass: store is still red and gets its turn.
	nodes = nil
	if _, err := runRecurse(t, context.Background(), root, 1, func(ctx context.Context, a RecurseAttempt) error {
		nodes = append(nodes, a.Node.ID)
		write(t, root, "store/store.go", fixedStore)
		return nil
	}); err != nil {
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
