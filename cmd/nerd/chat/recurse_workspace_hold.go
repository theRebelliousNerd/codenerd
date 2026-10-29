package chat

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// recurseHoldsWorkspace reports whether a recurse loop owns the workspace,
// and the refusal a workspace-mutating command or turn gets while it does.
// The loop's attempt cleanup restores every file that differs from HEAD --
// it cannot tell its attempt's writes from anyone else's -- so a write made
// beside a running attempt is not the user's to keep: a /recurse stop would
// put it back (audit recurse-compensation-write-ownership). The chat's own
// writers are refused instead; reading, status and read-only turns stay open.
func (m Model) recurseHoldsWorkspace() (string, bool) {
	if m.recurse == nil {
		return "", false
	}
	return "A recurse loop owns the workspace while it runs; this would change files it may restore when it stops. `/recurse stop` first, or wait for it to finish.", true
}

// replyAndReset answers a command with text and clears the input.
func (m Model) replyAndReset(text string) (tea.Model, tea.Cmd) {
	m = m.addMessage(Message{Role: "assistant", Content: text, Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}
