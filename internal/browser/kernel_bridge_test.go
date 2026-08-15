package browser

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"codenerd/internal/types"
)

// fakeKernel implements the read and retract halves of a live Cortex kernel.
type fakeKernel struct {
	facts     []types.Fact
	queryErr  error
	retracted []types.Fact
}

func (k *fakeKernel) Query(predicate string) ([]types.Fact, error) {
	if k.queryErr != nil {
		return nil, k.queryErr
	}
	var out []types.Fact
	for _, fact := range k.facts {
		if fact.Predicate == predicate {
			out = append(out, fact)
		}
	}
	return out, nil
}

func (k *fakeKernel) RetractExactFactsBatch(facts []types.Fact) error {
	k.retracted = append(k.retracted, facts...)
	return nil
}

func TestKernelFactQuerier_WhenFilteringArgs_ShouldMatchMangleSemantics(t *testing.T) {
	kernel := &fakeKernel{facts: []types.Fact{
		{Predicate: "is_honeypot", Args: []any{"elem_1"}},
		{Predicate: "is_honeypot", Args: []any{"elem_2"}},
		{Predicate: "honeypot_reason", Args: []any{"elem_1", "/css_hidden"}},
		{Predicate: "honeypot_reason", Args: []any{"elem_2", "/offscreen"}},
	}}
	querier := NewKernelFactQuerier(kernel)

	if got := len(querier.QueryFacts("is_honeypot")); got != 2 {
		t.Errorf("unfiltered query returned %d facts, want 2", got)
	}
	if got := len(querier.QueryFacts("is_honeypot", "elem_1")); got != 1 {
		t.Errorf("filtered query returned %d facts, want 1", got)
	}
	if got := len(querier.QueryFacts("is_honeypot", "elem_9")); got != 0 {
		t.Errorf("query for an unknown element returned %d facts, want 0", got)
	}

	// A /name constant must be matchable with or without its leading slash;
	// this is the mismatch that made a name-bound predicate look empty when it
	// was queried as a plain string.
	if got := len(querier.QueryFacts("honeypot_reason", "elem_1", "css_hidden")); got != 1 {
		t.Errorf("slash-free name filter returned %d facts, want 1", got)
	}
	if got := len(querier.QueryFacts("honeypot_reason", "elem_1", "/css_hidden")); got != 1 {
		t.Errorf("slash-prefixed name filter returned %d facts, want 1", got)
	}
	if got := len(querier.QueryFacts("honeypot_reason", "", "/offscreen")); got != 1 {
		t.Errorf("wildcard first slot returned %d facts, want 1", got)
	}

	if NewKernelFactQuerier(nil) != nil {
		t.Error("a nil kernel must not produce a querier that panics later")
	}
}

func TestKernelFactQuerier_WhenKernelErrors_ShouldReturnNothing(t *testing.T) {
	querier := NewKernelFactQuerier(&fakeKernel{queryErr: errors.New("kernel down")})
	if got := querier.QueryFacts("is_honeypot", "elem_1"); got != nil {
		t.Errorf("query error must yield no facts, got %v", got)
	}
}

// TestSessionManager_WhenKernelQuerierWired_ShouldExposeDetector covers the
// production shape: the Cortex hands the manager a write-only sink, so the
// detector is unreachable until the read side is supplied explicitly.
func TestSessionManager_WhenKernelQuerierWired_ShouldExposeDetector(t *testing.T) {
	manager := NewSessionManagerWithSink(DefaultConfig(), &testEngineSinkLocal{})
	if manager.HoneypotDetector() != nil {
		t.Fatal("a write-only sink must not yield a detector that always answers no")
	}

	manager.SetFactQuerier(NewKernelFactQuerier(&fakeKernel{
		facts: []types.Fact{{Predicate: "is_honeypot", Args: []any{"elem_1"}}},
	}))
	detector := manager.HoneypotDetector()
	if detector == nil {
		t.Fatal("detector unavailable after SetFactQuerier")
	}
	if !detector.isHoneypot("elem_1") {
		t.Error("detector did not read the kernel's verdict")
	}
	if detector.isHoneypot("elem_2") {
		t.Error("detector reported a verdict the kernel never derived")
	}
}

// TestRollSessionEpoch_WhenRetractorWired_ShouldCollectRetiredFacts is the real
// garbage collection: with a retract-capable owner, the previous page's facts
// are removed rather than merely marked stale.
func TestRollSessionEpoch_WhenRetractorWired_ShouldCollectRetiredFacts(t *testing.T) {
	kernel := &fakeKernel{}
	manager := NewSessionManagerWithSink(DefaultConfig(), &testEngineSinkLocal{})
	manager.SetFactRetractor(NewKernelFactRetractor(kernel))

	if err := manager.addStreamFacts("s1", streamFactBatch("s1", 5)); err != nil {
		t.Fatalf("addStreamFacts: %v", err)
	}
	manager.RollSessionEpoch("s1")

	if len(kernel.retracted) != 5 {
		t.Fatalf("retracted %d facts on epoch roll, want 5", len(kernel.retracted))
	}
	predicates := make([]string, 0, len(kernel.retracted))
	for _, fact := range kernel.retracted {
		predicates = append(predicates, fact.Predicate)
	}
	if slices.Contains(predicates, "browser_epoch") {
		t.Error("the new epoch's own watermark must not be collected")
	}

	// The second roll has nothing left to collect.
	manager.RollSessionEpoch("s1")
	if len(kernel.retracted) != 5 {
		t.Errorf("second roll retracted more facts (%d); tracking was not cleared", len(kernel.retracted))
	}
}

// TestAddStreamFacts_WhenNoRetractor_ShouldNotRetainFacts keeps the default
// path free of the retained copy: tracking exists only to serve collection.
func TestAddStreamFacts_WhenNoRetractor_ShouldNotRetainFacts(t *testing.T) {
	manager := NewSessionManagerWithSink(DefaultConfig(), &testEngineSinkLocal{})
	if err := manager.addStreamFacts("s1", streamFactBatch("s1", 10)); err != nil {
		t.Fatalf("addStreamFacts: %v", err)
	}
	manager.budgetMu.Lock()
	tracked := len(manager.budgets["s1"].tracked)
	manager.budgetMu.Unlock()
	if tracked != 0 {
		t.Errorf("retained %d facts with no retractor wired", tracked)
	}
}

// TestSessionManager_WhenConstructed_ShouldNotStartBrowser pins the lifecycle
// contract the Cortex relies on: the manager is built during boot and injected
// into the tactile router and chat model eagerly, which is only safe because
// construction is inert. Chrome starts on the first browser action.
func TestSessionManager_WhenConstructed_ShouldNotStartBrowser(t *testing.T) {
	manager := NewSessionManagerWithSink(DefaultConfig(), &testEngineSinkLocal{})
	if manager.IsConnected() {
		t.Error("constructing a SessionManager must not connect to a browser")
	}
	if manager.ControlURL() != "" {
		t.Error("constructing a SessionManager must not launch a browser")
	}
	if got := len(manager.List()); got != 0 {
		t.Errorf("new manager reports %d sessions", got)
	}
	// Read-only surfaces stay usable before any browser exists.
	if manager.SessionFactStats("never-seen").Epoch != 1 {
		t.Error("a session that has never run should still report epoch 1")
	}
	if err := manager.CloseSession(context.Background(), "never-seen"); err != nil {
		t.Errorf("closing an unknown session should be a no-op, got %v", err)
	}
}

// TestSessionManager_WhenFirstActionRuns_ShouldStartBrowserOnDemand is the live
// half of the same contract.
func TestSessionManager_WhenFirstActionRuns_ShouldStartBrowserOnDemand(t *testing.T) {
	cfg := liveBrowserConfig(t)
	manager := NewSessionManagerWithSink(cfg, &testEngineSinkLocal{})
	defer func() { _ = manager.Shutdown(context.Background()) }()

	if manager.IsConnected() {
		t.Fatal("manager connected before any action")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := manager.CreateSession(ctx, "about:blank"); err != nil {
		t.Fatalf("first action did not start the browser: %v", err)
	}
	if !manager.IsConnected() {
		t.Error("manager is not connected after its first action")
	}
}
