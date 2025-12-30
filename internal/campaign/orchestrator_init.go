package campaign

import (
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"path/filepath"
	"time"
)

// NewOrchestrator creates a new campaign orchestrator.
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	timer := logging.StartTimer(logging.CategoryCampaign, "NewOrchestrator")
	defer timer.Stop()

	nerdDir := filepath.Join(cfg.Workspace, ".nerd")

	// Apply timeout defaults unless explicitly disabled.
	if cfg.DisableTimeouts {
		cfg.CampaignTimeout = 0
		cfg.TaskTimeout = 0
	} else {
		if cfg.CampaignTimeout == 0 {
			cfg.CampaignTimeout = 4 * time.Hour
		}
		if cfg.TaskTimeout == 0 {
			cfg.TaskTimeout = 30 * time.Minute
		}
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.ReplanThreshold == 0 {
		cfg.ReplanThreshold = 3
	}
	if cfg.HeartbeatEvery == 0 {
		cfg.HeartbeatEvery = 15 * time.Second
	}
	if cfg.AutosaveEvery == 0 {
		cfg.AutosaveEvery = time.Minute
	}
	if cfg.TaskResultCacheLimit == 0 {
		cfg.TaskResultCacheLimit = 100
	}
	if cfg.RetryBackoffBase == 0 {
		cfg.RetryBackoffBase = 5 * time.Second
	}
	if cfg.RetryBackoffMax == 0 {
		cfg.RetryBackoffMax = 5 * time.Minute
	}

	logging.Campaign("Initializing campaign orchestrator for workspace: %s", cfg.Workspace)
	logging.CampaignDebug("Orchestrator config: maxParallel=%d, checkpointOnFail=%v, autoReplan=%v, campaignTimeout=%v, taskTimeout=%v",
		cfg.MaxParallelTasks, cfg.CheckpointOnFail, cfg.AutoReplan, cfg.CampaignTimeout, cfg.TaskTimeout)

	o := &Orchestrator{
		kernel:           cfg.Kernel,
		llmClient:        cfg.LLMClient,
		shardMgr:         cfg.ShardManager,
		taskExecutor:     cfg.TaskExecutor,
		executor:         cfg.Executor,
		virtualStore:     cfg.VirtualStore,
		workspace:        cfg.Workspace,
		nerdDir:          nerdDir,
		progressChan:     cfg.ProgressChan,
		eventChan:        cfg.EventChan,
		maxParallelTasks: defaultParallelTasks,
		taskResults:      make(map[string]string),
		taskResultOrder:  make([]string, 0),
		config:           cfg,
		promptProvider:   NewStaticPromptProvider(),
	}

	// Initialize sub-components
	o.contextPager = NewContextPager(cfg.Kernel, cfg.LLMClient, cfg.ContextBudget)
	o.checkpoint = NewCheckpointRunner(cfg.Executor, cfg.ShardManager, cfg.Workspace)
	o.replanner = NewReplanner(cfg.Kernel, cfg.LLMClient)
	o.decomposer = NewDecomposer(cfg.Kernel, cfg.LLMClient, cfg.Workspace)
	o.decomposer.SetShardLister(cfg.ShardManager) // Enable shard-aware planning
	o.transducer = perception.NewRealTransducer(cfg.LLMClient)

	// Wire intelligence integration components
	o.intelligenceGatherer = cfg.IntelligenceGatherer
	o.advisoryBoard = cfg.AdvisoryBoard
	o.edgeCaseDetector = cfg.EdgeCaseDetector
	o.toolPregenerator = cfg.ToolPregenerator

	// Wire intelligence components into decomposer for campaign planning
	if cfg.IntelligenceGatherer != nil {
		o.decomposer.SetIntelligenceGatherer(cfg.IntelligenceGatherer)
		logging.CampaignDebug("IntelligenceGatherer wired into decomposer")
	}
	if cfg.AdvisoryBoard != nil {
		o.decomposer.SetAdvisoryBoard(cfg.AdvisoryBoard)
		logging.CampaignDebug("AdvisoryBoard wired into decomposer")
	}
	if cfg.EdgeCaseDetector != nil {
		o.decomposer.SetEdgeCaseDetector(cfg.EdgeCaseDetector)
		logging.CampaignDebug("EdgeCaseDetector wired into decomposer")
	}
	if cfg.ToolPregenerator != nil {
		o.decomposer.SetToolPregenerator(cfg.ToolPregenerator)
		logging.CampaignDebug("ToolPregenerator wired into decomposer")
	}

	if cfg.MaxParallelTasks > 0 {
		o.maxParallelTasks = cfg.MaxParallelTasks
	}

	logging.Campaign("Orchestrator initialized with maxParallelTasks=%d, campaignTimeout=%v, taskTimeout=%v",
		o.maxParallelTasks, o.config.CampaignTimeout, o.config.TaskTimeout)

	return o
}

// SetPromptProvider wires a PromptProvider into the orchestrator's planning components.
// This enables JIT-compiled prompts for decomposition and replanning.
func (o *Orchestrator) SetPromptProvider(provider PromptProvider) {
	if provider == nil {
		return
	}
	o.promptProvider = provider
	if o.decomposer != nil {
		o.decomposer.SetPromptProvider(provider)
	}
	if o.replanner != nil {
		o.replanner.SetPromptProvider(provider)
	}
}

// SetTaskExecutor sets the task executor for JIT-driven task execution.
// When set, the orchestrator will prefer TaskExecutor over ShardManager.
//
// This is part of the migration from shards to the clean execution loop.
// Usage:
//
//	orch := campaign.NewOrchestrator(cfg)
//	orch.SetTaskExecutor(session.NewJITExecutor(executor, spawner, transducer))
func (o *Orchestrator) SetTaskExecutor(te session.TaskExecutor) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.taskExecutor = te
	logging.Campaign("TaskExecutor set on orchestrator")
}

// SetSpecialistKnowledgeProvider sets the provider for specialist knowledge injection.
// When set, the orchestrator will inject relevant knowledge from specialist databases
// into task context, enabling specialists to leverage their domain expertise.
func (o *Orchestrator) SetSpecialistKnowledgeProvider(provider SpecialistKnowledgeProvider) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.specialistKnowledgeProvider = provider
	logging.Campaign("SpecialistKnowledgeProvider set on orchestrator")
}

// SetNorthstarObserver sets the Northstar vision guardian observer.
// When set, the orchestrator will perform alignment checks at phase transitions,
// task completions, and other critical points during campaign execution.
//
// The observer monitors whether work remains aligned with the project vision,
// can block phases that drift from goals, and records observations for analysis.
func (o *Orchestrator) SetNorthstarObserver(observer *northstar.CampaignObserver) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.northstarObserver = observer
	logging.Campaign("NorthstarObserver set on orchestrator")
}

// SetIntelligenceGatherer sets the intelligence gatherer for pre-planning intelligence.
// When set, the decomposer will gather intelligence from 12 systems before planning.
func (o *Orchestrator) SetIntelligenceGatherer(gatherer *IntelligenceGatherer) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.intelligenceGatherer = gatherer
	if o.decomposer != nil {
		o.decomposer.SetIntelligenceGatherer(gatherer)
	}
	logging.Campaign("IntelligenceGatherer set on orchestrator")
}

// SetAdvisoryBoard sets the shard advisory board for plan review.
// When set, domain experts will review plans before execution.
func (o *Orchestrator) SetAdvisoryBoard(board *ShardAdvisoryBoard) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.advisoryBoard = board
	if o.decomposer != nil {
		o.decomposer.SetAdvisoryBoard(board)
	}
	logging.Campaign("AdvisoryBoard set on orchestrator")
}

// SetEdgeCaseDetector sets the edge case detector for file action decisions.
// When set, the decomposer will analyze files to determine create/extend/modularize actions.
func (o *Orchestrator) SetEdgeCaseDetector(detector *EdgeCaseDetector) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.edgeCaseDetector = detector
	if o.decomposer != nil {
		o.decomposer.SetEdgeCaseDetector(detector)
	}
	logging.Campaign("EdgeCaseDetector set on orchestrator")
}

// SetToolPregenerator sets the tool pregenerator for pre-execution tool generation.
// When set, the decomposer will generate missing tools before campaign execution.
func (o *Orchestrator) SetToolPregenerator(pregenerator *ToolPregenerator) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.toolPregenerator = pregenerator
	if o.decomposer != nil {
		o.decomposer.SetToolPregenerator(pregenerator)
	}
	logging.Campaign("ToolPregenerator set on orchestrator")
}
