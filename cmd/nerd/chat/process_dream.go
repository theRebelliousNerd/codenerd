// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains Dream State multi-agent simulation and learning mode.
package chat

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
	"codenerd/internal/world"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// DREAM STATE - Multi-Agent Simulation/Learning Mode
// =============================================================================

// DreamConsultation holds a shard's perspective on a hypothetical task.
type DreamConsultation struct {
	ShardName   string // e.g., "coder", "my-go-expert"
	ShardType   string // e.g., "ephemeral", "persistent", "system"
	Perspective string
	Tools       []string
	Concerns    []string
	Error       error
}

// containsWord checks if text contains keyword as a whole word (not substring).
// For short keywords (<=3 chars), requires word boundaries.
// For longer keywords, substring match is sufficient.
func containsWord(text, keyword string) bool {
	if len(keyword) <= 3 {
		// Short keyword - need word boundary check
		// Use regex with word boundaries
		pattern := `\b` + regexp.QuoteMeta(keyword) + `\b`
		matched, _ := regexp.MatchString(pattern, text)
		return matched
	}
	// Longer keywords - substring is fine
	return strings.Contains(text, keyword)
}

// isShardRelevantToTopic checks if a specialist shard is relevant to the given topic.
// Generic shards (coder, tester, reviewer, researcher) are always relevant.
// Specialist shards are only relevant if their domain matches the topic.
func isShardRelevantToTopic(shardName string, topic string) bool {
	lower := strings.ToLower(topic)
	shardLower := strings.ToLower(shardName)

	// Generic shards are always relevant
	genericShards := []string{"coder", "tester", "reviewer", "researcher", "security", "planner"}
	for _, g := range genericShards {
		if strings.Contains(shardLower, g) {
			return true
		}
	}

	// Domain-specific keyword mapping
	domainKeywords := map[string][]string{
		"go":        {"go", "golang", "gin", "echo", "fiber", "cobra", "viper", "bubbletea", "lipgloss", "rod", "chromedp"},
		"rod":       {"browser", "automation", "scrape", "scraping", "chromedp", "puppeteer", "selenium", "web driver", "headless"},
		"mangle":    {"mangle", "datalog", "logic", "predicate", "rule", "query", "facts", "kernel"},
		"bubbletea": {"tui", "terminal", "cli", "bubbletea", "bubbles", "charm", "lipgloss", "interactive"},
		"cobra":     {"cli", "command", "flag", "subcommand", "cobra", "viper", "config"},
		"react":     {"react", "jsx", "component", "hook", "frontend", "next.js", "nextjs"},
		"vue":       {"vue", "vuex", "nuxt", "component", "frontend"},
		"python":    {"python", "pip", "django", "flask", "fastapi", "pandas", "numpy"},
		"rust":      {"rust", "cargo", "tokio", "async", "ownership", "borrow"},
		"test":      {"test", "testing", "spec", "coverage", "mock", "stub", "assert"},
		"security":  {"security", "audit", "vulnerability", "owasp", "injection", "xss", "csrf"},
	}

	// Check if shard name contains a domain keyword
	for domain, keywords := range domainKeywords {
		if strings.Contains(shardLower, domain) {
			// This is a domain specialist - check if topic matches ANY of its keywords
			for _, kw := range keywords {
				if containsWord(lower, kw) {
					return true // Topic matches this specialist's domain
				}
			}
			// Topic doesn't match this specialist's domain - skip it
			return false
		}
	}

	// Unknown specialist - include by default (might be user-defined)
	return true
}

// handleDreamState implements the "what if" simulation mode.
// It consults ALL available shards (Type A ephemeral, Type B/U persistent specialists,
// and selected Type S system shards) in parallel WITHOUT executing anything,
// aggregates their perspectives, and presents a comprehensive plan for human-in-the-loop learning.
func (m Model) handleDreamState(ctx context.Context, intent perception.Intent, input string) tea.Msg {
	if m.shardMgr == nil {
		return errorMsg(fmt.Errorf("dream state requires shard manager"))
	}

	hypothetical := intent.Target
	if hypothetical == "" {
		hypothetical = input
	}

	// Get ALL available shards dynamically (Type A, B, U, and selected S)
	availableShards := m.shardMgr.ListAvailableShards()

	// DEBUG: Log all discovered shards
	logging.Dream("Discovered %d available shards", len(availableShards))
	for i, shard := range availableShards {
		logging.Dream("  [%d] Name: %s, Type: %s, HasKnowledge: %v", i+1, shard.Name, shard.Type, shard.HasKnowledge)
	}

	// Filter shards to consult - include all except low-level system internals
	skipShards := map[string]bool{
		"perception_firewall":  true, // Internal routing - not useful to consult
		"tactile_router":       true, // Internal routing - not useful to consult
		"world_model_ingestor": true, // Background service - not useful to consult
	}

	// Sort shards: persistent specialists FIRST, then ephemeral, then system
	// This prioritizes domain experts from agents.json before generalists
	type shardPriority struct {
		name         string
		shardType    types.ShardType
		hasKnowledge bool
		priority     int // Lower = higher priority
	}

	var prioritizedShards []shardPriority
	var shardDescriptions = make(map[string]string)

	for _, shard := range availableShards {
		if skipShards[shard.Name] {
			logging.Dream("Skipping internal shard: %s", shard.Name)
			continue
		}

		// Skip specialists that aren't relevant to this topic
		if shard.HasKnowledge && !isShardRelevantToTopic(shard.Name, hypothetical) {
			logging.Dream("Skipping irrelevant specialist: %s (topic: %s)", shard.Name, hypothetical)
			continue
		}

		// Calculate priority (lower = consulted first)
		priority := 100
		switch shard.Type {
		case types.ShardTypePersistent:
			if shard.HasKnowledge {
				priority = 1 // Persistent specialists with knowledge = highest priority
			} else {
				priority = 2 // Persistent without knowledge
			}
		case types.ShardTypeUser:
			if shard.HasKnowledge {
				priority = 3 // User-defined specialists with knowledge
			} else {
				priority = 4 // User-defined without knowledge
			}
		case types.ShardTypeEphemeral:
			priority = 10 // Generalist ephemeral shards
		case types.ShardTypeSystem:
			priority = 20 // System shards last
		}

		prioritizedShards = append(prioritizedShards, shardPriority{
			name:         shard.Name,
			shardType:    shard.Type,
			hasKnowledge: shard.HasKnowledge,
			priority:     priority,
		})

		// Build description for prompt context
		typeLabel := string(shard.Type)
		if shard.HasKnowledge {
			typeLabel += " with domain knowledge"
		}
		shardDescriptions[shard.Name] = typeLabel
	}

	// Sort by priority (lower first)
	sort.Slice(prioritizedShards, func(i, j int) bool {
		return prioritizedShards[i].priority < prioritizedShards[j].priority
	})

	// Extract sorted names
	var shardTypes []string
	for _, s := range prioritizedShards {
		shardTypes = append(shardTypes, s.name)
	}

	// DEBUG: Log shards that will be consulted (in priority order)
	logging.Dream("Will consult %d shards (priority ordered - specialists first)", len(shardTypes))
	for i, s := range prioritizedShards {
		knowledgeTag := ""
		if s.hasKnowledge {
			knowledgeTag = " ★"
		}
		logging.Dream("  [%d] %s (%s, priority=%d)%s", i+1, s.name, s.shardType, s.priority, knowledgeTag)
	}

	// Fallback to core shards if none found
	if len(shardTypes) == 0 {
		shardTypes = []string{"coder", "tester", "reviewer", "researcher"}
	}

	consultPromptTemplate := `DREAM STATE CONSULTATION - DO NOT EXECUTE ANYTHING

You are being consulted about a HYPOTHETICAL task. The user wants to understand how you would approach this WITHOUT actually doing it.

Hypothetical Task: %s

As the %s agent, provide your perspective:

1. **Your Role**: What would you specifically handle?
2. **Steps You'd Take**: Numbered list of actions (but don't do them)
3. **Tools You'd Use**: What existing tools/commands would you need?
4. **Tools You'd Need Created**: What tools don't exist that you'd want?
5. **Dependencies**: What would you need from other agents first?
6. **Risks/Concerns**: What could go wrong?
7. **Questions**: What clarifications would you need from the user?

Remember: This is a SIMULATION. Describe what you WOULD do, not what you ARE doing.
Format your response as a structured analysis.`

	// Rate limit: ~1.6 API calls per second max - process SEQUENTIALLY
	// 1 second between shards, each shard's LLM call has 600ms minimum spacing
	logging.Dream("Rate limiting: processing shards sequentially (1s delay between)")

	// Longer timeout for sequential processing (90s per shard for large context LLMs)
	consultCtx, cancel := context.WithTimeout(ctx, time.Duration(len(shardTypes)*90)*time.Second)
	defer cancel()

	consultations := make([]DreamConsultation, 0, len(shardTypes))

	// Track specialist responses for early stopping
	specialistResponded := false
	specialistResponseQuality := 0 // Sum of response lengths from specialists

	for i, shardName := range shardTypes {
		// Check if context cancelled
		if consultCtx.Err() != nil {
			logging.Dream("Context cancelled, stopping at shard %d/%d", i, len(shardTypes))
			break
		}

		// Rate limit: wait 1 second between shard spawns (after first one)
		if i > 0 {
			time.Sleep(1 * time.Second)
		}

		name := shardName
		typeDesc := shardDescriptions[name]
		isSpecialist := strings.Contains(typeDesc, "with domain knowledge")

		logging.Dream("[%d/%d] Consulting shard: %s (%s)",
			i+1, len(shardTypes), name, typeDesc)

		prompt := fmt.Sprintf(consultPromptTemplate, hypothetical, name)

		// Pass DreamMode=true so shards know NOT to execute, only describe
		dreamCtx := &types.SessionContext{
			DreamMode: true,
		}
		// Dream mode = low priority (background speculation)
		result, err := m.spawnTaskWithContext(consultCtx, name, prompt, dreamCtx, types.PriorityLow)

		consultation := DreamConsultation{
			ShardName: name,
			ShardType: typeDesc,
			Error:     err,
		}

		if err == nil {
			consultation.Perspective = result
			consultation.Tools = extractToolMentions(result)
			consultation.Concerns = extractConcerns(result)
			logging.Dream("✓ Shard %s responded (%d chars)", name, len(result))

			// Track specialist response quality
			if isSpecialist && len(result) > 200 {
				specialistResponded = true
				specialistResponseQuality += len(result)
			}
		} else {
			logging.Dream("✗ Shard %s failed: %v", name, err)
		}

		consultations = append(consultations, consultation)

		// Early stopping: if we have substantial specialist responses, skip remaining shards
		// A good specialist response (>1000 chars total) is sufficient - don't need generalists
		if specialistResponded && specialistResponseQuality > 1000 && i < len(shardTypes)-1 {
			remainingShards := len(shardTypes) - i - 1
			// Only skip if remaining are lower priority (generalists/system)
			// Check if we've processed all specialists
			allRemainingAreGeneralists := true
			for j := i + 1; j < len(shardTypes); j++ {
				desc := shardDescriptions[shardTypes[j]]
				if strings.Contains(desc, "with domain knowledge") {
					allRemainingAreGeneralists = false
					break
				}
			}
			if allRemainingAreGeneralists {
				logging.Dream("⚡ Early stopping: specialist(s) provided confident answer (%d chars). Skipping %d remaining generalist shards.",
					specialistResponseQuality, remainingShards)
				break
			}
		}
	}

	// DEBUG: Summary of collected consultations
	logging.Dream("Collected %d consultations", len(consultations))
	successCount := 0
	failCount := 0
	for _, c := range consultations {
		if c.Error != nil {
			failCount++
			logging.Dream("  ✗ %s (%s): ERROR - %v", c.ShardName, c.ShardType, c.Error)
		} else {
			successCount++
			logging.Dream("  ✓ %s (%s): OK (%d chars)", c.ShardName, c.ShardType, len(c.Perspective))
		}
	}
	logging.Dream("Summary: %d success, %d failed", successCount, failCount)

	// Aggregate and format the dream state response
	response := formatDreamStateResponse(hypothetical, consultations)

	// Store dream context for learning follow-up
	if m.kernel != nil {
		dreamFact := core.Fact{
			Predicate: "dream_state",
			Args:      []interface{}{hypothetical, time.Now().Unix()},
		}
		_ = m.kernel.Assert(dreamFact)
	}

	// Convert to core.DreamConsultation type for both learnings and plan extraction
	coreConsultations := make([]core.DreamConsultation, len(consultations))
	for i, c := range consultations {
		coreConsultations[i] = core.DreamConsultation{
			ShardName:   c.ShardName,
			ShardType:   c.ShardType,
			Perspective: c.Perspective,
			Tools:       c.Tools,
			Concerns:    c.Concerns,
			Error:       c.Error,
		}
	}

	// Extract learnings from consultations (§8.3.1 Dream Learning)
	if m.dreamCollector != nil {
		learnings := m.dreamCollector.ExtractLearnings(hypothetical, coreConsultations)
		if len(learnings) > 0 {
			logging.Dream("Extracted %d learnable insights, staged for user confirmation", len(learnings))
		}
	}

	// Extract actionable plan from consultations (§8.3.2 Dream Plan Execution)
	if m.dreamPlanManager != nil {
		plan, err := core.ExtractDreamPlan(hypothetical, coreConsultations)
		if err == nil && plan != nil && len(plan.Subtasks) > 0 {
			m.dreamPlanManager.StorePlan(plan)
			logging.Dream("Extracted %d actionable subtasks from dream state", len(plan.Subtasks))
		}
	}

	return assistantMsg{
		Surface:           response,
		DreamHypothetical: hypothetical,
	}
}

// formatDreamStateResponse aggregates shard consultations into a structured response.
func formatDreamStateResponse(hypothetical string, consultations []DreamConsultation) string {
	var sb strings.Builder

	sb.WriteString("# Dream State Analysis\n\n")
	sb.WriteString(fmt.Sprintf("**Hypothetical:** %s\n\n", hypothetical))

	// Show which shards were consulted
	sb.WriteString("**Agents Consulted:** ")
	for i, c := range consultations {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(c.ShardName)
	}
	sb.WriteString("\n\n---\n\n")

	// Collect all unique tools and concerns
	allTools := make(map[string]bool)
	allMissingTools := make(map[string]bool)
	allConcerns := make(map[string]bool)

	// Group by shard type for organized output
	typeOrder := []string{"ephemeral", "persistent", "user", "system"}
	typeLabels := map[string]string{
		"ephemeral":  "Type A - Ephemeral Agents (Generalists)",
		"persistent": "Type B - Persistent Specialists (Domain Experts)",
		"user":       "Type U - User-Defined Specialists",
		"system":     "Type S - System Agents (Policy/Safety)",
	}

	for _, shardType := range typeOrder {
		// Find consultations of this type
		var typeConsultations []DreamConsultation
		for _, c := range consultations {
			if strings.Contains(c.ShardType, shardType) || (shardType == "ephemeral" && c.ShardType == "") {
				typeConsultations = append(typeConsultations, c)
			}
		}

		if len(typeConsultations) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n\n", typeLabels[shardType]))

		for _, c := range typeConsultations {
			if c.Error != nil {
				sb.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(c.ShardName)))
				sb.WriteString(fmt.Sprintf("*Consultation failed: %v*\n\n", c.Error))
				continue
			}

			sb.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(c.ShardName)))
			sb.WriteString(c.Perspective)
			sb.WriteString("\n\n")

			// Aggregate tools
			for _, tool := range c.Tools {
				if strings.Contains(strings.ToLower(tool), "need") || strings.Contains(strings.ToLower(tool), "create") {
					allMissingTools[tool] = true
				} else {
					allTools[tool] = true
				}
			}

			// Aggregate concerns
			for _, concern := range c.Concerns {
				allConcerns[concern] = true
			}
		}

		sb.WriteString("---\n\n")
	}

	// Summary section
	sb.WriteString("## Aggregated Summary\n\n")

	if len(allTools) > 0 {
		sb.WriteString("### Existing Tools Required\n")
		for tool := range allTools {
			sb.WriteString(fmt.Sprintf("- %s\n", tool))
		}
		sb.WriteString("\n")
	}

	if len(allMissingTools) > 0 {
		sb.WriteString("### Tools to Create (Autopoiesis Candidates)\n")
		for tool := range allMissingTools {
			sb.WriteString(fmt.Sprintf("- %s\n", tool))
		}
		sb.WriteString("\n")
	}

	if len(allConcerns) > 0 {
		sb.WriteString("### Risks & Concerns\n")
		for concern := range allConcerns {
			sb.WriteString(fmt.Sprintf("- %s\n", concern))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")
	sb.WriteString("**This is a dry run.** I haven't executed anything.\n\n")

	sb.WriteString("### What would you like to do?\n\n")
	sb.WriteString("- Say **\"do it\"** or **\"execute that\"** -> Run this plan\n")
	sb.WriteString("- Say **\"correct!\"** or **\"perfect\"** -> Learn from this analysis\n")
	sb.WriteString("- Say **\"no, actually...\"** -> Teach me a correction\n")
	sb.WriteString("- Or just ask something else to dismiss\n\n")

	sb.WriteString("**Tip:** Use `Shift+Tab` to change execution mode (Auto/Confirm/Breakpoint)\n")

	return sb.String()
}

// extractToolMentions finds tool/command references in shard output.
func extractToolMentions(text string) []string {
	tools := make([]string, 0)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)
		// Look for tool-related lines
		if strings.Contains(lower, "tool") ||
			strings.Contains(lower, "command") ||
			strings.Contains(lower, "use ") ||
			strings.Contains(lower, "run ") ||
			strings.Contains(lower, "execute") {
			// Extract the meaningful part
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > 0 && len(trimmed) < 200 {
				tools = append(tools, trimmed)
			}
		}
	}

	return tools
}

// extractConcerns finds risk/concern mentions in shard output.
func extractConcerns(text string) []string {
	concerns := make([]string, 0)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)
		// Look for concern-related lines
		if strings.Contains(lower, "risk") ||
			strings.Contains(lower, "concern") ||
			strings.Contains(lower, "careful") ||
			strings.Contains(lower, "warning") ||
			strings.Contains(lower, "could fail") ||
			strings.Contains(lower, "might break") {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > 0 && len(trimmed) < 200 {
				concerns = append(concerns, trimmed)
			}
		}
	}

	return concerns
}

// shouldAutoClarify heuristically decides when to trigger the clarifier shard without a command.
func (m Model) shouldAutoClarify(intent *perception.Intent, input string) bool {
	// Avoid loops on the same input
	if strings.TrimSpace(input) != "" && strings.EqualFold(strings.TrimSpace(input), strings.TrimSpace(m.lastClarifyInput)) {
		return false
	}

	lower := strings.ToLower(input)

	looksLikeCampaign := strings.Contains(lower, "campaign") ||
		strings.Contains(lower, "plan") ||
		strings.Contains(lower, "roadmap") ||
		strings.Contains(lower, "project") ||
		strings.Contains(lower, "initiative") ||
		strings.Contains(lower, "blueprint") ||
		strings.Contains(lower, "feature")

	needsDetails := intent != nil && (intent.Target == "" || intent.Constraint == "" || intent.Verb == "/generate" || intent.Verb == "/scaffold")

	isBuildish := intent != nil && (intent.Category == "/mutation" || intent.Category == "/instruction")

	return isBuildish && (looksLikeCampaign || needsDetails)
}

func (m Model) shouldClarifyIntent(intent *perception.Intent, input string) bool {
	if intent == nil {
		return false
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return false
	}

	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "clarification:") {
		return false
	}

	if strings.EqualFold(trimmed, strings.TrimSpace(m.lastClarifyInput)) {
		return false
	}

	if isConversationalIntent(*intent) {
		return false
	}

	shardType := perception.GetShardTypeForVerb(intent.Verb)
	actionable := shardType != "" || intent.Verb == "/read" || intent.Verb == "/search" || intent.Verb == "/run" || intent.Verb == "/test" || intent.Verb == "/diff" || intent.Verb == "/git" || intent.Verb == "/build"

	if !actionable {
		return false
	}

	if len(intent.Ambiguity) > 0 {
		return true
	}

	if intent.Confidence < 0.45 {
		return true
	}

	target := strings.TrimSpace(intent.Target)
	if target == "" || target == "none" {
		return true
	}

	return false
}

func (m Model) needsWorkspaceScanForDelegation(intent perception.Intent) bool {
	if intent.Category != "/query" && intent.Category != "/mutation" {
		return false
	}
	return perception.GetShardTypeForVerb(intent.Verb) != ""
}

func (m Model) loadWorkspaceFacts(ctx context.Context, intent perception.Intent, warnings *[]string) bool {
	if m.scanner == nil || m.kernel == nil {
		return false
	}
	if intent.Category != "/query" && intent.Category != "/mutation" {
		return false
	}

	res, err := m.scanner.ScanWorkspaceIncremental(ctx, m.workspace, m.localDB, world.IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		if warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("Workspace scan skipped: %v", err))
		}
		return false
	}
	if res == nil || res.Unchanged || len(res.NewFacts) == 0 {
		return true
	}

	if applyErr := world.ApplyIncrementalResult(m.kernel, res); applyErr != nil {
		if warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("Workspace apply skipped: %v", applyErr))
		}
		return true
	}

	if m.virtualStore != nil {
		if err := m.virtualStore.PersistFactsToKnowledge(res.NewFacts, "fact", 5); err != nil && warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("Knowledge persistence warning: %v", err))
		}
		for _, f := range res.NewFacts {
			switch f.Predicate {
			case "dependency_link":
				if len(f.Args) >= 2 {
					a := fmt.Sprintf("%v", f.Args[0])
					b := fmt.Sprintf("%v", f.Args[1])
					rel := "depends_on"
					if len(f.Args) >= 3 {
						rel = "depends_on:" + fmt.Sprintf("%v", f.Args[2])
					}
					_ = m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]interface{}{"source": "scan"})
				}
			case "symbol_graph":
				if len(f.Args) >= 4 {
					sid := fmt.Sprintf("%v", f.Args[0])
					file := fmt.Sprintf("%v", f.Args[3])
					_ = m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]interface{}{"source": "scan"})
				}
			}
		}
	}

	return true
}

// systemExecutionResult holds the result of a system action execution.
type systemExecutionResult struct {
	ActionID   string
	ActionType string
	Target     string
	Success    bool
	Output     string
	Timestamp  int64
}

func (m Model) handleSystemDelegations(ctx context.Context, input string, intent perception.Intent, baseRouting, baseExec int) tea.Msg {
	if m.kernel == nil || m.shardMgr == nil {
		return nil
	}

	delegateFacts, _ := m.kernel.Query("delegate_task")
	execFacts := m.diffFacts("execution_result", baseExec)
	if len(execFacts) == 0 && shouldWaitForSystemResults(intent, len(delegateFacts) > 0) {
		_, execFacts = m.waitForSystemResults(ctx, baseRouting, baseExec, 1200*time.Millisecond)
	}

	executions := parseExecutionResults(execFacts)
	if msg := m.buildResponseFromExecutions(ctx, input, intent, delegateFacts, executions, baseRouting, baseExec); msg != nil {
		return msg
	}

	if len(delegateFacts) == 0 {
		return nil
	}

	return m.executeDelegateTaskFallback(ctx, input, intent, delegateFacts, baseRouting, baseExec)
}

func shouldWaitForSystemResults(intent perception.Intent, hasDelegations bool) bool {
	if hasDelegations {
		return true
	}
	if perception.GetShardTypeForVerb(intent.Verb) != "" {
		return true
	}
	switch intent.Verb {
	case "/read", "/search", "/run", "/test", "/diff", "/git", "/build":
		return true
	default:
		return false
	}
}

func (m Model) buildResponseFromExecutions(ctx context.Context, input string, intent perception.Intent, delegateFacts []core.Fact, executions []systemExecutionResult, baseRouting, baseExec int) tea.Msg {
	if len(executions) == 0 {
		return nil
	}

	sort.Slice(executions, func(i, j int) bool {
		return executions[i].Timestamp > executions[j].Timestamp
	})

	for _, exec := range executions {
		actionType := normalizeActionType(exec.ActionType)
		if actionType == "" {
			continue
		}

		if actionType == "run_tests" {
			surface := m.formatInterpretedResult(ctx, input, "tester", "run_tests", exec.Output, "")
			return assistantMsg{
				Surface: m.appendSystemSummary(surface, m.collectSystemSummary(ctx, baseRouting, baseExec)),
			}
		}

		shardType := actionTypeToShardType(actionType, exec.Target)
		if shardType == "" {
			task := strings.TrimSpace(strings.Join([]string{actionType, exec.Target}, " "))
			if task == "" {
				task = "system_action"
			}
			surface := m.formatInterpretedResult(ctx, input, "system", task, exec.Output, "")
			return assistantMsg{
				Surface: m.appendSystemSummary(surface, m.collectSystemSummary(ctx, baseRouting, baseExec)),
			}
		}

		task := resolveDelegateTask(shardType, delegateFacts, intent, m.workspace, m.lastShardResult)
		if task == "" {
			task = exec.Target
		}

		surface := m.formatDelegationOutput(ctx, input, shardType, task, exec.Output)
		payload := m.buildShardResultPayload(shardType, task, exec.Output, nil)
		if payload != nil && m.kernel != nil && len(payload.Facts) > 0 {
			_ = m.kernel.LoadFacts(payload.Facts)
		}
		return assistantMsg{
			Surface:     m.appendSystemSummary(surface, m.collectSystemSummary(ctx, baseRouting, baseExec)),
			ShardResult: payload,
		}
	}

	return nil
}

func (m Model) executeDelegateTaskFallback(ctx context.Context, input string, intent perception.Intent, delegateFacts []core.Fact, baseRouting, baseExec int) tea.Msg {
	for _, fact := range delegateFacts {
		shardType, taskDesc, pending := parseDelegateFact(fact)
		if !pending || shardType == "" {
			continue
		}

		task := resolveDelegateTask(shardType, delegateFacts, intent, m.workspace, m.lastShardResult)
		if task == "" {
			task = taskDesc
		}
		if task == "" {
			task = "codebase"
		}

		sessionCtx := m.buildSessionContext(ctx)
		result, spawnErr := m.spawnTaskWithContext(ctx, shardType, task, sessionCtx, types.PriorityHigh)
		payload := m.buildShardResultPayload(shardType, task, result, spawnErr)
		if payload != nil && m.kernel != nil && len(payload.Facts) > 0 {
			_ = m.kernel.LoadFacts(payload.Facts)
		}

		if spawnErr != nil {
			return errorMsg(fmt.Errorf("shard delegation failed: %w", spawnErr))
		}

		surface := m.formatDelegationOutput(ctx, input, shardType, task, result)
		return assistantMsg{
			Surface:     m.appendSystemSummary(surface, m.collectSystemSummary(ctx, baseRouting, baseExec)),
			ShardResult: payload,
		}
	}

	return nil
}

func (m Model) buildShardResultPayload(shardType, task, result string, err error) *ShardResultPayload {
	if m.shardMgr == nil {
		return nil
	}

	shardID := fmt.Sprintf("%s-system-%d", shardType, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)
	return &ShardResultPayload{
		ShardType: shardType,
		Task:      task,
		Result:    result,
		Facts:     facts,
	}
}

func (m Model) formatDelegationOutput(ctx context.Context, input, shardType, task, result string) string {
	if shardType == "reviewer" || shardType == "tester" {
		return m.formatInterpretedResult(ctx, input, shardType, task, result, "")
	}

	header := fmt.Sprintf("## %s Result", strings.Title(shardType))
	if shardType == "" {
		header = "## Delegated Result"
	}
	return fmt.Sprintf(`%s
**Agent**: %s
**Task**: %s

### Output
%s`, header, shardType, task, result)
}

func (m Model) buildShardInterpretationPrompt(ctx context.Context, input, shardType, task, result string) (string, string) {
	userPrompt := fmt.Sprintf(`USER REQUEST (ANSWER THIS):
%s

You are translating shard output into a clear, user-facing answer.
Requirements:
- Start with a direct answer in 1-3 sentences.
- If the request asks for the biggest/main issue, identify the single highest-impact issue (or say none found).
- Summarize key evidence from the output without dumping raw logs.
- Provide 3-7 concrete next steps or checks.
- Call out uncertainty if the output is incomplete.

SHARD TYPE: %s
TASK: %s
OUTPUT:
%s
`, input, shardType, task, result)

	if m.jitCompiler != nil {
		semanticQuery := fmt.Sprintf("Translate %s shard output into actionable summary", normalizeShardType(shardType))
		cc := prompt.NewCompilationContext().
			WithOperationalMode("/active").
			WithIntent("/translate", "").
			WithShard("/analysis_translator", "analysis_translator", "Analysis Translator").
			WithTokenBudget(12000, 2000).
			WithSemanticQuery(semanticQuery, 8)

		if res, err := m.jitCompiler.Compile(ctx, cc); err == nil && res != nil && strings.TrimSpace(res.Prompt) != "" {
			return res.Prompt, userPrompt
		}
	}

	fallbackPrompt := fmt.Sprintf(`%s

%s`, campaign.AnalysisLogic, userPrompt)
	return stevenMoorePersona, fallbackPrompt
}

func (m Model) interpretShardOutput(ctx context.Context, input, shardType, task, result string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("LLM client not initialized")
	}

	systemPrompt, userPrompt := m.buildShardInterpretationPrompt(ctx, input, shardType, task, result)
	interpResp, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Extract grounding sources if client supports it (e.g., Gemini with Google Search)
	var groundingSources []string
	if gp, ok := m.client.(types.GroundingProvider); ok {
		groundingSources = gp.GetLastGroundingSources()
	}

	var interpretation string
	processor := articulation.NewResponseProcessor()
	if processed, procErr := processor.Process(interpResp); procErr == nil && strings.TrimSpace(processed.Surface) != "" {
		interpretation = processed.Surface
	} else {
		trimmed := strings.TrimSpace(interpResp)
		if trimmed == "" {
			return "", fmt.Errorf("empty interpretation response")
		}
		interpretation = trimmed
	}

	// Append grounding sources for transparency
	if len(groundingSources) > 0 {
		interpretation += "\n\n**Sources:**\n"
		for _, src := range groundingSources {
			interpretation += fmt.Sprintf("- %s\n", src)
		}
	}

	return interpretation, nil
}

func (m Model) formatInterpretedResult(ctx context.Context, input, shardType, task, result, warning string) string {
	interpretation, err := m.interpretShardOutput(ctx, input, shardType, task, result)
	if err != nil {
		interpretation = fmt.Sprintf("Unable to interpret shard output automatically (%v). Raw output below.", err)
	}

	warning = strings.TrimSpace(warning)
	if warning != "" {
		interpretation = fmt.Sprintf("%s\n\n%s", interpretation, warning)
	}

	return fmt.Sprintf("%s\n\n<details><summary>Raw Output</summary>\n\n%s\n\n</details>", interpretation, result)
}

func parseExecutionResults(facts []core.Fact) []systemExecutionResult {
	results := make([]systemExecutionResult, 0, len(facts))
	for _, fact := range facts {
		if len(fact.Args) < 5 {
			continue
		}
		result := systemExecutionResult{
			ActionID:   fmt.Sprintf("%v", fact.Args[0]),
			ActionType: fmt.Sprintf("%v", fact.Args[1]),
			Target:     fmt.Sprintf("%v", fact.Args[2]),
			Success:    parseBool(fact.Args[3]),
			Output:     fmt.Sprintf("%v", fact.Args[4]),
		}
		if len(fact.Args) >= 6 {
			if ts, ok := fact.Args[5].(int64); ok {
				result.Timestamp = ts
			} else if tsVal, ok := fact.Args[5].(float64); ok {
				result.Timestamp = int64(tsVal)
			}
		}
		results = append(results, result)
	}
	return results
}

func parseBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1"
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func nextActionName(action core.Fact) string {
	if len(action.Args) > 0 {
		value := strings.TrimSpace(fmt.Sprintf("%v", action.Args[0]))
		if value != "" {
			if !strings.HasPrefix(value, "/") {
				value = "/" + value
			}
			return value
		}
	}
	return strings.TrimSpace(action.Predicate)
}

func normalizeActionType(actionType string) string {
	actionType = strings.TrimSpace(strings.TrimPrefix(actionType, "/"))
	if actionType == "" {
		return ""
	}
	return strings.ToLower(actionType)
}

func actionTypeToShardType(actionType, target string) string {
	switch normalizeActionType(actionType) {
	case "delegate_reviewer":
		return "reviewer"
	case "delegate_tester":
		return "tester"
	case "delegate_coder":
		return "coder"
	case "delegate_researcher":
		return "researcher"
	case "delegate_tool_generator":
		return "tool_generator"
	case "delegate":
		return normalizeShardType(target)
	default:
		return ""
	}
}

func parseDelegateFact(fact core.Fact) (string, string, bool) {
	if len(fact.Args) < 3 {
		return "", "", false
	}
	shardType := normalizeShardType(fmt.Sprintf("%v", fact.Args[0]))
	task := fmt.Sprintf("%v", fact.Args[1])
	status := strings.ToLower(fmt.Sprintf("%v", fact.Args[2]))
	pending := status == "/pending" || status == "pending"
	return shardType, strings.TrimSpace(task), pending
}

func normalizeShardType(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "/")
	return strings.ToLower(raw)
}

func (m Model) collectTraceShardTypes() []string {
	candidates := []string{"coder", "tester", "reviewer", "researcher", "planner", "security"}
	if m.lastShardResult != nil && m.lastShardResult.ShardType != "" {
		candidates = append(candidates, m.lastShardResult.ShardType)
	}
	for _, sr := range m.shardResultHistory {
		if sr != nil && sr.ShardType != "" {
			candidates = append(candidates, sr.ShardType)
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, shard := range candidates {
		normalized := normalizeShardType(shard)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		unique = append(unique, normalized)
	}

	return unique
}

func resolveDelegateTask(shardType string, delegateFacts []core.Fact, intent perception.Intent, workspace string, priorResult *ShardResult) string {
	task := ""
	for _, fact := range delegateFacts {
		parsedShard, taskDesc, pending := parseDelegateFact(fact)
		if !pending || parsedShard != shardType {
			continue
		}
		task = taskDesc
		break
	}

	task = strings.TrimSpace(task)
	if task == "" {
		task = strings.TrimSpace(intent.Target)
	}

	if task == "" {
		return ""
	}

	if strings.Contains(task, ":") || strings.Contains(task, " ") {
		return task
	}

	verb := defaultVerbForShard(shardType)
	if verb == "" {
		return task
	}

	return formatShardTaskWithContext(verb, task, intent.Constraint, workspace, priorResult)
}

func defaultVerbForShard(shardType string) string {
	switch shardType {
	case "reviewer":
		return "/review"
	case "tester":
		return "/test"
	case "researcher":
		return "/research"
	case "coder":
		return "/fix"
	default:
		return ""
	}
}

// collectSystemSummary waits briefly for newly derived routing/execution facts and formats them.
func (m Model) collectSystemSummary(ctx context.Context, baseRouting, baseExec int) string {
	if m.kernel == nil {
		return ""
	}
	// Avoid extra polling overhead unless we're displaying the summary or logging in debug mode.
	if !m.showSystemActions && !logging.IsDebugMode() {
		return ""
	}
	routingNew, execNew := m.waitForSystemResults(ctx, baseRouting, baseExec, 1500*time.Millisecond)
	return formatSystemResults(routingNew, execNew)
}

// waitForSystemResults polls for new routing_result/execution_result facts diffed from baselines.
func (m Model) waitForSystemResults(ctx context.Context, baseRouting, baseExec int, timeout time.Duration) ([]core.Fact, []core.Fact) {
	if m.kernel == nil {
		return nil, nil
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case <-timeoutCh:
			return m.diffFacts("routing_result", baseRouting), m.diffFacts("execution_result", baseExec)
		case <-ticker.C:
			routing := m.diffFacts("routing_result", baseRouting)
			exec := m.diffFacts("execution_result", baseExec)
			if len(routing) > 0 || len(exec) > 0 {
				return routing, exec
			}
		}
	}
}

// diffFacts returns facts beyond the baseline index for a predicate.
func (m Model) diffFacts(predicate string, baseline int) []core.Fact {
	facts, err := m.kernel.Query(predicate)
	if err != nil || len(facts) <= baseline {
		return nil
	}
	return facts[baseline:]
}

// formatSystemResults renders system action outputs for the chat surface.
func formatSystemResults(routing, exec []core.Fact) string {
	if len(routing) == 0 && len(exec) == 0 {
		return ""
	}
	const maxLines = 25
	const maxField = 160

	trunc := func(s string) string {
		s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
		if len(s) > maxField {
			return s[:maxField] + "..."
		}
		return s
	}

	lines := make([]string, 0, len(routing)+len(exec))

	// routing_result(ActionID, Result, Details, Timestamp).
	for _, f := range routing {
		if len(f.Args) < 2 {
			continue
		}
		actionID := fmt.Sprintf("%v", f.Args[0])
		result := fmt.Sprintf("%v", f.Args[1])
		details := ""
		if len(f.Args) >= 3 {
			details = trunc(fmt.Sprintf("%v", f.Args[2]))
		}
		if details == "" || details == "()" {
			lines = append(lines, fmt.Sprintf("- %s: %s", actionID, result))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", actionID, result, details))
	}

	// execution_result(ActionID, Type, Target, Success, Output, Timestamp).
	for _, f := range exec {
		if len(f.Args) < 4 {
			continue
		}
		actionID := fmt.Sprintf("%v", f.Args[0])
		actionType := fmt.Sprintf("%v", f.Args[1])
		target := ""
		success := ""
		output := ""
		if len(f.Args) >= 3 {
			target = trunc(fmt.Sprintf("%v", f.Args[2]))
		}
		if len(f.Args) >= 4 {
			success = fmt.Sprintf("%v", f.Args[3])
		}
		if len(f.Args) >= 5 {
			output = trunc(fmt.Sprintf("%v", f.Args[4]))
		}

		line := fmt.Sprintf("- %s: %s", actionID, actionType)
		if strings.TrimSpace(target) != "" {
			line += fmt.Sprintf(" target=%s", target)
		}
		if strings.TrimSpace(success) != "" {
			line += fmt.Sprintf(" success=%s", success)
		}
		if strings.TrimSpace(output) != "" {
			line += fmt.Sprintf(" output=%s", output)
		}
		lines = append(lines, line)
	}

	var sb strings.Builder
	total := len(lines)
	if total > maxLines {
		sb.WriteString(fmt.Sprintf("System actions (showing last %d of %d):\n", maxLines, total))
		lines = lines[total-maxLines:]
	} else {
		sb.WriteString("System actions:\n")
	}
	for _, line := range lines {
		sb.WriteString(line + "\n")
	}
	return strings.TrimSpace(sb.String())
}

// appendSystemSummary appends system action summaries to a response, if present.
func (m Model) appendSystemSummary(response, summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return response
	}
	// Always log when debug mode is enabled; keep chat surface clean by default.
	if logging.IsDebugMode() {
		logging.SessionDebug("System actions summary:\n%s", summary)
	}
	if !m.showSystemActions {
		return response
	}
	if strings.HasSuffix(response, "\n") {
		return response + summary
	}
	return response + "\n\n" + summary
}

// executeMultiStepTask runs multiple task steps in sequence
func (m Model) executeMultiStepTask(ctx context.Context, intent perception.Intent, rawInput string, steps []TaskStep) tea.Cmd {
	return func() tea.Msg {
		var results []string
		var stepResults = make(map[int]string) // Store results for dependency checking

		results = append(results, fmt.Sprintf("## Multi-Step Task Execution\n\n**Original Request**: %s\n**Steps**: %d\n", intent.Response, len(steps)))

		for i, step := range steps {
			m.ReportStatus(fmt.Sprintf("Step %d/%d: %s...", i+1, len(steps), step.Verb))
			// Check dependencies
			canExecute := true
			for _, depIdx := range step.DependsOn {
				if _, exists := stepResults[depIdx]; !exists {
					canExecute = false
					break
				}
			}

			if !canExecute {
				results = append(results, fmt.Sprintf("\n### Step %d: SKIPPED (dependencies not met)\n", i+1))
				continue
			}

			// Execute step
			results = append(results, fmt.Sprintf("\n### Step %d: %s\n**Target**: %s\n**Agent**: %s\n",
				i+1, strings.TrimPrefix(step.Verb, "/"), step.Target, step.ShardType))

			if step.ShardType != "" {
				result, err := m.spawnTask(ctx, step.ShardType, step.Task)

				// CRITICAL FIX: Inject multi-step shard results as facts
				shardID := fmt.Sprintf("%s-step%d-%d", step.ShardType, i, time.Now().UnixNano())
				facts := m.shardMgr.ResultToFacts(shardID, step.ShardType, step.Task, result, err)
				if m.kernel != nil && len(facts) > 0 {
					_ = m.kernel.LoadFacts(facts)
				}

				if err != nil {
					results = append(results, fmt.Sprintf("**Status**: Failed\n**Error**: %v\n", err))
					// Don't continue to dependent steps if this fails
					continue
				}

				// Store result for dependencies
				stepResults[i] = result

				formattedResult := result
				if normalizeShardType(step.ShardType) == "reviewer" || normalizeShardType(step.ShardType) == "tester" {
					formattedResult = m.formatInterpretedResult(ctx, rawInput, step.ShardType, step.Task, result, "")
				}

				results = append(results, fmt.Sprintf("**Status**: Complete\n```\n%s\n```\n", formattedResult))
			} else {
				results = append(results, "**Status**: No shard handler\n")
			}
		}

		// Summary
		successCount := len(stepResults)
		results = append(results, fmt.Sprintf("\n---\n**Summary**: %d/%d steps completed successfully\n", successCount, len(steps)))

		return responseMsg(strings.Join(results, ""))
	}
}
