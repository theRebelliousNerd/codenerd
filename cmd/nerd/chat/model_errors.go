package chat

import (
	"errors"
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/session"

	tea "github.com/charmbracelet/bubbletea"
)

// handleTurnError publishes a known terminal failure through the same history,
// error panel and persistence path. Typed terminal outcomes use this directly;
// legacy errorMsg's clarification handling remains at its dispatch boundary.
func (m Model) handleTurnError(err error) (tea.Model, tea.Cmd) {
	m.isLoading = false
	// A failed turn's streamed prefix is an incomplete attempt, not an
	// answer: it must not linger on screen as if it were one.
	m.isStreaming = false
	m.currentStream = ""
	m.currentThought = ""
	// A broker refusal carries its own explanation; other errors use the
	// configured category and remediation presentation.
	m.err = m.presentError(err)
	m.showError = true
	m.focusError = false
	m.refreshErrorViewport()
	m.errorVP.GotoTop()
	turnErrorLogger(err)("TUI error: %v", err)
	m.recordUXMetric("errors_encountered")
	logging.AuditWithSession(m.sessionID).Error("chat", err, false)
	m = m.pushAssistantMsg(fmt.Sprintf("## Failed\n\n%v", m.err))
	m = m.applyLayout()
	return m, m.saveSessionStateCmd()
}

// turnErrorLogger is the level a failed turn is logged at. A turn whose
// edits did not pass verification (session.ErrVerificationFailed) ended with
// a verdict on the workspace, which the turn reports to the user right here:
// Info. Every other turn error is ERROR, the problems log of codeNERD's own
// faults that debug-self diagnoses. Live 2026-11-12, a failed self-repair fix
// turn's "TUI error: ... post-edit verification failed" became the next
// self-repair's target.
func turnErrorLogger(err error) func(format string, args ...interface{}) {
	log := logging.Get(logging.CategorySession)
	if errors.Is(err, session.ErrVerificationFailed) {
		return log.Info
	}
	return log.Error
}
