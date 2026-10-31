package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// F-SEC-1: qwen3.8-max sometimes spends a turn on reasoning_content and stops
// with empty content and finish_reason "stop". Observed live 2026-08-08 07:01:
// reasoning_chars 2604, output_tokens 543, content empty.
//
// It is not budget exhaustion -- the model stopped on its own -- so raising
// max_tokens does not address it. Asking again without thinking does.

func newCompatClientAt(t *testing.T, vendor Provider, baseURL string) *OpenAICompatClient {
	t.Helper()
	cfg := DefaultOpenAICompatConfig(vendor, "test-key")
	cfg.BaseURL = baseURL
	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient: %v", err)
	}
	return c
}

func TestCompleteWithSystem_RetriesWithoutThinkingOnReasoningOnlyReply(t *testing.T) {
	var calls int32
	var secondRequestThinking *bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)

		var body OpenAIRequest
		_ = json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			// The observed failure: reasoning present, content empty, stop.
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"thinking hard about it"},"finish_reason":"stop"}],"usage":{"completion_tokens":543}}`))
			return
		}
		secondRequestThinking = body.EnableThinking
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"the real answer"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderDashScope, srv.URL)
	meter := broker.NewMeter(broker.MeterConfig{})
	brokerConfig := meter.ConfigFor(broker.ProviderCreds{Provider: string(ProviderDashScope), Model: c.GetModel()})
	receipts := broker.NewRingSink(4)
	brokerConfig.Sink = receipts
	client, err := broker.Wrap(c, brokerConfig)
	if err != nil {
		t.Fatal(err)
	}
	ctx, tracker := trackedContext(t)
	outer, inner := &retryUsageObserver{}, &retryUsageObserver{}
	ctx = usage.WithObserver(usage.WithObserver(ctx, outer), inner)

	got, err := client.CompleteWithSystem(ctx, "sys", "user")
	if err != nil {
		t.Fatalf("CompleteWithSystem returned an error instead of recovering: %v", err)
	}
	if got != "the real answer" {
		t.Errorf("got %q; want the recovered answer", got)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("expected exactly 2 calls (original + one retry), got %d", n)
	}
	if secondRequestThinking == nil || *secondRequestThinking {
		t.Error("the retry did not disable thinking, so it repeats the condition that produced the empty reply")
	}
	for name, observer := range map[string]*retryUsageObserver{"outer": outer, "inner": inner} {
		if observer.reports != 1 || observer.output != 543 || observer.unknown == 0 {
			t.Errorf("%s reports=%d output=%d unknown=%d; preserve the first bill and mark the missing retry unknown", name, observer.reports, observer.output, observer.unknown)
		}
	}
	if got := tracker.Stats().TotalProject.Total; got != 543 {
		t.Errorf("tracked tokens=%d, want the known 543 without a synthetic bill", got)
	}
	if got := receipts.Receipts(); len(got) != 1 || got[0].UsageStatus != broker.UsagePartial || got[0].Actual.OutputTokens != 543 {
		t.Errorf("receipt must retain known spend as a lower bound: %+v", got)
	}
}

// This fixture is synchronous: both provider attempts settle before observers
// are read. Each capture is also a parent of the broker's per-call capture.
type retryUsageObserver struct {
	reports, output, unknown int
}

func (o *retryUsageObserver) Observed(r usage.Report) {
	o.reports++
	o.output += r.Usage.OutputTokens
}

func (o *retryUsageObserver) UsageUnknown() { o.unknown++ }

func TestReportUsageDistinguishesMissingMetadataFromKnownZero(t *testing.T) {
	for _, tc := range []struct {
		name             string
		metadata         types.UsageMetadata
		reports, unknown int
	}{
		{name: "missing", unknown: 1},
		{name: "known_zero", metadata: types.UsageMetadata{Known: types.UsageTokens}},
		{name: "known_counts", metadata: types.UsageMetadata{OutputTokens: 7, Known: types.UsageTokens}, reports: 1},
		{name: "counts_without_presence", metadata: types.UsageMetadata{OutputTokens: 7}, reports: 1, unknown: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observer := &retryUsageObserver{}
			ctx := usage.WithObserver(context.Background(), observer)
			reportUsage(ctx, "fixture", ProviderDashScope, usageOpChat, tc.metadata)
			if observer.reports != tc.reports || observer.unknown != tc.unknown || observer.output != tc.metadata.OutputTokens {
				t.Fatalf("reports=%d output=%d unknown=%d", observer.reports, observer.output, observer.unknown)
			}
		})
	}
}

// The retry must be bounded. A vendor that returns reasoning-only forever must
// produce one retry and then the original diagnostic, not a loop.
func TestCompleteWithSystem_EmptyRetryIsBounded(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","reasoning_content":"still thinking"},"finish_reason":"stop"}],"usage":{"completion_tokens":543}}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderDashScope, srv.URL)

	if _, err := c.CompleteWithSystem(context.Background(), "sys", "user"); err == nil {
		t.Fatal("expected an error when both attempts come back empty")
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("expected exactly 2 calls, got %d — the retry is not bounded", n)
	}
}

// A genuinely empty reply with NO reasoning must not trigger the retry: there is
// nothing to suggest thinking caused it, and a blind extra call doubles cost.
func TestCompleteWithSystem_NoRetryWhenThereIsNoReasoning(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderDashScope, srv.URL)

	if _, err := c.CompleteWithSystem(context.Background(), "sys", "user"); err == nil {
		t.Fatal("expected an error for an empty completion")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("expected exactly 1 call with no reasoning present, got %d", n)
	}
}
