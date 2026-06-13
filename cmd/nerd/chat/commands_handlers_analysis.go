package chat

import (
	"fmt"
	"strings"
	"time"

	nerdinit "codenerd/internal/init"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleCmdLegislate(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/legislate <constraint>`\n\nExample: `/legislate Stop using fmt.Printf; use log.Info instead.`",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	task := strings.TrimSpace(strings.TrimPrefix(input, "/legislate"))
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Legislator engaged. Compiling and ratifying rule...",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, tea.Batch(m.spinner.Tick, m.spawnShard("legislator", task))
}

// handleCmdClarify handles the corresponding chat slash-command.
func (m Model) handleCmdClarify(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/clarify <goal>`\n\nExample: `/clarify build a campaign to harden auth`",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	task := strings.TrimSpace(strings.TrimPrefix(input, "/clarify"))
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Requirements Interrogator engaged. Drafting clarifying questions...",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, tea.Batch(m.spinner.Tick, m.spawnShard("requirements_interrogator", task))
}

// handleCmdLaunchCampaign handles the corresponding chat slash-command.
func (m Model) handleCmdLaunchCampaign(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/launchcampaign <goal>`\n\nThis will run clarifications, then auto-start a campaign hands-free if possible.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	goal := strings.TrimSpace(strings.TrimPrefix(input, "/launchcampaign"))
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Launching auto-campaign: running clarifier and then starting the campaign...",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, tea.Batch(m.spinner.Tick, m.runLaunchCampaign(goal))
}

// handleCmdInit handles the corresponding chat slash-command.
func (m Model) handleCmdInit(input string, parts []string) (tea.Model, tea.Cmd) {
	// Check for --force flag
	forceInit := false
	for _, part := range parts[1:] {
		if part == "--force" || part == "-f" {
			forceInit = true
			break
		}
	}

	// Check if already initialized and not forcing
	if nerdinit.IsInitialized(m.workspace) && !forceInit {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Workspace already initialized. Use `/init --force` to reinitialize.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Initializing codeNERD... This may take a few minutes for research and agent creation.",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.runInitialization(forceInit))
}

// handleCmdScan handles the corresponding chat slash-command.
func (m Model) handleCmdScan(input string, parts []string) (tea.Model, tea.Cmd) {
	deep := false
	for _, part := range parts[1:] {
		if part == "--deep" || part == "-d" {
			deep = true
			break
		}
	}
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Scanning workspace...",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.runScan(deep))
}

// handleCmdRefreshDocs handles the corresponding chat slash-command.
func (m Model) handleCmdRefreshDocs(input string, parts []string) (tea.Model, tea.Cmd) {
	// Refresh strategic knowledge by re-scanning and processing documentation
	// Uses Mangle tracking to only process new/changed docs
	force := false
	for _, part := range parts[1:] {
		if part == "--force" || part == "-f" {
			force = true
			break
		}
	}
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Scanning documentation for updates...\n\nThis will:\n- Discover new/changed docs\n- Use LLM to filter for relevance\n- Extract knowledge atoms incrementally\n- Update the strategic knowledge base",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.runDocRefresh(force))
}

// handleCmdScanPath handles the corresponding chat slash-command.
func (m Model) handleCmdScanPath(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: /scan-path <file1>[,<file2>...]",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	targets := strings.Split(parts[1], ",")
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Scanning %d path(s)...", len(targets)),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.runPartialScan(targets))
}

// handleCmdScanDir handles the corresponding chat slash-command.
func (m Model) handleCmdScanDir(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: /scan-dir <directory>",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
	dir := parts[1]
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Scanning directory: %s", dir),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.runDirScan(dir))
}

// handleCmdRead handles the corresponding chat slash-command.
func (m Model) handleCmdRead(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/read <path>`",
			Time:    time.Now(),
		})
	} else {
		path := parts[1]
		content, err := readFileContent(m.workspace, path, 16000)
		if err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to read file: %v", err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Contents of %s:**\n\n```\n%s\n```", path, content),
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdMkdir handles the corresponding chat slash-command.
func (m Model) handleCmdMkdir(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/mkdir <path>`",
			Time:    time.Now(),
		})
	} else {
		path := parts[1]
		if err := makeDir(m.workspace, path); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to create directory: %v", err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Created directory: %s", path),
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdWrite handles the corresponding chat slash-command.
func (m Model) handleCmdWrite(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 3 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/write <path> <content>`",
			Time:    time.Now(),
		})
	} else {
		path := parts[1]
		content := strings.Join(parts[2:], " ")
		if err := writeFileContent(m.workspace, path, content); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to write file: %v", err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Wrote to file: %s", path),
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdSearch handles the corresponding chat slash-command.
func (m Model) handleCmdSearch(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/search <pattern>`",
			Time:    time.Now(),
		})
	} else {
		pattern := strings.Join(parts[1:], " ")
		matches, err := searchInFiles(m.workspace, pattern, 20)
		if err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Search failed: %v", err),
				Time:    time.Now(),
			})
		} else if len(matches) == 0 {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("No matches found for: %s", pattern),
				Time:    time.Now(),
			})
		} else {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("**Found %d matches for '%s':**\n\n", len(matches), pattern))
			for _, match := range matches {
				sb.WriteString(fmt.Sprintf("- %s\n", match))
			}
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: sb.String(),
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdPatch handles the corresponding chat slash-command.
func (m Model) handleCmdPatch(input string, parts []string) (tea.Model, tea.Cmd) {
	m.inputMode = InputModePatch
	m.pendingPatchLines = nil
	m.textarea.Placeholder = "Paste patch lines (type --END-- when done)..."
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Patch mode enabled. Paste your patch line by line, then type `--END--` to apply.",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdEdit handles the corresponding chat slash-command.
func (m Model) handleCmdEdit(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/edit <path>` - Opens file for inline editing",
			Time:    time.Now(),
		})
	} else {
		path := parts[1]
		content, err := readFileContent(m.workspace, path, 16000)
		if err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to read file for editing: %v", err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Editing %s:**\n\n```\n%s\n```\n\nUse `/write %s <new content>` to save changes.", path, content, path),
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdAppend handles the corresponding chat slash-command.
func (m Model) handleCmdAppend(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 3 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/append <path> <content>`",
			Time:    time.Now(),
		})
	} else {
		path := parts[1]
		content := strings.Join(parts[2:], " ")
		if err := appendFileContent(m.workspace, path, content+"\n"); err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to append to file: %v", err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Appended to file: %s", path),
				Time:    time.Now(),
			})
		}
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdPick handles the corresponding chat slash-command.
func (m Model) handleCmdPick(input string, parts []string) (tea.Model, tea.Cmd) {
	m.viewMode = FilePickerView
	m.textarea.Reset()
	return m, m.filepicker.Init()
}

// handleCmdDefineAgent handles the corresponding chat slash-command.
func (m Model) handleCmdDefineAgent(input string, parts []string) (tea.Model, tea.Cmd) {
	// Enter agent definition wizard
	m.setInputMode(InputModeAgentWizard)
	m.agentWizard = &AgentWizardState{Step: 0} // Start at step 0 (Name)
	m.textarea.Placeholder = "Enter agent name (e.g., 'RustExpert')..."
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "**Agent Creation Wizard**\n\nLet's define a new specialist agent.\n\n**Step 1:** What should we name this agent? (Alphanumeric, e.g., `RustExpert`, `SecurityAuditor`)",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdAgents handles the corresponding chat slash-command.
func (m Model) handleCmdAgents(input string, parts []string) (tea.Model, tea.Cmd) {
	// List defined agents
	agents := m.loadType3Agents()
	if len(agents) == 0 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "No agents defined yet. Use `/define-agent` to create one, or run `/init` to auto-create agents.",
			Time:    time.Now(),
		})
	} else {
		var sb strings.Builder
		sb.WriteString("## Defined Agents\n\n")
		sb.WriteString("| Name | Type | KB Size | Status |\n")
		sb.WriteString("|------|------|---------|--------|\n")
		for _, agent := range agents {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n", agent.Name, agent.Type, agent.KBSize, agent.Status))
		}
		sb.WriteString("\n*Use `/spawn <name> <task>` to spawn an agent*")
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: sb.String(),
			Time:    time.Now(),
		})
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdAlignment handles the corresponding chat slash-command.
func (m Model) handleCmdAlignment(input string, parts []string) (tea.Model, tea.Cmd) {
	// Run on-demand alignment check against project vision
	subject := "Current session state"
	if len(parts) > 1 {
		subject = strings.Join(parts[1:], " ")
	}
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: "Running Northstar alignment check...",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.runAlignmentCheck(subject))
}

// handleCmdSpawn handles the corresponding chat slash-command.
func (m Model) handleCmdSpawn(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 3 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/spawn <type> <task>`\n\nTypes: coder, researcher, reviewer, tester, or a defined agent name",
			Time:    time.Now(),
		})
	} else {
		shardType := parts[1]
		task := strings.Join(parts[2:], " ")
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Spawning %s shard for: %s", shardType, task),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.spawnShard(shardType, task))
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// handleCmdIngest handles the corresponding chat slash-command.
func (m Model) handleCmdIngest(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 3 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "Usage: `/ingest <agent> <path>`\n\nExample: `/ingest mangleexpert .claude/skills/mangle-programming/references`",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}

	agentName := parts[1]
	docPath := strings.Join(parts[2:], " ")
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Ingesting documents into %s: %s", agentName, docPath),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.ingestAgentDocs(agentName, docPath))
}

// handleCmdSecurity handles the corresponding chat slash-command.
func (m Model) handleCmdSecurity(input string, parts []string) (tea.Model, tea.Cmd) {
	target := "."
	if len(parts) > 1 {
		target = parts[1]
	}
	task := formatShardTask("/security", target, "", m.workspace)
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Running security analysis on: %s", target),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.spawnShard("reviewer", task))
}

// handleCmdAnalyze handles the corresponding chat slash-command.
func (m Model) handleCmdAnalyze(input string, parts []string) (tea.Model, tea.Cmd) {
	target := "."
	if len(parts) > 1 {
		target = parts[1]
	}
	task := formatShardTask("/analyze", target, "", m.workspace)
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Running complexity analysis on: %s", target),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.spawnShard("reviewer", task))
}

// handleCmdTest handles the corresponding chat slash-command.
func (m Model) handleCmdTest(input string, parts []string) (tea.Model, tea.Cmd) {
	target := "run"
	if len(parts) > 1 {
		target = strings.Join(parts[1:], " ")
	}
	task := formatShardTask("/test", target, "", m.workspace)
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: fmt.Sprintf("Running test task: %s (with specialist matching)", task),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	m.isLoading = true
	// Use specialist-aware spawning for /test
	return m, tea.Batch(m.spinner.Tick, m.spawnShardWithSpecialists("/test", "tester", task, target))
}

// handleCmdFix handles the corresponding chat slash-command.
