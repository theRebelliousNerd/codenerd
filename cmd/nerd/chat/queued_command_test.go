package chat

import (
	"strings"
	"testing"

	"codenerd/internal/config"
)

// A slash command typed while a turn was in prework was queued as text and
// later sent to processInput: /help went to perception instead of the help
// renderer (audit queued-slash-command-kind-conservation). The queue now keeps
// the kind, and a queued command runs through the command dispatcher.
func TestQueuedSlashCommand_RunsAsACommandWhenTheTurnEnds(t *testing.T) {
	t.Parallel()
	m, _ := journeyModel(t, config.WorkModePropose)
	id := m.turn.accept()
	m.isLoading = true

	m, cmd := typeAndEnter(t, m, "/help")
	if cmd != nil || m.turn.queuedCount() != 1 {
		t.Fatalf("/help during prework was not queued (cmd=%v queued=%d)", cmd != nil, m.turn.queuedCount())
	}
	m, _ = typeAndEnter(t, m, "and then list the tests")

	// The running turn settles; the queue drains.
	m.turn.end(id)
	next, cmd := m.Update(responseMsg("Done."))
	m = next.(Model)

	help := NewHelpRenderer(m.workspace).WithGuidance(config.GuidanceNormal).RenderHelp("")
	var sawHelp bool
	for _, h := range m.history {
		if h.Role == "assistant" && h.Content == help {
			sawHelp = true
		}
	}
	if !sawHelp {
		t.Fatalf("the queued /help did not reach the help renderer; last reply:\n%s", lastAssistant(m))
	}
	// /help finishes on the spot, so the message behind it starts next.
	if m.turn.queuedCount() != 0 || !m.isLoading || cmd == nil {
		t.Fatalf("the message queued after /help did not start (queued=%d loading=%v)", m.turn.queuedCount(), m.isLoading)
	}
}

// A slash command typed while work runs is not a sentence for a side turn:
// it waits as a command.
func TestQueuedSlashCommand_WhileWorkRunsSkipsTheSideTurn(t *testing.T) {
	t.Parallel()
	m, _ := busyModel(t)
	m, cmd := typeAndEnter(t, m, "/help")
	if cmd != nil || m.turn.queuedCount() != 1 {
		t.Fatalf("/help while working went to a side turn (cmd=%v queued=%d)", cmd != nil, m.turn.queuedCount())
	}
	if !strings.Contains(lastAssistant(m), "`/help` runs as a command") {
		t.Fatalf("the queue notice does not say /help runs as a command: %q", lastAssistant(m))
	}
}
