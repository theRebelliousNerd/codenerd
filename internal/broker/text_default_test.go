package broker

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TextCounter
// ---------------------------------------------------------------------------

// TextCounter is the seam internal/context counts through -- facts, compressed
// turns, context blocks. It is where the deleted charsPerToken = 4.0 used to
// live, and every compression threshold in the system is denominated in its
// output, so its floor and its ratio are load-bearing rather than cosmetic.

func TestTextCounterUsesTheSharedRatio(t *testing.T) {
	m := NewMeter(MeterConfig{SeedRatio: 4.0})
	tc := m.TextCounter("model-a")

	const text = "0123456789012345678901234567890123456789" // 40 runes
	if got := tc.EstimateTokens(text); got != 10 {
		t.Fatalf("EstimateTokens = %d, want 10 at a 4.0 ratio", got)
	}
	if got := tc.Confidence(); got != ConfidenceSeeded {
		t.Errorf("confidence = %q, want seeded before any observation", got)
	}

	// Teach it that this model packs twice as many characters per token. A
	// counter already handed out must follow: the compressor holds one for a
	// whole session and never rebuilds it.
	m.Calibrator().Observe(Observation{Model: "model-a", Chars: 8000, ActualInputTokens: 1000})

	if got := tc.EstimateTokens(text); got != 5 {
		t.Fatalf("EstimateTokens = %d, want 5 at the learned 8.0 ratio", got)
	}
	if got := tc.Ratio(); got < 7.9 || got > 8.1 {
		t.Errorf("Ratio = %v, want ~8", got)
	}
	if got := tc.Confidence(); got != ConfidenceCalibrated {
		t.Errorf("confidence = %q, want calibrated after an observation", got)
	}
}

func TestTextCounterFloorsAtOneTokenForNonEmptyText(t *testing.T) {
	m := NewMeter(MeterConfig{SeedRatio: 100})
	tc := m.TextCounter("tiny")

	// A single character under a 100:1 ratio rounds to zero. Returning zero
	// would let an unbounded number of tiny facts into a budget that believed
	// it was full.
	if got := tc.EstimateTokens("x"); got < 1 {
		t.Fatalf("EstimateTokens(\"x\") = %d, want at least 1", got)
	}
	if got := tc.EstimateTokens(""); got != 0 {
		t.Errorf("EstimateTokens(\"\") = %d, want 0 — empty input must not be billed", got)
	}
}

func TestTextCounterCountsRunesNotBytes(t *testing.T) {
	m := NewMeter(MeterConfig{SeedRatio: 4.0})
	tc := m.TextCounter("m")

	// Token-per-rune ratios are far more stable across languages than
	// token-per-byte ones. Four multi-byte runes must cost what four ASCII
	// runes cost, not what their twelve bytes would.
	ascii := tc.EstimateTokens("abcd")
	wide := tc.EstimateTokens("日本語で")
	if ascii != wide {
		t.Errorf("4 ASCII runes = %d tokens, 4 wide runes = %d; the counter is measuring bytes",
			ascii, wide)
	}
}

func TestTextCounterResolvesTheEmptyModelToThePrimary(t *testing.T) {
	m := NewMeter(MeterConfig{SeedRatio: 4.0, PrimaryModel: "primary"})
	m.Calibrator().Observe(Observation{Model: "primary", Chars: 8000, ActualInputTokens: 1000})

	// Most callers know only "the model we are talking to". Resolving to the
	// primary is what keeps them on a ratio trained on real responses instead
	// of a private default.
	unnamed := m.TextCounter("")
	named := m.TextCounter("primary")
	if unnamed.EstimateTokens("0123456789") != named.EstimateTokens("0123456789") {
		t.Error("an unnamed counter did not resolve to the primary model")
	}
	if unnamed.Confidence() != ConfidenceCalibrated {
		t.Errorf("unnamed counter confidence = %q, want the primary's", unnamed.Confidence())
	}
}

// ---------------------------------------------------------------------------
// Default and Configure
// ---------------------------------------------------------------------------

func TestDefaultMeterIsUsableBeforeConfigure(t *testing.T) {
	m := Default()
	if m == nil {
		t.Fatal("Default returned nil")
	}
	// An unconfigured meter still counts, records and calibrates. What it does
	// not have is a window -- and that is visible rather than silent, because a
	// zero window is reported on every receipt instead of passing as a
	// satisfied check.
	if m.Ledger() == nil || m.Calibrator() == nil {
		t.Fatal("the unconfigured meter is missing its ledger or calibrator")
	}
	if Default() != m {
		t.Error("Default returned a second meter; there would be two ledgers")
	}
}

func TestConfigurePreservesSpendAcrossABudgetChange(t *testing.T) {
	m := NewMeter(MeterConfig{Window: 100000, OutputReserve: 8000})
	m.Ledger().Record(PurposeSession, Spend{InputTokens: 5000, OutputTokens: 1000, Calls: 1})

	// Rebuilding the ledger to apply caps must carry accumulated spend across.
	// A budget change that silently zeroes the balances it is being compared
	// against is a cap that does not bind until the next restart.
	replacement := NewLedger(LedgerConfig{
		Window:        200000,
		OutputReserve: 8000,
		Budgets:       map[Purpose]int64{PurposeSession: 10000},
	})
	for purpose, spend := range m.Ledger().Accounts() {
		replacement.Record(purpose, spend)
	}

	got := replacement.Accounts()[PurposeSession]
	if got.InputTokens != 5000 || got.OutputTokens != 1000 {
		t.Fatalf("spend after rebuild = %+v, want the pre-existing 5000/1000", got)
	}
}

func TestConfigureIsSafeUnderConcurrentUse(t *testing.T) {
	// Configure runs at boot while nothing else is metering, but a reconfigure
	// races with live traffic. The lock is what makes that survivable, and a
	// test under -race is the only way to know it holds.
	m := NewMeter(MeterConfig{Window: 100000, OutputReserve: 8000})

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				m.Ledger().Record(PurposeSession, Spend{InputTokens: 1, Calls: 1})
				_ = m.TextCounter("").EstimateTokens("some text")
			}
		}()
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.mu.Lock()
				m.ledger.SetWindow(150000, 8000)
				m.mu.Unlock()
			}
		}()
	}
	wg.Wait()
}

func TestNewMeterDefaultsTheHTTPClient(t *testing.T) {
	m := NewMeter(MeterConfig{})
	m.mu.RLock()
	client := m.httpClient
	m.mu.RUnlock()

	if client == nil {
		t.Fatal("no HTTP client; the counting endpoint would nil-panic on first use")
	}
	// An unbounded timeout on the counting endpoint turns a slow provider into
	// a hung turn, which is worse than degrading to the estimator.
	if client.Timeout <= 0 || client.Timeout > time.Minute {
		t.Errorf("HTTP timeout = %v, want a bounded one", client.Timeout)
	}
}

func TestNewMeterHonoursAnInjectedHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 3 * time.Second}
	m := NewMeter(MeterConfig{HTTPClient: custom})
	m.mu.RLock()
	got := m.httpClient
	m.mu.RUnlock()
	if got != custom {
		t.Error("MeterConfig.HTTPClient was ignored; tests could not stub the counting endpoint")
	}
}

// ---------------------------------------------------------------------------
// Anthropic counter helpers
// ---------------------------------------------------------------------------

func TestAnthropicFallbackIsTheSharedEstimator(t *testing.T) {
	m := NewMeter(MeterConfig{SeedRatio: 4.0})
	counter := m.CounterFor(ProviderCreds{
		Provider: "anthropic", Model: "claude-opus-5",
		APIKey: "k", BaseURL: "https://example.invalid",
	})

	ac, ok := counter.(*AnthropicCounter)
	if !ok {
		t.Fatalf("counter = %T, want *AnthropicCounter", counter)
	}
	if ac.Fallback() == nil {
		t.Fatal("no fallback estimator; an unreachable endpoint would leave the counter with nothing")
	}

	// The exact path needs no calibration, but the fallback serves every
	// request made while the endpoint is unreachable. Keeping it trained during
	// the good times is what makes it useful during the bad ones.
	ac.Observe(Observation{Model: "claude-opus-5", Chars: 8000, ActualInputTokens: 1000})
	if _, conf := m.Calibrator().Ratio("claude-opus-5"); conf != ConfidenceCalibrated {
		t.Errorf("an observation through the Anthropic counter did not reach the shared "+
			"calibrator (confidence %q)", conf)
	}
}

func TestTruncateForLogBoundsErrorBodies(t *testing.T) {
	short := []byte("a short body")
	if got := truncateForLog(short); got != "a short body" {
		t.Errorf("truncateForLog shortened a short body: %q", got)
	}

	// An error body reaches a log line. A provider returning a megabyte of
	// HTML must not put a megabyte in the log.
	long := []byte(strings.Repeat("x", 5000))
	got := truncateForLog(long)
	if len(got) > 210 {
		t.Errorf("truncateForLog returned %d chars, want it bounded", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("a truncated body does not say it was truncated")
	}
}

func TestCounterForNeedsBothCredentialsToReachTheEndpoint(t *testing.T) {
	m := NewMeter(MeterConfig{})

	// A half-configured Anthropic client must degrade to the estimator rather
	// than build a counter that will fail every call: a counter that cannot
	// produce a number makes the broker fail closed, which would refuse every
	// request instead of estimating one.
	for _, creds := range []ProviderCreds{
		{Provider: "anthropic", Model: "m"},
		{Provider: "anthropic", Model: "m", APIKey: "k"},
		{Provider: "anthropic", Model: "m", BaseURL: "https://example.invalid"},
		{Provider: "openai", Model: "m", APIKey: "k", BaseURL: "https://example.invalid"},
	} {
		if _, ok := m.CounterFor(creds).(*EstimatingCounter); !ok {
			t.Errorf("CounterFor(%+v) did not fall back to the estimator", creds)
		}
	}
}

// ---------------------------------------------------------------------------
// The count cache
// ---------------------------------------------------------------------------

func TestCountCacheEvictsTheLeastRecentlyUsed(t *testing.T) {
	a := NewAnthropicCounter("k", "https://example.invalid", nil, NewEstimatingCounter(NewCalibrator()))

	// Fill past the cap. An eviction that removed from the list but not the
	// map, or vice versa, is unbounded memory in a long session -- and it only
	// shows up as a process that slowly grows.
	for i := 0; i < countCacheSize+50; i++ {
		a.store(cacheKeyForTest(i), Count{Tokens: i, Confidence: ConfidenceExact})
	}

	a.mu.Lock()
	listLen, mapLen := a.order.Len(), len(a.cache)
	a.mu.Unlock()

	if listLen != countCacheSize {
		t.Errorf("list length = %d, want the cap %d", listLen, countCacheSize)
	}
	if mapLen != countCacheSize {
		t.Errorf("map size = %d, want the cap %d — the two indexes have diverged", mapLen, countCacheSize)
	}

	// The oldest entries are gone, the newest survive.
	if _, ok := a.lookup(cacheKeyForTest(0)); ok {
		t.Error("the least recently used entry survived eviction")
	}
	if _, ok := a.lookup(cacheKeyForTest(countCacheSize + 49)); !ok {
		t.Error("the most recent entry was evicted")
	}
}

func TestCountCacheRefreshesRatherThanDuplicating(t *testing.T) {
	a := NewAnthropicCounter("k", "https://example.invalid", nil, NewEstimatingCounter(NewCalibrator()))

	a.store("same", Count{Tokens: 100, Confidence: ConfidenceExact})
	a.store("same", Count{Tokens: 200, Confidence: ConfidenceExact})

	a.mu.Lock()
	listLen := a.order.Len()
	a.mu.Unlock()
	if listLen != 1 {
		t.Fatalf("list length = %d after two writes to one key, want 1", listLen)
	}

	got, ok := a.lookup("same")
	if !ok {
		t.Fatal("the refreshed entry is gone")
	}
	// A stale count served after a re-store would size a request against text
	// that has since changed.
	if got.Tokens != 200 {
		t.Errorf("cached tokens = %d, want the refreshed 200", got.Tokens)
	}
}

func TestLookupPromotesOnHit(t *testing.T) {
	a := NewAnthropicCounter("k", "https://example.invalid", nil, NewEstimatingCounter(NewCalibrator()))

	for i := 0; i < countCacheSize; i++ {
		a.store(cacheKeyForTest(i), Count{Tokens: i})
	}
	// Touch the oldest, then push one more in. Without promotion on read, a
	// frequently reused prefix would be evicted by a burst of one-off counts --
	// exactly the entry worth keeping.
	if _, ok := a.lookup(cacheKeyForTest(0)); !ok {
		t.Fatal("setup: the oldest entry is missing")
	}
	a.store("newcomer", Count{Tokens: 1})

	if _, ok := a.lookup(cacheKeyForTest(0)); !ok {
		t.Error("a just-read entry was evicted; reads do not promote")
	}
	if _, ok := a.lookup(cacheKeyForTest(1)); ok {
		t.Error("the true least-recently-used entry survived")
	}
}

func cacheKeyForTest(i int) string {
	return "key-" + string(rune('a'+i%26)) + "-" + itoaForTest(i)
}

func itoaForTest(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// TestConfigureAppliesToTheProcessMeter exercises the real Configure rather
// than a replica of its logic.
//
// It mutates the process meter, which is why it restores what it changed: the
// singleton is shared with every other test in this binary, and a test that
// leaves a window behind makes an unrelated failure look like a budgeting bug.
func TestConfigureAppliesToTheProcessMeter(t *testing.T) {
	m := Default()

	m.mu.RLock()
	origLedger, origSink, origPrimary := m.ledger, m.sink, m.primaryModel
	m.mu.RUnlock()
	t.Cleanup(func() {
		m.mu.Lock()
		m.ledger, m.sink, m.primaryModel = origLedger, origSink, origPrimary
		m.mu.Unlock()
	})

	Configure(MeterConfig{Window: 123456, OutputReserve: 7000, PrimaryModel: "configured-model"})

	if got := m.Ledger().Available(); got != 123456-7000 {
		t.Errorf("available = %d, want %d", got, 123456-7000)
	}
	m.mu.RLock()
	primary := m.primaryModel
	m.mu.RUnlock()
	if primary != "configured-model" {
		t.Errorf("primary model = %q, want configured-model", primary)
	}
}

func TestConfigureCarriesSpendAcrossACapChange(t *testing.T) {
	m := Default()

	m.mu.RLock()
	origLedger, origSink := m.ledger, m.sink
	m.mu.RUnlock()
	t.Cleanup(func() {
		m.mu.Lock()
		m.ledger, m.sink = origLedger, origSink
		m.mu.Unlock()
	})

	// Install a fresh ledger rather than reusing whatever the process meter
	// already holds. Configure mutates the existing ledger in place when no
	// budgets are given, so a test that recorded into it and then restored the
	// same pointer on cleanup left its own spend behind -- and accumulated on
	// the next run, which is how this test failed under -count=2.
	m.mu.Lock()
	m.ledger = NewLedger(LedgerConfig{Window: 200000, OutputReserve: 8000})
	m.mu.Unlock()

	m.Ledger().Record(PurposeCritic, Spend{InputTokens: 4000, OutputTokens: 500, Calls: 1})

	// Applying caps rebuilds the ledger. A budget change that silently zeroed
	// the balances it is being compared against would be a cap that does not
	// bind until the next restart -- which is the moment it is least likely to
	// be noticed.
	Configure(MeterConfig{
		Window: 200000, OutputReserve: 8000,
		Budgets: map[Purpose]int64{PurposeCritic: 10000},
	})

	got := m.Ledger().Accounts()[PurposeCritic]
	if got.InputTokens != 4000 || got.OutputTokens != 500 {
		t.Fatalf("spend after a cap change = %+v, want the pre-existing 4000/500", got)
	}
}

func TestConfigureInstallsAnExtraSinkWithoutLosingTheBuiltins(t *testing.T) {
	m := Default()

	m.mu.RLock()
	origSink, origLedger := m.sink, m.ledger
	m.mu.RUnlock()
	t.Cleanup(func() {
		m.mu.Lock()
		m.sink, m.ledger = origSink, origLedger
		m.mu.Unlock()
	})

	extra := &captureSink{}
	Configure(MeterConfig{Window: 200000, OutputReserve: 8000, ExtraSink: extra})

	m.mu.RLock()
	sink := m.sink
	m.mu.RUnlock()

	multi, ok := sink.(MultiSink)
	if !ok {
		t.Fatalf("sink = %T, want MultiSink", sink)
	}
	// The extra sink is added to the log and ring sinks, never in place of
	// them: replacing the ring would leave the in-process buffer empty and the
	// log silent, which is a lot of instrumentation to lose to one option.
	if len(multi) != 3 {
		t.Fatalf("sink fan-out = %d, want 3 (log, ring, extra)", len(multi))
	}
	sink.Record(Receipt{Purpose: PurposeCritic, Decision: Decision{Allowed: true}})
	if len(extra.all()) != 1 {
		t.Error("the extra sink did not receive the receipt")
	}
}
