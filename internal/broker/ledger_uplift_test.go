package broker

import (
	"context"
	"errors"
	"testing"
)

// A client wrapped before a cap change must enforce the new caps: broker cores
// capture the ledger pointer at Wrap time, so caps have to change in place.
// Swapping the meter's ledger would leave live clients metering against a
// discarded object while only future clients saw the new limits.
func TestSetBudgets_BindsLiveClients(t *testing.T) {
	ledger := NewLedger(LedgerConfig{Window: 100000})
	client, err := Wrap(newFakeClient(), Config{
		Provider: "test",
		Model:    "m",
		Counter:  fixedCounter{tokens: 4000, confidence: ConfidenceExact},
		Ledger:   ledger,
	})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	ctx := WithPurpose(context.Background(), PurposeCritic)

	if _, err := client.Complete(ctx, "first"); err != nil {
		t.Fatalf("uncapped call failed: %v", err)
	}
	// 4000 counted, 100 reported in + 50 out: the cap below binds on actuals.
	ledger.SetBudgets(map[Purpose]int64{PurposeCritic: 200})
	if _, err := client.Complete(ctx, "second"); err == nil {
		t.Fatal("live client admitted a call over the new cap; caps must bind live clients")
	} else {
		var adm *AdmissionError
		if !errors.As(err, &adm) || adm.Decision.Code != DecisionBudgetExhausted {
			t.Fatalf("wrong refusal: %v", err)
		}
	}
	if got := ledger.Account(PurposeCritic).Total(); got != 150 {
		t.Fatalf("spend after cap change = %d, want the pre-existing 150", got)
	}
}

// SetBudgets with no positive caps clears every cap: policy removal must be
// expressible, not just policy tightening.
func TestSetBudgets_EmptyClearsCaps(t *testing.T) {
	ledger := NewLedger(LedgerConfig{Budgets: map[Purpose]int64{PurposeCritic: 10}})
	ledger.SetBudgets(map[Purpose]int64{PurposeCritic: 0})
	d := ledger.Admit(PurposeCritic, Count{Tokens: 500, Confidence: ConfidenceExact})
	if !d.Allowed {
		t.Fatalf("cleared caps still refuse: %s", d.Reason)
	}
}
