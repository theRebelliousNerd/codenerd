package browser

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/mangle"
)

// recordingRetractor remembers every fact it was asked to remove and can be
// told to fail, so retirement tests assert on exact identity and on retry.
type recordingRetractor struct {
	mu        sync.Mutex
	retracted []mangle.Fact
	failNext  int
}

func (r *recordingRetractor) RetractFacts(facts []mangle.Fact) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNext > 0 {
		r.failNext--
		return errors.New("kernel retract unavailable")
	}
	r.retracted = append(r.retracted, facts...)
	return nil
}

func (r *recordingRetractor) snapshot() []mangle.Fact {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]mangle.Fact(nil), r.retracted...)
}

func openTestSession(manager *SessionManager, id string) {
	manager.mu.Lock()
	manager.sessions[id] = &sessionRecord{meta: Session{ID: id}}
	manager.mu.Unlock()
}

// The sink stores the redacted spelling of a fact. An exact retraction keyed by
// the raw spelling matches nothing, so the leaked fact would stay in the kernel
// while the manager believed it was collected.
func TestRetirement_WhenFactIsRedacted_ShouldRetractStoredSpelling(t *testing.T) {
	sink := &testEngineSinkLocal{}
	retractor := &recordingRetractor{}
	manager := NewSessionManagerWithSink(DefaultConfig(), sink)
	manager.SetFactRetractor(retractor)
	openTestSession(manager, "s1")

	raw := mangle.Fact{Predicate: "net_request", Args: []any{"s1", "r1", "https://example.test/cb?token=SECRET-VALUE&q=1", int64(1)}}
	if err := manager.addStreamFacts("s1", []mangle.Fact{raw}); err != nil {
		t.Fatalf("addStreamFacts: %v", err)
	}
	stored := sink.findFactsByPredicate("net_request")
	if len(stored) != 1 || strings.Contains(fmt.Sprint(stored[0].Args), "SECRET-VALUE") {
		t.Fatalf("sink stored %v; the redactor should have rewritten the URL", stored)
	}

	if err := manager.CloseSession(t.Context(), "s1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	retracted := retractor.snapshot()
	if len(retracted) != 1 || !reflect.DeepEqual(retracted[0].Args, stored[0].Args) {
		t.Fatalf("retracted %v, want the stored spelling %v", retracted, stored[0].Args)
	}
}

func TestRetirement_WhenProgressiveSnapshotRedacted_ShouldRetractStoredSpelling(t *testing.T) {
	sink := &testEngineSinkLocal{}
	retractor := &recordingRetractor{}
	manager := NewSessionManagerWithSink(DefaultConfig(), sink)
	manager.SetFactRetractor(retractor)
	openTestSession(manager, "s1")
	registry := NewElementRegistry()

	raw := mangle.Fact{Predicate: "net_request", Args: []any{"s1", "r1", "https://example.test/cb?token=SECRET-VALUE", int64(1)}}
	if _, err := manager.replaceProgressiveControlSnapshot("s1", registry.Generation(), registry, []mangle.Fact{raw}); err != nil {
		t.Fatalf("replaceProgressiveControlSnapshot: %v", err)
	}
	stored := sink.findFactsByPredicate("net_request")
	if len(stored) != 1 {
		t.Fatalf("sink stored %d facts, want 1", len(stored))
	}
	if err := manager.CloseSession(t.Context(), "s1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	retracted := retractor.snapshot()
	if len(retracted) != 1 || !reflect.DeepEqual(retracted[0].Args, stored[0].Args) {
		t.Fatalf("retracted %v, want the stored spelling %v", retracted, stored[0].Args)
	}
}

// browser_epoch and browser_stream_saturated bypass the event budget on
// purpose. They must still belong to a lifecycle, or every closed session
// leaves two markers in the kernel for good.
func TestRetirement_WhenSessionCloses_ShouldRetireLifecycleMarkers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxEpochEventFacts = 2
	sink := &testEngineSinkLocal{}
	retractor := &recordingRetractor{}
	manager := NewSessionManagerWithSink(cfg, sink)
	manager.SetFactRetractor(retractor)
	openTestSession(manager, "s1")

	manager.RollSessionEpoch("s1")
	for i := 0; i < 3; i++ {
		if err := manager.addStreamFacts("s1", streamFactBatch("s1", 2)); err != nil {
			t.Fatal(err)
		}
	}
	if len(sink.findFactsByPredicate("browser_stream_saturated")) != 1 || len(sink.findFactsByPredicate("browser_epoch")) != 1 {
		t.Fatalf("markers not asserted: epoch=%d saturated=%d",
			len(sink.findFactsByPredicate("browser_epoch")), len(sink.findFactsByPredicate("browser_stream_saturated")))
	}
	if err := manager.CloseSession(t.Context(), "s1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	seen := map[string]int{}
	for _, fact := range retractor.snapshot() {
		seen[fact.Predicate]++
	}
	if seen["browser_epoch"] != 1 || seen["browser_stream_saturated"] != 1 || seen["console_event"] != 2 {
		t.Fatalf("retired %v, want one browser_epoch, one browser_stream_saturated, two console_event", seen)
	}
}

// Retiring a saturated epoch retracts its notice, so the session keeps a
// bounded receipt that the stream was incomplete.
func TestRollSessionEpoch_WhenEpochSaturated_ShouldKeepIncompletenessReceipt(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxEpochEventFacts = 2
	manager := NewSessionManagerWithSink(cfg, &testEngineSinkLocal{})
	manager.SetFactRetractor(&recordingRetractor{})
	openTestSession(manager, "s1")
	for i := 0; i < 2; i++ {
		if err := manager.addStreamFacts("s1", streamFactBatch("s1", 2)); err != nil {
			t.Fatal(err)
		}
	}
	if stats := manager.SessionFactStats("s1"); !stats.Saturated || stats.Dropped == 0 {
		t.Fatalf("live epoch should report saturation: %+v", stats)
	}
	manager.RollSessionEpoch("s1")
	stats := manager.SessionFactStats("s1")
	if stats.Saturated || stats.SaturatedEpochs != 1 {
		t.Fatalf("after the roll the live epoch is clean but the receipt remains: %+v", stats)
	}
}

// A failed retraction is a cleanup result the closer must see, and the exact
// facts stay queued for the next retirement rather than being forgotten.
func TestCloseSession_WhenRetractionFails_ShouldReportAndRetryExactFacts(t *testing.T) {
	retractor := &recordingRetractor{failNext: 1}
	manager := NewSessionManagerWithSink(DefaultConfig(), &testEngineSinkLocal{})
	manager.SetFactRetractor(retractor)
	openTestSession(manager, "s1")
	openTestSession(manager, "s2")
	if err := manager.addStreamFacts("s1", streamFactBatch("s1", 3)); err != nil {
		t.Fatal(err)
	}

	err := manager.CloseSession(t.Context(), "s1")
	if err == nil || !strings.Contains(err.Error(), "retire session s1 facts") {
		t.Fatalf("CloseSession error = %v, want the retirement failure", err)
	}
	if got := manager.PendingRetirements(); got != 3 {
		t.Fatalf("pending retirements = %d, want the 3 exact facts", got)
	}

	if err := manager.CloseSession(t.Context(), "s2"); err != nil {
		t.Fatalf("second close should drain the pending facts: %v", err)
	}
	if manager.PendingRetirements() != 0 || len(retractor.snapshot()) != 3 {
		t.Fatalf("pending=%d retracted=%d after retry, want 0 and 3", manager.PendingRetirements(), len(retractor.snapshot()))
	}
}

// A caller-side sensor must publish through the manager so the facts count
// against the epoch budget and retire with the session.
func TestPublishSessionFacts_ShouldBeBudgetedTrackedAndRejectClosedSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxEpochEventFacts = 3
	sink := &testEngineSinkLocal{}
	retractor := &recordingRetractor{}
	manager := NewSessionManagerWithSink(cfg, sink)
	manager.SetFactRetractor(retractor)
	openTestSession(manager, "s1")

	for i := 0; i < 5; i++ {
		fact := mangle.Fact{Predicate: "dom_updated", Args: []any{"s1", int64(i)}}
		if err := manager.PublishSessionFacts("s1", []mangle.Fact{fact}); err != nil {
			t.Fatalf("PublishSessionFacts: %v", err)
		}
	}
	stats := manager.SessionFactStats("s1")
	if stats.Asserted != 3 || stats.Dropped != 2 || !stats.Saturated {
		t.Fatalf("stats = %+v, want 3 asserted, 2 dropped, saturated", stats)
	}
	if err := manager.CloseSession(t.Context(), "s1"); err != nil {
		t.Fatal(err)
	}
	var domRetracted int
	for _, fact := range retractor.snapshot() {
		if fact.Predicate == "dom_updated" {
			domRetracted++
		}
	}
	if domRetracted != 3 {
		t.Fatalf("retracted %d dom_updated facts, want the 3 that were asserted", domRetracted)
	}
	if err := manager.PublishSessionFacts("s1", []mangle.Fact{{Predicate: "dom_updated", Args: []any{"s1", int64(9)}}}); err == nil {
		t.Fatal("publishing to a closed session must fail instead of recreating its accounting")
	}
}
