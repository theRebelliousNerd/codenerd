package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/tactile"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	// Execution tracking
	isRunning  bool
	isPaused   bool
	cancelFunc context.CancelFunc
	lastError  error

	// Concurrency control
	maxParallelTasks int
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
	MaxRetries       int  // Max retries per task (default 3)
	CheckpointOnFail bool // Run checkpoint after task failure
	AutoReplan       bool // Auto-replan on too many failures
	ReplanThreshold  int  // Failures before replan (default 3)
	MaxParallelTasks int  // Max tasks to run in parallel (default 3)
}

// NewOrchestrator creates a new campaign orchestrator.
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	nerdDir := filepath.Join(cfg.Workspace, ".nerd")

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
	}

	// Initialize sub-components
	o.contextPager = NewContextPager(cfg.Kernel, cfg.LLMClient)
	o.checkpoint = NewCheckpointRunner(cfg.Executor, cfg.ShardManager, cfg.Workspace)
	o.replanner = NewReplanner(cfg.Kernel, cfg.LLMClient)
	o.decomposer = NewDecomposer(cfg.Kernel, cfg.LLMClient, cfg.Workspace)
	o.transducer = perception.NewRealTransducer(cfg.LLMClient)

	if cfg.MaxParallelTasks > 0 {
		o.maxParallelTasks = cfg.MaxParallelTasks
	}

	return o
}

// LoadCampaign loads an existing campaign from disk.
func (o *Orchestrator) LoadCampaign(campaignID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	campaignPath := filepath.Join(o.nerdDir, "campaigns", campaignID+".json")
	data, err := os.ReadFile(campaignPath)
	if err != nil {
		return fmt.Errorf("failed to load campaign: %w", err)
	}

	var campaign Campaign
	if err := json.Unmarshal(data, &campaign); err != nil {
		return fmt.Errorf("failed to parse campaign: %w", err)
	}

	o.campaign = &campaign

	// Load campaign facts into kernel
	facts := campaign.ToFacts()
	return o.kernel.LoadFacts(facts)
}

// SetCampaign sets the campaign to execute.
func (o *Orchestrator) SetCampaign(campaign *Campaign) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.campaign = campaign

	// Load campaign facts into kernel
	facts := campaign.ToFacts()
	if err := o.kernel.LoadFacts(facts); err != nil {
		return err
	}

	// Save campaign to disk
	return o.saveCampaign()
}

// saveCampaign persists the campaign to disk.
func (o *Orchestrator) saveCampaign() error {
	campaignsDir := filepath.Join(o.nerdDir, "campaigns")
	if err := os.MkdirAll(campaignsDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(o.campaign, "", "  ")
	if err != nil {
		return err
	}

	campaignPath := filepath.Join(campaignsDir, o.campaign.ID+".json")
	return os.WriteFile(campaignPath, data, 0644)
}

// resetInProgress clears in-flight task/phase states after restarts so work can resume.
func (o *Orchestrator) resetInProgress() {
	for pi := range o.campaign.Phases {
		phase := &o.campaign.Phases[pi]
		if phase.Status == PhaseInProgress {
			phase.Status = PhasePending
		}
		for ti := range phase.Tasks {
			task := &phase.Tasks[ti]
			if task.Status == TaskInProgress {
				task.Status = TaskPending
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
	_ = o.saveCampaign()
}

// Run executes the campaign until completion, pause, or failure.
func (o *Orchestrator) Run(ctx context.Context) error {
	o.mu.Lock()
	if o.campaign == nil {
		o.mu.Unlock()
		return fmt.Errorf("no campaign loaded")
	}
	if o.isRunning {
		o.mu.Unlock()
		return fmt.Errorf("campaign already running")
	}

	// Normalize any dangling in-progress tasks/phases (e.g., after restart)
	o.resetInProgress()

	// Set up cancellation
	ctx, cancel := context.WithCancel(ctx)
	o.cancelFunc = cancel
	o.isRunning = true
	o.isPaused = false
	o.updateCampaignStatus(StatusActive)
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.isRunning = false
		o.cancelFunc = nil
		o.mu.Unlock()
	}()

	// Main execution loop
	for {
		select {
		case <-ctx.Done():
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
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 1. Query Mangle for current state
		currentPhase := o.getCurrentPhase()
		if currentPhase == nil {
			// Check if campaign is complete
			if o.isCampaignComplete() {
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
				o.mu.Lock()
				o.updateCampaignStatus(StatusFailed)
				o.lastError = fmt.Errorf("campaign blocked: %s", blockReason)
				_ = o.saveCampaign()
				o.mu.Unlock()
				return o.lastError
			}

			// No current phase but not complete - start next eligible phase
			if err := o.startNextPhase(ctx); err != nil {
				o.lastError = err
				continue
			}
			continue
		}

		// 2. Page in context for current phase
		if err := o.contextPager.ActivatePhase(ctx, currentPhase); err != nil {
			o.emitEvent("context_error", currentPhase.ID, "", err.Error(), nil)
		}

		// 3. Execute the phase with parallelism + rolling checkpoints
		if err := o.runPhase(ctx, currentPhase); err != nil {
			o.lastError = err
			if ctx.Err() != nil {
				return err
			}
		}
	}
}

// Pause pauses campaign execution.
func (o *Orchestrator) Pause() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isPaused = true
	o.updateCampaignStatus(StatusPaused)
	_ = o.saveCampaign()
}

// Resume resumes paused campaign execution.
func (o *Orchestrator) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isPaused = false
	o.updateCampaignStatus(StatusActive)
}

// Stop stops campaign execution.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cancelFunc != nil {
		o.cancelFunc()
	}
	o.updateCampaignStatus(StatusPaused)
	_ = o.saveCampaign()
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
	if err != nil || len(facts) == 0 {
		return nil
	}

	phaseID := fmt.Sprintf("%v", facts[0].Args[0])

	// Find phase in campaign
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
			return &o.campaign.Phases[i]
		}
	}

	return nil
}

// getEligibleTasks returns all runnable tasks for the current phase.
func (o *Orchestrator) getEligibleTasks(phase *Phase) []*Task {
	if phase == nil {
		return nil
	}

	facts, err := o.kernel.Query("eligible_task")
	if err != nil || len(facts) == 0 {
		return nil
	}

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
	return tasks
}

// getNextTask gets the next task to execute from Mangle.
func (o *Orchestrator) getNextTask(phase *Phase) *Task {
	if phase == nil {
		return nil
	}

	facts, err := o.kernel.Query("next_campaign_task")
	if err != nil || len(facts) == 0 {
		return nil
	}

	taskID := fmt.Sprintf("%v", facts[0].Args[0])

	// Find task in phase
	for i := range phase.Tasks {
		if phase.Tasks[i].ID == taskID {
			return &phase.Tasks[i]
		}
	}

	return nil
}

// isCampaignComplete checks if all phases are complete.
func (o *Orchestrator) isCampaignComplete() bool {
	for _, phase := range o.campaign.Phases {
		if phase.Status != PhaseCompleted && phase.Status != PhaseSkipped {
			return false
		}
	}
	return true
}

// getCampaignBlockReason checks if campaign is blocked.
func (o *Orchestrator) getCampaignBlockReason() string {
	facts, err := o.kernel.Query("campaign_blocked")
	if err != nil || len(facts) == 0 {
		return ""
	}

	if len(facts[0].Args) >= 2 {
		return fmt.Sprintf("%v", facts[0].Args[1])
	}
	return "unknown"
}

// isPhaseComplete checks if all tasks in a phase are complete.
func (o *Orchestrator) isPhaseComplete(phase *Phase) bool {
	for _, task := range phase.Tasks {
		if task.Status != TaskCompleted && task.Status != TaskSkipped {
			return false
		}
	}
	return true
}

// startNextPhase starts the next eligible phase.
func (o *Orchestrator) startNextPhase(ctx context.Context) error {
	// Check for cancellation before starting phase transition
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	facts, err := o.kernel.Query("phase_eligible")
	if err != nil || len(facts) == 0 {
		return fmt.Errorf("no eligible phases")
	}

	phaseID := fmt.Sprintf("%v", facts[0].Args[0])

	// Find and update phase
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
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

	return fmt.Errorf("phase %s not found", phaseID)
}

// completePhase marks a phase as complete.
func (o *Orchestrator) completePhase(phase *Phase) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phase.ID {
			o.campaign.Phases[i].Status = PhaseCompleted
			o.campaign.CompletedPhases++

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

	active := make(map[string]bool)
	results := make(chan taskResult, o.maxParallelTasks*2)

	for {
		// Respect cancellation
		select {
		case <-ctx.Done():
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
				delete(active, res.taskID)
			default:
				goto schedule
			}
		}

	schedule:
		// If phase is done and no active tasks, run checkpoint and finish
		if o.isPhaseComplete(phase) && len(active) == 0 {
			if err := o.runPhaseCheckpoint(ctx, phase); err != nil {
				o.emitEvent("checkpoint_failed", phase.ID, "", err.Error(), nil)
			}
			if summary, count, compressedAt, err := o.contextPager.CompressPhase(ctx, phase); err != nil {
				o.emitEvent("compression_error", phase.ID, "", err.Error(), nil)
			} else {
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
		// Schedule eligible tasks up to the concurrency limit
		if len(active) < o.maxParallelTasks {
			runnable = o.getEligibleTasks(phase)
			for _, task := range runnable {
				if len(active) >= o.maxParallelTasks {
					break
				}
				if active[task.ID] || task.Status != TaskPending {
					continue
				}
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
	// Optional: refresh the world model / holographic graph after edits.
	// We rely on the VirtualStore scopes to refresh after writes; this hook
	// keeps the policy facts in sync across phases.
	if o.virtualStore != nil {
		// Best-effort scope refresh to update code graph facts
		_, _ = o.virtualStore.RouteAction(ctx, core.Fact{
			Predicate: "action",
			Args:      []interface{}{"/refresh_scope", o.workspace},
		})
	}

	if o.replanner != nil {
		if err := o.replanner.RefineNextPhase(ctx, o.campaign, completedPhase); err != nil {
			o.emitEvent("replan_failed", completedPhase.ID, "", err.Error(), nil)
			return
		}

		// Reload campaign facts after refinement to keep Mangle view up to date
		o.kernel.Retract("campaign_phase")
		o.kernel.Retract("campaign_task")
		_ = o.kernel.LoadFacts(o.campaign.ToFacts())

		o.emitEvent("replan", completedPhase.ID, "", "Rolling-wave refinement applied", map[string]any{
			"revision": o.campaign.RevisionNumber,
		})
	}
}

// runSingleTask executes a task and sends the result back to the phase loop.
func (o *Orchestrator) runSingleTask(ctx context.Context, phase *Phase, task *Task, results chan<- taskResult) {
	o.emitEvent("task_started", phase.ID, task.ID, task.Description, nil)
	result, err := o.executeTask(ctx, task)
	if err != nil {
		o.handleTaskFailure(ctx, phase, task, err)
		results <- taskResult{taskID: task.ID, err: err}
		return
	}

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
	// Update task status
	o.updateTaskStatus(task, TaskInProgress)

	// Determine execution strategy based on task type
	switch task.Type {
	case TaskTypeResearch:
		return o.executeResearchTask(ctx, task)
	case TaskTypeFileCreate, TaskTypeFileModify:
		return o.executeFileTask(ctx, task)
	case TaskTypeTestWrite:
		return o.executeTestWriteTask(ctx, task)
	case TaskTypeTestRun:
		return o.executeTestRunTask(ctx, task)
	case TaskTypeVerify:
		return o.executeVerifyTask(ctx, task)
	case TaskTypeShardSpawn:
		return o.executeShardSpawnTask(ctx, task)
	case TaskTypeRefactor:
		return o.executeRefactorTask(ctx, task)
	case TaskTypeIntegrate:
		return o.executeIntegrateTask(ctx, task)
	case TaskTypeDocument:
		return o.executeDocumentTask(ctx, task)
	case TaskTypeToolCreate:
		return o.executeToolCreateTask(ctx, task)
	default:
		return o.executeGenericTask(ctx, task)
	}
}

// executeResearchTask spawns a researcher shard.
func (o *Orchestrator) executeResearchTask(ctx context.Context, task *Task) (any, error) {
	result, err := o.shardMgr.Spawn(ctx, "researcher", task.Description)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"research_result": result}, nil
}

// executeFileTask creates or modifies a file using the Coder shard.
func (o *Orchestrator) executeFileTask(ctx context.Context, task *Task) (any, error) {
	// Get target path from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}

	// Build task string for coder shard
	action := "create"
	if task.Type == TaskTypeFileModify {
		action = "modify"
	}
	shardTask := fmt.Sprintf("%s file:%s instruction:%s", action, targetPath, task.Description)

	// Delegate to coder shard
	result, err := o.shardMgr.Spawn(ctx, "coder", shardTask)
	if err != nil {
		// Fallback to direct LLM if shard fails
		return o.executeFileTaskFallback(ctx, task, targetPath)
	}

	return map[string]interface{}{"coder_result": result, "path": targetPath}, nil
}

// executeFileTaskFallback uses direct LLM when shard is unavailable.
func (o *Orchestrator) executeFileTaskFallback(ctx context.Context, task *Task, targetPath string) (any, error) {
	prompt := fmt.Sprintf(`Generate the following file:
Task: %s
Target Path: %s

Output ONLY the file content, no explanation or markdown fences:`, task.Description, targetPath)

	content, err := o.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(o.workspace, targetPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return nil, err
	}

	return map[string]interface{}{"path": fullPath, "size": len(content)}, nil
}

// executeTestWriteTask writes tests for existing code using the Tester shard.
func (o *Orchestrator) executeTestWriteTask(ctx context.Context, task *Task) (any, error) {
	// Get target file from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}

	// Build task string for tester shard
	shardTask := fmt.Sprintf("generate_tests file:%s", targetPath)

	// Delegate to tester shard
	result, err := o.shardMgr.Spawn(ctx, "tester", shardTask)
	if err != nil {
		// Fallback to coder shard for test generation
		return o.executeFileTask(ctx, task)
	}

	return map[string]interface{}{"tester_result": result, "target": targetPath}, nil
}

// executeTestRunTask runs tests using the Tester shard.
func (o *Orchestrator) executeTestRunTask(ctx context.Context, task *Task) (any, error) {
	// Get target from artifacts or use default
	target := "./..."
	if len(task.Artifacts) > 0 {
		target = task.Artifacts[0].Path
	}

	// Build task string for tester shard
	shardTask := fmt.Sprintf("run_tests package:%s", target)

	// Delegate to tester shard
	result, err := o.shardMgr.Spawn(ctx, "tester", shardTask)
	if err != nil {
		// Fallback to direct execution
		cmd := tactile.ShellCommand{
			Binary:           "go",
			Arguments:        []string{"test", "./..."},
			WorkingDirectory: o.workspace,
			TimeoutSeconds:   300,
		}
		output, execErr := o.executor.Execute(ctx, cmd)
		if execErr != nil {
			return map[string]interface{}{"output": output, "passed": false}, execErr
		}
		return map[string]interface{}{"output": output, "passed": true}, nil
	}

	return map[string]interface{}{"tester_result": result, "target": target}, nil
}

// executeVerifyTask runs verification (build, lint, etc.).
func (o *Orchestrator) executeVerifyTask(ctx context.Context, task *Task) (any, error) {
	// Run build verification for this task
	cmd := tactile.ShellCommand{
		Binary:           "go",
		Arguments:        []string{"build", "./..."},
		WorkingDirectory: o.workspace,
		TimeoutSeconds:   300, // 5 minutes
	}
	output, err := o.executor.Execute(ctx, cmd)
	if err != nil {
		return map[string]interface{}{
			"task_id":  task.ID,
			"output":   output,
			"verified": false,
		}, err
	}
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
	result, err := o.shardMgr.Spawn(ctx, shardType, task.Description)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"shard_result": result}, nil
}

// executeRefactorTask refactors existing code using the Coder shard.
func (o *Orchestrator) executeRefactorTask(ctx context.Context, task *Task) (any, error) {
	// Get target files from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}

	// Build task string for coder shard
	shardTask := fmt.Sprintf("refactor file:%s instruction:%s", targetPath, task.Description)

	// Delegate to coder shard
	result, err := o.shardMgr.Spawn(ctx, "coder", shardTask)
	if err != nil {
		// Fallback to generic file task
		return o.executeFileTask(ctx, task)
	}

	return map[string]interface{}{"coder_result": result, "path": targetPath}, nil
}

// executeIntegrateTask integrates components.
func (o *Orchestrator) executeIntegrateTask(ctx context.Context, task *Task) (any, error) {
	return o.executeFileTask(ctx, task)
}

// executeDocumentTask generates documentation.
func (o *Orchestrator) executeDocumentTask(ctx context.Context, task *Task) (any, error) {
	return o.executeFileTask(ctx, task)
}

// executeToolCreateTask triggers tool generation via kernel-mediated autopoiesis.
// It asserts missing_tool_for fact to the kernel, which derives delegate_task(/tool_generator, ...).
// The autopoiesis orchestrator listens for these derived facts and generates the tool.
func (o *Orchestrator) executeToolCreateTask(ctx context.Context, task *Task) (any, error) {
	// Extract tool capability from task description or artifacts
	// For tool creation, the Path field contains the tool/capability name
	capability := task.Description
	if len(task.Artifacts) > 0 && task.Artifacts[0].Path != "" {
		capability = task.Artifacts[0].Path
	}

	// Generate intent ID for this tool creation request
	intentID := fmt.Sprintf("campaign_%s_task_%s", o.campaign.ID, task.ID)

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
	timeout := time.After(5 * time.Minute)
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

// executeGenericTask runs a generic task via shard delegation.
func (o *Orchestrator) executeGenericTask(ctx context.Context, task *Task) (any, error) {
	result, err := o.shardMgr.Spawn(ctx, "coder", task.Description)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"result": result}, nil
}

// updateTaskStatus updates task status in campaign and kernel.
func (o *Orchestrator) updateTaskStatus(task *Task, status TaskStatus) {
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
}

// handleTaskFailure handles task execution failure.
func (o *Orchestrator) handleTaskFailure(ctx context.Context, phase *Phase, task *Task, err error) {
	o.mu.Lock()

	// Record attempt
	for i := range o.campaign.Phases {
		for j := range o.campaign.Phases[i].Tasks {
			if o.campaign.Phases[i].Tasks[j].ID == task.ID {
				attemptNum := len(o.campaign.Phases[i].Tasks[j].Attempts) + 1
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

				// Check if max retries exceeded
				if attemptNum >= 3 {
					o.campaign.Phases[i].Tasks[j].Status = TaskFailed

					// Record in kernel
					o.kernel.Assert(core.Fact{
						Predicate: "task_error",
						Args:      []interface{}{task.ID, "max_retries", err.Error()},
					})
				}
				break
			}
		}
	}
	o.mu.Unlock()

	o.emitEvent("task_failed", phase.ID, task.ID, err.Error(), nil)

	// Check if replan is needed
	facts, _ := o.kernel.Query("replan_needed")
	if len(facts) > 0 {
		o.emitEvent("replan_triggered", "", "", "Too many failures, triggering replan", nil)
		if err := o.replanner.Replan(ctx, o.campaign); err != nil {
			o.emitEvent("replan_failed", "", "", err.Error(), nil)
		} else {
			o.mu.Lock()
			o.campaign.RevisionNumber++
			o.saveCampaign()
			o.mu.Unlock()
		}
	}
}

// runPhaseCheckpoint runs the checkpoint for a phase.
func (o *Orchestrator) runPhaseCheckpoint(ctx context.Context, phase *Phase) error {
	for _, obj := range phase.Objectives {
		if obj.VerificationMethod == VerifyNone {
			continue
		}

		passed, details, err := o.checkpoint.Run(ctx, phase, obj.VerificationMethod)
		if err != nil {
			return err
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

	return nil
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
	}
	o.mu.Unlock()
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
