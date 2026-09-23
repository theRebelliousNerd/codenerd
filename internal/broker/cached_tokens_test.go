package broker

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

// recordingCounter captures what calibration is fed.
type recordingCounter struct{ observed []Observation }

func (r *recordingCounter) Count(context.Context, *Request) (Count, error) { return Count{}, nil }
func (r *recordingCounter) Observe(o Observation)                          { r.observed = append(r.observed, o) }

// A cache hit is part of the input the provider billed, not an addition to
// it. Every client reports InputTokens including cache reads, so the broker
// must feed calibration and reconciliation exactly InputTokens. It used to add
// CachedTokens back: on a round with 55k input of which 40k were cached, the
// counter learned 95k and the reconciler was billed 95k.
func TestSettle_ACacheHitIsNotCountedTwice(t *testing.T) {
	counter := &recordingCounter{}
	reconciler := NewReconciler()
	c := &core{cfg: Config{
		Counter:    counter,
		Ledger:     NewLedger(LedgerConfig{}),
		Reconciler: reconciler,
	}}
	req := &Request{Model: "muse-spark-1.3-contributor", System: "stable prefix", User: "the task"}
	receipt := Receipt{Started: time.Now()}
	receipt.Estimated.Tokens = 55_000
	reported := &types.UsageMetadata{InputTokens: 55_000, OutputTokens: 900, CachedContentTokens: 40_000}

	c.settle(receipt, req, &callObserver{}, reported, nil)

	if len(counter.observed) != 1 {
		t.Fatalf("calibration observed %d times, want 1", len(counter.observed))
	}
	if got := counter.observed[0].ActualInputTokens; got != 55_000 {
		t.Errorf("calibration learned %d input tokens, want the billed 55000 (cache reads are inside it)", got)
	}
	drift := reconciler.Drift()
	if len(drift) != 1 || drift[0].ActualTotal != 55_000 {
		t.Errorf("reconciler billed %+v, want ActualTotal 55000", drift)
	}
}
