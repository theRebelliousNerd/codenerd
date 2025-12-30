// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains tool management command helpers.
package chat

import (
	nerdinit "codenerd/internal/init"
	"fmt"
	"path/filepath"
	"strings"
)

// =============================================================================
// STATUS AND TOOL COMMAND HELPERS
// =============================================================================

// buildStatusReport builds a status report for /status command
func (m Model) buildStatusReport() string {
	var sb strings.Builder
	sb.WriteString("## System Status\n\n")

	sb.WriteString("### Session\n")
	sb.WriteString(fmt.Sprintf("- Session ID: `%s`\n", m.sessionID))
	sb.WriteString(fmt.Sprintf("- Turn Count: %d\n", m.turnCount))
	sb.WriteString(fmt.Sprintf("- Workspace: %s\n", m.workspace))
	sb.WriteString(fmt.Sprintf("- Initialized: %v\n", nerdinit.IsInitialized(m.workspace)))
	sb.WriteString(fmt.Sprintf("- Session State: %s\n", filepath.Join(m.workspace, ".nerd", "session.json")))
	sb.WriteString(fmt.Sprintf("- Sessions Dir: %s\n", filepath.Join(m.workspace, ".nerd", "sessions")))
	sb.WriteString("\n")

	sb.WriteString("### Components\n")
	sb.WriteString("- Kernel: Active\n")
	sb.WriteString("- Transducer: Active\n")
	sb.WriteString("- Shard Manager: Active\n")
	sb.WriteString("- Dreamer: Precog safety enabled\n")
	sb.WriteString("- Legislator: Available via `/legislate`\n")
	sb.WriteString("- Requirements Interrogator: Available via `/clarify`\n")
	if m.activeCampaign != nil {
		sb.WriteString(fmt.Sprintf("- Active Campaign: %s\n", m.activeCampaign.Goal))
	}
	if m.autopoiesis != nil {
		sb.WriteString("- Autopoiesis: Active\n")
	}
	sb.WriteString("\n")

	// Query fact counts
	facts, _ := m.kernel.Query("*")
	sb.WriteString("### Kernel State\n")
	sb.WriteString(fmt.Sprintf("- Total Facts: %d\n", len(facts)))

	// List registered shards
	sb.WriteString("\n### Registered Shards\n")
	sb.WriteString("- coder\n")
	sb.WriteString("- reviewer\n")
	sb.WriteString("- tester\n")
	sb.WriteString("- researcher\n")
	sb.WriteString("- legislator\n")
	sb.WriteString("- requirements_interrogator\n")

	// List generated tools
	if m.autopoiesis != nil {
		tools := m.autopoiesis.ListTools()
		sb.WriteString("\n### Generated Tools\n")
		if len(tools) == 0 {
			sb.WriteString("- No tools generated yet\n")
			sb.WriteString("- Tools are created on-demand when capabilities are missing\n")
			sb.WriteString("- Use `/tool generate <description>` to create a tool\n")
		} else {
			sb.WriteString(fmt.Sprintf("- Total Tools: %d\n", len(tools)))
			sb.WriteString("- Recent Tools:\n")
			count := 0
			for _, tool := range tools {
				if count >= 5 {
					sb.WriteString(fmt.Sprintf("  ... and %d more (use `/tool list` for full list)\n", len(tools)-5))
					break
				}
				sb.WriteString(fmt.Sprintf("  - `%s`: %d executions\n", tool.Name, tool.ExecuteCount))
				count++
			}
		}
	}

	return sb.String()
}

// handleCleanupToolsCommand handles the /cleanup-tools command for managing tool execution storage.
// Flags:
//   - (no args): Show current storage stats
//   - --runtime: Cleanup by runtime hours budget (default: 336 hours)
//   - --size: Cleanup by size limit (default: 100MB)
//   - --smart: LLM-based intelligent cleanup (requires confirmation)
//   - --force: Skip confirmation prompts
func (m Model) handleCleanupToolsCommand(args []string) string {
	var sb strings.Builder

	// Check if ToolStore is available
	if m.toolStore == nil {
		return "Tool execution persistence is not enabled. Initialize with `/init` first."
	}

	// Parse flags
	var mode string
	var force bool
	for _, arg := range args {
		switch arg {
		case "--runtime":
			mode = "runtime"
		case "--size":
			mode = "size"
		case "--smart":
			mode = "smart"
		case "--force":
			force = true
		case "--help", "-h":
			return m.renderCleanupToolsHelp()
		}
	}

	// Get current stats
	stats, err := m.toolStore.GetStats()
	if err != nil {
		return fmt.Sprintf("Error getting storage stats: %v", err)
	}

	sb.WriteString("## Tool Execution Storage\n\n")
	sb.WriteString("### Current Status\n")
	sb.WriteString(fmt.Sprintf("- **Total Executions:** %d\n", stats.TotalExecutions))
	sb.WriteString(fmt.Sprintf("- **Storage Size:** %.2f MB\n", float64(stats.TotalSizeBytes)/1024/1024))
	sb.WriteString(fmt.Sprintf("- **Runtime Hours:** %.1f hours\n", stats.TotalRuntimeHours))
	sb.WriteString(fmt.Sprintf("- **Success/Failure:** %d/%d\n", stats.SuccessCount, stats.FailureCount))
	if len(stats.ToolBreakdown) > 0 {
		sb.WriteString("- **Tools Used:** ")
		first := true
		for tool, count := range stats.ToolBreakdown {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s(%d)", tool, count))
			first = false
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// If no mode specified, just show stats
	if mode == "" {
		sb.WriteString("### Cleanup Options\n")
		sb.WriteString("- `/cleanup-tools --runtime` - Delete executions exceeding 336 runtime hours\n")
		sb.WriteString("- `/cleanup-tools --size` - Delete oldest executions to stay under 100MB\n")
		sb.WriteString("- `/cleanup-tools --smart` - LLM-based intelligent cleanup\n")
		sb.WriteString("- Add `--force` to skip confirmation\n")
		return sb.String()
	}

	// Execute cleanup based on mode
	sb.WriteString("### Cleanup Results\n")

	switch mode {
	case "runtime":
		budgetHours := 336.0 // 14 days equivalent
		if !force {
			sb.WriteString(fmt.Sprintf("Would clean up executions exceeding %.0f runtime hours.\n", budgetHours))
			sb.WriteString("Add `--force` to execute.\n")
			return sb.String()
		}
		result, err := m.toolStore.CleanupByRuntimeBudget(budgetHours)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error during cleanup: %v\n", err))
		} else {
			sb.WriteString(fmt.Sprintf("- **Executions Deleted:** %d\n", result.ExecutionsDeleted))
			sb.WriteString(fmt.Sprintf("- **Space Freed:** %.2f MB\n", float64(result.BytesFreed)/1024/1024))
			sb.WriteString(fmt.Sprintf("- **Runtime Hours Freed:** %.1f hours\n", result.RuntimeHoursFreed))
		}

	case "size":
		maxBytes := int64(100 * 1024 * 1024) // 100MB
		if !force {
			sb.WriteString(fmt.Sprintf("Would clean up to stay under %.0f MB.\n", float64(maxBytes)/1024/1024))
			sb.WriteString("Add `--force` to execute.\n")
			return sb.String()
		}
		result, err := m.toolStore.CleanupBySizeLimit(maxBytes)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error during cleanup: %v\n", err))
		} else {
			sb.WriteString(fmt.Sprintf("- **Executions Deleted:** %d\n", result.ExecutionsDeleted))
			sb.WriteString(fmt.Sprintf("- **Space Freed:** %.2f MB\n", float64(result.BytesFreed)/1024/1024))
		}

	case "smart":
		sb.WriteString("LLM-based intelligent cleanup is not yet implemented.\n")
		sb.WriteString("Use `--runtime` or `--size` for now.\n")
	}

	return sb.String()
}

// renderCleanupToolsHelp renders the help text for /cleanup-tools command.
func (m Model) renderCleanupToolsHelp() string {
	return `## /cleanup-tools - Manage Tool Execution Storage

### Usage
` + "`/cleanup-tools [--runtime|--size|--smart] [--force]`" + `

### Options
- ` + "`(no args)`" + ` - Show current storage statistics
- ` + "`--runtime`" + ` - Delete executions exceeding 336 runtime hours (14 days equivalent)
- ` + "`--size`" + ` - Delete oldest executions to stay under 100MB storage limit
- ` + "`--smart`" + ` - LLM-based intelligent cleanup (coming soon)
- ` + "`--force`" + ` - Skip confirmation and execute cleanup immediately

### Examples
` + "```" + `
/cleanup-tools              # Show stats
/cleanup-tools --runtime    # Preview runtime cleanup
/cleanup-tools --size --force  # Execute size-based cleanup
` + "```" + `

### Storage Strategy
Tool executions are stored in ` + "`.nerd/tools.db`" + ` with:
- Full result content for debugging
- Reference tracking for usefulness scoring
- Session runtime for accurate retention policies
`
}
