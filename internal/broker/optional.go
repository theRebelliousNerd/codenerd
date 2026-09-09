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

	receipt, refusal := c.admit(ctx, req)
	if refusal != nil {
		c.settleRefusal(receipt)
		return nil, refusal
	}

	ctx, obs := c.observed(ctx)
	resp, err := provider.CompleteWithToolResults(ctx, systemPrompt, history, tools)
	c.settle(receipt, req, obs, usageOf(resp), err)
	return resp, err
}

// completeWithSchema meters a structured-output completion.
func (c *core) completeWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
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

	receipt, refusal := c.admit(ctx, req)
	if refusal != nil {
		c.settleRefusal(receipt)
		return "", refusal
	}

	ctx, obs := c.observed(ctx)
	out, err := provider.CompleteWithSchema(ctx, systemPrompt, userPrompt, jsonSchema)
	c.settle(receipt, req, obs, nil, err)
	return out, err
}

// completeWithStreamingAndThoughts meters a streamed turn that also emits
// reasoning summaries on a second channel.
func (c *core) completeWithStreamingAndThoughts(
	ctx context.Context,
	systemPrompt, userPrompt string,
	enableThinking bool,
) (<-chan string, <-chan string, <-chan error) {
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
