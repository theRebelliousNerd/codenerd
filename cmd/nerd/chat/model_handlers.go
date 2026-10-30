package chat

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	criticAutoLearnReason   = "/critic_autolearn"
	criticManualLearnReason = "/critic_manual"
)

// =============================================================================
// RENDERING CACHE HELPERS
// =============================================================================

// addMessage appends a message to history and pre-renders it into cache.
// This pre-computation during Update() avoids expensive rendering during View().
func (m Model) addMessage(msg Message) Model {
	msg = m.attribute(msg)
	idx := len(m.history)
	m.history = append(m.history, msg)

	// Pre-render and cache the message (saves work during View())
	if m.renderedCache != nil {
		rendered := m.renderSingleMessage(msg)
		if rendered != "" {
			m.renderedCache[idx] = rendered
		}
	}

	// Mark all messages as cached
	m.cacheInvalidFrom = len(m.history)

	return m
}

// pushAssistantMsg is a convenience wrapper that adds an assistant message,
// re-renders the viewport, and scrolls to bottom in one call.
// Replaces the repetitive 3-line pattern used throughout Update handlers.
func (m Model) pushAssistantMsg(content string) Model {
	m = m.addMessage(Message{Role: "assistant", Content: content, Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m
}

// pushAssistantMsgWithThought is a convenience wrapper that adds an assistant message
// containing a brief summary, re-renders the viewport, and scrolls to bottom.
func (m Model) pushAssistantMsgWithThought(content string, thought string) Model {
	m = m.addMessage(Message{Role: "assistant", Content: content, ThoughtSummary: thought, Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m
}

// pushUserMsg is a convenience wrapper that adds a user message,
// re-renders the viewport, and scrolls to bottom in one call.
func (m Model) pushUserMsg(content string) Model {
	m = m.addMessage(Message{Role: "user", Content: content, Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m
}

// addMessages appends multiple messages and pre-renders them into cache.
func (m Model) addMessages(msgs ...Message) Model {
	if len(msgs) == 0 {
		return m
	}

	startIdx := len(m.history)
	for i := range msgs {
		msgs[i] = m.attribute(msgs[i])
	}
	m.history = append(m.history, msgs...)

	// Pre-render all new messages
	if m.renderedCache != nil {
		for i, msg := range msgs {
			idx := startIdx + i
			rendered := m.renderSingleMessage(msg)
			if rendered != "" {
				m.renderedCache[idx] = rendered
			}
		}
	}

	// Mark all messages as cached
	m.cacheInvalidFrom = len(m.history)

	return m
}

// =============================================================================
// INPUT HANDLERS
// =============================================================================

// handleSubmit processes user input submission
func (m Model) handleSubmit() (tea.Model, tea.Cmd) {
	raw := m.textarea.Value()
	// Patch mode carries file syntax: a leading space is a context marker
	// and an empty submission is a blank line, so nothing is trimmed.
	if m.inputMode == InputModePatch {
		return m.ingestPatchSubmission(raw)
	}
	input := strings.TrimSpace(raw)
	if input == "" {
		// Empty Enter on a pause confirms the step it offered.
		if m.offeredSubtask != nil {
			return m, func() tea.Msg { return confirmContinueMsg{} }
		}
		return m, nil
	}

	m.recordUXMetric("commands_executed")

	// Check for special commands
	if strings.HasPrefix(input, "/") {
		// The untrimmed tail reaches handleCommand: /write and /append
		// carry its bytes as file content.
		return m.handleCommand(strings.TrimLeft(raw, " \t\r\n"))
	}

	// If we are collecting clarification answers for a future campaign launch,
	// accumulate the user's replies here (Update thread) so state persists.
	if m.launchClarifyPending {
		if m.launchClarifyAnswers != "" {
			m.launchClarifyAnswers += "\n"
		}
		m.launchClarifyAnswers += input
	}

	// Add user message to history
	m = m.addMessage(Message{
		Role:    "user",
		Content: input,
		Time:    time.Now(),
	})
	// Append to input history
	if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != input {
		m.inputHistory = append(m.inputHistory, input)
	}
	m.historyIndex = len(m.inputHistory)

	// Clear input
	m.textarea.Reset()

	// Update viewport
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	// Start loading
	if m.inputMode == InputModeAgentWizard {
		return m.handleAgentWizardInput(input)
	}

	// Config wizard mode
	if m.inputMode == InputModeConfigWizard {
		return m.handleConfigWizardInput(input)
	}

	// Northstar wizard mode
	if m.inputMode == InputModeNorthstar {
		return m.handleNorthstarWizardInput(input)
	}

	// Onboarding wizard mode (first-run experience)
	if m.inputMode == InputModeOnboarding {
		return m.handleOnboardingInput(input)
	}

	m.isLoading = true
	m.beginLiveTurn("Thinking...")

	// Check for dream state learning confirmation (§8.3.1)
	if m.dreamCollector != nil && isDreamConfirmation(input) {
		return m.handleDreamLearningConfirmation(input)
	}

	// Check for dream state learning correction ("no, actually...", "wrong, we use...")
	if m.dreamCollector != nil && isDreamCorrection(input) {
		return m.handleDreamLearningCorrection(input)
	}

	// Check for dream plan execution trigger ("do it", "execute that", "run the plan")
	if m.dreamPlanManager != nil && m.dreamPlanManager.HasPendingPlan() && isDreamExecutionTrigger(input) {
		return m.handleDreamPlanExecution(input)
	}

	// Process in background
	return m, tea.Batch(
		m.spinner.Tick,
		m.processInput(input, m.lastRequest()),
	)
}

// =============================================================================
// CLARIFICATION HANDLERS
// =============================================================================

type learningCandidate struct {
	Phrase string
	Verb   string
	Target string
	Reason string
}

func (m Model) loadLearningConfirmationCandidate() (*learningCandidate, bool) {
	if m.kernel == nil {
		return nil, false
	}
	facts, err := m.kernel.Query("learning_confirmation_needed")
	if err != nil || len(facts) == 0 {
		return nil, false
	}
	for _, f := range facts {
		if len(f.Args) < 4 {
			continue
		}
		return &learningCandidate{
			Phrase: strings.TrimSpace(types.ExtractString(f.Args[0])),
			Verb:   strings.TrimSpace(types.ExtractString(f.Args[1])),
			Target: strings.TrimSpace(types.ExtractString(f.Args[2])),
			Reason: strings.TrimSpace(types.ExtractString(f.Args[3])),
		}, true
	}
	return nil, false
}

func (m Model) loadLearningCandidateFact(candidate *learningCandidate) string {
	if candidate == nil || m.kernel == nil {
		return ""
	}
	facts, err := m.kernel.Query("learning_candidate_fact")
	if err != nil || len(facts) == 0 {
		return ""
	}
	for _, f := range facts {
		if len(f.Args) < 5 {
			continue
		}
		phrase := strings.TrimSpace(types.ExtractString(f.Args[0]))
		verb := strings.TrimSpace(types.ExtractString(f.Args[1]))
		target := strings.TrimSpace(types.ExtractString(f.Args[2]))
		reason := strings.TrimSpace(types.ExtractString(f.Args[3]))
		if phrase == candidate.Phrase && verb == candidate.Verb && target == candidate.Target && reason == candidate.Reason {
			return strings.TrimSpace(types.ExtractString(f.Args[4]))
		}
	}
	return ""
}

func (m Model) clearLearningCandidateFacts() {
	if m.kernel == nil {
		return
	}
	_ = m.kernel.Retract("learning_candidate")
	_ = m.kernel.Retract("learning_candidate_fact")
	_ = m.kernel.Retract("learning_candidate_count")
	_ = m.kernel.Retract("learning_confirmation_needed")
	_ = m.kernel.Retract("awaiting_clarification")
	_ = m.kernel.Retract("interrogative_mode")
	_ = m.kernel.Retract("clarification_question")
	_ = m.kernel.Retract("clarification_option")
	_ = m.kernel.Retract("no_action_reason")
	_ = m.kernel.Retract("intent_unknown")
	_ = m.kernel.Retract("intent_unmapped")
	_ = m.kernel.Retract("focus_clarification")
}

func (m Model) stageLearningCandidateFromFact(fact, reason string) (clarificationMsg, error) {
	if m.kernel == nil {
		return clarificationMsg{}, fmt.Errorf("kernel not initialized")
	}
	if strings.TrimSpace(fact) == "" {
		return clarificationMsg{}, fmt.Errorf("empty learned fact")
	}
	phrase, verb, target, qualifier, confidence, err := perception.ParseLearnedFact(fact)
	if err != nil {
		return clarificationMsg{}, fmt.Errorf("failed to parse learned fact: %w", err)
	}
	// The proposal is staged whole, normalized as it will be persisted:
	// confirming it, inline or by id, persists this text and nothing
	// reconstructed (audit manual-learning-evidence-conservation).
	normalized, err := perception.NormalizeLearnedFact(fact)
	if err != nil {
		return clarificationMsg{}, fmt.Errorf("failed to normalize learned fact: %w", err)
	}
	phrase = strings.TrimSpace(phrase)
	target = strings.TrimSpace(target)
	verb = normalizeVerbAtom(verb)
	if phrase == "" || verb == "" {
		return clarificationMsg{}, fmt.Errorf("invalid learned fact components")
	}
	reason = normalizeVerbAtom(reason)
	if reason == "" {
		return clarificationMsg{}, fmt.Errorf("invalid learning reason")
	}

	m.clearLearningCandidateFacts()

	var count int
	if m.localDB != nil {
		if recorded, err := m.localDB.RecordLearningCandidate(phrase, verb, target, reason, normalized); err != nil {
			return clarificationMsg{}, err
		} else {
			count = recorded
		}
	}

	facts := []core.Fact{
		{
			Predicate: "learning_candidate",
			Args: []any{
				phrase,
				core.MangleAtom(verb),
				target,
				core.MangleAtom(reason),
			},
		},
		{
			Predicate: "learning_candidate_fact",
			Args: []any{
				phrase,
				core.MangleAtom(verb),
				target,
				core.MangleAtom(reason),
				normalized,
			},
		},
	}
	if count > 0 {
		facts = append(facts, core.Fact{
			Predicate: "learning_candidate_count",
			Args:      []any{phrase, count},
		})
	}
	if err := m.kernel.AssertBatch(facts); err != nil {
		return clarificationMsg{}, fmt.Errorf("failed to stage learning candidate: %w", err)
	}

	question := fmt.Sprintf("I can learn this mapping: %s -> %s%s. Should I learn it?", phrase, verb, proposalDetail(target, qualifier, confidence))
	options := []string{
		"Yes, learn this mapping (/learn_yes)",
		"No, do not learn this (/learn_no)",
	}
	return clarificationMsg{
		Question: question,
		Options:  options,
	}, nil
}

// proposalDetail names what a proposal carries beyond phrase and verb, so
// the user confirms the fact that will be learned, not part of it.
func proposalDetail(target, qualifier string, confidence float64) string {
	var parts []string
	if target != "" {
		parts = append(parts, "target "+target)
	}
	if qualifier != "" {
		parts = append(parts, fmt.Sprintf("qualifier %q", qualifier))
	}
	parts = append(parts, fmt.Sprintf("confidence %.2f", confidence))
	return " (" + strings.Join(parts, ", ") + ")"
}

// learningContext is the context of a foreground "learn this" operation: the
// session's, so quitting cancels a pending embedding or write.
func (m Model) learningContext() context.Context {
	if m.shutdownCtx != nil {
		return m.shutdownCtx
	}
	return context.Background()
}

// partialLearningNote says, in one sentence, which projections of a confirmed
// correction did not commit. Empty when everything that has a backend did.
func partialLearningNote(outcome perception.PersistOutcome) string {
	if outcome.Complete() {
		return ""
	}
	return fmt.Sprintf(" Only part of it was stored (%s): %v", outcome, outcome.Err())
}

// confirmLearningCandidate persists a confirmed mapping and returns what was
// actually committed. The error is non-nil only when nothing durable was
// learned; a failed projection is in the outcome.
func (m Model) confirmLearningCandidate(candidate *learningCandidate) (perception.PersistOutcome, error) {
	if candidate == nil {
		return perception.PersistOutcome{}, fmt.Errorf("no learning candidate available")
	}
	if strings.TrimSpace(candidate.Phrase) == "" {
		return perception.PersistOutcome{}, fmt.Errorf("invalid learning candidate phrase")
	}
	// The staged proposal is what is learned. There used to be a fallback
	// that rebuilt the fact with an empty qualifier and confidence 0.90 when
	// the staged one was gone; it learned something nobody proposed.
	fact := m.loadLearningCandidateFact(candidate)
	if fact == "" {
		return perception.PersistOutcome{}, fmt.Errorf("the proposed fact for %q is no longer staged; run /learn again", candidate.Phrase)
	}
	if m.localDB != nil {
		if err := m.localDB.ConfirmLearningCandidateMatch(candidate.Phrase, candidate.Verb, candidate.Target, candidate.Reason, fact); err != nil && err != sql.ErrNoRows {
			return perception.PersistOutcome{}, err
		}
	}
	if perception.SharedTaxonomy == nil {
		return perception.PersistOutcome{}, fmt.Errorf("taxonomy engine not available")
	}
	return perception.SharedTaxonomy.PersistLearnedFact(m.learningContext(), fact)
}

func (m Model) rejectLearningCandidate(candidate *learningCandidate) error {
	if candidate == nil {
		return nil
	}
	if m.localDB == nil {
		return nil
	}
	if err := m.localDB.RejectLearningCandidateMatch(candidate.Phrase, candidate.Verb, candidate.Target, candidate.Reason, m.loadLearningCandidateFact(candidate)); err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

// handleClarificationResponse processes the user's response to a clarification request
func (m Model) handleClarificationResponse() (tea.Model, tea.Cmd) {
	var response string

	// Check if user selected an option or typed custom response
	if m.clarificationState != nil && len(m.clarificationState.Options) > 0 {
		inputText := strings.TrimSpace(m.textarea.Value())
		if inputText == "" {
			// Use selected option
			response = m.clarificationState.Options[m.selectedOption]
		} else {
			// Use custom input
			response = inputText
		}
	} else {
		response = strings.TrimSpace(m.textarea.Value())
		if response == "" {
			return m, nil
		}
	}

	// Add user response to history
	m = m.addMessage(Message{
		Role:    "user",
		Content: response,
		Time:    time.Now(),
	})
	// Append to input history
	if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != response {
		m.inputHistory = append(m.inputHistory, response)
	}
	m.historyIndex = len(m.inputHistory)

	// Clear clarification state (Resume)
	clarifyContext := ""
	if m.clarificationState != nil {
		clarifyContext = m.clarificationState.Context
	}
	pendingIntent := m.clarificationState.PendingIntent
	deliberationID := m.clarificationState.DeliberationID
	// Resume clarification protocol if applicable
	m.setInputMode(InputModeNormal)
	if clarifyContext != "" {
		m.lastClarifyInput = clarifyContext
	}

	// Reset input
	m.textarea.Reset()
	m.textarea.Placeholder = "Ask me anything... (Enter to send, Shift+Enter for newline, Ctrl+C to exit)"

	// Update viewport
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	// Start loading
	m.isLoading = true
	m.beginLiveTurn("Resuming after clarification...")

	// A values answer resumes the persisted thread with its original intent.
	if deliberationID != "" {
		return m, tea.Batch(m.spinner.Tick, m.resumeDeliberation(response, deliberationID, m.lastRequest()))
	}
	// Resume processing with clarification response
	return m, tea.Batch(
		m.spinner.Tick,
		m.processClarificationResponse(response, pendingIntent, clarifyContext, m.lastRequest()),
	)
}

// processClarificationResponse continues processing after user provides clarification
func (m Model) processClarificationResponse(response string, pendingIntent *perception.Intent, context string, request uint64) tea.Cmd {
	return func() tea.Msg {
		// Guard: kernel must be initialized
		if m.kernel == nil {
			return errorMsg(fmt.Errorf("system not ready: kernel not initialized"))
		}

		if candidate, ok := m.loadLearningConfirmationCandidate(); ok {
			confirmed, _ := learningConfirmationAnswer(response)
			if confirmed {
				outcome, err := m.confirmLearningCandidate(candidate)
				if err != nil {
					return errorMsg(fmt.Errorf("failed to confirm learning candidate: %w", err))
				}
				m.clearLearningCandidateFacts()
				if strings.TrimSpace(context) != "" {
					return m.processInput(context, request)()
				}
				return assistantMsg{
					Surface: fmt.Sprintf("Learned mapping: \"%s\" -> %s. You can retry the request when ready.%s", candidate.Phrase, candidate.Verb, partialLearningNote(outcome)),
				}
			}
			if err := m.rejectLearningCandidate(candidate); err != nil {
				return errorMsg(fmt.Errorf("failed to reject learning candidate: %w", err))
			}
			m.clearLearningCandidateFacts()
			return assistantMsg{
				Surface: "Okay, I won't learn that mapping. Please rephrase your request.",
			}
		}

		// Inject the clarification fact into the kernel
		clarificationFact := core.Fact{
			Predicate: "focus_clarification",
			Args:      []any{response},
		}
		if err := m.kernel.Assert(clarificationFact); err != nil {
			return errorMsg(fmt.Errorf("failed to inject clarification: %w", err))
		}

		clarifiedInput := strings.TrimSpace(response)
		if strings.TrimSpace(context) != "" {
			clarifiedInput = fmt.Sprintf("%s\nClarification: %s", context, response)
		}

		// If we have a pending intent, keep it in sync with the clarification.
		if pendingIntent != nil {
			copy := *pendingIntent
			copy.Constraint = strings.TrimSpace(copy.Constraint + "\nUser clarification: " + response)
			m.resumeIntent = &copy
		}

		return m.processInput(clarifiedInput, request)()
	}
}

// formatClarificationRequest formats a clarification request for display
func (m Model) formatClarificationRequest(state ClarificationState) string {
	var sb strings.Builder

	sb.WriteString("**I need some clarification:**\n\n")
	sb.WriteString(state.Question)
	sb.WriteString("\n\n")

	if len(state.Options) > 0 {
		sb.WriteString("**Options:**\n")
		for i, opt := range state.Options {
			if i == m.selectedOption {
				sb.WriteString(fmt.Sprintf("  -> **%d. %s** <-\n", i+1, opt))
			} else {
				sb.WriteString(fmt.Sprintf("    %d. %s\n", i+1, opt))
			}
		}
		sb.WriteString("\n_Use arrow keys to select, Enter to confirm, or type a custom answer_")
	}

	return sb.String()
}

// =============================================================================
// DREAM LEARNING HANDLERS (§8.3.1)
// =============================================================================

// handleDreamLearningConfirmation processes user confirmation of dream state learnings
func (m Model) handleDreamLearningConfirmation(input string) (tea.Model, tea.Cmd) {
	// Add user message to history
	m = m.pushUserMsg(input)
	m.textarea.Reset()

	// Confirm staged learnings
	confirmed := m.dreamCollector.ConfirmLearnings(input)
	if len(confirmed) == 0 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "No pending learnings to confirm. Ask me a hypothetical first with phrases like \"what if\" or \"imagine\".",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.isLoading = false
		return m, nil
	}

	// Route confirmed learnings to appropriate stores
	results := m.dreamRouter.RouteLearnings(confirmed)

	// Build response message
	var sb strings.Builder
	sb.WriteString("✅ **Learned from Dream State consultation:**\n\n")

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
			// Find the learning to show what was learned
			for _, l := range confirmed {
				if l.ID == r.LearningID {
					typeIcon := "📝"
					switch l.Type {
					case core.LearningTypeProcedural:
						typeIcon = "📋"
					case core.LearningTypeToolNeed:
						typeIcon = "🔧"
					case core.LearningTypeRiskPattern:
						typeIcon = "⚠️"
					case core.LearningTypePreference:
						typeIcon = "⭐"
					}
					// Truncate long content
					content := l.Content
					if len(content) > 100 {
						content = content[:100] + "..."
					}
					sb.WriteString(fmt.Sprintf("%s **%s** → %s\n   *%s*\n\n", typeIcon, l.Type, r.Destination, content))
					break
				}
			}
		}
	}

	if successCount > 0 {
		sb.WriteString(fmt.Sprintf("I'll remember these %d insights for future tasks.", successCount))
	}

	m = m.pushAssistantMsg(sb.String())

	m.isLoading = false
	return m, nil
}

// handleDreamLearningCorrection processes user corrections to dream state learnings
func (m Model) handleDreamLearningCorrection(input string) (tea.Model, tea.Cmd) {
	// Add user message to history
	m = m.pushUserMsg(input)
	m.textarea.Reset()

	// Extract the correction content (everything after the trigger phrase)
	correction := extractCorrectionContent(input)
	if correction == "" {
		correction = input
	}

	// Determine learning type from context
	learningType := core.LearningTypePreference // Default to preference
	lower := strings.ToLower(correction)
	if strings.Contains(lower, "tool") || strings.Contains(lower, "command") || strings.Contains(lower, "script") {
		learningType = core.LearningTypeToolNeed
	} else if strings.Contains(lower, "risk") || strings.Contains(lower, "careful") || strings.Contains(lower, "never") {
		learningType = core.LearningTypeRiskPattern
	} else if strings.Contains(lower, "step") || strings.Contains(lower, "first") || strings.Contains(lower, "then") {
		learningType = core.LearningTypeProcedural
	}

	// Learn the correction at high confidence
	learning := m.dreamCollector.LearnCorrection(correction, learningType)

	// Route to storage
	results := m.dreamRouter.RouteLearnings([]*core.DreamLearning{learning})

	var response string
	if len(results) > 0 && results[0].Success {
		response = fmt.Sprintf("✅ **Correction learned:**\n\n*%s*\n\n→ Stored in %s with high confidence (0.9)\n\nI'll apply this in future tasks.", correction, results[0].Destination)
	} else {
		response = fmt.Sprintf("✅ **Correction noted:**\n\n*%s*\n\nI'll remember this for future tasks.", correction)
	}

	m = m.pushAssistantMsg(response)

	m.isLoading = false
	return m, nil
}

// handleDreamPlanExecution processes user approval to execute a dream plan.
// This is triggered by phrases like "do it", "execute that", "run the plan".
func (m Model) handleDreamPlanExecution(input string) (tea.Model, tea.Cmd) {
	// Add user message to history
	m = m.pushUserMsg(input)
	m.textarea.Reset()

	plan := m.dreamPlanManager.GetCurrentPlan()
	if plan == nil || len(plan.Subtasks) == 0 {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: "No dream plan to execute. Ask a hypothetical question first (e.g., \"what if we added caching?\").",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.isLoading = false
		return m, nil
	}

	// Approve the plan
	if err := m.dreamPlanManager.ApprovePlan(); err != nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Cannot execute plan: %v", err),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.isLoading = false
		return m, nil
	}

	// Convert DreamSubtasks to the existing Subtask format for multi-step execution
	subtasks := make([]Subtask, len(plan.Subtasks))
	for i, ds := range plan.Subtasks {
		subtasks[i] = Subtask{
			ID:          ds.ID,
			Description: ds.Description,
			ShardType:   ds.ShardType,
			IsMutation:  ds.IsMutation,
		}
	}

	// Build execution summary
	var sb strings.Builder
	sb.WriteString("## Executing Dream Plan\n\n")
	sb.WriteString(fmt.Sprintf("**Goal:** %s\n\n", plan.Hypothetical))
	sb.WriteString(fmt.Sprintf("**Steps:** %d | **Risk:** %s | **Mode:** %s\n\n",
		len(subtasks), plan.RiskLevel, workModeLabel(m.workMode())))

	sb.WriteString("| # | Step | Shard |\n")
	sb.WriteString("|---|------|-------|\n")
	for i, s := range subtasks {
		mutation := ""
		if s.IsMutation {
			mutation = " *"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s%s | %s |\n", i+1, types.Ellipsize(s.Description, 50), mutation, s.ShardType))
	}
	sb.WriteString("\n_* = mutation (file change)_\n")

	m = m.pushAssistantMsg(sb.String())

	// Start execution using existing multi-step infrastructure
	if err := m.dreamPlanManager.StartExecution(); err != nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Failed to start execution: %v", err),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.isLoading = false
		return m, nil
	}

	// The plan manager owns the approved steps until each settles
	// (continuation_ledger.go); approval is consent to run the first ready
	// one. A dream plan is no chat request: no delivery chain carries over.
	m = m.endContinuation()
	m.continuationStep = 1
	m.continuationTotal = len(subtasks)
	first, ok := m.nextPlanStep()
	if !ok {
		m = m.pushAssistantMsg("The approved plan has no step whose dependencies are met; nothing was run.")
		m.dreamPlanManager.CancelPlan()
		return m, nil
	}
	return m.dispatchSubtask(first)
}

// =============================================================================
// LEARNING LOOP (OUROBOROS)
// =============================================================================
