package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// API SCHEDULER - COOPERATIVE SHARD SCHEDULING
// =============================================================================
//
// The APIScheduler manages API call slots independently of shard slots.
// This allows many shards to be spawned, but limits concurrent API calls.
// Shards yield their slot after each API call and must re-acquire for the next.
//
// Key concepts:
// - API Slot: Permission to make one LLM API call
// - Shard State: Tracks progress for resume after yielding
// - Cooperative Yielding: Shards release slots between API calls

// -----------------------------------------------------------------------------
// Shard Execution State
// -----------------------------------------------------------------------------

// ShardPhase represents where a shard is in its execution lifecycle.
type ShardPhase int

const (
	// PhaseInitializing - shard is setting up, hasn't made API calls yet
	PhaseInitializing ShardPhase = iota
	// PhaseWaitingForSlot - shard is queued waiting for an API slot
	PhaseWaitingForSlot
	// PhaseExecutingAPI - shard is actively making an API call
	PhaseExecutingAPI
	// PhaseProcessingResult - shard is processing API response (no slot needed)
	PhaseProcessingResult
	// PhaseCompleted - shard has finished all work
	PhaseCompleted
	// PhaseFailed - shard encountered an error
	PhaseFailed
)

func (p ShardPhase) String() string {
	switch p {
	case PhaseInitializing:
		return "initializing"
	case PhaseWaitingForSlot:
		return "waiting_for_slot"
	case PhaseExecutingAPI:
		return "executing_api"
	case PhaseProcessingResult:
		return "processing_result"
	case PhaseCompleted:
		return "completed"
	case PhaseFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", p)
	}
}

// ShardExecutionState tracks the progress of a shard for suspend/resume.
type ShardExecutionState struct {
	ShardID       string
	ShardType     string
	Phase         ShardPhase
	APICallCount  int           // Number of API calls made so far
	TotalWaitTime time.Duration // Total time spent waiting for slots
	StartTime     time.Time
	LastAPICall   time.Time
	Checkpoint    map[string]interface{} // Shard-specific state for resume
	Error         error
}

// -----------------------------------------------------------------------------
// API Scheduler
// -----------------------------------------------------------------------------

// APISchedulerConfig configures the scheduler.
type APISchedulerConfig struct {
	MaxConcurrentAPICalls int           // Max simultaneous API calls (matches LLM provider limit)
	SlotAcquireTimeout    time.Duration // Max time to wait for a slot
	EnableMetrics         bool          // Track detailed metrics
}

// DefaultAPISchedulerConfig returns sensible defaults.
func DefaultAPISchedulerConfig() APISchedulerConfig {
	return APISchedulerConfig{
		MaxConcurrentAPICalls: 5,               // Z.AI limit
		SlotAcquireTimeout:    5 * time.Minute, // Match typical API timeout
		EnableMetrics:         true,
	}
}

// APIScheduler manages API call slots with cooperative yielding.
type APIScheduler struct {
	config APISchedulerConfig
	slots  chan struct{} // Semaphore for API slots

	// State tracking
	mu          sync.RWMutex
	shardStates map[string]*ShardExecutionState
	waitQueue   []*waitingEntry // Shards waiting for slots (for logging/metrics)

	// Metrics
	totalAPICalls      int64
	totalWaitTime      int64 // nanoseconds
	currentlyWaiting   int32
	currentlyExecuting int32

	// Lifecycle
	stopCh chan struct{}
}

type waitingEntry struct {
	shardID   string
	shardType string
	waitStart time.Time
	priority  SpawnPriority
}

// NewAPIScheduler creates a new scheduler.
func NewAPIScheduler(config APISchedulerConfig) *APIScheduler {
	return &APIScheduler{
		config:      config,
		slots:       make(chan struct{}, config.MaxConcurrentAPICalls),
		shardStates: make(map[string]*ShardExecutionState),
		waitQueue:   make([]*waitingEntry, 0),
		stopCh:      make(chan struct{}),
	}
}

// RegisterShard creates state tracking for a new shard.
func (s *APIScheduler) RegisterShard(shardID, shardType string) *ShardExecutionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := &ShardExecutionState{
		ShardID:    shardID,
		ShardType:  shardType,
		Phase:      PhaseInitializing,
		StartTime:  time.Now(),
		Checkpoint: make(map[string]interface{}),
	}
	s.shardStates[shardID] = state

	logging.Shards("APIScheduler: registered shard %s (type=%s)", shardID, shardType)
	return state
}

// UnregisterShard removes state tracking for a completed shard.
func (s *APIScheduler) UnregisterShard(shardID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state, ok := s.shardStates[shardID]; ok {
		state.Phase = PhaseCompleted
		delete(s.shardStates, shardID)
		logging.Shards("APIScheduler: unregistered shard %s (api_calls=%d, total_wait=%v)",
			shardID, state.APICallCount, state.TotalWaitTime)
	}
}

// AcquireAPISlot acquires permission to make an API call.
// Blocks until a slot is available or context is cancelled.
// The shard enters PhaseWaitingForSlot while waiting.
func (s *APIScheduler) AcquireAPISlot(ctx context.Context, shardID string) error {
	s.mu.Lock()
	state, ok := s.shardStates[shardID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("shard %s not registered with scheduler", shardID)
	}
	state.Phase = PhaseWaitingForSlot
	waitStart := time.Now()

	// Add to wait queue for visibility
	entry := &waitingEntry{
		shardID:   shardID,
		shardType: state.ShardType,
		waitStart: waitStart,
	}
	s.waitQueue = append(s.waitQueue, entry)
	s.mu.Unlock()

	atomic.AddInt32(&s.currentlyWaiting, 1)
	defer atomic.AddInt32(&s.currentlyWaiting, -1)

	// Log if we're actually waiting
	activeSlots := len(s.slots)
	if activeSlots >= s.config.MaxConcurrentAPICalls {
		logging.Shards("APIScheduler: shard %s waiting for slot (active=%d/%d, waiting=%d)",
			shardID, activeSlots, s.config.MaxConcurrentAPICalls, atomic.LoadInt32(&s.currentlyWaiting))
	}

	waitCtx := ctx
	var waitCancel context.CancelFunc
	if timeout := s.config.SlotAcquireTimeout; timeout > 0 {
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > timeout {
			waitCtx, waitCancel = context.WithTimeout(ctx, timeout)
		}
	}
	if waitCancel != nil {
		defer waitCancel()
	}

	// Try to acquire slot
	select {
	case s.slots <- struct{}{}:
		// Got a slot
		waitDuration := time.Since(waitStart)

		s.mu.Lock()
		state.Phase = PhaseExecutingAPI
		state.TotalWaitTime += waitDuration
		state.LastAPICall = time.Now()

		// Remove from wait queue
		for i, e := range s.waitQueue {
			if e.shardID == shardID {
				s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
				break
			}
		}
		s.mu.Unlock()

		atomic.AddInt64(&s.totalWaitTime, int64(waitDuration))
		atomic.AddInt32(&s.currentlyExecuting, 1)

		if waitDuration > 100*time.Millisecond {
			logging.Shards("APIScheduler: shard %s acquired slot after %v", shardID, waitDuration)
		}
		return nil

	case <-waitCtx.Done():
		// Context cancelled while waiting
		s.mu.Lock()
		state.Phase = PhaseFailed
		state.Error = waitCtx.Err()
		// Remove from wait queue
		for i, e := range s.waitQueue {
			if e.shardID == shardID {
				s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
				break
			}
		}
		s.mu.Unlock()

		logging.Get(logging.CategoryShards).Warn("APIScheduler: shard %s cancelled while waiting for slot (waited %v)",
			shardID, time.Since(waitStart))
		return waitCtx.Err()

	case <-s.stopCh:
		// Clean up wait queue on scheduler stop
		s.mu.Lock()
		for i, e := range s.waitQueue {
			if e.shardID == shardID {
				s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		return fmt.Errorf("scheduler stopped")
	}
}

// ReleaseAPISlot releases the API slot after call completes.
// The shard enters PhaseProcessingResult and can do local work before next API call.
func (s *APIScheduler) ReleaseAPISlot(shardID string) {
	// Release the slot
	select {
	case <-s.slots:
		// Slot released
	default:
		// Shouldn't happen - means we're releasing without acquiring
		logging.Get(logging.CategoryShards).Error("APIScheduler: shard %s released slot it didn't hold", shardID)
		return
	}

	atomic.AddInt32(&s.currentlyExecuting, -1)
	atomic.AddInt64(&s.totalAPICalls, 1)

	s.mu.Lock()
	if state, ok := s.shardStates[shardID]; ok {
		state.Phase = PhaseProcessingResult
		state.APICallCount++
	}
	s.mu.Unlock()

	logging.ShardsDebug("APIScheduler: shard %s released slot (total_calls=%d)", shardID, atomic.LoadInt64(&s.totalAPICalls))
}

// SaveCheckpoint stores shard-specific state for resume after yielding.
func (s *APIScheduler) SaveCheckpoint(shardID string, key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state, ok := s.shardStates[shardID]; ok {
		state.Checkpoint[key] = value
	}
}

// LoadCheckpoint retrieves saved state.
func (s *APIScheduler) LoadCheckpoint(shardID string, key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if state, ok := s.shardStates[shardID]; ok {
		val, exists := state.Checkpoint[key]
		return val, exists
	}
	return nil, false
}

// GetShardState returns the current state of a shard.
// Returns a deep copy to avoid races with checkpoint map.
func (s *APIScheduler) GetShardState(shardID string) (*ShardExecutionState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.shardStates[shardID]
	if !ok {
		return nil, false
	}
	// Return a deep copy to avoid races
	stateCopy := *state
	// Deep copy the checkpoint map
	stateCopy.Checkpoint = make(map[string]interface{}, len(state.Checkpoint))
	for k, v := range state.Checkpoint {
		stateCopy.Checkpoint[k] = v
	}
	return &stateCopy, true
}

// GetMetrics returns current scheduler metrics.
func (s *APIScheduler) GetMetrics() APISchedulerMetrics {
	s.mu.RLock()
	activeShards := len(s.shardStates)
	waitingShards := len(s.waitQueue)

	// Calculate phase distribution
	phases := make(map[ShardPhase]int)
	for _, state := range s.shardStates {
		phases[state.Phase]++
	}
	s.mu.RUnlock()

	return APISchedulerMetrics{
		MaxSlots:          s.config.MaxConcurrentAPICalls,
		ActiveSlots:       int(atomic.LoadInt32(&s.currentlyExecuting)),
		WaitingForSlot:    int(atomic.LoadInt32(&s.currentlyWaiting)),
		TotalAPICalls:     atomic.LoadInt64(&s.totalAPICalls),
		TotalWaitTimeNs:   atomic.LoadInt64(&s.totalWaitTime),
		RegisteredShards:  activeShards,
		WaitingShards:     waitingShards,
		PhaseDistribution: phases,
	}
}

// APISchedulerMetrics provides observability into scheduler state.
type APISchedulerMetrics struct {
	MaxSlots          int
	ActiveSlots       int
	WaitingForSlot    int
	TotalAPICalls     int64
	TotalWaitTimeNs   int64
	RegisteredShards  int
	WaitingShards     int
	PhaseDistribution map[ShardPhase]int
}

// String returns a human-readable summary.
func (m APISchedulerMetrics) String() string {
	avgWait := time.Duration(0)
	if m.TotalAPICalls > 0 {
		avgWait = time.Duration(m.TotalWaitTimeNs / m.TotalAPICalls)
	}
	return fmt.Sprintf("slots=%d/%d, waiting=%d, api_calls=%d, avg_wait=%v, shards=%d",
		m.ActiveSlots, m.MaxSlots, m.WaitingForSlot, m.TotalAPICalls, avgWait, m.RegisteredShards)
}

// Stop shuts down the scheduler.
func (s *APIScheduler) Stop() {
	close(s.stopCh)
}

// -----------------------------------------------------------------------------
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

	// Make the actual LLM call
	return c.Client.Complete(ctx, prompt)
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

	// Make the actual LLM call
	return c.Client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
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

	underContent, underErr := streamer.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)

	go func() {
		defer c.Scheduler.ReleaseAPISlot(c.ShardID)
		defer close(contentChan)
		defer close(errorChan)

		contentClosed := false
		errClosed := false
		var firstErr error

		for !(contentClosed && errClosed) {
			select {
			case <-ctx.Done():
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				// Keep draining until channels close so upstream goroutines can exit cleanly.
			case chunk, ok := <-underContent:
				if !ok {
					contentClosed = true
					continue
				}
				select {
				case contentChan <- chunk:
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

		if firstErr != nil {
			errorChan <- firstErr
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
// Global Scheduler Instance
// -----------------------------------------------------------------------------

var (
	globalScheduler         *APIScheduler
	globalSchedulerOnce     sync.Once
	globalSchedulerConfigMu sync.Mutex
	globalSchedulerConfig   = DefaultAPISchedulerConfig()
)

// ConfigureGlobalAPIScheduler sets the config used for the global scheduler.
// Must be called before the first GetAPIScheduler() to take effect.
// If the global scheduler is already initialized, the call is ignored.
func ConfigureGlobalAPIScheduler(cfg APISchedulerConfig) {
	globalSchedulerConfigMu.Lock()
	defer globalSchedulerConfigMu.Unlock()

	if globalScheduler != nil {
		logging.Shards("APIScheduler: global already initialized; ignoring reconfigure (requested_max=%d)",
			cfg.MaxConcurrentAPICalls)
		return
	}

	// Apply defaults for unset fields
	if cfg.MaxConcurrentAPICalls <= 0 {
		cfg.MaxConcurrentAPICalls = DefaultAPISchedulerConfig().MaxConcurrentAPICalls
	}
	if cfg.SlotAcquireTimeout <= 0 {
		cfg.SlotAcquireTimeout = DefaultAPISchedulerConfig().SlotAcquireTimeout
	}

	globalSchedulerConfig = cfg
	logging.Shards("APIScheduler: global config set (max_slots=%d)", cfg.MaxConcurrentAPICalls)
}

// GetAPIScheduler returns the global API scheduler instance.
func GetAPIScheduler() *APIScheduler {
	globalSchedulerOnce.Do(func() {
		globalSchedulerConfigMu.Lock()
		cfg := globalSchedulerConfig
		globalSchedulerConfigMu.Unlock()
		globalScheduler = NewAPIScheduler(cfg)
		logging.Shards("APIScheduler: initialized global instance (max_slots=%d)",
			globalScheduler.config.MaxConcurrentAPICalls)
	})
	return globalScheduler
}

// NewScheduledLLMCall creates a wrapper for scheduled LLM calls.
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
