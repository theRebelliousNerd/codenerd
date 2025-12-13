package perception

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// ReasoningTrace captures a complete LLM interaction for learning and analysis.
type ReasoningTrace struct {
	ID            string `json:"id"`
	ShardID       string `json:"shard_id"`
	ShardType     string `json:"shard_type"`
	ShardCategory string `json:"shard_category"` // system, ephemeral, specialist
	SessionID     string `json:"session_id"`
	TaskContext   string `json:"task_context"`

	// LLM Interaction
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"`
	Response     string `json:"response"`

	// Metadata
	Model      string `json:"model,omitempty"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	DurationMs int64  `json:"duration_ms"`

	// Outcome (filled after shard completes)
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Learning metadata (filled after analysis)
	QualityScore  float64  `json:"quality_score,omitempty"`
	LearningNotes []string `json:"learning_notes,omitempty"`

	Timestamp time.Time `json:"timestamp"`
}

// TraceStore defines the interface for storing reasoning traces.
// This abstraction allows different storage backends.
type TraceStore interface {
	StoreReasoningTrace(trace *ReasoningTrace) error
}

// TracingLLMClient wraps any LLMClient and captures all interactions.
// All shard LLM calls flow through this wrapper for comprehensive tracing.
type TracingLLMClient struct {
	underlying LLMClient
	store      TraceStore

	// Current context (set before each shard execution)
	shardID       string
	shardType     string
	shardCategory string // system, ephemeral, specialist
	sessionID     string
	taskContext   string

	mu sync.RWMutex
}

// NewTracingLLMClient creates a tracing wrapper around an existing LLM client.
func NewTracingLLMClient(underlying LLMClient, store TraceStore) *TracingLLMClient {
	return &TracingLLMClient{
		underlying: underlying,
		store:      store,
	}
}

// SetShardContext sets the current shard context for trace attribution.
// Called by ShardManager before each shard execution.
func (tc *TracingLLMClient) SetShardContext(shardID, shardType, shardCategory, sessionID, taskContext string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.shardID = shardID
	tc.shardType = shardType
	tc.shardCategory = shardCategory
	tc.sessionID = sessionID
	tc.taskContext = taskContext
}

// ClearShardContext clears the current shard context after execution.
func (tc *TracingLLMClient) ClearShardContext() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.shardID = ""
	tc.shardType = ""
	tc.shardCategory = ""
	tc.taskContext = ""
}

// GetCurrentContext returns the current shard context (for debugging/testing).
func (tc *TracingLLMClient) GetCurrentContext() (shardID, shardType, shardCategory, sessionID, taskContext string) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.shardID, tc.shardType, tc.shardCategory, tc.sessionID, tc.taskContext
}

// Complete implements LLMClient.Complete with tracing.
func (tc *TracingLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return tc.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem implements LLMClient.CompleteWithSystem with tracing.
func (tc *TracingLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Capture current context
	tc.mu.RLock()
	shardID := tc.shardID
	shardType := tc.shardType
	shardCategory := tc.shardCategory
	sessionID := tc.sessionID
	taskContext := tc.taskContext
	tc.mu.RUnlock()

	start := time.Now()
	logging.API("LLM call started: shard=%s type=%s prompt_len=%d", shardID, shardType, len(userPrompt))

	// Make the actual LLM call
	response, err := tc.underlying.CompleteWithSystem(ctx, systemPrompt, userPrompt)

	duration := time.Since(start)
	if err != nil {
		logging.API("LLM call failed: shard=%s duration=%v error=%s", shardID, duration, err.Error())
	} else {
		logging.API("LLM call completed: shard=%s duration=%v response_len=%d", shardID, duration, len(response))
	}

	// Create trace
	trace := &ReasoningTrace{
		ID:            fmt.Sprintf("trace_%d", time.Now().UnixNano()),
		ShardID:       shardID,
		ShardType:     shardType,
		ShardCategory: shardCategory,
		SessionID:     sessionID,
		TaskContext:   taskContext,
		SystemPrompt:  systemPrompt,
		UserPrompt:    userPrompt,
		Response:      response,
		DurationMs:    duration.Milliseconds(),
		Success:       err == nil,
		Timestamp:     time.Now(),
	}

	if err != nil {
		trace.ErrorMessage = err.Error()
	}

	// Store trace asynchronously to not block execution
	if tc.store != nil {
		go func() {
			if storeErr := tc.store.StoreReasoningTrace(trace); storeErr != nil {
				logging.APIDebug("Failed to store reasoning trace: %v", storeErr)
			}
		}()
	}

	return response, err
}

type streamingChannelsClient interface {
	CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error)
}

type streamingCallbackClient interface {
	CompleteStreaming(ctx context.Context, systemPrompt, userPrompt string, callback StreamCallback) error
}

type modelGetter interface {
	GetModel() string
}

// CompleteWithStreaming implements streaming with trace capture.
// The response is streamed to the caller while also being buffered for persistence.
func (tc *TracingLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	// Capture current context
	tc.mu.RLock()
	shardID := tc.shardID
	shardType := tc.shardType
	shardCategory := tc.shardCategory
	sessionID := tc.sessionID
	taskContext := tc.taskContext
	tc.mu.RUnlock()

	start := time.Now()
	logging.API("LLM streaming call started: shard=%s type=%s prompt_len=%d", shardID, shardType, len(userPrompt))

	var underContent <-chan string
	var underErr <-chan error

	if streamer, ok := tc.underlying.(streamingChannelsClient); ok {
		underContent, underErr = streamer.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
	} else if streamer, ok := tc.underlying.(streamingCallbackClient); ok {
		contentChan := make(chan string, 100)
		errorChan := make(chan error, 1)
		go func() {
			defer close(contentChan)
			defer close(errorChan)
			err := streamer.CompleteStreaming(ctx, systemPrompt, userPrompt, func(chunk StreamChunk) error {
				if chunk.Error != "" {
					return fmt.Errorf("stream error: %s", chunk.Error)
				}
				delta := chunk.Text
				if delta == "" {
					delta = chunk.Content
				}
				if delta == "" || chunk.Done {
					return nil
				}
				select {
				case contentChan <- delta:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			if err != nil {
				errorChan <- err
			}
		}()
		underContent, underErr = contentChan, errorChan
	} else {
		contentChan := make(chan string)
		errorChan := make(chan error, 1)
		close(contentChan)
		errorChan <- core.ErrStreamingNotSupported
		close(errorChan)
		return contentChan, errorChan
	}

	outContent := make(chan string, 100)
	outErr := make(chan error, 1)

	go func() {
		defer close(outContent)
		defer close(outErr)

		var full strings.Builder
		contentClosed := false
		errClosed := false
		var firstErr error

		for !(contentClosed && errClosed) {
			select {
			case <-ctx.Done():
				if firstErr == nil {
					firstErr = ctx.Err()
				}
			case chunk, ok := <-underContent:
				if !ok {
					contentClosed = true
					continue
				}
				full.WriteString(chunk)
				select {
				case outContent <- chunk:
				case <-ctx.Done():
					if firstErr == nil {
						firstErr = ctx.Err()
					}
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
			logging.API("LLM streaming call failed: shard=%s duration=%v error=%s", shardID, duration, firstErr.Error())
		} else {
			logging.API("LLM streaming call completed: shard=%s duration=%v response_len=%d", shardID, duration, full.Len())
		}

		if firstErr != nil {
			outErr <- firstErr
		}

		trace := &ReasoningTrace{
			ID:            fmt.Sprintf("trace_%d", time.Now().UnixNano()),
			ShardID:       shardID,
			ShardType:     shardType,
			ShardCategory: shardCategory,
			SessionID:     sessionID,
			TaskContext:   taskContext,
			SystemPrompt:  systemPrompt,
			UserPrompt:    userPrompt,
			Response:      full.String(),
			DurationMs:    duration.Milliseconds(),
			Success:       firstErr == nil,
			Timestamp:     time.Now(),
		}
		if mg, ok := tc.underlying.(modelGetter); ok {
			trace.Model = mg.GetModel()
		}
		if firstErr != nil {
			trace.ErrorMessage = firstErr.Error()
		}

		// Store trace asynchronously to not block the caller
		if tc.store != nil {
			go func() {
				if storeErr := tc.store.StoreReasoningTrace(trace); storeErr != nil {
					logging.APIDebug("Failed to store streaming reasoning trace: %v", storeErr)
				}
			}()
		}
	}()

	return outContent, outErr
}

// GetUnderlying returns the wrapped LLM client.
// Use sparingly - prefer going through the tracing wrapper.
func (tc *TracingLLMClient) GetUnderlying() LLMClient {
	return tc.underlying
}

// ShardTraceAccessor provides shards with access to their own historical traces.
// This enables self-learning by querying past reasoning patterns.
type ShardTraceAccessor interface {
	// GetMyTraces retrieves recent traces for this shard type
	GetMyTraces(limit int) ([]ReasoningTrace, error)

	// GetMyFailedTraces retrieves failed traces for learning from errors
	GetMyFailedTraces(limit int) ([]ReasoningTrace, error)

	// GetSimilarTasks finds traces for tasks similar to the given pattern
	GetSimilarTasks(taskPattern string, limit int) ([]ReasoningTrace, error)

	// GetSuccessfulPatterns retrieves high-quality successful traces
	GetSuccessfulPatterns(limit int) ([]ReasoningTrace, error)
}

// ShardTraceStore implements ShardTraceAccessor for a specific shard type.
// Created by ShardManager and injected into each shard instance.
type ShardTraceStore struct {
	store     ShardTraceReader
	shardType string
}

// ShardTraceReader defines the read operations needed for shard self-access.
type ShardTraceReader interface {
	GetShardTraces(shardType string, limit int) ([]ReasoningTrace, error)
	GetFailedShardTraces(shardType string, limit int) ([]ReasoningTrace, error)
	GetSimilarTaskTraces(shardType, taskPattern string, limit int) ([]ReasoningTrace, error)
	GetHighQualityTraces(shardType string, minScore float64, limit int) ([]ReasoningTrace, error)
}

// NewShardTraceStore creates a trace store scoped to a specific shard type.
func NewShardTraceStore(store ShardTraceReader, shardType string) *ShardTraceStore {
	return &ShardTraceStore{
		store:     store,
		shardType: shardType,
	}
}

// GetMyTraces retrieves recent traces for this shard type.
func (sts *ShardTraceStore) GetMyTraces(limit int) ([]ReasoningTrace, error) {
	if sts.store == nil {
		return nil, nil
	}
	return sts.store.GetShardTraces(sts.shardType, limit)
}

// GetMyFailedTraces retrieves failed traces for learning from errors.
func (sts *ShardTraceStore) GetMyFailedTraces(limit int) ([]ReasoningTrace, error) {
	if sts.store == nil {
		return nil, nil
	}
	return sts.store.GetFailedShardTraces(sts.shardType, limit)
}

// GetSimilarTasks finds traces for tasks similar to the given pattern.
func (sts *ShardTraceStore) GetSimilarTasks(taskPattern string, limit int) ([]ReasoningTrace, error) {
	if sts.store == nil {
		return nil, nil
	}
	return sts.store.GetSimilarTaskTraces(sts.shardType, taskPattern, limit)
}

// GetSuccessfulPatterns retrieves high-quality successful traces.
func (sts *ShardTraceStore) GetSuccessfulPatterns(limit int) ([]ReasoningTrace, error) {
	if sts.store == nil {
		return nil, nil
	}
	return sts.store.GetHighQualityTraces(sts.shardType, 0.8, limit)
}

// ExtractLearnings analyzes traces and extracts actionable learnings.
// This is a helper function shards can use to learn from their history.
func ExtractLearnings(traces []ReasoningTrace) []string {
	var learnings []string

	successCount := 0
	failCount := 0
	var avgDuration int64

	for _, t := range traces {
		avgDuration += t.DurationMs
		if t.Success {
			successCount++
			if t.QualityScore >= 0.8 {
				// High quality success - extract the approach
				learnings = append(learnings, fmt.Sprintf("SUCCESS: %s", summarizeApproach(t)))
			}
		} else {
			failCount++
			// Learn from failure
			learnings = append(learnings, fmt.Sprintf("AVOID: %s (error: %s)", t.TaskContext, t.ErrorMessage))
		}
	}

	if len(traces) > 0 {
		avgDuration /= int64(len(traces))
		successRate := float64(successCount) / float64(len(traces)) * 100

		learnings = append(learnings, fmt.Sprintf("STATS: %.0f%% success rate, avg %dms latency", successRate, avgDuration))
	}

	return learnings
}

// summarizeApproach extracts a brief summary of the approach from a trace.
func summarizeApproach(t ReasoningTrace) string {
	// Truncate long responses for summary
	response := t.Response
	if len(response) > 200 {
		response = response[:200] + "..."
	}
	return response
}

// CategoryFromShardType determines the shard category based on type name.
func CategoryFromShardType(typeName string) string {
	// System shards (built-in, always-on)
	systemShards := map[string]bool{
		"perception_firewall": true,
		"constitution_gate":   true,
		"executive_policy":    true,
		"cost_guard":          true,
	}
	if systemShards[typeName] {
		return "system"
	}

	// Ephemeral shards (built-in factories)
	ephemeralShards := map[string]bool{
		"coder":      true,
		"tester":     true,
		"reviewer":   true,
		"researcher": true,
	}
	if ephemeralShards[typeName] {
		return "ephemeral"
	}

	// Everything else is a specialist (LLM-created or user-created)
	return "specialist"
}

// SystemLLMContext provides a helper for non-shard system components to make
// tracked LLM calls. This ensures proper logging attribution even for components
// that operate outside the shard lifecycle (e.g., compressor, transducer, feedback loop).
//
// Usage:
//
//	ctx := perception.NewSystemLLMContext(client, "compressor", "context-compression")
//	defer ctx.Clear()
//	response, err := ctx.Complete(ctx, prompt)
type SystemLLMContext struct {
	client      LLMClient
	componentID string
	taskContext string
	sessionID   string
}

// NewSystemLLMContext creates a context for a system component to make LLM calls.
// componentID should identify the component (e.g., "compressor", "transducer", "feedback_loop")
// taskContext describes the current operation (e.g., "context-compression", "intent-extraction")
func NewSystemLLMContext(client LLMClient, componentID, taskContext string) *SystemLLMContext {
	ctx := &SystemLLMContext{
		client:      client,
		componentID: componentID,
		taskContext: taskContext,
		sessionID:   fmt.Sprintf("system-%d", time.Now().UnixNano()),
	}

	// Set tracing context if the client supports it (including scheduler wrappers).
	type tracingContextSetter interface {
		SetShardContext(shardID, shardType, shardCategory, sessionID, taskContext string)
	}
	if tc, ok := client.(tracingContextSetter); ok {
		tc.SetShardContext(
			fmt.Sprintf("system-%s-%d", componentID, time.Now().UnixNano()),
			componentID,
			"system",
			ctx.sessionID,
			taskContext,
		)
	}

	return ctx
}

// Complete makes an LLM call with proper system context tracking.
func (s *SystemLLMContext) Complete(ctx context.Context, prompt string) (string, error) {
	return s.client.Complete(ctx, prompt)
}

// CompleteWithSystem makes an LLM call with system prompt and proper tracking.
func (s *SystemLLMContext) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return s.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

// Clear clears the system context after operations complete.
func (s *SystemLLMContext) Clear() {
	type tracingContextClearer interface {
		ClearShardContext()
	}
	if tc, ok := s.client.(tracingContextClearer); ok {
		tc.ClearShardContext()
	}
}
