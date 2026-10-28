package broker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// Config configures a broker.
type Config struct {
	Router          ClientRouter
	CredentialAlias string
	// Provider and Model name the backend, for receipts and counting.
	Provider string
	Model    string

	// Counter produces pre-flight token counts. Required.
	Counter Counter
	// Ledger holds limits and recorded spend. Required.
	Ledger *Ledger
	// Sink receives one receipt per call. Optional; nil discards.
	Sink ReceiptSink
	// Reconciler accumulates estimated-versus-billed drift per model.
	// Optional; nil disables reconciliation.
	Reconciler *Reconciler
}

// core carries the metering behaviour shared by every wrapper shape.
//
// It is not exported and is never used directly as an LLMClient: Wrap composes
// it with exactly the optional interfaces the underlying client implements, so
// that a caller probing for types.ToolResultsProvider gets the same answer it
// would have got from the unwrapped client. See wrap.go for why that matters.
type core struct {
	underlying types.LLMClient
	cfg        Config

	mu    sync.RWMutex
	model string
}

// Unwrap returns the client this broker wraps.
//
// Some call sites assert on a concrete client type to make a behavioural choice
// — cmd/nerd/chat tags the Codex CLI engine this way so JIT can select
// engine-specific prompt atoms. Those sites use Base to reach through the
// decorator chain instead of asserting on whatever happens to be outermost.
func (c *core) Unwrap() types.LLMClient { return c.delegate() }

// SetModel forwards to the underlying client when it supports it, and keeps the
// broker's own idea of the model in step so receipts and counts stay correct
// after a mid-session model switch.
func (c *core) SetModel(model string) {
	if model != "" {
		c.mu.Lock()
		c.model = model
		c.mu.Unlock()
	}
	c.eachClient(func(client types.LLMClient) {
		if setter, ok := client.(interface{ SetModel(string) }); ok {
			setter.SetModel(model)
		}
	})
}

// SetCachedContent forwards provider cached-content handles.
func (c *core) SetCachedContent(handle string) {
	if router := c.cfg.Router; router != nil {
		router.ConfigureCurrent(func(client types.LLMClient) {
			if _, supported := Base(client).(interface{ SetCachedContent(string) }); !supported {
				return
			}
			router.Pin(handle != "")
			if setter, ok := client.(interface{ SetCachedContent(string) }); ok {
				setter.SetCachedContent(handle)
			}
		})
		return
	}
	if setter, ok := c.underlying.(interface{ SetCachedContent(string) }); ok {
		setter.SetCachedContent(handle)
	}
}

// SchemaCapable forwards the underlying capability probe. core.AsSchemaCapable
// consults this after the interface assertion succeeds, so a wrapper that
// answered for itself here would re-enable structured output on a client that
// had deliberately switched it off.
func (c *core) SchemaCapable() bool {
	if checker, ok := c.delegate().(interface{ SchemaCapable() bool }); ok {
		return checker.SchemaCapable()
	}
	return true
}

func (c *core) currentModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.model != "" {
		return c.model
	}
	return c.cfg.Provider
}

// callObserver captures the provider's reported usage for one in-flight call.
//
// Each report is one billed attempt, normalized by its adapter. Reports
// accumulate rather than overwrite: a client that retries internally produces
// several reports for one logical call, and every one of them was billed.
type callObserver struct {
	mu      sync.Mutex
	usage   types.UsageMetadata
	reports int
}

// Observed implements usage.Observer.
func (o *callObserver) Observed(r usage.Report) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.reports == 0 {
		o.usage = r.Usage
	} else {
		o.usage = o.usage.Plus(r.Usage)
	}
	o.reports++
}

func (o *callObserver) result() (types.UsageMetadata, int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.usage, o.reports
}

var _ usage.Observer = (*callObserver)(nil)

// admit counts a request and asks the ledger whether it may proceed. It returns
// the receipt skeleton either way so a refusal is still recorded.
func (c *core) admit(ctx context.Context, req *Request) (Receipt, *AdmissionError) {
	purpose := PurposeFromContext(ctx)
	receipt := Receipt{
		Purpose:         purpose,
		CredentialAlias: c.cfg.CredentialAlias,
		Provider:        c.cfg.Provider,
		Model:           req.Model,
		Method:          req.Method,
		Started:         time.Now(),
		Scope:           usage.SessionIDFromContext(ctx),
		Phase:           PhaseFromContext(ctx),
		Prefix:          prefixFingerprint(req),
	}

	count, err := c.cfg.Counter.Count(ctx, req)
	if err != nil {
		// Fail closed. A request whose size is unknown is refused, because a
		// budget that cannot be checked is not a budget.
		receipt.Decision = Decision{
			Code:   DecisionCountUnavailable,
			Reason: err.Error(),
			Window: c.cfg.Ledger.Available(),
		}
		receipt.Err = err.Error()
		return receipt, &AdmissionError{Decision: receipt.Decision, Purpose: purpose}
	}

	decision := c.cfg.Ledger.Admit(purpose, count)
	receipt.Estimated = count
	receipt.Decision = decision

	if !decision.Allowed {
		return receipt, &AdmissionError{Decision: decision, Purpose: purpose}
	}
	return receipt, nil
}

// settle records actual spend, calibrates the counter against it, and emits the
// receipt. It is the only place spend is written to the ledger, which is what
// makes double counting a structural impossibility rather than a convention.
//
// Usage can arrive by two routes: the observer, which every adapter reports
// through on every path (streaming included), and a response-carried block
// (a tool response's Usage, a stream's terminal result). They describe the
// same bill. The observer is primary; the response is the fallback for a
// client that does not report through the usage plumbing, and otherwise only
// consistency evidence — a disagreement is recorded on the receipt, never
// added as a second charge.
func (c *core) settle(receipt Receipt, req *Request, obs *callObserver, reported *types.UsageMetadata, callErr error) {
	receipt.Duration = time.Since(receipt.Started)
	if callErr != nil {
		receipt.Err = callErr.Error()
	}

	observed, reports := obs.result()
	var report types.UsageMetadata
	switch {
	case reports > 0:
		report = observed
		receipt.Attempts = reports
		if reported != nil && reports == 1 {
			if !observed.SameCounts(*reported) {
				receipt.UsageConflict = fmt.Sprintf(
					"observer in=%d out=%d cache_read=%d cache_write=%d reasoning=%d; response in=%d out=%d cache_read=%d cache_write=%d reasoning=%d",
					observed.InputTokens, observed.OutputTokens, observed.CachedContentTokens, observed.CacheWriteTokens, observed.ThinkingTokens,
					reported.InputTokens, reported.OutputTokens, reported.CachedContentTokens, reported.CacheWriteTokens, reported.ThinkingTokens)
			}
			report = enrichUsage(report, *reported)
		}
	case reported != nil && responseReported(*reported):
		report = *reported
		if report.InputTokens > 0 || report.OutputTokens > 0 {
			report.Known |= types.UsageTokens
		}
		receipt.Attempts = 1
	}

	actual := Spend{
		InputTokens:      int64(report.InputTokens),
		OutputTokens:     int64(report.OutputTokens),
		CachedTokens:     int64(report.CachedContentTokens),
		CacheWriteTokens: int64(report.CacheWriteTokens),
		ThinkingTokens:   int64(report.ThinkingTokens),
	}
	if actual.InputTokens > 0 || actual.OutputTokens > 0 {
		actual.Calls = 1
	}
	receipt.UsageKnown = report.Known
	receipt.UsageStatus = usageStatus(report)

	receipt.Actual = actual
	if actual.InputTokens > 0 {
		receipt.EstimateErrorPct = float64(receipt.Estimated.Tokens-int(actual.InputTokens)) /
			float64(actual.InputTokens) * 100
	}

	c.cfg.Ledger.Record(receipt.Purpose, actual)

	// The estimator predicts one request. A multi-attempt total, or a partial
	// count from a stream cut before its usage block, is not the bill of the
	// request that was counted, and training on it would teach the ratio a
	// prompt size that was never sent. Spend still counts every attempt above.
	if receipt.Attempts <= 1 && !report.Partial {
		c.calibrate(req, actual)
		// Reconciliation compares the prediction against the bill. Calibration
		// alone cannot detect a bias shared by both sides -- it would quietly
		// absorb it into the ratio -- so the comparison is kept separately.
		if c.cfg.Reconciler != nil && receipt.Estimated.Tokens > 0 && actual.InputTokens > 0 {
			c.cfg.Reconciler.Observe(req.Model, receipt.Estimated.Tokens, actual.InputTokens)
		}
	}

	c.emit(receipt)
}

// responseReported reports whether a response-carried usage block holds any
// observation: a provider-flagged report, or nonzero totals from a client that
// predates the presence flags.
func responseReported(u types.UsageMetadata) bool {
	return u.Reported() || u.InputTokens > 0 || u.OutputTokens > 0
}

// enrichUsage fills categories the observer did not know from a response
// block that agreed on everything both knew. A category the observer knew is
// never overwritten.
func enrichUsage(observed, reported types.UsageMetadata) types.UsageMetadata {
	if !observed.SameCounts(reported) {
		return observed
	}
	if !observed.Has(types.UsageCacheRead) && reported.Has(types.UsageCacheRead) {
		observed.CachedContentTokens = reported.CachedContentTokens
		observed.Known |= types.UsageCacheRead
	}
	if !observed.Has(types.UsageCacheWrite) && reported.Has(types.UsageCacheWrite) {
		observed.CacheWriteTokens = reported.CacheWriteTokens
		observed.Known |= types.UsageCacheWrite
	}
	if !observed.Has(types.UsageReasoning) && reported.Has(types.UsageReasoning) {
		observed.ThinkingTokens = reported.ThinkingTokens
		observed.Known |= types.UsageReasoning
	}
	return observed
}

// usageStatus classifies what the receipt's Actual means: a complete
// provider report, lower bounds from a cut stream, or nothing reported at all
// (a zero Actual that is NOT a free call).
func usageStatus(u types.UsageMetadata) UsageStatus {
	switch {
	case u.Partial:
		return UsagePartial
	case u.Reported() || u.InputTokens > 0 || u.OutputTokens > 0:
		return UsageReported
	default:
		return UsageUnreported
	}
}

// calibrate feeds the provider's actual back into the counter so the next
// estimate is closer. This is the loop that makes the estimator different in
// kind from the constant it replaced.
func (c *core) calibrate(req *Request, actual Spend) {
	cal, ok := c.cfg.Counter.(CalibratingCounter)
	if !ok || req == nil {
		return
	}

	// InputTokens already includes cache reads (the contract in settle). This
	// used to add CachedTokens back, for a provider that excludes them; no
	// client reports that way, so every cache hit was counted twice. Measured
	// 2026-09-22 over 1,360 Meta receipts: cached <= input on every one, and the
	// estimate ran 1.40-1.50x the bill at every hit rate -- the calibrated ratio
	// learned a token count that was never sent, and the reconciler reported
	// "drifting" against a bill inflated by the cache it was meant to credit.
	input := actual.InputTokens
	if input <= 0 {
		return
	}

	cal.Observe(Observation{
		Model:             req.Model,
		Chars:             measure(req).Total(),
		ActualInputTokens: int(input),
	})
}

func (c *core) emit(receipt Receipt) {
	if c.cfg.Sink != nil {
		c.cfg.Sink.Record(receipt)
	}
}

// observed installs a capture observer on ctx and returns both.
func (c *core) observed(ctx context.Context) (context.Context, *callObserver) {
	obs := &callObserver{}
	return usage.WithObserver(ctx, obs), obs
}

// ---------------------------------------------------------------------------
// types.LLMClient
// ---------------------------------------------------------------------------

// Complete implements types.LLMClient.
func (c *core) Complete(ctx context.Context, prompt string) (string, error) {
	if c.cfg.Router != nil {
		return routeCall(ctx, c.cfg.Router, func(ctx context.Context, client types.LLMClient) (string, error) { return client.Complete(ctx, prompt) })
	}
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "Complete",
		User:     prompt,
	}
	// No system prompt on this method, so a restatement instruction rides on
	// the prompt itself.
	return compressing(c, ctx, req, true, nil, func(ctx context.Context, instruction string) (string, error) {
		return c.underlying.Complete(ctx, withInstruction(prompt, instruction))
	})
}

// CompleteWithSystem implements types.LLMClient.
func (c *core) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.cfg.Router != nil {
		return routeCall(ctx, c.cfg.Router, func(ctx context.Context, client types.LLMClient) (string, error) {
			return client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		})
	}
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "CompleteWithSystem",
		System:   systemPrompt,
		User:     userPrompt,
	}
	return compressing(c, ctx, req, false, nil, func(ctx context.Context, instruction string) (string, error) {
		return c.underlying.CompleteWithSystem(ctx, withInstruction(systemPrompt, instruction), userPrompt)
	})
}

// CompleteWithTools implements types.LLMClient.
func (c *core) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if c.cfg.Router != nil {
		return routeCall(ctx, c.cfg.Router, func(ctx context.Context, client types.LLMClient) (*types.LLMToolResponse, error) {
			return client.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
		})
	}
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "CompleteWithTools",
		System:   systemPrompt,
		User:     userPrompt,
		Tools:    tools,
	}
	return compressing(c, ctx, req, false, usageOf, func(ctx context.Context, instruction string) (*types.LLMToolResponse, error) {
		return c.underlying.CompleteWithTools(ctx, withInstruction(systemPrompt, instruction), userPrompt, tools)
	})
}

// CompleteWithStreaming implements types.LLMClient.
//
// Streaming settles late: the provider reports usage as the stream progresses
// and finishes only when it closes. The returned channels are proxied so the
// receipt is emitted when the turn actually ends rather than when it starts,
// which is also the only way a streamed turn's real cost is ever recorded.
func (c *core) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	if c.cfg.Router != nil {
		content, _, done := c.cfg.Router.Stream(ctx, func(ctx context.Context, client types.LLMClient) (<-chan string, <-chan string, <-chan types.StreamResult) {
			content, done := client.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
			return content, nil, done
		})
		return content, done
	}
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "CompleteWithStreaming",
		System:   systemPrompt,
		User:     userPrompt,
	}

	receipt, refusal := c.admit(ctx, req)
	if refusal != nil {
		c.settleRefusal(receipt)
		return types.FailedStream(refusal)
	}

	ctx, obs := c.observed(ctx)
	content, done := c.underlying.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
	w, outContent, outDone := types.NewStream()
	c.proxyStream(ctx, w, receipt, req, obs, content, nil, done)
	return outContent, outDone
}

// settleRefusal emits a receipt for a call that never reached the provider.
// Nothing is recorded against the ledger: refusing a request costs no tokens,
// and pretending otherwise would inflate the very number this package exists
// to make trustworthy.
func (c *core) settleRefusal(receipt Receipt) {
	receipt.Duration = time.Since(receipt.Started)
	c.emit(receipt)
}

func usageOf(resp *types.LLMToolResponse) *types.UsageMetadata {
	if resp == nil {
		return nil
	}
	return &resp.Usage
}
