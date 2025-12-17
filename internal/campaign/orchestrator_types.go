package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/tactile"
	"context"
	"sync"
	"time"
)

const defaultParallelTasks = 3

// Orchestrator runs the campaign execution loop.
// It manages phase transitions, task execution, context paging, and checkpoints.
type Orchestrator struct {
	mu sync.RWMutex

	// Core components
	kernel       *core.RealKernel
	llmClient    perception.LLMClient
	shardMgr     *core.ShardManager
	executor     *tactile.SafeExecutor
	virtualStore *core.VirtualStore
	transducer   *perception.RealTransducer

	// Campaign-specific components
	contextPager   *ContextPager
	checkpoint     *CheckpointRunner
	replanner      *Replanner
	decomposer     *Decomposer
	promptProvider PromptProvider

	// State
	campaign     *Campaign
	workspace    string
	nerdDir      string
	progressChan chan Progress
	eventChan    chan OrchestratorEvent
	lastPhaseID  string

	// Execution tracking
	isRunning  bool
	isPaused   bool
	cancelFunc context.CancelFunc
	lastError  error

	// Task result storage for context injection between tasks
	taskResults map[string]string
	resultsMu   sync.RWMutex
	// Insertion/LRU order for pruning task results
	taskResultOrder []string

	// Concurrency control
	maxParallelTasks int

	// Configuration (including timeouts)
	config OrchestratorConfig
}

// OrchestratorEvent represents an event during campaign execution.
type OrchestratorEvent struct {
	Type      string    `json:"type"` // task_started, task_completed, task_failed, phase_completed, checkpoint, replan, learning
	Timestamp time.Time `json:"timestamp"`
	PhaseID   string    `json:"phase_id,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	Message   string    `json:"message"`
	Data      any       `json:"data,omitempty"`
}

// OrchestratorConfig holds configuration for the orchestrator.
type OrchestratorConfig struct {
	Workspace            string
	Kernel               *core.RealKernel
	LLMClient            perception.LLMClient
	ShardManager         *core.ShardManager
	Executor             *tactile.SafeExecutor
	VirtualStore         *core.VirtualStore
	ProgressChan         chan Progress
	EventChan            chan OrchestratorEvent
	MaxRetries           int           // Max retries per task (default 3)
	CheckpointOnFail     bool          // Run checkpoint after task failure
	AutoReplan           bool          // Auto-replan on too many failures
	ReplanThreshold      int           // Failures before replan (default 3)
	MaxParallelTasks     int           // Max tasks to run in parallel (default 3)
	CampaignTimeout      time.Duration // Max total campaign runtime (default: 4 hours)
	TaskTimeout          time.Duration // Max time per task (default: 30 minutes)
	DisableTimeouts      bool          // Disable all timeouts for long-horizon campaigns
	HeartbeatEvery       time.Duration // Emit heartbeat/progress every N duration (default: 15s)
	AutosaveEvery        time.Duration // Persist campaign every N duration (default: 1m)
	TaskResultCacheLimit int           // Max task results kept for context injection (default: 100)
	RetryBackoffBase     time.Duration // Base backoff between retries (default: 5s)
	RetryBackoffMax      time.Duration // Max backoff between retries (default: 5m)
}

// taskResult is used to collect async task outcomes in runPhase.
type taskResult struct {
	taskID string
	result any
	err    error
}
