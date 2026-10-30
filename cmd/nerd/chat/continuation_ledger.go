package chat

// The continuation's step ledger: which step runs next, which step a pause
// offers, and how each step settles. The plan manager owns approved plan
// steps; a pause offers exactly one step. Advancing the plan retains earlier
// verification gaps so the closing verdict names why the run is unverified.

import (
	"errors"
	"fmt"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"

	tea "github.com/charmbracelet/bubbletea"
)

// subtask is the step a continueMsg names.
func (c continueMsg) subtask() Subtask {
	return Subtask{ID: c.subtaskID, Description: c.description, ShardType: c.shardType, IsMutation: c.isMutation}
}

// planSubtask is an approved plan step in the continuation's form.
func planSubtask(ds core.DreamSubtask) Subtask {
	return Subtask{ID: ds.ID, Description: ds.Description, ShardType: ds.ShardType, IsMutation: ds.IsMutation}
}

// holdsPlanStep reports whether the executing dream plan owns the step id.
func (m Model) holdsPlanStep(id string) bool {
	return m.dreamPlanManager != nil && m.dreamPlanManager.HoldsSubtask(id)
}

// nextPlanStep is the executing plan's next ready step.
func (m Model) nextPlanStep() (Subtask, bool) {
	if m.dreamPlanManager == nil {
		return Subtask{}, false
	}
	ds, ok := m.dreamPlanManager.NextReadySubtask()
	if !ok {
		return Subtask{}, false
	}
	return planSubtask(ds), true
}

// planStepsLeft counts the executing plan's steps that have not run.
func (m Model) planStepsLeft() int {
	if m.dreamPlanManager == nil {
		return 0
	}
	return len(m.dreamPlanManager.UnrunSubtasks())
}

// widenTotal keeps the progress total at least the current step plus the
// plan steps still to run after it.
func (m Model) widenTotal() Model {
	if need := m.continuationStep + m.planStepsLeft(); need > m.continuationTotal {
		m.continuationTotal = need
	}
	return m
}

// dispatchSubtask runs step now. A plan step is marked running and makes the
// chain the plan's.
func (m Model) dispatchSubtask(step Subtask) (Model, tea.Cmd) {
	m.offeredSubtask = nil
	if m.holdsPlanStep(step.ID) {
		m.dreamPlanManager.MarkSubtaskRunning(step.ID)
		m.planChain = true
	}
	m.isLoading = true
	m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, step.Description)
	return m, tea.Batch(m.spinner.Tick, m.executeSubtask(step))
}

// offerSubtask pauses on step: it is the one step Enter or /continue runs.
func (m Model) offerSubtask(step Subtask) Model {
	m.isLoading = false
	offered := step
	m.offeredSubtask = &offered
	return m.pushAssistantMsg(fmt.Sprintf("⏸️ Next: %s\n\nPress Enter to continue, or type new instructions.", step.Description))
}

// offerOrDispatch pauses on step when the work mode pauses before it, and
// runs it otherwise.
func (m Model) offerOrDispatch(step Subtask) (Model, tea.Cmd) {
	if m.pausesBefore(step.IsMutation) {
		return m.offerSubtask(step), nil
	}
	return m.dispatchSubtask(step)
}

// runOfferedSubtask runs the step the pause offered, and only that step.
func (m Model) runOfferedSubtask() (Model, tea.Cmd) {
	if m.offeredSubtask == nil {
		return m, nil
	}
	return m.dispatchSubtask(*m.offeredSubtask)
}

// settlePlanStep records in the plan how a step it owns ended. A step stopped
// before it ran goes back to pending: the plan still owns it.
func (m Model) settlePlanStep(ran Subtask, outcome continuationOutcome, result string) {
	if !m.holdsPlanStep(ran.ID) {
		return
	}
	switch outcome {
	case continuationFailed:
		m.dreamPlanManager.MarkSubtaskFailed(ran.ID, errors.New(result))
	case continuationInterrupted:
		m.dreamPlanManager.MarkSubtaskPending(ran.ID)
	default:
		m.dreamPlanManager.MarkSubtaskComplete(ran.ID, result)
	}
}

// stopPlan ends the executing plan when its chain stops, naming every step
// that did not run and why; the plan's record keeps them as skipped.
func (m Model) stopPlan(reason string) string {
	if m.dreamPlanManager == nil {
		return ""
	}
	unrun := m.dreamPlanManager.UnrunSubtasks()
	if len(unrun) == 0 {
		return ""
	}
	m.dreamPlanManager.CancelPlan()
	var b strings.Builder
	fmt.Fprintf(&b, "\n\nThe approved plan stopped (%s); %d step(s) did not run:", reason, len(unrun))
	for _, s := range unrun {
		fmt.Fprintf(&b, "\n- %s", s.Description)
	}
	return b.String()
}

// resultText is the text a step's result settles in the plan with.
func resultText(p *ShardResultPayload, fallback string) string {
	if p != nil && p.Result != "" {
		return p.Result
	}
	return fallback
}

// handleContinueMsg settles the step that ran and offers or runs the step it
// derived. The approved plan's remaining steps stay with the plan and run
// after the derived step's chain.
func (m Model) handleContinueMsg(msg continueMsg) (tea.Model, tea.Cmd) {
	if msg.completedShardResult != nil {
		m.storeShardResult(msg.completedShardResult.ShardType, msg.completedShardResult.Task, msg.completedShardResult.Result, msg.completedShardResult.Facts)
	}
	m.settlePlanStep(msg.ran, msg.ranOutcome, resultText(msg.completedShardResult, ""))
	if msg.ranOutcome != continuationCompleted {
		m.chainUnverified = true
	}
	if msg.totalSteps > m.continuationTotal {
		m.continuationTotal = msg.totalSteps
	}
	m.continuationStep++
	m = m.widenTotal()
	m.updateContinuationFacts()
	m = m.pushAssistantMsg(fmt.Sprintf("✓ [%d/%d] %s", m.continuationStep-1, m.continuationTotal, m.statusMessage))
	return m.offerOrDispatch(msg.subtask())
}

// handleContinuationDone settles the step that ran, then continues the
// approved plan when the chain was the plan's, or closes the chain with a
// verdict no stronger than its weakest step.
func (m Model) handleContinuationDone(msg continuationDoneMsg) (Model, tea.Cmd) {
	m.isStreaming = false
	m.currentStream = ""
	m.currentThought = ""
	if msg.completedShardResult != nil {
		m.storeShardResult(msg.completedShardResult.ShardType, msg.completedShardResult.Task, msg.completedShardResult.Result, msg.completedShardResult.Facts)
	}
	m.settlePlanStep(msg.ran, msg.outcome, resultText(msg.completedShardResult, msg.summary))
	m.isInterrupted = false
	if m.kernel != nil {
		if err := m.kernel.Retract("interrupt_requested"); err != nil {
			logging.Kernel("[continuation] failed to retract interrupt_requested: %v", err)
		}
	}

	switch msg.outcome {
	case continuationInterrupted:
		m = m.pushAssistantMsg(fmt.Sprintf("⏹️ Stopped by user after %d step(s).\n\n%s", msg.stepCount, msg.summary))
		if msg.ran.Description != "" {
			// The stopped step never ran: it stays this chain's, offered.
			return m.offerSubtask(msg.ran), nil
		}
		return m.endContinuation(), nil
	case continuationFailed:
		stopped := ""
		if m.planChain {
			stopped = m.stopPlan("a step failed")
		}
		m = m.pushAssistantMsg(fmt.Sprintf("❌ Stopped after %d step(s): %s%s%s", msg.stepCount, msg.summary, m.chainUnverifiedNote, stopped))
		return m.endContinuation(), nil
	}

	if msg.outcome != continuationCompleted {
		m.chainUnverified = true
	}
	if m.planChain {
		if next, ok := m.nextPlanStep(); ok {
			if msg.outcome != continuationCompleted && strings.TrimSpace(msg.summary) != "" {
				m.chainUnverifiedNote += "\n\n" + msg.summary
			}
			m.continuationStep++
			m = m.widenTotal()
			m.updateContinuationFacts()
			m = m.pushAssistantMsg(fmt.Sprintf("✓ [%d/%d] %s", m.continuationStep-1, m.continuationTotal, m.statusMessage))
			return m.offerOrDispatch(next)
		}
	}

	outcome, note := msg.outcome, m.chainUnverifiedNote
	if outcome == continuationCompleted && m.chainUnverified {
		outcome = continuationUnverified
		note = "\n\nAn earlier step of this run was not verified." + note
	}
	stopped := ""
	if m.planChain {
		stopped = m.stopPlan("its remaining steps depend on a step that failed")
	}
	switch outcome {
	case continuationCompleted:
		m = m.pushAssistantMsg(fmt.Sprintf("✅ All %d step(s) complete and verified.\n\n%s%s", msg.stepCount, msg.summary, stopped))
	default:
		// The steps ran; nothing showed the work holds. The checkmark is
		// reserved for a verdict, not for having run out of steps.
		m = m.pushAssistantMsg(fmt.Sprintf("⚠️ %d step(s) ran; the work is not verified.\n\n%s%s%s", msg.stepCount, msg.summary, note, stopped))
	}
	wasPlan := m.planChain
	m = m.endContinuation()
	if !wasPlan {
		// A request typed while an approved plan was paused has finished;
		// the plan resumes where it stopped, offered rather than run.
		if next, ok := m.nextPlanStep(); ok {
			done, total, _ := m.dreamPlanManager.GetProgress()
			m.continuationStep, m.continuationTotal = done+1, total
			return m.offerSubtask(next), nil
		}
	}
	return m, nil
}

// endContinuation closes the chain: no step offered, counters and the
// kernel's continuation facts cleared.
func (m Model) endContinuation() Model {
	m.isLoading = false
	m.continuationStep = 0
	m.continuationTotal = 0
	m.offeredSubtask = nil
	m.planChain = false
	m.chainUnverified = false
	m.chainUnverifiedNote = ""
	m.endDelivery()
	if m.kernel != nil {
		_ = m.kernel.Retract("continuation_step")
	}
	return m
}
