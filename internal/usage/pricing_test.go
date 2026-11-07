package usage

import (
	"codenerd/internal/types"
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
	tr.Record(ctx, Report{Model: "gpt-4o", Provider: "openai", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 1_000_000, OutputTokens: 1_000_000}})
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

	tr.Record(ctx, Report{Model: "some-local-llama", Provider: "ollama", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 400, OutputTokens: 100}})
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

// The planner tier names its model with a provider route prefix; the opus
// 5-5 rate must resolve through it, and a bare CLI alias must NOT be priced
// as a different generation (that alias fabricated opus-4 costs for opus
// 5-5 runs — the CLI's actual model id now comes from modelUsage).
func TestLookupPrice_OpusRoutesAndAlias(t *testing.T) {
	p, ok := LookupPrice("anthropic/claude-opus-5-5")
	if !ok || p.InputPerMTok != 4.00 || p.CacheReadPerMTok != 0.40 || p.CacheWritePerMTok != 5.00 {
		t.Fatalf("anthropic/claude-opus-5-5 price = %+v ok=%v, want 4.00 in / 0.40 cache-read / 5.00 cache-write", p, ok)
	}
	if _, ok := LookupPrice("opus"); ok {
		t.Error("bare \"opus\" resolved to a price; an unresolvable CLI alias must stay unpriced rather than borrow another generation's rate")
	}
}

func TestLookupPrice_GPT6FamilyResolvesToItsOwnRow(t *testing.T) {
	for model, wantIn := range map[string]float64{
		"gpt-6.1-sol":        2,
		"gpt-6-sol":          2,
		"gpt-6.1-terra":      1,
		"gpt-6-astra":        10,
		"gpt-6-luna":         0.10,
		"openai/gpt-6.1-sol": 2,
	} {
		p, ok := LookupPrice(model)
		if !ok || p.InputPerMTok != wantIn {
			t.Errorf("LookupPrice(%s) = %+v, %v; want input %v", model, p, ok, wantIn)
		}
	}
	sol, _ := LookupPrice("gpt-6.1-sol")
	// 1M uncached in + 1M cache read + 1M 30m cache write + 1M out.
	if got := sol.CostDetailed(3_000_000, 1_000_000, 1_000_000, 1_000_000); got != 2+0.10+2.5+10 {
		t.Errorf("gpt-6.1-sol detailed cost = %v", got)
	}
}

func TestCostDetailed_StabilizeClampsNegativeCache(t *testing.T) {
	p := Price{InputPerMTok: 10, OutputPerMTok: 20, CacheReadPerMTok: 1, CacheWritePerMTok: 2}
	got := p.CostDetailed(1000, 500, -5, -10)
	want := p.CostDetailed(1000, 500, 0, 0)
	if got != want {
		t.Errorf("negative cache not clamped: got %v want %v", got, want)
	}
}

func TestCostDetailed_StabilizeClampsExcessOverInput(t *testing.T) {
	p := Price{InputPerMTok: 10, OutputPerMTok: 20, CacheReadPerMTok: 1, CacheWritePerMTok: 2}
	if got, want := p.CostDetailed(100, 0, 80, 50), p.CostDetailed(100, 0, 80, 20); got != want {
		t.Errorf("excess cache not clamped to input: got %v want %v", got, want)
	}
	if got, want := p.CostDetailed(100, 0, 150, 10), p.CostDetailed(100, 0, 100, 0); got != want {
		t.Errorf("oversized cache-read not clamped to input: got %v want %v", got, want)
	}
}

func TestCostDetailed_StabilizeFallsBackToInputRate(t *testing.T) {
	p := Price{InputPerMTok: 4}
	approx(t, p.CostDetailed(1000000, 0, 1000000, 0), 4, "cache-read fallback")
	approx(t, p.CostDetailed(1000000, 0, 0, 1000000), 4, "cache-write fallback")
}

func TestLookupPrice_StabilizeEmptyUnknownExactAndColon(t *testing.T) {
	if _, _, ok := LookupPriceVersion(""); ok {
		t.Errorf("empty model should be unknown")
	}
	if _, _, ok := LookupPriceVersion("   "); ok {
		t.Errorf("blank model should be unknown")
	}
	if _, _, ok := LookupPriceVersion("no-such-model-xyz-stabilize"); ok {
		t.Errorf("unknown model should be unknown")
	}
	if _, _, ok := LookupPriceVersion("jev-1.14.0-extra"); ok {
		t.Errorf("exact entry must not prefix-match a longer id")
	}
	if _, _, ok := LookupPriceVersion("JEV-1.14.0"); !ok {
		t.Errorf("exact entry should match case-insensitively")
	}
	if _, _, ok := LookupPriceVersion("anthropic.claude-sonnet-4-v1:0"); !ok {
		t.Errorf("provider dot-prefix and colon suffix should normalize to a known row")
	}
	if cost, _, ok := EstimateCostVersioned("no-such-model-xyz-stabilize", 10, 10, 0, 0); ok || cost != 0 {
		t.Errorf("EstimateCostVersioned unknown = %v,%v want 0,false", cost, ok)
	}
}

func TestRegisterPrice_StabilizeEmptyPrefixNoOp(t *testing.T) {
	RegisterPrice("   ", Price{InputPerMTok: 999})
	if _, _, ok := LookupPriceVersion("   "); ok {
		t.Errorf("blank prefix registration should stay unknown")
	}
	if _, _, ok := LookupPriceVersion("gpt-4o"); !ok {
		t.Errorf("empty registration must not clear the table")
	}
	RegisterPrice("openai/stabilize-probe-xyz", Price{InputPerMTok: 7})
	if _, _, ok := LookupPriceVersion("stabilize-probe-xyz"); !ok {
		t.Errorf("registered prefix should resolve after route normalization")
	}
	if _, _, ok := LookupPriceVersion("openai/stabilize-probe-xyz"); !ok {
		t.Errorf("registered prefix should resolve through the same route")
	}
}

func TestObserverAndCredential_StabilizeNilSafety(t *testing.T) {
	if got := ObserverFromContext(nil); got != nil {
		t.Errorf("ObserverFromContext(nil) = %v want nil", got)
	}
	if got := ObserverFromContext(context.Background()); got != nil {
		t.Errorf("ObserverFromContext(background) = %v want nil", got)
	}
	base := context.Background()
	if got := WithObserver(base, nil); got != base {
		t.Errorf("WithObserver(nil obs) should return ctx unchanged")
	}
	if WithObserver(nil, nil) != nil {
		t.Errorf("WithObserver(nil,nil) should be nil")
	}
	NoteUnknownUsage(nil)
	NoteUnknownUsage(context.Background())
	RecordFromContext(nil, Report{Model: "gpt-4o", Provider: "openai"})
	if got := CredentialAliasFromContext(nil); got != "" {
		t.Errorf("CredentialAliasFromContext(nil) = %q want empty", got)
	}
	if got := CredentialAliasFromContext(context.Background()); got != "" {
		t.Errorf("CredentialAliasFromContext(background) = %q want empty", got)
	}
	aliasCtx := WithCredentialAlias(context.Background(), "alias-x")
	if got := CredentialAliasFromContext(aliasCtx); got != "alias-x" {
		t.Errorf("credential alias roundtrip = %q want alias-x", got)
	}
	if got := FromContext(nil); got != nil {
		t.Errorf("FromContext(nil) should be nil")
	}
	if got := SessionIDFromContext(nil); got != "" {
		t.Errorf("SessionIDFromContext(nil) = %q want empty", got)
	}
	if n, tn := ShardFromContext(nil); n != "" || tn != "" {
		t.Errorf("ShardFromContext(nil) = %q,%q want empty", n, tn)
	}
	if got := TurnIDFromContext(nil); got != "" {
		t.Errorf("TurnIDFromContext(nil) = %q want empty", got)
	}
	var nilTracker *Tracker
	if got := nilTracker.TurnTokens(""); got != (TokenCounts{}) {
		t.Errorf("nil TurnTokens should be zero")
	}
	if got := nilTracker.SessionTokens(""); got != (TokenCounts{}) {
		t.Errorf("nil SessionTokens should be zero")
	}
	m := map[string]TokenCounts{}
	addUsageToMap(m, "", types.UsageMetadata{InputTokens: 10, OutputTokens: 5}, 0)
	entry, ok := m["unknown"]
	if !ok {
		t.Fatalf("empty key should bucket as unknown")
	}
	if entry.Input != 10 || entry.Output != 5 || entry.Total != 15 {
		t.Errorf("unknown bucket = %+v want input 10 output 5 total 15", entry)
	}
	sessCtx := WithSessionID(nil, "sess-stabilize")
	if got := SessionIDFromContext(sessCtx); got != "sess-stabilize" {
		t.Errorf("WithSessionID(nil) roundtrip = %q", got)
	}
	turnCtx := WithTurnID(nil, "turn-stabilize")
	if got := TurnIDFromContext(turnCtx); got != "turn-stabilize" {
		t.Errorf("WithTurnID(nil) roundtrip = %q", got)
	}
}

func TestRecord_StabilizeRejectsEveryNegativeField(t *testing.T) {
	cases := map[string]types.UsageMetadata{
		"input":   {InputTokens: -1, OutputTokens: 10},
		"output":  {InputTokens: 10, OutputTokens: -1},
		"cached":  {InputTokens: 10, OutputTokens: 10, CachedContentTokens: -1},
		"written": {InputTokens: 10, OutputTokens: 10, CacheWriteTokens: -1},
		"reason":  {InputTokens: 10, OutputTokens: 10, ThinkingTokens: -1},
	}
	for name, meta := range cases {
		tr, _ := newTestTracker(t)
		ctx := context.Background()
		tr.Record(ctx, Report{Model: "gpt-4o", Provider: "openai", Operation: "chat", Usage: meta})
		stats := tr.Stats()
		if stats.TotalProject.Input != 0 || stats.TotalProject.Output != 0 || stats.TotalProject.Total != 0 {
			t.Errorf("%s: negative report was recorded: %+v", name, stats.TotalProject)
		}
		_ = tr.Close()
	}
	tr, _ := newTestTracker(t)
	tr.Record(context.Background(), Report{Model: "gpt-4o", Provider: "openai", Operation: "chat", Usage: types.UsageMetadata{}})
	if got := tr.Stats().TotalProject; got.Input != 0 || got.Output != 0 {
		t.Errorf("zero-token report should be ignored: %+v", got)
	}
	_ = tr.Close()
	if math.IsNaN(float64(tr.Stats().TotalProject.Input)) {
		t.Errorf("stats after close should be readable")
	}
}
