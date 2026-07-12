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

	// MinCallSpacing is the minimum gap between successive slot grants.
	// SuperGrok / subscription backends should set ~100-200ms to smooth bursts.
	// Zero disables spacing.
	MinCallSpacing time.Duration

	// AdaptiveConcurrency enables shrinking max slots after rate-limit errors
	// and restoring them after a quiet period of successes.
	AdaptiveConcurrency bool

	// AdaptiveFloor is the minimum slots when throttled (default 1).
	AdaptiveFloor int

	// AdaptiveRecoverAfter is how long without rate limits before restoring
	// one slot toward the configured max (default 30s).
	AdaptiveRecoverAfter time.Duration
}

// DefaultAPISchedulerConfig returns sensible defaults.
func DefaultAPISchedulerConfig() APISchedulerConfig {
	return APISchedulerConfig{
		MaxConcurrentAPICalls: 5,               // Default for modern LLM providers (Gemini: 60 RPM Flash, 15 RPM Pro)
		SlotAcquireTimeout:    5 * time.Minute, // Match typical API timeout
		EnableMetrics:         true,
		AdaptiveFloor:         1,
		AdaptiveRecoverAfter:  30 * time.Second,
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

	// baseMaxSlots is the configured ceiling; adaptive mode may temporarily
	// lower config.MaxConcurrentAPICalls without losing the original value.
	baseMaxSlots int

	// nextAllowedGrant enforces MinCallSpacing between grants.
	nextAllowedGrant time.Time

	// Adaptive concurrency state
	lastRateLimitAt time.Time
	lastSuccessAt   time.Time
	rateLimitEvents int64

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
	if config.AdaptiveFloor <= 0 {
		config.AdaptiveFloor = 1
	}
	if config.AdaptiveRecoverAfter <= 0 {
		config.AdaptiveRecoverAfter = 30 * time.Second
	}

	return &APIScheduler{
		config:       config,
		baseMaxSlots: config.MaxConcurrentAPICalls,
		slots:        make(chan struct{}, config.MaxConcurrentAPICalls),
		shardStates:  make(map[string]*ShardExecutionState),
		waitQueue:    make([]*waitingEntry, 0),
		waiters:      make([]*schedWaiter, 0),
		stopCh:       make(chan struct{}),
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
//
// Free slots are always granted in priority order (then FIFO). Callers never
// skip ahead of a higher-priority waiter already in the queue — including the
// simultaneous-arrival race at t=0.
func (s *APIScheduler) AcquireAPISlot(ctx context.Context, shardID string) error {
	if ctx == nil {
		return fmt.Errorf("nil context provided to AcquireAPISlot")
	}

	// Opportunistic recovery of adaptive concurrency before queueing.
	s.maybeRecoverAdaptive()

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

	// Visibility queue
	entry := &waitingEntry{
		shardID:   shardID,
		shardType: state.ShardType,
		waitStart: waitStart,
		priority:  initialPriority,
	}
	s.waitQueue = append(s.waitQueue, entry)

	// Always enqueue as a waiter first so free-slot grants are priority-aware.
	w := make(chan struct{})
	s.waiterSeq++
	s.waiters = append(s.waiters, &schedWaiter{ch: w, priority: initialPriority, seq: s.waiterSeq})

	// Fill free slots by priority (may include us if we are highest).
	s.grantAvailableSlotsLocked()
	s.mu.Unlock()

	// If we were granted immediately, w is already closed.
	select {
	case <-w:
		return s.finishGranted(ctx, shardID, state, 0)
	default:
	}

	// Otherwise wait for a release (or timeout/cancel).
	atomic.AddInt32(&s.currentlyWaiting, 1)
	defer atomic.AddInt32(&s.currentlyWaiting, -1)

	logging.Shards("APIScheduler: shard %s waiting for slot (active=%d/%d, waiting=%d, prio=%s)",
		shardID, atomic.LoadInt32(&s.currentlyExecuting), s.config.MaxConcurrentAPICalls,
		atomic.LoadInt32(&s.currentlyWaiting), initialPriority.String())

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

	select {
	case <-w:
		waitDuration := time.Since(waitStart)
		if err := s.finishGranted(ctx, shardID, state, waitDuration); err != nil {
			return err
		}
		if waitDuration > 100*time.Millisecond {
			logging.Shards("APIScheduler: shard %s acquired slot after %v", shardID, waitDuration)
		}
		return nil

	case <-waitCtx.Done():
		select {
		case <-w:
			// Granted in the race window — honor the grant.
			return s.finishGranted(ctx, shardID, state, time.Since(waitStart))
		default:
		}

		s.mu.Lock()
		state.Phase = PhaseFailed
		state.Error = waitCtx.Err()
		s.removeWaiterLocked(w)
		s.removeWaitQueueLocked(shardID)
		s.mu.Unlock()

		logging.Get(logging.CategoryShards).Warn("APIScheduler: shard %s cancelled while waiting for slot (waited %v)",
			shardID, time.Since(waitStart))
		return waitCtx.Err()

	case <-s.stopCh:
		s.mu.Lock()
		s.removeWaiterLocked(w)
		s.removeWaitQueueLocked(shardID)
		s.mu.Unlock()
		return fmt.Errorf("scheduler stopped")
	}
}

// grantAvailableSlotsLocked wakes the highest-priority waiters for each free
// slot. Caller must hold s.mu. Each grant increments currentlyExecuting and
// closes the waiter's channel (same contract as ReleaseAPISlot).
func (s *APIScheduler) grantAvailableSlotsLocked() {
	for int(atomic.LoadInt32(&s.currentlyExecuting)) < s.config.MaxConcurrentAPICalls && len(s.waiters) > 0 {
		w := s.popNextWaiterLocked()
		if w == nil {
			break
		}
		atomic.AddInt32(&s.currentlyExecuting, 1)
		select {
		case s.slots <- struct{}{}:
		default:
		}
		close(w.ch)
	}
}

// finishGranted finalizes state after a waiter was granted a slot (either
// immediately or after waiting). Applies MinCallSpacing while holding the slot.
func (s *APIScheduler) finishGranted(ctx context.Context, shardID string, state *ShardExecutionState, waitDuration time.Duration) error {
	if waitDuration > 0 {
		atomic.AddInt64(&s.totalWaitTime, int64(waitDuration))
	}

	// Min call spacing: hold the slot while delaying so bursts don't slam the
	// provider at the same instant (important for SuperGrok).
	if spacing := s.config.MinCallSpacing; spacing > 0 {
		s.mu.Lock()
		now := time.Now()
		delay := time.Duration(0)
		if s.nextAllowedGrant.After(now) {
			delay = s.nextAllowedGrant.Sub(now)
		}
		s.nextAllowedGrant = now.Add(delay).Add(spacing)
		s.mu.Unlock()
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				// Drop the slot we never used for an API call.
				s.forceReleaseSlot(shardID)
				return ctx.Err()
			}
		}
	}

	s.mu.Lock()
	if state != nil {
		state.Phase = PhaseExecutingAPI
		if waitDuration > 0 {
			state.TotalWaitTime += waitDuration
		}
		state.LastAPICall = time.Now()
	}
	s.removeWaitQueueLocked(shardID)
	s.mu.Unlock()
	return nil
}

// forceReleaseSlot releases a held slot without counting an API call (used when
// acquire is cancelled during spacing, before the LLM is invoked).
func (s *APIScheduler) forceReleaseSlot(shardID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if atomic.LoadInt32(&s.currentlyExecuting) <= 0 {
		return
	}
	atomic.AddInt32(&s.currentlyExecuting, -1)
	select {
	case <-s.slots:
	default:
	}
	if state, ok := s.shardStates[shardID]; ok {
		state.Phase = PhaseFailed
		state.Error = context.Canceled
	}
	s.removeWaitQueueLocked(shardID)
	// Wake next waiter if any
	if w := s.popNextWaiterLocked(); w != nil {
		atomic.AddInt32(&s.currentlyExecuting, 1)
		select {
		case s.slots <- struct{}{}:
		default:
		}
		close(w.ch)
	}
}

func (s *APIScheduler) removeWaitQueueLocked(shardID string) {
	for i, e := range s.waitQueue {
		if e.shardID == shardID {
			s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
			return
		}
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
		globalScheduler.config.MinCallSpacing = cfg.MinCallSpacing
		globalScheduler.config.AdaptiveConcurrency = cfg.AdaptiveConcurrency
		if cfg.AdaptiveFloor > 0 {
			globalScheduler.config.AdaptiveFloor = cfg.AdaptiveFloor
		}
		if cfg.AdaptiveRecoverAfter > 0 {
			globalScheduler.config.AdaptiveRecoverAfter = cfg.AdaptiveRecoverAfter
		}
		globalScheduler.mu.Unlock()

		logging.Shards("APIScheduler: global dynamically reconfigured (max_slots=%d, timeout=%v, spacing=%v, adaptive=%v)",
			cfg.MaxConcurrentAPICalls, cfg.SlotAcquireTimeout, cfg.MinCallSpacing, cfg.AdaptiveConcurrency)
	} else {
		logging.Shards("APIScheduler: global config set (max_slots=%d, spacing=%v, adaptive=%v)",
			cfg.MaxConcurrentAPICalls, cfg.MinCallSpacing, cfg.AdaptiveConcurrency)
	}
}

// UpdateMaxConcurrentAPICalls dynamically modifies the MaxConcurrentAPICalls slot capacity.
// Also updates the adaptive base ceiling so recovery targets the new max.
func (s *APIScheduler) UpdateMaxConcurrentAPICalls(newMax int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if newMax <= 0 {
		return
	}
	oldMax := s.config.MaxConcurrentAPICalls
	s.baseMaxSlots = newMax
	if oldMax == newMax {
		return
	}

	s.config.MaxConcurrentAPICalls = newMax
	s.resizeSlotsLocked(newMax)
	s.grantAvailableSlotsLocked()

	logging.Shards("APIScheduler: dynamically updated MaxConcurrentAPICalls from %d to %d (executing=%d)",
		oldMax, newMax, atomic.LoadInt32(&s.currentlyExecuting))
}

func (s *APIScheduler) resizeSlotsLocked(newMax int) {
	newSlots := make(chan struct{}, newMax)
	currentExecuting := int(atomic.LoadInt32(&s.currentlyExecuting))
	for i := 0; i < currentExecuting && i < newMax; i++ {
		newSlots <- struct{}{}
	}
	s.slots = newSlots
}

// ReportRateLimit records a provider rate-limit response and, when adaptive
// concurrency is enabled, shrinks the slot ceiling (floor AdaptiveFloor).
func (s *APIScheduler) ReportRateLimit() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastRateLimitAt = time.Now()
	s.rateLimitEvents++
	if !s.config.AdaptiveConcurrency {
		return
	}
	floor := s.config.AdaptiveFloor
	if floor <= 0 {
		floor = 1
	}
	old := s.config.MaxConcurrentAPICalls
	newMax := old - 1
	if newMax < floor {
		newMax = floor
	}
	if newMax == old {
		return
	}
	s.config.MaxConcurrentAPICalls = newMax
	s.resizeSlotsLocked(newMax)
	logging.Get(logging.CategoryShards).Warn(
		"APIScheduler: rate limit → adaptive concurrency %d → %d (base=%d, events=%d)",
		old, newMax, s.baseMaxSlots, s.rateLimitEvents,
	)
}

// ReportSuccess records a successful LLM call. May restore one adaptive slot
// after AdaptiveRecoverAfter without further rate limits.
func (s *APIScheduler) ReportSuccess() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.lastSuccessAt = time.Now()
	s.mu.Unlock()
	s.maybeRecoverAdaptive()
}

func (s *APIScheduler) maybeRecoverAdaptive() {
	if s == nil || !s.config.AdaptiveConcurrency {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.baseMaxSlots <= 0 {
		s.baseMaxSlots = s.config.MaxConcurrentAPICalls
	}
	if s.config.MaxConcurrentAPICalls >= s.baseMaxSlots {
		return
	}
	recoverAfter := s.config.AdaptiveRecoverAfter
	if recoverAfter <= 0 {
		recoverAfter = 30 * time.Second
	}
	if !s.lastRateLimitAt.IsZero() && time.Since(s.lastRateLimitAt) < recoverAfter {
		return
	}
	// Prefer at least one success after the last rate limit before growing.
	if !s.lastSuccessAt.IsZero() && s.lastSuccessAt.Before(s.lastRateLimitAt) {
		return
	}
	old := s.config.MaxConcurrentAPICalls
	newMax := old + 1
	if newMax > s.baseMaxSlots {
		newMax = s.baseMaxSlots
	}
	if newMax == old {
		return
	}
	s.config.MaxConcurrentAPICalls = newMax
	s.resizeSlotsLocked(newMax)
	s.grantAvailableSlotsLocked()
	logging.Shards("APIScheduler: adaptive concurrency recovered %d → %d (base=%d)", old, newMax, s.baseMaxSlots)
}

// EffectiveMaxSlots returns the current concurrency ceiling (may be reduced adaptively).
func (s *APIScheduler) EffectiveMaxSlots() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.MaxConcurrentAPICalls
}

// BaseMaxSlots returns the configured non-adaptive ceiling.
func (s *APIScheduler) BaseMaxSlots() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.baseMaxSlots > 0 {
		return s.baseMaxSlots
	}
	return s.config.MaxConcurrentAPICalls
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
