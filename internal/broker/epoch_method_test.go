package broker

import (
	"testing"
	"time"
)

func methodReceipt(scope, prefix, method string, at time.Time) Receipt {
	r := receiptAt(scope, "anthropic", "m", prefix, at)
	r.Method = method
	return r
}

// The epoch carries the shape of the call that opened it, because without that
// the median is a number two different findings can produce.
func TestSegmentRecordsTheOpeningMethod(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	epochs := Segment([]Receipt{
		methodReceipt("s1", "A", "CompleteWithToolResults", t0),
		methodReceipt("s1", "A", "CompleteWithToolResults", t0.Add(time.Second)),
		methodReceipt("s1", "B", "CompleteWithSystem", t0.Add(2*time.Second)),
	})
	if len(epochs) != 2 {
		t.Fatalf("epochs = %d, want 2", len(epochs))
	}
	if epochs[0].Method != "CompleteWithToolResults" {
		t.Errorf("first epoch method = %q", epochs[0].Method)
	}
	if epochs[1].Method != "CompleteWithSystem" {
		t.Errorf("second epoch method = %q", epochs[1].Method)
	}
}

// The case the split exists for, and the reason the aggregate alone would send
// Phase 4 the wrong way.
//
// Here the headline is discouraging — the median epoch is one call and most
// epochs are singletons — while every singleton belongs to plain completions,
// which have no loop to lengthen and which no cache strategy can help. The
// tool-calling shape, which carries most of the calls, has no singletons at
// all. Same data, opposite conclusion.
func TestHistogramByMethodSeparatesAnAmbiguousMedian(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	var rs []Receipt

	// Six plain completions, each its own prefix: six singleton epochs.
	for i := 0; i < 6; i++ {
		rs = append(rs, methodReceipt("s1", string(rune('a'+i)), "CompleteWithSystem",
			t0.Add(time.Duration(i)*time.Second)))
	}
	// One tool loop of eight rounds sharing a prefix: one epoch, eight calls.
	for i := 0; i < 8; i++ {
		rs = append(rs, methodReceipt("s2", "loop", "CompleteWithToolResults",
			t0.Add(time.Duration(100+i)*time.Second)))
	}

	h := Histogram(Segment(rs))

	// The aggregate reads badly.
	if h.P50 != 1 {
		t.Fatalf("p50 = %d, want 1 — the fixture is meant to produce a discouraging aggregate", h.P50)
	}
	if h.Singletons != 6 {
		t.Fatalf("singletons = %d, want 6", h.Singletons)
	}

	byMethod := map[string]MethodVerdict{}
	for _, mv := range h.ByMethod {
		byMethod[mv.Method] = mv
	}
	if len(byMethod) != 2 {
		t.Fatalf("ByMethod covered %d shapes, want 2: %+v", len(byMethod), h.ByMethod)
	}

	plain := byMethod["CompleteWithSystem"]
	if plain.Singletons != 6 || plain.Epochs != 6 {
		t.Errorf("plain completions: epochs=%d singletons=%d, want 6 and 6", plain.Epochs, plain.Singletons)
	}

	loop := byMethod["CompleteWithToolResults"]
	if loop.Singletons != 0 {
		t.Errorf("tool loop reported %d singletons; every singleton in this fixture "+
			"belongs to the plain completions, which is the whole point of the split", loop.Singletons)
	}
	if loop.Calls != 8 || loop.MaxCalls != 8 {
		t.Errorf("tool loop: calls=%d longest=%d, want 8 and 8", loop.Calls, loop.MaxCalls)
	}
	if loop.Mean != 8 {
		t.Errorf("tool loop mean = %v, want 8", loop.Mean)
	}

	// Most calls first: the shape carrying the spend is the one whose epoch
	// length decides whether the controller is worth building.
	if h.ByMethod[0].Method != "CompleteWithToolResults" {
		t.Errorf("ByMethod[0] = %q, want the shape with the most calls first", h.ByMethod[0].Method)
	}
}

// Receipts written before Method was recorded must not be silently attributed
// to some other shape.
func TestHistogramByMethodKeepsUnrecordedShapesSeparate(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	h := Histogram(Segment([]Receipt{
		methodReceipt("s1", "A", "", t0),
		methodReceipt("s2", "B", "CompleteWithSystem", t0.Add(time.Second)),
	}))
	if len(h.ByMethod) != 2 {
		t.Fatalf("ByMethod = %+v, want the unrecorded shape kept as its own row", h.ByMethod)
	}
	var sawEmpty bool
	for _, mv := range h.ByMethod {
		if mv.Method == "" {
			sawEmpty = true
			if mv.Epochs != 1 {
				t.Errorf("unrecorded shape epochs = %d, want 1", mv.Epochs)
			}
		}
	}
	if !sawEmpty {
		t.Error("an epoch with no recorded method was folded into another shape")
	}
}
