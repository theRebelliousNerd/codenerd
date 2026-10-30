package chat

import (
	"fmt"

	"codenerd/internal/logging"

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
	logging.Get(logging.CategorySession).Error("TUI error: %v", err)
	m.recordUXMetric("errors_encountered")
	logging.AuditWithSession(m.sessionID).Error("chat", err, false)
	m = m.pushAssistantMsg(fmt.Sprintf("## Failed\n\n%v", m.err))
	m = m.applyLayout()
	return m, m.saveSessionStateCmd()
}
