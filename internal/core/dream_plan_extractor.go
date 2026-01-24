// Package core implements Dream Plan extraction from Dream State consultations.
// This file parses shard perspectives into actionable subtasks.
package core

import (
	"codenerd/internal/logging"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Action verb patterns for classifying subtasks
var (
	// Mutation actions - these modify files or state
	mutationVerbs = []string{
		"create", "add", "write", "implement", "build", "generate",
		"modify", "update", "change", "edit", "refactor", "fix",
		"delete", "remove", "replace", "rename", "move",
		"install", "configure", "setup", "deploy",
	}

	// Query actions - these only read or analyze
	queryVerbs = []string{
		"read", "analyze", "review", "check", "verify", "validate",
		"test", "examine", "inspect", "scan", "search", "find",
		"list", "show", "display", "report", "document",
	}

	// Shard type mapping based on action keywords
	shardTypeKeywords = map[string][]string{
		"coder":      {"create", "implement", "write", "add", "modify", "fix", "refactor", "update", "build", "generate"},
		"tester":     {"test", "verify", "validate", "check"},
		"reviewer":   {"review", "analyze", "examine", "inspect", "audit"},
		"researcher": {"research", "find", "search", "document", "explore"},
	}

	// Numbered step pattern: matches "1.", "1)", "Step 1:", etc.
	numberedStepPattern = regexp.MustCompile(`(?m)^[\s]*(?:(?:\d+)[.\):]|(?:Step\s+\d+)[:\s]|[-*•])\s*(.+)$`)

	// File path pattern
	filePathPattern = regexp.MustCompile(`(?:[a-zA-Z]:[/\\]|[./])?[\w/\\-]+\.[a-zA-Z0-9]+`)

	// Risk keywords for assessment
	riskKeywords = []string{
		"careful", "caution", "warning", "danger", "risk",
		"breaking", "destructive", "irreversible", "production",
		"security", "sensitive", "critical", "important",
	}

	// Quoted target pattern for extractTarget
	quotedTargetPattern = regexp.MustCompile(`["'\x60]([^"'\x60]+)["'\x60]`)

	// Whitespace pattern for normalizeDescription
	whitespacePattern = regexp.MustCompile(`\s+`)

	// Question pattern for extractQuestions
	questionPattern = regexp.MustCompile(`(?m)\?[^\n]*`)
)

// ExtractDreamPlan parses shard consultations into an actionable execution plan.
// It extracts numbered steps from each shard's perspective, deduplicates similar steps,
// and orders them by dependencies.
func ExtractDreamPlan(hypothetical string, consultations []DreamConsultation) (*DreamPlan, error) {
	if len(consultations) == 0 {
		return nil, fmt.Errorf("no consultations to extract plan from")
	}

	planID := fmt.Sprintf("dream-%d", time.Now().UnixNano())
	plan := NewDreamPlan(planID, hypothetical)

	// Track consulted shards
	for _, c := range consultations {
		if c.Error == nil {
			plan.ConsultedShards = append(plan.ConsultedShards, c.ShardName)
		}
	}

	// Collect all tools mentioned
	toolSet := make(map[string]bool)
	for _, c := range consultations {
		for _, tool := range c.Tools {
			toolSet[tool] = true
		}
	}
	for tool := range toolSet {
		plan.RequiredTools = append(plan.RequiredTools, tool)
	}

	// Extract subtasks from each consultation
	allSubtasks := make([]DreamSubtask, 0)
	subtaskID := 0

	for _, consultation := range consultations {
		if consultation.Error != nil {
			continue
		}

		steps := extractStepsFromPerspective(consultation.Perspective)
		for _, step := range steps {
			subtask := DreamSubtask{
				ID:          fmt.Sprintf("dream-%d", subtaskID),
				ShardName:   consultation.ShardName,
				Description: step,
				Action:      classifyAction(step),
				Target:      extractTarget(step),
				IsMutation:  isMutationAction(step),
				Status:      SubtaskStatusPending,
			}

			// Determine shard type based on action
			subtask.ShardType = inferShardType(subtask.Action, consultation.ShardType)

			allSubtasks = append(allSubtasks, subtask)
			subtaskID++
		}
	}

	// Deduplicate and order subtasks
	deduped := deduplicateSubtasks(allSubtasks)
	ordered := orderByDependencies(deduped)

	// Add to plan with correct ordering
	for i, subtask := range ordered {
		subtask.Order = i
		subtask.ID = fmt.Sprintf("dream-%d", i)
		plan.AddSubtask(subtask)
	}

	// Assess risk level
	plan.RiskLevel = assessRiskLevel(consultations)

	// Extract any pending questions
	plan.PendingQuestions = extractQuestions(consultations)

	logging.Dream("Extracted %d subtasks from %d consultations, risk: %s",
		len(plan.Subtasks), len(consultations), plan.RiskLevel)

	return plan, nil
}

// extractStepsFromPerspective parses numbered or bulleted steps from shard output.
func extractStepsFromPerspective(perspective string) []string {
	steps := make([]string, 0)

	// Try to find numbered steps
	matches := numberedStepPattern.FindAllStringSubmatch(perspective, -1)
	for _, match := range matches {
		if len(match) > 1 {
			step := strings.TrimSpace(match[1])
			// Filter out meta-commentary
			if isActionableStep(step) {
				steps = append(steps, step)
			}
		}
	}

	// If no numbered steps found, try to parse paragraphs as implicit steps
	if len(steps) == 0 {
		paragraphs := strings.Split(perspective, "\n\n")
		for _, para := range paragraphs {
			para = strings.TrimSpace(para)
			if len(para) > 20 && len(para) < 500 && isActionableStep(para) {
				steps = append(steps, para)
			}
		}
	}

	return steps
}

// isActionableStep returns true if the text describes a concrete action.
func isActionableStep(text string) bool {
	lower := strings.ToLower(text)

	// Must contain an action verb
	hasAction := false
	for _, verb := range append(mutationVerbs, queryVerbs...) {
		if strings.Contains(lower, verb) {
			hasAction = true
			break
		}
	}

	// Filter out meta-commentary
	metaPhrases := []string{
		"i would", "we could", "you might", "in general",
		"typically", "usually", "remember that", "note that",
		"keep in mind", "be aware", "don't forget",
	}
	for _, phrase := range metaPhrases {
		if strings.HasPrefix(lower, phrase) {
			return false
		}
	}

	return hasAction
}

// classifyAction extracts the primary action verb from a step description.
func classifyAction(step string) string {
	lower := strings.ToLower(step)

	// Check mutation verbs first (higher priority)
	for _, verb := range mutationVerbs {
		if strings.Contains(lower, verb) {
			return verb
		}
	}

	// Then query verbs
	for _, verb := range queryVerbs {
		if strings.Contains(lower, verb) {
			return verb
		}
	}

	return "execute" // default
}

// extractTarget attempts to find a file path or symbol in the step.
func extractTarget(step string) string {
	// Try to find a file path
	matches := filePathPattern.FindAllString(step, -1)
	if len(matches) > 0 {
		return matches[0]
	}

	// Look for quoted strings as potential targets
	qMatches := quotedTargetPattern.FindAllStringSubmatch(step, -1)
	if len(qMatches) > 0 && len(qMatches[0]) > 1 {
		return qMatches[0][1]
	}

	return ""
}

// isMutationAction returns true if the action modifies files or state.
func isMutationAction(step string) bool {
	lower := strings.ToLower(step)
	for _, verb := range mutationVerbs {
		if strings.Contains(lower, verb) {
			return true
		}
	}
	return false
}

// inferShardType determines the appropriate shard type for an action.
func inferShardType(action, originalShardType string) string {
	// If the original shard type is specific, prefer it
	if originalShardType != "" && originalShardType != "ephemeral" {
		return originalShardType
	}

	// Otherwise, infer from action
	for shardType, keywords := range shardTypeKeywords {
		for _, keyword := range keywords {
			if action == keyword {
				return shardType
			}
		}
	}

	return "coder" // default to coder for most tasks
}

// deduplicateSubtasks removes similar steps, keeping the most detailed version.
func deduplicateSubtasks(subtasks []DreamSubtask) []DreamSubtask {
	if len(subtasks) == 0 {
		return subtasks
	}

	unique := make([]DreamSubtask, 0, len(subtasks))
	seen := make(map[string]int) // normalized description -> index in unique

	for _, subtask := range subtasks {
		normalized := normalizeDescription(subtask.Description)

		if existingIdx, exists := seen[normalized]; exists {
			// Keep the more detailed version
			if len(subtask.Description) > len(unique[existingIdx].Description) {
				unique[existingIdx] = subtask
			}
		} else {
			seen[normalized] = len(unique)
			unique = append(unique, subtask)
		}
	}

	return unique
}

// normalizeDescription creates a simplified version for comparison.
func normalizeDescription(desc string) string {
	// Lowercase and remove extra whitespace
	normalized := strings.ToLower(strings.TrimSpace(desc))
	normalized = whitespacePattern.ReplaceAllString(normalized, " ")

	// Remove common prefixes
	prefixes := []string{"first, ", "then, ", "next, ", "finally, ", "also, "}
	for _, prefix := range prefixes {
		normalized = strings.TrimPrefix(normalized, prefix)
	}

	// Take first N words for comparison
	words := strings.Fields(normalized)
	if len(words) > 10 {
		words = words[:10]
	}

	return strings.Join(words, " ")
}

// orderByDependencies arranges subtasks with analysis/review before implementation.
func orderByDependencies(subtasks []DreamSubtask) []DreamSubtask {
	if len(subtasks) == 0 {
		return subtasks
	}

	// Categorize by priority
	analysis := make([]DreamSubtask, 0)
	implementation := make([]DreamSubtask, 0)
	testing := make([]DreamSubtask, 0)

	for _, s := range subtasks {
		switch s.ShardType {
		case "reviewer", "researcher":
			analysis = append(analysis, s)
		case "tester":
			testing = append(testing, s)
		default:
			implementation = append(implementation, s)
		}
	}

	// Order: analysis -> implementation -> testing
	ordered := make([]DreamSubtask, 0, len(subtasks))
	ordered = append(ordered, analysis...)
	ordered = append(ordered, implementation...)
	ordered = append(ordered, testing...)

	// Set dependencies: implementation depends on analysis, testing depends on implementation
	analysisEnd := len(analysis)
	implEnd := analysisEnd + len(implementation)

	for i := range ordered {
		if i >= analysisEnd && i < implEnd && analysisEnd > 0 {
			// Implementation depends on last analysis step
			ordered[i].DependsOn = []int{analysisEnd - 1}
		} else if i >= implEnd && implEnd > analysisEnd {
			// Testing depends on last implementation step
			ordered[i].DependsOn = []int{implEnd - 1}
		}
	}

	return ordered
}

// assessRiskLevel evaluates concerns to determine overall risk.
func assessRiskLevel(consultations []DreamConsultation) string {
	totalConcerns := 0
	highRiskCount := 0

	for _, c := range consultations {
		totalConcerns += len(c.Concerns)
		for _, concern := range c.Concerns {
			lower := strings.ToLower(concern)
			for _, keyword := range riskKeywords {
				if strings.Contains(lower, keyword) {
					highRiskCount++
					break
				}
			}
		}
	}

	if highRiskCount >= 3 || totalConcerns >= 5 {
		return "high"
	} else if highRiskCount >= 1 || totalConcerns >= 2 {
		return "medium"
	}
	return "low"
}

// extractQuestions finds clarification questions from consultations.
func extractQuestions(consultations []DreamConsultation) []string {
	questions := make([]string, 0)

	for _, c := range consultations {
		matches := questionPattern.FindAllString(c.Perspective, -1)
		for _, match := range matches {
			q := strings.TrimSpace(match)
			if len(q) > 10 && len(q) < 200 {
				questions = append(questions, q)
			}
		}
	}

	return questions
}
