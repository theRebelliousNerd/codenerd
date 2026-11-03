package approval

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Expiry must fail closed: an approval whose deadline has passed must not be
// reservable even when the journal append that would record its expiry fails.
// broker.go expireLocked discards transitionLocked's error, so a failed expiry
// leaves an elapsed request Approved and Reserve proceeds to reserve it.
func TestReserve_ExpiredApprovalFailsClosedWhenExpiryJournalFails(t *testing.T) {
	tests := []struct {
		name         string
		advance      time.Duration
		breakJournal bool
	}{
		{
			name:         "approved past deadline with a working journal fails closed",
			advance:      2 * time.Minute,
			breakJournal: false,
		},
		{
			name:         "approved past deadline with a failing expiry journal fails closed",
			advance:      time.Hour,
			breakJournal: true,
		},
		{
			name:         "approved far past deadline with a failing expiry journal fails closed",
			advance:      24 * time.Hour,
			breakJournal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Unix(1000, 0)
			var mu sync.Mutex
			clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
			b, err := NewBroker(Options{
				TTL:              time.Minute,
				PreviewMaxBytes:  4096,
				JournalRetention: time.Hour,
				JournalPath:      filepath.Join(t.TempDir(), "journal.jsonl"),
				Now:              clock,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = b.Close() })
			h, err := b.AttachHost("tui:test", "operator")
			if err != nil {
				t.Fatal(err)
			}

			subject := testSubject()
			digest, err := subject.Digest()
			if err != nil {
				t.Fatal(err)
			}
			id, err := b.open("exec-1", subject, digest, testPreview(t))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.Decide(DecisionInput{
				RequestID: id, RequestVersion: 1, RequestDigest: digest,
				EventID: "ev-1", Verdict: VerdictApprove,
			}); err != nil {
				t.Fatal(err)
			}

			mu.Lock()
			now = now.Add(tt.advance)
			mu.Unlock()

			if tt.breakJournal {
				// Fail every subsequent journal append, including the expiry
				// write Reserve performs before checking the request status.
				if err := b.journal.file.Close(); err != nil {
					t.Fatal(err)
				}
			}

			grant := &Grant{broker: b, requestID: id, intentID: "exec-1", digest: digest, eventID: "ev-1"}
			if err := grant.Reserve("exec-1", subject); !errors.Is(err, ErrExpired) {
				t.Errorf("Reserve on an elapsed approval with a failing expiry journal = %v, want %v", err, ErrExpired)
			}
		})
	}
}

// The same fault at the expireLocked level: after a failed expiry append the
// request must read Expired, not stay Approved where a later Reserve admits it.
func TestExpireLocked_FailingJournalStillExpiresElapsedRequest(t *testing.T) {
	now := time.Unix(1000, 0)
	var mu sync.Mutex
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	b, err := NewBroker(Options{
		TTL:              time.Minute,
		PreviewMaxBytes:  4096,
		JournalRetention: time.Hour,
		JournalPath:      filepath.Join(t.TempDir(), "journal.jsonl"),
		Now:              clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	h, err := b.AttachHost("tui:test", "operator")
	if err != nil {
		t.Fatal(err)
	}

	subject := testSubject()
	digest, err := subject.Digest()
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.open("exec-1", subject, digest, testPreview(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Decide(DecisionInput{
		RequestID: id, RequestVersion: 1, RequestDigest: digest,
		EventID: "ev-1", Verdict: VerdictApprove,
	}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	now = now.Add(2 * time.Minute)
	mu.Unlock()

	if err := b.journal.file.Close(); err != nil {
		t.Fatal(err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked(b.requests[id])
	if got := b.requests[id].Status; got != StatusExpired {
		t.Errorf("status after failed expiry append = %s, want %s", got, StatusExpired)
	}
}

func TestExpireLocked_JournalFailureExpiresAndNotifies(t *testing.T) {
	for _, tt := range []struct {
		name        string
		failJournal bool
		wantReason  string
	}{
		{name: "journal succeeds", wantReason: "no claim before the request's deadline"},
		{name: "journal fails", failJournal: true, wantReason: "expiry journal failed:"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Unix(1000, 0)
			b, err := NewBroker(Options{
				TTL:              time.Minute,
				PreviewMaxBytes:  4096,
				JournalRetention: time.Hour,
				JournalPath:      filepath.Join(t.TempDir(), "journal.jsonl"),
				Now:              func() time.Time { return now },
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := b.Close(); err != nil && !(tt.failJournal && errors.Is(err, os.ErrClosed)) {
					t.Errorf("close broker: %v", err)
				}
			})
			h, err := b.AttachHost("tui:test", "operator")
			if err != nil {
				t.Fatal(err)
			}
			subject := testSubject()
			digest, err := subject.Digest()
			if err != nil {
				t.Fatal(err)
			}
			id, err := b.open("exec-1", subject, digest, testPreview(t))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.Decide(DecisionInput{RequestID: id, RequestVersion: 1, RequestDigest: digest, EventID: "ev-1", Verdict: VerdictApprove}); err != nil {
				t.Fatal(err)
			}
			now = now.Add(2 * time.Minute)
			if tt.failJournal {
				if err := b.journal.file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			b.mu.Lock()
			wake := b.changed
			b.expireLocked(b.requests[id])
			request := b.requests[id].clone()
			b.mu.Unlock()
			if request.Status != StatusExpired {
				t.Errorf("status = %s, want %s", request.Status, StatusExpired)
			}
			if !strings.Contains(request.StatusReason, tt.wantReason) {
				t.Errorf("status reason = %q, want %q", request.StatusReason, tt.wantReason)
			}
			select {
			case <-wake:
			default:
				t.Error("expiry did not wake waiters")
			}
		})
	}
}
