package chat

import (
	"fmt"
	"strings"

	"codenerd/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

// /tool value reports whether each tool earns its place; /tool retire <name>
// withdraws a generated tool the policy proposes to retire, after the same
// work-mode approval every other decision uses. Nothing retires on its own:
// the report only proposes, and a built-in has no retire path at all.

// renderToolValue is the kernel's classification of every tool, from the
// ledger the session has been writing.
func (m Model) renderToolValue() string {
	if m.autopoiesis == nil {
		return "Autopoiesis is not initialized, so no tool ledger is being kept."
	}
	report, err := m.autopoiesis.ToolValueReport()
	if err != nil {
		return fmt.Sprintf("Tool value could not be derived: %v", err)
	}
	var sb strings.Builder
	sb.WriteString("## Tool value\n\n```\n")
	sb.WriteString(report.Table())
	sb.WriteString("```\n")
	if len(report.Proposals) > 0 {
		sb.WriteString("\n`/tool retire <name>` asks for approval to withdraw a generated tool proposed for retirement.\n")
	}
	return sb.String()
}

// retireTool asks the approval broker to withdraw one generated tool. It
// blocks until the person decides (or the standing yolo instruction approves),
// so it runs as a command, not inline.
func (m Model) retireTool(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := m.sessionOperationContext()
		defer cancel()
		if m.autopoiesis == nil {
			return errorMsg(fmt.Errorf("autopoiesis not initialized"))
		}
		var registry *core.ToolRegistry
		if m.virtualStore != nil {
			registry = m.virtualStore.GetToolRegistry()
		}
		proposal, err := m.autopoiesis.RetireGeneratedTool(ctx, m.approvals, name, registry)
		if err != nil {
			return errorMsg(err)
		}
		return responseMsg(fmt.Sprintf("Withdrew generated tool `%s` (%s). Its source is kept, so it can be regenerated.", name, proposal.Reason))
	}
}
