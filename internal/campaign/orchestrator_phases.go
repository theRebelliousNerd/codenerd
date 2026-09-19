package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"fmt"
	"time"
)

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

	if len(facts[0].Args) == 0 || facts[0].Args[0] == nil {
		logging.CampaignDebug("current_phase fact is malformed: no or nil arguments")
		return nil
	}

	phaseID := types.ExtractString(facts[0].Args[0])
	if phaseID == "" {
		logging.CampaignDebug("current_phase fact argument is empty string")
		return nil
	}
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

// livePhaseByID returns the current pointer to the phase with the given ID within
// o.campaign. Callers that cache a *Phase across scheduling iterations must
// re-resolve it through this helper, because a concurrent rollback/replan/
// rolling-wave can swap o.campaign.Phases to a new backing array and orphan the
// cached pointer (see F-SCHED-2). Returns nil if the phase is no longer present.
func (o *Orchestrator) livePhaseByID(phaseID string) *Phase {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.campaign == nil {
		return nil
	}
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

	// The kernel decides what runs: eligible_task weighs dependencies, write-set
	// conflicts, ordering and backoff. An empty answer from a kernel that holds
	// this phase's pending tasks is a decision -- wait -- and nothing here
	// overrides it. Until 2026-09-19 an empty answer, or a failed query, fell
	// back to an in-memory scan of dependencies alone, which scheduled the very
	// tasks the kernel was holding back (external audit N02).
	o.tickKernelClock()
	tasks, err := o.kernelEligibleTasks(phase)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("eligible_task query failed for phase %s: %v; nothing is scheduled this round", phase.ID, err)
		return nil
	}
	if len(tasks) == 0 {
		if pending := o.pendingTasksTheKernelDoesNotHold(phase); pending > 0 {
			// Missing state, not a decision: the case the fallback was written
			// for (a resume whose campaign facts were not reloaded). Reload them
			// and ask again; the kernel still decides.
			logging.Get(logging.CategoryCampaign).Warn(
				"The kernel holds none of phase %s's %d pending task(s); reloading the campaign's facts", phase.ID, pending)
			o.mu.RLock()
			reload := o.campaign.ToFacts()
			o.mu.RUnlock()
			if lerr := o.kernel.LoadFacts(reload); lerr != nil {
				logging.Get(logging.CategoryCampaign).Error("reloading campaign facts for phase %s failed: %v; nothing is scheduled this round", phase.ID, lerr)
				return nil
			}
			if tasks, err = o.kernelEligibleTasks(phase); err != nil {
				logging.Get(logging.CategoryCampaign).Error("eligible_task query failed for phase %s after the reload: %v", phase.ID, err)
				return nil
			}
		}
	}
	logging.CampaignDebug("Matched %d eligible tasks for phase %s", len(tasks), phase.ID)
	return tasks
}

// tickKernelClock feeds the kernel the wall clock before it is asked to
// schedule. eligible_task withholds a task whose task_retry_at is ahead of
// current_time, and nothing in a campaign moved current_time -- the chat
// refreshes it once per user turn -- so a retry scheduled after the last
// refresh stayed in backoff for the rest of the run; the in-memory fallback
// hid it (external audit N02). current_time is whole seconds, so within one
// second the tick is skipped: each one re-evaluates the corpus (~60ms).
func (o *Orchestrator) tickKernelClock() {
	now := time.Now().Unix()
	if o.kernelClock.Swap(now) == now {
		return
	}
	clock := core.Fact{Predicate: "current_time", Args: []any{now}}
	if tr, ok := types.TransactorOf(o.kernel); ok {
		tx := tr.Transaction()
		tx.Retract("current_time")
		tx.Assert(clock)
		if err := tx.Commit(); err != nil {
			o.kernelClock.Store(0)
			logging.Get(logging.CategoryCampaign).Warn("feeding the kernel the clock failed: %v", err)
		}
		return
	}
	// Without a transaction a query can land between the two calls and see no
	// clock; the policy then keeps retry tasks in backoff, which only delays.
	if err := o.kernel.Retract("current_time"); err != nil {
		o.kernelClock.Store(0)
		logging.Get(logging.CategoryCampaign).Warn("retracting the kernel clock failed: %v", err)
		return
	}
	if err := o.kernel.Assert(clock); err != nil {
		o.kernelClock.Store(0)
		logging.Get(logging.CategoryCampaign).Warn("feeding the kernel the clock failed: %v", err)
	}
}

// kernelEligibleTasks returns the phase's tasks the kernel derives
// eligible_task for.
func (o *Orchestrator) kernelEligibleTasks(phase *Phase) ([]*Task, error) {
	facts, err := o.kernel.Query("eligible_task")
	if err != nil {
		return nil, err
	}
	eligible := make(map[string]bool, len(facts))
	for _, fact := range facts {
		if len(fact.Args) > 0 {
			eligible[types.ExtractString(fact.Args[0])] = true
		}
	}
	var tasks []*Task
	for i := range phase.Tasks {
		if eligible[phase.Tasks[i].ID] {
			tasks = append(tasks, &phase.Tasks[i])
		}
	}
	return tasks, nil
}

// pendingTasksTheKernelDoesNotHold counts the phase's pending tasks when the
// kernel holds a campaign_task row for none of them -- state that was never
// loaded, as opposed to a kernel that holds them and schedules none. It is 0
// when the kernel holds any of them or the phase has nothing pending.
func (o *Orchestrator) pendingTasksTheKernelDoesNotHold(phase *Phase) int {
	facts, err := o.kernel.Query("campaign_task")
	if err != nil {
		return 0
	}
	held := make(map[string]bool, len(facts))
	for _, fact := range facts {
		if len(fact.Args) > 0 {
			held[types.ExtractString(fact.Args[0])] = true
		}
	}
	pending := 0
	for i := range phase.Tasks {
		if phase.Tasks[i].Status != TaskPending {
			continue
		}
		if held[phase.Tasks[i].ID] {
			return 0
		}
		pending++
	}
	return pending
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

	taskID := types.ExtractString(facts[0].Args[0])
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
	if o.campaign == nil || len(o.campaign.Phases) == 0 {
		return true
	}
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
		reason = types.ExtractString(facts[0].Args[1])
	}
	logging.CampaignDebug("Campaign blocked detected: %s", reason)
	return reason
}

// isPhaseComplete checks if all tasks in a phase are complete.
func (o *Orchestrator) isPhaseComplete(phase *Phase) bool {
	if phase == nil {
		return false
	}
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
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
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

	phaseID := types.ExtractString(facts[0].Args[0])
	logging.Campaign("Phase transition: starting phase %s", phaseID)

	// Find and update phase
	o.mu.Lock()

	var phaseName string
	var phaseOrder int
	var phaseContextProfile string
	var found bool
	var campaignID string

	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
			logging.Campaign("=== Phase Started: %s (%s) ===", o.campaign.Phases[i].Name, phaseID)
			logging.CampaignDebug("Phase details: order=%d, tasks=%d, complexity=%s",
				o.campaign.Phases[i].Order, len(o.campaign.Phases[i].Tasks), o.campaign.Phases[i].EstimatedComplexity)

			o.campaign.Phases[i].Status = PhaseInProgress

			phaseName = o.campaign.Phases[i].Name
			phaseOrder = o.campaign.Phases[i].Order
			phaseContextProfile = o.campaign.Phases[i].ContextProfile
			campaignID = o.campaign.ID
			found = true
			break
		}
	}

	o.mu.Unlock()

	if found {
		o.markPhaseStart(phaseID)
		// Update kernel.
		//
		// The retract removes the phase's old campaign_phase row, so if the
		// re-assert is dropped the kernel is left with NO row for the phase at
		// all: phase_eligible, task gating and every phase-scoped rule go blind
		// while the orchestrator's in-memory copy says /in_progress. This used
		// to be a bare Assert with the error discarded, which made that state
		// unreportable. The phase has not started as far as the kernel is
		// concerned, so say so — the run loop records lastError and retries.
		_ = o.kernel.RetractFact(core.Fact{
			Predicate: "campaign_phase",
			Args:      []any{phaseID},
		})
		if err := o.kernel.Assert(core.Fact{
			Predicate: "campaign_phase",
			Args: []any{
				phaseID,
				campaignID,
				phaseName,
				phaseOrder,
				"/in_progress",
				phaseContextProfile,
			},
		}); err != nil {
			logging.Get(logging.CategoryCampaign).Error(
				"Phase %s started in memory but its campaign_phase fact was rejected: %v", phaseID, err)
			return fmt.Errorf("assert campaign_phase for %s: %w", phaseID, err)
		}

		// Northstar alignment check at phase transition
		if o.northstarObserver != nil {
			check, err := o.northstarObserver.OnPhaseStart(ctx, phaseID, phaseName)
			if err != nil {
				logging.Campaign("Northstar blocked phase %s: %v", phaseID, err)
				return fmt.Errorf("northstar alignment failed: %w", err)
			}
			if check != nil {
				logging.Campaign("Northstar phase check: %s score=%.2f", check.Result, check.Score)
			}
		}

		o.emitEvent(EventPhaseStarted, phaseID, "", phaseName, nil)
		return nil
	}

	logging.Get(logging.CategoryCampaign).Error("Phase not found: %s", phaseID)
	return fmt.Errorf("phase %s not found", phaseID)
}

// completePhase marks a phase as complete.
// closePhaseUnverified ends a phase whose tasks ran and whose checkpoint never
// passed within its attempts. It is not a completion: CompletedPhases does not
// move, the kernel's row says /unverified -- which every hard dependent reads
// as incomplete (has_incomplete_hard_dep), so none of them starts -- the
// Northstar observer is told the phase failed, and the status is persisted so
// a resume finds the debt and re-arms the checkpoint (PrepareResume).
func (o *Orchestrator) closePhaseUnverified(phase *Phase, failedSummary string) {
	if phase == nil {
		return
	}
	o.mu.Lock()
	var found bool
	var campaignID string
	attempts := 0
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phase.ID {
			logging.Campaign("=== Phase Unverified: %s (%s) ===", phase.Name, phase.ID)
			o.campaign.Phases[i].Status = PhaseUnverified
			attempts = o.campaign.Phases[i].CheckpointFailures
			campaignID = o.campaign.ID
			found = true
			break
		}
	}
	o.mu.Unlock()
	if !found {
		return
	}

	o.observePhaseDuration(phase.ID)
	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "campaign_phase",
		Args:      []any{phase.ID},
	})
	if err := o.kernel.Assert(core.Fact{
		Predicate: "campaign_phase",
		Args: []any{
			phase.ID,
			campaignID,
			phase.Name,
			phase.Order,
			"/unverified",
			phase.ContextProfile,
		},
	}); err != nil {
		logging.Get(logging.CategoryCampaign).Error(
			"Phase %s closed unverified but its /unverified campaign_phase fact was rejected; the kernel now has no row for it: %v",
			phase.ID, err)
	}

	if o.northstarObserver != nil {
		summary := fmt.Sprintf("checkpoint never passed in %d attempt(s): %s", attempts, failedSummary)
		_ = o.northstarObserver.OnPhaseComplete(context.Background(), phase.ID, false, summary)
	}

	o.mu.Lock()
	o.persistCampaign("phase unverified")
	o.mu.Unlock()
}

func (o *Orchestrator) completePhase(phase *Phase) {

	if phase == nil {
		return
	}

	o.mu.Lock()

	var completedTasks int
	var totalTasks int
	var found bool
	var phaseStatus PhaseStatus
	var campaignID string

	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phase.ID {
			logging.Campaign("=== Phase Completed: %s (%s) ===", phase.Name, phase.ID)

			for _, t := range o.campaign.Phases[i].Tasks {
				if t.Status == TaskCompleted {
					completedTasks++
				}
			}
			totalTasks = len(o.campaign.Phases[i].Tasks)
			logging.CampaignDebug("Phase stats: completed tasks=%d/%d", completedTasks, totalTasks)

			o.campaign.Phases[i].Status = PhaseCompleted
			o.campaign.CompletedPhases++

			logging.Campaign("Campaign progress: phases=%d/%d",
				o.campaign.CompletedPhases, o.campaign.TotalPhases)

			phaseStatus = o.campaign.Phases[i].Status
			campaignID = o.campaign.ID
			found = true
			break
		}
	}

	o.mu.Unlock()

	if found {
		o.observePhaseDuration(phase.ID)
		// Update kernel
		_ = o.kernel.RetractFact(core.Fact{
			Predicate: "campaign_phase",
			Args:      []any{phase.ID},
		})
		// Same retract-then-assert hazard as startNextPhase: a dropped assert
		// leaves the kernel with no row for a phase the orchestrator considers
		// finished, so the next phase may never become eligible. completePhase
		// has no error to return and the in-memory campaign is already correct,
		// so this is logged at Error rather than escalated — but it is no
		// longer invisible.
		if err := o.kernel.Assert(core.Fact{
			Predicate: "campaign_phase",
			Args: []any{
				phase.ID,
				campaignID,
				phase.Name,
				phase.Order,
				"/completed",
				phase.ContextProfile,
			},
		}); err != nil {
			logging.Get(logging.CategoryCampaign).Error(
				"Phase %s completed but its /completed campaign_phase fact was rejected; the kernel now has no row for it: %v",
				phase.ID, err)
		}

		// Northstar observation on phase completion
		if o.northstarObserver != nil {
			success := phaseStatus == PhaseCompleted
			summary := fmt.Sprintf("Completed %d/%d tasks", completedTasks, totalTasks)
			_ = o.northstarObserver.OnPhaseComplete(context.Background(), phase.ID, success, summary)
		}

		o.emitEvent(EventPhaseCompleted, phase.ID, "", phase.Name, nil)

		o.mu.Lock()
		o.persistCampaign("phase completion")
		o.mu.Unlock()
	}
}
