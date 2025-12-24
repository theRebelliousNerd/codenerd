// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains shard spawning and task delegation helpers.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/shards"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// Functions for formatting tasks, spawning shards, and handling delegation
// from natural language to specialized agents.

// formatShardTaskWithContext formats the task with prior shard context (blackboard pattern).
// This enables cross-shard communication: reviewer findings flow to coder, test results to debugger, etc.
func formatShardTaskWithContext(verb, target, constraint, workspace string, priorResult *ShardResult) string {
	baseTask := formatShardTask(verb, target, constraint, workspace)

	// No prior context - return base task
	if priorResult == nil {
		return baseTask
	}

	// Inject context based on verb and prior shard type
	switch verb {
	case "/fix":
		// If fixing after a review, include the specific findings
		if priorResult.ShardType == "reviewer" && len(priorResult.Findings) > 0 {
			findingsStr := formatFindingsForTask(priorResult.Findings, target)
			if findingsStr != "" {
				// Determine target file from prior result if current target is generic
				actualTarget := target
				if actualTarget == "codebase" || actualTarget == "none" || actualTarget == "" {
					// Extract file from findings or task
					if file := extractFileFromFindings(priorResult.Findings); file != "" {
						actualTarget = file
					}
				}
				return fmt.Sprintf("fix file:%s findings:[%s]", actualTarget, findingsStr)
			}
		}
		// If fixing after a test failure, include test errors
		if priorResult.ShardType == "tester" && priorResult.RawOutput != "" {
			return fmt.Sprintf("fix file:%s test_errors:[%s]", target, truncateForTask(priorResult.RawOutput, 500))
		}

	case "/refactor":
		// If refactoring after a review, include improvement suggestions
		if priorResult.ShardType == "reviewer" && len(priorResult.Findings) > 0 {
			suggestions := filterFindingsBySeverity(priorResult.Findings, []string{"info", "warning"})
			if len(suggestions) > 0 {
				return fmt.Sprintf("refactor file:%s suggestions:[%s]", target, formatFindingsForTask(suggestions, target))
			}
		}

	case "/test":
		// If testing after a fix, include what was fixed
		if priorResult.ShardType == "coder" {
			return fmt.Sprintf("write_tests for %s after_fix context:[%s]", target, truncateForTask(priorResult.RawOutput, 300))
		}

	case "/debug":
		// Include prior test or error context
		if priorResult.ShardType == "tester" || priorResult.ShardType == "reviewer" {
			return fmt.Sprintf("debug %s context:[%s]", target, truncateForTask(priorResult.RawOutput, 500))
		}
	}

	return baseTask
}

// formatFindingsForTask converts findings to a compact string for task injection
func formatFindingsForTask(findings []map[string]any, targetFile string) string {
	var parts []string
	for _, f := range findings {
		file, _ := f["file"].(string)
		// Filter to target file if specified
		if targetFile != "" && targetFile != "codebase" && file != "" && !strings.HasSuffix(file, targetFile) {
			continue
		}
		line, _ := f["line"].(float64)
		msg, _ := f["message"].(string)
		sev, _ := f["severity"].(string)

		if msg != "" {
			if line > 0 {
				parts = append(parts, fmt.Sprintf("%s@L%d:%s", sev, int(line), truncateForTask(msg, 100)))
			} else {
				parts = append(parts, fmt.Sprintf("%s:%s", sev, truncateForTask(msg, 100)))
			}
		}
	}
	return strings.Join(parts, "; ")
}

// extractFileFromFindings extracts the primary file from findings
func extractFileFromFindings(findings []map[string]any) string {
	fileCount := make(map[string]int)
	for _, f := range findings {
		if file, ok := f["file"].(string); ok && file != "" {
			fileCount[file]++
		}
	}
	// Return most common file
	maxFile := ""
	maxCount := 0
	for file, count := range fileCount {
		if count > maxCount {
			maxCount = count
			maxFile = file
		}
	}
	return maxFile
}

// filterFindingsBySeverity filters findings to only include specified severities
func filterFindingsBySeverity(findings []map[string]any, severities []string) []map[string]any {
	var result []map[string]any
	sevSet := make(map[string]bool)
	for _, s := range severities {
		sevSet[strings.ToLower(s)] = true
	}
	for _, f := range findings {
		if sev, ok := f["severity"].(string); ok && sevSet[strings.ToLower(sev)] {
			result = append(result, f)
		}
	}
	return result
}

// truncateForTask truncates a string for embedding in task strings
func truncateForTask(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func formatShardTask(verb, target, constraint, workspace string) string {
	// Normalize target
	if target == "" || target == "none" {
		target = "codebase"
	}

	// Handle file paths - make them relative to workspace if needed
	if strings.HasPrefix(target, workspace) {
		if rel, err := filepath.Rel(workspace, target); err == nil {
			target = rel
		}
	}

	// Discover files if target is broad (codebase, all files, etc.)
	var fileList string
	if target == "codebase" || strings.Contains(strings.ToLower(target), "all") || strings.Contains(target, "*") {
		files := discoverFiles(workspace, constraint)
		if len(files) > 0 {
			fileList = strings.Join(files, ",")
		}
	}

	switch verb {
	case "/review":
		if fileList != "" {
			return fmt.Sprintf("review files:%s", fileList)
		}
		if target == "codebase" {
			return "review all"
		}
		return fmt.Sprintf("review file:%s", target)

	case "/security":
		if fileList != "" {
			return fmt.Sprintf("security_scan files:%s", fileList)
		}
		if target == "codebase" {
			return "security_scan all"
		}
		return fmt.Sprintf("security_scan file:%s", target)

	case "/analyze":
		if fileList != "" {
			return fmt.Sprintf("complexity files:%s", fileList)
		}
		if target == "codebase" {
			return "complexity all"
		}
		return fmt.Sprintf("complexity file:%s", target)

	case "/fix":
		return fmt.Sprintf("fix issue in %s", target)

	case "/refactor":
		return fmt.Sprintf("refactor %s", target)

	case "/create":
		return fmt.Sprintf("create %s", target)

	case "/test":
		if strings.Contains(target, "run") || target == "codebase" {
			return "run_tests"
		}
		return fmt.Sprintf("write_tests for %s", target)

	case "/debug":
		return fmt.Sprintf("debug %s", target)

	case "/research":
		return fmt.Sprintf("research %s", target)

	case "/explore":
		return fmt.Sprintf("explore %s", target)

	case "/document":
		return fmt.Sprintf("document %s", target)

	case "/diff":
		return fmt.Sprintf("review diff:%s", target)

	default:
		// Generic task format
		if constraint != "none" && constraint != "" {
			return fmt.Sprintf("%s %s with constraint: %s", verb, target, constraint)
		}
		return fmt.Sprintf("%s %s", verb, target)
	}
}

// formatDelegatedResponse creates a user-friendly response from shard execution.
func formatDelegatedResponse(intent perception.Intent, shardType, task, result string) string {
	// Build header based on verb
	var header string
	switch intent.Verb {
	case "/review":
		header = "## Code Review Results"
	case "/security":
		header = "## Security Analysis Results"
	case "/analyze":
		header = "## Code Analysis Results"
	case "/fix":
		header = "## Fix Applied"
	case "/refactor":
		header = "## Refactoring Complete"
	case "/test":
		header = "## Test Results"
	case "/debug":
		header = "## Debug Analysis"
	case "/research":
		header = "## Research Findings"
	default:
		header = fmt.Sprintf("## %s Results", strings.Title(strings.TrimPrefix(intent.Verb, "/")))
	}

	// Include the LLM's surface response if meaningful
	surfaceNote := ""
	if intent.Response != "" && len(intent.Response) < 500 {
		surfaceNote = fmt.Sprintf("\n\n> %s\n", intent.Response)
	}

	return fmt.Sprintf(`%s
%s
**Target**: %s
**Agent**: %s
**Task**: %s

### Output
%s`, header, surfaceNote, intent.Target, shardType, task, result)
}

// spawnShard spawns a shard agent for a task
func (m Model) spawnShard(shardType, task string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().ShardExecutionTimeout)
		defer cancel()

		m.ReportStatus(fmt.Sprintf("Spawning %s...", shardType))

		result, err := m.shardMgr.Spawn(ctx, shardType, task)

		// Generate a shard ID for fact tracking
		shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())

		// CRITICAL FIX: Convert shard result to facts and inject into kernel
		// This is the missing bridge that enables cross-turn context propagation
		facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)
		if m.kernel != nil && len(facts) > 0 {
			if loadErr := m.kernel.LoadFacts(facts); loadErr != nil {
				// Log but don't fail - the response should still be shown
				fmt.Printf("[ShardFacts] Warning: failed to inject facts: %v\n", loadErr)
			}
		}

		if err != nil {
			return errorMsg(fmt.Errorf("shard spawn failed: %w", err))
		}

		response := fmt.Sprintf(`## Shard Execution Complete

**Agent**: %s
**Task**: %s

### Result
%s`, shardType, task, result)

		m.ReportStatus(fmt.Sprintf("%s complete", shardType))
		return assistantMsg{
			Surface: response,
			ShardResult: &ShardResultPayload{
				ShardType: shardType,
				Task:      task,
				Result:    result,
				Facts:     facts,
			},
		}
	}
}

// spawnShardWithSpecialists spawns a shard with specialist support based on execution mode.
// Execution modes:
//   - ModeParallel: All shards execute in parallel (for /review, /security)
//   - ModeAdvisory: Specialists advise, then generic shard executes (for /create, /debug)
//   - ModeAdvisoryWithCritique: Advise → Execute → Critique (for /fix, /refactor)
func (m Model) spawnShardWithSpecialists(verb, shardType, task, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().ShardExecutionTimeout)
		defer cancel()

		startTime := time.Now()

		// 1. Resolve target files for specialist matching
		files := m.resolveReviewTarget(target)
		if len(files) == 0 {
			fullPath := target
			if !filepath.IsAbs(target) {
				fullPath = filepath.Join(m.workspace, target)
			}
			if _, err := os.Stat(fullPath); err == nil {
				files = []string{fullPath}
			}
		}

		// 2. Load agent registry and match specialists
		registry := m.loadAgentRegistryForMatching()
		specialists := shards.MatchSpecialistsForTask(ctx, verb, files, registry)

		// 3. No specialists? Fall back to simple spawn
		if len(specialists) == 0 {
			return m.spawnSimpleShard(ctx, shardType, task, startTime)
		}

		// 4. Route based on execution mode
		mode := shards.GetExecutionMode(verb)
		switch mode {
		case shards.ModeAdvisory:
			return m.executeAdvisoryMode(ctx, verb, shardType, task, target, files, specialists, startTime)
		case shards.ModeAdvisoryWithCritique:
			return m.executeAdvisoryWithCritiqueMode(ctx, verb, shardType, task, target, files, specialists, startTime)
		default: // ModeParallel
			return m.executeParallelMode(ctx, verb, shardType, task, target, specialists, startTime)
		}
	}
}

// spawnSimpleShard handles the case where no specialists are matched
func (m Model) spawnSimpleShard(ctx context.Context, shardType, task string, startTime time.Time) tea.Msg {
	m.ReportStatus(fmt.Sprintf("Spawning %s...", shardType))
	result, err := m.shardMgr.Spawn(ctx, shardType, task)
	shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)
	if m.kernel != nil && len(facts) > 0 {
		_ = m.kernel.LoadFacts(facts)
	}
	if err != nil {
		return errorMsg(fmt.Errorf("shard spawn failed: %w", err))
	}
	response := fmt.Sprintf("## Shard Execution Complete\n\n**Agent**: %s\n**Task**: %s\n**Duration**: %s\n\n### Result\n%s",
		shardType, task, time.Since(startTime).Round(time.Second), result)
	return assistantMsg{
		Surface: response,
		ShardResult: &ShardResultPayload{
			ShardType: shardType,
			Task:      task,
			Result:    result,
			Facts:     facts,
		},
	}
}

// executeParallelMode runs all shards in parallel (for /review, /security, /test)
func (m Model) executeParallelMode(ctx context.Context, verb, shardType, task, target string, specialists []shards.SpecialistMatch, startTime time.Time) tea.Msg {
	m.ReportStatus(fmt.Sprintf("Spawning %s + %d specialists in parallel...", shardType, len(specialists)))

	resultsChan := make(chan spawnResult, len(specialists)+1)
	var wg sync.WaitGroup

	// Spawn generic shard
	if shards.ShouldIncludeGenericShard(verb) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := m.shardMgr.Spawn(ctx, shardType, task)
			resultsChan <- spawnResult{Name: shardType, Result: result, Err: err}
		}()
	}

	// Spawn specialists in parallel
	for _, spec := range specialists {
		wg.Add(1)
		go func(s shards.SpecialistMatch) {
			defer wg.Done()
			specTask := fmt.Sprintf("%s files:%s context:[matched for %s]",
				strings.TrimPrefix(verb, "/"), strings.Join(s.Files, ","), s.Reason)
			result, err := m.shardMgr.Spawn(ctx, s.AgentName, specTask)
			resultsChan <- spawnResult{Name: s.AgentName, Result: result, Err: err}
		}(spec)
	}

	go func() { wg.Wait(); close(resultsChan) }()

	var results []spawnResult
	for r := range resultsChan {
		results = append(results, r)
	}

	return m.formatParallelResults(verb, shardType, task, target, results, startTime)
}

// executeAdvisoryMode: Phase 1 (advice) → Phase 2 (execute with advice)
func (m Model) executeAdvisoryMode(ctx context.Context, verb, shardType, task, target string, files []string, specialists []shards.SpecialistMatch, startTime time.Time) tea.Msg {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s with Specialist Advisory\n\n", strings.Title(strings.TrimPrefix(verb, "/"))))
	sb.WriteString(fmt.Sprintf("**Target**: %s\n\n", target))

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 1: Gather specialist advice in parallel
	// ═══════════════════════════════════════════════════════════════════════════
	m.ReportStatus(fmt.Sprintf("Phase 1: Gathering advice from %d specialists...", len(specialists)))
	sb.WriteString("### Phase 1: Specialist Advisory\n\n")

	adviceResults := m.gatherSpecialistAdvice(ctx, verb, files, specialists)
	var combinedAdvice strings.Builder
	for _, adv := range adviceResults {
		if adv.Err != nil {
			sb.WriteString(fmt.Sprintf("**%s**: ⚠️ Failed to provide advice\n\n", adv.Name))
		} else {
			sb.WriteString(fmt.Sprintf("**%s** (%s):\n%s\n\n", adv.Name, adv.Reason, adv.Result))
			combinedAdvice.WriteString(fmt.Sprintf("[%s advice]: %s\n", adv.Name, adv.Result))
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 2: Execute with specialist advice as context
	// ═══════════════════════════════════════════════════════════════════════════
	m.ReportStatus(fmt.Sprintf("Phase 2: Executing %s with specialist context...", shardType))
	sb.WriteString("---\n\n### Phase 2: Execution\n\n")

	// Inject advice into the task
	enhancedTask := task
	if combinedAdvice.Len() > 0 {
		enhancedTask = fmt.Sprintf("%s\n\n[SPECIALIST ADVICE - Consider these domain-specific recommendations]:\n%s",
			task, combinedAdvice.String())
	}

	result, err := m.shardMgr.Spawn(ctx, shardType, enhancedTask)
	shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)

	if err != nil {
		sb.WriteString(fmt.Sprintf("**%s**: ❌ Failed: %v\n\n", shardType, err))
	} else {
		sb.WriteString(fmt.Sprintf("**%s** output:\n\n%s\n\n", shardType, result))
	}

	// Inject facts
	if m.kernel != nil && len(facts) > 0 {
		_ = m.kernel.LoadFacts(facts)
	}

	sb.WriteString(fmt.Sprintf("---\n\n**Duration**: %s\n", time.Since(startTime).Round(time.Second)))
	sb.WriteString(fmt.Sprintf("**Advisors**: %s\n", formatAdvisorNames(adviceResults)))

	m.ReportStatus(fmt.Sprintf("%s complete", shardType))
	return assistantMsg{
		Surface: sb.String(),
		ShardResult: &ShardResultPayload{
			ShardType: shardType,
			Task:      task,
			Result:    result,
			Facts:     facts,
		},
	}
}

// executeAdvisoryWithCritiqueMode: Phase 1 (advice) → Phase 2 (execute) → Phase 3 (critique)
func (m Model) executeAdvisoryWithCritiqueMode(ctx context.Context, verb, shardType, task, target string, files []string, specialists []shards.SpecialistMatch, startTime time.Time) tea.Msg {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s with Specialist Advisory & Critique\n\n", strings.Title(strings.TrimPrefix(verb, "/"))))
	sb.WriteString(fmt.Sprintf("**Target**: %s\n\n", target))

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 1: Gather specialist advice in parallel
	// ═══════════════════════════════════════════════════════════════════════════
	m.ReportStatus(fmt.Sprintf("Phase 1: Gathering advice from %d specialists...", len(specialists)))
	sb.WriteString("### Phase 1: Specialist Advisory\n\n")

	adviceResults := m.gatherSpecialistAdvice(ctx, verb, files, specialists)
	var combinedAdvice strings.Builder
	for _, adv := range adviceResults {
		if adv.Err != nil {
			sb.WriteString(fmt.Sprintf("**%s**: ⚠️ Failed to provide advice\n\n", adv.Name))
		} else {
			sb.WriteString(fmt.Sprintf("**%s** (%s):\n%s\n\n", adv.Name, adv.Reason, adv.Result))
			combinedAdvice.WriteString(fmt.Sprintf("[%s advice]: %s\n", adv.Name, adv.Result))
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 2: Execute with specialist advice as context
	// ═══════════════════════════════════════════════════════════════════════════
	m.ReportStatus(fmt.Sprintf("Phase 2: Executing %s with specialist context...", shardType))
	sb.WriteString("---\n\n### Phase 2: Execution\n\n")

	enhancedTask := task
	if combinedAdvice.Len() > 0 {
		enhancedTask = fmt.Sprintf("%s\n\n[SPECIALIST ADVICE - Consider these domain-specific recommendations]:\n%s",
			task, combinedAdvice.String())
	}

	result, err := m.shardMgr.Spawn(ctx, shardType, enhancedTask)
	shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)

	if err != nil {
		sb.WriteString(fmt.Sprintf("**%s**: ❌ Failed: %v\n\n", shardType, err))
		// Skip critique if execution failed
		sb.WriteString("---\n\n### Phase 3: Critique\n\n⚠️ Skipped due to execution failure.\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("**%s** output:\n\n%s\n\n", shardType, result))

		// ═══════════════════════════════════════════════════════════════════════════
		// PHASE 3: Specialist critique of the result
		// ═══════════════════════════════════════════════════════════════════════════
		m.ReportStatus(fmt.Sprintf("Phase 3: Gathering specialist critiques..."))
		sb.WriteString("---\n\n### Phase 3: Specialist Critique\n\n")

		critiqueResults := m.gatherSpecialistCritique(ctx, verb, files, specialists, result)
		for _, crit := range critiqueResults {
			if crit.Err != nil {
				sb.WriteString(fmt.Sprintf("**%s**: ⚠️ Failed to critique\n\n", crit.Name))
			} else {
				sb.WriteString(fmt.Sprintf("**%s**:\n%s\n\n", crit.Name, crit.Result))
			}
		}
	}

	// Inject facts
	if m.kernel != nil && len(facts) > 0 {
		_ = m.kernel.LoadFacts(facts)
	}

	sb.WriteString(fmt.Sprintf("---\n\n**Duration**: %s\n", time.Since(startTime).Round(time.Second)))
	sb.WriteString(fmt.Sprintf("**Advisors**: %s\n", formatAdvisorNames(adviceResults)))

	m.ReportStatus(fmt.Sprintf("%s complete", shardType))
	return assistantMsg{
		Surface: sb.String(),
		ShardResult: &ShardResultPayload{
			ShardType: shardType,
			Task:      task,
			Result:    result,
			Facts:     facts,
		},
	}
}

// adviceResult holds the result from a specialist advice/critique query
type adviceResult struct {
	Name   string
	Reason string
	Result string
	Err    error
}

// gatherSpecialistAdvice queries specialists for advice in parallel
func (m Model) gatherSpecialistAdvice(ctx context.Context, verb string, files []string, specialists []shards.SpecialistMatch) []adviceResult {
	resultsChan := make(chan adviceResult, len(specialists))
	var wg sync.WaitGroup

	for _, spec := range specialists {
		wg.Add(1)
		go func(s shards.SpecialistMatch) {
			defer wg.Done()
			// Advisory task prompt
			adviceTask := fmt.Sprintf(`ADVISORY REQUEST: Provide domain-specific advice for a %s operation.

Target files: %s
Your expertise: %s

Please provide:
1. Key considerations specific to your domain
2. Common pitfalls to avoid
3. Best practices to follow
4. Any specific patterns or approaches to use

Keep your advice concise and actionable. Do NOT make changes yourself - just advise.`,
				strings.TrimPrefix(verb, "/"),
				strings.Join(s.Files, ", "),
				s.Reason)

			result, err := m.shardMgr.Spawn(ctx, s.AgentName, adviceTask)
			resultsChan <- adviceResult{Name: s.AgentName, Reason: s.Reason, Result: result, Err: err}
		}(spec)
	}

	go func() { wg.Wait(); close(resultsChan) }()

	var results []adviceResult
	for r := range resultsChan {
		results = append(results, r)
	}
	return results
}

// gatherSpecialistCritique queries specialists to critique the execution result
func (m Model) gatherSpecialistCritique(ctx context.Context, verb string, files []string, specialists []shards.SpecialistMatch, executionResult string) []adviceResult {
	resultsChan := make(chan adviceResult, len(specialists))
	var wg sync.WaitGroup

	// Truncate execution result if too long
	truncatedResult := executionResult
	if len(truncatedResult) > 3000 {
		truncatedResult = truncatedResult[:3000] + "\n... [truncated]"
	}

	for _, spec := range specialists {
		wg.Add(1)
		go func(s shards.SpecialistMatch) {
			defer wg.Done()
			// Critique task prompt
			critiqueTask := fmt.Sprintf(`CRITIQUE REQUEST: Review the following %s result from your domain expertise perspective.

Target files: %s
Your expertise: %s

=== EXECUTION RESULT ===
%s
=== END RESULT ===

Please provide:
1. ✅ What was done well
2. ⚠️ Potential issues or concerns from your domain perspective
3. 💡 Suggestions for improvement (if any)

Be concise. Focus on domain-specific insights others might miss.`,
				strings.TrimPrefix(verb, "/"),
				strings.Join(s.Files, ", "),
				s.Reason,
				truncatedResult)

			result, err := m.shardMgr.Spawn(ctx, s.AgentName, critiqueTask)
			resultsChan <- adviceResult{Name: s.AgentName, Reason: s.Reason, Result: result, Err: err}
		}(spec)
	}

	go func() { wg.Wait(); close(resultsChan) }()

	var results []adviceResult
	for r := range resultsChan {
		results = append(results, r)
	}
	return results
}

// spawnResult holds the result from a parallel shard spawn
type spawnResult struct {
	Name   string
	Result string
	Err    error
}

// formatParallelResults formats the output for parallel execution mode
func (m Model) formatParallelResults(verb, shardType, task, target string, results []spawnResult, startTime time.Time) tea.Msg {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Multi-Specialist %s Complete\n\n", strings.Title(strings.TrimPrefix(verb, "/"))))
	sb.WriteString(fmt.Sprintf("**Target**: %s\n", target))
	sb.WriteString(fmt.Sprintf("**Duration**: %s\n\n", time.Since(startTime).Round(time.Second)))

	participants := make([]string, 0, len(results))
	var combinedResult strings.Builder
	allFacts := make([]core.Fact, 0)

	for _, r := range results {
		participants = append(participants, r.Name)
		if r.Err != nil {
			sb.WriteString(fmt.Sprintf("### %s (failed)\n\nError: %v\n\n", r.Name, r.Err))
		} else {
			sb.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", r.Name, r.Result))
			combinedResult.WriteString(r.Result)
			combinedResult.WriteString("\n\n")

			shardID := fmt.Sprintf("%s-%d", r.Name, time.Now().UnixNano())
			facts := m.shardMgr.ResultToFacts(shardID, r.Name, task, r.Result, r.Err)
			allFacts = append(allFacts, facts...)
		}
	}

	sb.WriteString(fmt.Sprintf("**Participants**: %s\n", strings.Join(participants, ", ")))

	if m.kernel != nil && len(allFacts) > 0 {
		_ = m.kernel.LoadFacts(allFacts)
	}

	m.ReportStatus(fmt.Sprintf("%s + specialists complete", shardType))
	return assistantMsg{
		Surface: sb.String(),
		ShardResult: &ShardResultPayload{
			ShardType: shardType,
			Task:      task,
			Result:    combinedResult.String(),
			Facts:     allFacts,
		},
	}
}

// formatAdvisorNames extracts advisor names for display
func formatAdvisorNames(results []adviceResult) string {
	names := make([]string, 0, len(results))
	for _, r := range results {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}

// loadAgentRegistryForMatching loads the agent registry for specialist matching.
// This is a lightweight version that returns the shards.AgentRegistry type.
func (m Model) loadAgentRegistryForMatching() *shards.AgentRegistry {
	registryPath := filepath.Join(m.workspace, ".nerd", "agents.json")

	data, err := os.ReadFile(registryPath)
	if err != nil {
		return nil
	}

	var registry shards.AgentRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil
	}

	return &registry
}

// createDirIfNotExists creates a directory if it doesn't exist
func createDirIfNotExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// ProjectTypeInfo holds detected project characteristics
type ProjectTypeInfo struct {
	Language     string
	Framework    string
	Architecture string
}

// detectProjectType analyzes the workspace to determine project type
func detectProjectType(workspace string) ProjectTypeInfo {
	// Get UI styles for consistent formatting
	styles := getUIStyles()
	_ = styles // Ensure styles are available for future enhancements

	pt := ProjectTypeInfo{
		Language:     "unknown",
		Framework:    "unknown",
		Architecture: "unknown",
	}

	// Check for language markers
	markers := map[string]struct {
		lang  string
		build string
	}{
		"go.mod":           {"go", "go"},
		"Cargo.toml":       {"rust", "cargo"},
		"package.json":     {"javascript", "npm"},
		"requirements.txt": {"python", "pip"},
		"pom.xml":          {"java", "maven"},
	}

	for file, info := range markers {
		if _, err := os.Stat(workspace + "/" + file); err == nil {
			pt.Language = info.lang
			break
		}
	}

	// Detect architecture based on directory structure
	dirs := []string{"cmd", "internal", "pkg", "api", "services"}
	foundDirs := 0
	for _, dir := range dirs {
		if info, err := os.Stat(workspace + "/" + dir); err == nil && info.IsDir() {
			foundDirs++
		}
	}

	if foundDirs >= 3 {
		pt.Architecture = "clean_architecture"
	} else if _, err := os.Stat(workspace + "/docker-compose.yml"); err == nil {
		pt.Architecture = "microservices"
	} else {
		pt.Architecture = "monolith"
	}

	return pt
}

func getUIStyles() ui.Styles {
	return ui.DefaultStyles()
}

// =============================================================================
// MULTI-STEP TASK HANDLING
// =============================================================================

// TaskStep represents a single step in a multi-step task
type TaskStep struct {
	Verb      string
	Target    string
	ShardType string
	Task      string
	DependsOn []int // Indices of steps that must complete first
}

// detectMultiStepTask checks if input requires multiple steps
func detectMultiStepTask(input string, intent perception.Intent) bool {
	lower := strings.ToLower(input)

	// Multi-step indicators
	multiStepKeywords := []string{
		"and then", "after that", "next", "then",
		"first", "second", "third", "finally",
		"step 1", "step 2", "1.", "2.", "3.",
		"also", "additionally", "furthermore",
	}

	for _, keyword := range multiStepKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	// Check for multiple verbs in the input
	verbCount := 0
	for _, entry := range perception.VerbCorpus {
		for _, synonym := range entry.Synonyms {
			if strings.Contains(lower, synonym) {
				verbCount++
				if verbCount >= 2 {
					return true
				}
			}
		}
	}

	// Check for compound tasks (review + test, fix + test, etc.)
	compoundPatterns := []string{
		"review.*test", "fix.*test", "refactor.*test",
		"create.*test", "implement.*test",
	}

	for _, pattern := range compoundPatterns {
		if matched, _ := regexp.MatchString(pattern, lower); matched {
			return true
		}
	}

	return false
}

// decomposeTask breaks a complex task into discrete steps using the encyclopedic corpus.
// This function uses the multi-step pattern corpus for comprehensive decomposition.
func decomposeTask(input string, intent perception.Intent, workspace string) []TaskStep {
	// Try to match against the encyclopedic multi-step corpus first
	pattern, captures := MatchMultiStepPattern(input)

	if pattern != nil {
		// Use the corpus-based decomposition strategy
		steps := DecomposeWithStrategy(input, captures, pattern, workspace)
		if len(steps) > 1 {
			return steps
		}
		// If decomposition returned only 1 step, fall through to legacy handling
	}

	// Legacy fallback patterns for backwards compatibility
	var steps []TaskStep
	lower := strings.ToLower(input)

	// Pattern 1: "fix X and test it" or "create X and test"
	if strings.Contains(lower, "test") && (intent.Verb == "/fix" || intent.Verb == "/create" || intent.Verb == "/refactor") {
		// Step 1: Primary action
		step1 := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step1.Task = formatShardTask(step1.Verb, step1.Target, intent.Constraint, workspace)
		steps = append(steps, step1)

		// Step 2: Testing
		step2 := TaskStep{
			Verb:      "/test",
			Target:    intent.Target,
			ShardType: "tester",
			DependsOn: []int{0}, // Depends on step 1
		}
		step2.Task = formatShardTask(step2.Verb, step2.Target, "none", workspace)
		steps = append(steps, step2)

		return steps
	}

	// Pattern 2: "review codebase" or "review all files" - already handled by multi-file discovery
	// Single step with multiple files
	if intent.Verb == "/review" || intent.Verb == "/security" || intent.Verb == "/analyze" {
		step := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step.Task = formatShardTask(step.Verb, step.Target, intent.Constraint, workspace)
		steps = append(steps, step)
		return steps
	}

	// Default: single step
	if len(steps) == 0 {
		step := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step.Task = formatShardTask(step.Verb, step.Target, intent.Constraint, workspace)
		steps = append(steps, step)
	}

	return steps
}

// discoverFiles finds files in the workspace based on constraint filters
func discoverFiles(workspace, constraint string) []string {
	var files []string

	// Determine file patterns based on constraint
	var extensions []string
	constraintLower := strings.ToLower(constraint)

	switch {
	case strings.Contains(constraintLower, "go"):
		extensions = []string{".go"}
	case strings.Contains(constraintLower, "python") || strings.Contains(constraintLower, "py"):
		extensions = []string{".py"}
	case strings.Contains(constraintLower, "javascript") || strings.Contains(constraintLower, "js"):
		extensions = []string{".js", ".jsx", ".ts", ".tsx"}
	case strings.Contains(constraintLower, "rust"):
		extensions = []string{".rs"}
	case strings.Contains(constraintLower, "java"):
		extensions = []string{".java"}
	default:
		// Default: all common code file extensions
		extensions = []string{".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs", ".java", ".c", ".cpp", ".h"}
	}

	// Walk workspace and collect matching files
	filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip hidden directories and files
		if strings.Contains(path, "/.") || strings.Contains(path, "\\.") {
			return nil
		}

		// Skip vendor, node_modules, etc.
		skipDirs := []string{"vendor", "node_modules", ".git", ".nerd", "dist", "build"}
		for _, skip := range skipDirs {
			if strings.Contains(path, string(filepath.Separator)+skip+string(filepath.Separator)) {
				return nil
			}
		}

		// Check if file matches extension filter
		ext := filepath.Ext(path)
		for _, allowedExt := range extensions {
			if ext == allowedExt {
				// Convert to relative path
				if relPath, err := filepath.Rel(workspace, path); err == nil {
					files = append(files, relPath)
				}
				break
			}
		}

		return nil
	})

	// Limit to 50 files for safety (avoid overwhelming the shard)
	if len(files) > 50 {
		files = files[:50]
	}

	return files
}
