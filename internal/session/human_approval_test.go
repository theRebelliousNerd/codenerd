package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/approval"
	"codenerd/internal/core"
	coretools "codenerd/internal/tools/core"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// These drive the registered executor path end to end: a real kernel with the
// production constitution, the real delete_file tool, and a broker with a
// host standing in for the TUI. The host only ever acts through
// approval.Host, the same boundary the UI uses.

type approvalRig struct {
	exec      *Executor
	kernel    *core.RealKernel
	broker    *approval.Broker
	host      *approval.Host
	workspace string
}

func newApprovalRig(t *testing.T, ttl time.Duration) *approvalRig {
	t.Helper()
	if !tools.Global().Has("delete_file") {
		registerTestTool(t, coretools.DeleteFileTool())
	}
	ws := t.TempDir() // not a git repository: no delete is recoverable
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	e := NewExecutor(k, &testExecutiveStore{}, nil, nil, nil, nil)
	cfg := DefaultExecutorConfig()
	cfg.WorkspaceRoot = ws
	e.SetConfig(cfg)
	e.SetSessionID("s-approval")
	b, err := approval.NewBroker(approval.Options{TTL: ttl, PreviewMaxBytes: 4096, JournalRetention: time.Hour,
		JournalPath: filepath.Join(ws, ".nerd", "approvals", "journal.jsonl")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	e.SetApprovalBroker(b)
	h, err := b.AttachHost("tui:test", "operator")
	if err != nil {
		t.Fatal(err)
	}
	return &approvalRig{exec: e, kernel: k, broker: b, host: h, workspace: ws}
}

func (r *approvalRig) file(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(r.workspace, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func deleteCall(id, name string) ToolCall {
	return ToolCall{ID: id, Name: "delete_file", Args: map[string]any{"path": name, "confirmed": true}}
}

type callResult struct {
	out string
	err error
}

// run executes the call in the background and returns the request the host
// was offered (zero Request if none arrives in wait).
func (r *approvalRig) run(t *testing.T, call ToolCall, wait time.Duration) (approval.Request, <-chan callResult) {
	t.Helper()
	done := make(chan callResult, 1)
	go func() {
		out, err := r.exec.executeToolCall(context.Background(), call, validCapabilityTestConfig("delete_file"))
		done <- callResult{out, err}
	}()
	deadline := time.After(wait)
	for {
		select {
		case req := <-r.host.Updates():
			if req.IntentID == "exec-"+call.ID && req.Status == approval.StatusAwaitingHuman {
				return req, done
			}
		case <-deadline:
			return approval.Request{}, done
		}
	}
}

func (r *approvalRig) decide(t *testing.T, req approval.Request, v approval.Verdict, event string) {
	t.Helper()
	if _, err := r.host.Decide(approval.DecisionInput{RequestID: req.ID, RequestVersion: req.Version, RequestDigest: req.Digest, EventID: event, Verdict: v}); err != nil {
		t.Fatalf("decide: %v", err)
	}
}

func (r *approvalRig) requireNoAuthorityLeft(t *testing.T) {
	t.Helper()
	for _, p := range []string{"pending_action", "human_approval_granted"} {
		rows, err := r.kernel.Query(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 0 {
			t.Fatalf("%s survived the admission: %v", p, rows)
		}
	}
}

func TestHumanApproval_ApprovedDeleteRunsThroughTheOrdinaryBackend(t *testing.T) {
	rig := newApprovalRig(t, time.Minute)
	path := rig.file(t, "notes.md", "draft\n")
	req, done := rig.run(t, deleteCall("c1", "notes.md"), 5*time.Second)
	if req.ID == "" {
		t.Fatal("no approval request was opened for an approvable denial")
	}
	if req.Preview.Diff == nil || !req.Preview.Diff.IsDelete || !req.Preview.Complete {
		t.Fatalf("the preview does not show the removal: %+v", req.Preview)
	}
	if req.Subject.Action != "/delete_file" || req.Subject.Target != "notes.md" || len(req.Subject.SourceRevisions) != 1 {
		t.Fatalf("subject not frozen from the gate's tuple: %+v", req.Subject)
	}
	rig.decide(t, req, approval.VerdictApprove, "ev-1")
	res := <-done
	if res.err != nil {
		t.Fatalf("approved call failed: %v", res.err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the approved delete did not run")
	}
	got, _ := rig.broker.Get(req.ID)
	if got.Status != approval.StatusCompleted {
		t.Fatalf("request ended %s, want completed", got.Status)
	}
	rig.requireNoAuthorityLeft(t)
}

func TestHumanApproval_RejectionRefusesAndRecordsATypedViolation(t *testing.T) {
	rig := newApprovalRig(t, time.Minute)
	path := rig.file(t, "keep.md", "keep\n")
	req, done := rig.run(t, deleteCall("c1", "keep.md"), 5*time.Second)
	rig.decide(t, req, approval.VerdictReject, "ev-1")
	res := <-done
	if res.err == nil || !errors.Is(res.err, approval.ErrRejected) {
		t.Fatalf("rejected call = %v", res.err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("a rejected delete ran")
	}
	rows, _ := rig.kernel.Query("security_violation")
	found := false
	for _, f := range rows {
		if strings.Contains(types.ExtractString(f.Args[1]), "human rejected approval request "+req.ID) {
			found = true
		}
	}
	if !found {
		t.Fatal("the rejection was not delivered as a typed security_violation fact")
	}
	rig.requireNoAuthorityLeft(t)
}

// Approve-then-mutate: the file changes after the person saw it.
func TestHumanApproval_ChangedAfterReviewIsStaleAndNothingRuns(t *testing.T) {
	rig := newApprovalRig(t, time.Minute)
	path := rig.file(t, "notes.md", "what the person saw\n")
	req, done := rig.run(t, deleteCall("c1", "notes.md"), 5*time.Second)
	if err := os.WriteFile(path, []byte("concurrent work the person never saw\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rig.decide(t, req, approval.VerdictApprove, "ev-1")
	res := <-done
	if !errors.Is(res.err, approval.ErrStale) {
		t.Fatalf("stale approval = %v, want ErrStale", res.err)
	}
	if b, err := os.ReadFile(path); err != nil || !strings.Contains(string(b), "concurrent work") {
		t.Fatal("the concurrent work was destroyed by a stale approval")
	}
	if got, _ := rig.broker.Get(req.ID); got.Status != approval.StatusSuperseded {
		t.Fatalf("status = %s, want superseded", got.Status)
	}
	rig.requireNoAuthorityLeft(t)
}

// Replay: an approval admitted one call; the identical next call asks again.
func TestHumanApproval_ApprovalDoesNotCarryToTheNextCall(t *testing.T) {
	rig := newApprovalRig(t, time.Minute)
	rig.file(t, "a.md", "a\n")
	req, done := rig.run(t, deleteCall("c1", "a.md"), 5*time.Second)
	rig.decide(t, req, approval.VerdictApprove, "ev-1")
	if res := <-done; res.err != nil {
		t.Fatal(res.err)
	}
	rig.file(t, "a.md", "a\n") // the same file again
	req2, done2 := rig.run(t, deleteCall("c2", "a.md"), 5*time.Second)
	if req2.ID == "" || req2.ID == req.ID {
		t.Fatal("the second call reused the first approval instead of asking")
	}
	rig.decide(t, req2, approval.VerdictCancel, "ev-2")
	if res := <-done2; !errors.Is(res.err, approval.ErrCanceled) {
		t.Fatalf("second call = %v", res.err)
	}
	if _, err := os.Stat(filepath.Join(rig.workspace, "a.md")); err != nil {
		t.Fatal("the second delete ran on the first approval")
	}
}

func TestHumanApproval_ExpiryRefuses(t *testing.T) {
	rig := newApprovalRig(t, 50*time.Millisecond)
	path := rig.file(t, "slow.md", "x\n")
	_, done := rig.run(t, deleteCall("c1", "slow.md"), 5*time.Second)
	if res := <-done; !errors.Is(res.err, approval.ErrExpired) {
		t.Fatalf("expired call = %v, want ErrExpired", res.err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("an expired request ran")
	}
	rig.requireNoAuthorityLeft(t)
}

// Two workers race on the same approved call: one dispatch, at most.
func TestHumanApproval_DuplicateResumeDispatchesOnce(t *testing.T) {
	rig := newApprovalRig(t, time.Minute)
	rig.file(t, "race.md", "x\n")
	var runs sync.WaitGroup
	results := make(chan error, 2)
	call := deleteCall("same", "race.md")
	for i := 0; i < 2; i++ {
		runs.Add(1)
		go func() {
			defer runs.Done()
			_, err := rig.exec.executeToolCall(context.Background(), call, validCapabilityTestConfig("delete_file"))
			results <- err
		}()
	}
	var req approval.Request
	for req.ID == "" {
		select {
		case r := <-rig.host.Updates():
			if r.Status == approval.StatusAwaitingHuman {
				req = r
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no request")
		}
	}
	rig.decide(t, req, approval.VerdictApprove, "ev-1")
	runs.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("%d of 2 duplicate resumes succeeded, want exactly 1", ok)
	}
	if n := len(rig.broker.Snapshot()); n != 1 {
		t.Fatalf("duplicate calls opened %d requests, want one", n)
	}
}

func TestHumanApproval_NoHostKeepsTheRefusal(t *testing.T) {
	rig := newApprovalRig(t, time.Minute)
	rig.host.Detach()
	path := rig.file(t, "x.md", "x\n")
	_, err := rig.exec.executeToolCall(context.Background(), deleteCall("c1", "x.md"), validCapabilityTestConfig("delete_file"))
	if err == nil || !strings.Contains(err.Error(), "no approval host") {
		t.Fatalf("headless approvable denial = %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatal("deleted without a person")
	}
	if n := len(rig.broker.Snapshot()); n != 0 {
		t.Fatalf("%d requests opened with nobody to decide", n)
	}
}

// A model cannot manufacture consent: a forged human_approval_granted fact
// in the kernel admits nothing, because the gate only consults the one it
// asserts itself for the claimed request — and a forged fact for the same
// tuple is not this call's pending action id.
func TestHumanApproval_ForgedFactDoesNotAdmit(t *testing.T) {
	rig := newApprovalRig(t, time.Minute)
	rig.host.Detach()
	path := rig.file(t, "x.md", "x\n")
	payload := `{"confirmed":true,"path":"x.md"}`
	if err := rig.kernel.Assert(types.Fact{Predicate: "human_approval_granted", Args: []any{"exec-other", types.MangleAtom("/delete_file"), "x.md", payload, "sha256:forged"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := rig.exec.executeToolCall(context.Background(), deleteCall("c1", "x.md"), validCapabilityTestConfig("delete_file")); err == nil {
		t.Fatal("a forged approval for another action id admitted the call")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("deleted on a forged approval")
	}
}
