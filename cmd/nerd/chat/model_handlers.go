package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// RENDERING CACHE HELPERS
// =============================================================================

// addMessage appends a message to history and pre-renders it into cache.
// This pre-computation during Update() avoids expensive rendering during View().
func (m Model) addMessage(msg Message) Model {
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

// addMessages appends multiple messages and pre-renders them into cache.
func (m Model) addMessages(msgs ...Message) Model {
	if len(msgs) == 0 {
		return m
	}

	startIdx := len(m.history)
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
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" {
		// Empty Enter: Check for continuation confirmation
		if len(m.pendingSubtasks) > 0 {
			// Treat Enter as confirmation to continue
			return m, func() tea.Msg { return confirmContinueMsg{} }
		}
		return m, nil
	}

	// Patch ingestion mode
	if m.awaitingPatch {
		// Accumulate lines until user types --END--
		if input == "--END--" {
			patch := strings.Join(m.pendingPatchLines, "\n")
			m.pendingPatchLines = nil
			m.awaitingPatch = false
			m.textarea.Placeholder = "Ask me anything... (Enter to send, Shift+Enter for newline, Ctrl+C to exit)"
			m.history = append(m.history, Message{
				Role:    "assistant",
				Content: applyPatchResult(m.workspace, patch),
				Time:    time.Now(),
			})
			m.textarea.Reset()
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		m.pendingPatchLines = append(m.pendingPatchLines, input)
		m.textarea.Reset()
		return m, nil
	}

	// Check for special commands
	if strings.HasPrefix(input, "/") {
		return m.handleCommand(input)
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
	m.history = append(m.history, Message{
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
	if m.awaitingAgentDefinition {
		return m.handleAgentWizardInput(input)
	}

	// Config wizard mode
	if m.awaitingConfigWizard {
		return m.handleConfigWizardInput(input)
	}

	// Northstar wizard mode
	if m.awaitingNorthstar {
		return m.handleNorthstarWizardInput(input)
	}

	// Onboarding wizard mode (first-run experience)
	if m.awaitingOnboarding {
		return m.handleOnboardingInput(input)
	}

	m.isLoading = true

	// Check for negative feedback auto-trigger
	if isNegativeFeedback(input) {
		return m.triggerLearningLoop(input)
	}

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
		m.processInput(input),
	)
}

// =============================================================================
// CLARIFICATION HANDLERS
// =============================================================================

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
	m.history = append(m.history, Message{
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
	m.awaitingClarification = false
	m.clarificationState = nil
	m.selectedOption = 0
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

	// Resume processing with clarification response
	return m, tea.Batch(
		m.spinner.Tick,
		m.processClarificationResponse(response, pendingIntent, clarifyContext),
	)
}

// processClarificationResponse continues processing after user provides clarification
func (m Model) processClarificationResponse(response string, pendingIntent *perception.Intent, context string) tea.Cmd {
	return func() tea.Msg {
		// Guard: kernel must be initialized
		if m.kernel == nil {
			return errorMsg(fmt.Errorf("system not ready: kernel not initialized"))
		}

		// Inject the clarification fact into the kernel
		clarificationFact := core.Fact{
			Predicate: "focus_clarification",
			Args:      []interface{}{response},
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
			pendingIntent.Target = response
		}

		return m.processInput(clarifiedInput)()
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
	m.history = append(m.history, Message{
		Role:    "user",
		Content: input,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()

	// Confirm staged learnings
	confirmed := m.dreamCollector.ConfirmLearnings(input)
	if len(confirmed) == 0 {
		m.history = append(m.history, Message{
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

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	m.isLoading = false
	return m, nil
}

// handleDreamLearningCorrection processes user corrections to dream state learnings
func (m Model) handleDreamLearningCorrection(input string) (tea.Model, tea.Cmd) {
	// Add user message to history
	m.history = append(m.history, Message{
		Role:    "user",
		Content: input,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
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

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: response,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	m.isLoading = false
	return m, nil
}

// handleDreamPlanExecution processes user approval to execute a dream plan.
// This is triggered by phrases like "do it", "execute that", "run the plan".
func (m Model) handleDreamPlanExecution(input string) (tea.Model, tea.Cmd) {
	// Add user message to history
	m.history = append(m.history, Message{
		Role:    "user",
		Content: input,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()

	plan := m.dreamPlanManager.GetCurrentPlan()
	if plan == nil || len(plan.Subtasks) == 0 {
		m.history = append(m.history, Message{
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
		m.history = append(m.history, Message{
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
	sb.WriteString(fmt.Sprintf("**Steps:** %d | **Risk:** %s | **Mode:** [%c] %s\n\n",
		len(subtasks), plan.RiskLevel,
		'A'+rune(m.continuationMode), m.continuationMode.String()))

	sb.WriteString("| # | Step | Shard |\n")
	sb.WriteString("|---|------|-------|\n")
	for i, s := range subtasks {
		mutation := ""
		if s.IsMutation {
			mutation = " *"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s%s | %s |\n", i+1, truncateString(s.Description, 50), mutation, s.ShardType))
	}
	sb.WriteString("\n_* = mutation (file change)_\n")

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: sb.String(),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	// Start execution using existing multi-step infrastructure
	if err := m.dreamPlanManager.StartExecution(); err != nil {
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Failed to start execution: %v", err),
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.isLoading = false
		return m, nil
	}

	// Set up pending subtasks for the continuation protocol
	m.pendingSubtasks = subtasks
	m.continuationStep = 1
	m.continuationTotal = len(subtasks)
	m.isLoading = true

	// Execute the first subtask
	if len(subtasks) > 0 {
		first := subtasks[0]
		return m, m.executeSubtask(first.ID, first.Description, first.ShardType)
	}

	m.isLoading = false
	return m, nil
}

// =============================================================================
// LEARNING LOOP (OUROBOROS)
// =============================================================================

// triggerLearningLoop initiates the Ouroboros self-correction process
func (m Model) triggerLearningLoop(userInput string) (tea.Model, tea.Cmd) {
	// Add the user's complaint to history first so the Critic sees it
	m.history = append(m.history, Message{
		Role:    "user",
		Content: userInput,
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()

	// Notify user we are paying attention
	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: "I detect dissatisfaction. Invoking Meta-Cognitive Supervisor to analyze our interaction and learn from this mistake...",
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()

	m.isLoading = true

	learningCmd := func() tea.Msg {
		// Convert history to traces
		var traces []perception.ReasoningTrace
		for _, msg := range m.history {
			t := perception.ReasoningTrace{
				UserPrompt: "...",
				Response:   msg.Content,
				Success:    true,
			}
			if msg.Role == "user" {
				t.UserPrompt = msg.Content
			}
			traces = append(traces, t)
		}

		// Execute Learning
		perception.SharedTaxonomy.SetClient(m.client)
		perception.SharedTaxonomy.SetWorkspace(m.workspace) // Ensure .nerd paths resolve correctly
		fact, err := perception.SharedTaxonomy.LearnFromInteraction(context.Background(), traces)
		if err != nil {
			return responseMsg(fmt.Sprintf("Auto-learning failed: %v", err))
		}
		if fact == "" {
			return responseMsg("I analyzed the interaction but couldn't identify a clear pattern to generalize yet. I will keep this in mind.")
		}
		return responseMsg(fmt.Sprintf("I have crystallized a new rule from this interaction:\n```\n%s\n```\nI will apply this correction in future turns.", fact))
	}

	return m, tea.Batch(m.spinner.Tick, learningCmd)
}
