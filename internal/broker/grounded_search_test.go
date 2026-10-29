package broker

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// fakeGroundedSearcher is a client that can ground, or can say it cannot.
type fakeGroundedSearcher struct {
	*fakeClient
	supports bool
	queries  []string
}

func (f *fakeGroundedSearcher) SupportsGroundedWebSearch() bool { return f.supports }

func (f *fakeGroundedSearcher) GroundedWebSearch(_ context.Context, query string) (*types.GroundedWebSearchResult, error) {
	f.queries = append(f.queries, query)
	return &types.GroundedWebSearchResult{
		Text:  "grounded answer",
		Usage: types.GroundedUsage{InputTokens: 30, OutputTokens: 12, TotalTokens: 42},
	}, nil
}

// The metering wrapper must not hide a client's grounded search: init's
// domain research and the research tools probe for it, and a hidden capability
// read as "this client cannot ground" for a Meta client that can.
func TestWrapForwardsGroundedWebSearchAndMetersIt(t *testing.T) {
	ledger := NewLedger(LedgerConfig{Window: 100000})
	inner := &fakeGroundedSearcher{fakeClient: newFakeClient(), supports: true}
	client, err := Wrap(inner, Config{Provider: "test", Model: "m", Counter: fixedCounter{tokens: 10, confidence: ConfidenceExact}, Ledger: ledger})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	searcher, ok := client.(types.GroundedWebSearcher)
	if !ok || !searcher.SupportsGroundedWebSearch() {
		t.Fatal("the wrapped client hides the grounded search its client supports")
	}
	result, err := searcher.GroundedWebSearch(context.Background(), "what changed in mangle 0.7")
	if err != nil || result == nil || result.Text != "grounded answer" {
		t.Fatalf("GroundedWebSearch = %+v, %v", result, err)
	}
	if len(inner.queries) != 1 || inner.queries[0] != "what changed in mangle 0.7" {
		t.Fatalf("the query did not reach the client as sent: %v", inner.queries)
	}
	if got := ledger.Total(); got.Total() == 0 {
		t.Fatal("a grounded search spent nothing on the ledger; it must be metered like any call")
	}
}

func TestWrapReportsNoGroundedWebSearchForAClientWithout(t *testing.T) {
	for name, inner := range map[string]types.LLMClient{
		"no method":      newFakeClient(),
		"says it cannot": &fakeGroundedSearcher{fakeClient: newFakeClient(), supports: false},
	} {
		client, err := Wrap(inner, Config{Provider: "test", Model: "m", Counter: fixedCounter{tokens: 10, confidence: ConfidenceExact}})
		if err != nil {
			t.Fatalf("%s: Wrap: %v", name, err)
		}
		searcher, ok := client.(types.GroundedWebSearcher)
		if ok && searcher.SupportsGroundedWebSearch() {
			t.Errorf("%s: the wrapper claims grounded search the client cannot do", name)
		}
		if ok {
			if _, err := searcher.GroundedWebSearch(context.Background(), "q"); err == nil {
				t.Errorf("%s: a search through a client that cannot ground succeeded", name)
			}
		}
	}
}
