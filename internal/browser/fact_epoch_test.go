package browser

import (
	"testing"

	"codenerd/internal/mangle"
)

func streamFactBatch(sessionID string, count int) []mangle.Fact {
	facts := make([]mangle.Fact, 0, count)
	for i := 0; i < count; i++ {
		facts = append(facts, mangle.Fact{
			Predicate: "console_event",
			Args:      []any{sessionID, "log", "noise", int64(i)},
		})
	}
	return facts
}

// TestStreamFactBudget_WhenEpochExhausted_ShouldStopAsserting proves the event
// stream is bounded. Before this, a tab left open on a busy page asserted into
// the kernel forever and nothing retracted any of it.
func TestStreamFactBudget_WhenEpochExhausted_ShouldStopAsserting(t *testing.T) {
	sink := &testEngineSinkLocal{}
	cfg := DefaultConfig()
	cfg.MaxEpochEventFacts = 10
	manager := NewSessionManagerWithSink(cfg, sink)

	for i := 0; i < 8; i++ {
		if err := manager.addStreamFacts("s1", streamFactBatch("s1", 4)); err != nil {
			t.Fatalf("addStreamFacts: %v", err)
		}
	}

	stats := manager.SessionFactStats("s1")
	if stats.Asserted > cfg.MaxEpochEventFacts+4 {
		t.Errorf("asserted %d facts with a budget of %d", stats.Asserted, cfg.MaxEpochEventFacts)
	}
	if stats.Dropped == 0 {
		t.Error("expected dropped facts once the budget was exhausted")
	}

	consoleFacts := len(sink.findFactsByPredicate("console_event"))
	if consoleFacts > cfg.MaxEpochEventFacts+4 {
		t.Errorf("sink received %d console facts, budget was %d", consoleFacts, cfg.MaxEpochEventFacts)
	}

	// The saturation notice must itself survive saturation, and must be
	// asserted exactly once per epoch rather than on every dropped batch.
	saturated := sink.findFactsByPredicate("browser_stream_saturated")
	if len(saturated) != 1 {
		t.Fatalf("browser_stream_saturated asserted %d times, want 1", len(saturated))
	}
	if got := saturated[0].Args[0]; got != "s1" {
		t.Errorf("saturation fact session = %v, want s1", got)
	}
}

// TestRollSessionEpoch_WhenNavigating_ShouldResetBudgetAndMarkWatermark checks
// the garbage-collection watermark: a navigation retires the previous page's
// facts and gives the session a fresh budget.
func TestRollSessionEpoch_WhenNavigating_ShouldResetBudgetAndMarkWatermark(t *testing.T) {
	sink := &testEngineSinkLocal{}
	cfg := DefaultConfig()
	cfg.MaxEpochEventFacts = 4
	manager := NewSessionManagerWithSink(cfg, sink)

	if err := manager.addStreamFacts("s1", streamFactBatch("s1", 6)); err != nil {
		t.Fatalf("addStreamFacts: %v", err)
	}
	if err := manager.addStreamFacts("s1", streamFactBatch("s1", 6)); err != nil {
		t.Fatalf("addStreamFacts: %v", err)
	}
	if manager.SessionFactStats("s1").Dropped == 0 {
		t.Fatal("expected the second batch to be dropped")
	}

	epoch := manager.RollSessionEpoch("s1")
	if epoch != 2 {
		t.Errorf("epoch after one roll = %d, want 2", epoch)
	}

	stats := manager.SessionFactStats("s1")
	if stats.Asserted != 0 || stats.Dropped != 0 {
		t.Errorf("budget was not reset by the epoch roll: %+v", stats)
	}

	watermarks := sink.findFactsByPredicate("browser_epoch")
	if len(watermarks) != 1 {
		t.Fatalf("browser_epoch asserted %d times, want 1", len(watermarks))
	}
	if got := watermarks[0].Args[1]; got != int64(2) {
		t.Errorf("browser_epoch epoch = %v (%T), want int64(2)", got, got)
	}

	// The fresh epoch accepts facts again.
	if err := manager.addStreamFacts("s1", streamFactBatch("s1", 2)); err != nil {
		t.Fatalf("addStreamFacts after roll: %v", err)
	}
	if manager.SessionFactStats("s1").Asserted != 2 {
		t.Errorf("post-roll assertions = %d, want 2", manager.SessionFactStats("s1").Asserted)
	}
}

// TestStreamFactBudget_WhenDisabled_ShouldPassEverythingThrough keeps the
// escape hatch honest for operators who want the raw stream.
func TestStreamFactBudget_WhenDisabled_ShouldPassEverythingThrough(t *testing.T) {
	sink := &testEngineSinkLocal{}
	cfg := DefaultConfig()
	cfg.MaxEpochEventFacts = -1
	manager := NewSessionManagerWithSink(cfg, sink)

	for i := 0; i < 5; i++ {
		if err := manager.addStreamFacts("s1", streamFactBatch("s1", 100)); err != nil {
			t.Fatalf("addStreamFacts: %v", err)
		}
	}
	if got := len(sink.findFactsByPredicate("console_event")); got != 500 {
		t.Errorf("sink received %d facts, want 500 with the budget disabled", got)
	}
	if manager.SessionFactStats("s1").Dropped != 0 {
		t.Error("nothing may be dropped when the budget is disabled")
	}
}

// TestCloseSession_WhenSessionGone_ShouldForgetBudget stops the accounting map
// from growing with the lifetime tab count.
func TestCloseSession_WhenSessionGone_ShouldForgetBudget(t *testing.T) {
	manager := NewSessionManagerWithSink(DefaultConfig(), &testEngineSinkLocal{})
	manager.sessions["s1"] = &sessionRecord{meta: Session{ID: "s1"}}

	if err := manager.addStreamFacts("s1", streamFactBatch("s1", 3)); err != nil {
		t.Fatalf("addStreamFacts: %v", err)
	}
	if manager.SessionFactStats("s1").Asserted != 3 {
		t.Fatal("budget did not record the batch")
	}

	if err := manager.CloseSession(t.Context(), "s1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	manager.budgetMu.Lock()
	_, tracked := manager.budgets["s1"]
	manager.budgetMu.Unlock()
	if tracked {
		t.Error("closed session still has budget accounting")
	}
}

func TestConfigHeaderIngestion_WhenModeSet_ShouldResolvePolicy(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		want    string
		ingests bool
	}{
		{"default is off for operator safety", Config{}, HeaderIngestionOff, false},
		{"legacy bool means redacted", Config{EnableHeaderIngestion: true}, HeaderIngestionRedacted, true},
		{"explicit off wins over legacy bool", Config{EnableHeaderIngestion: true, HeaderIngestionMode: "off"}, HeaderIngestionOff, false},
		{"research alias", Config{HeaderIngestionMode: "research"}, HeaderIngestionRedacted, true},
		{"unknown value falls back to the legacy bool", Config{HeaderIngestionMode: "banana"}, HeaderIngestionOff, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.GetHeaderIngestionMode(); got != tc.want {
				t.Errorf("GetHeaderIngestionMode() = %q, want %q", got, tc.want)
			}
			if got := tc.cfg.ShouldIngestHeaders(); got != tc.ingests {
				t.Errorf("ShouldIngestHeaders() = %v, want %v", got, tc.ingests)
			}
		})
	}
}

func TestConfigHoneypotGuard_WhenModeSet_ShouldResolveMode(t *testing.T) {
	cases := []struct {
		cfg  Config
		want string
	}{
		{Config{}, HoneypotGuardBlock},
		{Config{HoneypotGuard: "off"}, HoneypotGuardOff},
		{Config{HoneypotGuard: "WARN"}, HoneypotGuardWarn},
		{Config{HoneypotGuard: "block"}, HoneypotGuardBlock},
		{Config{HoneypotGuard: "nonsense"}, HoneypotGuardBlock},
	}
	for _, tc := range cases {
		if got := tc.cfg.GetHoneypotGuard(); got != tc.want {
			t.Errorf("GetHoneypotGuard(%q) = %q, want %q", tc.cfg.HoneypotGuard, got, tc.want)
		}
	}
}
