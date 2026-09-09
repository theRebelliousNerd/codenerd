package broker

import (
	"context"
	"sync"
	"time"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// Config configures a broker.
type Config struct {
	// Provider and Model name the backend, for receipts and counting.
	Provider string
	Model    string

	// Counter produces pre-flight token counts. Required.
	Counter Counter
	// Ledger holds limits and recorded spend. Required.
	Ledger *Ledger
	// Sink receives one receipt per call. Optional; nil discards.
	Sink ReceiptSink
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
func (c *core) Unwrap() types.LLMClient { return c.underlying }

// SetModel forwards to the underlying client when it supports it, and keeps the
// broker's own idea of the model in step so receipts and counts stay correct
// after a mid-session model switch.
func (c *core) SetModel(model string) {
	if model != "" {
		c.mu.Lock()
		c.model = model
		c.mu.Unlock()
	}
	if setter, ok := c.underlying.(interface{ SetModel(string) }); ok {
		setter.SetModel(model)
	}
}

// SetCachedContent forwards provider cached-content handles.
func (c *core) SetCachedContent(handle string) {
	if p, ok := c.underlying.(interface{ SetCachedContent(string) }); ok {
		p.SetCachedContent(handle)
	}
}

// SchemaCapable forwards the underlying capability probe. core.AsSchemaCapable
// consults this after the interface assertion succeeds, so a wrapper that
// answered for itself here would re-enable structured output on a client that
// had deliberately switched it off.
func (c *core) SchemaCapable() bool {
	if checker, ok := c.underlying.(interface{ SchemaCapable() bool }); ok {
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
// Reports accumulate rather than overwrite: a client that retries internally,
// or a streaming turn that reports input at message_start and output at
// message_delta, produces several reports for one logical call, and every one
// of them was billed.
type callObserver struct {
	mu    sync.Mutex
	spend Spend
	seen  bool
}

// Observed implements usage.Observer.
func (o *callObserver) Observed(_, _ string, input, output int, _ string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if input > 0 {
		o.spend.InputTokens += int64(input)
	}
	if output > 0 {
		o.spend.OutputTokens += int64(output)
	}
	o.seen = true
}

func (o *callObserver) result() (Spend, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.spend, o.seen
}

var _ usage.Observer = (*callObserver)(nil)

// admit counts a request and asks the ledger whether it may proceed. It returns
// the receipt skeleton either way so a refusal is still recorded.
func (c *core) admit(ctx context.Context, req *Request) (Receipt, *AdmissionError) {
	purpose := PurposeFromContext(ctx)
	receipt := Receipt{
		Purpose:  purpose,
		Provider: c.cfg.Provider,
		Model:    req.Model,
		Method:   req.Method,
		Started:  time.Now(),
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

	decision := c.cfg.Ledger.Admit(purpose, count, RequiresExact(ctx))
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
func (c *core) settle(receipt Receipt, req *Request, obs *callObserver, reported *types.UsageMetadata, callErr error) {
	receipt.Duration = time.Since(receipt.Started)
	if callErr != nil {
		receipt.Err = callErr.Error()
	}

	actual, seen := obs.result()

	// The observer is the primary source because it fires on every provider and
	// every code path, including streaming, where the return value carries no
	// usage at all. A response-carried usage block is the fallback for clients
	// that do not report through the usage plumbing.
	if !seen && reported != nil {
		actual.InputTokens = int64(reported.InputTokens)
		actual.OutputTokens = int64(reported.OutputTokens)
	}
	// Cached and thinking counts are sub-classifications the observer channel
	// does not carry. They are metadata, already inside the input/output totals
	// above, so copying them here enriches the receipt without double counting.
	if reported != nil {
		actual.CachedTokens = int64(reported.CachedContentTokens)
		actual.ThinkingTokens = int64(reported.ThinkingTokens)
	}

	if actual.InputTokens > 0 || actual.OutputTokens > 0 {
		actual.Calls = 1
	}

	receipt.Actual = actual
	if actual.InputTokens > 0 {
		receipt.EstimateErrorPct = float64(receipt.Estimated.Tokens-int(actual.InputTokens)) /
			float64(actual.InputTokens) * 100
	}

	c.cfg.Ledger.Record(receipt.Purpose, actual)
	c.calibrate(req, actual)
	c.emit(receipt)
}

// calibrate feeds the provider's actual back into the counter so the next
// estimate is closer. This is the loop that makes the estimator different in
// kind from the constant it replaced.
func (c *core) calibrate(req *Request, actual Spend) {
	cal, ok := c.cfg.Counter.(CalibratingCounter)
	if !ok || req == nil {
		return
	}

	// Add cached tokens back before calibrating. A provider that excludes cache
	// reads from input_tokens makes a cache hit look like content that
	// tokenized ten times more densely than it did, and calibrating on that
	// corrupts the ratio with exactly the optimization that was working.
	input := actual.InputTokens + actual.CachedTokens
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
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "Complete",
		User:     prompt,
	}

	receipt, refusal := c.admit(ctx, req)
	if refusal != nil {
		c.settleRefusal(receipt)
		return "", refusal
	}

	ctx, obs := c.observed(ctx)
	out, err := c.underlying.Complete(ctx, prompt)
	c.settle(receipt, req, obs, nil, err)
	return out, err
}

// CompleteWithSystem implements types.LLMClient.
func (c *core) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "CompleteWithSystem",
		System:   systemPrompt,
		User:     userPrompt,
	}

	receipt, refusal := c.admit(ctx, req)
	if refusal != nil {
		c.settleRefusal(receipt)
		return "", refusal
	}

	ctx, obs := c.observed(ctx)
	out, err := c.underlying.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	c.settle(receipt, req, obs, nil, err)
	return out, err
}

// CompleteWithTools implements types.LLMClient.
func (c *core) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "CompleteWithTools",
		System:   systemPrompt,
		User:     userPrompt,
		Tools:    tools,
	}

	receipt, refusal := c.admit(ctx, req)
	if refusal != nil {
		c.settleRefusal(receipt)
		return nil, refusal
	}

	ctx, obs := c.observed(ctx)
	resp, err := c.underlying.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
	c.settle(receipt, req, obs, usageOf(resp), err)
	return resp, err
}

// CompleteWithStreaming implements types.LLMClient.
//
// Streaming settles late: the provider reports usage as the stream progresses
// and finishes only when it closes. The returned channels are proxied so the
// receipt is emitted when the turn actually ends rather than when it starts,
// which is also the only way a streamed turn's real cost is ever recorded.
func (c *core) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
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
		return closedStringChan(), errChanWith(refusal)
	}

	ctx, obs := c.observed(ctx)
	content, errs := c.underlying.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
	outContent, outErrs := c.proxyStream(ctx, receipt, req, obs, content, errs)
	return outContent, outErrs
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
