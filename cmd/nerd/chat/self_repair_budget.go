package chat

import (
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/logging"
	"codenerd/internal/problemlog"
)

// An unattended fix chain has a budget. The kernel decides when it is spent
// (self_repair_fix_over_budget, policy/self_repair.mg: the chain's window is
// still open and the clock is past its start plus
// logging.self_repair_fix_budget_sec); the chat only asks, on the idle timer's
// tick, and stops the chain's turn as Ctrl+X would. settleFixTurn then undoes
// its writes, saves the attempt and records it over budget, which counts as a
// failure and is told to the retry.
//
// Live 2026-11-12: a correct, tested change spent 55 minutes inside its own
// turn's gates -- a step plan that went on after a step failed on a provider
// 503, two specialist reviews and an uplift round that broke the tests, a
// pinning repair -- with no verdict and nothing to end it.

// selfRepairBudgetMsg names the fix chains the kernel derived as over budget.
type selfRepairBudgetMsg struct{ jobs []string }

// openFixJob is the job of the fix chain whose journal is open, or "".
func (s *selfRepairState) openFixJob() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.journal == nil {
		return ""
	}
	return s.journalJob
}

// checkFixBudgetCmd asks the kernel, off the Update thread, whether the open
// fix chain is over budget. Nil when no chain is open.
func (m Model) checkFixBudgetCmd() tea.Cmd {
	if m.selfRepair == nil || m.kernel == nil || m.selfRepair.openFixJob() == "" {
		return nil
	}
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		return nil
	}
	now, kernel := m.selfRepair.clock(), m.kernel
	return func() tea.Msg {
		log := logging.Get(logging.CategoryKernel)
		if err := problemlog.ReplaceRepair(kernel, ledger, now); err != nil {
			log.Warn("%s self-repair facts: %v", selfDebugMarker, err)
			return nil
		}
		jobs, err := problemlog.OverBudgetJobs(kernel)
		if err != nil {
			log.Warn("%s self-repair budget: %v", selfDebugMarker, err)
			return nil
		}
		if len(jobs) == 0 {
			return nil
		}
		return selfRepairBudgetMsg{jobs: jobs}
	}
}

// handleFixOverBudget stops the open fix chain the kernel named, once. A
// chain that settled while the question was asked is left alone.
func (m Model) handleFixOverBudget(msg selfRepairBudgetMsg) (Model, tea.Cmd) {
	if m.selfRepair == nil {
		return m, nil
	}
	job := m.selfRepair.openFixJob()
	if job == "" || !slices.Contains(msg.jobs, job) {
		return m, nil
	}
	m.selfRepair.mu.Lock()
	already := m.selfRepair.overBudgetJob == job
	m.selfRepair.overBudgetJob = job
	fixTurn := m.selfRepair.fixTurn
	m.selfRepair.mu.Unlock()
	if already {
		return m, nil
	}
	budget := m.Config.Logging.SelfRepairFixBudget()
	logging.Get(logging.CategoryKernel).Info("%s self-repair fix chain %s is over its %s budget; stopping it", selfDebugMarker, job, budget)
	owner := m.turn.current()
	if owner == 0 || owner != fixTurn {
		// Between turns: nothing to cancel. The chain settles at its next
		// boundary, or now if it has stopped running.
		return m.settleFixTurn(), nil
	}
	if !m.turn.stop() {
		return m, nil
	}
	m.isLoading = false
	m.turn.announceStop(owner)
	m = m.pushAssistantMsg(fmt.Sprintf("_Self-repair: the fix chain has run past its %s budget and is being stopped; its edits will be undone and the attempt saved._", budget))
	return m, nil
}
