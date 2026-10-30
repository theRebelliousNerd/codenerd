package chat

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
)

func countAssistant(m Model, sub string) int {
	n := 0
	for _, h := range m.history {
		if h.Role == "assistant" && strings.Contains(h.Content, sub) {
			n++
		}
	}
	return n
}

// Ctrl+X said "Stopped" the moment it was pressed, while the canceled work
// was still winding down. It now says "Stopping…" until the execution has
// settled, then "Stopped", once.
func TestCtrlX_SaysStoppingUntilTheWorkSettles(t *testing.T) {
	t.Parallel()
	m, ctx := busyModel(t)
	owner := m.turn.current()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)
	if ctx.Err() == nil {
		t.Fatal("Ctrl+X did not cancel the work")
	}
	if !strings.Contains(lastAssistant(m), "Stopping…") || countAssistant(m, "⏹️ Stopped") != 0 {
		t.Fatalf("Ctrl+X claimed the work stopped before it settled:\n%s", lastAssistant(m))
	}

	// A message that arrives while it winds down does not announce it.
	next, _ = m.Update(spinnerTickFor(m))
	m = next.(Model)
	if countAssistant(m, "⏹️ Stopped") != 0 {
		t.Fatal("Stopped was announced while the work still owned the slot")
	}

	// The execution settles: its ending message arrives.
	m.turn.end(owner)
	next, _ = m.Update(errorMsg(context.Canceled))
	m = next.(Model)
	if countAssistant(m, "⏹️ Stopped.") != 1 {
		t.Fatalf("the settled stop was not announced once:\n%s", lastAssistant(m))
	}
	next, _ = m.Update(responseMsg("later"))
	m = next.(Model)
	if countAssistant(m, "⏹️ Stopped.") != 1 {
		t.Fatal("the stop was announced twice")
	}
}

// Work that owns no slot has no cancel to reach: Ctrl+X does not claim to
// have stopped it.
func TestCtrlX_DoesNotClaimToStopWorkItCannotCancel(t *testing.T) {
	t.Parallel()
	m, _ := journeyModel(t, config.WorkModePropose)
	m.isLoading = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)
	if !strings.Contains(lastAssistant(m), "cannot be canceled") || strings.Contains(lastAssistant(m), "⏹️ Stopped.") {
		t.Fatalf("Ctrl+X on uncancelable work said: %q", lastAssistant(m))
	}
}

// spinnerTickFor is a tick for m's spinner: an unrelated message.
func spinnerTickFor(m Model) tea.Msg { return m.spinner.Tick() }
