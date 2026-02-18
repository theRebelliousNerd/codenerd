package chat

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/auth/antigravity"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/transparency"
	"codenerd/internal/ux"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Performance instrumentation: Track slow Update() handlers
	startTime := time.Now()
	defer func() {
		elapsed := time.Since(startTime)
		// Warn if Update() takes > 100ms (Bubbletea target is 60fps = 16ms per frame)
		if elapsed > 100*time.Millisecond {
			msgType := fmt.Sprintf("%T", msg)
			logging.Get(logging.CategoryPerformance).Warn(
				"Slow Update() handler: %s took %v (target: <100ms)",
				msgType, elapsed,
			)
		}
	}()

	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		var handled bool
		m, tiCmd, handled = m.handleKeyMsg(msg)
		if handled {
			return m, tiCmd
		}

	case windowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 4
		footerHeight := 3
		inputHeight := 3   // Smaller input height for textinput
		paddingHeight := 2 // Extra padding for safety

		// Calculate layout
		chatWidth := msg.Width - 4
		if m.showLogic {
			logicWidth := msg.Width / 3
			chatWidth = msg.Width - logicWidth - 4 // minus padding/borders
		}
		if chatWidth < 1 {
			chatWidth = 1
		}

		errorPanelHeight := 0
		if m.err != nil && m.showError {
			// 1 header line + viewport height + 2 border lines
			errorPanelHeight = 1 + errorPanelViewportHeight + 2
		}

		calcHeight := msg.Height - headerHeight - footerHeight - inputHeight - paddingHeight - errorPanelHeight
		if calcHeight < 1 {
			calcHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(chatWidth, calcHeight)
			m.ready = true
		} else {
			m.viewport.Width = chatWidth
			m.viewport.Height = calcHeight
		}

		// Error viewport lives inside a bordered box within the content area.
		// Box uses 1-col padding left/right plus 1-col border left/right => total 4 cols.
		m.errorVP.Width = chatWidth - 4
		if m.errorVP.Width < 1 {
			m.errorVP.Width = 1
		}
		m.errorVP.Height = errorPanelViewportHeight
		if m.err != nil {
			m.refreshErrorViewport()
		}

		// Reduce input width to accommodate border (2) + padding (2) + safety margin
		m.textarea.SetWidth(chatWidth - 4)

		// Update split pane dimensions
		if m.splitPane != nil {
			m.list.SetSize(msg.Width, msg.Height)
			m.filepicker.Height = msg.Height - 15
			m.splitPane.SetSize(msg.Width, msg.Height-headerHeight-footerHeight)
			m.usagePage.SetSize(msg.Width, msg.Height-headerHeight)
			m.campaignPage.SetSize(msg.Width, msg.Height-headerHeight)
			m.autoPage.SetSize(msg.Width, msg.Height-headerHeight)

		}
		if m.logicPane != nil {
			m.logicPane.SetSize(msg.Width/3, msg.Height-headerHeight-footerHeight-inputHeight-paddingHeight)
		}

		// Update renderer word wrap
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(chatWidth-4),
			)
			// Re-render history with new wrapping
			m.viewport.SetContent(m.renderHistory())
		}

	case tea.WindowSizeMsg:
		// Convert to our alias and re-process
		return m.Update(windowSizeMsg(msg))

	case clarificationReply:
		// Handle clarification reply
		return m, m.processClarificationResponse(string(msg), m.clarificationState.PendingIntent, m.clarificationState.Context)

	case spinner.TickMsg:
		if m.isLoading || m.isBooting {
			m.spinner, spCmd = m.spinner.Update(msg)
			return m, spCmd
		}

	case traceUpdateMsg:
		m.isLoading = false

		if m.logicPane != nil {
			m.logicPane.SetTraceMangle(msg.Trace)
		}

		// If ShowInChat is true (from /why command), show explanation in chat
		if msg.ShowInChat {
			explainer := transparency.NewExplainer()
			explanation := explainer.ExplainTrace(msg.Trace)

			m = m.pushAssistantMsg(explanation)
		}
		return m, nil

	case assistantMsg:
		m.isLoading = false
		m.turnCount++

		// Apply any state updates carried by the message
		if msg.ClarifyUpdate != nil {
			m.lastClarifyInput = msg.ClarifyUpdate.LastClarifyInput
			m.launchClarifyPending = msg.ClarifyUpdate.LaunchClarifyPending
			m.launchClarifyGoal = msg.ClarifyUpdate.LaunchClarifyGoal
			m.launchClarifyAnswers = msg.ClarifyUpdate.LaunchClarifyAnswers
		}
		if msg.DreamHypothetical != "" {
			m.lastDreamHypothetical = msg.DreamHypothetical
		}
		if msg.ShardResult != nil {
			m.storeShardResult(msg.ShardResult.ShardType, msg.ShardResult.Task, msg.ShardResult.Result, msg.ShardResult.Facts)
		}

		m = m.pushAssistantMsg(msg.Surface)
		m.saveSessionState()

	case responseMsg:
		m.isLoading = false
		m.turnCount++
		m = m.pushAssistantMsg(string(msg))
		// Persist session after each response
		m.saveSessionState()

	case alignmentCheckMsg:
		m.isLoading = false
		var content string
		if msg.Err != nil {
			content = fmt.Sprintf("## Alignment Check Failed\n\n**Error:** %v", msg.Err)
		} else {
			content = m.formatAlignmentCheckResult(msg)
		}
		m = m.pushAssistantMsg(content)

	case multiShardReviewMsg:
		// Multi-shard review completed
		m.isLoading = false
		m.turnCount++
		if msg.err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Multi-shard review failed: %v", msg.err),
				Time:    time.Now(),
			})
		} else if msg.review != nil {
			// Format and display the aggregated review
			content := formatMultiShardResponse(msg.review)
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: content,
				Time:    time.Now(),
			})
			m.storeAggregatedReviewResult(msg.review, content)
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()

	case clarificationMsg:
		// Enter clarification mode (Pause)
		m.isLoading = false
		m.awaitingClarification = true
		m.clarificationState = &ClarificationState{
			Question:      msg.Question,
			Options:       msg.Options,
			DefaultOption: msg.DefaultOption,
			Context:       msg.Context,
			PendingIntent: msg.PendingIntent,
		}
		m.selectedOption = 0
		if msg.Context != "" {
			m.lastClarifyInput = msg.Context
		}

		// Update UI to show clarification request
		m.textarea.Placeholder = "Select option or type your answer..."
		if len(msg.Options) > 0 {
			m.textarea.Placeholder = "Use ↑/↓ to select, Enter to confirm, or type custom answer..."
		}

		// Add clarification question to history
		m = m.pushAssistantMsg(m.formatClarificationRequest(ClarificationState(msg)))

	case errorMsg:
		m.isLoading = false
		// Check if this is a clarification request disguised as an error
		if strings.Contains(msg.Error(), "USER_INPUT_REQUIRED") || strings.Contains(msg.Error(), "clarification") {
			// Extract the question from the error message
			question := extractClarificationQuestion(msg.Error())
			return m, func() tea.Msg {
				return clarificationMsg{
					Question: question,
					Options:  []string{},
				}
			}
		}
		m.err = msg
		m.showError = true
		m.focusError = false
		m.refreshErrorViewport()
		m.errorVP.GotoTop()
		logging.Get(logging.CategorySession).Error("TUI error: %v", msg)
		// Trigger resize so the error panel reserves space immediately.
		return m, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} }

	case campaignErrorMsg:
		m.isLoading = false
		m = m.pushAssistantMsg(fmt.Sprintf("## Campaign Error\n\n%v", msg.err))

	case northstarDocsAnalyzedMsg:
		m.isLoading = false
		if m.northstarWizard != nil {
			if msg.err != nil {
				m = m.pushAssistantMsg(fmt.Sprintf("⚠️ Document analysis encountered an error: %v\n\nContinuing without extracted insights.", msg.err))
			} else if len(msg.facts) > 0 {
				m.northstarWizard.ExtractedFacts = msg.facts
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("## Research Analysis Complete\n\nExtracted **%d key insights** from your documents:\n\n", len(msg.facts)))
				for i, fact := range msg.facts {
					if i < 5 { // Show first 5
						sb.WriteString(fmt.Sprintf("- %s\n", fact))
					}
				}
				if len(msg.facts) > 5 {
					sb.WriteString(fmt.Sprintf("\n_...and %d more insights that will inform the process._\n", len(msg.facts)-5))
				}
				sb.WriteString("\n---\n\n## Phase 2: Problem Statement\n\n**What problem does this project solve?**\n\n_Your research insights will help refine this._")
				m = m.pushAssistantMsg(sb.String())
			}
			m.northstarWizard.Phase = NorthstarProblemStatement
			m.textarea.Placeholder = "Describe the problem..."
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	case requirementsGeneratedMsg:
		m.isLoading = false
		if m.northstarWizard != nil {
			if msg.err != nil {
				m = m.pushAssistantMsg(fmt.Sprintf("⚠️ Requirement generation encountered an error: %v\n\nYou can add requirements manually.", msg.err))
			} else if len(msg.requirements) > 0 {
				// Append generated requirements to wizard state
				m.northstarWizard.Requirements = append(m.northstarWizard.Requirements, msg.requirements...)
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("## Requirements Generated\n\nAdded **%d requirements** from your vision and capabilities:\n\n", len(msg.requirements)))
				for i, req := range msg.requirements {
					if i < 5 { // Show first 5
						sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", req.ID, req.Priority, req.Description))
					}
				}
				if len(msg.requirements) > 5 {
					sb.WriteString(fmt.Sprintf("\n_...and %d more requirements._\n", len(msg.requirements)-5))
				}
				sb.WriteString("\n_Add more requirements manually or type \"done\" to continue._")
				m = m.pushAssistantMsg(sb.String())
			} else {
				m = m.pushAssistantMsg("No requirements could be auto-generated. Please add requirements manually.")
			}
			m.textarea.Placeholder = "Add requirement or 'done'..."
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	// Campaign message handlers
	case campaignStartedMsg:
		m.isLoading = false
		m.activeCampaign = msg.campaign
		m.campaignOrch = msg.orch
		m.campaignProgressChan = msg.progressChan // Store channels for listening
		m.campaignEventChan = msg.eventChan
		m.showCampaignPanel = true
		m = m.pushAssistantMsg(m.renderCampaignStarted(msg.campaign))

		// Start orchestrator execution in background and return both listeners
		if m.campaignOrch != nil {
			return m, tea.Batch(
				m.runCampaignOrchestrator(),
				m.listenCampaignEvents(), // Listen for events in parallel
			)
		}

	case campaignProgressMsg:
		m.campaignProgress = msg
		// Update campaign panel without adding to history (live update)
		if m.activeCampaign != nil {
			m.activeCampaign.CompletedPhases = msg.CompletedPhases
			m.activeCampaign.CompletedTasks = msg.CompletedTasks
		}

		// Update Campaign Page
		if m.activeCampaign != nil {
			prog := campaign.Progress(*msg)
			m.campaignPage.UpdateContent(&prog, m.activeCampaign)
		}

		// Continue listening for progress updates via channel (not polling)
		if m.campaignProgressChan != nil && m.activeCampaign != nil {
			return m, m.listenCampaignProgress()
		}

	case campaignEventMsg:
		// Handle real-time events from the orchestrator
		// Events are informational - we can log them or show in UI
		// Continue listening for more events
		if m.campaignEventChan != nil && m.activeCampaign != nil {
			return m, m.listenCampaignEvents()
		}

	case campaignCompletedMsg:
		m.isLoading = false
		m.activeCampaign = nil
		m.campaignOrch = nil
		m.campaignProgress = nil
		m.campaignProgressChan = nil // Clear channels to stop listeners
		m.campaignEventChan = nil
		m.showCampaignPanel = false
		m = m.pushAssistantMsg(m.renderCampaignCompleted(msg))

	// =========================================================================
	// CONTINUATION PROTOCOL MESSAGE HANDLERS
	// =========================================================================

	case continuationInitMsg:
		// First step already completed; render its surface and initialize counters.
		if msg.firstResult != nil {
			m.storeShardResult(msg.firstResult.ShardType, msg.firstResult.Task, msg.firstResult.Result, msg.firstResult.Facts)
		}

		if msg.totalSteps > 0 {
			m.continuationTotal = msg.totalSteps
		} else if m.continuationTotal == 0 {
			m.continuationTotal = 2
		}
		m.continuationStep = 1

		m = m.pushAssistantMsg(msg.completedSurface)

		// Decide whether to pause before next step.
		shouldPause := false
		switch m.continuationMode {
		case ContinuationModeConfirm:
			shouldPause = true
		case ContinuationModeBreakpoint:
			shouldPause = msg.next.isMutation
		}

		if shouldPause {
			m.isLoading = false
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("?? Next: %s\n\nPress Enter to continue, or type new instructions.", msg.next.description),
				Time:    time.Now(),
			})
			m.pendingSubtasks = append(m.pendingSubtasks, Subtask{
				ID:          msg.next.subtaskID,
				Description: msg.next.description,
				ShardType:   msg.next.shardType,
				IsMutation:  msg.next.isMutation,
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		// Auto-continue immediately.
		m.isLoading = true
		m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, msg.next.description)
		return m, tea.Batch(
			m.spinner.Tick,
			m.executeSubtask(msg.next.subtaskID, msg.next.description, msg.next.shardType),
		)

	case continueMsg:
		// Store the just-completed subtask result for follow-ups.
		if msg.completedShardResult != nil {
			m.storeShardResult(msg.completedShardResult.ShardType, msg.completedShardResult.Task, msg.completedShardResult.Result, msg.completedShardResult.Facts)
		}
		if msg.totalSteps > 0 && msg.totalSteps > m.continuationTotal {
			m.continuationTotal = msg.totalSteps
		}

		m.continuationStep++

		// Show progress for completed step
		m = m.pushAssistantMsg(fmt.Sprintf("✓ [%d/%d] %s", m.continuationStep-1, m.continuationTotal, m.statusMessage))

		// Check continuation mode to decide whether to pause
		shouldPause := false
		switch m.continuationMode {
		case ContinuationModeConfirm:
			shouldPause = true // Always pause in Confirm mode
		case ContinuationModeBreakpoint:
			shouldPause = msg.isMutation // Pause only for mutations
		}

		if shouldPause {
			m.isLoading = false
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("⏸️ Next: %s\n\nPress Enter to continue, or type new instructions.", msg.description),
				Time:    time.Now(),
			})
			m.pendingSubtasks = append(m.pendingSubtasks, Subtask{
				ID:          msg.subtaskID,
				Description: msg.description,
				ShardType:   msg.shardType,
				IsMutation:  msg.isMutation,
			})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		// Auto mode: continue immediately
		m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, msg.description)
		return m, tea.Batch(
			m.spinner.Tick,
			m.executeSubtask(msg.subtaskID, msg.description, msg.shardType),
		)

	case confirmContinueMsg:
		// Resume from paused state (Enter pressed in Confirm/Breakpoint mode)
		if len(m.pendingSubtasks) > 0 {
			next := m.pendingSubtasks[0]
			m.pendingSubtasks = m.pendingSubtasks[1:]
			m.isLoading = true
			m.statusMessage = fmt.Sprintf("[%d/%d] %s", m.continuationStep, m.continuationTotal, next.Description)
			return m, tea.Batch(
				m.spinner.Tick,
				m.executeSubtask(next.ID, next.Description, next.ShardType),
			)
		}
		return m, nil

	case continuationDoneMsg:
		if msg.completedShardResult != nil {
			m.storeShardResult(msg.completedShardResult.ShardType, msg.completedShardResult.Task, msg.completedShardResult.Result, msg.completedShardResult.Facts)
		}

		m.isLoading = false
		m.continuationStep = 0
		m.continuationTotal = 0
		m.pendingSubtasks = nil
		m.isInterrupted = false
		// Clear continuation facts from kernel
		if m.kernel != nil {
			_ = m.kernel.Retract("interrupt_requested")
		}
		m = m.pushAssistantMsg(fmt.Sprintf("✅ All %d steps complete.\n\n%s", msg.stepCount, msg.summary))

	case initCompleteMsg:
		m.isLoading = false
		// Set up learning store and connect to shard manager via adapter
		if msg.learningStore != nil {
			m.learningStore = msg.learningStore
			if m.embeddingEngine != nil {
				m.learningStore.SetEmbeddingEngine(m.embeddingEngine)
			}
			if m.Config != nil {
				m.learningStore.SetReflectionConfig(m.Config.GetReflectionConfig())
			}
			adapter := &learningStoreAdapter{store: msg.learningStore}
			m.shardMgr.SetLearningStore(adapter)
		}
		// Build summary message from result
		m = m.pushAssistantMsg(m.renderInitComplete(msg.result))
		// Persist session after init
		m.saveSessionState()

	case scanCompleteMsg:
		startupScan := m.isBooting && m.bootStage == BootStageScanning
		m.isLoading = false
		if msg.err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Scan failed:** %v", msg.err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role: "assistant",
				Content: fmt.Sprintf(`**Scan complete**

| Metric | Value |
|--------|-------|
| Files indexed | %d |
| Directories | %d |
| Facts generated | %d |
| Duration | %.2fs |

The kernel has been updated with fresh codebase facts.`, msg.fileCount, msg.directoryCount, msg.factCount, msg.duration.Seconds()),
				Time: time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()

		// If this was the startup scan, unlock chat input now that we're green.
		if startupScan {
			m.isBooting = false
			m.textarea.Placeholder = "Ask me anything... (Enter to send, Shift+Enter for newline, Ctrl+C to exit)"
			m.textarea.Focus()

			// Fire first-run onboarding check AND LLM health check in parallel.
			// The health check is a non-blocking smoke test that also generates
			// a project-aware welcome message. If it fails, the user sees a
			// warning but can still use slash commands.
			return m, tea.Batch(
				checkFirstRun(m.workspace),
				m.performWelcomeHealthCheck(msg),
			)
		}

	case docRefreshCompleteMsg:
		m.isLoading = false
		if msg.err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Document refresh failed:** %v", msg.err),
				Time:    time.Now(),
			})
		} else {
			m = m.addMessage(Message{
				Role: "assistant",
				Content: fmt.Sprintf(`**Document refresh complete**

| Metric | Value |
|--------|-------|
| Docs discovered | %d |
| Docs processed | %d |
| Atoms stored | %d |
| Duration | %.2fs |

The strategic knowledge base has been updated with new documentation.`, msg.docsDiscovered, msg.docsProcessed, msg.atomsStored, msg.duration.Seconds()),
				Time: time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()

	case reembedCompleteMsg:
		m.isLoading = false
		if msg.err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**Re-embedding failed:** %v", msg.err),
				Time:    time.Now(),
			})
		} else {
			var sb strings.Builder
			sb.WriteString("**Re-embedding complete**\n\n")
			sb.WriteString(fmt.Sprintf("| Metric | Value |\n|--------|-------|\n| DBs processed | %d |\n| Vectors re-embedded | %d |\n| Prompt atoms re-embedded | %d |\n| Traces re-embedded | %d |\n| Learnings re-embedded | %d |\n| Duration | %.2fs |\n",
				msg.dbCount, msg.vectorsDone, msg.atomsDone, msg.tracesDone, msg.learningsDone, msg.duration.Seconds()))
			if len(msg.skipped) > 0 {
				sb.WriteString("\nSkipped/errored DBs:\n")
				for _, s := range msg.skipped {
					sb.WriteString(fmt.Sprintf("- %s\n", s))
				}
			}
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: sb.String(),
				Time:    time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.saveSessionState()

	case statusMsg:
		m.statusMessage = string(msg)
		return m, m.waitForStatus() // Listen for next update

	case glassBoxEventMsg:
		// Handle Glass Box event - add to history and re-render
		m.handleGlassBoxEvent(transparency.GlassBoxEvent(msg))
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, m.listenGlassBoxEvents() // Listen for next event

	case evolutionResultMsg:
		// Handle evolution cycle completion
		m.isLoading = false
		var content string
		if msg.err != nil {
			content = fmt.Sprintf("Evolution cycle failed: %v", msg.err)
		} else {
			content = formatEvolutionResult(msg.result)
		}
		m = m.pushAssistantMsg(content)
		return m, nil

	case toolEventMsg:
		// Handle tool event - ALWAYS add to history (not gated by Glass Box)
		m.handleToolEvent(transparency.ToolEvent(msg))
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, m.listenToolEvents() // Listen for next event

	case memUsageMsg:
		m.memAllocBytes = msg.Alloc
		m.memSysBytes = msg.Sys
		return m, m.tickMemory()

	case bootCompleteMsg:
		if msg.err != nil {
			// Boot failed - unlock input so user can fix config.
			m.isBooting = false
			m.bootStage = BootStageBooting
			m.err = msg.err
			m.showError = true
			m.focusError = false
			m.refreshErrorViewport()
			m.errorVP.GotoTop()
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**System Boot Failed:** %v", msg.err),
				Time:    time.Now(),
			})
		} else {
			// Boot succeeded - keep UI in boot screen while we scan/index workspace.
			m.isBooting = true
			m.bootStage = BootStageScanning
			// Populate components from the heavy initialization
			c := msg.components
			m.kernel = c.Kernel
			m.shardMgr = c.ShardMgr
			m.taskExecutor = c.TaskExecutor
			m.shadowMode = c.ShadowMode
			m.transducer = c.Transducer
			m.executor = c.Executor
			m.emitter = c.Emitter
			m.virtualStore = c.VirtualStore
			m.scanner = c.Scanner
			m.localDB = c.LocalDB
			m.embeddingEngine = c.EmbeddingEngine
			m.learningStore = c.LearningStore
			m.compressor = c.Compressor
			m.feedbackStore = c.FeedbackStore
			m.autopoiesis = c.Autopoiesis
			m.autopoiesisCancel = c.AutopoiesisCancel
			m.autopoiesisListenerCh = c.AutopoiesisListenerCh
			m.verifier = c.Verifier
			m.client = c.Client

			if m.learningStore != nil && m.shardMgr != nil {
				adapter := &learningStoreAdapter{store: m.learningStore}
				m.shardMgr.SetLearningStore(adapter)
			}
			if m.learningStore != nil {
				if m.embeddingEngine != nil {
					m.learningStore.SetEmbeddingEngine(m.embeddingEngine)
				}
				if m.Config != nil {
					m.learningStore.SetReflectionConfig(m.Config.GetReflectionConfig())
				}
			}

			// Wire browser manager for graceful shutdown
			m.browserMgr = c.BrowserManager
			m.browserCtxCancel = c.BrowserCtxCancel
			m.jitCompiler = c.JITCompiler

			// Wire Mangle file watcher for real-time .mg validation
			m.mangleWatcher = c.MangleWatcher

			// Initialize Glass Box debug mode event bus
			m.initGlassBox(c.GlassBoxEventBus)

			// Initialize Tool Event bus for always-visible tool execution
			m.initToolEventBus(c.ToolEventBus)

			// Wire Tool Store for tool execution persistence
			m.toolStore = c.ToolStore

			// Wire Prompt Evolution System (System Prompt Learning)
			m.promptEvolver = c.PromptEvolver

			// Wire Background Observer Manager (Northstar alignment guardian)
			m.observerMgr = c.ObserverMgr

			// Wire Consultation Manager (cross-specialist collaboration protocol)
			m.consultationMgr = c.ConsultationMgr

			// Initialize Dream State learning collector and router (§8.3.1)
			m.dreamCollector = core.NewDreamLearningCollector()
			m.dreamRouter = core.NewDreamRouter(m.kernel, nil, m.localDB)

			// Initialize Dream Plan Manager for dream-to-execute pipeline (§8.3.2)
			m.dreamPlanManager = core.NewDreamPlanManager(m.kernel)

			// Load previous session state if available (now that kernel is ready)
			loadedSession, _ := hydrateNerdState(m.workspace, m.kernel, m.shardMgr, &m.history)
			m.sessionID = resolveSessionID(loadedSession)
			m.turnCount = resolveTurnCount(loadedSession)

			// Rehydrate semantic compression state for this session (if persisted).
			m.hydrateCompressorForSession(m.sessionID)
		}

		// If boot failed, allow input immediately. If boot succeeded, wait until scan completes.
		if msg.err != nil {
			m.textarea.Placeholder = "System boot failed. Fix config then retry /scan or restart."
			m.textarea.Focus()
		} else {
			m.textarea.Placeholder = "Indexing workspace..."
		}

		// Append any initial messages generated during boot
		if msg.components != nil && len(msg.components.InitialMessages) > 0 {
			m = m.addMessages(msg.components.InitialMessages...)
		}

		// Now trigger the workspace scan (deferred). This keeps chat input hidden until ready.
		// Also start listening for tool events (always active, not gated by Glass Box).
		return m, tea.Batch(m.runScan(false), m.listenToolEvents())

	case onboardingCheckMsg:
		// Handle first-run detection result
		if msg.IsFirstRun {
			// Start onboarding wizard for new users
			return m.startOnboarding()
		}
		// Existing user - run migration silently
		_, _ = ux.MigratePreferences(msg.Workspace)
		return m, nil

	case onboardingCompleteMsg:
		// Onboarding finished - record in preferences
		if !msg.Skipped && msg.ExperienceLevel != "" {
			// Update guidance level based on experience
			if m.Config != nil && m.Config.Guidance != nil {
				switch msg.ExperienceLevel {
				case "beginner":
					m.Config.Guidance.Level = config.GuidanceVerbose
				case "intermediate":
					m.Config.Guidance.Level = config.GuidanceNormal
				case "advanced", "expert":
					m.Config.Guidance.Level = config.GuidanceMinimal
				}
			}
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case welcomeHealthCheckMsg:
		// LLM health check completed (or failed) after boot
		if msg.err != nil {
			// Health check failed — warn user but don't block
			providerInfo := msg.provider
			if msg.model != "" {
				providerInfo += "/" + msg.model
			}
			if providerInfo == "" {
				providerInfo = "unknown"
			}
			m = m.pushAssistantMsg(fmt.Sprintf(
				"**LLM health check failed** (%s): %v\n\nYou can still use slash commands. Check your API key and network connection.",
				providerInfo, msg.err,
			))
		} else if msg.welcome != "" {
			// Health check succeeded — show LLM-generated welcome
			m = m.pushAssistantMsg(msg.welcome)
		}
		// If welcome is empty but no error, silently skip (LLM returned empty)
		return m, nil

	case knowledgeGatheredMsg:
		// Knowledge gathering from specialists is complete.
		// Store results and re-process with enriched context.
		m.awaitingKnowledge = false

		// Store gathered knowledge for this turn and for history
		m.pendingKnowledge = msg.Results
		m.knowledgeHistory = append(m.knowledgeHistory, msg.Results...)

		// Persist knowledge to SQLite for future retrieval
		m.persistKnowledgeResults(msg.Results)

		// Log knowledge gathering summary
		var successCount, failCount int
		for _, kr := range msg.Results {
			if kr.Error != nil {
				failCount++
			} else {
				successCount++
			}
		}
		logging.Get(logging.CategoryContext).Info(
			"Knowledge gathering complete: %d succeeded, %d failed",
			successCount, failCount,
		)

		// Show interim response to the user if provided
		historyUpdated := false
		if msg.InterimResponse != "" {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: msg.InterimResponse,
				Time:    time.Now(),
			})
			historyUpdated = true
		}

		if historyUpdated {
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.saveSessionState()
		}

		// Re-process the original input with knowledge context
		// The knowledge is now available via m.pendingKnowledge which
		// will be injected into SessionContext by buildSessionContext()
		return m, m.processInputWithKnowledge(msg.OriginalInput)

	case antigravityOAuthResultMsg:
		// Handle OAuth result from Antigravity account addition
		if m.configWizard == nil || m.configWizard.Step != StepAntigravityWaiting {
			// Ignore if not in the right wizard state
			return m, nil
		}

		if msg.err != nil {
			m = m.addMessage(Message{
				Role:    "assistant",
				Content: fmt.Sprintf("**OAuth Failed:** %v\n\nPress Enter to try again or **q** to go back.", msg.err),
				Time:    time.Now(),
			})
			m.configWizard.Step = StepAntigravityAccounts
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}

		// Success! Show confirmation and ask if they want to add more
		m.configWizard.AntigravityAuthState = nil
		m.configWizard.Step = StepAntigravityAddMore

		// Refresh the account list
		store, _ := antigravity.NewAccountStore()
		if store != nil {
			m.configWizard.AntigravityAccounts = store.ListAccounts()
		}

		m = m.addMessage(Message{
			Role: "assistant",
			Content: fmt.Sprintf(`## Account Added Successfully!

**Email:** %s
**Project:** %s

You now have **%d account(s)** configured.

For better rate limit handling, consider adding more accounts.
Would you like to add another Google account? (y/n)`,
				msg.account.Email,
				msg.account.ProjectID,
				len(m.configWizard.AntigravityAccounts)),
			Time: time.Now(),
		})
		m.textarea.Placeholder = "Add another account? (y/n)..."
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}
