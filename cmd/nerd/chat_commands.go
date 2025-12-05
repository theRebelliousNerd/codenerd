// Package main provides the codeNERD CLI entry point.
// This file contains command handling for the chat interface.
package main

import (
	"codenerd/cmd/nerd/config"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/perception"
	"codenerd/internal/campaign"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// COMMAND HANDLING
// =============================================================================
// handleCommand processes all /command inputs from the user.
// Commands are organized by category: session, config, shard, query, campaign.

func (m chatModel) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/quit", "/exit", "/q":
		return m, tea.Quit

	case "/clear":
		m.history = []chatMessage{}
		m.viewport.SetContent("")
		m.textinput.Reset()
		// Save empty history
		m.saveSessionState()
		return m, nil

	case "/new-session":
		// Start a completely new session with fresh ID
		m.history = []chatMessage{}
		m.sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
		m.turnCount = 0
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🆕 Started new session: `%s`\n\nPrevious history saved.", m.sessionID),
			time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
		m.saveSessionState()
		return m, nil

	case "/sessions":
		// List available sessions
		sessions, err := nerdinit.ListSessionHistories(m.workspace)
		if err != nil || len(sessions) == 0 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "No saved sessions found.",
				time:    time.Now(),
			})
		} else {
			var sb strings.Builder
			sb.WriteString("## 📜 Saved Sessions\n\n")
			for _, sess := range sessions {
				current := ""
				if sess == m.sessionID {
					current = " *(current)*"
				}
				sb.WriteString(fmt.Sprintf("- `%s`%s\n", sess, current))
			}
			sb.WriteString("\n*Use `/load-session <id>` to restore a session*")
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: sb.String(),
				time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textinput.Reset()
		return m, nil

	case "/help":
		help := `## Available Commands

| Command | Description |
|---------|-------------|
| /help | Show this help message |
| /clear | Clear chat history |
| /new-session | Start a fresh session (preserves old) |
| /sessions | List saved sessions |
| /status | Show system status |
| /init | Initialize codeNERD in the workspace |
| /init --force | Reinitialize (preserves learned preferences) |
| /scan | Refresh codebase index without full reinit |
| /config set-key <key> | Set API key |
| /config set-theme <theme> | Set theme (light/dark) |
| /spawn <type> <task> | Spawn a shard agent |
| /define-agent <name> | Define a Type 4 specialist agent |
| /agents | List all defined agents |
| /query <predicate> | Query the Mangle kernel |
| /why [predicate] | Explain logic derivation |
| /logic [predicate] | Show derivation trace in Glass Box |
| /shadow <action> | Start Shadow Mode simulation |
| /whatif <action> | Quick counterfactual query |
| /approve | Review pending mutations (Interactive Diff) |
| /quit, /exit, /q | Exit the CLI |

## Quick Actions (Convenience Commands)
| Command | Description |
|---------|-------------|
| /review [file] | Code review (all files if no arg) |
| /security [file] | Security vulnerability scan |
| /analyze [file] | Code complexity/quality analysis |
| /test [file] | Run tests |
| /fix <issue> | Fix a bug or issue |
| /refactor <target> | Refactor code |

## Natural Language
You can also just type naturally! Examples:
- "review this file" → triggers code review
- "check for security issues" → security scan
- "what does this function do" → explanation
- "fix the bug in auth.go" → fix attempt
- "refactor the handler" → refactoring

## Campaign Orchestration
| Command | Description |
|---------|-------------|
| /campaign start <goal> | Start a new long-running campaign |
| /campaign status | Show active campaign progress |
| /campaign pause | Pause the active campaign |
| /campaign resume | Resume a paused campaign |
| /campaign list | List all campaigns |
| Ctrl+P | Toggle campaign progress panel |

## Glass Box Interface (Split-Pane TUI)
| Keybinding | Description |
|------------|-------------|
| Ctrl+L | Toggle logic pane on/off |
| Ctrl+G | Cycle views: Chat → Split → Logic |
| Ctrl+R | Toggle focus between panes |

## Shard Types
| Type | Lifecycle | Use Case |
|------|-----------|----------|
| Type 1 (System) | Always On | Core functions |
| Type 2 (Ephemeral) | Spawn->Die | Quick tasks |
| Type 3 (Persistent) | LLM-Created | Background monitoring |
| Type 4 (User) | User-Defined | Domain experts |

## Tips
- **Enter** to send a message
- **Ctrl+C** or **Esc** to exit
- Use **↑/↓** to scroll history
`
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: help,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/config":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/config set-key <key>` or `/config set-theme <light|dark>`",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		subCmd := parts[1]
		switch subCmd {
		case "set-key":
			if len(parts) < 3 {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "Usage: `/config set-key <key>`",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}
			key := parts[2]
			m.config.APIKey = key
			if err := config.Save(m.config); err != nil {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: fmt.Sprintf("Error saving config: %v", err),
					time:    time.Now(),
				})
			} else {
				// Ensure config directory exists (Save already creates it) and inform user where it lives
				// Re-initialize client
				m.client = perception.NewZAIClient(key)
				m.transducer = perception.NewRealTransducer(m.client)
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "✅ API key saved to ~/.codenerd/config.json and client updated.",
					time:    time.Now(),
				})
			}

		case "set-theme":
			if len(parts) < 3 {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "Usage: `/config set-theme <light|dark>`",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}
			theme := parts[2]
			if theme != "light" && theme != "dark" {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "Invalid theme. Use 'light' or 'dark'.",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}
			m.config.Theme = theme
			if err := config.Save(m.config); err != nil {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: fmt.Sprintf("Error saving config: %v", err),
					time:    time.Now(),
				})
			} else {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: fmt.Sprintf("✅ Theme set to '%s'. Restart CLI to apply.", theme),
					time:    time.Now(),
				})
			}
		}

		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/status":
		status := fmt.Sprintf(`## System Status

- **Workspace**: %s
- **Kernel**: Active
- **Shards**: %d active
- **Session**: %s (Turn %d)
- **Time**: %s
- **Config**: %s
`, m.workspace, len(m.shardMgr.GetActiveShards()), m.sessionID[:16], m.turnCount, time.Now().Format(time.RFC3339), func() string {
			path, _ := config.ConfigFile()
			return path
		}())

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: status,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/read":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/read <path>`",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		target := strings.Join(parts[1:], " ")
		content, err := readFileContent(m.workspace, target, 8000)
		resp := ""
		if err != nil {
			resp = fmt.Sprintf("Error reading `%s`: %v", target, err)
		} else {
			resp = fmt.Sprintf("### %s\n```\n%s\n```", target, content)
		}
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: resp,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/mkdir":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/mkdir <path>`",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		target := strings.Join(parts[1:], " ")
		if err := makeDir(m.workspace, target); err != nil {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("Error creating directory `%s`: %v", target, err),
				time:    time.Now(),
			})
		} else {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("✅ Created directory `%s`", target),
				time:    time.Now(),
			})
		}
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/write":
		if len(parts) < 3 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/write <path> <content>`",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		target := parts[1]
		content := strings.Join(parts[2:], " ")
		if err := writeFileContent(m.workspace, target, content); err != nil {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("Error writing `%s`: %v", target, err),
				time:    time.Now(),
			})
		} else {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("✅ Wrote `%s` (%d bytes)", target, len(content)),
				time:    time.Now(),
			})
		}
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/search":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/search <pattern> [path]` (path defaults to workspace)",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		pattern := parts[1]
		root := m.workspace
		if len(parts) >= 3 {
			root = resolvePath(m.workspace, strings.Join(parts[2:], " "))
		}
		matches, err := searchInFiles(root, pattern, 20)
		var resp strings.Builder
		if err != nil {
			resp.WriteString(fmt.Sprintf("Search error: %v", err))
		} else if len(matches) == 0 {
			resp.WriteString(fmt.Sprintf("No matches for `%s` in `%s`", pattern, root))
		} else {
			resp.WriteString(fmt.Sprintf("Matches for `%s`:\n", pattern))
			for _, mpath := range matches {
				resp.WriteString(fmt.Sprintf("- %s\n", mpath))
			}
		}
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: resp.String(),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/patch":
		// Enter patch collection mode: user will paste a unified diff, end with a line containing only `--END--`
		m.awaitingAgentDefinition = false
		m.textinput.Placeholder = "Paste unified diff, end with a line containing --END--"
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: "🔧 Paste your unified diff now. End input with a line containing `--END--`.",
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.awaitingPatch = true
		return m, nil

	case "/edit":
		if len(parts) < 3 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/edit <path> <content>`",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		target := parts[1]
		content := strings.Join(parts[2:], " ")
		if err := writeFileContent(m.workspace, target, content); err != nil {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("Error editing `%s`: %v", target, err),
				time:    time.Now(),
			})
		} else {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("✅ Wrote `%s` (%d bytes)", target, len(content)),
				time:    time.Now(),
			})
		}
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/append":
		if len(parts) < 3 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/append <path> <content>`",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		target := parts[1]
		content := strings.Join(parts[2:], " ")
		if err := appendFileContent(m.workspace, target, content); err != nil {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("Error appending `%s`: %v", target, err),
				time:    time.Now(),
			})
		} else {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("✅ Appended to `%s` (+%d bytes)", target, len(content)),
				time:    time.Now(),
			})
		}
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/init":
		// Check if already initialized
		if nerdinit.IsInitialized(m.workspace) {
			// Check for --force flag
			forceReinit := len(parts) > 1 && parts[1] == "--force"
			if !forceReinit {
				m.history = append(m.history, chatMessage{
					role: "assistant",
					content: `⚠️ **Workspace already initialized**

The ` + "`.nerd/`" + ` directory already exists with a project profile.

Options:
- Use ` + "`/init --force`" + ` to reinitialize (preserves learned preferences)
- Use ` + "`/scan`" + ` to refresh the codebase index
- Use ` + "`/agents`" + ` to see available agents`,
					time: time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}
		}

		// Run comprehensive initialization in background
		m.history = append(m.history, chatMessage{
			role: "assistant",
			content: `🚀 **Initializing codeNERD in workspace...**

This comprehensive initialization will:
1. 📁 Create ` + "`.nerd/`" + ` directory structure
2. 📊 Deep scan the codebase for project profile
3. 🔬 Run Researcher shard for analysis
4. 🧠 Generate initial Mangle facts
5. 🤖 Determine & create Type 3 agents
6. 📚 Build knowledge bases for each agent
7. ⚙️ Initialize preferences & session state
8. 📖 Initialize learning store for Autopoiesis

_This may take a minute for large codebases..._`,
			time: time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.runInit(),
		)

	case "/scan":
		// Refresh codebase index without full reinit
		if !nerdinit.IsInitialized(m.workspace) {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "⚠️ Workspace not initialized. Run `/init` first to set up the `.nerd/` directory.",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		m.history = append(m.history, chatMessage{
			role: "assistant",
			content: `🔍 **Scanning codebase...**

Refreshing the codebase index:
1. 📁 Scanning file structure
2. 🌳 Extracting AST symbols
3. 🔗 Updating dependency graph
4. 🧠 Asserting facts to kernel

_This may take a moment for large codebases..._`,
			time: time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.runScan(),
		)

	case "/define-agent":
		if len(parts) < 2 {
			m.awaitingAgentDefinition = true
			m.textinput.Placeholder = "Describe the specialist you want (domain, tasks, constraints)..."
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "🧠 Creating a specialist agent.\n\nTell me what you need (domain, tasks, constraints). I’ll propose a name/topic and wire up its knowledge shard.",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		agentName := parts[1]
		topic := ""
		for i, p := range parts {
			if p == "--topic" && i+1 < len(parts) {
				topic = strings.Join(parts[i+1:], " ")
				break
			}
		}

		// Define the agent profile (Type 4: User Configured)
		config := core.DefaultSpecialistConfig(agentName, fmt.Sprintf(".nerd/shards/%s_knowledge.db", agentName))
		m.shardMgr.DefineProfile(agentName, config)
		_ = persistAgentProfile(m.workspace, agentName, "persistent", config.KnowledgePath, 0, "ready")

		response := fmt.Sprintf(`## Agent Defined: %s

**Type**: 4 (User Configured - Persistent Specialist)
**Topic**: %s
**Knowledge Path**: %s
**Model**: High Reasoning (glm4)

The agent will undergo deep research on first spawn to build its knowledge base.

**Next steps:**
- Run research: `+"`/spawn researcher %s research`"+`
- Use the agent: `+"`/spawn %s <task>`", agentName, topic, config.KnowledgePath, topic, agentName)

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: response,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/agents":
		var sb strings.Builder
		sb.WriteString("## Defined Agents\n\n")

		// Built-in agents (Type 2: Ephemeral)
		sb.WriteString("### Built-in (Type 2: Ephemeral)\n")
		sb.WriteString("| Name | Capabilities |\n")
		sb.WriteString("|------|-------------|\n")
		sb.WriteString("| researcher | Deep web research, codebase analysis |\n")
		sb.WriteString("| coder | Code generation, refactoring |\n")
		sb.WriteString("| reviewer | Code review, best practices |\n")
		sb.WriteString("| tester | Test generation, TDD loop |\n\n")

		// Type 3 agents (LLM-Created Persistent)
		type3Agents := m.loadType3Agents()
		if len(type3Agents) > 0 {
			sb.WriteString("### Auto-Created (Type 3: Persistent)\n")
			sb.WriteString("| Name | KB Size | Status |\n")
			sb.WriteString("|------|---------|--------|\n")
			for _, agent := range type3Agents {
				sb.WriteString(fmt.Sprintf("| %s | %d atoms | %s |\n", agent.Name, agent.KBSize, agent.Status))
			}
			sb.WriteString("\n")
		}

		// User-defined agents (Type 4)
		sb.WriteString("### User-Defined (Type 4: Specialist)\n")
		profiles := m.getDefinedProfiles()
		if len(profiles) == 0 {
			sb.WriteString("_No user-defined agents. Use `/define-agent <name>` to create one._\n")
		} else {
			sb.WriteString("| Name | Knowledge Path |\n")
			sb.WriteString("|------|---------------|\n")
			for name, cfg := range profiles {
				sb.WriteString(fmt.Sprintf("| %s | %s |\n", name, cfg.KnowledgePath))
			}
		}

		sb.WriteString("\n### Commands\n")
		sb.WriteString("- Spawn agent: `/spawn <agent> <task>`\n")
		sb.WriteString("- Define new: `/define-agent <name> --topic <topic>`\n")

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: sb.String(),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/spawn":
		if len(parts) < 3 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/spawn <agent-type> <task>`\n\nExamples:\n```\n/spawn researcher \"analyze auth system\"\n/spawn coder \"implement user login\"\n/spawn RustExpert \"review async code\"\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		shardType := parts[1]
		task := strings.Join(parts[2:], " ")

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔄 Spawning **%s** shard for task: %s", shardType, task),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.spawnShard(shardType, task),
		)

	case "/review":
		// Convenience command for code review
		target := "all"
		if len(parts) > 1 {
			target = "file:" + strings.Join(parts[1:], " ")
		}
		task := "review " + target

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔍 Starting code review: %s", target),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.spawnShard("reviewer", task),
		)

	case "/security":
		// Convenience command for security scan
		target := "all"
		if len(parts) > 1 {
			target = "file:" + strings.Join(parts[1:], " ")
		}
		task := "security_scan " + target

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔒 Starting security scan: %s", target),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.spawnShard("reviewer", task),
		)

	case "/analyze":
		// Convenience command for code analysis (complexity, style, etc.)
		target := "all"
		if len(parts) > 1 {
			target = "file:" + strings.Join(parts[1:], " ")
		}
		task := "complexity " + target

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("📊 Starting code analysis: %s", target),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.spawnShard("reviewer", task),
		)

	case "/test":
		// Convenience command for running tests
		task := "run_tests"
		if len(parts) > 1 {
			task = "run_tests file:" + strings.Join(parts[1:], " ")
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: "🧪 Running tests...",
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.spawnShard("tester", task),
		)

	case "/fix":
		// Convenience command for fixing issues
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/fix <description or file:path>`\n\nExamples:\n```\n/fix the null pointer in auth.go\n/fix file:src/main.go\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		task := strings.Join(parts[1:], " ")
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔧 Attempting fix: %s", task),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.spawnShard("coder", "fix "+task),
		)

	case "/refactor":
		// Convenience command for refactoring
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/refactor <file or description>`\n\nExamples:\n```\n/refactor src/main.go\n/refactor the authentication module\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		task := strings.Join(parts[1:], " ")
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔄 Refactoring: %s", task),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.spawnShard("coder", "refactor "+task),
		)

	case "/query":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/query <predicate>`\n\nExamples:\n```\n/query next_action\n/query impacted\n/query block_commit\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		predicate := parts[1]
		facts, err := m.kernel.Query(predicate)

		var response string
		if err != nil {
			response = fmt.Sprintf("Query error: %v", err)
		} else if len(facts) == 0 {
			response = fmt.Sprintf("No facts found for `%s`", predicate)
		} else {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("## Query: %s\n\n", predicate))
			sb.WriteString("```datalog\n")
			for _, fact := range facts {
				sb.WriteString(fact.String() + "\n")
			}
			sb.WriteString("```\n")
			response = sb.String()
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: response,
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/why":
		predicate := "next_action"
		if len(parts) >= 2 {
			predicate = parts[1]
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## Explaining: %s\n\n", predicate))

		facts, _ := m.kernel.Query(predicate)
		if len(facts) == 0 {
			sb.WriteString("No facts derived for this predicate.\n\n")
			sb.WriteString("**Possible reasons:**\n")
			sb.WriteString("- Required preconditions not met\n")
			sb.WriteString("- No matching rules triggered\n")
			sb.WriteString("- Workspace not scanned\n")
		} else {
			sb.WriteString("**Derived facts:**\n```datalog\n")
			for _, fact := range facts {
				sb.WriteString(fact.String() + "\n")
			}
			sb.WriteString("```\n\n")

			// Show related rules
			sb.WriteString("**Related policy rules:**\n")
			switch predicate {
			case "next_action":
				sb.WriteString("```datalog\nnext_action(A) :- user_intent(_, V, T, _), action_mapping(V, A).\nnext_action(/ask_user) :- clarification_needed(_).\nnext_action(/interrogative_mode) :- ambiguity_detected(_).\n```")
			case "block_commit":
				sb.WriteString("```datalog\nblock_commit(\"Build Broken\") :- diagnostic(/error, _, _, _, _).\nblock_commit(\"Tests Failing\") :- test_state(/failing).\n```")
			case "impacted":
				sb.WriteString("```datalog\nimpacted(X) :- dependency_link(X, Y, _), modified(Y).\nimpacted(X) :- dependency_link(X, Z, _), impacted(Z). # Transitive\n```")
			case "clarification_needed":
				sb.WriteString("```datalog\nclarification_needed(Ref) :- focus_resolution(Ref, _, _, Score), Score < 0.85.\nclarification_needed(File) :- chesterton_fence_warning(File, _).\n```")
			default:
				sb.WriteString("_(See policy.gl for rule definitions)_")
			}
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: sb.String(),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/logic":
		// Show derivation trace in the Glass Box pane
		predicate := "next_action"
		if len(parts) >= 2 {
			predicate = parts[1]
		}

		// Query the kernel
		facts, _ := m.kernel.Query(predicate)

		// Build derivation trace
		trace := m.buildDerivationTrace(predicate, facts)

		// Update the logic pane
		if m.splitPane != nil && m.splitPane.RightPane != nil {
			m.splitPane.RightPane.SetTrace(trace)
		}
		if m.logicPane != nil {
			m.logicPane.SetTrace(trace)
		}

		// Enable split view if not already enabled
		if !m.showLogic {
			m.showLogic = true
			m.paneMode = ui.ModeSplitPane
			m.splitPane.SetMode(ui.ModeSplitPane)
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔬 Showing derivation trace for `%s` in the Glass Box pane.\n\nUse **Ctrl+L** to toggle the logic view.", predicate),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/shadow":
		// Start Shadow Mode simulation
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/shadow <action-type> <target>`\n\n**Action Types:**\n- `write <file>` - Simulate file modification\n- `delete <file>` - Simulate file deletion\n- `refactor <file>` - Simulate refactoring\n- `commit` - Simulate git commit\n\n**Examples:**\n```\n/shadow write src/auth/handler.go\n/shadow refactor internal/core/kernel.go\n/shadow commit\n```",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		actionType := parts[1]
		target := ""
		if len(parts) >= 3 {
			target = strings.Join(parts[2:], " ")
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🌑 **Starting Shadow Mode Simulation**\n\nAction: `%s`\nTarget: `%s`\n\n_Running counterfactual analysis..._", actionType, target),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.runShadowSimulation(actionType, target),
		)

	case "/whatif":
		// Quick counterfactual query
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: "Usage: `/whatif <scenario>`\n\n**Examples:**\n```\n/whatif I delete auth/handler.go\n/whatif I refactor the login function\n/whatif tests fail after this change\n```\n\nThis runs a quick counterfactual analysis without starting a full simulation.",
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		scenario := strings.Join(parts[1:], " ")

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("🔮 **What-If Analysis**\n\nScenario: _\"%s\"_\n\n_Projecting effects..._", scenario),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.isLoading = true

		return m, tea.Batch(
			m.spinner.Tick,
			m.runWhatIfQuery(scenario),
		)

	case "/approve":
		// Interactive Diff Approval
		// Query for pending mutations requiring approval
		pendingMutations, _ := m.kernel.Query("pending_mutation")
		requiresApproval, _ := m.kernel.Query("requires_approval")

		var sb strings.Builder
		sb.WriteString("## 📝 Interactive Diff Approval\n\n")

		if len(pendingMutations) == 0 {
			sb.WriteString("✅ **No pending mutations** - All changes have been reviewed or there are no pending changes.\n\n")
			sb.WriteString("Mutations require approval when:\n")
			sb.WriteString("- Chesterton's Fence warning is triggered (recent changes by others)\n")
			sb.WriteString("- Code impacts other files transitively\n")
			sb.WriteString("- Shadow Mode simulation detected potential issues\n")
		} else {
			sb.WriteString(fmt.Sprintf("Found **%d pending mutation(s)** requiring review:\n\n", len(pendingMutations)))

			sb.WriteString("| # | File | Reason |\n")
			sb.WriteString("|---|------|--------|\n")
			for i, mutation := range pendingMutations {
				file := "unknown"
				if len(mutation.Args) > 1 {
					file = fmt.Sprintf("%v", mutation.Args[1])
				}
				reason := "approval_required"
				for _, ra := range requiresApproval {
					if len(ra.Args) > 0 && fmt.Sprintf("%v", ra.Args[0]) == fmt.Sprintf("%v", mutation.Args[0]) {
						reason = "safety_check"
					}
				}
				sb.WriteString(fmt.Sprintf("| %d | %s | %s |\n", i+1, file, reason))
			}

			sb.WriteString("\n### Approval Commands\n\n")
			sb.WriteString("```\n")
			sb.WriteString("/approve accept <id>  - Approve a specific mutation\n")
			sb.WriteString("/approve reject <id>  - Reject a specific mutation\n")
			sb.WriteString("/approve all          - Approve all pending mutations\n")
			sb.WriteString("/approve clear        - Clear all pending mutations\n")
			sb.WriteString("```\n")
		}

		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: sb.String(),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case "/campaign":
		if len(parts) < 2 {
			m.history = append(m.history, chatMessage{
				role: "assistant",
				content: `## Campaign Orchestration

Usage: ` + "`/campaign <subcommand> [args]`" + `

| Subcommand | Description |
|------------|-------------|
| start <goal> | Start a new campaign with the given goal |
| status | Show active campaign progress |
| pause | Pause the active campaign |
| resume | Resume a paused campaign |
| list | List all campaigns |

**Examples:**
` + "```" + `
/campaign start "Build a user authentication system"
/campaign start --type greenfield "Create a REST API for inventory management"
/campaign status
/campaign pause
` + "```",
				time: time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		subCmd := parts[1]
		switch subCmd {
		case "start":
			if len(parts) < 3 {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "Usage: `/campaign start <goal>`\n\nExample: `/campaign start \"Build authentication system\"`",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}

			// Parse campaign type if specified
			campaignType := campaign.CampaignTypeFeature
			goal := strings.Join(parts[2:], " ")

			for i, p := range parts {
				if p == "--type" && i+1 < len(parts) {
					switch parts[i+1] {
					case "greenfield":
						campaignType = campaign.CampaignTypeGreenfield
					case "feature":
						campaignType = campaign.CampaignTypeFeature
					case "migration":
						campaignType = campaign.CampaignTypeMigration
					case "stabilization":
						campaignType = campaign.CampaignTypeAudit
					case "refactor":
						campaignType = campaign.CampaignTypeRemediation
					}
					// Remove type flag from goal
					goal = strings.Join(append(parts[2:i], parts[i+2:]...), " ")
					break
				}
			}

			// Clean up goal (remove quotes)
			goal = strings.Trim(goal, "\"'")

			m.history = append(m.history, chatMessage{
				role: "assistant",
				content: fmt.Sprintf(`## 🚀 Starting Campaign

**Goal**: %s
**Type**: %s

_Analyzing goal and creating execution plan..._`, goal, campaignType),
				time: time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.isLoading = true

			return m, tea.Batch(
				m.spinner.Tick,
				m.startCampaign(goal, campaignType),
			)

		case "status":
			if m.activeCampaign == nil {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "No active campaign. Start one with `/campaign start <goal>`",
					time:    time.Now(),
				})
			} else {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: m.renderCampaignStatus(),
					time:    time.Now(),
				})
				m.showCampaignPanel = true
			}
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil

		case "pause":
			if m.activeCampaign == nil {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "No active campaign to pause.",
					time:    time.Now(),
				})
			} else {
				m.activeCampaign.Status = campaign.StatusPaused
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: fmt.Sprintf("⏸️ Campaign **%s** paused.\n\nResume with `/campaign resume`", m.activeCampaign.Title),
					time:    time.Now(),
				})
			}
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil

		case "resume":
			if m.activeCampaign == nil {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "No paused campaign to resume.",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}
			if m.activeCampaign.Status != campaign.StatusPaused {
				m.history = append(m.history, chatMessage{
					role:    "assistant",
					content: "Campaign is not paused.",
					time:    time.Now(),
				})
				m.textinput.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				return m, nil
			}

			m.activeCampaign.Status = campaign.StatusActive
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("▶️ Campaign **%s** resumed.\n\n_Continuing execution..._", m.activeCampaign.Title),
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.isLoading = true

			return m, tea.Batch(
				m.spinner.Tick,
				m.resumeCampaign(),
			)

		case "list":
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: m.renderCampaignList(),
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil

		default:
			m.history = append(m.history, chatMessage{
				role:    "assistant",
				content: fmt.Sprintf("Unknown campaign subcommand: `%s`. Use `/campaign` for usage.", subCmd),
				time:    time.Now(),
			})
			m.textinput.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

	default:
		m.history = append(m.history, chatMessage{
			role:    "assistant",
			content: fmt.Sprintf("Unknown command: `%s`. Type `/help` for available commands.", cmd),
			time:    time.Now(),
		})
		m.textinput.Reset()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}
}

