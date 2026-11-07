package typesafe

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"codenerd/internal/usage"
)

type captureObserver struct {
	mu      sync.Mutex
	reports []usage.Report
}

func (o *captureObserver) Observed(r usage.Report) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reports = append(o.reports, r)
}

// An answered call is billed to the use that made it; a failed one bills
// nothing, since no answer came back.
func TestAsk_ReportsUsageUnderTheCallersOperation(t *testing.T) {
	c, _ := newTestClient(t, reply(http.StatusOK, answersJSON), nil)
	obs := &captureObserver{}
	ctx := WithOperation(usage.WithObserver(context.Background(), obs), "jev_turn_judgments")
	if _, err := c.Ask(ctx, map[string]any{"message": "hi"}, testQuestions()); err != nil {
		t.Fatal(err)
	}
	if len(obs.reports) != 1 {
		t.Fatalf("reports = %d, want 1", len(obs.reports))
	}
	r := obs.reports[0]
	if r.Provider != Provider || r.Model != "jev-1.14.0" || r.Operation != "jev_turn_judgments" || r.Usage.InputTokens != 120 || !r.Usage.Reported() {
		t.Fatalf("report = %+v", r)
	}

	failing, _ := newTestClient(t, reply(http.StatusBadRequest, `{"detail":"bad"}`), nil)
	obs2 := &captureObserver{}
	if _, err := failing.Ask(usage.WithObserver(context.Background(), obs2), map[string]any{"message": "hi"}, testQuestions()); err == nil {
		t.Fatal("a 400 answered")
	}
	if len(obs2.reports) != 0 {
		t.Fatalf("a failed call was billed: %+v", obs2.reports)
	}
}

func TestAsk_UnnamedOperationIsJev(t *testing.T) {
	c, _ := newTestClient(t, reply(http.StatusOK, answersJSON), nil)
	obs := &captureObserver{}
	if _, err := c.Ask(usage.WithObserver(context.Background(), obs), "state", testQuestions()); err != nil {
		t.Fatal(err)
	}
	if len(obs.reports) != 1 || obs.reports[0].Operation != defaultOperation {
		t.Fatalf("reports = %+v", obs.reports)
	}
}

// Attempt failures remain unknown even if a later retry returns usage. The
// successful report is delivered once; uncertainty is not an extra charge.
type incompleteCapture struct {
	captureObserver
	unknown int
}

func (o *incompleteCapture) UsageUnknown() { o.unknown++ }

func TestAsk_RetriedAnswerPreservesUnknownAttemptUsage(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			reply(http.StatusTooManyRequests, `{"detail":"retry"}`)(w, r)
			return
		}
		reply(http.StatusOK, answersJSON)(w, r)
	}, nil)
	obs := &incompleteCapture{}
	if _, err := c.Ask(usage.WithObserver(context.Background(), obs), "state", testQuestions()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(obs.reports) != 1 || obs.unknown != 1 {
		t.Fatalf("attempts=%d reports=%d unknown=%d", calls, len(obs.reports), obs.unknown)
	}
}

func TestWithOperation_BlankIsDefault(t *testing.T) {
	if got := operationFrom(WithOperation(context.Background(), "")); got != defaultOperation {
		t.Errorf("empty operation = %q, want %q", got, defaultOperation)
	}
	if got := operationFrom(WithOperation(context.Background(), "   ")); got != defaultOperation {
		t.Errorf("whitespace operation = %q, want %q", got, defaultOperation)
	}
	if got := operationFrom(WithOperation(context.Background(), "jev_turn_judgments")); got != "jev_turn_judgments" {
		t.Errorf("named operation = %q", got)
	}
	if got := operationFrom(context.WithValue(context.Background(), operationKey{}, 123)); got != defaultOperation {
		t.Errorf("wrong-type operation = %q, want %q", got, defaultOperation)
	}
}

func TestRecordUsage_FallsBackToConfiguredModel(t *testing.T) {
	c, _ := newTestClient(t, reply(http.StatusOK, answersJSON), nil)
	obs := &captureObserver{}
	ctx := usage.WithObserver(context.Background(), obs)
	c.recordUsage(ctx, &Response{Usage: Usage{InputTokens: 5, OutputTokens: 7}})
	if len(obs.reports) != 1 {
		t.Fatalf("reports = %d, want 1", len(obs.reports))
	}
	r := obs.reports[0]
	if r.Model != "jev-latest" {
		t.Errorf("model = %q, want configured fallback %q", r.Model, "jev-latest")
	}
	if r.Usage.InputTokens != 5 || r.Usage.OutputTokens != 7 || r.Usage.TotalTokens != 12 {
		t.Errorf("tokens = %+v, want 5/7/12", r.Usage)
	}
	if r.Provider != Provider || r.Operation != defaultOperation {
		t.Errorf("report = %+v", r)
	}
}

func TestExchangeUsage_FallsBackToConfiguredModel(t *testing.T) {
	c, _ := newTestClient(t, reply(http.StatusOK, answersJSON), nil)
	u := c.exchangeUsage(&Response{Usage: Usage{InputTokens: 5, OutputTokens: 7}})
	if u == nil || u.InputTokens != 5 || u.OutputTokens != 7 || u.Status != "reported" {
		t.Fatalf("usage = %+v", u)
	}
}
