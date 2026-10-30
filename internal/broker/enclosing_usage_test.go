package broker

import (
	"context"
	"testing"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

type enclosingBudget struct{ reports, unknown int }

func (b *enclosingBudget) Observed(usage.Report) { b.reports++ }
func (b *enclosingBudget) UsageUnknown()         { b.unknown++ }

func TestBrokerKeepsMissingCallUnknownInsideOtherwiseMeteredPhase(t *testing.T) {
	budget := &enclosingBudget{}
	ctx := usage.WithObserver(context.Background(), budget)
	fake := newFakeClient()
	fake.reportUsage = &types.UsageMetadata{InputTokens: 10, OutputTokens: 5, Known: types.UsageTokens}
	client := meteredClient(t, fake, testMeter(200000, 8000), nil)
	if _, err := client.Complete(ctx, "metered"); err != nil {
		t.Fatal(err)
	}
	fake.reportTimes = 0
	if _, err := client.Complete(ctx, "unmetered"); err != nil {
		t.Fatal(err)
	}
	if budget.reports != 1 || budget.unknown != 1 {
		t.Fatalf("outer reports=%d unknown=%d; missing usage must survive per-call capture", budget.reports, budget.unknown)
	}
}
