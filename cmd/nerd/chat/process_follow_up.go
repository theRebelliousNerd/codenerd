// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains follow-up detection and handling for conversation context.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// FOLLOW-UP DETECTION AND HANDLING
// =============================================================================

// FollowUpType indicates the type of follow-up question detected.
type FollowUpType string

const (
	FollowUpNone     FollowUpType = ""
	FollowUpShowMore FollowUpType = "show_more" // "what are the other suggestions?"
	FollowUpExplain  FollowUpType = "explain"   // "explain the first warning"
	FollowUpFilter   FollowUpType = "filter"    // "show only critical issues"
	FollowUpDetails  FollowUpType = "details"   // "tell me more about X"
	FollowUpGeneric  FollowUpType = "generic"   // Generic follow-up
)

// detectFollowUpQuestion checks if the input is a follow-up about the last shard result.
// Returns true and the follow-up type if detected.
func detectFollowUpQuestion(input string, lastResult *ShardResult) (bool, FollowUpType) {
	if lastResult == nil {
		return false, FollowUpNone
	}

	lower := strings.ToLower(input)

	// Patterns that indicate follow-up questions about previous output
	showMorePatterns := []string{
		"what are the other",
		"show me the other",
		"show all",
		"list all",
		"what other",
		"more details",
		"full list",
		"complete list",
		"all the warnings",
		"all warnings",
		"all the suggestions",
		"all suggestions",
		"all the findings",
		"all findings",
		"rest of",
		"remaining",
	}

	explainPatterns := []string{
		"explain the",
		"what does",
		"why is",
		"tell me about",
		"what is the",
		"can you explain",
	}

	filterPatterns := []string{
		"show only",
		"filter by",
		"just the",
		"only show",
		"only the",
	}

	detailPatterns := []string{
		"more detail",
		"more about",
		"details on",
		"elaborate on",
		"expand on",
	}

	// Check patterns
	for _, p := range showMorePatterns {
		if strings.Contains(lower, p) {
			return true, FollowUpShowMore
		}
	}

	for _, p := range explainPatterns {
		if strings.Contains(lower, p) {
			return true, FollowUpExplain
		}
	}

	for _, p := range filterPatterns {
		if strings.Contains(lower, p) {
			return true, FollowUpFilter
		}
	}

	for _, p := range detailPatterns {
		if strings.Contains(lower, p) {
			return true, FollowUpDetails
		}
	}

	// Generic follow-up detection (pronouns referring to previous context)
	genericPatterns := []string{
		"those", "these", "that", "this",
		"the above", "mentioned", "previous",
	}
	for _, p := range genericPatterns {
		if strings.Contains(lower, p) {
			// Only trigger if it's a short question (likely referential)
			if len(input) < 100 {
				return true, FollowUpGeneric
			}
		}
	}

	return false, FollowUpNone
}

// handleFollowUpQuestion processes a follow-up question using conversation context.
func (m Model) handleFollowUpQuestion(ctx context.Context, input string, followUpType FollowUpType) tea.Msg {
	sr := m.lastShardResult
	if sr == nil {
		return responseMsg("I don't have any previous results to reference. Could you provide more context?")
	}

	// Build conversation context with recent history (includes blackboard + compressed context)
	var compressedCtx string
	if m.compressor != nil {
		if ctxStr, err := m.compressor.GetContextString(ctx); err == nil {
			compressedCtx = ctxStr
		}
	}
	convCtx := &ConversationContext{
		RecentTurns:     m.getRecentTurns(6), // Last 6 turns
		LastShardResult: sr,
		TurnNumber:      m.turnCount,
		ShardHistory:    m.shardResultHistory, // Blackboard
		CompressedCtx:   compressedCtx,        // Infinite context
	}

	// For "show more" type questions about reviewer findings, we can answer directly
	if followUpType == FollowUpShowMore && sr.ShardType == "reviewer" && len(sr.Findings) > 0 {
		return m.formatAllFindings(sr, input)
	}

	// For user-defined agents, continue the conversation by re-running the same agent with follow-up context.
	// This avoids dumping internal system action logs while giving the specialist a chance to expand.
	if followUpType != FollowUpNone && sr.ShardType != "" && m.shardMgr != nil {
		if registry := m.loadAgentRegistry(); registry != nil {
			isRegistered := false
			for _, agent := range registry.Agents {
				if strings.EqualFold(agent.Name, sr.ShardType) {
					isRegistered = true
					break
				}
			}
			if isRegistered {
				m.ReportStatus(fmt.Sprintf("Follow-up: spawning %s...", sr.ShardType))

				const maxPrev = 1500
				prev := sr.RawOutput
				if len(prev) > maxPrev {
					prev = prev[:maxPrev] + "\n... (truncated)"
				}

				spawnTask := fmt.Sprintf(`Follow-up request: %s

Original task: %s

Previous answer (excerpt):
%s

Please respond with substantially more detail than before. Expand key findings, include concrete syntax/code examples, and call out trade-offs and edge-cases.`, strings.TrimSpace(input), strings.TrimSpace(sr.Task), strings.TrimSpace(prev))

				displayTask := fmt.Sprintf("follow-up: %s (original: %s)", strings.TrimSpace(input), strings.TrimSpace(sr.Task))

				sessionCtx := m.buildSessionContext(ctx)
				result, err := m.shardMgr.SpawnWithPriority(ctx, sr.ShardType, spawnTask, sessionCtx, core.PriorityHigh)

				shardID := fmt.Sprintf("%s-followup-%d", sr.ShardType, time.Now().UnixNano())
				facts := m.shardMgr.ResultToFacts(shardID, sr.ShardType, displayTask, result, err)
				if m.kernel != nil && len(facts) > 0 {
					_ = m.kernel.LoadFacts(facts)
				}

				if err != nil {
					return errorMsg(fmt.Errorf("follow-up shard spawn failed: %w", err))
				}

				response := fmt.Sprintf(`## Shard Execution Complete

**Agent**: %s
**Task**: %s

### Result
%s`, sr.ShardType, displayTask, result)

				return assistantMsg{
					Surface: response,
					ShardResult: &ShardResultPayload{
						ShardType: sr.ShardType,
						Task:      displayTask,
						Result:    result,
						Facts:     facts,
					},
				}
			}
		}
	}

	// For other follow-ups, use LLM with full context
	intent := perception.Intent{
		Category:   "/query",
		Verb:       "/explain",
		Target:     "previous_output",
		Constraint: string(followUpType),
		Confidence: 0.9,
		Response:   "", // Will be generated
	}

	// Get context facts from kernel
	contextFacts, _ := m.kernel.Query("context_to_inject")

	// Build the follow-up prompt with conversation context
	systemPrompt := `You are answering a follow-up question about a previous shard execution result.
The user is asking about details from the last operation. Use the "Last Shard Execution Result"
section above to provide accurate, specific answers. Reference the actual findings, warnings,
or metrics from that result.`

	artOutput, err := articulateWithConversation(ctx, m.client, intent,
		payloadForArticulation(intent, nil), contextFacts, nil, systemPrompt, convCtx)
	if err != nil {
		return errorMsg(fmt.Errorf("follow-up articulation failed: %w", err))
	}

	return responseMsg(artOutput.Surface)
}

// formatAllFindings formats all findings from a shard result for display.
func (m Model) formatAllFindings(sr *ShardResult, query string) tea.Msg {
	var sb strings.Builder
	lower := strings.ToLower(query)

	sb.WriteString(fmt.Sprintf("## All Findings from %s\n\n", strings.Title(sr.ShardType)))
	sb.WriteString(fmt.Sprintf("**Task**: %s\n\n", sr.Task))

	// Determine filter based on query
	showWarnings := strings.Contains(lower, "warning")
	showInfo := strings.Contains(lower, "info") || strings.Contains(lower, "suggestion")
	showErrors := strings.Contains(lower, "error") || strings.Contains(lower, "critical")
	showAll := !showWarnings && !showInfo && !showErrors

	// Group findings by severity
	var critical, errors, warnings, infos []map[string]any
	for _, f := range sr.Findings {
		sev, _ := f["severity"].(string)
		switch strings.ToLower(sev) {
		case "critical":
			critical = append(critical, f)
		case "error":
			errors = append(errors, f)
		case "warning":
			warnings = append(warnings, f)
		case "info":
			infos = append(infos, f)
		default:
			infos = append(infos, f) // Default to info
		}
	}

	// Format each group
	if (showAll || showErrors) && len(critical) > 0 {
		sb.WriteString("### Critical Issues\n")
		for _, f := range critical {
			formatFinding(&sb, f)
		}
		sb.WriteString("\n")
	}

	if (showAll || showErrors) && len(errors) > 0 {
		sb.WriteString("### Errors\n")
		for _, f := range errors {
			formatFinding(&sb, f)
		}
		sb.WriteString("\n")
	}

	if (showAll || showWarnings) && len(warnings) > 0 {
		sb.WriteString("### Warnings\n")
		for _, f := range warnings {
			formatFinding(&sb, f)
		}
		sb.WriteString("\n")
	}

	if (showAll || showInfo) && len(infos) > 0 {
		sb.WriteString("### Info/Suggestions\n")
		for _, f := range infos {
			formatFinding(&sb, f)
		}
		sb.WriteString("\n")
	}

	// Add metrics if available
	if len(sr.Metrics) > 0 {
		sb.WriteString("### Metrics\n")
		for k, v := range sr.Metrics {
			sb.WriteString(fmt.Sprintf("- **%s**: %v\n", k, v))
		}
	}

	return responseMsg(sb.String())
}

// formatFinding formats a single finding for display.
func formatFinding(sb *strings.Builder, f map[string]any) {
	file, _ := f["file"].(string)
	line, _ := f["line"].(float64)
	msg, _ := f["message"].(string)
	category, _ := f["category"].(string)

	if file != "" && line > 0 {
		sb.WriteString(fmt.Sprintf("- **%s:%d** - %s", file, int(line), msg))
	} else if file != "" {
		sb.WriteString(fmt.Sprintf("- **%s** - %s", file, msg))
	} else {
		sb.WriteString(fmt.Sprintf("- %s", msg))
	}

	if category != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", category))
	}
	sb.WriteString("\n")
}

// getRecentTurns returns the last N conversation turns.
func (m Model) getRecentTurns(n int) []Message {
	if len(m.history) <= n {
		return m.history
	}
	return m.history[len(m.history)-n:]
}

func (m Model) createAgentFromPrompt(description string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		systemPrompt := "You design specialist software agents. Respond in English. Return JSON with fields: name (CamelCase, no spaces), topic (<=80 chars), knowledge_path (path string). Keep responses compact."
		userPrompt := fmt.Sprintf("Workspace: %s\nSpecialist description: %s", m.workspace, description)

		raw, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			return errorMsg(fmt.Errorf("agent creation failed: %w", err))
		}

		var out struct {
			Name          string `json:"name"`
			Topic         string `json:"topic"`
			KnowledgePath string `json:"knowledge_path"`
		}

		if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil {
			return errorMsg(fmt.Errorf("agent creation: invalid JSON from LLM: %w (got: %s)", err, raw))
		}

		name := strings.TrimSpace(out.Name)
		if name == "" {
			return errorMsg(fmt.Errorf("agent creation: LLM returned empty name"))
		}
		topic := strings.TrimSpace(out.Topic)
		kp := strings.TrimSpace(out.KnowledgePath)
		if kp == "" {
			// Use workspace root for .nerd path to avoid creating in wrong directory
			kp = filepath.Join(m.workspace, ".nerd", "shards", fmt.Sprintf("%s_knowledge.db", name))
		}

		cfg := core.DefaultSpecialistConfig(name, kp)
		m.shardMgr.DefineProfile(name, cfg)
		_ = persistAgentProfile(m.workspace, name, "persistent", kp, 0, "ready")

		surface := fmt.Sprintf("## Agent Created: %s\n\n**Topic**: %s\n**Knowledge Path**: %s\n\nNext: `/spawn %s <task>`", name, topic, kp, name)
		return responseMsg(surface)
	}
}
