package broker

import (
	"math"
	"sync"
	"testing"
)

func TestCalibrationConvergesOnObservedRatio(t *testing.T) {
	cal := NewCalibratorWithSeed(4.0)
	const model = "conv"
	const trueRatio = 2.75

	if _, conf := cal.Ratio(model); conf != ConfidenceSeeded {
		t.Fatalf("before any observation confidence should be seeded, got %q", conf)
	}

	// Feed observations drawn from a model that really packs 2.75 chars per
	// token. The whole claim of this package is that the estimate stops being
	// a guess as evidence arrives.
	for i := 0; i < 20; i++ {
		chars := 4000 + i*137
		cal.Observe(Observation{
			Model:             model,
			Chars:             chars,
			ActualInputTokens: int(float64(chars) / trueRatio),
		})
	}

	got, conf := cal.Ratio(model)
	if conf != ConfidenceCalibrated {
		t.Errorf("after observations confidence should be calibrated, got %q", conf)
	}
	if math.Abs(got-trueRatio) > 0.05 {
		t.Errorf("ratio did not converge: got %.4f, want ~%.2f", got, trueRatio)
	}
	if n := cal.Observations(model); n != 20 {
		t.Errorf("observations = %d, want 20", n)
	}
}

func TestCalibrationFirstObservationReplacesSeed(t *testing.T) {
	// The first response must dominate. A slow ramp from a wrong seed means the
	// first several admissions of a session are made on a number nobody chose.
	cal := NewCalibratorWithSeed(10.0)
	cal.Observe(Observation{Model: "m", Chars: 3000, ActualInputTokens: 1000})

	got, _ := cal.Ratio("m")
	if math.Abs(got-3.0) > 0.001 {
		t.Errorf("first observation should replace the seed outright: got %.4f, want 3.0", got)
	}
}

func TestCalibrationRejectsUnusableObservations(t *testing.T) {
	cases := []struct {
		name string
		obs  Observation
	}{
		{"no model", Observation{Chars: 4000, ActualInputTokens: 1000}},
		{"sample too small", Observation{Model: "m", Chars: 10, ActualInputTokens: 3}},
		{"zero tokens", Observation{Model: "m", Chars: 4000, ActualInputTokens: 0}},
		{"negative tokens", Observation{Model: "m", Chars: 4000, ActualInputTokens: -5}},
		{"ratio absurdly high", Observation{Model: "m", Chars: 100000, ActualInputTokens: 1}},
		{"ratio below one token per char", Observation{Model: "m", Chars: 1000, ActualInputTokens: 2000}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cal := NewCalibratorWithSeed(4.0)
			cal.Observe(tc.obs)
			if n := cal.Observations("m"); n != 0 {
				t.Errorf("unusable observation was accepted (n=%d)", n)
			}
			if got, conf := cal.Ratio("m"); got != 4.0 || conf != ConfidenceSeeded {
				t.Errorf("ratio moved on an unusable observation: %.4f (%s)", got, conf)
			}
		})
	}
}

func TestCalibrationResistsSingleOutlierOnceTrained(t *testing.T) {
	cal := NewCalibratorWithSeed(4.0)
	const model = "outlier"

	for i := 0; i < 30; i++ {
		cal.Observe(Observation{Model: model, Chars: 4000, ActualInputTokens: 1000}) // ratio 4.0
	}
	before, _ := cal.Ratio(model)

	// One wild-but-in-band sample must not relocate a well-trained ratio.
	cal.Observe(Observation{Model: model, Chars: 19000, ActualInputTokens: 1000}) // ratio 19.0
	after, _ := cal.Ratio(model)

	if after <= before {
		t.Fatalf("outlier had no effect at all: %.4f -> %.4f", before, after)
	}
	if after > before+(19.0-before)*minAlpha+0.001 {
		t.Errorf("outlier moved the ratio more than the alpha floor permits: %.4f -> %.4f", before, after)
	}
}

func TestCalibrationClampsIntoSaneBand(t *testing.T) {
	cal := NewCalibratorWithSeed(4.0)
	for i := 0; i < 50; i++ {
		cal.Observe(Observation{Model: "hi", Chars: 20000, ActualInputTokens: 1001}) // ~19.98
		cal.Observe(Observation{Model: "lo", Chars: 1000, ActualInputTokens: 999})   // ~1.001
	}

	hi, _ := cal.Ratio("hi")
	lo, _ := cal.Ratio("lo")
	if hi > maxRatio {
		t.Errorf("ratio exceeded the upper clamp: %.4f > %.4f", hi, maxRatio)
	}
	if lo < minRatio {
		t.Errorf("ratio fell below the lower clamp: %.4f < %.4f", lo, minRatio)
	}
}

func TestCalibrationSeedFallsBackWhenOutOfRange(t *testing.T) {
	for _, seed := range []float64{0, -1, 1e9} {
		cal := NewCalibratorWithSeed(seed)
		if got, _ := cal.Ratio("x"); got != DefaultSeedRatio {
			t.Errorf("seed %v should fall back to %.2f, got %.4f", seed, DefaultSeedRatio, got)
		}
	}
}

func TestCalibrationIsRaceFree(t *testing.T) {
	cal := NewCalibrator()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(2)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				cal.Observe(Observation{Model: "shared", Chars: 4000 + i, ActualInputTokens: 1000 + i/4})
			}
		}(w)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_, _ = cal.Ratio("shared")
				_ = cal.Observations("shared")
				_ = cal.Snapshot()
			}
		}()
	}
	wg.Wait()

	if n := cal.Observations("shared"); n != 4000 {
		t.Errorf("lost observations under concurrency: got %d, want 4000", n)
	}
}

func TestEstimateNeverReturnsZeroForNonEmptyContent(t *testing.T) {
	cal := NewCalibrator()
	tokens, _, _ := cal.Estimate("m", 1)
	if tokens < 1 {
		t.Errorf("one character estimated at %d tokens; a floor of 1 is what stops an unbounded "+
			"number of tiny items entering a budget that believed it was full", tokens)
	}
}
