package shards

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/core" // For LimitsEnforcer
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// SPAWN QUEUE WITH BACKPRESSURE
// =============================================================================
//
// SpawnQueue provides prioritized, backpressure-aware shard spawning.
// Instead of failing immediately when limits are reached, requests queue
// and wait for available slots, enabling graceful degradation under load.

var (
	// ErrQueueFull is returned when queue cannot accept more requests.
	ErrQueueFull = errors.New("spawn queue is full")

	// ErrQueueTimeout is returned when request times out waiting in queue.
	ErrQueueTimeout = errors.New("spawn request timed out in queue")

	// ErrQueueStopped is returned when queue is stopped.
	ErrQueueStopped = errors.New("spawn queue is stopped")
)

// SpawnQueueConfig configures the spawn queue behavior.
type SpawnQueueConfig struct {
	MaxQueueSize        int           // Max total requests across all priorities
	MaxQueuePerPriority int           // Max requests per priority level
	DefaultTimeout      time.Duration // Default timeout for queued requests
	HighWaterMark       float64       // Queue utilization to start signaling backpressure (0.7)
	WorkerCount         int           // Number of concurrent spawn workers
	DrainTimeout        time.Duration // Timeout when stopping queue
}

// DefaultSpawnQueueConfig returns sensible defaults.
func DefaultSpawnQueueConfig() SpawnQueueConfig {
	return SpawnQueueConfig{
		MaxQueueSize:        100,
		MaxQueuePerPriority: 30,
		DefaultTimeout:      5 * time.Minute,
		HighWaterMark:       0.7,
		WorkerCount:         2,
		DrainTimeout:        30 * time.Second,
	}
}

// BackpressureStatus represents the current queue state for callers.
type BackpressureStatus struct {
	QueueDepth       int     // Total items in all queues
	QueueUtilization float64 // 0.0-1.0, how full the queue is
	AvailableSlots   int     // Estimated available spawn slots
	Accepting        bool    // Whether queue is accepting new requests
	Reason           string  // If not accepting, why
}

// SpawnRequest represents a queued spawn request.
// Wrapper around types if needed, or use types directly.
// We'll use a local struct for queue management but referencing types.
type spawnRequestWrapper struct {
	ID          string
	TypeName    string
	Task        string
	SessionCtx  *types.SessionContext
	Priority    types.SpawnPriority
	SubmittedAt time.Time
	Deadline    time.Time
	ResultCh    chan SpawnResult
	Ctx         context.Context
	Detached    bool
}

// SpawnResult contains the outcome of a spawn request.
type SpawnResult struct {
	ShardID string        // ID of spawned shard (empty if error)
	Result  string        // Execution result
	Error   error         // Error if spawn or execution failed
	Queued  time.Duration // How long request waited in queue
}

// SpawnQueue manages prioritized, backpressured shard spawning.
type SpawnQueue struct {
	mu sync.RWMutex

	// Priority queues (4 levels: Low, Normal, High, Critical)
	queues [4]chan *spawnRequestWrapper

	// Configuration
	config SpawnQueueConfig

	// Dependencies
	shardManager   *ShardManager
	limitsEnforcer *core.LimitsEnforcer

	// State
	isRunning bool
	stopCh    chan struct{}
	workerWg  sync.WaitGroup

	// Metrics (atomic for lock-free reads)
	totalQueued   int64
	totalSpawned  int64
	totalTimedOut int64
	totalRejected int64

	// Request ID counter
	requestCounter int64
}

// NewSpawnQueue creates a new spawn queue.
func NewSpawnQueue(sm *ShardManager, le *core.LimitsEnforcer, cfg SpawnQueueConfig) *SpawnQueue {
	// Apply defaults for zero values
	if cfg.MaxQueueSize <= 0 {
		cfg.MaxQueueSize = 100
	}
	if cfg.MaxQueuePerPriority <= 0 {
		cfg.MaxQueuePerPriority = 30
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 5 * time.Minute
	}
	if cfg.HighWaterMark <= 0 || cfg.HighWaterMark > 1 {
		cfg.HighWaterMark = 0.7
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 2
	}
	if cfg.DrainTimeout <= 0 {
		cfg.DrainTimeout = 30 * time.Second
	}

	sq := &SpawnQueue{
		config:         cfg,
		shardManager:   sm,
		limitsEnforcer: le,
		stopCh:         make(chan struct{}),
	}

	// Initialize priority queues
	for i := 0; i < 4; i++ {
		sq.queues[i] = make(chan *spawnRequestWrapper, cfg.MaxQueuePerPriority)
	}

	return sq
}

// Start begins processing the queue.
func (sq *SpawnQueue) Start() error {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	if sq.isRunning {
		return nil
	}

	sq.isRunning = true
	sq.stopCh = make(chan struct{})

	// Start worker goroutines
	for i := 0; i < sq.config.WorkerCount; i++ {
		sq.workerWg.Add(1)
		go sq.worker(i)
	}

	logging.Shards("SpawnQueue: started with %d workers, max_queue=%d, high_water=%.0f%%",
		sq.config.WorkerCount, sq.config.MaxQueueSize, sq.config.HighWaterMark*100)

	return nil
}

// Stop gracefully shuts down the queue.
func (sq *SpawnQueue) Stop() error {
	sq.mu.Lock()
	if !sq.isRunning {
		sq.mu.Unlock()
		return nil
	}
	sq.isRunning = false
	close(sq.stopCh)
	sq.mu.Unlock()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		sq.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logging.Shards("SpawnQueue: stopped gracefully")
	case <-time.After(sq.config.DrainTimeout):
		logging.Get(logging.CategoryShards).Warn("SpawnQueue: drain timeout exceeded, some requests may be lost")
	}

	// Drain remaining requests with errors
	for i := 0; i < 4; i++ {
		for {
			select {
			case req := <-sq.queues[i]:
				sq.sendResult(req, SpawnResult{
					Error:  ErrQueueStopped,
					Queued: time.Since(req.SubmittedAt),
				})
			default:
				goto nextQueue
			}
		}
	nextQueue:
	}

	return nil
}

// Submit submits a spawn request to the queue.
// Returns immediately with a channel that will receive the result.
func (sq *SpawnQueue) Submit(ctx context.Context, typeName, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority, deadline time.Time, detached bool) (<-chan SpawnResult, error) {
	sq.mu.RLock()
	if !sq.isRunning {
		sq.mu.RUnlock()
		return nil, ErrQueueStopped
	}
	sq.mu.RUnlock()

	// Check capacity
	can, reason := sq.CanAccept(priority)
	if !can {
		atomic.AddInt64(&sq.totalRejected, 1)
		return nil, fmt.Errorf("%w: %s", ErrQueueFull, reason)
	}

	// Set up request
	req := spawnRequestWrapper{
		ID:          fmt.Sprintf("spawn-%d", atomic.AddInt64(&sq.requestCounter, 1)),
		TypeName:    typeName,
		Task:        task,
		SessionCtx:  sessionCtx,
		Priority:    priority,
		SubmittedAt: time.Now(),
		ResultCh:    make(chan SpawnResult, 1),
		Ctx:         ctx,
		Detached:    detached,
		Deadline:    deadline,
	}

	// Apply default timeout if not set
	if req.Deadline.IsZero() {
		deadline, ok := ctx.Deadline()
		if ok {
			req.Deadline = deadline
		} else {
			req.Deadline = time.Now().Add(sq.config.DefaultTimeout)
		}
	}

	// Submit to appropriate priority queue
	select {
	case sq.queues[req.Priority] <- &req:
		atomic.AddInt64(&sq.totalQueued, 1)
		logging.ShardsDebug("SpawnQueue: queued request %s (type=%s, priority=%s)",
			req.ID, req.TypeName, req.Priority)
		return req.ResultCh, nil
	default:
		// Priority queue is full
		atomic.AddInt64(&sq.totalRejected, 1)
		return nil, fmt.Errorf("%w: priority %s queue full", ErrQueueFull, req.Priority)
	}
}

// SubmitAndWait submits and blocks until result or timeout.
func (sq *SpawnQueue) SubmitAndWait(ctx context.Context, typeName, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority) (SpawnResult, error) {
	resultCh, err := sq.Submit(ctx, typeName, task, sessionCtx, priority, time.Time{}, false)
	if err != nil {
		return SpawnResult{}, err
	}

	select {
	case result := <-resultCh:
		return result, result.Error
	case <-ctx.Done():
		return SpawnResult{Error: ctx.Err()}, ctx.Err()
	}
}

// worker logic
func (sq *SpawnQueue) worker(id int) {
	defer sq.workerWg.Done()
	logging.ShardsDebug("SpawnQueue: worker %d started", id)

	for {
		select {
		case <-sq.stopCh:
			logging.ShardsDebug("SpawnQueue: worker %d stopping", id)
			return
		default:
			req := sq.selectNextRequest()
			if req == nil {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			sq.processRequest(id, req)
		}
	}
}

func (sq *SpawnQueue) selectNextRequest() *spawnRequestWrapper {
	// Check from highest to lowest priority
	for pri := types.PriorityCritical; pri >= types.PriorityLow; pri-- {
		select {
		case req := <-sq.queues[pri]:
			return req
		default:
			continue
		}
	}
	return nil
}

func (sq *SpawnQueue) processRequest(workerID int, req *spawnRequestWrapper) {
	queuedDuration := time.Since(req.SubmittedAt)

	logging.ShardsDebug("SpawnQueue: worker %d processing request %s (type=%s, priority=%s, queued=%v)",
		workerID, req.ID, req.TypeName, req.Priority, queuedDuration)

	// Check validity
	if err := req.Ctx.Err(); err != nil {
		atomic.AddInt64(&sq.totalTimedOut, 1)
		sq.sendResult(req, SpawnResult{
			Error:  fmt.Errorf("request cancelled while queued: %w", err),
			Queued: queuedDuration,
		})
		return
	}

	if !req.Deadline.IsZero() && time.Now().After(req.Deadline) {
		atomic.AddInt64(&sq.totalTimedOut, 1)
		sq.sendResult(req, SpawnResult{
			Error:  ErrQueueTimeout,
			Queued: queuedDuration,
		})
		logging.Get(logging.CategoryShards).Warn("SpawnQueue: request %s timed out after %v in queue",
			req.ID, queuedDuration)
		return
	}

	// Wait for slot
	if err := sq.waitForSlot(req.Ctx, req.Deadline); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			atomic.AddInt64(&sq.totalTimedOut, 1)
		}
		sq.sendResult(req, SpawnResult{
			Error:  fmt.Errorf("waiting for slot: %w", err),
			Queued: time.Since(req.SubmittedAt),
		})
		return
	}

	// Spawn
	shardID, err := sq.shardManager.SpawnAsyncWithContext(req.Ctx, req.TypeName, req.Task, req.SessionCtx)
	if err != nil {
		sq.sendResult(req, SpawnResult{
			Error:  fmt.Errorf("spawn failed: %w", err),
			Queued: time.Since(req.SubmittedAt),
		})
		return
	}

	if req.Detached {
		atomic.AddInt64(&sq.totalSpawned, 1)
		sq.sendResult(req, SpawnResult{
			ShardID: shardID,
			Result:  "Shard started (detached)",
			Queued:  time.Since(req.SubmittedAt),
		})
		return
	}

	// Wait for completion
	result := sq.waitForShardCompletion(req.Ctx, shardID)
	result.Queued = time.Since(req.SubmittedAt)
	atomic.AddInt64(&sq.totalSpawned, 1)
	sq.sendResult(req, result)
	logging.ShardsDebug("SpawnQueue: request %s completed (shard=%s, queued=%v)", req.ID, shardID, result.Queued)
}

func (sq *SpawnQueue) waitForSlot(ctx context.Context, deadline time.Time) error {
	backoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second
	waitStart := time.Now()

	for {
		if sq.CanSpawnNow() {
			return nil
		}

		if !deadline.IsZero() && time.Now().After(deadline) {
			return ErrQueueTimeout
		}

		waitTime := backoff
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining < waitTime {
				waitTime = remaining
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (sq *SpawnQueue) waitForShardCompletion(ctx context.Context, shardID string) SpawnResult {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return SpawnResult{ShardID: shardID, Error: ctx.Err()}
		case <-ticker.C:
			result, ok := sq.shardManager.GetResult(shardID)
			if ok {
				return SpawnResult{
					ShardID: shardID,
					Result:  result.Result,
					Error:   result.Error,
				}
			}
		}
	}
}

func (sq *SpawnQueue) sendResult(req *spawnRequestWrapper, result SpawnResult) {
	select {
	case req.ResultCh <- result:
	default:
		logging.Get(logging.CategoryShards).Warn("SpawnQueue: could not send result for request %s", req.ID)
	}
}

// API methods
func (sq *SpawnQueue) GetBackpressureStatus() BackpressureStatus {
	depth := sq.GetQueueDepth()
	utilization := float64(depth) / float64(sq.config.MaxQueueSize)
	slots := sq.GetAvailableSlots()
	accepting := true
	reason := ""
	if utilization >= 1.0 {
		accepting = false
		reason = "queue full"
	} else if slots == 0 && utilization > sq.config.HighWaterMark {
		accepting = false
		reason = "no slots available and queue at high water mark"
	}
	return BackpressureStatus{
		QueueDepth:       depth,
		QueueUtilization: utilization,
		AvailableSlots:   slots,
		Accepting:        accepting,
		Reason:           reason,
	}
}

func (sq *SpawnQueue) CanAccept(priority types.SpawnPriority) (bool, string) {
	depth := sq.GetQueueDepth()
	utilization := float64(depth) / float64(sq.config.MaxQueueSize)

	if depth >= sq.config.MaxQueueSize {
		return false, "total queue capacity reached"
	}
	if len(sq.queues[priority]) >= sq.config.MaxQueuePerPriority {
		return false, fmt.Sprintf("%s priority queue full", priority)
	}

	switch {
	case utilization > 0.9:
		if priority < types.PriorityCritical {
			return false, "queue >90% full, only critical requests accepted"
		}
	case utilization > sq.config.HighWaterMark:
		if priority == types.PriorityLow {
			return false, fmt.Sprintf("queue >%.0f%% full, low priority rejected", sq.config.HighWaterMark*100)
		}
	}
	return true, ""
}

func (sq *SpawnQueue) GetQueueDepth() int {
	total := 0
	for i := 0; i < 4; i++ {
		total += len(sq.queues[i])
	}
	return total
}

func (sq *SpawnQueue) IsRunning() bool {
	sq.mu.RLock()
	defer sq.mu.RUnlock()
	return sq.isRunning
}

func (sq *SpawnQueue) GetAvailableSlots() int {
	if sq.limitsEnforcer == nil {
		return 10
	}
	activeCount := 0
	if sq.shardManager != nil {
		activeCount = sq.shardManager.GetActiveNonSystemShardCount()
	}
	return sq.limitsEnforcer.GetAvailableShardSlots(activeCount)
}

func (sq *SpawnQueue) CanSpawnNow() bool {
	if sq.limitsEnforcer == nil {
		return true
	}
	activeCount := 0
	if sq.shardManager != nil {
		activeCount = sq.shardManager.GetActiveNonSystemShardCount()
	}
	if err := sq.limitsEnforcer.CheckShardLimit(activeCount); err != nil {
		return false
	}
	if err := sq.limitsEnforcer.CheckMemory(); err != nil {
		return false
	}
	return true
}

// SpawnQueueMetrics provides observability into queue state.
type SpawnQueueMetrics struct {
	QueueDepthByPriority [4]int
	TotalQueued          int64
	TotalSpawned         int64
	TotalTimedOut        int64
	TotalRejected        int64
	CurrentUtilization   float64
	IsRunning            bool
}

func (sq *SpawnQueue) GetMetrics() SpawnQueueMetrics {
	sq.mu.RLock()
	running := sq.isRunning
	sq.mu.RUnlock()

	metrics := SpawnQueueMetrics{
		TotalQueued:   atomic.LoadInt64(&sq.totalQueued),
		TotalSpawned:  atomic.LoadInt64(&sq.totalSpawned),
		TotalTimedOut: atomic.LoadInt64(&sq.totalTimedOut),
		TotalRejected: atomic.LoadInt64(&sq.totalRejected),
		IsRunning:     running,
	}
	for i := 0; i < 4; i++ {
		metrics.QueueDepthByPriority[i] = len(sq.queues[i])
	}
	metrics.CurrentUtilization = float64(sq.GetQueueDepth()) / float64(sq.config.MaxQueueSize)
	return metrics
}
