package usage

import (
	"context"
	"math"
	"testing"
)

func approx(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestLookupPrice_ShouldPreferLongestPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model     string
		wantIn    float64
		wantFound bool
	}{
		{"claude-opus-4-5-20260101", 15.00, true},
		{"claude-sonnet-4-20260514", 3.00, true},
		{"claude-3-haiku-20240307", 0.25, true},
		{"gpt-4o-mini-2024-07-18", 0.15, true},
		{"gpt-4o-2024-11-20", 2.50, true},
		{"gpt-4-turbo-preview", 10.00, true},
		{"gemini-2.5-flash-latest", 0.30, true},
		{"deepseek-reasoner", 0.55, true},
		{"totally-unknown-model", 0, false},
		{"", 0, false},
	}

	for _, tc := range tests {
		p, ok := LookupPrice(tc.model)
		if ok != tc.wantFound {
			t.Errorf("LookupPrice(%q) found = %v, want %v", tc.model, ok, tc.wantFound)
			continue
		}
		if ok {
			approx(t, p.InputPerMTok, tc.wantIn, "LookupPrice("+tc.model+").InputPerMTok")
		}
	}
}

// TestLookupPrice_ShouldNormalizeProviderRoutes covers the several ways engines
// spell the same model.
func TestLookupPrice_ShouldNormalizeProviderRoutes(t *testing.T) {
	t.Parallel()
	for _, model := range []string{
		"openai/gpt-4o",
		"OpenAI/GPT-4o",
		"anthropic.claude-sonnet-4-v1:0",
		"vertex_ai/gemini-2.5-pro",
		"  gpt-4o  ",
	} {
		if _, ok := LookupPrice(model); !ok {
			t.Errorf("LookupPrice(%q) did not resolve", model)
		}
	}
}

func TestPrice_Cost_ShouldScalePerMillionTokens(t *testing.T) {
	t.Parallel()
	p := Price{InputPerMTok: 3.00, OutputPerMTok: 15.00}

	approx(t, p.Cost(1_000_000, 0), 3.00, "1M input")
	approx(t, p.Cost(0, 1_000_000), 15.00, "1M output")
	approx(t, p.Cost(500_000, 100_000), 1.50+1.50, "mixed")
	approx(t, p.Cost(0, 0), 0, "zero")
}

func TestEstimateCost_ShouldReportUnknownModels(t *testing.T) {
	t.Parallel()
	if _, ok := EstimateCost("no-such-model", 1000, 1000); ok {
		t.Error("EstimateCost reported an unknown model as priced")
	}
	cost, ok := EstimateCost("gpt-4o", 1_000_000, 1_000_000)
	if !ok {
		t.Fatal("gpt-4o should be priced")
	}
	approx(t, cost, 12.50, "gpt-4o 1M/1M")
}

func TestRegisterPrice_ShouldOverrideListPrice(t *testing.T) {
	// Not parallel: mutates the shared price table.
	const model = "test-negotiated-model"
	RegisterPrice(model, Price{InputPerMTok: 1.00, OutputPerMTok: 2.00})

	cost, ok := EstimateCost(model+"-v9", 1_000_000, 1_000_000)
	if !ok {
		t.Fatal("registered model should be priced")
	}
	approx(t, cost, 3.00, "negotiated cost")
}

// TestTrack_ShouldAccumulateCost checks cost lands on every aggregate dimension.
func TestTrack_ShouldAccumulateCost(t *testing.T) {
	tr, _ := newTestTracker(t)
	ctx := WithShardContext(context.Background(), "coder-1", "specialist", "sess-1")

	// 1M input @ $2.50 + 1M output @ $10.00 = $12.50
	tr.Track(ctx, "gpt-4o", "openai", 1_000_000, 1_000_000, "chat")
	stats := tr.Stats()

	approx(t, stats.TotalProject.Cost, 12.50, "TotalProject.Cost")
	approx(t, stats.ByModel["gpt-4o"].Cost, 12.50, "ByModel cost")
	approx(t, stats.ByProvider["openai"].Cost, 12.50, "ByProvider cost")
	approx(t, stats.ByShardName["coder-1"].Cost, 12.50, "ByShardName cost")
	approx(t, stats.BySession["sess-1"].Cost, 12.50, "BySession cost")
	if stats.UnpricedTokens != 0 {
		t.Errorf("UnpricedTokens = %d, want 0", stats.UnpricedTokens)
	}
}

// TestTrack_ShouldCountUnpricedTokens is why a $0.00 total is not ambiguous:
// unpriced spend is reported separately rather than silently reading as free.
func TestTrack_ShouldCountUnpricedTokens(t *testing.T) {
	tr, _ := newTestTracker(t)
	ctx := WithShardContext(context.Background(), "c", "coder", "s")

	tr.Track(ctx, "some-local-llama", "ollama", 400, 100, "chat")
	stats := tr.Stats()

	if stats.TotalProject.Cost != 0 {
		t.Errorf("Cost = %v, want 0 for an unpriced model", stats.TotalProject.Cost)
	}
	if stats.UnpricedTokens != 500 {
		t.Errorf("UnpricedTokens = %d, want 500", stats.UnpricedTokens)
	}
}

func TestNormalizeModelName(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"OpenAI/GPT-4o":                  "gpt-4o",
		"anthropic.claude-sonnet-4-v1:0": "claude-sonnet-4-v1",
		"  Gemini-2.5-Pro  ":             "gemini-2.5-pro",
		"":                               "",
	}
	for in, want := range tests {
		if got := normalizeModelName(in); got != want {
			t.Errorf("normalizeModelName(%q) = %q, want %q", in, got, want)
		}
	}
}
