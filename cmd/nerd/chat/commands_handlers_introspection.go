package chat

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The three commands below existed only as test call sites until 2026-09-18:
// /shards, /autopoiesis and /facts were in the help tests and the live-kernel
// tests, but neither the registry nor the dispatcher knew them, so typing any
// of them printed "Unknown command" and the tests had been written to accept
// that. Each is the slash form of a view or query the TUI already has: the
// Shard Console (Alt+S), the Autopoiesis Dashboard (Alt+A), and the kernel
// fact query behind /logic.

// handleCmdShards opens the Shard Console and summarises the active shards.
func (m Model) handleCmdShards(input string, parts []string) (tea.Model, tea.Cmd) {
	var sb strings.Builder
	if m.shardMgr == nil {
		sb.WriteString("Shard manager not initialized; no shards to show.")
	} else {
		active := m.shardMgr.GetActiveShards()
		m.shardPage.UpdateContent(active, m.shardMgr.GetBackpressureStatus())
		m.viewMode = ShardPage
		sb.WriteString(fmt.Sprintf("## Shards\n\n%d active shard(s). Shard Console opened (Alt+S toggles it).\n", len(active)))
		for _, s := range active {
			sb.WriteString(fmt.Sprintf("- %v\n", s))
		}
	}
	m = m.addMessage(Message{Role: "assistant", Content: sb.String(), Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdAutopoiesis opens the Autopoiesis Dashboard and summarises what it holds.
func (m Model) handleCmdAutopoiesis(input string, parts []string) (tea.Model, tea.Cmd) {
	var sb strings.Builder
	if m.autopoiesis == nil {
		sb.WriteString("Autopoiesis is not initialized in this session.")
	} else {
		patterns := m.autopoiesis.GetAllPatterns(0.0)
		learnings := m.autopoiesis.GetAllLearnings()
		m.autoPage.UpdateContent(patterns, learnings)
		m.viewMode = AutopoiesisPage
		sb.WriteString(fmt.Sprintf("## Autopoiesis\n\n%d pattern(s), %d learning(s). Dashboard opened (Alt+A toggles it).\n", len(patterns), len(learnings)))
	}
	m = m.addMessage(Message{Role: "assistant", Content: sb.String(), Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdFacts shows kernel facts: all of them summarised, or one predicate
// in full when named (`/facts <predicate>`).
func (m Model) handleCmdFacts(input string, parts []string) (tea.Model, tea.Cmd) {
	var sb strings.Builder
	switch {
	case m.kernel == nil:
		sb.WriteString("Kernel not initialized; no facts to show.")
	case len(parts) > 1:
		pred := strings.TrimSpace(parts[1])
		facts, err := m.kernel.Query(pred)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Query %s failed: %v", pred, err))
			break
		}
		sb.WriteString(fmt.Sprintf("## Facts: %s (%d)\n\n", pred, len(facts)))
		for _, f := range facts {
			sb.WriteString(fmt.Sprintf("- %s\n", f.String()))
		}
	default:
		facts, _ := m.kernel.Query("*")
		const shown = 20
		sb.WriteString(fmt.Sprintf("## Kernel Facts (%d)\n\n", len(facts)))
		for i, f := range facts {
			if i >= shown {
				sb.WriteString(fmt.Sprintf("... and %d more (use `/facts <predicate>` to list one predicate in full)\n", len(facts)-shown))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", f.String()))
		}
	}
	m = m.addMessage(Message{Role: "assistant", Content: sb.String(), Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}
