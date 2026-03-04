package campaign

import (
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const defaultParallelTasks = 3

// Orchestrator runs the campaign execution loop.
// It manages phase transitions, task execution, context paging, and checkpoints.
type Orchestrator struct {
	mu sync.RWMutex

	// Core components
	kernel       core.Kernel
	llmClient    perception.LLMClient
	shardMgr     *coreshards.ShardManager // For monitoring (GetActiveShards, GetBackpressureStatus). Use taskExecutor for task execution.
	taskExecutor session.TaskExecutor     // For task execution (replaces direct shardMgr.Spawn calls)
	executor     tactile.Executor
	virtualStore *core.VirtualStore
	transducer   perception.Transducer

	// Campaign-specific components
	contextPager                *ContextPager
	checkpoint                  *CheckpointRunner
	replanner                   *Replanner
	decomposer                  *Decomposer
	promptProvider              PromptProvider
	specialistKnowledgeProvider SpecialistKnowledgeProvider
	northstarObserver           *northstar.CampaignObserver

	// Intelligence integration (Campaign Intelligence Plan)
	intelligenceGatherer *IntelligenceGatherer // Pre-planning intelligence from 12 systems
	advisoryBoard        *ShardAdvisoryBoard   // Domain expert consultation
	edgeCaseDetector     *EdgeCaseDetector     // File action decisions
	toolPregenerator     *ToolPregenerator     // Tool pre-generation via Ouroboros

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

	// Deterministic write-set locking for mutating tasks.
	writeSetLocks *writeSetLockManager

	// Durable journal sequence for event-before-ack persistence.
	journalSeq atomic.Uint64

	// Cached campaign risk decision for deterministic auto-wiring.
	riskDecision *CampaignRiskDecision

	// Resolved gate wiring (auto + overrides) used by deterministic risk gating.
	riskGateState riskGateResolved

	// Preserves configured observer even when runtime gate disables northstar checks.
	configuredNorthstarObserver *northstar.CampaignObserver
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
	Kernel               core.Kernel
	LLMClient            perception.LLMClient
	Transducer           perception.Transducer    // Optional: Inject transducer for testing
	ShardManager         *coreshards.ShardManager // For monitoring. Use TaskExecutor for task execution.
	TaskExecutor         session.TaskExecutor     // For task execution (replaces direct ShardManager.Spawn calls)
	Executor             tactile.Executor
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
	ContextBudget        int           // Token budget for context pager (default: from config.ContextWindow.MaxTokens)
	WriteSetLockTimeout  time.Duration // Max wait to acquire write_set lock before timeout route (default: 15s)
	WriteSetLockRetry    time.Duration // Delay before retrying a timed-out write_set lock attempt (default: 500ms)
	WriteSetLockPoll     time.Duration // Poll interval while waiting for write_set lock (default: 10ms)

	// Deterministic risk gating.
	EnableRiskAutoWiring    bool            // Enable deterministic risk gate enforcement (default: true)
	RiskGateThreshold       int             // Score threshold to enable strict gates (default: 70)
	GlobalRiskGate          bool            // Global default gate toggle (default: true)
	CampaignRiskOverride    *bool           // Campaign-level gate override
	TaskRiskOverrides       map[string]bool // Task-level gate overrides (highest precedence)
	RiskGateMode            RiskGateMode    // /auto, /force_allow, /force_block (default: /auto)
	AdvisoryGateToggle      RiskGateToggle  // /auto, /enabled, /disabled (default: /auto)
	EdgeGateToggle          RiskGateToggle  // /auto, /enabled, /disabled (default: /auto)
	NorthstarGateToggle     RiskGateToggle  // /auto, /enabled, /disabled (default: /auto)
	RiskIntelligenceTimeout time.Duration   // Optional timeout for pre-run intelligence snapshot gathering

	// Intelligence integration (Campaign Intelligence Plan)
	IntelligenceGatherer *IntelligenceGatherer // Pre-planning intelligence from 12 systems
	AdvisoryBoard        *ShardAdvisoryBoard   // Domain expert consultation
	EdgeCaseDetector     *EdgeCaseDetector     // File action decisions
	ToolPregenerator     *ToolPregenerator     // Tool pre-generation via Ouroboros
}

// taskResult is used to collect async task outcomes in runPhase.
type taskResult struct {
	taskID string
	result any
	err    error
}
