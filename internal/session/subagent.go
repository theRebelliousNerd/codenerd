package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// SubAgentState represents the lifecycle state of a subagent.
type SubAgentState int32

const (
	SubAgentStateIdle SubAgentState = iota
	SubAgentStateRunning
	SubAgentStateCompleted
	SubAgentStateFailed
)

func (s SubAgentState) String() string {
	switch s {
	case SubAgentStateIdle:
		return "idle"
	case SubAgentStateRunning:
		return "running"
	case SubAgentStateCompleted:
		return "completed"
	case SubAgentStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// SubAgentType determines the persistence and lifecycle behavior.
type SubAgentType string

// Compressor defines the interface for memory compression.
// This allows the session package to work with compressors without importing internal/context.
type Compressor interface {
	Compress(ctx context.Context, turns []perception.ConversationTurn) (string, error)
}

const (
	// SubAgentTypeEphemeral dies after task completion.
	SubAgentTypeEphemeral SubAgentType = "ephemeral"

	// SubAgentTypePersistent survives across sessions with compressed memory.
	SubAgentTypePersistent SubAgentType = "persistent"

	// SubAgentTypeSystem runs continuously in the background.
	SubAgentTypeSystem SubAgentType = "system"
)

// SubAgentConfig configures a subagent's behavior.
type SubAgentConfig struct {
	// ID is the unique identifier for this subagent instance.
	ID string

	// Name is the human-readable name (e.g., "coder", "my-specialist").
	Name string

	// Type determines lifecycle behavior.
	Type SubAgentType

	// AgentConfig from JIT provides identity, tools, and policies.
	AgentConfig *config.AgentConfig

	// Timeout for the entire subagent execution.
	Timeout time.Duration

	// MaxTurns limits conversation turns before forcing completion.
	MaxTurns int

	// SessionContext provides shared state (e.g., DreamMode, Blackboard)
	SessionContext *types.SessionContext
}

// DefaultSubAgentConfig returns sensible defaults.
func DefaultSubAgentConfig(name string) SubAgentConfig {
	return SubAgentConfig{
		ID:       fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		Name:     name,
		Type:     SubAgentTypeEphemeral,
		Timeout:  30 * time.Minute,
		MaxTurns: 100,
	}
}

// SubAgent is a context-isolated instance of the clean execution loop.
//
// Each subagent has:
//   - Its own LLM conversation history (context isolation)
//   - JIT-provided identity and tools (no hardcoded behavior)
//   - Memory compression for long-running tasks
//
// SubAgents replace the old shard architecture. The difference:
//   - OLD: CoderShard with 600 lines of hardcoded Go
//   - NEW: SubAgent with same ~50-line loop, all behavior from JIT
type SubAgent struct {
	mu sync.RWMutex

	// Configuration
	config SubAgentConfig

	// Execution components
	executor *Executor // Uses the same clean loop

	// Context isolation - own conversation history
	conversationHistory []perception.ConversationTurn

	// Memory compression for long-running agents
	compressor Compressor

	// State tracking
	state     int32 // atomic SubAgentState
	startTime time.Time
	endTime   time.Time
	turnCount int

	// Results
	result string
	err    error

	// Cancellation
	cancel context.CancelFunc
}

// NewSubAgent creates a new subagent with the given configuration.
func NewSubAgent(
	cfg SubAgentConfig,
	kernel types.Kernel,
	virtualStore types.VirtualStore,
	llmClient types.LLMClient,
	jitCompiler JITCompiler,
	configFactory ConfigFactory,
	transducer perception.Transducer,
) *SubAgent {
	logging.Session("Creating SubAgent: %s (type: %s)", cfg.Name, cfg.Type)

	// Create executor for this subagent
	executor := NewExecutor(kernel, virtualStore, llmClient, jitCompiler, configFactory, transducer)

	// Set session context if provided in config
	if cfg.SessionContext != nil {
		executor.SetSessionContext(cfg.SessionContext)
	}

	// Apply agent config to executor if provided
	if cfg.AgentConfig != nil {
		// The executor will use this config for all operations
		logging.SessionDebug("SubAgent %s configured with %d tools", cfg.Name, len(cfg.AgentConfig.Tools.AllowedTools))
	}

	return &SubAgent{
		config:              cfg,
		executor:            executor,
		conversationHistory: make([]perception.ConversationTurn, 0),
		compressor:          NewSemanticCompressor(llmClient),
		state:               int32(SubAgentStateIdle),
	}
}

// GetID returns the subagent's unique identifier.
func (s *SubAgent) GetID() string {
	return s.config.ID
}

// GetName returns the subagent's name.
func (s *SubAgent) GetName() string {
	return s.config.Name
}

// GetState returns the current state.
func (s *SubAgent) GetState() SubAgentState {
	return SubAgentState(atomic.LoadInt32(&s.state))
}

// GetResult returns the final result and error after completion.
func (s *SubAgent) GetResult() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.result, s.err
}

// Run executes the subagent's task asynchronously.
// Returns immediately; use Wait() or GetResult() for results.
func (s *SubAgent) Run(ctx context.Context, task string) {
	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.startTime = time.Now()
	s.mu.Unlock()

	// Per-agent capability hint for shared LLM clients (e.g. Codex CLI reasoning multiplexing).
	// Only set if the caller didn't provide an explicit hint.
	if ctx.Value(types.CtxKeyModelCapability) == nil {
		ctx = context.WithValue(ctx, types.CtxKeyModelCapability, capabilityHintForAgentName(s.config.Name))
	}

	// Apply timeout
	if s.config.Timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, s.config.Timeout)
		defer timeoutCancel()
	}

	atomic.StoreInt32(&s.state, int32(SubAgentStateRunning))
	logging.Session("SubAgent %s starting task: %s", s.config.Name, truncateTask(task))

	// Run the task
	result, err := s.execute(ctx, task)

	// Store results
	s.mu.Lock()
	s.result = result
	s.err = err
	s.endTime = time.Now()
	s.mu.Unlock()

	if err != nil {
		atomic.StoreInt32(&s.state, int32(SubAgentStateFailed))
		logging.Get(logging.CategorySession).Error("SubAgent %s failed: %v", s.config.Name, err)
	} else {
		atomic.StoreInt32(&s.state, int32(SubAgentStateCompleted))
		logging.Session("SubAgent %s completed successfully", s.config.Name)
	}
}

// execute runs the clean loop for this subagent.
func (s *SubAgent) execute(ctx context.Context, task string) (string, error) {
	// For single-turn execution, just process the task
	result, err := s.executor.Process(ctx, task)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.turnCount++
	s.mu.Unlock()

	return result.Response, nil
}

func capabilityHintForAgentName(name string) types.ModelCapability {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "coder", "reviewer", "nemesis", "legislator":
		return types.CapabilityHighReasoning
	case "tester":
		return types.CapabilityHighSpeed
	case "researcher", "librarian", "planner":
		return types.CapabilityBalanced
	default:
		// Custom specialists typically benefit from deeper reasoning.
		return types.CapabilityHighReasoning
	}
}

// Stop cancels the subagent's execution.
func (s *SubAgent) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
		logging.Session("SubAgent %s stop requested", s.config.Name)
	}

	return nil
}

// Wait blocks until the subagent completes.
func (s *SubAgent) Wait() (string, error) {
	// Poll for completion
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		state := s.GetState()
		if state == SubAgentStateCompleted || state == SubAgentStateFailed {
			return s.GetResult()
		}
		<-ticker.C
	}
}

// GetMetrics returns execution metrics for this subagent.
func (s *SubAgent) GetMetrics() SubAgentMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	duration := time.Duration(0)
	if !s.startTime.IsZero() {
		if !s.endTime.IsZero() {
			duration = s.endTime.Sub(s.startTime)
		} else {
			duration = time.Since(s.startTime)
		}
	}

	return SubAgentMetrics{
		ID:        s.config.ID,
		Name:      s.config.Name,
		Type:      s.config.Type,
		State:     s.GetState(),
		TurnCount: s.turnCount,
		Duration:  duration,
	}
}

// SubAgentMetrics holds execution metrics.
type SubAgentMetrics struct {
	ID        string
	Name      string
	Type      SubAgentType
	State     SubAgentState
	TurnCount int
	Duration  time.Duration
}

// CompressMemory compresses the conversation history if it exceeds threshold.
func (s *SubAgent) CompressMemory(ctx context.Context, threshold int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.conversationHistory) <= threshold {
		return nil
	}

	if s.compressor == nil {
		// No compressor configured
		return nil
	}

	logging.SessionDebug("SubAgent %s compressing memory: %d turns (threshold: %d)", s.config.Name, len(s.conversationHistory), threshold)

	// Strategy:
	// 1. Keep the most recent (threshold/2) turns intact (recency bias).
	// 2. Compress the older turns into a single summary.
	// 3. New History = [Summary] + [Recent Turns]

	// Determine split point
	keepCount := threshold / 2
	if keepCount < 1 {
		keepCount = 1
	}

	// Index of the first item to KEEP
	splitIndex := len(s.conversationHistory) - keepCount
	if splitIndex <= 0 {
		return nil // Should be covered by initial check, but safety first
	}

	// Slice and dice
	toCompress := s.conversationHistory[:splitIndex]
	recentTurns := s.conversationHistory[splitIndex:]

	// Run compression
	summary, err := s.compressor.Compress(ctx, toCompress)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("SubAgent %s memory compression failed: %v", s.config.Name, err)
		// Fallback: simple trim to threshold
		s.conversationHistory = s.conversationHistory[len(s.conversationHistory)-threshold:]
		return nil
	}

	// Create summary turn
	// We use "assistant" role with a specific prefix so the model understands this is context
	summaryTurn := perception.ConversationTurn{
		Role:    "assistant",
		Content: fmt.Sprintf("[MEMORY SUMMARY] Previous context: %s", summary),
	}

	// Reconstruct history
	newHistory := make([]perception.ConversationTurn, 0, len(recentTurns)+1)
	newHistory = append(newHistory, summaryTurn)
	newHistory = append(newHistory, recentTurns...)

	s.conversationHistory = newHistory
	logging.Session("SubAgent %s memory compressed: %d -> %d turns", s.config.Name, len(toCompress)+len(recentTurns), len(newHistory))

	return nil
}

// SetCompressor sets the memory compressor for long-running agents.
func (s *SubAgent) SetCompressor(c Compressor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compressor = c
}

// truncateTask returns a shortened version of the task for logging.
func truncateTask(task string) string {
	const maxLen = 100
	if len(task) <= maxLen {
		return task
	}
	return task[:maxLen] + "..."
}
