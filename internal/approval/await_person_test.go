package approval

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// The standing yolo instruction approves every ordinary request itself. A
// request only a person may decide (the user's "confirm first" on one request)
// is more specific than that standing instruction and is not decided by it.
func TestAwaitPerson_YoloDoesNotDecideIt(t *testing.T) {
	b := newTestBroker(t, time.Minute, filepath.Join(t.TempDir(), "j.jsonl"))
	h, err := b.AttachHost("tui:test", "operator")
	if err != nil {
		t.Fatal(err)
	}
	b.SetAutonomous(true)

	// Control: the ordinary request is approved by autonomy, nobody asked.
	g, req, err := b.Await(context.Background(), "ordinary", testSubject(), testPreview(t))
	if err != nil || g == nil || req.Channel != AutonomyChannel {
		t.Fatalf("yolo Await = %v, %v, channel %q; want an autonomous approval", g, err, req.Channel)
	}

	// The person-only request waits for the host and carries the person's decision.
	done := make(chan awaitResult, 1)
	go func() {
		g, r, err := b.AwaitPerson(context.Background(), "personal", testSubject(), testPreview(t))
		done <- awaitResult{g, r, err}
	}()
	var offered Request
	for offered.ID == "" {
		select {
		case r := <-h.Updates():
			if r.IntentID == "personal" && r.Status == StatusAwaitingHuman {
				offered = r
			}
			if r.IntentID == "personal" && r.Channel == AutonomyChannel {
				t.Fatal("autonomy decided a person-only request")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the host was never offered the person-only request")
		}
	}
	select {
	case res := <-done:
		t.Fatalf("AwaitPerson returned before the person decided: %+v", res)
	case <-time.After(100 * time.Millisecond):
	}
	decide(t, h, offered, VerdictApprove, "ev-person")
	res := <-done
	if res.err != nil || res.grant == nil {
		t.Fatalf("approved AwaitPerson = %v", res.err)
	}
	if res.req.Decision == nil || res.req.Decision.Actor == AutonomyActor {
		t.Fatalf("the decision was not a person's: %+v", res.req.Decision)
	}
}

// With no host attached, nobody can decide a person-only request, yolo or not.
func TestAwaitPerson_NoHostIsRefusedEvenUnderYolo(t *testing.T) {
	b := newTestBroker(t, time.Minute, filepath.Join(t.TempDir(), "j.jsonl"))
	b.SetAutonomous(true)
	if g, _, err := b.AwaitPerson(context.Background(), "personal", testSubject(), testPreview(t)); err == nil || g != nil {
		t.Fatalf("AwaitPerson with no host = %v, %v; want a refusal", g, err)
	}
}
