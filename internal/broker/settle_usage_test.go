package broker

import (
	"testing"
	"time"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// settleWith runs one settlement on a bare core and returns the receipt, the
// ledger total and what calibration was fed.
func settleWith(t *testing.T, reports []types.UsageMetadata, reported *types.UsageMetadata) (Receipt, Spend, []Observation) {
	t.Helper()
	counter := &recordingCounter{}
	sink := &captureSink{}
	ledger := NewLedger(LedgerConfig{})
	c := &core{cfg: Config{Counter: counter, Ledger: ledger, Sink: sink}}
	obs := &callObserver{}
	for _, u := range reports {
		obs.Observed(usage.Report{Model: "m", Provider: "anthropic", Operation: "chat", Usage: u})
	}
	receipt := Receipt{Started: time.Now()}
	receipt.Estimated.Tokens = 1000
	c.settle(receipt, &Request{Model: "m", User: "the task"}, obs, reported, nil)
	r, ok := sink.last()
	if !ok {
		t.Fatal("settle emitted no receipt")
	}
	return r, ledger.Total(), counter.observed
}

var anthropicKnown = types.UsageTokens | types.UsageCacheRead | types.UsageCacheWrite

// One bill can arrive by both routes. Agreement is one charge; disagreement
// is evidence on the receipt, and the observed report is what is charged.
func TestSettle_DualSourceIsOneChargeAndDisagreementIsRecorded(t *testing.T) {
	observed := types.UsageMetadata{InputTokens: 1000, OutputTokens: 100, CachedContentTokens: 400, Known: anthropicKnown}

	r, total, _ := settleWith(t, []types.UsageMetadata{observed}, &observed)
	if r.UsageConflict != "" || total.InputTokens != 1000 || total.Calls != 1 {
		t.Fatalf("agreeing routes: conflict %q, ledger %+v; want no conflict and one 1000-token charge", r.UsageConflict, total)
	}

	conflicting := observed
	conflicting.InputTokens = 1700
	r, total, _ = settleWith(t, []types.UsageMetadata{observed}, &conflicting)
	if r.UsageConflict == "" {
		t.Fatal("disagreeing routes recorded no conflict")
	}
	if total.InputTokens != 1000 {
		t.Fatalf("ledger input = %d, want the observed 1000: a disagreement is never a second charge", total.InputTokens)
	}
}

// Missing, partial and observed-zero usage are three different receipts. A zero
// Actual alone cannot say which, and a policy reading it as a cheap call would
// be wrong in two of the three cases.
func TestSettle_PresenceSemanticsAreDistinguishable(t *testing.T) {
	// No report at all: a CLI-style engine that bills nothing visible.
	r, _, _ := settleWith(t, nil, nil)
	if r.UsageStatus != UsageUnreported {
		t.Errorf("no usage: status %q, want %q", r.UsageStatus, UsageUnreported)
	}

	// A true zero cache: reported, and the cache categories are known.
	zero := types.UsageMetadata{InputTokens: 500, OutputTokens: 50, Known: anthropicKnown}
	r, _, _ = settleWith(t, []types.UsageMetadata{zero}, nil)
	if r.UsageStatus != UsageReported || r.UsageKnown&types.UsageCacheRead == 0 || r.Actual.CachedTokens != 0 {
		t.Errorf("observed zero cache: status %q known %b cached %d", r.UsageStatus, r.UsageKnown, r.Actual.CachedTokens)
	}

	// Unknown cache: same zero count, but the category was never reported.
	unknown := types.UsageMetadata{InputTokens: 500, OutputTokens: 50, Known: types.UsageTokens}
	r, _, _ = settleWith(t, []types.UsageMetadata{unknown}, nil)
	if r.UsageKnown&types.UsageCacheRead != 0 {
		t.Errorf("unreported cache marked known: %b", r.UsageKnown)
	}

	// A canceled stream: counts are lower bounds and calibration is not fed.
	partial := types.UsageMetadata{InputTokens: 500, Known: types.UsageTokens, Partial: true}
	r, _, observedCal := settleWith(t, []types.UsageMetadata{partial}, nil)
	if r.UsageStatus != UsagePartial {
		t.Errorf("partial usage: status %q, want %q", r.UsageStatus, UsagePartial)
	}
	if len(observedCal) != 0 {
		t.Errorf("a partial count calibrated the estimator: %+v", observedCal)
	}
}

// Every billed attempt is spend, but a multi-attempt total is not the bill of
// the one request the estimator counted.
func TestSettle_RetryAttemptsSumButDoNotCalibrate(t *testing.T) {
	attempt := types.UsageMetadata{InputTokens: 1000, OutputTokens: 10, Known: types.UsageTokens}
	r, total, observedCal := settleWith(t, []types.UsageMetadata{attempt, attempt}, nil)
	if r.Attempts != 2 || total.InputTokens != 2000 {
		t.Fatalf("attempts %d, ledger input %d; want 2 attempts summing to 2000", r.Attempts, total.InputTokens)
	}
	if len(observedCal) != 0 {
		t.Fatalf("calibration trained on a two-attempt aggregate: %+v", observedCal)
	}

	_, _, observedCal = settleWith(t, []types.UsageMetadata{attempt}, nil)
	if len(observedCal) != 1 || observedCal[0].ActualInputTokens != 1000 {
		t.Fatalf("single attempt calibration = %+v, want one observation of 1000", observedCal)
	}
}

// A response block that agrees on everything both routes know fills in only
// the categories the observer lacked; it never overwrites one it had.
func TestSettle_ResponseEnrichesOnlyUnknownCategories(t *testing.T) {
	observed := types.UsageMetadata{InputTokens: 800, OutputTokens: 300, Known: types.UsageTokens}
	reported := types.UsageMetadata{InputTokens: 800, OutputTokens: 300, ThinkingTokens: 120, Known: types.UsageTokens | types.UsageReasoning}
	r, _, _ := settleWith(t, []types.UsageMetadata{observed}, &reported)
	if r.Actual.ThinkingTokens != 120 || r.UsageKnown&types.UsageReasoning == 0 || r.UsageConflict != "" {
		t.Fatalf("receipt thinking %d known %b conflict %q; want reasoning enriched from the response", r.Actual.ThinkingTokens, r.UsageKnown, r.UsageConflict)
	}
}
