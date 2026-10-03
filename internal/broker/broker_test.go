package broker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestBrokerReserveAdmissionContract(test *testing.T) {
	maximumInt := int(^uint(0) >> 1)
	minimumInt := -maximumInt - 1
	cases := []struct {
		name     string
		window   int
		reserve  int
		tokens   int
		budget   int64
		headroom int
		code     DecisionCode
	}{
		{name: "reserve equals window", window: 1000, reserve: 1000, tokens: 1, headroom: -1, code: DecisionWindowExceeded},
		{name: "reserve exceeds window", window: 1000, reserve: 1001, tokens: 1, headroom: -1, code: DecisionWindowExceeded},
		{name: "maximum reserve", window: 1, reserve: maximumInt, tokens: 1, headroom: -1, code: DecisionWindowExceeded},
		{name: "below capacity", window: 1000, reserve: 200, tokens: 799, headroom: 1, code: DecisionAdmitted},
		{name: "exact capacity", window: 1000, reserve: 200, tokens: 800, code: DecisionAdmitted},
		{name: "one over capacity", window: 1000, reserve: 200, tokens: 801, headroom: -1, code: DecisionWindowExceeded},
		{name: "negative constructor reserve", window: 1000, reserve: -1, tokens: 1000, code: DecisionAdmitted},
		{name: "minimum constructor reserve", window: 1000, reserve: minimumInt, tokens: 1000, code: DecisionAdmitted},
		{name: "minimum constructor reserve one over", window: 1000, reserve: minimumInt, tokens: 1001, headroom: -1, code: DecisionWindowExceeded},
		{name: "unknown window", reserve: maximumInt, tokens: 10000, code: DecisionAdmitted},
		{name: "minimum unknown window", window: minimumInt, reserve: maximumInt, tokens: 1, code: DecisionAdmitted},
		{name: "unknown window purpose fits", reserve: maximumInt, tokens: 100, budget: 200, code: DecisionAdmitted},
		{name: "unknown window purpose exhausted", reserve: maximumInt, tokens: 201, budget: 200, code: DecisionBudgetExhausted},
		{name: "negative window purpose exhausted", window: -1, reserve: maximumInt, tokens: 201, budget: 200, code: DecisionBudgetExhausted},
		{name: "zero count with unknown window", budget: 200, code: DecisionCountUnavailable},
		{name: "negative count precedes exhausted window", window: 1000, reserve: 1000, tokens: -1, code: DecisionCountUnavailable},
	}
	methods := []string{
		"Complete", "CompleteWithSystem", "CompleteWithTools", "CompleteWithToolResults",
		"CompleteWithSchema", "CompleteWithStreaming", "CompleteWithStreamingAndThoughts",
	}
	seeds := []struct {
		name  string
		spend Spend
	}{
		{name: "fresh"},
		{name: "seeded", spend: Spend{InputTokens: 10, OutputTokens: 5, Calls: 1}},
	}
	for _, testcase := range cases {
		for _, method := range methods {
			for _, seed := range seeds {
				test.Run(testcase.name+"/"+method+"/"+seed.name, func(test *testing.T) {
					config := MeterConfig{Window: testcase.window, OutputReserve: testcase.reserve, ReceiptBuffer: 8}
					if testcase.budget > 0 {
						config.Budgets = map[Purpose]int64{PurposeSession: testcase.budget}
					}
					meter := NewMeter(config)
					ledger := meter.Ledger()
					ledger.Record(PurposeSession, seed.spend)
					spendBefore := ledger.Total()
					sink := &captureSink{}
					backend := newFakeAll()
					backend.streamChunks = []string{backend.respText}
					wrapperConfig := meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
					wrapperConfig.Counter = fixedCounter{tokens: testcase.tokens, confidence: ConfidenceExact}
					wrapperConfig.Sink = sink
					client, err := Wrap(backend, wrapperConfig)
					if err != nil {
						test.Fatalf("Wrap: %v", err)
					}
					callerContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					callerContext = WithPurpose(callerContext, PurposeSession)
					output, callErr := invokeBrokerReserveMethod(test, callerContext, client, method)
					allowed := testcase.code == DecisionAdmitted
					if allowed {
						if callErr != nil || output != backend.respText {
							test.Fatalf("admitted call output=%q error=%v, want backend output %q", output, callErr, backend.respText)
						}
						if calls := backend.callNames(); len(calls) != 1 || calls[0] != method {
							test.Fatalf("backend calls=%v, want exactly %s", calls, method)
						}
						waitFor(test, func() bool { return len(sink.all()) == 1 })
					} else {
						admissionError, recognized := IsAdmissionError(callErr)
						if !recognized || admissionError.Purpose != PurposeSession || admissionError.Decision.Code != testcase.code {
							test.Fatalf("call error=%v, want session admission refusal %s", callErr, testcase.code)
						}
						if output != "" || len(backend.callNames()) != 0 {
							test.Fatalf("refused call reached backend: output=%q calls=%v", output, backend.callNames())
						}
						if receipts := sink.all(); len(receipts) != 1 || receipts[0].Decision != admissionError.Decision {
							test.Fatalf("refusal receipt does not match returned decision: %+v", receipts)
						}
					}
					receipts := sink.all()
					if len(receipts) != 1 {
						test.Fatalf("receipt count=%d, want exactly one", len(receipts))
					}
					receipt := receipts[0]
					if receipt.Method != method || receipt.Provider != "fake" || receipt.Model != "fake-model" || receipt.Purpose != PurposeSession {
						test.Errorf("receipt lost call identity: %+v", receipt)
					}
					if receipt.Decision.Code != testcase.code || receipt.Decision.Allowed != allowed || receipt.Decision.Window != testcase.window || receipt.Decision.Headroom != testcase.headroom {
						test.Errorf("receipt decision=%+v, want %s with window %d and headroom %d", receipt.Decision, testcase.code, testcase.window, testcase.headroom)
					}
					if receipt.Estimated.Tokens != testcase.tokens || receipt.Estimated.Confidence != ConfidenceExact || receipt.Estimated.Source != "fixed" || receipt.Estimated.Model != "fake-model" || receipt.Decision.Count != receipt.Estimated {
						test.Errorf("receipt lost the outbound count: %+v", receipt)
					}
					if receipt.Started.IsZero() || receipt.Duration < 0 {
						test.Errorf("receipt has invalid timing: %+v", receipt)
					}
					if !allowed && receipt.Decision.Reason == "" {
						test.Error("refusal receipt has no reason")
					}
					expectedActual := Spend{}
					if allowed {
						expectedActual = Spend{InputTokens: 100, OutputTokens: 50, Calls: 1}
					}
					expectedTotal := spendBefore
					expectedTotal.Add(expectedActual)
					if receipt.Actual != expectedActual || ledger.Total() != expectedTotal || ledger.Account(PurposeSession) != expectedTotal {
						test.Fatalf("settlement changed spend incorrectly: actual=%+v total=%+v account=%+v, want actual=%+v total=%+v", receipt.Actual, ledger.Total(), ledger.Account(PurposeSession), expectedActual, expectedTotal)
					}
					if ledger.Account(PurposeUnattributed) != (Spend{}) {
						test.Fatal("tagged call recorded unattributed spend")
					}
				})
			}
		}
	}
}

func invokeBrokerReserveMethod(test *testing.T, ctx context.Context, client types.LLMClient, method string) (string, error) {
	test.Helper()
	switch method {
	case "Complete":
		return client.Complete(ctx, "user")
	case "CompleteWithSystem":
		return client.CompleteWithSystem(ctx, "system", "user")
	case "CompleteWithTools":
		response, err := client.CompleteWithTools(ctx, "system", "user", nil)
		if response == nil {
			return "", err
		}
		return response.Text, err
	case "CompleteWithToolResults":
		provider, supported := client.(types.ToolResultsProvider)
		if !supported {
			test.Fatal("wrapper dropped tool-results support")
		}
		response, err := provider.CompleteWithToolResults(ctx, "system", nil, nil)
		if response == nil {
			return "", err
		}
		return response.Text, err
	case "CompleteWithSchema":
		provider, supported := client.(interface {
			CompleteWithSchema(context.Context, string, string, string) (string, error)
		})
		if !supported {
			test.Fatal("wrapper dropped schema support")
		}
		return provider.CompleteWithSchema(ctx, "system", "user", `{"type":"object"}`)
	case "CompleteWithStreaming":
		content, failures := client.CompleteWithStreaming(ctx, "system", "user", false)
		output, _, err := drainBrokerReserveStreams(test, ctx, content, nil, failures)
		return output, err
	case "CompleteWithStreamingAndThoughts":
		provider, supported := client.(interface {
			CompleteWithStreamingAndThoughts(context.Context, string, string, bool) (<-chan string, <-chan string, <-chan error)
		})
		if !supported {
			test.Fatal("wrapper dropped thought-stream support")
		}
		content, thoughts, failures := provider.CompleteWithStreamingAndThoughts(ctx, "system", "user", true)
		output, thoughtOutput, err := drainBrokerReserveStreams(test, ctx, content, thoughts, failures)
		if (err == nil && thoughtOutput != "thinking") || (err != nil && thoughtOutput != "") {
			test.Fatalf("thought stream output=%q error=%v", thoughtOutput, err)
		}
		return output, err
	default:
		test.Fatalf("unknown broker method %q", method)
		return "", nil
	}
}

func drainBrokerReserveStreams(test *testing.T, ctx context.Context, content, thoughts <-chan string, failures <-chan error) (string, string, error) {
	test.Helper()
	if content == nil || failures == nil {
		test.Fatal("broker returned a nil content or error channel")
	}
	var output strings.Builder
	var thoughtOutput strings.Builder
	var callErr error
	for content != nil || thoughts != nil || failures != nil {
		select {
		case chunk, open := <-content:
			if !open {
				content = nil
			} else {
				output.WriteString(chunk)
			}
		case chunk, open := <-thoughts:
			if !open {
				thoughts = nil
			} else {
				thoughtOutput.WriteString(chunk)
			}
		case failure, open := <-failures:
			if !open {
				failures = nil
			} else {
				if failure == nil || callErr != nil {
					test.Fatalf("invalid or duplicate stream failure: %v", failure)
				}
				callErr = failure
			}
		case <-ctx.Done():
			test.Fatalf("broker stream did not terminate: %v", ctx.Err())
		}
	}
	return output.String(), thoughtOutput.String(), callErr
}

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
	// Duration >= 0, not > 0. The Windows monotonic clock has ~0.5ms
	// granularity, so a mock provider that returns immediately genuinely
	// measures as zero — that is the clock reporting a call faster than it can
	// resolve, not a missing measurement. Demanding a positive number here
	// asserts that the test's fake is slow, which is not a property anyone
	// wants to keep true.
	if r.Duration < 0 {
		t.Errorf("receipt carries a negative duration: %v", r.Duration)
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

// A phase rides the context like a purpose and lands on the receipt, so the
// meter can split a repair round from the ordinary rounds of the same purpose.
func TestReceipt_WhenTheContextCarriesAPhase_ShouldRecordIt(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	client := meteredClient(t, newFakeClient(), meter, sink)

	ctx := WithPhase(WithPurpose(context.Background(), PurposeSession), PhaseRepair)
	if _, err := client.CompleteWithSystem(ctx, "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	r, ok := sink.last()
	if !ok {
		t.Fatal("no receipt emitted")
	}
	if r.Purpose != PurposeSession || r.Phase != PhaseRepair {
		t.Errorf("receipt = (%q, %q), want (session, repair)", r.Purpose, r.Phase)
	}
	if _, err := client.CompleteWithSystem(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if r, _ := sink.last(); r.Phase != "" {
		t.Errorf("an untagged call recorded phase %q, want none", r.Phase)
	}
}
