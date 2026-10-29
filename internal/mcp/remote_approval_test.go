package mcp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/approval"
)

// testApprover stands in for the TUI: a real broker with an attached host
// that answers each offered request through approval.Host, the only path a
// decision can take. decide picks the verdict per offered request ("" leaves
// it undecided).
type testApprover struct {
	broker  *approval.Broker
	host    *approval.Host
	offered atomic.Int32
	mu      sync.Mutex
	last    approval.Request
	decide  func(n int32, req approval.Request) approval.Verdict
}

func attachTestApprover(test *testing.T, manager *MCPClientManager, ttl time.Duration, decide func(n int32, req approval.Request) approval.Verdict) *testApprover {
	test.Helper()
	broker, err := approval.NewBroker(approval.Options{TTL: ttl, PreviewMaxBytes: 4096, JournalRetention: time.Hour})
	if err != nil {
		test.Fatal(err)
	}
	host, err := broker.AttachHost("tui:test", "operator")
	if err != nil {
		test.Fatal(err)
	}
	a := &testApprover{broker: broker, host: host, decide: decide}
	manager.SetApprovalBroker(broker)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case req := <-host.Updates():
				if req.Status != approval.StatusAwaitingHuman {
					continue
				}
				n := a.offered.Add(1)
				a.mu.Lock()
				a.last = req
				a.mu.Unlock()
				if verdict := a.decide(n, req); verdict != "" {
					_, _ = host.Decide(approval.DecisionInput{RequestID: req.ID, RequestVersion: req.Version, RequestDigest: req.Digest, EventID: "ev-" + req.ID, Verdict: verdict})
				}
			case <-stop:
				return
			}
		}
	}()
	test.Cleanup(func() {
		close(stop)
		wg.Wait()
		_ = broker.Close()
	})
	return a
}

func approveAll(int32, approval.Request) approval.Verdict { return approval.VerdictApprove }

func (a *testApprover) lastRequest() approval.Request {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

// Replay: the approval admitted one call; the identical next call is a new
// request, and refusing it dispatches nothing more.
func TestRemoteApproval_DoesNotCarryToTheNextCall(t *testing.T) {
	fixture := newAuthorityFixture(t)
	fixture.reviewEffect(t, RemoteDelete, RiskSafe)
	fixture.confirmed = true
	approver := attachTestApprover(t, fixture.manager, time.Minute, func(n int32, _ approval.Request) approval.Verdict {
		if n == 1 {
			return approval.VerdictApprove
		}
		return approval.VerdictReject
	})
	if err := fixture.call(t, "manager"); err != nil {
		t.Fatalf("approved call: %v", err)
	}
	first := approver.lastRequest()
	if got, _ := approver.broker.Get(first.ID); got.Status != approval.StatusCompleted {
		t.Fatalf("approved request ended %s, want completed", got.Status)
	}
	fixture.ctx = WithRemoteCallIdentity(context.Background(), RemoteCallIdentity{Scope: "scope-2", CallID: "call-2"})
	err := fixture.call(t, "manager")
	if err == nil || !errors.Is(err, approval.ErrRejected) {
		t.Fatalf("second call = %v, want a rejection of its own request", err)
	}
	if approver.offered.Load() != 2 || approver.lastRequest().ID == first.ID {
		t.Fatal("the second call reused the first approval instead of asking")
	}
	if fixture.transport.calls != 1 {
		t.Fatalf("dispatches = %d, want 1", fixture.transport.calls)
	}
	requireAuthorityClean(t, fixture.kernel)
}

// confirm_risk is deliberateness, not consent: alone it creates no request
// and dispatches nothing; without it no request is offered either.
func TestRemoteApproval_ConfirmRiskIsNotConsent(t *testing.T) {
	fixture := newAuthorityFixture(t)
	fixture.reviewEffect(t, RemoteDelete, RiskSafe)
	fixture.confirmed = true
	if err := fixture.call(t, "manager"); err == nil || fixture.transport.calls != 0 {
		t.Fatalf("confirm_risk alone dispatched: %v, %d", err, fixture.transport.calls)
	}
	approver := attachTestApprover(t, fixture.manager, time.Minute, approveAll)
	fixture.confirmed = false
	fixture.ctx = WithRemoteCallIdentity(context.Background(), RemoteCallIdentity{Scope: "s2", CallID: "c2"})
	if err := fixture.call(t, "manager"); err == nil || fixture.transport.calls != 0 {
		t.Fatalf("an unconfirmed call dispatched: %v", err)
	}
	if approver.offered.Load() != 0 {
		t.Fatal("an unconfirmed call was offered for approval; the deliberateness check comes first")
	}
	requireAuthorityClean(t, fixture.kernel)
}

// Stale: the server's endpoint changes between review and resume.
func TestRemoteApproval_ChangedRegistrationIsStale(t *testing.T) {
	fixture := newAuthorityFixture(t)
	fixture.reviewEffect(t, RemoteDelete, RiskSafe)
	fixture.confirmed = true
	approver := attachTestApprover(t, fixture.manager, time.Minute, func(int32, approval.Request) approval.Verdict {
		fixture.manager.mu.Lock()
		fixture.manager.servers["srv"].Server.Endpoint = "https://moved.example"
		fixture.manager.mu.Unlock()
		return approval.VerdictApprove
	})
	err := fixture.call(t, "manager")
	if err == nil || !errors.Is(err, approval.ErrStale) {
		t.Fatalf("stale approval = %v, want ErrStale", err)
	}
	if fixture.transport.calls != 0 {
		t.Fatal("a stale approval dispatched")
	}
	if got, _ := approver.broker.Get(approver.lastRequest().ID); got.Status != approval.StatusSuperseded {
		t.Fatalf("status = %s, want superseded", got.Status)
	}
	requireAuthorityClean(t, fixture.kernel)
}

func TestRemoteApproval_ExpiryAndNoHostRefuse(t *testing.T) {
	t.Run("expiry", func(test *testing.T) {
		fixture := newAuthorityFixture(test)
		fixture.reviewEffect(test, RemoteDelete, RiskSafe)
		fixture.confirmed = true
		attachTestApprover(test, fixture.manager, 50*time.Millisecond, func(int32, approval.Request) approval.Verdict { return "" })
		if err := fixture.call(test, "manager"); err == nil || !errors.Is(err, approval.ErrExpired) || fixture.transport.calls != 0 {
			test.Fatalf("expired request = %v, calls %d", err, fixture.transport.calls)
		}
		requireAuthorityClean(test, fixture.kernel)
	})
	t.Run("no-host", func(test *testing.T) {
		fixture := newAuthorityFixture(test)
		fixture.reviewEffect(test, RemoteDelete, RiskSafe)
		fixture.confirmed = true
		approver := attachTestApprover(test, fixture.manager, time.Minute, approveAll)
		approver.host.Detach()
		if err := fixture.call(test, "manager"); err == nil || fixture.transport.calls != 0 {
			test.Fatalf("hostless approvable call = %v, calls %d", err, fixture.transport.calls)
		}
		if len(approver.broker.Snapshot()) != 0 {
			test.Fatal("a request was opened with nobody to decide it")
		}
	})
}

// Two concurrent calls carrying one call identity: one admission, at most one
// dispatch.
func TestRemoteApproval_DuplicateCallDispatchesAtMostOnce(t *testing.T) {
	fixture := newAuthorityFixture(t)
	fixture.reviewEffect(t, RemoteDelete, RiskSafe)
	fixture.confirmed = true
	attachTestApprover(t, fixture.manager, time.Minute, approveAll)
	var wg sync.WaitGroup
	var admitted atomic.Int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := fixture.manager.CallTool(WithRiskConfirmation(fixture.ctx, true), "srv/op", fixture.args)
			if err == nil && result != nil && result.Success {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if admitted.Load() != 1 || fixture.transport.calls != 1 {
		t.Fatalf("admitted=%d dispatches=%d, want 1/1", admitted.Load(), fixture.transport.calls)
	}
	requireAuthorityClean(t, fixture.kernel)
}
