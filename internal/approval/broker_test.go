package approval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testSubject() Subject {
	return Subject{
		Workspace: "/ws", Session: "s1", Agent: "session", Backend: "modular",
		Tool: "delete_file", ToolVersion: "sha256:tool", Action: "/delete_file",
		Target: "notes.md", Payload: `{"confirmed":true,"path":"notes.md"}`, Effect: "write",
		SourceRevisions: []SourceRevision{{Path: "notes.md", Digest: "sha256:aaa"}},
	}
}

func testPreview(t *testing.T) Preview {
	t.Helper()
	p, err := BuildPreview(PreviewInput{Summary: "delete notes.md", Arguments: testSubject().Payload, MaxBytes: 4096,
		File: &FileChange{Path: "notes.md", Before: []byte("hello\n"), Exists: true}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newTestBroker(t *testing.T, ttl time.Duration, journal string) *Broker {
	t.Helper()
	b, err := NewBroker(Options{TTL: ttl, PreviewMaxBytes: 4096, JournalRetention: time.Hour, JournalPath: journal})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

type awaitResult struct {
	grant *Grant
	req   Request
	err   error
}

// startAwait runs Await in the background and returns the request the host
// was offered.
func startAwait(t *testing.T, ctx context.Context, b *Broker, h *Host, intent string, s Subject) (Request, <-chan awaitResult) {
	t.Helper()
	done := make(chan awaitResult, 1)
	go func() {
		g, r, err := b.Await(ctx, intent, s, testPreview(t))
		done <- awaitResult{g, r, err}
	}()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case r := <-h.Updates():
			if r.IntentID == intent && r.Status == StatusAwaitingHuman {
				return r, done
			}
		case <-deadline:
			t.Fatal("the host was never offered the request")
			return Request{}, nil
		}
	}
}

func decide(t *testing.T, h *Host, r Request, v Verdict, event string) Request {
	t.Helper()
	got, err := h.Decide(DecisionInput{RequestID: r.ID, RequestVersion: r.Version, RequestDigest: r.Digest, EventID: event, Verdict: v})
	if err != nil {
		t.Fatalf("decide %s: %v", v, err)
	}
	return got
}

func TestSubjectDigest_BindsEveryAuthorityField(t *testing.T) {
	base := testSubject()
	want, err := base.Digest()
	if err != nil {
		t.Fatal(err)
	}
	reordered := base
	reordered.SourceRevisions = []SourceRevision{{Path: "z", Digest: "absent"}, {Path: "a", Digest: "dir"}}
	again := base
	again.SourceRevisions = []SourceRevision{{Path: "a", Digest: "dir"}, {Path: "z", Digest: "absent"}}
	d1, _ := reordered.Digest()
	d2, _ := again.Digest()
	if d1 != d2 {
		t.Error("source revision order changed the digest; the set is unordered")
	}
	mutations := map[string]func(*Subject){
		"workspace": func(s *Subject) { s.Workspace = "/other" },
		"session":   func(s *Subject) { s.Session = "s2" },
		"agent":     func(s *Subject) { s.Agent = "coder" },
		"backend":   func(s *Subject) { s.Backend = "mcp" },
		"tool":      func(s *Subject) { s.Tool = "rm" },
		"version":   func(s *Subject) { s.ToolVersion = "sha256:new" },
		"action":    func(s *Subject) { s.Action = "/write_file" },
		"target":    func(s *Subject) { s.Target = "notes2.md" },
		"payload":   func(s *Subject) { s.Payload = `{"confirmed":true,"path":"notes2.md"}` },
		"effect":    func(s *Subject) { s.Effect = "execute" },
		"endpoint":  func(s *Subject) { s.Endpoint = "https://x" },
		"revision":  func(s *Subject) { s.SourceRevisions[0].Digest = "sha256:bbb" },
	}
	for name, mutate := range mutations {
		s := testSubject()
		mutate(&s)
		got, err := s.Digest()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got == want {
			t.Errorf("changing %s did not change the digest", name)
		}
	}
	bad := testSubject()
	bad.Payload = "\xff\xfe"
	if _, err := bad.Digest(); err == nil {
		t.Error("invalid UTF-8 was digested; encoding/json would have folded it into U+FFFD")
	}
	dup := testSubject()
	dup.SourceRevisions = append(dup.SourceRevisions, dup.SourceRevisions[0])
	if _, err := dup.Digest(); err == nil {
		t.Error("a duplicated source revision was accepted")
	}
}

func TestAwait_WithoutHostCreatesNoRequest(t *testing.T) {
	b := newTestBroker(t, time.Minute, "")
	if _, _, err := b.Await(context.Background(), "exec-1", testSubject(), testPreview(t)); !errors.Is(err, ErrNoHost) {
		t.Fatalf("err = %v, want ErrNoHost", err)
	}
	if n := len(b.Snapshot()); n != 0 {
		t.Fatalf("%d requests exist with no host to decide them", n)
	}
}

func TestApproveReserveDispatchComplete(t *testing.T) {
	b := newTestBroker(t, time.Minute, "")
	h, err := b.AttachHost("tui:test", "operator")
	if err != nil {
		t.Fatal(err)
	}
	req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
	decide(t, h, req, VerdictApprove, "ev-1")
	res := <-done
	if res.err != nil {
		t.Fatalf("await: %v", res.err)
	}
	if res.req.Decision == nil || res.req.Decision.Actor != "operator" || res.req.Decision.Channel != "tui:test" {
		t.Fatalf("decision identity not bound to the host: %+v", res.req.Decision)
	}
	if err := res.grant.Reserve("exec-1", testSubject()); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := res.grant.DispatchStarted(); err != nil {
		t.Fatal(err)
	}
	res.grant.Finish(nil)
	got, _ := b.Get(req.ID)
	if got.Status != StatusCompleted || got.Claim == nil || got.Claim.Outcome != "completed" {
		t.Fatalf("final record = %+v", got)
	}
}

// Approve, then mutate: the operation that runs must be the one reviewed.
func TestReserve_ApproveThenMutateIsStale(t *testing.T) {
	for name, mutate := range map[string]func(*Subject){
		"arguments":       func(s *Subject) { s.Payload = `{"confirmed":true,"path":"other.md"}` },
		"source revision": func(s *Subject) { s.SourceRevisions[0].Digest = "sha256:changed" },
		"tool version":    func(s *Subject) { s.ToolVersion = "sha256:reregistered" },
	} {
		t.Run(name, func(t *testing.T) {
			b := newTestBroker(t, time.Minute, "")
			h, _ := b.AttachHost("tui:test", "operator")
			req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
			decide(t, h, req, VerdictApprove, "ev-1")
			res := <-done
			current := testSubject()
			mutate(&current)
			if err := res.grant.Reserve("exec-1", current); !errors.Is(err, ErrStale) {
				t.Fatalf("reserve after mutation = %v, want ErrStale", err)
			}
			if got, _ := b.Get(req.ID); got.Status != StatusSuperseded {
				t.Fatalf("status = %s, want superseded", got.Status)
			}
			// The original operation cannot borrow it afterwards either.
			if err := res.grant.Reserve("exec-1", testSubject()); err == nil {
				t.Fatal("a superseded approval was reserved")
			}
		})
	}
}

func TestReserve_ReplayAndSiblingCannotBorrow(t *testing.T) {
	b := newTestBroker(t, time.Minute, "")
	h, _ := b.AttachHost("tui:test", "operator")
	req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
	decide(t, h, req, VerdictApprove, "ev-1")
	res := <-done
	if err := res.grant.Reserve("exec-2", testSubject()); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("another intent reserved the approval: %v", err)
	}
	if err := res.grant.Reserve("exec-1", testSubject()); err != nil {
		t.Fatal(err)
	}
	if err := res.grant.Reserve("exec-1", testSubject()); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("replayed reserve = %v, want ErrAlreadyClaimed", err)
	}
}

func TestReserve_ConcurrentClaimsAdmitAtMostOne(t *testing.T) {
	b := newTestBroker(t, time.Minute, "")
	h, _ := b.AttachHost("tui:test", "operator")
	req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
	decide(t, h, req, VerdictApprove, "ev-1")
	res := <-done
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res.grant.Reserve("exec-1", testSubject()) == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("%d concurrent claims succeeded, want exactly 1", wins.Load())
	}
}

func TestDecide_DuplicateEventIsOneDecision(t *testing.T) {
	b := newTestBroker(t, time.Minute, "")
	h, _ := b.AttachHost("tui:test", "operator")
	req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
	first := decide(t, h, req, VerdictApprove, "ev-1")
	second := decide(t, h, req, VerdictApprove, "ev-1")
	if first.Decision.DecidedAt != second.Decision.DecidedAt || second.Status != StatusApproved {
		t.Fatal("a duplicate event created a second decision")
	}
	<-done
	other := testSubject()
	other.Target = "other.md"
	req2, done2 := startAwait(t, context.Background(), b, h, "exec-2", other)
	if _, err := h.Decide(DecisionInput{RequestID: req2.ID, RequestVersion: 1, RequestDigest: req2.Digest, EventID: "ev-1", Verdict: VerdictApprove}); !errors.Is(err, ErrDecisionMismatch) {
		t.Fatalf("an event ID was reused for another request: %v", err)
	}
	decide(t, h, req2, VerdictCancel, "ev-2")
	<-done2
}

func TestDecide_RejectedStaysRejected(t *testing.T) {
	b := newTestBroker(t, time.Minute, "")
	h, _ := b.AttachHost("tui:test", "operator")
	req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
	decide(t, h, req, VerdictReject, "ev-1")
	if res := <-done; !errors.Is(res.err, ErrRejected) {
		t.Fatalf("await = %v, want ErrRejected", res.err)
	}
	if _, err := h.Decide(DecisionInput{RequestID: req.ID, RequestVersion: 1, RequestDigest: req.Digest, EventID: "ev-2", Verdict: VerdictApprove}); !errors.Is(err, ErrNotDecidable) {
		t.Fatalf("approving a rejected request = %v, want ErrNotDecidable", err)
	}
	if got, _ := b.Get(req.ID); got.Status != StatusRejected {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestDecide_MustNameTheDisplayedRequest(t *testing.T) {
	b := newTestBroker(t, time.Minute, "")
	h, _ := b.AttachHost("tui:test", "operator")
	req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
	for name, in := range map[string]DecisionInput{
		"digest":  {RequestID: req.ID, RequestVersion: 1, RequestDigest: "sha256:other", EventID: "e1", Verdict: VerdictApprove},
		"version": {RequestID: req.ID, RequestVersion: 2, RequestDigest: req.Digest, EventID: "e2", Verdict: VerdictApprove},
	} {
		if _, err := h.Decide(in); !errors.Is(err, ErrDecisionMismatch) {
			t.Errorf("%s mismatch = %v", name, err)
		}
	}
	// A second host cannot attach while one is live, so a stale channel
	// cannot decide.
	if _, err := b.AttachHost("tui:other", "intruder"); !errors.Is(err, ErrHostAttached) {
		t.Fatalf("second host attached: %v", err)
	}
	h.Detach()
	if res := <-done; !errors.Is(res.err, ErrCanceled) {
		t.Fatalf("detach left the call waiting: %v", res.err)
	}
	if _, err := h.Decide(DecisionInput{RequestID: req.ID, RequestVersion: 1, RequestDigest: req.Digest, EventID: "e3", Verdict: VerdictApprove}); !errors.Is(err, ErrWrongChannel) {
		t.Fatalf("a detached host decided: %v", err)
	}
}

func TestAwait_ExpiresWithoutDecision(t *testing.T) {
	b := newTestBroker(t, 30*time.Millisecond, "")
	h, _ := b.AttachHost("tui:test", "operator")
	req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
	if res := <-done; !errors.Is(res.err, ErrExpired) {
		t.Fatalf("await = %v, want ErrExpired", res.err)
	}
	if _, err := h.Decide(DecisionInput{RequestID: req.ID, RequestVersion: 1, RequestDigest: req.Digest, EventID: "late", Verdict: VerdictApprove}); err == nil {
		t.Fatal("an expired request accepted a decision")
	}
}

func TestReserve_ApprovedButExpiredFailsClosed(t *testing.T) {
	now := time.Unix(1000, 0)
	var mu sync.Mutex
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	b, err := NewBroker(Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour, Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	h, _ := b.AttachHost("tui:test", "operator")
	req, done := startAwait(t, context.Background(), b, h, "exec-1", testSubject())
	decide(t, h, req, VerdictApprove, "ev-1")
	res := <-done
	mu.Lock()
	now = now.Add(2 * time.Minute)
	mu.Unlock()
	if err := res.grant.Reserve("exec-1", testSubject()); !errors.Is(err, ErrExpired) {
		t.Fatalf("reserve after expiry = %v, want ErrExpired", err)
	}
}

func TestAwait_ContextCancelCancelsRequest(t *testing.T) {
	b := newTestBroker(t, time.Minute, "")
	h, _ := b.AttachHost("tui:test", "operator")
	ctx, cancel := context.WithCancel(context.Background())
	req, done := startAwait(t, ctx, b, h, "exec-1", testSubject())
	cancel()
	if res := <-done; !errors.Is(res.err, ErrCanceled) {
		t.Fatalf("await = %v", res.err)
	}
	if got, _ := b.Get(req.ID); got.Status != StatusCanceled {
		t.Fatalf("status = %s", got.Status)
	}
}

// Restart: nothing is resumed or replayed, and the states a crash can leave
// stay distinguishable.
func TestJournal_RestartReconcilesWithoutInventingDecisions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "approvals", "journal.jsonl")
	b, err := NewBroker(Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour, JournalPath: path})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := b.AttachHost("tui:test", "operator")
	approveTo := func(intent, target string) *Grant {
		s := testSubject()
		s.Target = target
		req, done := startAwait(t, context.Background(), b, h, intent, s)
		decide(t, h, req, VerdictApprove, "ev-"+intent)
		res := <-done
		if res.err != nil {
			t.Fatal(res.err)
		}
		if err := res.grant.Reserve(intent, s); err != nil {
			t.Fatal(err)
		}
		return res.grant
	}
	reserved := approveTo("exec-reserved", "a")
	started := approveTo("exec-started", "b")
	if err := started.DispatchStarted(); err != nil {
		t.Fatal(err)
	}
	done := approveTo("exec-done", "c")
	_ = done.DispatchStarted()
	done.Finish(nil)
	pendingReq, _ := startAwait(t, context.Background(), b, h, "exec-pending", testSubject())
	// Simulate the crash: no Close, the process is gone. A dead process holds no
	// handle, and on Windows the restart's compaction renames over the journal,
	// which fails while the old process's handle is open; so the handle goes
	// without the broker's shutdown transitions. Append a torn line.
	if err := b.journal.close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"at":"2026-01-01T00:00:00Z","request":{"id":"apr-torn`)
	_ = f.Close()

	b2, err := NewBroker(Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour, JournalPath: path})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	want := map[string]Status{
		reserved.RequestID(): StatusAdmissionFailed,
		started.RequestID():  StatusOutcomeUnknown,
		done.RequestID():     StatusCompleted,
		pendingReq.ID:        StatusExpired,
	}
	for id, status := range want {
		got, ok := b2.Get(id)
		if !ok || got.Status != status {
			t.Errorf("%s after restart = %s, want %s", id, got.Status, status)
		}
		if got.Decision == nil && id != pendingReq.ID {
			t.Errorf("%s lost its decision on replay", id)
		}
	}
	if got, _ := b2.Get(pendingReq.ID); got.Decision != nil {
		t.Error("restart invented a decision for a request nobody decided")
	}
	if len(b2.Snapshot()) != 4 {
		t.Errorf("the torn line produced a record: %d records", len(b2.Snapshot()))
	}
	// The grant held by the dead process cannot act on the new broker, and
	// the reconciled records accept no decision.
	h2, _ := b2.AttachHost("tui:test", "operator")
	if _, err := h2.Decide(DecisionInput{RequestID: pendingReq.ID, RequestVersion: 1, RequestDigest: pendingReq.Digest, EventID: "after-restart", Verdict: VerdictApprove}); err == nil {
		t.Error("a request expired by restart accepted an approval")
	}
	_ = b.journal.close()
}

func TestPreview_WithholdingIsExplicit(t *testing.T) {
	cases := map[string]PreviewInput{
		"binary":    {Summary: "s", MaxBytes: 64, File: &FileChange{Path: "x.bin", Before: []byte{1, 0, 2}, Exists: true}},
		"oversize":  {Summary: "s", MaxBytes: 4, File: &FileChange{Path: "x.txt", Before: []byte("hello world"), Exists: true}},
		"directory": {Summary: "s", MaxBytes: 64, File: &FileChange{Path: "dir", Exists: true, IsDir: true}},
		"arguments": {Summary: "s", MaxBytes: 4, Arguments: "0123456789"},
	}
	for name, in := range cases {
		p, err := BuildPreview(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.Complete || len(p.Incomplete) == 0 {
			t.Errorf("%s: preview claims to be complete", name)
		}
		if p.Diff != nil {
			t.Errorf("%s: a diff was shown for withheld content", name)
		}
	}
	p, err := BuildPreview(PreviewInput{Summary: "s", MaxBytes: 64, File: &FileChange{Path: "a.txt", Before: []byte("one\n"), Exists: true}})
	if err != nil || !p.Complete || p.Diff == nil || !p.Diff.IsDelete {
		t.Fatalf("plain removal preview = %+v, %v", p, err)
	}
	if !p.Verify() {
		t.Fatal("a sealed preview does not verify")
	}
	p.Summary = "something else"
	if p.Verify() {
		t.Fatal("an edited preview still verifies")
	}
	b := newTestBroker(t, time.Minute, "")
	_, _ = b.AttachHost("tui:test", "operator")
	if _, _, err := b.Await(context.Background(), "exec-1", testSubject(), p); err == nil || !strings.Contains(err.Error(), "sealed") {
		t.Fatalf("an unsealed preview was offered: %v", err)
	}
}

func TestOpenShared_OneBrokerPerJournal(t *testing.T) {
	opts := Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour, JournalPath: filepath.Join(t.TempDir(), "j.jsonl")}
	first, releaseFirst, err := OpenShared(opts)
	if err != nil {
		t.Fatal(err)
	}
	second, releaseSecond, err := OpenShared(opts)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("two brokers were opened on one journal")
	}
	if err := releaseFirst(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.AttachHost("tui:x", "op"); err != nil {
		t.Fatalf("the broker closed while still referenced: %v", err)
	}
	if err := releaseSecond(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.AttachHost("tui:y", "op"); err == nil {
		t.Fatal("the last release did not close the broker")
	}
}
