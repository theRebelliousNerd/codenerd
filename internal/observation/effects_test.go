package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

func rejectedReceipt() EffectReceipt {
	return EffectReceipt{
		Call: "call-7", Tool: "write_file", Seq: 3,
		Effects: []ResourceEffect{
			{Path: "cfg.json", Before: "sha256:aa", After: "sha256:bb", State: "changed"},
			{Path: "other.json", Before: "sha256:cc", After: "sha256:cc", State: "unchanged"},
		},
		Validation: "rejected", Reason: "invalid JSON", Recovery: "/repair",
	}
}

// A parent used to receive the changed path and nothing about the call
// behind it: a write validation rejected projected exactly like one it
// passed. The receipt's recovery now reaches the parent as uncertainty.
func TestProjectReturn_CarriesAnUnsettledEffectReceipt(t *testing.T) {
	r := Return{Output: "done", Outcome: "/unverified", Changed: []string{"cfg.json"}, Effects: []EffectReceipt{rejectedReceipt()}}
	text := ProjectReturn(r, ReturnLimits{}).Text("")
	for _, want := range []string{"tool effect not settled", "write_file (call-7) changed cfg.json", "validation rejected (invalid JSON)", "/repair"} {
		if !strings.Contains(text, want) {
			t.Errorf("projection missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "other.json") {
		t.Errorf("an unchanged resource was reported as changed:\n%s", text)
	}
	if !strings.Contains(r.PartialState(), "owed recovery /repair") {
		t.Errorf("partial state lost the receipt:\n%s", r.PartialState())
	}
}

func TestProjectReturn_AMeasuredReceiptAddsNoUncertainty(t *testing.T) {
	measured := rejectedReceipt()
	measured.Validation, measured.Reason, measured.Recovery = "verified", "", ""
	r := Return{Output: "done", Effects: []EffectReceipt{measured}}
	if got := ProjectReturn(r, ReturnLimits{}).Uncertainty; len(got) != 0 {
		t.Fatalf("a measured change is not uncertainty: %v", got)
	}
}

// Receipts are retained with the return: identities only, round-tripping.
func TestReturn_EffectsSurviveSerialization(t *testing.T) {
	b, err := json.Marshal(Return{Effects: []EffectReceipt{rejectedReceipt()}})
	if err != nil {
		t.Fatal(err)
	}
	var back Return
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Effects) != 1 || back.Effects[0].Effects[0].After != "sha256:bb" || back.Effects[0].Recovery != "/repair" {
		t.Fatalf("round trip lost the receipt: %+v", back.Effects)
	}
}
