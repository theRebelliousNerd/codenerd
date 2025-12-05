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

	// State
	campaign      *Campaign
	workspace     string
	nerdDir       string
	progressChan  chan Progress
	eventChan     chan OrchestratorEvent

	// Execution tracking
	isRunning     bool
	isPaused      bool
	cancelFunc    context.CancelFunc
	lastError     error
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
}

// NewOrchestrator creates a new campaign orchestrator.
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	nerdDir := filepath.Join(cfg.Workspace, ".nerd")

	o := &Orchestrator{
		kernel:       cfg.Kernel,
		llmClient:    cfg.LLMClient,
		shardMgr:     cfg.ShardManager,
		executor:     cfg.Executor,
		virtualStore: cfg.VirtualStore,
		workspace:    cfg.Workspace,
		nerdDir:      nerdDir,
		progressChan: cfg.ProgressChan,
		eventChan:    cfg.EventChan,
	}

	// Initialize sub-components
	o.contextPager = NewContextPager(cfg.Kernel, cfg.LLMClient)
	o.checkpoint = NewCheckpointRunner(cfg.Executor, cfg.Workspace)
	o.replanner = NewReplanner(cfg.Kernel, cfg.LLMClient)
	o.transducer = perception.NewRealTransducer(cfg.LLMClient)

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

	// Set up cancellation
	ctx, cancel := context.WithCancel(ctx)
	o.cancelFunc = cancel
	o.isRunning = true
	o.isPaused = false
	o.campaign.Status = StatusActive
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
			o.campaign.Status = StatusPaused
			o.saveCampaign()
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
				o.campaign.Status = StatusCompleted
				o.saveCampaign()
				o.mu.Unlock()
				o.emitEvent("campaign_completed", "", "", "Campaign completed successfully", nil)
				return nil
			}

			// Check if blocked
			blockReason := o.getCampaignBlockReason()
			if blockReason != "" {
				o.mu.Lock()
				o.campaign.Status = StatusFailed
				o.lastError = fmt.Errorf("campaign blocked: %s", blockReason)
				o.saveCampaign()
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

		// 3. Get next task
		nextTask := o.getNextTask(currentPhase)
		if nextTask == nil {
			// No more tasks - check if phase is complete
			if o.isPhaseComplete(currentPhase) {
				// Run checkpoint
				if err := o.runPhaseCheckpoint(ctx, currentPhase); err != nil {
					o.emitEvent("checkpoint_failed", currentPhase.ID, "", err.Error(), nil)
					// Don't fail campaign on checkpoint failure - mark phase as completed but with warning
				}

				// Compress phase context
				if err := o.contextPager.CompressPhase(ctx, currentPhase); err != nil {
					o.emitEvent("compression_error", currentPhase.ID, "", err.Error(), nil)
				}

				// Mark phase complete
				o.completePhase(currentPhase)
				continue
			}

			// Tasks exist but none available - might be blocked
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 4. Execute task
		o.emitEvent("task_started", currentPhase.ID, nextTask.ID, nextTask.Description, nil)
		result, err := o.executeTask(ctx, nextTask)
		if err != nil {
			o.handleTaskFailure(ctx, currentPhase, nextTask, err)
			continue
		}

		// 5. Mark task complete and record artifacts
		o.completeTask(nextTask, result)
		o.emitEvent("task_completed", currentPhase.ID, nextTask.ID, "Task completed", result)

		// 6. Apply learnings (autopoiesis)
		o.applyLearnings(ctx, nextTask, result)

		// 7. Emit progress
		o.emitProgress()

		// 8. Save state
		o.mu.Lock()
		o.saveCampaign()
		o.mu.Unlock()
	}
}

// Pause pauses campaign execution.
func (o *Orchestrator) Pause() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isPaused = true
	o.campaign.Status = StatusPaused
	o.saveCampaign()
}

// Resume resumes paused campaign execution.
func (o *Orchestrator) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isPaused = false
	o.campaign.Status = StatusActive
}

// Stop stops campaign execution.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cancelFunc != nil {
		o.cancelFunc()
	}
	o.campaign.Status = StatusPaused
	o.saveCampaign()
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
		ActiveShards:    []string{}, // TODO: Get from shard manager
		ContextUsage:    contextUsage,
		Learnings:       len(o.campaign.Learnings),
		Replans:         o.campaign.RevisionNumber,
	}
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
			break
		}
	}
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

// executeFileTask creates or modifies a file using LLM.
func (o *Orchestrator) executeFileTask(ctx context.Context, task *Task) (any, error) {
	// Get target path from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}

	// Use LLM to generate file content
	prompt := fmt.Sprintf(`Generate the following file:
Task: %s
Target Path: %s

Output ONLY the file content, no explanation or markdown fences:`, task.Description, targetPath)

	content, err := o.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Write file
	fullPath := filepath.Join(o.workspace, targetPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return nil, err
	}

	return map[string]interface{}{"path": fullPath, "size": len(content)}, nil
}

// executeTestWriteTask writes tests for existing code.
func (o *Orchestrator) executeTestWriteTask(ctx context.Context, task *Task) (any, error) {
	// Similar to file task but specialized for tests
	return o.executeFileTask(ctx, task)
}

// executeTestRunTask runs tests.
func (o *Orchestrator) executeTestRunTask(ctx context.Context, task *Task) (any, error) {
	// Run tests via executor
	cmd := tactile.ShellCommand{
		Binary:           "go",
		Arguments:        []string{"test", "./..."},
		WorkingDirectory: o.workspace,
		TimeoutSeconds:   300, // 5 minutes
	}
	output, err := o.executor.Execute(ctx, cmd)
	if err != nil {
		return map[string]interface{}{"output": output, "passed": false}, err
	}
	return map[string]interface{}{"output": output, "passed": true}, nil
}

// executeVerifyTask runs verification (build, lint, etc.).
func (o *Orchestrator) executeVerifyTask(ctx context.Context, task *Task) (any, error) {
	// Run build
	cmd := tactile.ShellCommand{
		Binary:           "go",
		Arguments:        []string{"build", "./..."},
		WorkingDirectory: o.workspace,
		TimeoutSeconds:   300, // 5 minutes
	}
	output, err := o.executor.Execute(ctx, cmd)
	if err != nil {
		return map[string]interface{}{"output": output, "verified": false}, err
	}
	return map[string]interface{}{"output": output, "verified": true}, nil
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

// executeRefactorTask refactors existing code.
func (o *Orchestrator) executeRefactorTask(ctx context.Context, task *Task) (any, error) {
	return o.executeFileTask(ctx, task)
}

// executeIntegrateTask integrates components.
func (o *Orchestrator) executeIntegrateTask(ctx context.Context, task *Task) (any, error) {
	return o.executeFileTask(ctx, task)
}

// executeDocumentTask generates documentation.
func (o *Orchestrator) executeDocumentTask(ctx context.Context, task *Task) (any, error) {
	return o.executeFileTask(ctx, task)
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
	// Query for learnings to apply
	facts, err := o.kernel.Query("promote_to_long_term")
	if err != nil {
		return
	}

	o.mu.Lock()
	for _, fact := range facts {
		learning := Learning{
			Type:      "/success_pattern",
			Pattern:   fmt.Sprintf("%v", fact.Args[0]),
			Fact:      task.Description,
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
