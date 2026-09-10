package broker

import (
	"context"
	"math"
	"testing"
	"time"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// ---------------------------------------------------------------------------
// prefixFingerprint
// ---------------------------------------------------------------------------

func TestPrefixFingerprintEmptyWhenNothingCacheable(t *testing.T) {
	if got := prefixFingerprint(nil); got != "" {
		t.Fatalf("nil request: want empty fingerprint, got %q", got)
	}
	if got := prefixFingerprint(&Request{User: "hello"}); got != "" {
		t.Fatalf("no system and no tools: want empty fingerprint, got %q", got)
	}
}

func TestPrefixFingerprintIgnoresMessagesAndUser(t *testing.T) {
	base := &Request{System: "you are a helpful agent"}
	grown := &Request{
		System:   "you are a helpful agent",
		User:     "and now a totally different question",
		Messages: []types.Message{{Role: "user", Text: "turn one"}, {Role: "assistant", Text: "reply"}},
	}

	// Appending to the conversation extends the cached prefix rather than
	// breaking it. If this ever fails, every conversation reports as a run of
	// singleton epochs and Phase 4 gets killed by an artifact of the meter.
	if prefixFingerprint(base) != prefixFingerprint(grown) {
		t.Fatal("growing the message history changed the prefix fingerprint")
	}
}

func TestPrefixFingerprintDistinguishesFieldBoundaries(t *testing.T) {
	// The length-prefixing case: without it, moving a character across a field
	// boundary is invisible and two different prefixes collapse into one epoch.
	a := &Request{Tools: []types.ToolDefinition{{Name: "ab", Description: "c"}}}
	b := &Request{Tools: []types.ToolDefinition{{Name: "a", Description: "bc"}}}

	if prefixFingerprint(a) == prefixFingerprint(b) {
		t.Fatal("field boundary shift produced the same fingerprint")
	}
}

func TestPrefixFingerprintIsOrderSensitiveOverTools(t *testing.T) {
	one := types.ToolDefinition{Name: "read", Description: "read a file"}
	two := types.ToolDefinition{Name: "write", Description: "write a file"}

	forward := &Request{System: "sys", Tools: []types.ToolDefinition{one, two}}
	reverse := &Request{System: "sys", Tools: []types.ToolDefinition{two, one}}

	// Provider caches are prefixes of the token stream, so reordering tools
	// invalidates the entry even though the set is identical.
	if prefixFingerprint(forward) == prefixFingerprint(reverse) {
		t.Fatal("reordering tools left the fingerprint unchanged")
	}
}

func TestPrefixFingerprintIsStableAcrossSchemaMapIteration(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"alpha": map[string]any{"type": "string"},
			"beta":  map[string]any{"type": "number"},
			"gamma": map[string]any{"type": "boolean"},
			"delta": map[string]any{"type": "array"},
		},
	}
	req := &Request{System: "sys", Tools: []types.ToolDefinition{{Name: "t", InputSchema: schema}}}

	first := prefixFingerprint(req)
	// Go randomizes map iteration order, so a fingerprint that walked the map
	// directly would be unstable within a single process. Repeat enough times
	// that an unstable implementation cannot pass by luck.
	for i := 0; i < 200; i++ {
		if got := prefixFingerprint(req); got != first {
			t.Fatalf("fingerprint unstable across iterations: %q then %q", first, got)
		}
	}
}

func TestPrefixFingerprintSeparatesUnmarshallableSchemas(t *testing.T) {
	// Two different broken schemas must not hash alike: collapsing them would
	// splice distinct prefixes into one long epoch and overstate reuse.
	a := &Request{System: "s", Tools: []types.ToolDefinition{{Name: "t", InputSchema: map[string]any{"f": func() {}}}}}
	b := &Request{System: "s", Tools: []types.ToolDefinition{{Name: "t", InputSchema: map[string]any{"g": make(chan int)}}}}

	if prefixFingerprint(a) == prefixFingerprint(b) {
		t.Fatal("distinct unmarshallable schemas produced the same fingerprint")
	}
}

func TestPrefixFingerprintChangesWithSystemPrompt(t *testing.T) {
	a := &Request{System: "you are an agent"}
	b := &Request{System: "you are an agent."}
	if prefixFingerprint(a) == prefixFingerprint(b) {
		t.Fatal("a one-character system prompt change did not change the fingerprint")
	}
}

// ---------------------------------------------------------------------------
// CacheEconomics
// ---------------------------------------------------------------------------

func TestBreakEvenDerivation(t *testing.T) {
	tests := []struct {
		name string
		econ CacheEconomics
		want float64
	}{
		{
			// (1.25-0.10)/(1-0.10) = 1.278. Anthropic's cache pays back on the
			// first reuse, so any epoch of two calls or more is profitable.
			name: "anthropic pays back on first reuse",
			econ: CacheEconomics{WriteMultiplier: 1.25, ReadMultiplier: 0.10},
			want: 1.15 / 0.9,
		},
		{
			// Free writes still need the call that performs the write.
			name: "free write floors at one call",
			econ: CacheEconomics{WriteMultiplier: 1.00, ReadMultiplier: 0.50},
			want: 1,
		},
		{
			name: "expensive write needs more reuse",
			econ: CacheEconomics{WriteMultiplier: 2.00, ReadMultiplier: 0.50},
			want: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.econ.BreakEvenCalls()
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("break-even = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBreakEvenIsInfiniteWithoutADiscount(t *testing.T) {
	// A provider whose cache reads cost full price can never repay a write, at
	// any epoch length. Reporting a finite number here would approve a caching
	// strategy that loses money on every call.
	if got := noCaching.BreakEvenCalls(); !math.IsInf(got, 1) {
		t.Fatalf("no-discount break-even = %v, want +Inf", got)
	}
	if got := (CacheEconomics{WriteMultiplier: 1, ReadMultiplier: 1.2}).BreakEvenCalls(); !math.IsInf(got, 1) {
		t.Fatalf("read-penalty break-even = %v, want +Inf", got)
	}
}

func TestEconomicsForUnknownProviderRefusesToGuess(t *testing.T) {
	got := EconomicsFor("some-cli-engine-with-no-published-rates")
	if !math.IsInf(got.BreakEvenCalls(), 1) {
		t.Fatal("an unknown provider was credited with a working cache")
	}
}

// ---------------------------------------------------------------------------
// Segment
// ---------------------------------------------------------------------------

// receiptAt builds an admitted receipt for segmentation tests.
func receiptAt(scope, provider, model, prefix string, at time.Time) Receipt {
	return Receipt{
		Scope:    scope,
		Provider: provider,
		Model:    model,
		Prefix:   prefix,
		Started:  at,
		Duration: time.Second,
		Decision: Decision{Allowed: true, Code: DecisionAdmitted},
		Actual:   Spend{InputTokens: 100, Calls: 1},
	}
}

func TestSegmentCutsWhenThePrefixChanges(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	rs := []Receipt{
		receiptAt("s1", "anthropic", "m", "A", t0),
		receiptAt("s1", "anthropic", "m", "A", t0.Add(time.Second)),
		receiptAt("s1", "anthropic", "m", "B", t0.Add(2*time.Second)),
		receiptAt("s1", "anthropic", "m", "A", t0.Add(3*time.Second)),
	}

	epochs := Segment(rs)
	if len(epochs) != 3 {
		t.Fatalf("epochs = %d, want 3", len(epochs))
	}
	// The fourth call returns to prefix A, but the cache entry was displaced by
	// B in between: a return is a new epoch, not a resumption of the old one.
	want := []int{2, 1, 1}
	for i, w := range want {
		if epochs[i].Calls != w {
			t.Fatalf("epoch %d calls = %d, want %d", i, epochs[i].Calls, w)
		}
	}
}

func TestSegmentDoesNotSpliceConcurrentScopes(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	// Two sessions interleaving on the wall clock, each with a stable prefix.
	// Ungrouped, this reads as six alternating singletons; grouped, it is two
	// epochs of three. The difference is the whole Q1 answer.
	var rs []Receipt
	for i := 0; i < 3; i++ {
		rs = append(rs,
			receiptAt("s1", "anthropic", "m", "A", t0.Add(time.Duration(2*i)*time.Second)),
			receiptAt("s2", "anthropic", "m", "B", t0.Add(time.Duration(2*i+1)*time.Second)),
		)
	}

	epochs := Segment(rs)
	if len(epochs) != 2 {
		t.Fatalf("epochs = %d, want 2 (one per scope)", len(epochs))
	}
	for _, e := range epochs {
		if e.Calls != 3 {
			t.Fatalf("scope %q epoch has %d calls, want 3", e.Scope, e.Calls)
		}
	}
}

func TestSegmentSeparatesModelsWithinAScope(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	rs := []Receipt{
		receiptAt("s1", "anthropic", "haiku", "A", t0),
		receiptAt("s1", "anthropic", "opus", "A", t0.Add(time.Second)),
		receiptAt("s1", "anthropic", "haiku", "A", t0.Add(2*time.Second)),
	}
	// The same text tokenizes into a different cache entry per model, so a
	// model switch is a cache miss even with an identical prefix.
	if got := len(Segment(rs)); got != 2 {
		t.Fatalf("epochs = %d, want 2 (one per model)", got)
	}
}

func TestSegmentExcludesRefusals(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	refused := receiptAt("s1", "anthropic", "m", "A", t0.Add(time.Second))
	refused.Decision = Decision{Allowed: false, Code: DecisionWindowExceeded}

	rs := []Receipt{
		receiptAt("s1", "anthropic", "m", "A", t0),
		refused,
		receiptAt("s1", "anthropic", "m", "A", t0.Add(2*time.Second)),
	}

	epochs := Segment(rs)
	if len(epochs) != 1 {
		t.Fatalf("epochs = %d, want 1", len(epochs))
	}
	// A refused call never reached the provider, so it neither warmed nor read
	// a cache. Counting it would inflate the epoch with a call that could not
	// have benefited.
	if epochs[0].Calls != 2 {
		t.Fatalf("calls = %d, want 2 (the refusal must not be counted)", epochs[0].Calls)
	}
}

func TestSegmentOrdersByStartNotByArrival(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	// Receipts settle when a call finishes, so a slow early call lands in the
	// ring after a fast later one. Segmenting in arrival order would cut epochs
	// at points that never happened.
	rs := []Receipt{
		receiptAt("s1", "anthropic", "m", "A", t0.Add(2*time.Second)),
		receiptAt("s1", "anthropic", "m", "A", t0),
		receiptAt("s1", "anthropic", "m", "A", t0.Add(time.Second)),
	}

	epochs := Segment(rs)
	if len(epochs) != 1 || epochs[0].Calls != 3 {
		t.Fatalf("epochs = %+v, want one epoch of 3", epochs)
	}
	if !epochs[0].Started.Equal(t0) {
		t.Fatalf("epoch start = %v, want %v", epochs[0].Started, t0)
	}
}

func TestSegmentAccumulatesBilledTotals(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	a := receiptAt("s1", "anthropic", "m", "A", t0)
	a.Actual = Spend{InputTokens: 1000, CachedTokens: 0}
	a.Estimated.Segments = Segments{System: 700, Tools: 200, User: 100}
	b := receiptAt("s1", "anthropic", "m", "A", t0.Add(time.Second))
	b.Actual = Spend{InputTokens: 120, CachedTokens: 900}

	epochs := Segment([]Receipt{a, b})
	if len(epochs) != 1 {
		t.Fatalf("epochs = %d, want 1", len(epochs))
	}
	e := epochs[0]
	if e.InputTokens != 1120 || e.CachedTokens != 900 {
		t.Fatalf("totals = in %d cached %d, want in 1120 cached 900", e.InputTokens, e.CachedTokens)
	}
	// PrefixTokens comes from the first receipt, which is the one that paid to
	// write the entry.
	if e.PrefixTokens != 900 {
		t.Fatalf("prefix tokens = %d, want 900 (system 700 + tools 200)", e.PrefixTokens)
	}
}

func TestSegmentEmptyInput(t *testing.T) {
	if got := Segment(nil); len(got) != 0 {
		t.Fatalf("Segment(nil) = %v, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Histogram
// ---------------------------------------------------------------------------

// epochsOfLengths builds one epoch per given call count, all in distinct scopes
// so segmentation is not re-run.
func epochsOfLengths(provider string, lengths ...int) []Epoch {
	t0 := time.Unix(1700000000, 0)
	out := make([]Epoch, 0, len(lengths))
	for i, n := range lengths {
		out = append(out, Epoch{
			Scope:    string(rune('a' + i)),
			Provider: provider,
			Model:    "m",
			Prefix:   "p",
			Calls:    n,
			Started:  t0,
			Ended:    t0.Add(time.Second),
		})
	}
	return out
}

func TestHistogramEmpty(t *testing.T) {
	h := Histogram(nil)
	if h.Epochs != 0 || h.Calls != 0 {
		t.Fatalf("empty histogram reported %d epochs / %d calls", h.Epochs, h.Calls)
	}
	// The buckets must still be present so a caller rendering a chart does not
	// have to special-case "no data".
	if len(h.Buckets) != len(bucketBounds) {
		t.Fatalf("buckets = %d, want %d", len(h.Buckets), len(bucketBounds))
	}
}

func TestHistogramBucketsAndPercentiles(t *testing.T) {
	h := Histogram(epochsOfLengths("anthropic", 1, 1, 2, 3, 5, 9, 17, 33, 100, 4))

	if h.Epochs != 10 {
		t.Fatalf("epochs = %d, want 10", h.Epochs)
	}
	if h.Calls != 1+1+2+3+5+9+17+33+100+4 {
		t.Fatalf("calls = %d", h.Calls)
	}
	if h.Min != 1 || h.Max != 100 {
		t.Fatalf("min/max = %d/%d, want 1/100", h.Min, h.Max)
	}
	if h.Singletons != 2 {
		t.Fatalf("singletons = %d, want 2", h.Singletons)
	}
	if math.Abs(h.SingletonPct-20) > 1e-9 {
		t.Fatalf("singleton pct = %v, want 20", h.SingletonPct)
	}

	// sorted: 1 1 2 3 4 5 9 17 33 100. Nearest rank keeps the reported value an
	// epoch length that actually occurred rather than an interpolation.
	if h.P50 != 4 {
		t.Fatalf("p50 = %d, want 4", h.P50)
	}
	if h.P90 != 33 {
		t.Fatalf("p90 = %d, want 33", h.P90)
	}

	byLabel := map[string]int{}
	for _, b := range h.Buckets {
		byLabel[b.Label] = b.Count
	}
	want := map[string]int{"1": 2, "2": 1, "3-4": 2, "5-8": 1, "9-16": 1, "17-32": 1, "33+": 2}
	for label, n := range want {
		if byLabel[label] != n {
			t.Fatalf("bucket %q = %d, want %d", label, byLabel[label], n)
		}
	}

	total := 0
	for _, b := range h.Buckets {
		total += b.Count
	}
	// Every epoch lands in exactly one bucket. A gap or an overlap in the
	// bounds would silently drop or double-count evidence.
	if total != h.Epochs {
		t.Fatalf("bucket total = %d, want %d", total, h.Epochs)
	}
}

func TestHistogramCountsNoPrefixCallsSeparately(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	epochs := []Epoch{
		{Provider: "anthropic", Prefix: "", Calls: 4, Started: t0, Ended: t0.Add(time.Second)},
		{Provider: "anthropic", Prefix: "p", Calls: 6, Started: t0, Ended: t0.Add(time.Second)},
	}
	h := Histogram(epochs)

	// A long epoch with no cacheable head is a different problem from a short
	// one, and needs a different fix. Folding it into the singletons would hide
	// it entirely.
	if h.NoPrefixCalls != 4 {
		t.Fatalf("no-prefix calls = %d, want 4", h.NoPrefixCalls)
	}
	if math.Abs(h.NoPrefixPct-40) > 1e-9 {
		t.Fatalf("no-prefix pct = %v, want 40", h.NoPrefixPct)
	}
	if len(h.ByProvider) != 1 || h.ByProvider[0].PayingEpochs != 1 {
		t.Fatalf("paying epochs = %+v, want only the prefixed epoch", h.ByProvider)
	}
}

func TestHistogramPerProviderBreakEven(t *testing.T) {
	epochs := append(
		epochsOfLengths("anthropic", 1, 2, 5),
		epochsOfLengths("some-cli", 1, 2, 5)...,
	)
	h := Histogram(epochs)

	if len(h.ByProvider) != 2 {
		t.Fatalf("providers = %d, want 2", len(h.ByProvider))
	}

	byName := map[string]ProviderVerdict{}
	for _, pv := range h.ByProvider {
		byName[pv.Provider] = pv
	}

	// Anthropic's break-even is 1.28, so the 2- and 5-call epochs pay.
	if got := byName["anthropic"].PayingEpochs; got != 2 {
		t.Fatalf("anthropic paying = %d, want 2", got)
	}
	// A provider with no published cache economics pays on nothing. Reporting
	// otherwise would credit a discount that does not exist.
	if got := byName["some-cli"].PayingEpochs; got != 0 {
		t.Fatalf("unknown-provider paying = %d, want 0", got)
	}
}

func TestHistogramTreatsTTLExpiryAsNotPaying(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	epochs := []Epoch{
		// Long enough to clear break-even, but spread over an hour: on a
		// five-minute cache every call after the first is a miss.
		{Provider: "anthropic", Prefix: "p", Calls: 20, Started: t0, Ended: t0.Add(time.Hour)},
		{Provider: "anthropic", Prefix: "p", Calls: 20, Started: t0, Ended: t0.Add(time.Minute)},
	}
	h := Histogram(epochs)

	pv := h.ByProvider[0]
	if pv.ExpiredEpochs != 1 {
		t.Fatalf("expired = %d, want 1", pv.ExpiredEpochs)
	}
	if pv.PayingEpochs != 1 {
		t.Fatalf("paying = %d, want 1 (the expired epoch must not count)", pv.PayingEpochs)
	}
}

func TestHistogramReuseRatio(t *testing.T) {
	h := Histogram(epochsOfLengths("anthropic", 1, 1, 1, 1))
	// Four epochs, four calls: no prefix ever served a second call. This is the
	// shape that kills Phase 4, and it must be unmistakable in the readout.
	if math.Abs(h.ReuseRatio-1) > 1e-9 {
		t.Fatalf("reuse ratio = %v, want 1", h.ReuseRatio)
	}
	if math.Abs(h.SingletonPct-100) > 1e-9 {
		t.Fatalf("singleton pct = %v, want 100", h.SingletonPct)
	}
}

func TestPercentileNearestRankEdges(t *testing.T) {
	if got := percentileInts(nil, 50); got != 0 {
		t.Fatalf("percentile of empty = %d, want 0", got)
	}
	one := []int{7}
	for _, p := range []int{0, 1, 50, 99, 100} {
		if got := percentileInts(one, p); got != 7 {
			t.Fatalf("p%d of single-element = %d, want 7", p, got)
		}
	}
	ten := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := percentileInts(ten, 100); got != 10 {
		t.Fatalf("p100 = %d, want 10", got)
	}
	if got := percentileInts(ten, 10); got != 1 {
		t.Fatalf("p10 = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// End to end: capture through the real broker, then segment
// ---------------------------------------------------------------------------

// TestEpochsFormFromRealBrokeredCalls is the wiring proof for Q1. The analysis
// above is worthless if the capture never happens, and a fingerprint computed
// on a request the broker does not actually build would produce a beautiful,
// meaningless histogram.
func TestEpochsFormFromRealBrokeredCalls(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	client := meteredClient(t, newFakeClient(), meter, sink)

	ctx := usage.WithSessionID(context.Background(), "session-alpha")

	const stable = "you are codenerd"
	// Three calls on one system prompt, then a fourth that changes it.
	for i := 0; i < 3; i++ {
		if _, err := client.CompleteWithSystem(ctx, stable, "turn"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if _, err := client.CompleteWithSystem(ctx, stable+" (revised)", "turn"); err != nil {
		t.Fatalf("revised call: %v", err)
	}

	receipts := sink.all()
	if len(receipts) != 4 {
		t.Fatalf("receipts = %d, want 4", len(receipts))
	}
	for i, r := range receipts {
		if r.Scope != "session-alpha" {
			t.Fatalf("receipt %d scope = %q, want session-alpha", i, r.Scope)
		}
		if r.Prefix == "" {
			t.Fatalf("receipt %d carries no prefix fingerprint despite a system prompt", i)
		}
	}

	epochs := Segment(receipts)
	if len(epochs) != 2 {
		t.Fatalf("epochs = %d, want 2", len(epochs))
	}
	if epochs[0].Calls != 3 || epochs[1].Calls != 1 {
		t.Fatalf("epoch lengths = %d,%d, want 3,1", epochs[0].Calls, epochs[1].Calls)
	}

	h := Histogram(epochs)
	if h.Singletons != 1 {
		t.Fatalf("singletons = %d, want 1", h.Singletons)
	}
	if math.Abs(h.ReuseRatio-2) > 1e-9 {
		t.Fatalf("reuse ratio = %v, want 2 (4 calls over 2 epochs)", h.ReuseRatio)
	}
}

// TestUntaggedCallsShareTheGlobalScope records the behaviour when nothing tags
// the context, so a reader of a histogram knows what an empty scope means
// rather than guessing.
func TestUntaggedCallsShareTheGlobalScope(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	client := meteredClient(t, newFakeClient(), meter, sink)

	for i := 0; i < 2; i++ {
		if _, err := client.CompleteWithSystem(context.Background(), "sys", "turn"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	for _, r := range sink.all() {
		if r.Scope != "" {
			t.Fatalf("untagged call got scope %q, want empty", r.Scope)
		}
	}
	// They merge into one epoch, which is correct for a single-session process
	// and wrong the moment two sessions run untagged in one process. That is a
	// wiring gap in the caller, not in the meter, and it shows up as an
	// implausibly long epoch rather than as silence.
	if got := len(Segment(sink.all())); got != 1 {
		t.Fatalf("epochs = %d, want 1", got)
	}
}

// TestPrefixIsBlankWhenThereIsNothingToCache proves the no-prefix path reaches
// the receipt, since that population is what NoPrefixCalls reports on.
func TestPrefixIsBlankWhenThereIsNothingToCache(t *testing.T) {
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	client := meteredClient(t, newFakeClient(), meter, sink)

	if _, err := client.Complete(context.Background(), "just a bare prompt"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	r, ok := sink.last()
	if !ok {
		t.Fatal("no receipt emitted")
	}
	if r.Prefix != "" {
		t.Fatalf("prefix = %q, want empty for a call with no system prompt and no tools", r.Prefix)
	}
}
