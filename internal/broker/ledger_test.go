package broker

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestLedgerAdmitsWithinWindow(t *testing.T) {
	l := NewLedger(LedgerConfig{Window: 100000, OutputReserve: 8000})

	d := l.Admit(PurposeSession, Count{Tokens: 50000, Confidence: ConfidenceExact})
	if !d.Allowed || d.Code != DecisionAdmitted {
		t.Fatalf("in-window request refused: %+v", d)
	}
	if want := 92000 - 50000; d.Headroom != want {
		t.Errorf("headroom = %d, want %d", d.Headroom, want)
	}
}

func TestLedgerRefusesWhenRequestExceedsWindowMinusReserve(t *testing.T) {
	l := NewLedger(LedgerConfig{Window: 100000, OutputReserve: 8000})

	// 95k fits the raw window but leaves the model nowhere to write its answer.
	// Refusing locally is strictly cheaper than being rejected by the API after
	// transmitting, and being billed for the attempt.
	d := l.Admit(PurposeSession, Count{Tokens: 95000, Confidence: ConfidenceExact})
	if d.Allowed {
		t.Fatal("request that leaves no room for the response was admitted")
	}
	if d.Code != DecisionWindowExceeded {
		t.Errorf("code = %q, want %q", d.Code, DecisionWindowExceeded)
	}
	if d.Reason == "" {
		t.Error("a refusal must carry a reason an operator can act on")
	}
}

func TestLedgerFailsClosedOnUncountableRequest(t *testing.T) {
	l := NewLedger(LedgerConfig{Window: 100000, OutputReserve: 8000})

	d := l.Admit(PurposeSession, Count{Tokens: 0, Confidence: ConfidenceSeeded})
	if d.Allowed {
		t.Fatal("a request with no token count was admitted: a budget that cannot be checked is not enforced")
	}
	if d.Code != DecisionCountUnavailable {
		t.Errorf("code = %q, want %q", d.Code, DecisionCountUnavailable)
	}
}

func TestLedgerAdmitsOnConfidenceItCannotImprove(t *testing.T) {
	// The ledger no longer refuses a count for being an estimate. The opt-in
	// that used to gate that -- WithRequireExact -- had no caller anywhere, so
	// the check could only ever be skipped, and a safety property nothing can
	// switch on is decoration.
	//
	// Confidence is still carried on every Count and every Receipt, which is
	// what makes a measurement distinguishable from a guess after the fact.
	// What is gone is the pre-flight refusal, and it comes back with its first
	// real caller rather than ahead of one.
	l := NewLedger(LedgerConfig{Window: 100000, OutputReserve: 8000})

	for _, conf := range []Confidence{ConfidenceExact, ConfidenceCalibrated, ConfidenceSeeded} {
		d := l.Admit(PurposeSession, Count{Tokens: 1000, Confidence: conf})
		if !d.Allowed {
			t.Errorf("a %s count that fits the window was refused: %+v", conf, d)
		}
		if d.Count.Confidence != conf {
			t.Errorf("decision dropped the confidence: got %q, want %q", d.Count.Confidence, conf)
		}
	}
}

func TestLedgerEnforcesPerPurposeBudget(t *testing.T) {
	l := NewLedger(LedgerConfig{
		Window:        1000000,
		OutputReserve: 0,
		Budgets:       map[Purpose]int64{PurposeCompression: 10000},
	})

	l.Record(PurposeCompression, Spend{InputTokens: 6000, OutputTokens: 3000, Calls: 1})

	// 9000 spent of 10000; a 2000-token request does not fit.
	d := l.Admit(PurposeCompression, Count{Tokens: 2000, Confidence: ConfidenceExact})
	if d.Allowed {
		t.Fatal("purpose budget was not enforced")
	}
	if d.Code != DecisionBudgetExhausted {
		t.Errorf("code = %q, want %q", d.Code, DecisionBudgetExhausted)
	}

	// A different purpose is unaffected: budgets are per-account, not global.
	if d := l.Admit(PurposeSession, Count{Tokens: 2000, Confidence: ConfidenceExact}); !d.Allowed {
		t.Error("one purpose exhausting its budget must not block another")
	}
}

func TestLedgerAdmitsWhenWindowUnknownButReportsNoHeadroom(t *testing.T) {
	// An unconfigured window is "we do not know the limit", not "the check
	// passed". It must admit — refusing every call before config loads would
	// brick boot — but it must be visible as zero headroom on every receipt.
	l := NewLedger(LedgerConfig{})

	d := l.Admit(PurposeSession, Count{Tokens: 5_000_000, Confidence: ConfidenceExact})
	if !d.Allowed {
		t.Fatal("an unconfigured window must not refuse")
	}
	if d.Window != 0 || d.Headroom != 0 {
		t.Errorf("unconfigured window should report zero window and headroom, got window=%d headroom=%d",
			d.Window, d.Headroom)
	}
}

func TestLedgerRecordsSpendPerPurposeAndInTotal(t *testing.T) {
	l := NewLedger(LedgerConfig{Window: 100000})

	l.Record(PurposeSession, Spend{InputTokens: 100, OutputTokens: 20, Calls: 1})
	l.Record(PurposeSession, Spend{InputTokens: 50, OutputTokens: 10, Calls: 1})
	l.Record(PurposePerception, Spend{InputTokens: 7, OutputTokens: 3, Calls: 1})

	session := l.Account(PurposeSession)
	if session.InputTokens != 150 || session.OutputTokens != 30 || session.Calls != 2 {
		t.Errorf("session account = %+v", session)
	}

	total := l.Total()
	if total.InputTokens != 157 || total.OutputTokens != 33 || total.Calls != 3 {
		t.Errorf("total = %+v", total)
	}

	if got := l.Account(PurposeCampaign); got.Calls != 0 {
		t.Errorf("an untouched account should be zero, got %+v", got)
	}
}

func TestLedgerIgnoresEmptySpend(t *testing.T) {
	// A refused call spends nothing. Recording a zero would inflate the call
	// count, which is the denominator of every per-call figure downstream.
	l := NewLedger(LedgerConfig{Window: 1000})
	l.Record(PurposeSession, Spend{})
	if l.Total().Calls != 0 {
		t.Errorf("an empty spend was recorded: %+v", l.Total())
	}
}

func TestLedgerSetWindowUpdatesEnforcement(t *testing.T) {
	l := NewLedger(LedgerConfig{})
	if l.Available() != 0 {
		t.Fatalf("unconfigured ledger should report zero available, got %d", l.Available())
	}

	l.SetWindow(50000, 5000)
	if got := l.Available(); got != 45000 {
		t.Errorf("Available() = %d, want 45000", got)
	}

	if d := l.Admit(PurposeSession, Count{Tokens: 46000, Confidence: ConfidenceExact}); d.Allowed {
		t.Error("a request over the newly-learned window was admitted")
	}
}

func TestLedgerResetClearsSpendButKeepsLimits(t *testing.T) {
	l := NewLedger(LedgerConfig{Window: 50000, OutputReserve: 5000})
	l.Record(PurposeSession, Spend{InputTokens: 1000, Calls: 1})

	l.Reset()

	if l.Total().Calls != 0 {
		t.Error("Reset must clear recorded spend")
	}
	if l.Available() != 45000 {
		t.Errorf("Reset must not clear limits: Available() = %d, want 45000", l.Available())
	}
}

func TestLedgerIsRaceFree(t *testing.T) {
	l := NewLedger(LedgerConfig{Window: 1000000, OutputReserve: 1000})

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for i := 0; i < 400; i++ {
				l.Record(PurposeSession, Spend{InputTokens: 1, OutputTokens: 1, Calls: 1})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 400; i++ {
				l.Admit(PurposeSession, Count{Tokens: 10, Confidence: ConfidenceExact})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 400; i++ {
				_ = l.Total()
				_ = l.Accounts()
				_ = l.Available()
			}
		}()
	}
	wg.Wait()

	if got := l.Total().Calls; got != 3200 {
		t.Errorf("lost writes under concurrency: calls = %d, want 3200", got)
	}
}

func TestLedgerAccountsReturnsCopy(t *testing.T) {
	// A caller mutating the returned map must not be able to rewrite the
	// ledger's balances.
	l := NewLedger(LedgerConfig{Window: 1000})
	l.Record(PurposeSession, Spend{InputTokens: 10, Calls: 1})

	snapshot := l.Accounts()
	snapshot[PurposeSession] = Spend{InputTokens: 999999}

	if l.Account(PurposeSession).InputTokens != 10 {
		t.Error("Accounts() exposed internal state to mutation")
	}
}

// TestIsAdmissionErrorSeesThroughWrapping is the case the original type
// assertion could not handle. A refusal raised inside perception reaches the
// caller as "observation failed: %w", so an assertion on the outermost type
// answers false for every path a refusal actually travels -- and the function
// exists precisely so a caller can tell a refusal from any other failure.
func TestIsAdmissionErrorSeesThroughWrapping(t *testing.T) {
	refusal := &AdmissionError{
		Purpose: PurposePerception,
		Decision: Decision{
			Code:   DecisionWindowExceeded,
			Reason: "counted 210000 tokens against a 200000 window",
			Count:  Count{Tokens: 210000, Confidence: ConfidenceExact},
			Window: 200000,
		},
	}

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"bare", refusal},
		{"wrapped once", fmt.Errorf("observation failed: %w", refusal)},
		{"wrapped twice", fmt.Errorf("turn failed: %w", fmt.Errorf("observation failed: %w", refusal))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := IsAdmissionError(tc.err)
			if !ok {
				t.Fatalf("IsAdmissionError(%v) = false; a caller cannot tell this from any other failure", tc.err)
			}
			if got.Decision.Code != DecisionWindowExceeded {
				t.Errorf("code = %q, want %q", got.Decision.Code, DecisionWindowExceeded)
			}
			if got.Purpose != PurposePerception {
				t.Errorf("purpose = %q, want %q", got.Purpose, PurposePerception)
			}
		})
	}
}

func TestIsAdmissionErrorRejectsOtherFailures(t *testing.T) {
	for _, err := range []error{
		nil,
		errors.New("connection reset"),
		fmt.Errorf("observation failed: %w", errors.New("timeout")),
	} {
		if _, ok := IsAdmissionError(err); ok {
			t.Errorf("IsAdmissionError(%v) = true; an ordinary failure must not be reported as a refusal", err)
		}
	}
}
