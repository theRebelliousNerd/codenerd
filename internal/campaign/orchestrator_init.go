package campaign

import (
	"codenerd/internal/logging"
	"codenerd/internal/perception"
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
	o.contextPager = NewContextPager(cfg.Kernel, cfg.LLMClient)
	o.checkpoint = NewCheckpointRunner(cfg.Executor, cfg.ShardManager, cfg.Workspace)
	o.replanner = NewReplanner(cfg.Kernel, cfg.LLMClient)
	o.decomposer = NewDecomposer(cfg.Kernel, cfg.LLMClient, cfg.Workspace)
	o.decomposer.SetShardLister(cfg.ShardManager) // Enable shard-aware planning
	o.transducer = perception.NewRealTransducer(cfg.LLMClient)

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
