package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// Scheduled LLM Call Wrapper
// -----------------------------------------------------------------------------

// ScheduledLLMCall wraps an LLM call with slot acquisition/release.
// This is the primary integration point for shards making API calls.
// Implements LLMClient interface so it can be injected transparently.
type ScheduledLLMCall struct {
	Scheduler *APIScheduler
	ShardID   string
	Client    LLMClient

	// Router builds the client for a call whose context names a provider
	// (types.WithProvider, set from a shard profile's provider). Nil means
	// this client routes nowhere, and such a call is refused.
	Router  ProviderRouter
	routeMu sync.Mutex
	routed  map[string]LLMClient
}

// ProviderRouter builds the client that serves model on provider. It is called
// once per provider and model; the result is kept.
type ProviderRouter func(provider, model string) (LLMClient, error)

// clientFor is the client a call under ctx runs on: the wrapped one, unless ctx
// names a provider, in which case it is that provider's. A route that cannot be
// built fails the call. It never falls back to the wrapped client: a shard
// configured for one vendor and silently run on another is the defect this
// exists to end (2026-09-21: every shard profile named an OpenRouter model under
// provider "meta", and the Meta client rewrote the model on every call).
func (c *ScheduledLLMCall) clientFor(ctx context.Context) (LLMClient, error) {
	provider, ok := types.ProviderFromContext(ctx)
	if !ok {
		return c.Client, nil
	}
	model, _ := types.ModelNameFromContext(ctx)
	if c.Router == nil {
		return nil, fmt.Errorf("the call is routed to provider %q (model %q) and this client has no provider router", provider, model)
	}
	key := provider + "|" + model
	c.routeMu.Lock()
	defer c.routeMu.Unlock()
	if client, ok := c.routed[key]; ok {
		return client, nil
	}
	client, err := c.Router(provider, model)
	if err != nil {
		return nil, fmt.Errorf("route to provider %q (model %q): %w", provider, model, err)
	}
	if client == nil {
		return nil, fmt.Errorf("route to provider %q (model %q): no client was built", provider, model)
	}
	if c.routed == nil {
		c.routed = make(map[string]LLMClient)
	}
	c.routed[key] = client
	return client, nil
}

// Compile-time assertion that ScheduledLLMCall implements LLMClient
var _ LLMClient = (*ScheduledLLMCall)(nil)

// Unwrap exposes the wrapped client so broker.Base and broker.IsBrokered can
// walk the decorator chain.
//
// Without it this type is opaque: Base stops here and returns the scheduler
// instead of the concrete client, and IsBrokered reports an already-metered
// chain as un-metered. Neither shows up as an error -- Base's callers use a
// comma-ok type assertion, so a miss reads as "not that engine".
func (c *ScheduledLLMCall) Unwrap() LLMClient { return c.Client }

// Complete makes an LLM call with cooperative scheduling (single prompt).
// Acquires a slot, makes the call, releases the slot.
func (c *ScheduledLLMCall) Complete(ctx context.Context, prompt string) (string, error) {
	if c.Scheduler == nil {
		return "", fmt.Errorf("scheduler not configured on ScheduledLLMCall")
	}
	if c.Client == nil {
		return "", fmt.Errorf("underlying LLM client is nil")
	}
	target, routeErr := c.clientFor(ctx)
	if routeErr != nil {
		return "", routeErr
	}

	// Acquire slot (blocks until available)
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return "", fmt.Errorf("failed to acquire API slot: %w", err)
	}

	// Always release the slot when done
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	// LLM I/O tracing: log the prompt before the call
	model := c.GetModel()
	logging.LogLLMRequest(c.ShardID, "", prompt, nil, model, 0)

	// Make the actual LLM call with panic recovery
	var result string
	var err error
	start := time.Now()
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic during LLM call: %v", r)
			}
		}()
		result, err = target.Complete(ctx, prompt)
	}()
	duration := time.Since(start)

	// LLM I/O tracing: log the response or error
	if err != nil {
		logging.LogLLMError(c.ShardID, err, duration)
		if isRateLimitErr(err) {
			c.Scheduler.ReportRateLimit()
		}
	} else {
		logging.LogLLMResponse(c.ShardID, result, duration, len(result)/4)
		c.Scheduler.ReportSuccess()
	}

	return result, err
}

// CompleteWithSystem makes an LLM call with system prompt and cooperative scheduling.
// Acquires a slot, makes the call, releases the slot.
func (c *ScheduledLLMCall) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.Scheduler == nil {
		return "", fmt.Errorf("scheduler not configured on ScheduledLLMCall")
	}
	if c.Client == nil {
		return "", fmt.Errorf("underlying LLM client is nil")
	}
	target, routeErr := c.clientFor(ctx)
	if routeErr != nil {
		return "", routeErr
	}

	// Acquire slot (blocks until available)
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return "", fmt.Errorf("failed to acquire API slot: %w", err)
	}

	// Always release the slot when done
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	// LLM I/O tracing: log the full prompt package before the call
	model := c.GetModel()
	logging.LogLLMRequest(c.ShardID, systemPrompt, userPrompt, nil, model, 0)

	// Make the actual LLM call with panic recovery
	var result string
	var err error
	start := time.Now()
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic during LLM call: %v", r)
			}
		}()
		result, err = target.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	}()
	duration := time.Since(start)

	// LLM I/O tracing: log the response or error
	if err != nil {
		logging.LogLLMError(c.ShardID, err, duration)
		if isRateLimitErr(err) {
			c.Scheduler.ReportRateLimit()
		}
	} else {
		logging.LogLLMResponse(c.ShardID, result, duration, len(result)/4)
		c.Scheduler.ReportSuccess()
	}

	return result, err
}

// isRateLimitErr detects provider 429 / rate-limit errors without importing
// every client package (string match is enough for adaptive concurrency).
func isRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "resource_exhausted")
}

// CompleteWithSchema makes a scheduled LLM call with response schema enforcement.
func (c *ScheduledLLMCall) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	if c.Scheduler == nil {
		return "", fmt.Errorf("scheduler not configured on ScheduledLLMCall")
	}
	if c.Client == nil {
		return "", fmt.Errorf("underlying LLM client is nil")
	}
	target, routeErr := c.clientFor(ctx)
	if routeErr != nil {
		return "", routeErr
	}

	// Acquire slot (blocks until available)
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return "", fmt.Errorf("failed to acquire API slot: %w", err)
	}

	// Always release the slot when done
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	// LLM I/O tracing: log the full prompt package before the call
	model := c.GetModel()
	schemaNote := fmt.Sprintf("[SCHEMA-CONSTRAINED, schema=%d chars]", len(jsonSchema))
	logging.LogLLMRequest(c.ShardID+"-schema", systemPrompt, userPrompt+"\n"+schemaNote, nil, model, 0)

	// Make the actual LLM call with panic recovery
	var result string
	var err error
	start := time.Now()
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic during LLM call: %v", r)
			}
		}()
		sc, ok := AsSchemaCapable(target)
		if !ok {
			err = ErrSchemaNotSupported
			return
		}
		result, err = sc.CompleteWithSchema(ctx, systemPrompt, userPrompt, jsonSchema)
	}()
	duration := time.Since(start)

	// LLM I/O tracing: log the response or error
	if err != nil {
		logging.LogLLMError(c.ShardID+"-schema", err, duration)
		if isRateLimitErr(err) {
			c.Scheduler.ReportRateLimit()
		}
	} else {
		logging.LogLLMResponse(c.ShardID+"-schema", result, duration, len(result)/4)
		c.Scheduler.ReportSuccess()
	}

	return result, err
}

// CompleteWithTools makes an LLM call with tools and cooperative scheduling.
// Acquires a slot, makes the call, releases the slot.
func (c *ScheduledLLMCall) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if c.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured on ScheduledLLMCall")
	}
	if c.Client == nil {
		return nil, fmt.Errorf("underlying LLM client is nil")
	}
	target, routeErr := c.clientFor(ctx)
	if routeErr != nil {
		return nil, routeErr
	}

	// Acquire slot (blocks until available)
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return nil, fmt.Errorf("failed to acquire API slot: %w", err)
	}

	// Always release the slot when done
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	// LLM I/O tracing: log the request, including a summary of the tools
	// offered to the model.
	model := c.GetModel()
	logging.LogLLMRequest(c.ShardID+"-tools", systemPrompt, userPrompt+"\n"+traceToolCatalog(tools), nil, model, 0)

	// Make the actual LLM call with panic recovery
	var resp *types.LLMToolResponse
	var err error
	start := time.Now()
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic during LLM call: %v", r)
			}
		}()
		resp, err = target.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
	}()
	duration := time.Since(start)

	// LLM I/O tracing: log the response or error
	if err != nil {
		logging.LogLLMError(c.ShardID+"-tools", err, duration)
	} else if resp != nil {
		summary := resp.Text
		if len(resp.ToolCalls) > 0 {
			callNames := make([]string, 0, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				callNames = append(callNames, tc.Name)
			}
			summary += fmt.Sprintf("\n[TOOL_CALLS, count=%d, names=%v, stop=%s]", len(resp.ToolCalls), callNames, resp.StopReason)
		}
		logging.LogLLMResponse(c.ShardID+"-tools", summary, duration, len(summary)/4)
	}

	return resp, err
}

// CompleteWithToolResults continues a multi-turn tool conversation under the
// same slot accounting as CompleteWithTools. Required so the session executor
// can type-assert ToolResultsProvider through the scheduler wrapper.
func (c *ScheduledLLMCall) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if c.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured on ScheduledLLMCall")
	}
	if c.Client == nil {
		return nil, fmt.Errorf("underlying LLM client is nil")
	}
	target, routeErr := c.clientFor(ctx)
	if routeErr != nil {
		return nil, routeErr
	}
	trp, ok := target.(types.ToolResultsProvider)
	if !ok {
		return nil, fmt.Errorf("LLM client %T does not implement ToolResultsProvider", target)
	}

	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return nil, fmt.Errorf("failed to acquire API slot: %w", err)
	}
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	model := c.GetModel()
	// The transcript is the request. Tracing only its length left every
	// tool-loop round in llm_io.log as "[TOOL_RESULTS history_turns=3]",
	// so what the model actually saw of its tool results, and whether the
	// orchestrator's steering reached it, could not be read back from the
	// trace of a stalled run.
	logging.LogLLMRequest(c.ShardID+"-tool-results", systemPrompt,
		fmt.Sprintf("[TOOL_RESULTS history_turns=%d]\n%s", len(history), traceToolCatalog(tools)), traceMessages(history), model, 0)

	var resp *types.LLMToolResponse
	var err error
	start := time.Now()
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic during LLM tool-results call: %v", r)
			}
		}()
		resp, err = trp.CompleteWithToolResults(ctx, systemPrompt, history, tools)
	}()
	duration := time.Since(start)
	if err != nil {
		logging.LogLLMError(c.ShardID+"-tool-results", err, duration)
		if isRateLimitErr(err) {
			c.Scheduler.ReportRateLimit()
		}
	} else if resp != nil {
		summary := resp.Text
		if len(resp.ToolCalls) > 0 {
			summary += fmt.Sprintf("\n[TOOL_CALLS, count=%d, stop=%s]", len(resp.ToolCalls), resp.StopReason)
		}
		logging.LogLLMResponse(c.ShardID+"-tool-results", summary, duration, len(summary)/4)
		c.Scheduler.ReportSuccess()
	}
	return resp, err
}

type tracingContextSetter interface {
	SetShardContext(shardID, shardType, shardCategory, sessionID, taskContext string)
	ClearShardContext()
}

type semaphoreDisabler interface {
	DisableSemaphore()
}

// SetShardContext forwards tracing context into the wrapped client, if supported.
// This enables accurate attribution even when clients are wrapped by the scheduler.
func (c *ScheduledLLMCall) SetShardContext(shardID, shardType, shardCategory, sessionID, taskContext string) {
	if tc, ok := c.Client.(tracingContextSetter); ok {
		tc.SetShardContext(shardID, shardType, shardCategory, sessionID, taskContext)
	}
}

// ClearShardContext forwards tracing context clearing into the wrapped client, if supported.
func (c *ScheduledLLMCall) ClearShardContext() {
	if tc, ok := c.Client.(tracingContextSetter); ok {
		tc.ClearShardContext()
	}
}

// SchemaCapable reports whether the wrapped client supports schema enforcement.
func (c *ScheduledLLMCall) SchemaCapable() bool {
	if checker, ok := c.Client.(interface{ SchemaCapable() bool }); ok {
		return checker.SchemaCapable()
	}
	_, ok := c.Client.(SchemaCapableLLMClient)
	return ok
}

// =============================================================================
// ThinkingProvider Interface Pass-Through
// =============================================================================
// These methods allow the ScheduledLLMCall to be transparent to thinking mode
// detection, so that GeminiThinkingTransducer can be properly selected.

// IsThinkingEnabled delegates to the underlying client if it supports thinking.
func (c *ScheduledLLMCall) IsThinkingEnabled() bool {
	if p, ok := c.Client.(interface{ IsThinkingEnabled() bool }); ok {
		return p.IsThinkingEnabled()
	}
	return false
}

// GetThinkingLevel delegates to the underlying client if it supports thinking.
func (c *ScheduledLLMCall) GetThinkingLevel() string {
	if p, ok := c.Client.(interface{ GetThinkingLevel() string }); ok {
		return p.GetThinkingLevel()
	}
	return ""
}

// GetLastThoughtSummary delegates to the underlying client if it supports thinking.
func (c *ScheduledLLMCall) GetLastThoughtSummary() string {
	if p, ok := c.Client.(interface{ GetLastThoughtSummary() string }); ok {
		return p.GetLastThoughtSummary()
	}
	return ""
}

// GetLastThinkingTokens delegates to the underlying client if it supports thinking.
func (c *ScheduledLLMCall) GetLastThinkingTokens() int {
	if p, ok := c.Client.(interface{ GetLastThinkingTokens() int }); ok {
		return p.GetLastThinkingTokens()
	}
	return 0
}

// =============================================================================
// ThoughtSignatureProvider Interface Pass-Through
// =============================================================================

// GetLastThoughtSignature delegates to the underlying client for multi-turn function calling.
func (c *ScheduledLLMCall) GetLastThoughtSignature() string {
	if p, ok := c.Client.(interface{ GetLastThoughtSignature() string }); ok {
		return p.GetLastThoughtSignature()
	}
	return ""
}

// =============================================================================
// GroundingProvider Interface Pass-Through
// =============================================================================

// GetLastGroundingSources delegates to the underlying client.
func (c *ScheduledLLMCall) GetLastGroundingSources() []string {
	if p, ok := c.Client.(interface{ GetLastGroundingSources() []string }); ok {
		return p.GetLastGroundingSources()
	}
	return nil
}

// IsGoogleSearchEnabled delegates to the underlying client.
func (c *ScheduledLLMCall) IsGoogleSearchEnabled() bool {
	if p, ok := c.Client.(interface{ IsGoogleSearchEnabled() bool }); ok {
		return p.IsGoogleSearchEnabled()
	}
	return false
}

// IsURLContextEnabled delegates to the underlying client.
func (c *ScheduledLLMCall) IsURLContextEnabled() bool {
	if p, ok := c.Client.(interface{ IsURLContextEnabled() bool }); ok {
		return p.IsURLContextEnabled()
	}
	return false
}

// =============================================================================
// CacheProvider Interface Pass-Through
// =============================================================================

// CreateCachedContent delegates to the underlying client.
func (c *ScheduledLLMCall) CreateCachedContent(ctx context.Context, files []string, ttl int) (string, error) {
	if p, ok := c.Client.(interface {
		CreateCachedContent(ctx context.Context, files []string, ttl int) (string, error)
	}); ok {
		return p.CreateCachedContent(ctx, files, ttl)
	}
	return "", fmt.Errorf("underlying client does not implement CacheProvider")
}

// GetCachedContent delegates to the underlying client.
func (c *ScheduledLLMCall) GetCachedContent(ctx context.Context, cacheName string) (any, error) {
	if p, ok := c.Client.(interface {
		GetCachedContent(ctx context.Context, cacheName string) (any, error)
	}); ok {
		return p.GetCachedContent(ctx, cacheName)
	}
	return nil, fmt.Errorf("underlying client does not implement CacheProvider")
}

// DeleteCachedContent delegates to the underlying client.
func (c *ScheduledLLMCall) DeleteCachedContent(ctx context.Context, cacheName string) error {
	if p, ok := c.Client.(interface {
		DeleteCachedContent(ctx context.Context, cacheName string) error
	}); ok {
		return p.DeleteCachedContent(ctx, cacheName)
	}
	return fmt.Errorf("underlying client does not implement CacheProvider")
}

// ListCachedContent delegates to the underlying client.
func (c *ScheduledLLMCall) ListCachedContent(ctx context.Context) ([]string, error) {
	if p, ok := c.Client.(interface {
		ListCachedContent(ctx context.Context) ([]string, error)
	}); ok {
		return p.ListCachedContent(ctx)
	}
	return nil, fmt.Errorf("underlying client does not implement CacheProvider")
}

// SetCachedContent delegates to the underlying client.
func (c *ScheduledLLMCall) SetCachedContent(name string) {
	if p, ok := c.Client.(interface{ SetCachedContent(string) }); ok {
		p.SetCachedContent(name)
	}
}

// =============================================================================
// FileProvider Interface Pass-Through
// =============================================================================

// UploadFile delegates to the underlying client.
func (c *ScheduledLLMCall) UploadFile(ctx context.Context, path string, mimeType string) (string, error) {
	if p, ok := c.Client.(interface {
		UploadFile(ctx context.Context, path string, mimeType string) (string, error)
	}); ok {
		return p.UploadFile(ctx, path, mimeType)
	}
	return "", fmt.Errorf("underlying client does not implement FileProvider")
}

// DeleteFile delegates to the underlying client.
func (c *ScheduledLLMCall) DeleteFile(ctx context.Context, fileID string) error {
	if p, ok := c.Client.(interface {
		DeleteFile(ctx context.Context, fileID string) error
	}); ok {
		return p.DeleteFile(ctx, fileID)
	}
	return fmt.Errorf("underlying client does not implement FileProvider")
}

// ListFiles delegates to the underlying client.
func (c *ScheduledLLMCall) ListFiles(ctx context.Context) ([]string, error) {
	if p, ok := c.Client.(interface {
		ListFiles(ctx context.Context) ([]string, error)
	}); ok {
		return p.ListFiles(ctx)
	}
	return nil, fmt.Errorf("underlying client does not implement FileProvider")
}

// GetFile delegates to the underlying client.
func (c *ScheduledLLMCall) GetFile(ctx context.Context, fileID string) (any, error) {
	if p, ok := c.Client.(interface {
		GetFile(ctx context.Context, fileID string) (any, error)
	}); ok {
		return p.GetFile(ctx, fileID)
	}
	return nil, fmt.Errorf("underlying client does not implement FileProvider")
}

type llmStreamingChannels interface {
	CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error)
}

// LLMStreamingWithThoughts is the opt-in 3-channel streaming surface for
// clients that can expose the model's thinking trace alongside visible
// output. Implementing this is OPTIONAL — callers must type-assert and
// fall back to the 2-channel CompleteWithStreaming when not supported.
//
// The contract: content and thoughts are streamed in arrival order on
// their respective channels; both channels (and the error channel) are
// closed when the stream ends. The thoughts channel may be empty for the
// entire stream (e.g. if thinking is disabled or the model produced no
// thinking content); callers must handle a closed-without-data thought
// stream gracefully.
//
// Currently implemented by: GeminiClient (Gemini 3.x with thinking mode).
type LLMStreamingWithThoughts interface {
	CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (content <-chan string, thoughts <-chan string, err <-chan error)
}

// CompleteWithStreaming makes a scheduled streaming LLM call.
// The API slot is held for the duration of the stream and released when the stream ends.
func (c *ScheduledLLMCall) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	// Acquire slot (blocks until available)
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		close(contentChan)
		errorChan <- fmt.Errorf("failed to acquire API slot: %w", err)
		close(errorChan)
		return contentChan, errorChan
	}

	streamer, ok := c.Client.(llmStreamingChannels)
	if !ok {
		c.Scheduler.ReleaseAPISlot(c.ShardID)
		close(contentChan)
		errorChan <- ErrStreamingNotSupported
		close(errorChan)
		return contentChan, errorChan
	}

	// LLM I/O tracing: log the full prompt package before the call
	model := c.GetModel()
	logging.LogLLMRequest(c.ShardID, systemPrompt, userPrompt, nil, model, 0)

	start := time.Now()
	underContent, underErr := streamer.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)

	go func() {
		defer c.Scheduler.ReleaseAPISlot(c.ShardID)
		defer close(contentChan)
		defer close(errorChan)

		// Nil channels block forever on receive in Go. Treat them as immediately closed.
		contentClosed := underContent == nil
		errClosed := underErr == nil
		var firstErr error
		var fullResponse string

		for !(contentClosed && errClosed) {
			select {
			case <-ctx.Done():
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				// Context cancelled — stop forwarding immediately.
				// The upstream goroutine is responsible for its own cleanup.
				contentClosed = true
				errClosed = true
			case chunk, ok := <-underContent:
				if !ok {
					contentClosed = true
					continue
				}
				select {
				case contentChan <- chunk:
					fullResponse += chunk
				case <-ctx.Done():
					if firstErr == nil {
						firstErr = ctx.Err()
					}
					contentClosed = true
					errClosed = true
				}
			case err, ok := <-underErr:
				if !ok {
					errClosed = true
					continue
				}
				if err != nil && firstErr == nil {
					firstErr = err
				}
			}
		}

		duration := time.Since(start)
		if firstErr != nil {
			logging.LogLLMError(c.ShardID, firstErr, duration)
			errorChan <- firstErr
		} else {
			logging.LogLLMResponse(c.ShardID, fullResponse, duration, len(fullResponse)/4)
		}
	}()

	return contentChan, errorChan
}

// CompleteWithStreamingAndThoughts makes a scheduled streaming LLM call
// and forwards the model's thinking trace on a separate channel. If the
// underlying client doesn't implement LLMStreamingWithThoughts, falls
// back transparently to CompleteWithStreaming with an already-closed
// (empty) thoughts channel — so callers can use the same code path
// regardless of which provider is configured.
//
// The API slot is held for the lifetime of the stream and released when
// it ends (either both content+thoughts close or the error channel
// receives).
func (c *ScheduledLLMCall) CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	thoughtsChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	// Acquire slot before starting (releases below in deferred close goroutine).
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		close(contentChan)
		close(thoughtsChan)
		errorChan <- fmt.Errorf("failed to acquire API slot: %w", err)
		close(errorChan)
		return contentChan, thoughtsChan, errorChan
	}

	thoughtsStreamer, supportsThoughts := c.Client.(LLMStreamingWithThoughts)
	streamer, supportsStreaming := c.Client.(llmStreamingChannels)
	if !supportsThoughts && !supportsStreaming {
		c.Scheduler.ReleaseAPISlot(c.ShardID)
		close(contentChan)
		close(thoughtsChan)
		errorChan <- ErrStreamingNotSupported
		close(errorChan)
		return contentChan, thoughtsChan, errorChan
	}

	model := c.GetModel()
	logging.LogLLMRequest(c.ShardID, systemPrompt, userPrompt, nil, model, 0)

	start := time.Now()

	// Resolve underlying channels: either native 3-channel from a
	// thoughts-capable client, or a 2-channel stream with an
	// immediately-closed thoughts side.
	var underContent, underThoughts <-chan string
	var underErr <-chan error
	if supportsThoughts {
		underContent, underThoughts, underErr = thoughtsStreamer.CompleteWithStreamingAndThoughts(ctx, systemPrompt, userPrompt, enableThinking)
	} else {
		underContent, underErr = streamer.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
		closedThoughts := make(chan string)
		close(closedThoughts)
		underThoughts = closedThoughts
	}

	go func() {
		defer c.Scheduler.ReleaseAPISlot(c.ShardID)
		defer close(contentChan)
		defer close(thoughtsChan)
		defer close(errorChan)

		contentClosed := underContent == nil
		thoughtsClosed := underThoughts == nil
		errClosed := underErr == nil
		var firstErr error
		var fullResponse string

		for !(contentClosed && thoughtsClosed && errClosed) {
			select {
			case <-ctx.Done():
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				contentClosed = true
				thoughtsClosed = true
				errClosed = true
			case chunk, ok := <-underContent:
				if !ok {
					contentClosed = true
					continue
				}
				select {
				case contentChan <- chunk:
					fullResponse += chunk
				case <-ctx.Done():
					if firstErr == nil {
						firstErr = ctx.Err()
					}
					contentClosed = true
					thoughtsClosed = true
					errClosed = true
				}
			case chunk, ok := <-underThoughts:
				if !ok {
					thoughtsClosed = true
					continue
				}
				select {
				case thoughtsChan <- chunk:
				case <-ctx.Done():
					if firstErr == nil {
						firstErr = ctx.Err()
					}
					contentClosed = true
					thoughtsClosed = true
					errClosed = true
				}
			case err, ok := <-underErr:
				if !ok {
					errClosed = true
					continue
				}
				if err != nil && firstErr == nil {
					firstErr = err
				}
			}
		}

		duration := time.Since(start)
		if firstErr != nil {
			logging.LogLLMError(c.ShardID, firstErr, duration)
			errorChan <- firstErr
		} else {
			logging.LogLLMResponse(c.ShardID, fullResponse, duration, len(fullResponse)/4)
		}
	}()

	return contentChan, thoughtsChan, errorChan
}

// CompleteWithRetry makes an LLM call with retries and cooperative scheduling.
func (c *ScheduledLLMCall) CompleteWithRetry(ctx context.Context, systemPrompt, userPrompt string, maxRetries int) (string, error) {
	if c.Scheduler == nil {
		return "", fmt.Errorf("scheduler not configured on ScheduledLLMCall")
	}
	if c.Client == nil {
		return "", fmt.Errorf("underlying LLM client is nil")
	}
	target, routeErr := c.clientFor(ctx)
	if routeErr != nil {
		return "", routeErr
	}

	// Enforce absolute max retry cap and time budget to prevent infinite starvation
	if maxRetries > 5 {
		maxRetries = 5
	}
	budgetCtx, budgetCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer budgetCancel()

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Acquire slot for this attempt
		if err := c.Scheduler.AcquireAPISlot(budgetCtx, c.ShardID); err != nil {
			return "", fmt.Errorf("failed to acquire API slot (attempt %d): %w", attempt+1, err)
		}

		// Make the call and guarantee slot release even on panic
		result, err := func() (res string, callErr error) {
			defer func() {
				if r := recover(); r != nil {
					callErr = fmt.Errorf("panic during LLM call: %v", r)
				}
				c.Scheduler.ReleaseAPISlot(c.ShardID)
			}()
			return target.CompleteWithSystem(budgetCtx, systemPrompt, userPrompt)
		}()

		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if we should retry
		if attempt < maxRetries {
			// Brief pause before retry (exponential backoff)
			backoff := min(time.Duration(1<<attempt)*100*time.Millisecond, 5*time.Second)

			select {
			case <-budgetCtx.Done():
				return "", budgetCtx.Err()
			case <-time.After(backoff):
				logging.ShardsDebug("ScheduledLLMCall: retrying after error (attempt %d/%d): %v",
					attempt+1, maxRetries, err)
			}
		}
	}

	return "", fmt.Errorf("all %d attempts failed, last error: %w", maxRetries+1, lastErr)
}

// -----------------------------------------------------------------------------

func NewScheduledLLMCall(shardID string, client LLMClient) *ScheduledLLMCall {
	return NewScheduledLLMCallWithPriority(shardID, client, types.PriorityNormal)
}

// NewScheduledLLMCallWithPriority creates a scheduled wrapper whose calls
// default to the given slot priority. Use PriorityHigh for interactive
// clients (the chat turn's perception/articulation path) so a user waiting
// on a response is never queued behind background shard work.
func NewScheduledLLMCallWithPriority(shardID string, client LLMClient, priority types.SpawnPriority) *ScheduledLLMCall {
	scheduler := GetAPIScheduler()

	// Register shard if not already registered
	if _, ok := scheduler.GetShardState(shardID); !ok {
		scheduler.RegisterShardWithPriority(shardID, "unknown", priority)
	}

	if disabler, ok := client.(semaphoreDisabler); ok {
		disabler.DisableSemaphore()
	}

	return &ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   shardID,
		Client:    client,
	}
}

// SetModel changes the model used for completions on the underlying client.
func (c *ScheduledLLMCall) SetModel(model string) {
	if setter, ok := c.Client.(interface{ SetModel(string) }); ok {
		setter.SetModel(model)
	}
}

// GetModel returns the model of the underlying client.
func (c *ScheduledLLMCall) GetModel() string {
	if getter, ok := c.Client.(interface{ GetModel() string }); ok {
		return getter.GetModel()
	}
	return ""
}

var _ types.PiggybackToolProvider = (*ScheduledLLMCall)(nil)

// ShouldUsePiggybackTools forwards the optional PiggybackToolProvider
// capability so the claim survives the scheduling boundary. Without this the
// executor takes the native function-calling path for a wrapped client that
// requires Piggyback (a Gemini client with grounding enabled), which conflicts
// with the provider's built-in tools. A nil or non-piggyback inner client
// stays native.
func (c *ScheduledLLMCall) ShouldUsePiggybackTools() bool {
	if c == nil || c.Client == nil {
		return false
	}
	if ptp, ok := c.Client.(types.PiggybackToolProvider); ok {
		return ptp.ShouldUsePiggybackTools()
	}
	return false
}

// =============================================================================
// GroundedWebSearcher Pass-Through (scheduled, slot-accounted)
// =============================================================================

var _ types.GroundedWebSearcher = (*ScheduledLLMCall)(nil)

// SupportsGroundedWebSearch forwards capability detection.
func (c *ScheduledLLMCall) SupportsGroundedWebSearch() bool {
	if c == nil || c.Client == nil {
		return false
	}
	if gws, ok := c.Client.(types.GroundedWebSearcher); ok {
		return gws.SupportsGroundedWebSearch()
	}
	return false
}

// GroundedWebSearch performs a grounded web search under scheduler slot accounting.
// It acquires and releases a scheduler slot, reports rate limits and successes, and
// preserves context cancellation. It never exposes reasoning traces or API keys,
// delegating sanitization to the underlying provider.
func (c *ScheduledLLMCall) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	if c == nil {
		return nil, fmt.Errorf("scheduled call is nil")
	}
	if c.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured on ScheduledLLMCall")
	}
	if c.Client == nil {
		return nil, fmt.Errorf("underlying LLM client is nil")
	}
	gws, ok := c.Client.(types.GroundedWebSearcher)
	if !ok {
		return nil, fmt.Errorf("LLM client %T does not implement GroundedWebSearcher", c.Client)
	}
	if !gws.SupportsGroundedWebSearch() {
		return nil, fmt.Errorf("grounded web search is not supported by the configured provider")
	}

	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return nil, fmt.Errorf("failed to acquire API slot: %w", err)
	}
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	model := c.GetModel()
	logging.LogLLMRequest(c.ShardID+"-grounded", "", query, nil, model, 0)

	var result *types.GroundedWebSearchResult
	var callErr error
	start := time.Now()
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic during grounded search: %v", r)
			}
		}()
		result, callErr = gws.GroundedWebSearch(ctx, query)
	}()
	duration := time.Since(start)

	if callErr != nil {
		logging.LogLLMError(c.ShardID+"-grounded", callErr, duration)
		if isRateLimitErr(callErr) {
			c.Scheduler.ReportRateLimit()
		}
	} else if result == nil {
		callErr = fmt.Errorf("grounded_web_search: empty result")
		logging.LogLLMError(c.ShardID+"-grounded", callErr, duration)
	} else {
		summary := result.Text
		// Keep logging bounded; never log raw provider messages beyond length.
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		logging.LogLLMResponse(c.ShardID+"-grounded", summary, duration, len(summary)/4)
		c.Scheduler.ReportSuccess()
	}
	return result, callErr
}

// traceToolCatalog renders the tool schemas a request offers, whole, for the
// LLM I/O trace. The catalog is ~10% of a tool-loop request and is first on
// the wire; the trace used to log only a count (or the names), so the context
// a model was given could not be read back from it and had to be
// reconstructed from broker estimates (context sweep 2026-09-22, CTX-B5). The
// catalog id is a digest of the schemas: equal ids, equal catalogs.
func traceToolCatalog(tools []types.ToolDefinition) string {
	schemas, err := json.Marshal(tools)
	if err != nil {
		return fmt.Sprintf("[TOOLS count=%d, schemas unencodable: %v]", len(tools), err)
	}
	sum := sha256.Sum256(schemas)
	return fmt.Sprintf("[TOOLS count=%d catalog=%s]\n%s", len(tools), hex.EncodeToString(sum[:8]), schemas)
}

// traceMessages renders a tool-loop transcript for the LLM I/O trace, block by
// block in the order the provider receives them: a turn's replayed reasoning,
// its text, its tool calls with their arguments, its tool results whole.
// Whole, because the orchestrator's steering is appended to the end of a
// round's last result, and a trace that cut results short is exactly the trace
// that could not show whether the model was ever told. The reasoning is
// replayed on every round (encrypted on the Meta Responses surface, where the
// signature is all there is), and it was left out, so ~3% of every request was
// invisible in the trace.
func traceMessages(history []types.Message) []logging.LLMMessage {
	out := make([]logging.LLMMessage, 0, len(history))
	for _, m := range history {
		var sb strings.Builder
		for _, b := range m.Content() {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			switch b.Kind {
			case types.BlockThinking:
				fmt.Fprintf(&sb, "[reasoning id=%s redacted=%t tokens=%d signature_chars=%d]", b.ID, b.Redacted, b.Tokens, len(b.Signature))
				if b.Text != "" {
					sb.WriteString("\n")
					sb.WriteString(b.Text)
				}
				if b.Signature != "" {
					sb.WriteString("\n[signature] ")
					sb.WriteString(b.Signature)
				}
			case types.BlockToolUse:
				args, err := json.Marshal(b.Input)
				if err != nil {
					args = []byte(fmt.Sprintf("%v", b.Input))
				}
				fmt.Fprintf(&sb, "[tool_use id=%s name=%s] %s", b.ID, b.Name, args)
			case types.BlockToolResult:
				fmt.Fprintf(&sb, "[tool_result id=%s error=%t]\n%s", b.ToolUseID, b.IsError, b.Text)
			default:
				sb.WriteString(b.Text)
			}
		}
		out = append(out, logging.LLMMessage{Role: m.Role, Content: sb.String()})
	}
	return out
}
