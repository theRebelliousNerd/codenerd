package chat

import (
	"fmt"
	"slices"
	"strings"

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
type selfRepairBudgetMsg struct {
	jobs  []string
	kinds map[string][]string
}

// sessionTokenTotal is what the session has spent in model tokens so far, zero
// when usage is not tracked (the token budget then derives nothing and the
// time budget stands alone).
func (m Model) sessionTokenTotal() int64 {
	if m.usageTracker == nil {
		return 0
	}
	return m.usageTracker.SessionTokens(m.sessionID).Total
}

// fixBudgetPhrase names the budgets a chain ran past, for the transcript and
// the ledger note: "25m0s", "3000000-token", or both.
func (m Model) fixBudgetPhrase(kinds []string) string {
	var parts []string
	for _, kind := range kinds {
		switch kind {
		case "time":
			parts = append(parts, m.Config.Logging.SelfRepairFixBudget().String())
		case "tokens":
			parts = append(parts, fmt.Sprintf("%d-token", m.Config.Logging.SelfRepairFixTokenBudget()))
		}
	}
	if len(parts) == 0 {
		return m.Config.Logging.SelfRepairFixBudget().String()
	}
	return strings.Join(parts, " and ")
}

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
	now, kernel, job := m.selfRepair.clock(), m.kernel, m.selfRepair.openFixJob()
	m.selfRepair.mu.Lock()
	start := m.selfRepair.fixStartTokens
	m.selfRepair.mu.Unlock()
	tracked, spent := m.usageTracker != nil, m.sessionTokenTotal()-start
	return func() tea.Msg {
		log := logging.Get(logging.CategoryKernel)
		if err := problemlog.ReplaceRepair(kernel, ledger, now); err != nil {
			log.Warn("%s self-repair facts: %v", selfDebugMarker, err)
			return nil
		}
		// The spend is a measurement only where usage is tracked; an
		// untracked session asserts none, and the time budget stands alone.
		if tracked {
			if err := problemlog.ReplaceFixSpent(kernel, job, spent); err != nil {
				log.Warn("%s self-repair spend: %v", selfDebugMarker, err)
			}
		}
		jobs, err := problemlog.OverBudgetJobs(kernel)
		if err != nil {
			log.Warn("%s self-repair budget: %v", selfDebugMarker, err)
			return nil
		}
		if len(jobs) == 0 {
			return nil
		}
		kinds, err := problemlog.OverBudgetKinds(kernel)
		if err != nil {
			log.Warn("%s self-repair budget kinds: %v", selfDebugMarker, err)
		}
		return selfRepairBudgetMsg{jobs: jobs, kinds: kinds}
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
	m.selfRepair.overBudgetKinds = msg.kinds[job]
	fixTurn := m.selfRepair.fixTurn
	m.selfRepair.mu.Unlock()
	if already {
		return m, nil
	}
	budget := m.fixBudgetPhrase(msg.kinds[job])
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
