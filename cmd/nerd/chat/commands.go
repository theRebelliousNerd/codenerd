// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains command handling for the chat interface.
//
// File Index (modularized):
//
//	commands.go            - Main command dispatcher (handleCommand switch)
//	help_renderer.go       - Experience-level aware /help rendering
//	command_categories.go  - /help command registry (single source of truth)
//	commands_tools.go      - Tool/status helpers (buildStatusReport, handleCleanupToolsCommand)
//	commands_evolution.go  - Prompt Evolution helpers (renderEvolutionStats, runEvolutionCycle)
//
// Command Categories (within handleCommand switch):
//
//	Session:    /quit, /exit, /continue, /usage, /clear, /reset, /new-session, /sessions
//	Help:       /help, /status
//	Init:       /init, /scan, /refresh-docs, /scan-path, /scan-dir
//	Config:     /config, /embedding, /features
//	Files:      /read, /mkdir, /write, /search, /patch, /edit, /append, /pick
//	Agents:     /define-agent, /northstar, /learn, /agents, /spawn, /ingest
//	Analysis:   /review, /security, /analyze, /test, /fix, /refactor
//	Campaigns:  /legislate, /clarify, /launchcampaign, /campaign
//	Query:      /query, /why, /logic, /glassbox, /transparency, /shadow, /whatif
//	Review:     /approve, /reject-finding, /accept-finding, /review-accuracy
//	Tools:      /tool, /jit, /cleanup-tools
//	Evolution:  /evolve, /evolution-stats, /evolved-atoms, /promote-atom, /reject-atom, /strategies
package chat

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// COMMAND HANDLING
// =============================================================================
// handleCommand processes all /command inputs from the user.
// Commands are organized by category: session, config, shard, query, campaign.

func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	// Sanitize: strip null bytes and ANSI escapes before processing
	input = sanitizeCommandInput(input)
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}
	cmd := parts[0]

	switch cmd {
	case "/quit", "/exit", "/q":
		return m, tea.Quit

	case "/continue", "/resume":
		return m.handleCmdContinue(input, parts)

	case "/usage":
		return m.handleCmdUsage(input, parts)

	case "/clear":
		return m.handleCmdClear(input, parts)

	case "/reset":
		return m.handleCmdReset(input, parts)

	case "/model":
		return m.handleCmdModel(input, parts)

	case "/new-session":
		return m.handleCmdNewSession(input, parts)

	case "/sessions":
		return m.handleCmdSessions(input, parts)

	case "/load-session":
		return m.handleCmdLoadSession(input, parts)

	case "/help", "/h", "/?":
		return m.handleCmdHelp(input, parts)

	case "/status":
		return m.handleCmdStatus(input, parts)

	case "/reflection":
		return m.handleCmdReflection(input, parts)

	case "/knowledge":
		return m.handleCmdKnowledge(input, parts)

	case "/legislate":
		return m.handleCmdLegislate(input, parts)

	case "/clarify":
		return m.handleCmdClarify(input, parts)

	case "/launchcampaign":
		return m.handleCmdLaunchCampaign(input, parts)

	case "/init":
		return m.handleCmdInit(input, parts)
	case "/scan":
		return m.handleCmdScan(input, parts)

	case "/refresh-docs", "/scan-docs":
		return m.handleCmdRefreshDocs(input, parts)

	case "/scan-path":
		return m.handleCmdScanPath(input, parts)

	case "/scan-dir":
		return m.handleCmdScanDir(input, parts)

	case "/features":
		return m.handleCmdFeatures(input, parts)

	case "/config":
		return m.handleConfigCommand(input, parts)
	case "/embedding":
		return m.handleEmbeddingCommand(input, parts)
	case "/read":
		return m.handleCmdRead(input, parts)

	case "/mkdir":
		return m.handleCmdMkdir(input, parts)

	case "/write":
		return m.handleCmdWrite(input, parts)

	case "/search":
		return m.handleCmdSearch(input, parts)

	case "/patch":
		return m.handleCmdPatch(input, parts)

	case "/edit":
		return m.handleCmdEdit(input, parts)

	case "/append":
		return m.handleCmdAppend(input, parts)

	case "/pick":
		return m.handleCmdPick(input, parts)

	case "/define-agent", "/agent":
		return m.handleCmdDefineAgent(input, parts)

	case "/northstar", "/vision", "/spec":
		return m.handleNorthstarCommand(input, parts)
	case "/learn":
		return m.handleLearnCommand(input, parts)
	case "/agents":
		return m.handleCmdAgents(input, parts)

	case "/alignment", "/align":
		return m.handleCmdAlignment(input, parts)

	case "/spawn":
		return m.handleCmdSpawn(input, parts)

	case "/ingest":
		return m.handleCmdIngest(input, parts)

	case "/review":
		return m.handleReviewCommand(input, parts)
	case "/security":
		return m.handleCmdSecurity(input, parts)

	case "/analyze":
		return m.handleCmdAnalyze(input, parts)

	case "/test":
		return m.handleCmdTest(input, parts)

	case "/fix":
		return m.handleCmdFix(input, parts)

	case "/refactor":
		return m.handleCmdRefactor(input, parts)

	case "/query":
		return m.handleQueryCommand(input, parts)
	case "/why":
		return m.handleCmdWhy(input, parts)

	case "/explain":
		return m.handleExplainCommand(input, parts)
	case "/explain-off":
		return m.handleCmdExplainOff(input, parts)

	case "/logic":
		return m.handleCmdLogic(input, parts)

	case "/glassbox":
		return m.handleCmdGlassbox(input, parts)

	case "/transparency":
		return m.handleTransparencyCommand(input, parts)
	case "/shadow":
		return m.handleCmdShadow(input, parts)

	case "/whatif":
		return m.handleCmdWhatif(input, parts)

	case "/approve":
		return m.handleCmdApprove(input, parts)

	case "/reject-finding":
		return m.handleRejectFindingCommand(input, parts)
	case "/accept-finding":
		return m.handleAcceptFindingCommand(input, parts)
	case "/review-accuracy":
		return m.handleCmdReviewAccuracy(input, parts)

	case "/campaign":
		return m.handleCampaignCommand(input, parts)
	case "/tool":
		return m.handleToolCommand(input, parts)
	case "/jit":
		return m.handleCmdJit(input, parts)

	case "/cleanup-tools":
		return m.handleCmdCleanupTools(input, parts)

	// =============================================================================
	// PROMPT EVOLUTION COMMANDS (System Prompt Learning)
	// =============================================================================

	case "/evolve":
		return m.handleCmdEvolve(input, parts)

	case "/evolution-stats":
		return m.handleCmdEvolutionStats(input, parts)

	case "/evolved-atoms":
		return m.handleCmdEvolvedAtoms(input, parts)

	case "/strategies":
		return m.handleCmdStrategies(input, parts)

	case "/promote-atom":
		return m.handlePromoteAtomCommand(input, parts)
	case "/reject-atom":
		return m.handleCmdRejectAtom(input, parts)

	default:
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Unknown command: %s. Type `/help` for available commands.", cmd),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		return m, nil
	}
}

func setModelRecursive(client perception.LLMClient, model string) {
	if client == nil {
		return
	}
	if setter, ok := client.(interface{ SetModel(string) }); ok {
		setter.SetModel(model)
	}
	if sched, ok := client.(*core.ScheduledLLMCall); ok {
		setModelRecursive(sched.Client, model)
	}
	if tc, ok := client.(*perception.TracingLLMClient); ok {
		setModelRecursive(tc.GetUnderlying(), model)
	}
}

func getModelRecursive(client perception.LLMClient) string {
	if client == nil {
		return ""
	}
	if getter, ok := client.(interface{ GetModel() string }); ok {
		return getter.GetModel()
	}
	if sched, ok := client.(*core.ScheduledLLMCall); ok {
		return getModelRecursive(sched.Client)
	}
	if tc, ok := client.(*perception.TracingLLMClient); ok {
		return getModelRecursive(tc.GetUnderlying())
	}
	return ""
}
