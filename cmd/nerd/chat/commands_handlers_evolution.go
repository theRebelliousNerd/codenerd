package chat

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleCmdFix(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/fix <issue description>`",
			Time:    time.Now(),
		})
	} else {
		target := strings.Join(parts[1:], " ")
		task := formatShardTask("/fix", target, "", m.workspace)
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Attempting to fix: %s (with specialist matching)", target),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		// Use specialist-aware spawning for /fix
		return m, tea.Batch(m.spinner.Tick, m.spawnShardWithSpecialists("/fix", "coder", task, target))
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdRefactor handles the corresponding chat slash-command.
func (m Model) handleCmdRefactor(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/refactor <target>`",
			Time:    time.Now(),
		})
	} else {
		target := strings.Join(parts[1:], " ")
		task := formatShardTask("/refactor", target, "", m.workspace)
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Refactoring: %s (with specialist matching)", target),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		// Use specialist-aware spawning for /refactor
		return m, tea.Batch(m.spinner.Tick, m.spawnShardWithSpecialists("/refactor", "coder", task, target))
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdWhy handles the corresponding chat slash-command.
func (m Model) handleCmdWhy(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/why <fact>` - Explains why a fact was derived\n\nExamples:\n- `/why next_action` - Explain why an action was chosen\n- `/why permitted` - Explain what's permitted\n- `/why user_intent` - Show how input was interpreted",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	fact := strings.Join(parts[1:], " ")
	m.isLoading = true
	m.statusMessage = "Tracing derivation..."
	return m, tea.Batch(m.spinner.Tick, m.fetchTraceForWhy(fact))
}

// handleCmdExplainOff handles the corresponding chat slash-command.
func (m Model) handleCmdExplainOff(input string, parts []string) (tea.Model, tea.Cmd) {
	if m.kernel == nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "_error: kernel not available_\n",
			Time:    time.Now(),
		})
	} else {
		m.kernel.DisableProvenance()
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "_Provenance recording disabled._\n",
			Time:    time.Now(),
		})
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdLogic handles the corresponding chat slash-command.
func (m Model) handleCmdLogic(input string, parts []string) (tea.Model, tea.Cmd) {
	// Show current logic pane content
	var sb strings.Builder
	sb.WriteString("## Logic Pane Content\n\n")

	// Get recent facts
	facts, _ := m.kernel.Query("*")
	if len(facts) > 0 {
		sb.WriteString("### Recent Facts\n\n")
		for i, fact := range facts {
			if i >= 20 {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(facts)-20))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", fact.String()))
		}
	}

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdGlassbox handles the corresponding chat slash-command.
func (m Model) handleCmdGlassbox(input string, parts []string) (tea.Model, tea.Cmd) {
	// Glass Box debug mode - inline system visibility
	var response string
	if len(parts) > 1 {
		switch parts[1] {
		case "status":
			response = m.glassBoxStatus()
		case "verbose":
			response = m.toggleGlassBoxVerbose()
		default:
			// Try as category toggle
			response = m.toggleGlassBoxCategory(parts[1])
		}
	} else {
		// Toggle Glass Box mode
		response = m.toggleGlassBox()
	}
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: response,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	// Start listening for events if enabled
	if m.glassBoxEnabled {
		return m, m.listenGlassBoxEvents()
	}
	return m, nil
}

// handleCmdShadow handles the corresponding chat slash-command.
func (m Model) handleCmdShadow(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/shadow <action>` - Run a shadow mode simulation",
			Time:    time.Now(),
		})
	} else {
		action := strings.Join(parts[1:], " ")
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Running shadow simulation for: %s", action),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runShadowSimulation(action))
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdWhatif handles the corresponding chat slash-command.
func (m Model) handleCmdWhatif(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/whatif <change>` - Run a counterfactual query",
			Time:    time.Now(),
		})
	} else {
		change := strings.Join(parts[1:], " ")
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Running counterfactual analysis for: %s", change),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.runWhatIfQuery(change))
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdApprove handles the corresponding chat slash-command.
func (m Model) handleCmdApprove(input string, parts []string) (tea.Model, tea.Cmd) {
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Approval noted. Proceeding with pending changes.",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdReviewAccuracy handles the corresponding chat slash-command.
func (m Model) handleCmdReviewAccuracy(input string, parts []string) (tea.Model, tea.Cmd) {
	// Show accuracy report for the last review
	reviewID := "unknown"
	if m.lastShardResult != nil && m.lastShardResult.ShardType == "reviewer" {
		reviewID = fmt.Sprintf("review-%d-%d", m.lastShardResult.TurnNumber, m.lastShardResult.Timestamp.Unix())
	}
	report := m.shardMgr.GetReviewAccuracyReport(reviewID)
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: fmt.Sprintf("## Review Accuracy Report\n\n%s", report),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdJit handles the corresponding chat slash-command.
func (m Model) handleCmdJit(input string, parts []string) (tea.Model, tea.Cmd) {
	// JIT Prompt Compiler inspector
	content := m.renderJITStatus()
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: content,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdCleanupTools handles the corresponding chat slash-command.
func (m Model) handleCmdCleanupTools(input string, parts []string) (tea.Model, tea.Cmd) {
	// Tool execution cleanup command
	content := m.handleCleanupToolsCommand(parts[1:])
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: content,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdEvolve handles the corresponding chat slash-command.
func (m Model) handleCmdEvolve(input string, parts []string) (tea.Model, tea.Cmd) {
	// Trigger manual evolution cycle
	if m.promptEvolver == nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Prompt Evolution system not initialized.\n\nEnable it in config.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Running evolution cycle...",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.isLoading = true
	m.textarea.Reset()
	return m, tea.Batch(m.spinner.Tick, m.runEvolutionCycle())
}

// handleCmdEvolutionStats handles the corresponding chat slash-command.
func (m Model) handleCmdEvolutionStats(input string, parts []string) (tea.Model, tea.Cmd) {
	content := m.renderEvolutionStats()
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: content,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdEvolvedAtoms handles the corresponding chat slash-command.
func (m Model) handleCmdEvolvedAtoms(input string, parts []string) (tea.Model, tea.Cmd) {
	content := m.renderEvolvedAtoms()
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: content,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdStrategies handles the corresponding chat slash-command.
func (m Model) handleCmdStrategies(input string, parts []string) (tea.Model, tea.Cmd) {
	content := m.renderStrategies()
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: content,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdRejectAtom handles the corresponding chat slash-command.
func (m Model) handleCmdRejectAtom(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/reject-atom <atom-id>`",
			Time:    time.Now(),
		})
	} else if m.promptEvolver == nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Prompt Evolution system not initialized.",
			Time:    time.Now(),
		})
	} else {
		atomID := parts[1]
		if err := m.promptEvolver.RejectAtom(atomID); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to reject atom: %v", err),
				Time:    time.Now(),
			})
		} else if err := m.refreshEvolvedAtomsInCompiler(); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Atom `%s` rejected, but JIT refresh failed: %v", atomID, err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Atom `%s` rejected.", atomID),
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}
