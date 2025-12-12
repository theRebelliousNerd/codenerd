package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/tactile"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	contextPager *ContextPager
	checkpoint   *CheckpointRunner
	replanner    *Replanner
	decomposer   *Decomposer

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
	Workspace        string
	Kernel           *core.RealKernel
	LLMClient        perception.LLMClient
	ShardManager     *core.ShardManager
	Executor         *tactile.SafeExecutor
	VirtualStore     *core.VirtualStore
	ProgressChan     chan Progress
	EventChan        chan OrchestratorEvent
	MaxRetries       int           // Max retries per task (default 3)
	CheckpointOnFail bool          // Run checkpoint after task failure
	AutoReplan       bool          // Auto-replan on too many failures
	ReplanThreshold  int           // Failures before replan (default 3)
	MaxParallelTasks int           // Max tasks to run in parallel (default 3)
	CampaignTimeout  time.Duration // Max total campaign runtime (default: 4 hours)
	TaskTimeout      time.Duration // Max time per task (default: 30 minutes)
	DisableTimeouts  bool          // Disable all timeouts for long-horizon campaigns
	HeartbeatEvery   time.Duration // Emit heartbeat/progress every N duration (default: 15s)
	AutosaveEvery    time.Duration // Persist campaign every N duration (default: 1m)
	TaskResultCacheLimit int       // Max task results kept for context injection (default: 100)
	RetryBackoffBase time.Duration // Base backoff between retries (default: 5s)
	RetryBackoffMax  time.Duration // Max backoff between retries (default: 5m)
}

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
	if o.decomposer != nil {
		o.decomposer.SetPromptProvider(provider)
	}
	if o.replanner != nil {
		o.replanner.SetPromptProvider(provider)
	}
}

// LoadCampaign loads an existing campaign from disk.
func (o *Orchestrator) LoadCampaign(campaignID string) error {
	timer := logging.StartTimer(logging.CategoryCampaign, "LoadCampaign")
	defer timer.Stop()

	logging.Campaign("Loading campaign: %s", campaignID)

	o.mu.Lock()
	defer o.mu.Unlock()

	campaignPath := filepath.Join(o.nerdDir, "campaigns", campaignID+".json")
	logging.CampaignDebug("Reading campaign from: %s", campaignPath)

	data, err := os.ReadFile(campaignPath)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to read campaign file: %v", err)
		return fmt.Errorf("failed to load campaign: %w", err)
	}

	var campaign Campaign
	if err := json.Unmarshal(data, &campaign); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to parse campaign JSON: %v", err)
		return fmt.Errorf("failed to parse campaign: %w", err)
	}

	o.campaign = &campaign

	logging.Campaign("Campaign loaded: %s (title=%s, phases=%d, tasks=%d)",
		campaign.ID, campaign.Title, len(campaign.Phases), campaign.TotalTasks)

	// Load campaign facts into kernel
	facts := campaign.ToFacts()
	logging.CampaignDebug("Loading %d facts into kernel", len(facts))
	if err := o.kernel.LoadFacts(facts); err != nil {
		return err
	}
	// Apply runtime config + budget
	o.assertCampaignConfigFacts()
	if o.contextPager != nil && o.campaign.ContextBudget > 0 {
		o.contextPager.SetBudget(o.campaign.ContextBudget)
	}
	return nil
}

// SetCampaign sets the campaign to execute.
func (o *Orchestrator) SetCampaign(campaign *Campaign) error {
	logging.Campaign("Setting campaign: %s (title=%s)", campaign.ID, campaign.Title)

	o.mu.Lock()
	defer o.mu.Unlock()

	o.campaign = campaign

	// Load campaign facts into kernel
	facts := campaign.ToFacts()
	logging.CampaignDebug("Loading %d campaign facts into kernel", len(facts))
	if err := o.kernel.LoadFacts(facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to load campaign facts: %v", err)
		return err
	}
	// Apply runtime config + budget
	o.assertCampaignConfigFacts()
	if o.contextPager != nil && campaign.ContextBudget > 0 {
		o.contextPager.SetBudget(campaign.ContextBudget)
	}

	// Save campaign to disk
	logging.CampaignDebug("Persisting campaign to disk")
	return o.saveCampaign()
}

// saveCampaign persists the campaign to disk.
func (o *Orchestrator) saveCampaign() error {
	logging.CampaignDebug("Saving campaign to disk: %s", o.campaign.ID)
	campaignsDir := filepath.Join(o.nerdDir, "campaigns")
	if err := os.MkdirAll(campaignsDir, 0755); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to create campaigns directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(o.campaign, "", "  ")
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to marshal campaign JSON: %v", err)
		return err
	}

	campaignPath := filepath.Join(campaignsDir, o.campaign.ID+".json")
	if err := os.WriteFile(campaignPath, data, 0644); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to write campaign file: %v", err)
		return err
	}
	logging.CampaignDebug("Campaign saved successfully: %s (%d bytes)", campaignPath, len(data))
	return nil
}

// resetInProgress clears in-flight task/phase states after restarts so work can resume.
func (o *Orchestrator) resetInProgress() {
	logging.Campaign("Resetting in-progress states after restart")
	resetCount := 0

	for pi := range o.campaign.Phases {
		phase := &o.campaign.Phases[pi]
		if phase.Status == PhaseInProgress {
			logging.CampaignDebug("Resetting phase %s from in_progress to pending", phase.ID)
			phase.Status = PhasePending
			resetCount++
		}
		for ti := range phase.Tasks {
			task := &phase.Tasks[ti]
			if task.Status == TaskInProgress {
				logging.CampaignDebug("Resetting task %s from in_progress to pending", task.ID)
				task.Status = TaskPending
				resetCount++
				// Update kernel fact for the task
				_ = o.kernel.RetractFact(core.Fact{
					Predicate: "campaign_task",
					Args:      []interface{}{task.ID},
				})
				_ = o.kernel.Assert(core.Fact{
					Predicate: "campaign_task",
					Args:      []interface{}{task.ID, task.PhaseID, task.Description, string(TaskPending), string(task.Type)},
				})
			}
		}
	}

	logging.Campaign("Reset %d in-progress items", resetCount)
	_ = o.saveCampaign()
}

// Run executes the campaign until completion, pause, or failure.
func (o *Orchestrator) Run(ctx context.Context) error {
	runTimer := logging.StartTimer(logging.CategoryCampaign, "Run")

	o.mu.Lock()
	if o.campaign == nil {
		o.mu.Unlock()
		logging.Get(logging.CategoryCampaign).Error("Run called with no campaign loaded")
		return fmt.Errorf("no campaign loaded")
	}
	if o.isRunning {
		o.mu.Unlock()
		logging.Get(logging.CategoryCampaign).Warn("Campaign already running: %s", o.campaign.ID)
		return fmt.Errorf("campaign already running")
	}

	logging.Campaign("=== Starting campaign execution: %s ===", o.campaign.ID)
	logging.Campaign("Campaign: %s (type=%s, phases=%d, tasks=%d)",
		o.campaign.Title, o.campaign.Type, o.campaign.TotalPhases, o.campaign.TotalTasks)

	// Normalize any dangling in-progress tasks/phases (e.g., after restart)
	o.resetInProgress()

	// Set up cancellation
	ctx, cancel := context.WithCancel(ctx)
	o.cancelFunc = cancel
	o.isRunning = true
	o.isPaused = false
	o.updateCampaignStatus(StatusActive)
	o.mu.Unlock()

	// Apply campaign-level timeout
	if o.config.CampaignTimeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, o.config.CampaignTimeout)
		defer timeoutCancel()
		logging.Campaign("Campaign timeout set: %v", o.config.CampaignTimeout)
	}

	// Start heartbeat/autosave loop for long-running durability.
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go o.runHeartbeatLoop(heartbeatCtx)

	defer func() {
		o.mu.Lock()
		o.isRunning = false
		o.cancelFunc = nil
		o.mu.Unlock()
		runTimer.StopWithInfo()
	}()

	// Main execution loop
	loopCount := 0
	for {
		loopCount++
		logging.CampaignDebug("Execution loop iteration %d", loopCount)

		select {
		case <-ctx.Done():
			logging.Campaign("Campaign execution cancelled: %v", ctx.Err())
			o.mu.Lock()
			o.updateCampaignStatus(StatusPaused)
			_ = o.saveCampaign()
			o.mu.Unlock()
			return ctx.Err()
		default:
		}

		// Check if paused
		o.mu.RLock()
		paused := o.isPaused
		o.mu.RUnlock()
		if paused {
			logging.CampaignDebug("Campaign paused, waiting...")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 1. Query Mangle for current state
		currentPhase := o.getCurrentPhase()
		if currentPhase == nil {
			// Check if campaign is complete
			if o.isCampaignComplete() {
				logging.Campaign("=== Campaign completed successfully: %s ===", o.campaign.ID)
				logging.Campaign("Final stats: phases=%d/%d, tasks=%d/%d",
					o.campaign.CompletedPhases, o.campaign.TotalPhases,
					o.campaign.CompletedTasks, o.campaign.TotalTasks)
				o.mu.Lock()
				o.updateCampaignStatus(StatusCompleted)
				_ = o.saveCampaign()
				o.mu.Unlock()
				o.emitEvent("campaign_completed", "", "", "Campaign completed successfully", nil)
				return nil
			}

			// Check if blocked
			blockReason := o.getCampaignBlockReason()
			if blockReason != "" {
				logging.Get(logging.CategoryCampaign).Error("Campaign blocked: %s", blockReason)
				o.mu.Lock()
				o.updateCampaignStatus(StatusFailed)
				o.lastError = fmt.Errorf("campaign blocked: %s", blockReason)
				_ = o.saveCampaign()
				o.mu.Unlock()
				return o.lastError
			}

			// No current phase but not complete - start next eligible phase
			logging.CampaignDebug("No current phase, starting next eligible phase")
			if err := o.startNextPhase(ctx); err != nil {
				logging.Get(logging.CategoryCampaign).Warn("Failed to start next phase: %v", err)
				o.lastError = err
				continue
			}
			continue
		}

		logging.CampaignDebug("Current phase: %s (%s)", currentPhase.Name, currentPhase.ID)

		// 2. Page in context for current phase only on transition
		if o.contextPager != nil && currentPhase.ID != o.lastPhaseID {
			o.contextPager.ResetPhaseContext()
			if err := o.contextPager.ActivatePhase(ctx, currentPhase); err != nil {
				logging.Get(logging.CategoryCampaign).Warn("Context activation error: %v", err)
				o.emitEvent("context_error", currentPhase.ID, "", err.Error(), nil)
			}
			// Prefetch upcoming tasks for this phase
			var upcoming []Task
			for _, t := range currentPhase.Tasks {
				if t.Status == TaskPending {
					upcoming = append(upcoming, t)
				}
			}
			_ = o.contextPager.PrefetchNextTasks(ctx, upcoming, 3)
			o.lastPhaseID = currentPhase.ID
		}

		// 3. Execute the phase with parallelism + rolling checkpoints
		if err := o.runPhase(ctx, currentPhase); err != nil {
			logging.Get(logging.CategoryCampaign).Error("Phase execution error: %v", err)
			o.lastError = err
			if ctx.Err() != nil {
				return err
			}
		}
	}
}

// runHeartbeatLoop periodically emits progress, updates kernel heartbeat facts,
// and persists the campaign even when tasks are idle or blocked.
func (o *Orchestrator) runHeartbeatLoop(ctx context.Context) {
	heartbeatTicker := time.NewTicker(o.config.HeartbeatEvery)
	autosaveTicker := time.NewTicker(o.config.AutosaveEvery)
	defer heartbeatTicker.Stop()
	defer autosaveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			o.emitProgress()
			o.mu.RLock()
			campaignID := ""
			if o.campaign != nil {
				campaignID = o.campaign.ID
			}
			o.mu.RUnlock()
			if campaignID != "" && o.kernel != nil {
				_ = o.kernel.RetractFact(core.Fact{
					Predicate: "campaign_heartbeat",
					Args:      []interface{}{campaignID},
				})
				_ = o.kernel.Assert(core.Fact{
					Predicate: "campaign_heartbeat",
					Args:      []interface{}{campaignID, time.Now().Unix()},
				})
			}
		case <-autosaveTicker.C:
			o.mu.Lock()
			if o.campaign != nil {
				_ = o.saveCampaign()
			}
			o.mu.Unlock()
		}
	}
}

// Pause pauses campaign execution.
func (o *Orchestrator) Pause() {
	o.mu.Lock()
	defer o.mu.Unlock()
	logging.Campaign("Pausing campaign: %s", o.campaign.ID)
	o.isPaused = true
	o.updateCampaignStatus(StatusPaused)
	_ = o.saveCampaign()
}

// Resume resumes paused campaign execution.
func (o *Orchestrator) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	logging.Campaign("Resuming campaign: %s", o.campaign.ID)
	o.isPaused = false
	o.updateCampaignStatus(StatusActive)
}

// Stop stops campaign execution.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	logging.Campaign("Stopping campaign: %s", o.campaign.ID)
	if o.cancelFunc != nil {
		o.cancelFunc()
	}
	o.updateCampaignStatus(StatusPaused)
	_ = o.saveCampaign()

	// Close channels to signal consumers
	if o.progressChan != nil {
		close(o.progressChan)
		o.progressChan = nil
	}
	if o.eventChan != nil {
		close(o.eventChan)
		o.eventChan = nil
	}
}

// GetProgress returns current campaign progress.
func (o *Orchestrator) GetProgress() Progress {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.campaign == nil {
		return Progress{}
	}

	currentPhase := o.getCurrentPhase()
	currentPhaseName := ""
	currentPhaseIdx := 0
	if currentPhase != nil {
		currentPhaseName = currentPhase.Name
		currentPhaseIdx = currentPhase.Order
	}

	currentTask := ""
	nextTask := o.getNextTask(currentPhase)
	if nextTask != nil {
		currentTask = nextTask.Description
	}

	// Calculate progress
	phaseProgress := 0.0
	if currentPhase != nil && len(currentPhase.Tasks) > 0 {
		completed := 0
		for _, t := range currentPhase.Tasks {
			if t.Status == TaskCompleted || t.Status == TaskSkipped {
				completed++
			}
		}
		phaseProgress = float64(completed) / float64(len(currentPhase.Tasks))
	}

	overallProgress := 0.0
	if o.campaign.TotalTasks > 0 {
		overallProgress = float64(o.campaign.CompletedTasks) / float64(o.campaign.TotalTasks)
	}

	contextUsage := 0.0
	if o.campaign.ContextBudget > 0 {
		contextUsage = float64(o.campaign.ContextUsed) / float64(o.campaign.ContextBudget)
	}

	return Progress{
		CampaignID:      o.campaign.ID,
		CampaignTitle:   o.campaign.Title,
		CampaignStatus:  string(o.campaign.Status),
		CurrentPhase:    currentPhaseName,
		CurrentPhaseIdx: currentPhaseIdx,
		TotalPhases:     o.campaign.TotalPhases,
		PhaseProgress:   phaseProgress,
		OverallProgress: overallProgress,
		CurrentTask:     currentTask,
		CompletedTasks:  o.campaign.CompletedTasks,
		TotalTasks:      o.campaign.TotalTasks,
		ActiveShards:    o.getActiveShardNames(),
		ContextUsage:    contextUsage,
		Learnings:       len(o.campaign.Learnings),
		Replans:         o.campaign.RevisionNumber,
	}
}

// getActiveShardNames returns the names of currently active shards.
func (o *Orchestrator) getActiveShardNames() []string {
	if o.shardMgr == nil {
		return []string{}
	}
	activeShards := o.shardMgr.GetActiveShards()
	names := make([]string, 0, len(activeShards))
	for _, shard := range activeShards {
		names = append(names, shard.GetConfig().Name)
	}
	return names
}

// getCurrentPhase gets the current active phase from Mangle.
func (o *Orchestrator) getCurrentPhase() *Phase {
	facts, err := o.kernel.Query("current_phase")
	if err != nil {
		logging.CampaignDebug("Error querying current_phase: %v", err)
		return nil
	}
	if len(facts) == 0 {
		logging.CampaignDebug("No current_phase fact found")
		return nil
	}

	phaseID := fmt.Sprintf("%v", facts[0].Args[0])
	logging.CampaignDebug("Current phase from kernel: %s", phaseID)

	// Find phase in campaign
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
			return &o.campaign.Phases[i]
		}
	}

	logging.CampaignDebug("Phase %s not found in campaign structure", phaseID)
	return nil
}

// getEligibleTasks returns all runnable tasks for the current phase.
func (o *Orchestrator) getEligibleTasks(phase *Phase) []*Task {
	if phase == nil {
		return nil
	}

	facts, err := o.kernel.Query("eligible_task")
	if err != nil {
		logging.CampaignDebug("Error querying eligible_task: %v", err)
		return nil
	}
	if len(facts) == 0 {
		logging.CampaignDebug("No eligible_task facts found for phase %s", phase.ID)
		return nil
	}

	logging.CampaignDebug("Found %d eligible_task facts from kernel", len(facts))

	tasks := make([]*Task, 0, len(facts))
	for i := range phase.Tasks {
		for _, fact := range facts {
			taskID := fmt.Sprintf("%v", fact.Args[0])
			if phase.Tasks[i].ID == taskID {
				tasks = append(tasks, &phase.Tasks[i])
				break
			}
		}
	}
	logging.CampaignDebug("Matched %d eligible tasks for phase %s", len(tasks), phase.ID)

	// Respect retry backoff windows.
	now := time.Now()
	filtered := make([]*Task, 0, len(tasks))
	skipped := 0
	for _, t := range tasks {
		if !t.NextRetryAt.IsZero() && t.NextRetryAt.After(now) {
			skipped++
			continue
		}
		filtered = append(filtered, t)
	}
	if skipped > 0 {
		logging.CampaignDebug("Filtered %d eligible tasks due to backoff", skipped)
	}
	return filtered
}

// getNextTask gets the next task to execute from Mangle.
func (o *Orchestrator) getNextTask(phase *Phase) *Task {
	if phase == nil {
		return nil
	}

	facts, err := o.kernel.Query("next_campaign_task")
	if err != nil {
		logging.CampaignDebug("Error querying next_campaign_task: %v", err)
		return nil
	}
	if len(facts) == 0 {
		logging.CampaignDebug("No next_campaign_task fact found")
		return nil
	}

	taskID := fmt.Sprintf("%v", facts[0].Args[0])
	logging.CampaignDebug("Next task from kernel: %s", taskID)

	// Find task in phase
	for i := range phase.Tasks {
		if phase.Tasks[i].ID == taskID {
			return &phase.Tasks[i]
		}
	}

	logging.CampaignDebug("Task %s not found in phase %s", taskID, phase.ID)
	return nil
}

// isCampaignComplete checks if all phases are complete.
func (o *Orchestrator) isCampaignComplete() bool {
	completedCount := 0
	skippedCount := 0
	for _, phase := range o.campaign.Phases {
		if phase.Status == PhaseCompleted {
			completedCount++
		} else if phase.Status == PhaseSkipped {
			skippedCount++
		} else {
			logging.CampaignDebug("Campaign not complete: phase %s is %s", phase.ID, phase.Status)
			return false
		}
	}
	logging.CampaignDebug("Campaign complete check: completed=%d, skipped=%d, total=%d",
		completedCount, skippedCount, len(o.campaign.Phases))
	return true
}

// getCampaignBlockReason checks if campaign is blocked.
func (o *Orchestrator) getCampaignBlockReason() string {
	facts, err := o.kernel.Query("campaign_blocked")
	if err != nil {
		logging.CampaignDebug("Error querying campaign_blocked: %v", err)
		return ""
	}
	if len(facts) == 0 {
		return ""
	}

	reason := "unknown"
	if len(facts[0].Args) >= 2 {
		reason = fmt.Sprintf("%v", facts[0].Args[1])
	}
	logging.CampaignDebug("Campaign blocked detected: %s", reason)
	return reason
}

// isPhaseComplete checks if all tasks in a phase are complete.
func (o *Orchestrator) isPhaseComplete(phase *Phase) bool {
	completedCount := 0
	skippedCount := 0
	for _, task := range phase.Tasks {
		if task.Status == TaskCompleted {
			completedCount++
		} else if task.Status == TaskSkipped {
			skippedCount++
		} else {
			logging.CampaignDebug("Phase %s not complete: task %s is %s", phase.ID, task.ID, task.Status)
			return false
		}
	}
	logging.CampaignDebug("Phase %s complete check: completed=%d, skipped=%d, total=%d",
		phase.ID, completedCount, skippedCount, len(phase.Tasks))
	return true
}

// startNextPhase starts the next eligible phase.
func (o *Orchestrator) startNextPhase(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryCampaign, "startNextPhase")
	defer timer.Stop()

	// Check for cancellation before starting phase transition
	select {
	case <-ctx.Done():
		logging.CampaignDebug("Phase transition cancelled")
		return ctx.Err()
	default:
	}

	facts, err := o.kernel.Query("phase_eligible")
	if err != nil || len(facts) == 0 {
		logging.CampaignDebug("No eligible phases found")
		return fmt.Errorf("no eligible phases")
	}

	phaseID := fmt.Sprintf("%v", facts[0].Args[0])
	logging.Campaign("Phase transition: starting phase %s", phaseID)

	// Find and update phase
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
			logging.Campaign("=== Phase Started: %s (%s) ===", o.campaign.Phases[i].Name, phaseID)
			logging.CampaignDebug("Phase details: order=%d, tasks=%d, complexity=%s",
				o.campaign.Phases[i].Order, len(o.campaign.Phases[i].Tasks), o.campaign.Phases[i].EstimatedComplexity)

			o.campaign.Phases[i].Status = PhaseInProgress

			// Update kernel
			_ = o.kernel.RetractFact(core.Fact{
				Predicate: "campaign_phase",
				Args:      []interface{}{phaseID},
			})
			o.kernel.Assert(core.Fact{
				Predicate: "campaign_phase",
				Args: []interface{}{
					phaseID,
					o.campaign.ID,
					o.campaign.Phases[i].Name,
					o.campaign.Phases[i].Order,
					"/in_progress",
					o.campaign.Phases[i].ContextProfile,
				},
			})

			o.emitEvent("phase_started", phaseID, "", o.campaign.Phases[i].Name, nil)
			return nil
		}
	}

	logging.Get(logging.CategoryCampaign).Error("Phase not found: %s", phaseID)
	return fmt.Errorf("phase %s not found", phaseID)
}

// completePhase marks a phase as complete.
func (o *Orchestrator) completePhase(phase *Phase) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phase.ID {
			logging.Campaign("=== Phase Completed: %s (%s) ===", phase.Name, phase.ID)

			completedTasks := 0
			for _, t := range o.campaign.Phases[i].Tasks {
				if t.Status == TaskCompleted {
					completedTasks++
				}
			}
			logging.CampaignDebug("Phase stats: completed tasks=%d/%d", completedTasks, len(o.campaign.Phases[i].Tasks))

			o.campaign.Phases[i].Status = PhaseCompleted
			o.campaign.CompletedPhases++

			logging.Campaign("Campaign progress: phases=%d/%d",
				o.campaign.CompletedPhases, o.campaign.TotalPhases)

			// Update kernel
			_ = o.kernel.RetractFact(core.Fact{
				Predicate: "campaign_phase",
				Args:      []interface{}{phase.ID},
			})
			o.kernel.Assert(core.Fact{
				Predicate: "campaign_phase",
				Args: []interface{}{
					phase.ID,
					o.campaign.ID,
					phase.Name,
					phase.Order,
					"/completed",
					phase.ContextProfile,
				},
			})

			o.emitEvent("phase_completed", phase.ID, "", phase.Name, nil)
			_ = o.saveCampaign()
			break
		}
	}
}

// taskResult is used to collect async task outcomes in runPhase.
type taskResult struct {
	taskID string
	result any
	err    error
}

// runPhase executes all tasks in a phase with bounded parallelism, checkpoints,
// and rolling-wave refinement of the next phase once complete.
func (o *Orchestrator) runPhase(ctx context.Context, phase *Phase) error {
	if phase == nil {
		return nil
	}

	phaseTimer := logging.StartTimer(logging.CategoryCampaign, fmt.Sprintf("runPhase(%s)", phase.Name))
	defer phaseTimer.StopWithInfo()

	logging.Campaign("Executing phase: %s (tasks=%d)", phase.Name, len(phase.Tasks))

	active := make(map[string]bool)
	results := make(chan taskResult, o.maxParallelTasks*2)

	for {
		// Respect cancellation
		select {
		case <-ctx.Done():
			logging.Campaign("Phase %s cancelled", phase.Name)
			return ctx.Err()
		default:
		}

		// Respect pause (no new work scheduled while paused)
		o.mu.RLock()
		paused := o.isPaused
		o.mu.RUnlock()
		if paused {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Drain any completed tasks
		for {
			select {
			case res := <-results:
				logging.CampaignDebug("Task result received: %s (err=%v)", res.taskID, res.err)
				delete(active, res.taskID)
			default:
				goto schedule
			}
		}

	schedule:
		// If phase is done and no active tasks, run checkpoint and finish
		if o.isPhaseComplete(phase) && len(active) == 0 {
			logging.Campaign("Phase %s complete, running checkpoint", phase.Name)
			allPassed, failedSummary, err := o.runPhaseCheckpoint(ctx, phase)
			if err != nil {
				logging.Get(logging.CategoryCampaign).Error("Checkpoint errored for phase %s: %v", phase.ID, err)
				o.emitEvent("checkpoint_failed", phase.ID, "", err.Error(), nil)
			}

			// If any verification failed, trigger a replan and keep the phase open.
			if !allPassed {
				logging.Get(logging.CategoryCampaign).Warn("Phase %s checkpoint failures: %s", phase.ID, failedSummary)
				o.emitEvent("checkpoint_failed", phase.ID, "", failedSummary, nil)

				// Seed a replan trigger so Replanner has a hard signal.
				_ = o.kernel.Assert(core.Fact{
					Predicate: "replan_trigger",
					Args:      []interface{}{o.campaign.ID, "/checkpoint_failed", time.Now().Unix()},
				})

				if o.replanner != nil {
					if repErr := o.replanner.Replan(ctx, o.campaign, ""); repErr != nil {
						logging.Get(logging.CategoryCampaign).Error("Replan after checkpoint failure failed: %v", repErr)
						o.emitEvent("replan_failed", phase.ID, "", repErr.Error(), nil)
					} else {
						o.mu.Lock()
						_ = o.saveCampaign()
						o.mu.Unlock()
					}
				}

				// Return to main loop; policy will keep current phase active.
				return nil
			}

			logging.CampaignDebug("Compressing phase context: %s", phase.ID)
			if summary, count, compressedAt, err := o.contextPager.CompressPhase(ctx, phase); err != nil {
				logging.Get(logging.CategoryCampaign).Warn("Context compression error: %v", err)
				o.emitEvent("compression_error", phase.ID, "", err.Error(), nil)
			} else {
				logging.CampaignDebug("Phase compressed: atoms=%d, summary_len=%d", count, len(summary))
				o.mu.Lock()
				phase.CompressedSummary = summary
				phase.OriginalAtomCount = count
				phase.CompressedAt = compressedAt
				_ = o.saveCampaign()
				o.mu.Unlock()
			}
			o.completePhase(phase)
			o.triggerRollingWave(ctx, phase)
			return nil
		}

		var runnable []*Task

		// Calculate adaptive concurrency limit
		currentLimit := o.determineConcurrencyLimit(active, phase)
		logging.CampaignDebug("Concurrency: active=%d, limit=%d", len(active), currentLimit)

		// Schedule eligible tasks up to the concurrency limit
		if len(active) < currentLimit {
			runnable = o.getEligibleTasks(phase)
			for _, task := range runnable {
				if len(active) >= currentLimit {
					break
				}
				if active[task.ID] || task.Status != TaskPending {
					continue
				}
				logging.Campaign("Scheduling task: %s (type=%s)", task.Description[:min(50, len(task.Description))], task.Type)
				active[task.ID] = true
				o.updateTaskStatus(task, TaskInProgress)
				go o.runSingleTask(ctx, phase, task, results)
			}
		}

		// If nothing is running or eligible, we may be blocked
		if len(active) == 0 {
			if runnable == nil {
				runnable = o.getEligibleTasks(phase)
			}
			if len(runnable) == 0 {
				if reason := o.getCampaignBlockReason(); reason != "" {
					logging.Get(logging.CategoryCampaign).Error("Phase blocked: %s", reason)
					o.emitEvent("campaign_blocked", phase.ID, "", reason, nil)
					o.mu.Lock()
					o.updateCampaignStatus(StatusFailed)
					o.lastError = fmt.Errorf("phase blocked: %s", reason)
					_ = o.saveCampaign()
					o.mu.Unlock()
					return fmt.Errorf("phase blocked: %s", reason)
				}
			}
		}

		// Wait for activity (completion or new eligibility)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res := <-results:
			delete(active, res.taskID)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// triggerRollingWave refreshes downstream plans after a phase completes.
func (o *Orchestrator) triggerRollingWave(ctx context.Context, completedPhase *Phase) {
	logging.Campaign("Rolling-wave refinement triggered after phase: %s", completedPhase.Name)
	timer := logging.StartTimer(logging.CategoryCampaign, "triggerRollingWave")
	defer timer.Stop()

	// Optional: refresh the world model / holographic graph after edits.
	// We rely on the VirtualStore scopes to refresh after writes; this hook
	// keeps the policy facts in sync across phases.
	if o.virtualStore != nil {
		logging.CampaignDebug("Refreshing world model scope")
		// Best-effort scope refresh to update code graph facts
		_, _ = o.virtualStore.RouteAction(ctx, core.Fact{
			Predicate: "action",
			Args:      []interface{}{"/refresh_scope", o.workspace},
		})
	}

	if o.replanner != nil {
		logging.CampaignDebug("Refining next phase based on completed phase: %s", completedPhase.ID)
		if err := o.replanner.RefineNextPhase(ctx, o.campaign, completedPhase); err != nil {
			logging.Get(logging.CategoryCampaign).Warn("Rolling-wave refinement failed: %v", err)
			o.emitEvent("replan_failed", completedPhase.ID, "", err.Error(), nil)
			return
		}

		// Reload campaign facts after refinement to keep Mangle view up to date
		logging.CampaignDebug("Reloading campaign facts after refinement")
		o.kernel.Retract("campaign_phase")
		o.kernel.Retract("campaign_task")
		_ = o.kernel.LoadFacts(o.campaign.ToFacts())

		logging.Campaign("Rolling-wave refinement applied (revision=%d)", o.campaign.RevisionNumber)
		o.emitEvent("replan", completedPhase.ID, "", "Rolling-wave refinement applied", map[string]any{
			"revision": o.campaign.RevisionNumber,
		})
	}
}

// runSingleTask executes a task and sends the result back to the phase loop.
func (o *Orchestrator) runSingleTask(ctx context.Context, phase *Phase, task *Task, results chan<- taskResult) {
	// Apply task-level timeout
	if o.config.TaskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.config.TaskTimeout)
		defer cancel()
	}

	taskTimer := logging.StartTimer(logging.CategoryCampaign, fmt.Sprintf("task(%s)", task.ID))

	logging.Campaign("Task started: %s (type=%s, phase=%s)", task.ID, task.Type, phase.Name)
	logging.CampaignDebug("Task description: %s", task.Description)

	o.emitEvent("task_started", phase.ID, task.ID, task.Description, nil)
	result, err := o.executeTask(ctx, task)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Task failed: %s - %v", task.ID, err)
		taskTimer.Stop()
		o.handleTaskFailure(ctx, phase, task, err)
		results <- taskResult{taskID: task.ID, err: err}
		return
	}

	taskTimer.StopWithInfo()
	logging.Campaign("Task completed: %s", task.ID)

	o.completeTask(task, result)
	o.emitEvent("task_completed", phase.ID, task.ID, "Task completed", result)
	o.applyLearnings(ctx, task, result)
	o.emitProgress()

	o.mu.Lock()
	o.saveCampaign()
	o.mu.Unlock()

	results <- taskResult{taskID: task.ID, result: result}
}

// executeTask executes a single task.
func (o *Orchestrator) executeTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing task %s with type %s, shard=%s", task.ID, task.Type, task.Shard)

	// Update task status
	o.updateTaskStatus(task, TaskInProgress)

	// If task has explicit shard specified, use generic shard routing with context injection
	if task.Shard != "" {
		logging.CampaignDebug("Using explicit shard routing: %s", task.Shard)
		return o.executeWithExplicitShard(ctx, task)
	}

	// Fallback to type-based routing for backward compatibility
	switch task.Type {
	case TaskTypeResearch:
		logging.CampaignDebug("Delegating to research task handler")
		return o.executeResearchTask(ctx, task)
	case TaskTypeFileCreate, TaskTypeFileModify:
		logging.CampaignDebug("Delegating to file task handler")
		return o.executeFileTask(ctx, task)
	case TaskTypeTestWrite:
		logging.CampaignDebug("Delegating to test write handler")
		return o.executeTestWriteTask(ctx, task)
	case TaskTypeTestRun:
		logging.CampaignDebug("Delegating to test run handler")
		return o.executeTestRunTask(ctx, task)
	case TaskTypeVerify:
		logging.CampaignDebug("Delegating to verify handler")
		return o.executeVerifyTask(ctx, task)
	case TaskTypeShardSpawn:
		logging.CampaignDebug("Delegating to shard spawn handler")
		return o.executeShardSpawnTask(ctx, task)
	case TaskTypeRefactor:
		logging.CampaignDebug("Delegating to refactor handler")
		return o.executeRefactorTask(ctx, task)
	case TaskTypeIntegrate:
		logging.CampaignDebug("Delegating to integrate handler")
		return o.executeIntegrateTask(ctx, task)
	case TaskTypeDocument:
		logging.CampaignDebug("Delegating to document handler")
		return o.executeDocumentTask(ctx, task)
	case TaskTypeToolCreate:
		logging.CampaignDebug("Delegating to tool create handler (Ouroboros)")
		return o.executeToolCreateTask(ctx, task)
	case TaskTypeCampaignRef:
		logging.CampaignDebug("Delegating to sub-campaign handler")
		return o.executeCampaignRefTask(ctx, task)
	default:
		logging.CampaignDebug("Using generic task handler for type: %s", task.Type)
		return o.executeGenericTask(ctx, task)
	}
}

// executeWithExplicitShard handles tasks with explicitly specified shard routing.
// This enables the campaign system to call ANY shard at will with context injection.
func (o *Orchestrator) executeWithExplicitShard(ctx context.Context, task *Task) (any, error) {
	shardType := task.Shard
	logging.Campaign("Executing task %s with explicit shard: %s", task.ID, shardType)

	// Build input with context injection from dependent tasks
	input := o.buildTaskInput(task)
	logging.CampaignDebug("Built shard input (%d bytes) for task %s", len(input), task.ID)

	// Spawn the shard
	result, err := o.shardMgr.Spawn(ctx, shardType, input)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Shard %s failed for task %s: %v", shardType, task.ID, err)
		return nil, fmt.Errorf("shard %s failed: %w", shardType, err)
	}

	logging.CampaignDebug("Shard %s completed for task %s, result_len=%d", shardType, task.ID, len(result))

	return map[string]interface{}{
		"shard":  shardType,
		"result": result,
		"task":   task.ID,
	}, nil
}

// executeResearchTask spawns a researcher shard.
func (o *Orchestrator) executeResearchTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Spawning researcher shard for task %s", task.ID)
	result, err := o.shardMgr.Spawn(ctx, "researcher", task.Description)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Researcher shard failed for task %s: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Researcher shard completed for task %s", task.ID)
	return map[string]interface{}{"research_result": result}, nil
}

// executeFileTask creates or modifies a file using the Coder shard.
func (o *Orchestrator) executeFileTask(ctx context.Context, task *Task) (any, error) {
	// Get target path from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing file task %s: path=%s", task.ID, targetPath)

	// Build task string for coder shard
	// NOTE: Don't use "instruction:<value>" format because strings.Fields() splits on spaces,
	// causing multi-word instructions to be truncated. Use simpler format where bare words
	// are joined into the instruction by parseTask.
	action := "create"
	if task.Type == TaskTypeFileModify {
		action = "modify"
	}
	shardTask := fmt.Sprintf("%s file:%s %s", action, targetPath, task.Description)
	logging.CampaignDebug("Spawning coder shard: action=%s, path=%s, task=%s", action, targetPath, shardTask)

	// Delegate to coder shard
	result, err := o.shardMgr.Spawn(ctx, "coder", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Coder shard failed for task %s, using fallback: %v", task.ID, err)
		// Fallback to direct LLM if shard fails
		return o.executeFileTaskFallback(ctx, task, targetPath)
	}

	logging.CampaignDebug("Coder shard completed for task %s, result_len=%d", task.ID, len(result))
	return map[string]interface{}{"coder_result": result, "path": targetPath}, nil
}

// executeFileTaskFallback uses direct LLM when shard is unavailable.
func (o *Orchestrator) executeFileTaskFallback(ctx context.Context, task *Task, targetPath string) (any, error) {
	logging.CampaignDebug("Executing file task fallback for %s via direct LLM", task.ID)
	prompt := fmt.Sprintf(`Generate the following file:
Task: %s
Target Path: %s

Output ONLY the file content, no explanation or markdown fences:`, task.Description, targetPath)

	content, err := o.llmClient.Complete(ctx, prompt)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM file generation failed for task %s: %v", task.ID, err)
		return nil, err
	}

	fullPath := filepath.Join(o.workspace, targetPath)
	logging.CampaignDebug("Writing generated file: %s (%d bytes)", fullPath, len(content))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to create directory for %s: %v", fullPath, err)
		return nil, err
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to write file %s: %v", fullPath, err)
		return nil, err
	}

	logging.CampaignDebug("File fallback completed: %s", fullPath)
	return map[string]interface{}{"path": fullPath, "size": len(content)}, nil
}

// executeTestWriteTask writes tests for existing code using the Tester shard.
func (o *Orchestrator) executeTestWriteTask(ctx context.Context, task *Task) (any, error) {
	// Get target file from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing test write task %s: target=%s", task.ID, targetPath)

	// Build task string for tester shard
	shardTask := fmt.Sprintf("generate_tests file:%s", targetPath)
	logging.CampaignDebug("Spawning tester shard for test generation")

	// Delegate to tester shard
	result, err := o.shardMgr.Spawn(ctx, "tester", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Tester shard failed for test write task %s, falling back to coder: %v", task.ID, err)
		// Fallback to coder shard for test generation
		return o.executeFileTask(ctx, task)
	}

	logging.CampaignDebug("Test write task completed: %s", task.ID)
	return map[string]interface{}{"tester_result": result, "target": targetPath}, nil
}

// executeTestRunTask runs tests using the Tester shard.
func (o *Orchestrator) executeTestRunTask(ctx context.Context, task *Task) (any, error) {
	// Get target from artifacts or use default
	target := "./..."
	if len(task.Artifacts) > 0 {
		target = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing test run task %s: target=%s", task.ID, target)

	// Build task string for tester shard
	shardTask := fmt.Sprintf("run_tests package:%s", target)
	logging.CampaignDebug("Spawning tester shard for test execution")

	// Delegate to tester shard
	result, err := o.shardMgr.Spawn(ctx, "tester", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Tester shard failed for test run task %s, using direct execution: %v", task.ID, err)
		// Fallback to direct execution
		cmd := tactile.ShellCommand{
			Binary:           "go",
			Arguments:        []string{"test", target},
			WorkingDirectory: o.workspace,
			TimeoutSeconds:   300,
		}
		logging.CampaignDebug("Executing tests directly via tactile: go test %s", target)
		output, execErr := o.executor.Execute(ctx, cmd)
		if execErr != nil {
			logging.Get(logging.CategoryCampaign).Error("Test execution failed: %v", execErr)
			return map[string]interface{}{"output": output, "passed": false}, execErr
		}
		logging.Campaign("Tests passed via direct execution")
		return map[string]interface{}{"output": output, "passed": true}, nil
	}

	logging.CampaignDebug("Test run task completed: %s", task.ID)
	return map[string]interface{}{"tester_result": result, "target": target}, nil
}

// executeVerifyTask runs verification (build, lint, etc.).
func (o *Orchestrator) executeVerifyTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing verify task %s: go build ./...", task.ID)
	// Run build verification for this task
	cmd := tactile.ShellCommand{
		Binary:           "go",
		Arguments:        []string{"build", "./..."},
		WorkingDirectory: o.workspace,
		TimeoutSeconds:   300, // 5 minutes
	}
	output, err := o.executor.Execute(ctx, cmd)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Verify task %s failed: %v", task.ID, err)
		return map[string]interface{}{
			"task_id":  task.ID,
			"output":   output,
			"verified": false,
		}, err
	}
	logging.Campaign("Verify task %s passed", task.ID)
	return map[string]interface{}{
		"task_id":  task.ID,
		"output":   output,
		"verified": true,
	}, nil
}

// executeShardSpawnTask spawns a specialized shard.
func (o *Orchestrator) executeShardSpawnTask(ctx context.Context, task *Task) (any, error) {
	// Extract shard type from description
	shardType := "coder" // Default
	logging.CampaignDebug("Executing shard spawn task %s: type=%s", task.ID, shardType)
	result, err := o.shardMgr.Spawn(ctx, shardType, task.Description)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Shard spawn task %s failed: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Shard spawn task completed: %s", task.ID)
	return map[string]interface{}{"shard_result": result}, nil
}

// executeRefactorTask refactors existing code using the Coder shard.
func (o *Orchestrator) executeRefactorTask(ctx context.Context, task *Task) (any, error) {
	// Get target files from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing refactor task %s: path=%s", task.ID, targetPath)

	// Build task string for coder shard
	shardTask := fmt.Sprintf("refactor file:%s instruction:%s", targetPath, task.Description)
	logging.CampaignDebug("Spawning coder shard for refactoring")

	// Delegate to coder shard
	result, err := o.shardMgr.Spawn(ctx, "coder", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Refactor shard failed for task %s, falling back to file task: %v", task.ID, err)
		// Fallback to generic file task
		return o.executeFileTask(ctx, task)
	}

	logging.CampaignDebug("Refactor task completed: %s", task.ID)
	return map[string]interface{}{"coder_result": result, "path": targetPath}, nil
}

// executeIntegrateTask integrates components.
func (o *Orchestrator) executeIntegrateTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing integrate task %s via file task", task.ID)
	return o.executeFileTask(ctx, task)
}

// executeDocumentTask generates documentation.
func (o *Orchestrator) executeDocumentTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing document task %s via file task", task.ID)
	return o.executeFileTask(ctx, task)
}

// executeToolCreateTask triggers tool generation via kernel-mediated autopoiesis.
// It asserts missing_tool_for fact to the kernel, which derives delegate_task(/tool_generator, ...).
// The autopoiesis orchestrator listens for these derived facts and generates the tool.
func (o *Orchestrator) executeToolCreateTask(ctx context.Context, task *Task) (any, error) {
	logging.Campaign("Executing tool create task %s (Ouroboros)", task.ID)
	// Extract tool capability from task description or artifacts
	// For tool creation, the Path field contains the tool/capability name
	capability := task.Description
	if len(task.Artifacts) > 0 && task.Artifacts[0].Path != "" {
		capability = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Tool capability requested: %s", capability)

	// Generate intent ID for this tool creation request
	intentID := fmt.Sprintf("campaign_%s_task_%s", o.campaign.ID, task.ID)
	logging.CampaignDebug("Tool creation intent ID: %s", intentID)

	// Assert missing_tool_for to kernel - this triggers the policy rules:
	// 1. delegate_task(/tool_generator, Cap, /pending) derives
	// 2. next_action(/generate_tool) derives
	// 3. Autopoiesis orchestrator picks up the delegation
	err := o.kernel.Assert(core.Fact{
		Predicate: "missing_tool_for",
		Args:      []interface{}{intentID, capability},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assert missing_tool_for: %w", err)
	}

	// Also assert goal_requires so the policy can derive properly
	err = o.kernel.Assert(core.Fact{
		Predicate: "goal_requires",
		Args:      []interface{}{o.campaign.Goal, capability},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assert goal_requires: %w", err)
	}

	// Emit event for visibility
	o.emitEvent("tool_generation_requested", "", task.ID, capability, map[string]interface{}{
		"intent_id":  intentID,
		"capability": capability,
	})

	// Poll for tool_ready or tool_registered fact (with timeout)
	timeout := time.After(30 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			// Tool generation timed out - return partial success
			// The tool may still be generating in the background
			return map[string]interface{}{
				"status":     "pending",
				"capability": capability,
				"message":    "tool generation initiated but not yet complete",
			}, nil
		case <-ticker.C:
			// Check if tool is now registered
			facts, err := o.kernel.Query("tool_registered")
			if err == nil {
				for _, fact := range facts {
					if len(fact.Args) > 0 {
						if toolName, ok := fact.Args[0].(string); ok && toolName == capability {
							return map[string]interface{}{
								"status":     "complete",
								"capability": capability,
								"tool_name":  toolName,
							}, nil
						}
					}
				}
			}

			// Also check has_capability
			capFacts, capErr := o.kernel.Query("has_capability")
			if capErr == nil {
				for _, fact := range capFacts {
					if len(fact.Args) > 0 {
						if cap, ok := fact.Args[0].(string); ok && cap == capability {
							return map[string]interface{}{
								"status":     "complete",
								"capability": capability,
							}, nil
						}
					}
				}
			}
		}
	}
}

// executeCampaignRefTask handles a sub-campaign reference.
// Currently it validates the sub-campaign ID and logs the intent.
// In a full fractal implementation, this would spawn a child Orchestrator.
func (o *Orchestrator) executeCampaignRefTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing campaign ref task %s", task.ID)
	if task.SubCampaignID == "" {
		logging.Get(logging.CategoryCampaign).Error("Task %s has type /campaign_ref but no sub_campaign_id", task.ID)
		return nil, fmt.Errorf("task %s has type /campaign_ref but no sub_campaign_id", task.ID)
	}

	logging.Campaign("Linking sub-campaign: %s", task.SubCampaignID)
	o.emitEvent("sub_campaign_referenced", "", task.ID, fmt.Sprintf("Linking sub-campaign %s", task.SubCampaignID), nil)

	// In the future, this would look like:
	// childOrch := NewOrchestrator(o.kernel, o.llmClient, ...)
	// childOrch.LoadCampaign(task.SubCampaignID)
	// err := childOrch.Run(ctx)

	// For now, we treat it as a pointer that is "satisfied" if the sub-campaign exists or is acknowledged.
	logging.CampaignDebug("Sub-campaign %s linked (fractal execution not yet implemented)", task.SubCampaignID)
	return map[string]interface{}{
		"sub_campaign_id": task.SubCampaignID,
		"status":          "linked",
	}, nil
}

// executeGenericTask runs a generic task via shard delegation.
func (o *Orchestrator) executeGenericTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing generic task %s via coder shard", task.ID)
	result, err := o.shardMgr.Spawn(ctx, "coder", task.Description)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Generic task %s failed: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Generic task completed: %s", task.ID)
	return map[string]interface{}{"result": result}, nil
}

// updateTaskStatus updates task status in campaign and kernel.
func (o *Orchestrator) updateTaskStatus(task *Task, status TaskStatus) {
	logging.CampaignDebug("Task status update: %s -> %s", task.ID, status)
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.campaign.Phases {
		for j := range o.campaign.Phases[i].Tasks {
			if o.campaign.Phases[i].Tasks[j].ID == task.ID {
				o.campaign.Phases[i].Tasks[j].Status = status
				break
			}
		}
	}

	// Update kernel
	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "campaign_task",
		Args:      []interface{}{task.ID},
	})
	o.kernel.Assert(core.Fact{
		Predicate: "campaign_task",
		Args:      []interface{}{task.ID, task.PhaseID, task.Description, string(status), string(task.Type)},
	})
}

// completeTask marks a task as complete.
func (o *Orchestrator) completeTask(task *Task, result any) {
	o.updateTaskStatus(task, TaskCompleted)

	o.mu.Lock()
	o.campaign.CompletedTasks++
	o.mu.Unlock()

	// Record task result for learning and audit trail
	resultSummary := ""
	if result != nil {
		if data, err := json.Marshal(result); err == nil {
			resultSummary = string(data)
			// Truncate if too long
			if len(resultSummary) > 1000 {
				resultSummary = resultSummary[:1000] + "..."
			}
		}
	}
	o.kernel.Assert(core.Fact{
		Predicate: "task_result",
		Args:      []interface{}{task.ID, "/success", resultSummary},
	})

	// Store result for context injection into dependent tasks
	o.storeTaskResult(task.ID, resultSummary)
}

// storeTaskResult stores a task's result for context injection into dependent tasks.
func (o *Orchestrator) storeTaskResult(taskID, result string) {
	// Compute which results are still needed by pending/active tasks.
	needed := o.computeNeededResultIDs()

	o.resultsMu.Lock()
	defer o.resultsMu.Unlock()
	// Truncate if too large (keep first 10KB for context injection)
	if len(result) > 10240 {
		result = result[:10240] + "\n... [truncated]"
	}

	// Maintain insertion/LRU order
	if _, exists := o.taskResults[taskID]; exists {
		for i, id := range o.taskResultOrder {
			if id == taskID {
				o.taskResultOrder = append(o.taskResultOrder[:i], o.taskResultOrder[i+1:]...)
				break
			}
		}
	}
	o.taskResultOrder = append(o.taskResultOrder, taskID)

	o.taskResults[taskID] = result
	logging.CampaignDebug("Stored result for task %s (%d bytes)", taskID, len(result))

	// Prune cache if needed.
	limit := o.config.TaskResultCacheLimit
	if limit <= 0 {
		limit = 100
	}
	if len(o.taskResultOrder) > limit {
		pruned := 0
		rotations := 0
		for len(o.taskResultOrder) > limit && rotations < len(o.taskResultOrder) {
			oldest := o.taskResultOrder[0]
			o.taskResultOrder = o.taskResultOrder[1:]
			if needed[oldest] {
				// Keep needed results by rotating to the back.
				o.taskResultOrder = append(o.taskResultOrder, oldest)
			} else {
				delete(o.taskResults, oldest)
				pruned++
			}
			rotations++
		}
		if pruned > 0 {
			logging.CampaignDebug("Pruned %d task results (limit=%d)", pruned, limit)
		}
	}
}

// computeNeededResultIDs returns the set of task IDs whose results are referenced
// by pending/in-progress/blocked tasks via ContextFrom.
func (o *Orchestrator) computeNeededResultIDs() map[string]bool {
	needed := make(map[string]bool)
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.campaign == nil {
		return needed
	}
	for _, phase := range o.campaign.Phases {
		for _, task := range phase.Tasks {
			if task.Status == TaskPending || task.Status == TaskInProgress || task.Status == TaskBlocked {
				for _, dep := range task.ContextFrom {
					needed[dep] = true
				}
			}
		}
	}
	return needed
}

// getTaskResult retrieves a stored task result for context injection.
func (o *Orchestrator) getTaskResult(taskID string) (string, bool) {
	o.resultsMu.RLock()
	defer o.resultsMu.RUnlock()
	result, ok := o.taskResults[taskID]
	return result, ok
}

// buildTaskInput constructs the input for a shard by combining the task's
// ShardInput/Description with context from dependent tasks.
func (o *Orchestrator) buildTaskInput(task *Task) string {
	// Start with explicit shard input if provided, otherwise use description
	input := task.ShardInput
	if input == "" {
		input = task.Description
	}

	// Inject context from dependent tasks specified in ContextFrom
	if len(task.ContextFrom) > 0 {
		for _, depID := range task.ContextFrom {
			if result, ok := o.getTaskResult(depID); ok && result != "" {
				input += fmt.Sprintf("\n\n=== CONTEXT FROM TASK %s ===\n%s", depID, result)
				logging.CampaignDebug("Injected context from task %s (%d bytes)", depID, len(result))
			}
		}
	}

	return input
}

// inferShardFromTaskType maps a TaskType to its default shard for backward compatibility.
// Tasks with explicit Shard fields bypass this inference.
func inferShardFromTaskType(taskType TaskType) string {
	switch taskType {
	case TaskTypeFileCreate, TaskTypeFileModify, TaskTypeRefactor, TaskTypeDocument, TaskTypeIntegrate:
		return "coder"
	case TaskTypeTestWrite, TaskTypeTestRun:
		return "tester"
	case TaskTypeResearch:
		return "researcher"
	case TaskTypeVerify:
		return "reviewer"
	default:
		return "coder" // Default to coder for unknown types
	}
}

// handleTaskFailure handles task execution failure.
func (o *Orchestrator) handleTaskFailure(ctx context.Context, phase *Phase, task *Task, err error) {
	logging.Get(logging.CategoryCampaign).Warn("Handling task failure: %s - %v", task.ID, err)

	errorType := classifyTaskError(err)

	o.mu.Lock()
	markedFailed := false
	newStatus := TaskPending
	nextRetryAt := time.Time{}

	// Record attempt and update retry/backoff state
	for i := range o.campaign.Phases {
		for j := range o.campaign.Phases[i].Tasks {
			if o.campaign.Phases[i].Tasks[j].ID != task.ID {
				continue
			}

			attemptNum := len(o.campaign.Phases[i].Tasks[j].Attempts) + 1
			logging.CampaignDebug("Task %s attempt %d failed", task.ID, attemptNum)

			o.campaign.Phases[i].Tasks[j].Attempts = append(
				o.campaign.Phases[i].Tasks[j].Attempts,
				TaskAttempt{
					Number:    attemptNum,
					Outcome:   "/failure",
					Timestamp: time.Now(),
					Error:     err.Error(),
				},
			)
			o.campaign.Phases[i].Tasks[j].LastError = err.Error()

			maxRetries := o.config.MaxRetries
			if maxRetries <= 0 {
				maxRetries = 3
			}

			if attemptNum >= maxRetries {
				logging.Get(logging.CategoryCampaign).Error("Task %s exceeded max retries (%d), marking as failed", task.ID, maxRetries)
				o.campaign.Phases[i].Tasks[j].Status = TaskFailed
				o.campaign.Phases[i].Tasks[j].NextRetryAt = time.Time{}
				markedFailed = true
				newStatus = TaskFailed

				// Record in kernel
				_ = o.kernel.Assert(core.Fact{
					Predicate: "task_error",
					Args:      []interface{}{task.ID, fmt.Sprintf("max_retries_%d", maxRetries), err.Error()},
				})
			} else {
				// Backoff before retrying to avoid tight failure loops.
				backoff := o.computeRetryBackoff(errorType, attemptNum)
				nextRetryAt = time.Now().Add(backoff)
				o.campaign.Phases[i].Tasks[j].Status = TaskPending
				o.campaign.Phases[i].Tasks[j].NextRetryAt = nextRetryAt
				newStatus = TaskPending
			}
			break
		}
	}
	o.mu.Unlock()

	// Update kernel-visible task status for retries.
	o.updateTaskStatus(task, newStatus)

	// Record error taxonomy + retry window for policy/debugging.
	_ = o.kernel.Assert(core.Fact{
		Predicate: "task_error",
		Args:      []interface{}{task.ID, errorType, err.Error()},
	})
	if !nextRetryAt.IsZero() {
		_ = o.kernel.RetractFact(core.Fact{
			Predicate: "task_retry_at",
			Args:      []interface{}{task.ID},
		})
		_ = o.kernel.Assert(core.Fact{
			Predicate: "task_retry_at",
			Args:      []interface{}{task.ID, nextRetryAt.Unix()},
		})
	}

	o.emitEvent("task_failed", phase.ID, task.ID, err.Error(), nil)

	// Update computed failed-task count for Mangle replanning threshold rules.
	o.updateFailedTaskCount()

	// Optionally run checkpoint immediately after a task is fully failed.
	if markedFailed && o.config.CheckpointOnFail {
		if _, _, chkErr := o.runPhaseCheckpoint(ctx, phase); chkErr != nil {
			logging.Get(logging.CategoryCampaign).Warn("Checkpoint-on-fail error: %v", chkErr)
			o.emitEvent("checkpoint_failed", phase.ID, "", chkErr.Error(), nil)
		}
	}

	// Check if replan is needed
	facts, _ := o.kernel.Query("replan_needed")
	if len(facts) > 0 {
		logging.Campaign("Replan triggered due to task failures")
		o.emitEvent("replan_triggered", "", "", "Too many failures, triggering replan", nil)
		if repErr := o.replanner.Replan(ctx, o.campaign, task.ID); repErr != nil {
			logging.Get(logging.CategoryCampaign).Error("Replan failed: %v", repErr)
			o.emitEvent("replan_failed", "", "", repErr.Error(), nil)
		} else {
			o.mu.Lock()
			logging.Campaign("Campaign replanned, new revision: %d", o.campaign.RevisionNumber)
			_ = o.saveCampaign()
			o.mu.Unlock()
		}
	}

	// Persist failure updates for durability.
	o.mu.Lock()
	_ = o.saveCampaign()
	o.mu.Unlock()
}

// classifyTaskError uses heuristics to bucket errors into retry taxonomies.
func classifyTaskError(err error) string {
	if err == nil {
		return "/logic"
	}
	msg := strings.ToLower(err.Error())
	transientHints := []string{
		"timeout",
		"context deadline",
		"rate limit",
		"too many requests",
		"temporar",
		"connection",
		"unavailable",
		"network",
		"i/o",
	}
	for _, h := range transientHints {
		if strings.Contains(msg, h) {
			return "/transient"
		}
	}
	return "/logic"
}

// computeRetryBackoff returns exponential backoff based on attempt number.
func (o *Orchestrator) computeRetryBackoff(errorType string, attemptNum int) time.Duration {
	base := o.config.RetryBackoffBase
	if base <= 0 {
		base = 5 * time.Second
	}
	maxBackoff := o.config.RetryBackoffMax
	if maxBackoff <= 0 {
		maxBackoff = 5 * time.Minute
	}

	shift := attemptNum - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 10 {
		shift = 10
	}
	backoff := base * time.Duration(1<<shift)

	// Logic errors often benefit from faster replans; cap their backoff lower.
	if errorType == "/logic" && backoff > 30*time.Second {
		backoff = 30 * time.Second
	}
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

// assertCampaignConfigFacts publishes runtime configuration to the kernel for policy rules.
func (o *Orchestrator) assertCampaignConfigFacts() {
	if o.campaign == nil || o.kernel == nil {
		return
	}
	campaignID := o.campaign.ID
	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "campaign_config",
		Args:      []interface{}{campaignID},
	})

	maxRetries := o.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	threshold := o.config.ReplanThreshold
	if threshold <= 0 {
		threshold = 3
	}
	autoReplan := "/false"
	if o.config.AutoReplan {
		autoReplan = "/true"
	}
	checkpointOnFail := "/false"
	if o.config.CheckpointOnFail {
		checkpointOnFail = "/true"
	}

	_ = o.kernel.Assert(core.Fact{
		Predicate: "campaign_config",
		Args:      []interface{}{campaignID, maxRetries, threshold, autoReplan, checkpointOnFail},
	})
}

// updateFailedTaskCount recomputes failed task totals and asserts a computed count fact.
func (o *Orchestrator) updateFailedTaskCount() {
	if o.campaign == nil || o.kernel == nil {
		return
	}
	failedCount := 0
	o.mu.RLock()
	for _, phase := range o.campaign.Phases {
		for _, t := range phase.Tasks {
			if t.Status == TaskFailed {
				failedCount++
			}
		}
	}
	campaignID := o.campaign.ID
	o.mu.RUnlock()

	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "failed_campaign_task_count_computed",
		Args:      []interface{}{campaignID},
	})
	_ = o.kernel.Assert(core.Fact{
		Predicate: "failed_campaign_task_count_computed",
		Args:      []interface{}{campaignID, failedCount},
	})
}

// runPhaseCheckpoint runs the checkpoint for a phase.
func (o *Orchestrator) runPhaseCheckpoint(ctx context.Context, phase *Phase) (bool, string, error) {
	logging.Campaign("Running checkpoint for phase: %s", phase.Name)
	timer := logging.StartTimer(logging.CategoryCampaign, fmt.Sprintf("checkpoint(%s)", phase.Name))
	defer timer.Stop()

	allPassed := true
	var failedSummaries []string

	for _, obj := range phase.Objectives {
		if obj.VerificationMethod == VerifyNone {
			logging.CampaignDebug("Skipping verification (method=none) for objective: %s", obj.Description)
			continue
		}

		logging.CampaignDebug("Running verification: %s", obj.VerificationMethod)
		passed, details, err := o.checkpoint.Run(ctx, phase, obj.VerificationMethod)
		if err != nil {
			logging.Get(logging.CategoryCampaign).Error("Checkpoint error: %v", err)
			return false, "", err
		}

		if passed {
			logging.Campaign("Checkpoint PASSED: %s", obj.VerificationMethod)
		} else {
			logging.Get(logging.CategoryCampaign).Warn("Checkpoint FAILED: %s - %s", obj.VerificationMethod, details)
			allPassed = false
			// Keep summaries concise for replanning context.
			short := details
			if len(short) > 500 {
				short = short[:500] + "..."
			}
			failedSummaries = append(failedSummaries, fmt.Sprintf("%s: %s", obj.VerificationMethod, short))
		}

		checkpoint := Checkpoint{
			Type:      string(obj.VerificationMethod),
			Passed:    passed,
			Details:   details,
			Timestamp: time.Now(),
		}

		o.mu.Lock()
		for i := range o.campaign.Phases {
			if o.campaign.Phases[i].ID == phase.ID {
				o.campaign.Phases[i].Checkpoints = append(o.campaign.Phases[i].Checkpoints, checkpoint)
				break
			}
		}
		o.mu.Unlock()

		// Record in kernel
		o.kernel.Assert(core.Fact{
			Predicate: "phase_checkpoint",
			Args:      []interface{}{phase.ID, string(obj.VerificationMethod), passed, details, time.Now().Unix()},
		})
	}

	return allPassed, strings.Join(failedSummaries, " | "), nil
}

// applyLearnings applies autopoiesis learnings from task execution.
func (o *Orchestrator) applyLearnings(ctx context.Context, task *Task, result any) {
	// Check for cancellation before applying learnings
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Query for learnings to apply
	facts, err := o.kernel.Query("promote_to_long_term")
	if err != nil {
		return
	}

	if len(facts) == 0 {
		return
	}

	logging.CampaignDebug("Applying %d learnings from task %s", len(facts), task.ID)

	// Summarize result for learning context
	resultContext := ""
	if result != nil {
		if data, err := json.Marshal(result); err == nil {
			resultContext = string(data)
			if len(resultContext) > 500 {
				resultContext = resultContext[:500] + "..."
			}
		}
	}

	o.mu.Lock()
	for _, fact := range facts {
		// Combine task description with result context for richer learning
		factStr := task.Description
		if resultContext != "" {
			factStr = fmt.Sprintf("%s [result: %s]", task.Description, resultContext)
		}
		learning := Learning{
			Type:      "/success_pattern",
			Pattern:   fmt.Sprintf("%v", fact.Args[0]),
			Fact:      factStr,
			AppliedAt: time.Now(),
		}
		o.campaign.Learnings = append(o.campaign.Learnings, learning)
		logging.CampaignDebug("Learning captured: %s", learning.Pattern)
	}
	o.mu.Unlock()

	logging.Campaign("Captured %d learnings (total=%d)", len(facts), len(o.campaign.Learnings))
}

// emitProgress sends progress update to channel.
func (o *Orchestrator) emitProgress() {
	if o.progressChan == nil {
		return
	}

	select {
	case o.progressChan <- o.GetProgress():
	default:
		// Channel full, skip
	}
}

// emitEvent sends an event to the event channel.
func (o *Orchestrator) emitEvent(eventType, phaseID, taskID, message string, data any) {
	if o.eventChan == nil {
		return
	}

	event := OrchestratorEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		PhaseID:   phaseID,
		TaskID:    taskID,
		Message:   message,
		Data:      data,
	}

	select {
	case o.eventChan <- event:
	default:
		// Channel full, skip
	}
}

// updateCampaignStatus sets the in-memory campaign status and refreshes the canonical kernel fact.
func (o *Orchestrator) updateCampaignStatus(status CampaignStatus) {
	if o.campaign == nil {
		return
	}

	o.campaign.Status = status
	campaignID := o.campaign.ID
	cType := string(o.campaign.Type)
	title := o.campaign.Title
	source := ""
	if len(o.campaign.SourceMaterial) > 0 {
		source = o.campaign.SourceMaterial[0]
	}

	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "campaign",
		Args:      []interface{}{campaignID},
	})
	_ = o.kernel.Assert(core.Fact{
		Predicate: "campaign",
		Args:      []interface{}{campaignID, cType, title, source, string(status)},
	})
}

// determineConcurrencyLimit calculates the dynamic parallelism limit based on active workload.
func (o *Orchestrator) determineConcurrencyLimit(active map[string]bool, phase *Phase) int {
	// Base limit from config
	limit := o.maxParallelTasks

	// Check backpressure from spawn queue first
	if o.shardMgr != nil {
		if status := o.shardMgr.GetBackpressureStatus(); status != nil {
			if status.QueueUtilization > 0.8 {
				// High queue utilization, throttle down to single task
				logging.Campaign("Throttling concurrency: spawn queue at %.0f%%", status.QueueUtilization*100)
				return 1
			} else if status.QueueUtilization > 0.5 {
				// Moderate backpressure, reduce parallelism
				limit = limit / 2
				if limit < 1 {
					limit = 1
				}
				logging.CampaignDebug("Reducing concurrency due to spawn queue pressure (%.0f%%)", status.QueueUtilization*100)
			}
		}
	}

	// Count active task types
	var researchCount, refactorCount, testCount int
	for taskID := range active {
		// Find task in phase
		for _, t := range phase.Tasks {
			if t.ID == taskID {
				switch t.Type {
				case TaskTypeResearch, TaskTypeDocument:
					researchCount++
				case TaskTypeRefactor, TaskTypeIntegrate:
					refactorCount++
				case TaskTypeTestRun, TaskTypeVerify:
					testCount++
				}
				break
			}
		}
	}

	// Adaptive Logic:
	// 1. Refactoring is high-risk/CPU-heavy -> Throttle down
	if refactorCount > 0 {
		return 1 // Serial execution for refactoring to prevent race conditions/clobbering
	}

	// 2. Integration is complex -> Low parallelism
	// (Handled by Refactor count above if we treat them similar, or separate)

	// 3. Research/Tests are IO-bound -> Warning: Research spawns Shards which use memory.
	// We can scale up, but let's be conservative.
	if researchCount > 0 || testCount > 0 {
		// Boost limit for IO heavy work
		limit = o.maxParallelTasks * 2
		if limit > 10 {
			limit = 10
		}
	}

	return limit
}

// HandleNewRequirement processes a dynamic requirement injection from an external system (e.g., Autopoiesis).
// It wraps ReplanForNewRequirement.
func (o *Orchestrator) HandleNewRequirement(ctx context.Context, requirement string) error {
	logging.Campaign("New requirement received: %s", requirement[:min(100, len(requirement))])
	timer := logging.StartTimer(logging.CategoryCampaign, "HandleNewRequirement")
	defer timer.Stop()

	o.emitEvent("new_requirement_received", "", "", requirement, nil)

	// Pause temporarily to safely modify plan
	o.mu.Lock()
	wasPaused := o.isPaused
	o.isPaused = true
	logging.CampaignDebug("Temporarily pausing campaign to integrate new requirement")
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.isPaused = wasPaused
		logging.CampaignDebug("Resuming campaign after requirement integration")
		o.mu.Unlock()
	}()

	// Call the previously unwired Replanner method
	if err := o.replanner.ReplanForNewRequirement(ctx, o.campaign, requirement); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to integrate new requirement: %v", err)
		o.emitEvent("new_requirement_failed", "", "", err.Error(), nil)
		return err
	}

	logging.Campaign("New requirement successfully integrated into plan")
	o.emitEvent("new_requirement_integrated", "", "", "Plan updated with new requirement", nil)
	return nil
}
