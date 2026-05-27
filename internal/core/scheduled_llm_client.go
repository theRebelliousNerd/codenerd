package core

import (
"context"
"fmt"
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
}

// Compile-time assertion that ScheduledLLMCall implements LLMClient
var _ LLMClient = (*ScheduledLLMCall)(nil)

// Complete makes an LLM call with cooperative scheduling (single prompt).
// Acquires a slot, makes the call, releases the slot.
func (c *ScheduledLLMCall) Complete(ctx context.Context, prompt string) (string, error) {
	// Acquire slot (blocks until available)
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return "", fmt.Errorf("failed to acquire API slot: %w", err)
	}

	// Always release the slot when done
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	// LLM I/O tracing: log the prompt before the call
	model := c.GetModel()
	logging.LogLLMRequest(c.ShardID, "", prompt, nil, model, 0)

	// Make the actual LLM call
	start := time.Now()
	result, err := c.Client.Complete(ctx, prompt)
	duration := time.Since(start)

	// LLM I/O tracing: log the response or error
	if err != nil {
		logging.LogLLMError(c.ShardID, err, duration)
	} else {
		logging.LogLLMResponse(c.ShardID, result, duration, len(result)/4)
	}

	return result, err
}

// CompleteWithSystem makes an LLM call with system prompt and cooperative scheduling.
// Acquires a slot, makes the call, releases the slot.
func (c *ScheduledLLMCall) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Acquire slot (blocks until available)
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return "", fmt.Errorf("failed to acquire API slot: %w", err)
	}

	// Always release the slot when done
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	// LLM I/O tracing: log the full prompt package before the call
	model := c.GetModel()
	logging.LogLLMRequest(c.ShardID, systemPrompt, userPrompt, nil, model, 0)

	// Make the actual LLM call
	start := time.Now()
	result, err := c.Client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	duration := time.Since(start)

	// LLM I/O tracing: log the response or error
	if err != nil {
		logging.LogLLMError(c.ShardID, err, duration)
	} else {
		logging.LogLLMResponse(c.ShardID, result, duration, len(result)/4)
	}

	return result, err
}

// CompleteWithSchema makes a scheduled LLM call with response schema enforcement.
func (c *ScheduledLLMCall) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
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

	// Make the actual LLM call
	start := time.Now()
	sc, ok := AsSchemaCapable(c.Client)
	if !ok {
		return "", ErrSchemaNotSupported
	}
	result, err := sc.CompleteWithSchema(ctx, systemPrompt, userPrompt, jsonSchema)
	duration := time.Since(start)

	// LLM I/O tracing: log the response or error
	if err != nil {
		logging.LogLLMError(c.ShardID+"-schema", err, duration)
	} else {
		logging.LogLLMResponse(c.ShardID+"-schema", result, duration, len(result)/4)
	}

	return result, err
}

// CompleteWithTools makes an LLM call with tools and cooperative scheduling.
// Acquires a slot, makes the call, releases the slot.
func (c *ScheduledLLMCall) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	// Acquire slot (blocks until available)
	if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
		return nil, fmt.Errorf("failed to acquire API slot: %w", err)
	}

	// Always release the slot when done
	defer c.Scheduler.ReleaseAPISlot(c.ShardID)

	// Make the actual LLM call with tools
	return c.Client.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
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
func (c *ScheduledLLMCall) GetCachedContent(ctx context.Context, cacheName string) (interface{}, error) {
	if p, ok := c.Client.(interface {
		GetCachedContent(ctx context.Context, cacheName string) (interface{}, error)
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
func (c *ScheduledLLMCall) GetFile(ctx context.Context, fileID string) (interface{}, error) {
	if p, ok := c.Client.(interface {
		GetFile(ctx context.Context, fileID string) (interface{}, error)
	}); ok {
		return p.GetFile(ctx, fileID)
	}
	return nil, fmt.Errorf("underlying client does not implement FileProvider")
}

type llmStreamingChannels interface {
	CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error)
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

// CompleteWithRetry makes an LLM call with retries and cooperative scheduling.
func (c *ScheduledLLMCall) CompleteWithRetry(ctx context.Context, systemPrompt, userPrompt string, maxRetries int) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Acquire slot for this attempt
		if err := c.Scheduler.AcquireAPISlot(ctx, c.ShardID); err != nil {
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
			return c.Client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		}()

		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if we should retry
		if attempt < maxRetries {
			// Brief pause before retry (exponential backoff)
			backoff := time.Duration(1<<attempt) * 100 * time.Millisecond
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}

			select {
			case <-ctx.Done():
				return "", ctx.Err()
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
	scheduler := GetAPIScheduler()

	// Register shard if not already registered
	if _, ok := scheduler.GetShardState(shardID); !ok {
		scheduler.RegisterShard(shardID, "unknown")
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

