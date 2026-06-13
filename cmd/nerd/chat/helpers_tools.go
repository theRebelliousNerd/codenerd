package chat

import (
	"codenerd/internal/config"
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// TOOL HELPERS - Autopoiesis Tool Management
// =============================================================================

// renderToolList renders a list of all registered tools
func (m Model) renderToolList() string {
	var sb strings.Builder
	sb.WriteString("## Generated Tools\n\n")

	if m.autopoiesis == nil {
		sb.WriteString("*Autopoiesis not initialized*\n")
		return sb.String()
	}

	// Get the ouroboros loop's tool list via the orchestrator
	tools := m.autopoiesis.ListTools()

	if len(tools) == 0 {
		sb.WriteString("*No tools have been generated yet.*\n\n")
		sb.WriteString("Use `/tool generate <description>` to create a new tool.\n")
		return sb.String()
	}

	sb.WriteString("| Name | Description | Executions | Registered |\n")
	sb.WriteString("|------|-------------|------------|------------|\n")

	for _, tool := range tools {
		desc := tool.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %d | %s |\n",
			tool.Name, desc, tool.ExecuteCount, tool.RegisteredAt.Format("2006-01-02")))
	}

	sb.WriteString("\n*Use `/tool run <name> <input>` to execute a tool*\n")
	sb.WriteString("*Use `/tool info <name>` for details*\n")

	return sb.String()
}

// renderToolInfo renders detailed information about a specific tool
func (m Model) renderToolInfo(toolName string) string {
	var sb strings.Builder

	if m.autopoiesis == nil {
		sb.WriteString("*Autopoiesis not initialized*\n")
		return sb.String()
	}

	info, exists := m.autopoiesis.GetToolInfo(toolName)
	if !exists {
		sb.WriteString(fmt.Sprintf("Tool `%s` not found.\n\n", toolName))
		sb.WriteString("Use `/tool list` to see available tools.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("## Tool: %s\n\n", info.Name))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", info.Description))
	sb.WriteString("### Details\n\n")
	sb.WriteString(fmt.Sprintf("- **Binary Path**: `%s`\n", info.BinaryPath))
	sb.WriteString(fmt.Sprintf("- **Hash**: `%s`\n", info.Hash[:16]+"..."))
	sb.WriteString(fmt.Sprintf("- **Registered**: %s\n", info.RegisteredAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Execution Count**: %d\n\n", info.ExecuteCount))

	// Get quality profile if available
	if profile := m.autopoiesis.GetToolProfile(toolName); profile != nil {
		sb.WriteString("### Quality Profile\n\n")
		sb.WriteString(fmt.Sprintf("- **Type**: %s\n", profile.ToolType))
		sb.WriteString(fmt.Sprintf("- **Expected Duration**: %v - %v\n",
			profile.Performance.ExpectedDurationMin, profile.Performance.ExpectedDurationMax))
		if profile.Output.ExpectsPagination {
			sb.WriteString("- **Expects Pagination**: Yes\n")
		}
	}

	// Get recent execution stats from ToolStore
	if m.toolStore != nil {
		recent, err := m.toolStore.GetRecentByTool(toolName, 5)
		if err == nil && len(recent) > 0 {
			sb.WriteString("\n### Recent Executions\n\n")
			sb.WriteString("| Time | Input Summary | Status | Duration | Refs |\n")
			sb.WriteString("|------|---------------|--------|----------|------|\n")
			for _, r := range recent {
				inputSummary := r.Input
				if len(inputSummary) > 25 {
					inputSummary = inputSummary[:22] + "..."
				}
				if inputSummary == "" {
					inputSummary = "_"
				}
				status := "✅ SUCCESS"
				if !r.Success {
					status = "❌ FAILED"
				}
				sb.WriteString(fmt.Sprintf("| %s | `%s` | %s | %dms | %d |\n",
					r.CreatedAt.Format("15:04:05"), inputSummary, status, r.DurationMs, r.ReferenceCount))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n*Use `/tool run " + toolName + " <input>` to execute*\n")

	return sb.String()
}

// runTool executes a generated tool asynchronously
func (m Model) runTool(toolName, input string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().ShardExecutionTimeout)
		defer cancel()

		if m.autopoiesis == nil {
			return errorMsg(fmt.Errorf("autopoiesis not initialized"))
		}

		m.ReportStatus(fmt.Sprintf("Executing tool: %s...", toolName))

		// Execute the tool with quality evaluation
		output, assessment, err := m.autopoiesis.ExecuteAndEvaluateWithProfile(ctx, toolName, input)
		if err != nil {
			return errorMsg(fmt.Errorf("tool execution failed: %w", err))
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## Tool Execution: %s\n\n", toolName))

		if input != "" {
			sb.WriteString(fmt.Sprintf("**Input**: `%s`\n\n", input))
		}

		sb.WriteString("### Output\n\n")
		sb.WriteString("```\n")
		sb.WriteString(output)
		sb.WriteString("\n```\n\n")

		if assessment != nil {
			sb.WriteString("### Quality Assessment\n\n")
			sb.WriteString(fmt.Sprintf("- **Score**: %.2f\n", assessment.Score))
			sb.WriteString(fmt.Sprintf("- **Completeness**: %.2f\n", assessment.Completeness))
			sb.WriteString(fmt.Sprintf("- **Accuracy**: %.2f\n", assessment.Accuracy))
			if len(assessment.Issues) > 0 {
				sb.WriteString("- **Issues**: ")
				for i, issue := range assessment.Issues {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(string(issue.Type))
				}
				sb.WriteString("\n")
			}
		}

		m.ReportStatus("Tool execution complete")
		return responseMsg(sb.String())
	}
}

// generateTool generates a new tool using the Ouroboros Loop
func (m Model) generateTool(description string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().OuroborosTimeout)
		defer cancel()

		if m.autopoiesis == nil {
			return errorMsg(fmt.Errorf("autopoiesis not initialized"))
		}

		m.ReportStatus("Detecting tool requirements...")

		var sb strings.Builder
		sb.WriteString("## Tool Generation\n\n")
		sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", description))

		// Detect tool need from description
		toolNeed, err := m.autopoiesis.DetectToolNeed(ctx, description)
		if err != nil {
			return errorMsg(fmt.Errorf("failed to analyze tool need: %w", err))
		}

		if toolNeed == nil {
			sb.WriteString("Could not determine tool requirements from description.\n")
			sb.WriteString("Try being more specific about what the tool should do.\n")
			return responseMsg(sb.String())
		}

		sb.WriteString(fmt.Sprintf("**Detected Need**: %s\n", toolNeed.Name))
		sb.WriteString(fmt.Sprintf("**Purpose**: %s\n", toolNeed.Purpose))
		sb.WriteString(fmt.Sprintf("**Confidence**: %.2f\n\n", toolNeed.Confidence))

		// Execute the Ouroboros Loop
		m.ReportStatus(fmt.Sprintf("Generating tool: %s...", toolNeed.Name))
		sb.WriteString("### Ouroboros Loop Execution\n\n")

		result := m.autopoiesis.ExecuteOuroborosLoop(ctx, toolNeed)

		sb.WriteString(fmt.Sprintf("- **Stage Reached**: %s\n", result.Stage))
		sb.WriteString(fmt.Sprintf("- **Duration**: %v\n\n", result.Duration))

		if result.SafetyReport != nil {
			sb.WriteString("**Safety Check**:\n")
			if result.SafetyReport.Safe {
				sb.WriteString(fmt.Sprintf("- Score: %.2f (PASSED)\n", result.SafetyReport.Score))
			} else {
				sb.WriteString("- FAILED\n")
				for _, v := range result.SafetyReport.Violations {
					sb.WriteString(fmt.Sprintf("  - %s: %s\n", v.Type, v.Description))
				}
			}
			sb.WriteString("\n")
		}

		if result.CompileResult != nil && result.CompileResult.Success {
			sb.WriteString("**Compilation**: SUCCESS\n")
			sb.WriteString(fmt.Sprintf("- Binary: `%s`\n", result.CompileResult.OutputPath))
			sb.WriteString(fmt.Sprintf("- Compile Time: %v\n\n", result.CompileResult.CompileTime))
		}

		if result.Success {
			sb.WriteString("### Tool Registered Successfully!\n\n")
			sb.WriteString(fmt.Sprintf("Tool `%s` is now available.\n\n", result.ToolName))
			sb.WriteString(fmt.Sprintf("*Use `/tool run %s <input>` to execute*\n", result.ToolName))
		} else if result.Error != "" {
			sb.WriteString(fmt.Sprintf("### Generation Failed\n\n**Error**: %s\n", result.Error))
		}

		m.ReportStatus("Tool generation complete")
		return responseMsg(sb.String())
	}
}
