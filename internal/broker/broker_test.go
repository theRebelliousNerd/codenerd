package broker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func meteredClient(t *testing.T, underlying types.LLMClient, meter *Meter, sink ReceiptSink) types.LLMClient {
	t.Helper()
	cfg := meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	cfg.Sink = sink
	client, err := Wrap(underlying, cfg)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	return client
}

func TestReceiptRecordsProviderActuals(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	fake := newFakeClient()
	fake.reportInput, fake.reportOutput = 1234, 567

	client := meteredClient(t, fake, meter, sink)
	if _, err := client.CompleteWithSystem(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}

	r, ok := sink.last()
	if !ok {
		t.Fatal("no receipt emitted")
	}
	if r.Actual.InputTokens != 1234 || r.Actual.OutputTokens != 567 {
		t.Errorf("receipt did not carry the provider's reported usage: %+v", r.Actual)
	}
	if r.Actual.Calls != 1 {
		t.Errorf("calls = %d, want 1", r.Actual.Calls)
	}
	if r.Estimated.Tokens <= 0 {
		t.Error("receipt carries no pre-flight estimate")
	}
	if r.Duration <= 0 {
		t.Error("receipt carries no duration; latency is unmeasurable without it")
	}
	if r.Method != "CompleteWithSystem" {
		t.Errorf("method = %q", r.Method)
	}
}

func TestSpendIsRecordedAgainstTheContextPurpose(t *testing.T) {
	meter := testMeter(200000, 8000)
	fake := newFakeClient()
	fake.reportInput, fake.reportOutput = 300, 100

	client := meteredClient(t, fake, meter, nil)
	ctx := WithPurpose(context.Background(), PurposeCompression)
	if _, err := client.Complete(ctx, "prompt"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	acct := meter.Ledger().Account(PurposeCompression)
	if acct.InputTokens != 300 || acct.OutputTokens != 100 {
		t.Errorf("compression account = %+v, want in=300 out=100", acct)
	}
	if meter.Ledger().Account(PurposeSession).Calls != 0 {
		t.Error("spend leaked into an unrelated purpose account")
	}
}

func TestUntaggedSpendLandsInUnattributedRatherThanVanishing(t *testing.T) {
	// Unattributed spend is a wiring bug. It must be a visible number, not a
	// silent discard, or the bug is undiscoverable.
	meter := testMeter(200000, 8000)
	client := meteredClient(t, newFakeClient(), meter, nil)

	if _, err := client.Complete(context.Background(), "prompt"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if meter.Ledger().Account(PurposeUnattributed).Calls != 1 {
		t.Errorf("untagged call did not land in the unattributed account: %+v",
			meter.Ledger().Account(PurposeUnattributed))
	}
}

func TestNoDoubleCounting(t *testing.T) {
	// The observer and the response's usage block both describe the same call.
	// Adding them would double every tool-call turn in the system.
	meter := testMeter(200000, 8000)
	fake := newFakeClient()
	fake.reportInput, fake.reportOutput = 500, 200
	fake.usageOnResponse = &types.UsageMetadata{InputTokens: 500, OutputTokens: 200}

	client := meteredClient(t, fake, meter, nil)
	if _, err := client.CompleteWithTools(context.Background(), "sys", "user", nil); err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}

	total := meter.Ledger().Total()
	if total.InputTokens != 500 || total.OutputTokens != 200 {
		t.Errorf("usage counted twice: %+v, want in=500 out=200", total)
	}
	if total.Calls != 1 {
		t.Errorf("calls = %d, want 1", total.Calls)
	}
}

func TestRepeatedProviderReportsForOneCallAccumulate(t *testing.T) {
	// A client that retries internally, or streams input and output usage in
	// separate events, produces several reports for one logical call. Every one
	// of them was billed, so they add rather than overwrite.
	meter := testMeter(200000, 8000)
	fake := newFakeClient()
	fake.reportInput, fake.reportOutput, fake.reportTimes = 100, 40, 3

	client := meteredClient(t, fake, meter, nil)
	if _, err := client.Complete(context.Background(), "prompt"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	total := meter.Ledger().Total()
	if total.InputTokens != 300 || total.OutputTokens != 120 {
		t.Errorf("repeated reports did not accumulate: %+v", total)
	}
	if total.Calls != 1 {
		t.Errorf("three reports for one call recorded as %d calls", total.Calls)
	}
}

func TestFallsBackToResponseUsageWhenProviderDoesNotReport(t *testing.T) {
	meter := testMeter(200000, 8000)
	fake := newFakeClient()
	fake.reportTimes = 0 // reports nothing through the usage plumbing
	fake.usageOnResponse = &types.UsageMetadata{InputTokens: 42, OutputTokens: 7}

	client := meteredClient(t, fake, meter, nil)
	if _, err := client.CompleteWithTools(context.Background(), "sys", "user", nil); err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}

	total := meter.Ledger().Total()
	if total.InputTokens != 42 || total.OutputTokens != 7 {
		t.Errorf("response-carried usage was not used as the fallback: %+v", total)
	}
}

func TestRefusedRequestNeverReachesTheProvider(t *testing.T) {
	meter := NewMeter(MeterConfig{Window: 1000, OutputReserve: 900, ReceiptBuffer: 8})
	sink := &captureSink{}
	fake := newFakeClient()

	cfg := meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	cfg.Counter = fixedCounter{tokens: 5000, confidence: ConfidenceExact}
	cfg.Sink = sink
	client, err := Wrap(fake, cfg)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	_, callErr := client.CompleteWithSystem(context.Background(), "sys", "user")
	if callErr == nil {
		t.Fatal("an over-window request was not refused")
	}

	ae, ok := IsAdmissionError(callErr)
	if !ok {
		t.Fatalf("error is not an AdmissionError: %T %v", callErr, callErr)
	}
	if ae.Decision.Code != DecisionWindowExceeded {
		t.Errorf("decision code = %q", ae.Decision.Code)
	}
	if !strings.Contains(ae.Error(), "window") {
		t.Errorf("refusal message should name the constraint: %q", ae.Error())
	}

	if names := fake.callNames(); len(names) != 0 {
		t.Errorf("a refused request still reached the provider: %v", names)
	}
	if meter.Ledger().Total().Calls != 0 {
		t.Error("a refused request was recorded as spend; refusing costs no tokens")
	}

	r, ok := sink.last()
	if !ok {
		t.Fatal("a refusal must still emit a receipt")
	}
	if r.Decision.Allowed {
		t.Error("refusal receipt claims the call was allowed")
	}
}

func TestFailsClosedWhenCounterErrors(t *testing.T) {
	meter := testMeter(200000, 8000)
	fake := newFakeClient()

	cfg := meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	cfg.Counter = failingCounter{err: errors.New("counting backend down")}
	client, err := Wrap(fake, cfg)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	if _, callErr := client.Complete(context.Background(), "prompt"); callErr == nil {
		t.Fatal("a request whose size could not be determined was admitted")
	}
	if names := fake.callNames(); len(names) != 0 {
		t.Errorf("an uncountable request reached the provider: %v", names)
	}
}

func TestCalibrationFeedsBackFromRealResponses(t *testing.T) {
	// End-to-end proof of the loop that makes the estimator different in kind
	// from the constant it replaced.
	meter := testMeter(1000000, 8000)
	fake := newFakeClient()
	fake.reportInput, fake.reportOutput = 2000, 10

	client := meteredClient(t, fake, meter, nil)
	before := meter.Calibrator().Observations("fake-model")

	long := strings.Repeat("some representative prompt content ", 300)
	if _, err := client.CompleteWithSystem(context.Background(), "system", long); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}

	after := meter.Calibrator().Observations("fake-model")
	if after != before+1 {
		t.Errorf("a completed response did not train the calibrator: %d -> %d", before, after)
	}
	if _, conf := meter.Calibrator().Ratio("fake-model"); conf != ConfidenceCalibrated {
		t.Errorf("confidence after one real response = %q, want calibrated", conf)
	}
}

func TestEstimateErrorIsReportedOnTheReceipt(t *testing.T) {
	meter := testMeter(1000000, 8000)
	sink := &captureSink{}
	fake := newFakeClient()
	fake.reportInput, fake.reportOutput = 1000, 10

	cfg := meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	cfg.Counter = fixedCounter{tokens: 1200, confidence: ConfidenceSeeded}
	cfg.Sink = sink
	client, err := Wrap(fake, cfg)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	if _, err := client.Complete(context.Background(), "prompt"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	r, _ := sink.last()
	// (1200 - 1000) / 1000 = +20%
	if r.EstimateErrorPct < 19.9 || r.EstimateErrorPct > 20.1 {
		t.Errorf("estimate error = %.2f%%, want ~20%%. This is the number that says whether the meter can be trusted.",
			r.EstimateErrorPct)
	}
}

func TestUnderlyingErrorsPropagateAndStillSettle(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	fake := newFakeClient()
	fake.respErr = errors.New("provider exploded")
	fake.reportInput, fake.reportOutput = 80, 0 // input was still billed

	client := meteredClient(t, fake, meter, sink)
	if _, err := client.Complete(context.Background(), "prompt"); err == nil {
		t.Fatal("underlying error was swallowed")
	}

	r, ok := sink.last()
	if !ok {
		t.Fatal("a failed call must still emit a receipt")
	}
	if r.Err == "" {
		t.Error("receipt does not record the failure")
	}
	if meter.Ledger().Total().InputTokens != 80 {
		t.Error("a failed call still costs its input tokens and must be recorded")
	}
}
