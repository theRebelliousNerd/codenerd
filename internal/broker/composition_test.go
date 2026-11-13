package broker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// A request's composition rides the context to its receipt, so the meter can
// say which part of a request its tokens went to; an untagged call records
// none.
func TestReceipt_WhenTheContextCarriesAComposition_ShouldRecordIt(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	client := meteredClient(t, newFakeClient(), meter, sink)

	comp := Composition{System: 4000, Catalog: 9000, Task: 300, ToolResults: 12000, Views: 25921, RepairPayload: 31000, RepairPayloads: 1, RetiredPayloads: 2}
	ctx := WithComposition(WithPhase(WithPurpose(context.Background(), PurposeSession), PhaseRepair), comp)
	if _, err := client.CompleteWithSystem(ctx, "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	r, ok := sink.last()
	if !ok || r.Composition == nil || *r.Composition != comp {
		t.Fatalf("receipt composition = %+v, want %+v", r.Composition, comp)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"composition_bytes":{"system":4000`) || !strings.Contains(string(raw), `"repair_payload":31000`) {
		t.Fatalf("the receipt line does not carry the composition: %s", raw)
	}
	if comp.Total() != 4000+9000+300+12000+25921+31000 {
		t.Fatalf("total = %d", comp.Total())
	}
	if _, err := client.CompleteWithSystem(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if r, _ := sink.last(); r.Composition != nil {
		t.Errorf("an untagged call recorded composition %+v", r.Composition)
	}
}
