package usage

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// A dollar figure without the rates behind it cannot be reproduced: the table
// is list price and RegisterPrice overrides it. Each priced event names the
// entry it used, and an unpriced model carries no version (unpriced spend, not
// free spend).
func TestEventsNameThePriceEntryThatPricedThem(t *testing.T) {
	tr, _ := newTestTracker(t, WithEventLog())
	use := types.UsageMetadata{InputTokens: 1000, OutputTokens: 100}
	tr.Record(context.Background(), Report{Model: "gpt-4o", Provider: "openai", Operation: "chat", Usage: use})
	tr.Record(context.Background(), Report{Model: "no-such-model-zz", Provider: "x", Operation: "chat", Usage: use})

	evs := tr.Events()
	if len(evs) != 2 {
		t.Fatalf("events = %d", len(evs))
	}
	if evs[0].PriceVersion == "" || evs[0].CostUSD == 0 {
		t.Errorf("priced event = %+v, want a price version", evs[0])
	}
	if evs[1].PriceVersion != "" || evs[1].CostUSD != 0 {
		t.Errorf("unpriced event = %+v, want no version and no cost", evs[1])
	}

	// The same model under different rates is a different version.
	_, before, _ := LookupPriceVersion("zz-negotiated-model")
	if before != "" {
		t.Fatal("test model unexpectedly priced")
	}
	RegisterPrice("zz-negotiated-model", Price{InputPerMTok: 1, OutputPerMTok: 2})
	_, v1, _ := LookupPriceVersion("zz-negotiated-model-1")
	RegisterPrice("zz-negotiated-model", Price{InputPerMTok: 1, OutputPerMTok: 3})
	_, v2, _ := LookupPriceVersion("zz-negotiated-model-1")
	if v1 == "" || v1 == v2 {
		t.Errorf("versions %q, %q: a changed rate must change the version", v1, v2)
	}
}
