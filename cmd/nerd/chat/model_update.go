package chat

import (
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/approval"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/shards"
	"codenerd/internal/transparency"
	"codenerd/internal/ux"

	"codenerd/internal/sessionlog"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.updateCore(msg)
	// A Ctrl+X stop is announced as stopped once its execution has settled.
	if nm, ok := next.(Model); ok && nm.turn.stopSettled() {
		next = nm.pushAssistantMsg(nm.stoppedNotice())
	}
	// A message queued while a turn ran starts once nothing is in flight.
	if nm, ok := next.(Model); ok && !nm.isLoading && nm.turn.queuedCount() > 0 {
		var queued tea.Cmd
		nm, queued = nm.startQueuedTurn()
		return nm, tea.Batch(cmd, queued)
	}
	return next, cmd
}

func (m Model) updateCore(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		tiCmd      tea.Cmd
		vpCmd      tea.Cmd
		spCmd      tea.Cmd
		persistCmd tea.Cmd // async session-state save; see saveSessionStateCmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		var handled bool
		m, tiCmd, handled = m.handleKeyMsg(msg)
		if handled {
			return m, tiCmd
		}

	case windowSizeMsg:
		m = m.handleWindowSizeMsg(msg)
	case tea.WindowSizeMsg:
		// Convert to our alias and re-process
		return m.Update(windowSizeMsg(msg))

	case clarificationReply:
		// Handle clarification reply
		// The reply enters no history, so it is no request of its own.
		return m, m.processClarificationResponse(string(msg), m.clarificationState.PendingIntent, m.clarificationState.Context, 0)

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

	case streamStartMsg:
		m.isStreaming = true
		m.currentStream = ""
		m.currentThought = ""
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		// Watch both surface and thoughts in parallel. waitForThoughts
		// returns nil when thoughtsChan is nil (provider has no thought
		// stream) — tea.Batch drops nil cmds, so this is safe.
		return m, tea.Batch(
			waitForStream(msg.streamChan),
			waitForThoughts(msg.thoughtsChan),
			waitForResult(msg.resultChan, msg.errChan),
		)

	case streamChunkMsg:
		if m.isStreaming {
			switch msg.kind {
			case streamKindThought:
				m.currentThought += msg.chunk
			default:
				m.currentStream += msg.chunk
			}
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
		}
		// Continue watching the same channel for the next chunk.
		if msg.kind == streamKindThought {
			return m, waitForThoughts(msg.sub)
		}
		return m, waitForStream(msg.sub)

	case streamEndMsg:
		m.isStreaming = false
		m.currentThought = ""
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case cmdMsg:
		return m, msg.cmd

	case assistantMsg:
		m.isLoading = false
		m.isStreaming = false
		m.currentStream = ""
		m.currentThought = ""
		m.turnCount++
		m.recordUXMetric("successful_tasks")

		m.awaitingKnowledge = msg.AwaitingKnowledge
		if msg.Reflection != nil {
			m.lastReflection = msg.Reflection
		}
		if msg.DreamHypothetical != "" {
			m.lastDreamHypothetical = msg.DreamHypothetical
		}
		if msg.ShardResult != nil {
			m.storeShardResult(msg.ShardResult.ShardType, msg.ShardResult.Task, msg.ShardResult.Result, msg.ShardResult.Facts)
		}

		m = m.pushAssistantMsgWithThought(msg.Surface, msg.ThoughtSummary)
		persistCmd = m.saveSessionStateCmd()

	case sideTurnMsg:
		m = m.applySideTurn(msg)
		return m, nil

	case backgroundStartMsg:
		// The turn handed its work to the background: answer now, run it.
		m.isLoading = false
		m.turnCount++
		if msg.Reflection != nil {
			m.lastReflection = msg.Reflection
		}
		m = m.pushAssistantMsg(msg.Surface)
		return m, tea.Batch(m.startBackground(msg.Job, msg.Job.Task), m.saveSessionStateCmd())

	case answerMsg:
		return m.applyAnswer(msg)

	case backgroundDoneMsg:
		var cmd tea.Cmd
		// The job's report answers the request that started it, in its own
		// session; a job that settles elsewhere writes its answer there.
		prev := m.answering
		if msg.Job.Session == "" || msg.Job.Session == m.sessionID {
			m.answering = msg.Job.RequestID
		}
		m, cmd = m.finishBackground(msg)
		m.answering = prev
		return m, tea.Batch(cmd, m.saveSessionStateCmd())

	case holdMsg:
		// Work held for the user's go-ahead: the conversation stays open.
		m.isLoading = false
		m.turnCount++
		m.pendingWork = msg.Work
		m = m.pushAssistantMsg(msg.Surface)
		persistCmd = m.saveSessionStateCmd()

	case approveWorkMsg:
		// The kernel read the reply as the go-ahead on the held proposal.
		m.isLoading = false
		var cmd tea.Cmd
		m, cmd = m.approvePendingWork()
		return m, cmd

	case responseMsg:
		m.isLoading = false
		m.turnCount++
		m.recordUXMetric("successful_tasks")
		m = m.pushAssistantMsg(string(msg))
		// Persist session after each response (off the UI event loop).
		persistCmd = m.saveSessionStateCmd()

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
		persistCmd = m.saveSessionStateCmd()

	case clarificationMsg:
		// Enter clarification mode (Pause)
		m.isLoading = false
		m.recordUXMetric("clarifications_needed")
		m.inputMode = InputModeClarification
		m.clarificationState = &ClarificationState{
			DeliberationID: msg.DeliberationID,
			Question:       msg.Question,
			Options:        msg.Options,
			DefaultOption:  msg.DefaultOption,
			Context:        msg.Context,
			PendingIntent:  msg.PendingIntent,
		}
		m.selectedOption = 0

		// Update UI to show clarification request
		m.textarea.Placeholder = "Select option or type your answer..."
		if len(msg.Options) > 0 {
			m.textarea.Placeholder = "Use ↑/↓ to select, Enter to confirm, or type custom answer..."
		}

		// Add clarification question to history
		m = m.pushAssistantMsg(m.formatClarificationRequest(ClarificationState(msg)))
		persistCmd = m.saveSessionStateCmd()

	case errorMsg:
		m.isLoading = false
		// A failed turn's streamed prefix is an incomplete attempt, not an
		// answer: it never reaches history, and it must not linger on screen
		// as if it were one.
		m.isStreaming = false
		m.currentStream = ""
		m.currentThought = ""
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
		// A broker refusal means the turn never reached a provider. Presenting
		// it as an ordinary failure sends the reader looking for a network or
		// model problem that is not there.
		m.err = m.presentError(msg)
		m.showError = true
		m.focusError = false
		m.refreshErrorViewport()
		m.errorVP.GotoTop()
		logging.Get(logging.CategorySession).Error("TUI error: %v", msg)
		m.recordUXMetric("errors_encountered")
		logging.AuditWithSession(m.sessionID).Error("chat", msg, false)
		// Failure belongs to the conversation as well as the error panel.
		m = m.pushAssistantMsg(fmt.Sprintf("## Failed\n\n%v", m.err))
		// Refit so the error panel reserves space immediately, and persist
		// this terminal message without requiring another user interaction.
		m = m.applyLayout()
		return m, m.saveSessionStateCmd()

	case campaignErrorMsg:
		if msg.orch != nil && msg.orch != m.campaignOrch {
			// An earlier run's failure: it must not clear this run's state.
			logging.Kernel("[campaign] error from a run that is no longer current: %v", msg.err)
			return m, nil
		}
		m.isLoading = false
		// A risk gate refusal is a policy decision, not a failure, and it has a
		// full report behind it (which gate, what score, hard vs advisory).
		// Rendering it as a bare "Campaign Error: <sentence>" hid everything the
		// operator needs to decide what to do about it.
		if report, ok := campaign.FormatRiskBlock(msg.err); ok {
			m = m.pushAssistantMsg("## Campaign Blocked by Risk Gate\n\n```\n" + report + "```")
		} else {
			m = m.pushAssistantMsg(fmt.Sprintf("## Campaign Error\n\n%v", msg.err))
		}

	case northstarDocsAnalyzedMsg:
		m = m.handleNorthstarDocsAnalyzedMsg(msg)
	case requirementsGeneratedMsg:
		m = m.handleRequirementsGeneratedMsg(msg)
	case campaignStartedMsg:
		m.isLoading = false
		if m.campaignOrch != nil && msg.orch != m.campaignOrch {
			// One managed campaign: a second accepted run does not replace a
			// live one. It was planned and saved; it is not started here.
			if msg.orch != nil {
				_ = msg.orch.Close()
			}
			title := ""
			if msg.campaign != nil {
				title = msg.campaign.Title
			}
			m = m.pushAssistantMsg(fmt.Sprintf("## Campaign Not Started\n\nA campaign is already running (%s). %q was planned and saved but not started; stop or finish the running one first.", m.activeCampaign.Title, title))
			return m, nil
		}
		m.activeCampaign = msg.campaign
		m.campaignOrch = msg.orch
		m.campaignProgressChan = msg.progressChan // Store channels for listening
		m.campaignEventChan = msg.eventChan
		m.showCampaignPanel = true
		m = m.pushAssistantMsg(m.renderCampaignStarted(msg.campaign))
		m = m.applyLayout()

		// Start orchestrator execution in background and return both listeners
		if m.campaignOrch != nil {
			return m, tea.Batch(
				m.runCampaignOrchestrator(),
				listenCampaignEvents(m.campaignOrch, m.campaignEventChan), // Listen for events in parallel
			)
		}

	case campaignProgressMsg:
		if msg.orch != m.campaignOrch {
			// An earlier run's snapshot: it is not this run's progress, and
			// its listener is not re-armed on this run's slot.
			return m, nil
		}
		m.campaignProgress = msg.progress
		// Update campaign panel without adding to history (live update)
		if m.activeCampaign != nil {
			m.activeCampaign.CompletedPhases = msg.progress.CompletedPhases
			m.activeCampaign.CompletedTasks = msg.progress.CompletedTasks
		}

		// Update Campaign Page
		if m.activeCampaign != nil {
			prog := *msg.progress
			m.campaignPage.UpdateContent(&prog, m.activeCampaign)
		}

		// Continue listening for this run's progress (not polling)
		if m.campaignProgressChan != nil && m.activeCampaign != nil {
			return m, listenCampaignProgress(msg.orch, m.campaignProgressChan, m.activeCampaign)
		}

	case campaignEventMsg:
		// Risk-gate events are the campaign's only voice when preflight
		// refuses to start: no progress will ever publish, so swallowing them
		// strands the transcript on a run that already ended. Render the
		// report and, on a hard block, stand the campaign UI down the way
		// completion does. Anything else stays informational.
		if msg.orch != m.campaignOrch {
			return m, nil
		}
		if text, terminal := renderRiskGateEvent(msg.event); text != "" {
			m = m.pushAssistantMsg(text)
			if terminal {
				m.isLoading = false
				m.activeCampaign = nil
				m.campaignOrch = nil
				m.campaignProgress = nil
				m.campaignProgressChan = nil
				m.campaignEventChan = nil
				m.showCampaignPanel = false
				m = m.applyLayout()
			}
		}
		if m.campaignEventChan != nil && m.activeCampaign != nil {
			return m, listenCampaignEvents(msg.orch, m.campaignEventChan)
		}

	case campaignRequirementMsg:
		m = m.handleCampaignRequirementMsg(msg)

	case recurseStartedMsg, recurseLineMsg, recurseFinishedMsg:
		next, cmd, _ := m.handleRecurseMsg(msg)
		return next, cmd

	case campaignSettledMsg:
		m = m.handleCampaignSettled(msg)

	case campaignCompletedMsg:
		if msg.orch != m.campaignOrch {
			return m, nil
		}
		m = m.retireCampaign()
		m = m.pushAssistantMsg(m.renderCampaignCompleted(msg.campaign))

	// =========================================================================
	// CONTINUATION PROTOCOL MESSAGE HANDLERS
	// =========================================================================

	case continuationInitMsg:
		return m.handleContinuationInitMsg(msg)
	case continueMsg:
		return m.handleContinueMsg(msg)

	case confirmContinueMsg:
		// Enter on a pause runs the step the pause offered.
		return m.runOfferedSubtask()

	case continuationDoneMsg:
		m, tiCmd = m.handleContinuationDone(msg)
		// Save the settled outcome now, including failure and unverified work,
		// rather than waiting for the next user message.
		persistCmd = m.saveSessionStateCmd()

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
		// Persist session after init (off the UI event loop).
		persistCmd = m.saveSessionStateCmd()

	case scanCompleteMsg:
		var shouldReturn bool
		m, persistCmd, shouldReturn = m.handleScanCompleteMsg(msg)
		if shouldReturn {
			return m, persistCmd
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
| Docs extracted only in part | %d |
| Atoms stored | %d |
| Duration | %.2fs |

The strategic knowledge base has been updated with new documentation.%s`, msg.docsDiscovered, msg.docsProcessed, msg.docsPartial, msg.atomsStored, msg.duration.Seconds(), partialDocsNote(msg.docsPartial)),
				Time: time.Now(),
			})
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		persistCmd = m.saveSessionStateCmd()

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
		persistCmd = m.saveSessionStateCmd()

	case statusMsg:
		m.statusMessage = string(msg)
		// Keep the live pulse chrome moving even when Glass Box events lag —
		// status pings are the heartbeat of "something is happening".
		if strings.TrimSpace(string(msg)) != "" {
			m.activityLine = string(msg)
			m.activityIconCh = string(transparency.CategoryControl)
			m.activityAt = time.Now()
			m.pushActivityPulse(activityPulse{
				Summary:  string(msg),
				Category: transparency.CategoryControl,
				At:       m.activityAt,
			})
			m = m.applyLayout()
		}
		return m, m.waitForStatus() // Listen for next update

	case glassBoxEventMsg:
		// Full stream: fold the waking event plus any already-buffered
		// events into one frame so multi-shard bursts keep up with chat.
		m.drainGlassBoxEvents(transparency.GlassBoxEvent(msg))
		// The activity trail may have grown lines; refit so its height is
		// reserved instead of pushing the input off-frame.
		m = m.applyLayout()
		m.viewport.GotoBottom()
		return m, m.listenGlassBoxEvents() // Listen for next event

	case evolutionResultMsg:
		// Handle evolution cycle completion
		m.isLoading = false
		var content string
		if msg.err != nil {
			content = fmt.Sprintf("Evolution cycle failed: %v", msg.err)
		} else {
			if refreshErr := m.refreshEvolvedAtomsInCompiler(); refreshErr != nil {
				content = fmt.Sprintf("%s\n\nWarning: failed to refresh evolved atoms in JIT compiler: %v", formatEvolutionResult(msg.result), refreshErr)
			} else {
				content = formatEvolutionResult(msg.result)
			}
		}
		m = m.pushAssistantMsg(content)
		return m, nil

	case approvalUpdateMsg:
		return m.handleApprovalUpdate(approval.Request(msg))

	case ui.ApprovalDecisionMsg:
		return m.handleApprovalDecisions(msg), nil

	case delegatedProgressMsg:
		m.handleDelegatedProgress(sessionlog.LiveEvent(msg))
		m = m.applyLayout()
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, m.listenDelegatedProgress()

	case toolEventMsg:
		// Handle tool event - ALWAYS add to history (not gated by Glass Box)
		m.handleToolEvent(transparency.ToolEvent(msg))
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, m.listenToolEvents() // Listen for next event

	case observerAssessmentMsg:
		// Handle background observer assessment
		content := shards.FormatAssessment(shards.ObserverAssessment(msg))
		m = m.addMessage(Message{
			Role:    "system",
			Content: content,
			Time:    msg.Timestamp,
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, m.listenObserverAssessments() // Keep listening

	case memUsageMsg:
		m.memAllocBytes = msg.Alloc
		m.memSysBytes = msg.Sys
		// Services do not drive a task spinner. Refit their separate health
		// section on the existing idle-safe refresh tick.
		m = m.applyLayout()
		return m, m.tickMemory()

	case bootCompleteMsg:
		return m.handleBootCompleteMsg(msg)
	case rebuildWatchMsg:
		return m.handleRebuildWatch(msg)
	case binaryRebuiltMsg:
		return m.handleBinaryRebuilt(msg)
	case restartCheckMsg:
		return m.handleRestartCheck()
	case policyReloadMsg:
		return m.handlePolicyReload(msg)
	case bootDeferredMsg:
		return m.handleBootDeferredMsg(msg)
	case onboardingCheckMsg:
		// Handle first-run detection result
		if msg.IsFirstRun {
			// Start onboarding wizard for new users
			m = m.openSessionRecord(nil, nil)
			return m.startOnboarding()
		}
		// Existing user - run migration silently, then open the session on
		// the migrated preferences.
		m = m.openSessionRecord(ux.MigratePreferences(msg.Workspace))
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
			// Health check succeeded: the greeting opens the conversation,
			// however late it arrives (placeWelcome).
			m = m.placeWelcome(msg.welcome)
		}
		// If welcome is empty but no error, silently skip (LLM returned empty)
		return m, nil

	case knowledgeGatheredMsg:
		return m.handleKnowledgeGatheredMsg(msg)

	case sessionStatePersistedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("save session: %w", msg.err)
			m.showError = true
			m.focusError = false
			m.refreshErrorViewport()
			m.errorVP.GotoTop()
			m = m.applyLayout()
		}
		return m, nil
	}

	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd, persistCmd)
}

// handleBootCompleteMsg processes the bootCompleteMsg.
func (m Model) handleBootCompleteMsg(msg bootCompleteMsg) (tea.Model, tea.Cmd) {
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
		// Without this the boot-time retriever was unreachable and every issue
		// seed built a throwaway one with a cold keyword cache.
		m.retriever = c.Retriever
		m.localDB = c.LocalDB
		m.embeddingEngine = c.EmbeddingEngine
		// The process's Tier-4 searcher: the boot built it over the
		// workspace's persistent vector store and warms it in the background,
		// and the session executor's task passes share it, so the chat's
		// issue seed only ranks.
		m.semanticSearcher = c.SemanticSearcher
		m.learningStore = c.LearningStore
		m.compressor = c.Compressor
		m.feedbackStore = c.FeedbackStore
		m.cortex = c.Cortex
		m.autopoiesis = c.Autopoiesis
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

		// Become the human-approval host: approvable denials ask here.
		if err := m.attachApprovalHost(c.Approvals); err != nil {
			logging.Get(logging.CategoryBoot).Warn("Human approval host not attached (approvable actions stay refused): %v", err)
		}

		// Wire Prompt Evolution System (System Prompt Learning)
		m.promptEvolver = c.PromptEvolver

		// Wire Background Observer Manager (Northstar alignment guardian)
		m.observerMgr = c.ObserverMgr

		// Wire Consultation Manager (cross-specialist collaboration protocol)
		m.consultationMgr = c.ConsultationMgr

		// Initialize Dream State learning collector and reuse the
		// Dream subsystem singletons already constructed by the
		// system factory (internal/system/factory.go). Constructing
		// them a second time here used to spawn a parallel
		// DreamRouter/DreamPlanManager that bypassed the VirtualStore
		// wiring and caused duplicate "Creating DreamRouter" /
		// "Creating DreamPlanManager" boot log lines.
		m.dreamCollector = core.NewDreamLearningCollector()
		if m.virtualStore != nil {
			if dreamer := m.virtualStore.GetDreamer(); dreamer != nil {
				m.dreamRouter = dreamer.GetDreamRouter()
				m.dreamPlanManager = dreamer.GetDreamPlanManager()
			}
		}

		// Wire Dream → Ouroboros tool need bridge (§8.3.1).
		// The factory creates the router but does not have access to
		// the Ouroboros queue, so we connect it here on the singleton
		// instance.
		if c.DreamToolQ != nil && m.dreamRouter != nil {
			m.dreamRouter.SetOuroborosQueue(c.DreamToolQ)
		}

		// Boot already hydrated session state; trust the boot payload instead of
		// re-hydrating and minting a second session identity here.
		m.sessionID = c.SessionID
		m.turnCount = c.TurnCount
		if m.sessionID == "" {
			m.sessionID = resolveSessionID(nil)
		}
		m.selected.set(m.sessionID)
		if m.shardMgr != nil {
			m.shardMgr.SetSessionID(m.sessionID)
		}

		// The compressor snapshot load (SQLite read) runs off the Update loop
		// in runBootDeferred and lands as bootDeferredMsg; only the cheap
		// reset stays here so the boot handler stays under the 100ms target.
		m.prepareBootHydration(m.sessionID)

		// Mirror persisted yolo autonomy into the kernel: the fact is session
		// state and does not survive restarts, but the config does.
		m.syncWorkModeFacts()
	}

	// If boot failed, allow input immediately. If boot succeeded, wait until scan completes.
	if msg.err != nil {
		m.textarea.Placeholder = "System boot failed. Fix config then retry /scan or restart."
		m.textarea.Focus()
		return m, nil
	}

	m.textarea.Placeholder = "Indexing workspace..."

	// Append any initial messages generated during boot
	if msg.components != nil && len(msg.components.InitialMessages) > 0 {
		m = m.addMessages(msg.components.InitialMessages...)
	}

	// Wire Background Observer Manager callback now (cheap, channel + callback);
	// the blocking Start runs off the Update loop in runBootDeferred and its
	// result lands as bootDeferredMsg, keeping this handler under 100ms.
	if m.observerMgr != nil {
		m.observerAssessmentChan = make(chan shards.ObserverAssessment, 100)
		assessCh := m.observerAssessmentChan
		m.observerMgr.AddCallback(func(assessment shards.ObserverAssessment) {
			select {
			case assessCh <- assessment:
			default:
			}
		})
	}
	bootSessionID := m.sessionID

	// A restart onto a rebuilt binary resumes its session here, once the
	// boot (under the same session id) has wired the kernel; and every
	// session watches for the next rebuild.
	m = m.applyRestore()
	rebuildWatch := m.startRebuildWatch()
	var policyReloads tea.Cmd
	if m.cortex != nil && m.cortex.DevPolicyDir() != "" {
		m = m.pushAssistantMsg(fmt.Sprintf("Dev mode: policy is read from `%s`; edits are reloaded live once they compile.", m.cortex.DevPolicyDir()))
		policyReloads = listenPolicyReloads(m.cortex.PolicyReloads())
	}

	// Now trigger the workspace scan (deferred). This keeps chat input hidden until ready.
	// Also start listening for tool events, background observer assessments,
	// and Glass Box events (the latter is no-op if the bus is disabled).
	// runBootDeferred carries the slow wiring (compressor snapshot load,
	// observer start) off the Update loop; its result returns as bootDeferredMsg.
	return m, tea.Batch(m.runBootDeferred(bootSessionID), m.runScan(false), m.listenToolEvents(), m.listenObserverAssessments(), m.listenGlassBoxEvents(), m.listenApprovals(), rebuildWatch, policyReloads)

}

// prepareBootHydration does the cheap, synchronous part of compressor
// rehydration: reset any prior session state and bind the new session ID.
// The SQLite read stays in runBootDeferred off the Update loop.
func (m *Model) prepareBootHydration(sessionID string) {
	if m.compressor == nil {
		return
	}
	m.compressor.Reset()
	m.compressor.SetSessionID(sessionID)
}

// runBootDeferred carries the slow boot wiring off the Update loop. It loads
// the persisted compressor snapshot (SQLite read) and starts the background
// observer manager; the result returns as bootDeferredMsg for the Update
// thread to apply. Nothing here touches the viewport or input, so the TUI
// stays responsive while it runs.
func (m Model) runBootDeferred(sessionID string) tea.Cmd {
	localDB := m.localDB
	observerMgr := m.observerMgr
	return func() tea.Msg {
		out := bootDeferredMsg{sessionID: sessionID}
		if localDB != nil && m.compressor != nil {
			stateJSON, turnNumber, _, err := localDB.LoadLatestCompressedState(sessionID)
			if err == nil && len(stateJSON) > 0 {
				out.stateJSON = stateJSON
				out.turnNumber = turnNumber
				out.hasState = true
			}
		}
		if observerMgr != nil {
			out.observerErr = observerMgr.Start()
		}
		return out
	}
}

// handleBootDeferredMsg applies the slow boot wiring back on the Update
// thread. It preserves every behavior the synchronous boot once did: the
// compressor snapshot is validated and loaded with the same guards, the
// turn counter advances when the snapshot is ahead, and the observer start
// result is logged the same way.
func (m Model) handleBootDeferredMsg(msg bootDeferredMsg) (tea.Model, tea.Cmd) {
	// A session switch that landed before the deferred work returned leaves
	// the snapshot for another session: refuse it the way hydration does.
	if msg.sessionID != "" && msg.sessionID != m.sessionID {
		if msg.observerErr != nil {
			logging.Get(logging.CategoryShards).Error("Failed to start background observer manager: %v", msg.observerErr)
		} else if m.observerMgr != nil {
			logging.Get(logging.CategoryShards).Info("Background observer manager started successfully")
		}
		return m, nil
	}
	if msg.hasState {
		m.applyBootHydratedState(msg.stateJSON, msg.turnNumber, msg.sessionID)
	} else if m.compressor != nil {
		m.compressor.RefreshBudget()
	}
	if msg.observerErr != nil {
		logging.Get(logging.CategoryShards).Error("Failed to start background observer manager: %v", msg.observerErr)
	} else if m.observerMgr != nil {
		logging.Get(logging.CategoryShards).Info("Background observer manager started successfully")
	}
	return m, nil
}

// applyBootHydratedState loads a previously fetched compressor snapshot. It
// mirrors hydrateCompressorForSession minus the SQLite read, with identical
// guards: empty or unparseable state is ignored, a snapshot belonging to
// another session is refused, and the budget refresh keeps the status bar
// accurate.
func (m *Model) applyBootHydratedState(stateJSON string, turnNumber int, sessionID string) {
	if m.compressor == nil {
		return
	}
	if len(stateJSON) == 0 {
		m.compressor.RefreshBudget()
		return
	}
	state, err := ctxcompress.UnmarshalCompressedState([]byte(stateJSON))
	if err != nil {
		logging.Session("hydrateCompressorForSession: failed to parse compressed state: %v", err)
		return
	}
	if state.SessionID != "" && state.SessionID != sessionID {
		logging.Get(logging.CategorySession).Warn("hydrateCompressorForSession: the state stored for %s belongs to %s; not loaded", sessionID, state.SessionID)
		return
	}
	state.SessionID = sessionID
	if err := m.compressor.LoadState(state); err != nil {
		logging.Session("hydrateCompressorForSession: failed to load compressed state: %v", err)
		return
	}
	if turnNumber > m.turnCount {
		m.turnCount = turnNumber
	}
	m.compressor.RefreshBudget()
}

// handleKnowledgeGatheredMsg processes the knowledgeGatheredMsg.
func (m Model) handleKnowledgeGatheredMsg(msg knowledgeGatheredMsg) (tea.Model, tea.Cmd) {
	var persistCmd tea.Cmd
	// Knowledge gathering from specialists is complete. Synthesize the
	// final answer with ONE follow-up LLM call. (This used to re-enter the
	// entire processInput pipeline — perception again, the full DECIDE
	// waterfall again, articulation again — doubling turn cost and
	// re-rolling routing mid-turn. The synthesis call is bounded and
	// deterministic.)
	m.awaitingKnowledge = msg.AwaitingKnowledge

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
		persistCmd = m.saveSessionStateCmd()
	}

	return m, tea.Batch(persistCmd, m.synthesizeWithKnowledge(msg.OriginalInput, msg.Parent, msg.Results))

}

// handleScanCompleteMsg processes the scanCompleteMsg.
func (m Model) handleScanCompleteMsg(msg scanCompleteMsg) (Model, tea.Cmd, bool) {
	var persistCmd tea.Cmd
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
	persistCmd = m.saveSessionStateCmd()

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
			persistCmd,
			checkFirstRun(m.workspace),
			m.performWelcomeHealthCheck(msg),
		), true
	}

	return m, persistCmd, false
}

// handleContinuationInitMsg processes the continuationInitMsg.
func (m Model) handleContinuationInitMsg(msg continuationInitMsg) (tea.Model, tea.Cmd) {
	// First step already completed; render its surface and initialize counters.
	m.delivery = msg.delivery
	if msg.firstResult != nil {
		m.storeShardResult(msg.firstResult.ShardType, msg.firstResult.Task, msg.firstResult.Result, msg.firstResult.Facts)
	}

	if msg.totalSteps > 0 {
		m.continuationTotal = msg.totalSteps
	} else if m.continuationTotal == 0 {
		m.continuationTotal = 2
	}
	m.continuationStep = 1
	m.updateContinuationFacts()

	m = m.pushAssistantMsg(msg.completedSurface)
	// A new request's chain: it is not the approved plan's, so its end
	// resumes a paused plan by offering it rather than advancing it.
	m.planChain = false
	m.chainUnverified = false
	m.chainUnverifiedNote = ""
	return m.offerOrDispatch(msg.next.subtask())
}

// handleNorthstarDocsAnalyzedMsg processes the northstarDocsAnalyzedMsg.
func (m Model) handleNorthstarDocsAnalyzedMsg(msg northstarDocsAnalyzedMsg) Model {
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

	return m
}

// handleRequirementsGeneratedMsg processes the requirementsGeneratedMsg.
func (m Model) handleRequirementsGeneratedMsg(msg requirementsGeneratedMsg) Model {
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
	return m
}

// sessionStatePersistedMsg reports asynchronous persistence to the event loop.
// A failed save is presented without retrying recursively or replacing the
// delegated outcome already in the conversation.
type sessionStatePersistedMsg struct {
	err error
}

// saveSessionStateCmd persists session state off the Bubbletea event loop.
//
// Previously the UI Update() handler invoked m.saveSessionState() synchronously
// at the end of several message branches. saveSessionState calls
// compressor.GetState() (which executes context.GetHighActivationFacts — ~322ms
// against a busy kernel) and then localDB.StoreCompressedState (synchronous
// SQLite write). Together they blocked Update() for ~360ms — roughly 22 frames
// at 60fps — visibly stalling input and rendering.
//
// We snapshot the receiver and run the entire persistence step in a goroutine
// scheduled by Bubbletea. The Cmd reports any file persistence error back to
// Update without blocking rendering or recursively scheduling another save.
func (m Model) saveSessionStateCmd() tea.Cmd {
	if m.workspace == "" || m.sessionID == "" {
		return nil
	}
	// Snapshot history into a fresh slice to avoid racing with subsequent
	// m.addMessage() appends on the next Update() tick.
	historySnapshot := make([]Message, len(m.history))
	copy(historySnapshot, m.history)
	snapshot := m
	snapshot.history = historySnapshot
	return func() tea.Msg {
		return sessionStatePersistedMsg{err: snapshot.saveSessionState()}
	}
}

// updateContinuationFacts asserts/retracts continuation facts in the kernel.
func (m *Model) updateContinuationFacts() {
	if m.kernel == nil {
		return
	}
	_ = m.kernel.Retract("continuation_step")
	_ = m.kernel.Assert(core.Fact{
		Predicate: "continuation_step",
		Args:      []any{int64(m.continuationStep), int64(m.continuationTotal)},
	})
	// No step ceiling is asserted. Until 2026-09-18 max_continuation_steps(10)
	// was written here and the continuation ended on it, which is a count as a
	// completion criterion; the loop is now bounded by the obligations
	// themselves (each dispatched once, each incomplete step retried once --
	// policy/codedom_continuation.mg).
}
