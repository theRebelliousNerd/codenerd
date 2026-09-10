package broker

import (
	"sync"
	"testing"
)

func TestReconcilerReportsNothingBelowTheSampleFloor(t *testing.T) {
	// Early calls on a fresh model are served by the seeded ratio and are
	// expected to be wrong. Alarming on them trains operators to ignore alarms.
	r := NewReconciler()
	for i := 0; i < reconcileMinSamples-1; i++ {
		r.Observe("m", 2000, 1000) // 100% error
	}

	drift := r.Drift()
	if len(drift) != 1 {
		t.Fatalf("expected one model, got %d", len(drift))
	}
	if !drift[0].Trustworthy {
		t.Error("a model below the sample floor must not be reported as untrustworthy: " +
			"it has not failed, it has not been tested")
	}
	if drift[0].Samples != reconcileMinSamples-1 {
		t.Errorf("samples = %d", drift[0].Samples)
	}
}

func TestReconcilerFlagsPersistentDrift(t *testing.T) {
	r := NewReconciler()
	for i := 0; i < reconcileMinSamples*2; i++ {
		r.Observe("drifting", 1500, 1000) // consistently +50%
	}

	drift := r.Drift()
	if drift[0].Trustworthy {
		t.Error("a model 50% off across 40 calls was reported as trustworthy")
	}
	if drift[0].MeanAbsErrorPct < 49 || drift[0].MeanAbsErrorPct > 51 {
		t.Errorf("MeanAbsErrorPct = %.1f, want ~50", drift[0].MeanAbsErrorPct)
	}
	if drift[0].NetBiasPct < 49 || drift[0].NetBiasPct > 51 {
		t.Errorf("NetBiasPct = %.1f, want ~50 for a one-directional error", drift[0].NetBiasPct)
	}
}

func TestReconcilerDoesNotLetOvershootAndUndershootCancel(t *testing.T) {
	// This is why mean absolute error is the headline and net bias is not.
	// Totals alone would score a wildly inaccurate estimator as perfect.
	r := NewReconciler()
	for i := 0; i < reconcileMinSamples; i++ {
		r.Observe("noisy", 1300, 1000) // +30%
		r.Observe("noisy", 700, 1000)  // -30%
	}

	drift := r.Drift()
	if drift[0].NetBiasPct > 1 || drift[0].NetBiasPct < -1 {
		t.Errorf("NetBiasPct = %.1f, want ~0: the two directions should cancel in the totals",
			drift[0].NetBiasPct)
	}
	if drift[0].MeanAbsErrorPct < 29 || drift[0].MeanAbsErrorPct > 31 {
		t.Errorf("MeanAbsErrorPct = %.1f, want ~30: per-call error must not cancel",
			drift[0].MeanAbsErrorPct)
	}
	if drift[0].Trustworthy {
		t.Error("an estimator wrong by 30% on every single call was reported as trustworthy " +
			"because its errors happened to balance")
	}
}

func TestReconcilerAcceptsAnAccurateEstimator(t *testing.T) {
	r := NewReconciler()
	for i := 0; i < reconcileMinSamples*2; i++ {
		r.Observe("good", 1020, 1000) // +2%
	}
	if !r.Drift()[0].Trustworthy {
		t.Error("a model within 2% was flagged")
	}
}

func TestReconcilerIgnoresUnusableObservations(t *testing.T) {
	r := NewReconciler()
	r.Observe("", 100, 100)
	r.Observe("m", 0, 100)
	r.Observe("m", 100, 0)
	r.Observe("m", -5, 100)
	if len(r.Drift()) != 0 {
		t.Errorf("unusable observations were recorded: %+v", r.Drift())
	}
}

func TestReconcilerRanksWorstFirst(t *testing.T) {
	r := NewReconciler()
	for i := 0; i < reconcileMinSamples; i++ {
		r.Observe("fine", 1010, 1000)
		r.Observe("bad", 1800, 1000)
		r.Observe("middling", 1200, 1000)
	}

	drift := r.Drift()
	if len(drift) != 3 {
		t.Fatalf("expected 3 models, got %d", len(drift))
	}
	if drift[0].Model != "bad" || drift[2].Model != "fine" {
		t.Errorf("not ranked worst-first: %s, %s, %s",
			drift[0].Model, drift[1].Model, drift[2].Model)
	}
}

func TestReconcilerIsRaceFree(t *testing.T) {
	r := NewReconciler()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				r.Observe("shared", 1050, 1000)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				_ = r.Drift()
			}
		}()
	}
	wg.Wait()

	if got := r.Drift()[0].Samples; got != 2400 {
		t.Errorf("lost observations under concurrency: %d, want 2400", got)
	}
}

func TestBrokerFeedsTheReconcilerFromRealCalls(t *testing.T) {
	// End to end: the meter's reconciler must see what actually happened, not
	// what the estimator predicted.
	meter := testMeter(1000000, 8000)
	fake := newFakeClient()
	fake.reportInput, fake.reportOutput = 1000, 10

	cfg := meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	cfg.Counter = fixedCounter{tokens: 1500, confidence: ConfidenceSeeded}
	client, err := Wrap(fake, cfg)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	for i := 0; i < reconcileMinSamples; i++ {
		if _, err := client.Complete(nil2ctx(), "prompt"); err != nil {
			t.Fatalf("Complete: %v", err)
		}
	}

	drift := meter.Drift()
	if len(drift) == 0 {
		t.Fatal("the reconciler saw no calls; the settle path is not feeding it")
	}
	if drift[0].Trustworthy {
		t.Error("a counter overshooting by 50% on every call was not flagged")
	}
	if drift[0].ActualTotal != int64(1000*reconcileMinSamples) {
		t.Errorf("ActualTotal = %d, want %d", drift[0].ActualTotal, 1000*reconcileMinSamples)
	}
}
