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
