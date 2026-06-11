package core

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
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
	Checkpoint    map[string]any // Shard-specific state for resume
	Error         error

	// DefaultPriority is used for slot arbitration when the request context
	// carries no explicit CtxKeyPriority. Interactive callers (the chat
	// turn's perception/articulation clients) register as PriorityHigh so a
	// user staring at a spinner is never queued behind background learning
	// or consolidation work.
	DefaultPriority types.SpawnPriority
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
		MaxConcurrentAPICalls: 5,               // Default for modern LLM providers (Gemini: 60 RPM Flash, 15 RPM Pro)
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
	waiters     []*schedWaiter  // Waiter list: highest priority first, FIFO within priority
	waiterSeq   uint64          // Monotonic sequence for FIFO tie-breaking

	// Metrics
	totalAPICalls      int64
	totalWaitTime      int64 // nanoseconds
	currentlyWaiting   int32
	currentlyExecuting int32

	// Lifecycle
	stopCh   chan struct{}
	stopOnce sync.Once
}

type waitingEntry struct {
	shardID   string
	shardType string
	waitStart time.Time
	priority  types.SpawnPriority
}

// schedWaiter is a queued slot request. Wake-up order is highest priority
// first, FIFO (by seq) within the same priority. Before this existed, the
// priority was parsed from the context and then ignored — waiters woke in
// strict FIFO order, so an interactive turn could queue behind a pile of
// background learning calls for minutes.
type schedWaiter struct {
	ch       chan struct{}
	priority types.SpawnPriority
	seq      uint64
}

// popNextWaiterLocked removes and returns the next waiter to wake:
// highest priority, then earliest sequence. Caller must hold s.mu.
// Returns nil when no waiters are queued.
func (s *APIScheduler) popNextWaiterLocked() *schedWaiter {
	if len(s.waiters) == 0 {
		return nil
	}
	best := 0
	for i := 1; i < len(s.waiters); i++ {
		w := s.waiters[i]
		b := s.waiters[best]
		if w.priority > b.priority || (w.priority == b.priority && w.seq < b.seq) {
			best = i
		}
	}
	w := s.waiters[best]
	s.waiters = append(s.waiters[:best], s.waiters[best+1:]...)
	return w
}

// removeWaiterLocked removes a specific waiter (by channel identity).
// Caller must hold s.mu.
func (s *APIScheduler) removeWaiterLocked(ch chan struct{}) {
	for i, w := range s.waiters {
		if w.ch == ch {
			s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
			return
		}
	}
}

// NewAPIScheduler creates a new scheduler.
func NewAPIScheduler(config APISchedulerConfig) *APIScheduler {
	if config.MaxConcurrentAPICalls <= 0 {
		config.MaxConcurrentAPICalls = 5 // Defensively use default
	}
	if config.SlotAcquireTimeout <= 0 {
		config.SlotAcquireTimeout = 5 * time.Minute // Defensively use default
	}

	return &APIScheduler{
		config:      config,
		slots:       make(chan struct{}, config.MaxConcurrentAPICalls),
		shardStates: make(map[string]*ShardExecutionState),
		waitQueue:   make([]*waitingEntry, 0),
		waiters:     make([]*schedWaiter, 0),
		stopCh:      make(chan struct{}),
	}
}

// RegisterShard creates state tracking for a new shard.
func (s *APIScheduler) RegisterShard(shardID, shardType string) *ShardExecutionState {
	return s.RegisterShardWithPriority(shardID, shardType, types.PriorityNormal)
}

// RegisterShardWithPriority creates state tracking with a default slot
// priority. Interactive callers register PriorityHigh so user-facing calls
// jump the queue ahead of background work.
func (s *APIScheduler) RegisterShardWithPriority(shardID, shardType string, priority types.SpawnPriority) *ShardExecutionState {
	if shardID == "" {
		logging.Get(logging.CategoryShards).Error("APIScheduler: attempt to register shard with empty ID")
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := &ShardExecutionState{
		ShardID:         shardID,
		ShardType:       shardType,
		Phase:           PhaseInitializing,
		StartTime:       time.Now(),
		Checkpoint:      make(map[string]any),
		DefaultPriority: priority,
	}
	s.shardStates[shardID] = state

	logging.Shards("APIScheduler: registered shard %s (type=%s, default_priority=%d)", shardID, shardType, priority)
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
	if ctx == nil {
		return fmt.Errorf("nil context provided to AcquireAPISlot")
	}

	s.mu.Lock()
	state, ok := s.shardStates[shardID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("shard %s not registered with scheduler", shardID)
	}
	state.Phase = PhaseWaitingForSlot
	waitStart := time.Now()

	// Determine the priority bucket: an explicit context priority (set by
	// ShardManager.Spawn for prioritized spawns) wins; otherwise fall back to
	// the priority the shard registered with (interactive clients register
	// high), defaulting to Normal.
	initialPriority := state.DefaultPriority
	if prioVal := ctx.Value(types.CtxKeyPriority); prioVal != nil {
		if p, ok := prioVal.(types.SpawnPriority); ok {
			initialPriority = p
		}
	}

	// Add to wait queue for visibility
	entry := &waitingEntry{
		shardID:   shardID,
		shardType: state.ShardType,
		waitStart: waitStart,
		priority:  initialPriority,
	}
	s.waitQueue = append(s.waitQueue, entry)

	// If we have an available slot immediately, acquire it
	if int(atomic.LoadInt32(&s.currentlyExecuting)) < s.config.MaxConcurrentAPICalls {
		atomic.AddInt32(&s.currentlyExecuting, 1)

		// Fill s.slots non-blocking to keep len(s.slots) aligned if needed
		select {
		case s.slots <- struct{}{}:
		default:
		}

		state.Phase = PhaseExecutingAPI
		state.LastAPICall = time.Now()

		// Remove from wait queue
		for i, e := range s.waitQueue {
			if e.shardID == shardID {
				s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		return nil
	}

	// Otherwise, we must queue up and wait!
	w := make(chan struct{})
	s.waiterSeq++
	s.waiters = append(s.waiters, &schedWaiter{ch: w, priority: initialPriority, seq: s.waiterSeq})
	s.mu.Unlock()

	atomic.AddInt32(&s.currentlyWaiting, 1)
	defer atomic.AddInt32(&s.currentlyWaiting, -1)

	// Log if we're actually waiting
	logging.Shards("APIScheduler: shard %s waiting for slot (active=%d/%d, waiting=%d)",
		shardID, atomic.LoadInt32(&s.currentlyExecuting), s.config.MaxConcurrentAPICalls, atomic.LoadInt32(&s.currentlyWaiting))

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
	case <-w:
		// Got the slot! The releaser has already incremented s.currentlyExecuting for us.
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
		if waitDuration > 100*time.Millisecond {
			logging.Shards("APIScheduler: shard %s acquired slot after %v", shardID, waitDuration)
		}
		return nil

	case <-waitCtx.Done():
		// Check if we were actually woken up just as we cancelled (TOCTOU prevention)
		select {
		case <-w:
			// We got the slot after all! Ignore the cancellation/timeout.
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
			return nil
		default:
			// Genuinely cancelled before getting slot
		}

		// Context cancelled while waiting
		s.mu.Lock()
		state.Phase = PhaseFailed
		state.Error = waitCtx.Err()

		// Remove our waiter channel from the list
		s.removeWaiterLocked(w)

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
		s.removeWaiterLocked(w)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	// Release the slot
	if atomic.LoadInt32(&s.currentlyExecuting) <= 0 {
		logging.Get(logging.CategoryShards).Error("APIScheduler: shard %s released slot it didn't hold", shardID)
		return
	}

	atomic.AddInt32(&s.currentlyExecuting, -1)
	atomic.AddInt64(&s.totalAPICalls, 1)

	// Keep len(s.slots) aligned if needed
	select {
	case <-s.slots:
	default:
	}

	if state, ok := s.shardStates[shardID]; ok {
		state.Phase = PhaseProcessingResult
		state.APICallCount++
	}

	// Wake the highest-priority waiter if any (FIFO within priority)
	if w := s.popNextWaiterLocked(); w != nil {
		atomic.AddInt32(&s.currentlyExecuting, 1)

		// Align len(s.slots)
		select {
		case s.slots <- struct{}{}:
		default:
		}

		close(w.ch)
	}

	logging.ShardsDebug("APIScheduler: shard %s released slot (total_calls=%d)", shardID, atomic.LoadInt64(&s.totalAPICalls))
}

// SaveCheckpoint stores shard-specific state for resume after yielding.
func (s *APIScheduler) SaveCheckpoint(shardID string, key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state, ok := s.shardStates[shardID]; ok {
		if len(state.Checkpoint) >= 1000 {
			logging.Get(logging.CategoryShards).Warn("APIScheduler: checkpoint size limit (1000) reached for shard %s; ignoring save", shardID)
			return
		}
		state.Checkpoint[key] = value
	}
}

// LoadCheckpoint retrieves saved state.
func (s *APIScheduler) LoadCheckpoint(shardID string, key string) (any, bool) {
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
	stateCopy.Checkpoint = make(map[string]any, len(state.Checkpoint))
	maps.Copy(stateCopy.Checkpoint, state.Checkpoint)
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

// Stop shuts down the scheduler. Safe to call multiple times; subsequent
// calls are no-ops. Without sync.Once, a double-close of stopCh would panic.
func (s *APIScheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
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
// If the global scheduler is already initialized, we now dynamically reconfigure it!
func ConfigureGlobalAPIScheduler(cfg APISchedulerConfig) {
	globalSchedulerConfigMu.Lock()
	defer globalSchedulerConfigMu.Unlock()

	// Apply defaults for unset fields
	if cfg.MaxConcurrentAPICalls <= 0 {
		cfg.MaxConcurrentAPICalls = DefaultAPISchedulerConfig().MaxConcurrentAPICalls
	}
	if cfg.SlotAcquireTimeout <= 0 {
		cfg.SlotAcquireTimeout = DefaultAPISchedulerConfig().SlotAcquireTimeout
	}

	globalSchedulerConfig = cfg

	if globalScheduler != nil {
		globalScheduler.UpdateMaxConcurrentAPICalls(cfg.MaxConcurrentAPICalls)

		globalScheduler.mu.Lock()
		globalScheduler.config.SlotAcquireTimeout = cfg.SlotAcquireTimeout
		globalScheduler.config.EnableMetrics = cfg.EnableMetrics
		globalScheduler.mu.Unlock()

		logging.Shards("APIScheduler: global dynamically reconfigured (max_slots=%d, timeout=%v)",
			cfg.MaxConcurrentAPICalls, cfg.SlotAcquireTimeout)
	} else {
		logging.Shards("APIScheduler: global config set (max_slots=%d)", cfg.MaxConcurrentAPICalls)
	}
}

// UpdateMaxConcurrentAPICalls dynamically modifies the MaxConcurrentAPICalls slot capacity.
func (s *APIScheduler) UpdateMaxConcurrentAPICalls(newMax int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if newMax <= 0 {
		return
	}
	oldMax := s.config.MaxConcurrentAPICalls
	if oldMax == newMax {
		return
	}

	s.config.MaxConcurrentAPICalls = newMax

	// Recreate slots channel to match new capacity
	newSlots := make(chan struct{}, newMax)
	currentExecuting := int(atomic.LoadInt32(&s.currentlyExecuting))
	for i := 0; i < currentExecuting && i < newMax; i++ {
		newSlots <- struct{}{}
	}
	s.slots = newSlots

	// Wake up as many waiters as new capacity allows (priority order)
	for int(atomic.LoadInt32(&s.currentlyExecuting)) < s.config.MaxConcurrentAPICalls && len(s.waiters) > 0 {
		w := s.popNextWaiterLocked()
		if w == nil {
			break
		}
		atomic.AddInt32(&s.currentlyExecuting, 1)

		// Fill s.slots non-blocking
		select {
		case s.slots <- struct{}{}:
		default:
		}

		close(w.ch)
	}

	logging.Shards("APIScheduler: dynamically updated MaxConcurrentAPICalls from %d to %d (executing=%d)", oldMax, newMax, currentExecuting)
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
