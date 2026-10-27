package broker

import (
	"context"

	"codenerd/internal/types"
)

// The methods here back the optional interfaces some clients implement. They
// live on core so every wrapper shape in wrap.go can forward to one
// implementation, but core itself is never handed out as an LLMClient — if it
// were, every client would appear to support every optional interface, and the
// session executor would take the native tool-loop path against a provider that
// cannot serve it.

// completeWithToolResults meters a multi-turn tool conversation.
func (c *core) completeWithToolResults(
	ctx context.Context,
	systemPrompt string,
	history []types.Message,
	tools []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	if c.cfg.Router != nil {
		return routeCall(ctx, c.cfg.Router, func(ctx context.Context, client types.LLMClient) (*types.LLMToolResponse, error) {
			return client.(types.ToolResultsProvider).CompleteWithToolResults(ctx, systemPrompt, history, tools)
		})
	}
	provider, ok := c.underlying.(types.ToolResultsProvider)
	if !ok {
		// Unreachable through Wrap, which only builds a tool-results shape over
		// a client that implements the interface. Guarded anyway: a nil-pointer
		// panic inside a tool loop is a very expensive way to learn that an
		// invariant moved.
		return nil, ErrNoUnderlying
	}

	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "CompleteWithToolResults",
		System:   systemPrompt,
		Messages: history,
		Tools:    tools,
	}
	return compressing(c, ctx, req, false, usageOf, func(ctx context.Context, instruction string) (*types.LLMToolResponse, error) {
		return provider.CompleteWithToolResults(ctx, withInstruction(systemPrompt, instruction), history, tools)
	})
}

// completeWithSchema meters a structured-output completion.
func (c *core) completeWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	if c.cfg.Router != nil {
		return routeCall(ctx, c.cfg.Router, func(ctx context.Context, client types.LLMClient) (string, error) {
			return client.(interface {
				CompleteWithSchema(context.Context, string, string, string) (string, error)
			}).CompleteWithSchema(ctx, systemPrompt, userPrompt, jsonSchema)
		})
	}
	provider, ok := c.underlying.(interface {
		CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
	})
	if !ok {
		return "", ErrNoUnderlying
	}

	// The schema is transmitted and tokenized like any other content, so it is
	// counted. Charging only the prompts would under-report a structured call by
	// the size of its schema, which for a large tool contract is most of it.
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "CompleteWithSchema",
		System:   systemPrompt,
		User:     userPrompt + jsonSchema,
	}
	return compressing(c, ctx, req, false, nil, func(ctx context.Context, instruction string) (string, error) {
		return provider.CompleteWithSchema(ctx, withInstruction(systemPrompt, instruction), userPrompt, jsonSchema)
	})
}

// completeWithStreamingAndThoughts meters a streamed turn that also emits
// reasoning summaries on a second channel.
func (c *core) completeWithStreamingAndThoughts(
	ctx context.Context,
	systemPrompt, userPrompt string,
	enableThinking bool,
) (<-chan string, <-chan string, <-chan error) {
	if c.cfg.Router != nil {
		return c.cfg.Router.Stream(ctx, func(ctx context.Context, client types.LLMClient) (<-chan string, <-chan string, <-chan error) {
			return client.(interface {
				CompleteWithStreamingAndThoughts(context.Context, string, string, bool) (<-chan string, <-chan string, <-chan error)
			}).CompleteWithStreamingAndThoughts(ctx, systemPrompt, userPrompt, enableThinking)
		})
	}
	provider, ok := c.underlying.(interface {
		CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error)
	})
	if !ok {
		return closedStringChan(), closedStringChan(), errChanWith(ErrNoUnderlying)
	}

	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "CompleteWithStreamingAndThoughts",
		System:   systemPrompt,
		User:     userPrompt,
	}

	receipt, refusal := c.admit(ctx, req)
	if refusal != nil {
		c.settleRefusal(receipt)
		return closedStringChan(), closedStringChan(), errChanWith(refusal)
	}

	ctx, obs := c.observed(ctx)
	content, thoughts, errs := provider.CompleteWithStreamingAndThoughts(ctx, systemPrompt, userPrompt, enableThinking)

	// Thoughts are billed output. They are forwarded untouched and settle with
	// the content stream, so a thinking-heavy turn is not recorded as a cheap one.
	outContent, outErrs := c.proxyStream(ctx, receipt, req, obs, content, errs)
	return outContent, thoughts, outErrs
}

// SupportsGroundedWebSearch implements types.GroundedWebSearcher by asking the
// client underneath. Every wrapper shape carries the method, so the answer, not
// the method set, is what callers branch on; it is false for a client that
// cannot ground. Without it the wrapper hid the capability, and init's domain
// research reported "no grounded web search" for a Meta client that has it.
func (c *core) SupportsGroundedWebSearch() bool {
	searcher, ok := c.delegate().(types.GroundedWebSearcher)
	return ok && searcher.SupportsGroundedWebSearch()
}

// GroundedWebSearch implements types.GroundedWebSearcher: a metered call, and
// through the key-pool router when one selects the client, like every other
// completion.
func (c *core) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	if c.cfg.Router != nil {
		return routeCall(ctx, c.cfg.Router, func(ctx context.Context, client types.LLMClient) (*types.GroundedWebSearchResult, error) {
			searcher, ok := client.(types.GroundedWebSearcher)
			if !ok || !searcher.SupportsGroundedWebSearch() {
				return nil, ErrNoUnderlying
			}
			return searcher.GroundedWebSearch(ctx, query)
		})
	}
	searcher, ok := c.underlying.(types.GroundedWebSearcher)
	if !ok || !searcher.SupportsGroundedWebSearch() {
		return nil, ErrNoUnderlying
	}
	req := &Request{
		Provider: c.cfg.Provider,
		Model:    c.currentModel(),
		Method:   "GroundedWebSearch",
		User:     query,
	}
	return compressing(c, ctx, req, true, groundedUsage, func(ctx context.Context, instruction string) (*types.GroundedWebSearchResult, error) {
		return searcher.GroundedWebSearch(ctx, withInstruction(query, instruction))
	})
}

func groundedUsage(result *types.GroundedWebSearchResult) *types.UsageMetadata {
	if result == nil {
		return nil
	}
	return &types.UsageMetadata{
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		TotalTokens:  result.Usage.TotalTokens,
	}
}
