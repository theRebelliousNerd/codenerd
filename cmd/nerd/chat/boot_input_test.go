package chat

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// What is typed while the system boots is kept: keys reach the input, Enter
// queues the message instead of running it against a half-booted system, and
// the queue starts once the boot is done. Until 2026-11-06 the input was
// blurred during boot and every key was dropped without a word.
func TestBootInput_IsKeptQueuedAndRunAfterBoot(t *testing.T) {
	t.Parallel()
	m := NewTestModel(WithSize(100, 40))
	t.Cleanup(m.shutdownCancel)
	m.isBooting = true
	m.textarea.Reset()
	m.textarea.Focus()

	for _, r := range "status" {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	if got := m.textarea.Value(); got != "status" {
		t.Fatalf("typed during boot, the input holds %q", got)
	}
	if screen := m.View(); !strings.Contains(screen, "› status") {
		t.Fatalf("the boot screen does not show the draft:\n%s", screen)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.turn.queuedCount() != 1 || m.isLoading || cmd != nil {
		t.Fatalf("Enter during boot: queued %d, loading %v, command %v; want it queued and nothing run", m.turn.queuedCount(), m.isLoading, cmd != nil)
	}
	if m.textarea.Value() != "" {
		t.Fatal("the queued message stayed in the input")
	}

	// The boot finishes: the next update starts the queued message.
	m.isBooting = false
	next, cmd = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)
	if m.turn.queuedCount() != 0 || !m.isLoading || cmd == nil {
		t.Fatalf("after boot: queued %d, loading %v; want the queued message started", m.turn.queuedCount(), m.isLoading)
	}
}
