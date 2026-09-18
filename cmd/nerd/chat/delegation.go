// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains shard spawning and task delegation helpers.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	prompt_evolution "codenerd/internal/autopoiesis/prompt_evolution"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	promptpkg "codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/shards"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// TASK EXECUTION HELPERS - Migration from ShardManager to TaskExecutor
// =============================================================================

// spawnTask is the unified entry point for task execution in the chat model.
// It uses TaskExecutor for all task execution.
//
// The shardType argument may be either an intent verb (e.g. "/fix") or a
// persona / agent name (e.g. "coder", "reviewer", "requirements_interrogator").
// shardTypeToTaskRequest normalizes these into a valid TaskRequest so the
// executor's strict IntentVerb validation doesn't trip on persona names.
func (m *Model) spawnTask(ctx context.Context, shardType string, task string) (string, error) {
	if m.taskExecutor == nil {
		return "", fmt.Errorf("taskExecutor not initialized")
	}
	ctx = m.withShardModelContext(ctx, shardType)
	return m.taskExecutor.Execute(ctx, shardTypeToTaskRequest(shardType, task))
}

// spawnTaskWithContext spawns a task with additional session context and
// priority, returning what the executor OBSERVED about the run rather than
// only its prose.
//
// The structured return is the point. Every caller here feeds a decision —
// whether the step is done, whether a follow-up is owed, what the user is told
// happened — and each of them used to re-derive that decision by reading the
// result string for words like "TODO" or "test". The executor already recorded
// the kernel's verdict, the change stage and the gate outcomes; this carries
// them instead of throwing them away and guessing them back.
//
// An executor that cannot answer ExecuteObservedWithContext still runs the
// task: the observation then holds the output and nothing else, and consumers
// must read an absent verdict as unverified — never as success.
func (m *Model) spawnTaskWithContext(ctx context.Context, shardType string, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority) (observation.Return, error) {
	if m.taskExecutor == nil {
		return observation.Return{}, fmt.Errorf("taskExecutor not initialized")
	}
	ctx = m.withShardModelContext(ctx, shardType)
	req := shardTypeToTaskRequest(shardType, task)
	if observed, ok := m.taskExecutor.(session.ObservedTaskExecutor); ok {
		return observed.ExecuteObservedWithContext(ctx, req, sessionCtx, priority)
	}
	out, err := m.taskExecutor.ExecuteWithContext(ctx, req, sessionCtx, priority)
	return observation.Return{Agent: shardType, Task: task, Output: out}, err
}

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
			return fmt.Sprintf("fix file:%s test_errors:[%s]", target, priorShardContext(priorResult))
		}

	case "/refactor":
		// If refactoring after a review, include improvement suggestions
		if priorResult.ShardType == "reviewer" && len(priorResult.Findings) > 0 {
			// The reviewer atom's ladder is CRITICAL/HIGH/MEDIUM/LOW, and
			// extractFindings normalizes onto it. This filter used to name
			// "info" and "warning" -- log-level words the reviewer is never
			// told to use -- so a refactor after a review found no suggestions
			// no matter what the reviewer wrote.
			suggestions := filterFindingsBySeverity(priorResult.Findings, []string{"low", "medium"})
			if len(suggestions) > 0 {
				return fmt.Sprintf("refactor file:%s suggestions:[%s]", target, formatFindingsForTask(suggestions, target))
			}
		}

	case "/test":
		// If testing after a fix, include what was fixed
		if priorResult.ShardType == "coder" {
			return fmt.Sprintf("write_tests for %s after_fix context:[%s]", target, priorShardContext(priorResult))
		}

	case "/debug":
		// Include prior test or error context
		if priorResult.ShardType == "tester" || priorResult.ShardType == "reviewer" {
			return fmt.Sprintf("debug %s context:[%s]", target, priorShardContext(priorResult))
		}
	}

	return baseTask
}

// priorShardContext projects one agent's return into the task string of the
// next one.
//
// This is the blackboard: a tester's output reaching a coder, a coder's
// reaching a tester, a reviewer's reaching a debugger. It used to be
// truncateForTask(RawOutput, 500) — the first five hundred bytes of another
// agent's whole turn, newlines flattened, cut wherever byte five hundred fell.
// A shard states its plan before its findings, so that window reliably carried
// the preamble and dropped the conclusions; and because 500 is a fixed cut, a
// tester that named ten failing tests handed the coder the first two and no
// indication there were eight more. An interim fix inlined the whole output
// flattened onto one line (flattenForTask); nothing was cut, but the next
// shard still had to find the conclusions inside the whole log.
//
// The projection is bounded by structure instead. It carries the failures the
// runner named, the citations, and what is unsettled, and it retains the whole
// output under a handle the receiving agent can redeem with subagent_expand —
// which reads the retained bytes and never re-runs the shard, so expanding "the
// rest of what you already told me" cannot cost a second execution.
func priorShardContext(prior *ShardResult) string {
	if prior == nil {
		return ""
	}
	ret := observation.Return{
		Agent:  prior.ShardType,
		Task:   prior.Task,
		Output: prior.RawOutput,
	}
	// Findings arrive already parsed here — extractFindings gives file, line,
	// severity and message — so they are projected from rather than re-read out
	// of the rendered text. Re-parsing would lose exactly the fields that
	// survived, and would disagree with the copy this same ShardResult hands
	// every other consumer.
	for _, f := range prior.Findings {
		file, _ := f["file"].(string)
		severity, _ := f["severity"].(string)
		message, _ := f["message"].(string)
		if message == "" {
			message, _ = f["raw"].(string)
		}
		ret.Findings = append(ret.Findings, observation.Finding{
			File:     file,
			Line:     findingLineNumber(f),
			Severity: severity,
			Message:  message,
		})
	}
	// A tester's return IS a test runner's output, and this is the one layer
	// that knows that: the codec sees only text, and reading a pass/fail verdict
	// out of arbitrary text is how a code review saying "the error from Flush is
	// discarded" scores three test failures. Because the knowledge lives here,
	// the parse happens here, and the verdict is marked reported rather than
	// observed — nothing in this process watched the run.
	if prior.ShardType == "tester" {
		ret.Tests = observation.ReportedTests(prior.RawOutput)
	}

	return observation.SharedSubagents().
		EncodeReturn(ret, observation.ReturnLimits{}).
		Text(toolscore.SubagentExpandToolName)
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
		line := findingLineNumber(f)
		msg, _ := f["message"].(string)
		sev, _ := f["severity"].(string)

		if msg != "" {
			if line > 0 {
				parts = append(parts, fmt.Sprintf("%s@L%d:%s", sev, line, flattenForTask(msg)))
			} else {
				parts = append(parts, fmt.Sprintf("%s:%s", sev, flattenForTask(msg)))
			}
		}
	}
	return strings.Join(parts, "; ")
}

// extractFileFromFindings extracts the primary file from findings
// findingSeverityRank orders the reviewer's severity ladder so it can be
// compared. Anything unrecognised sorts below /low rather than above
// /critical, because an unknown word is missing information, not an emergency.
func findingSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// extractFileFromFindings picks the one file the fixer is sent to.
//
// The count alone does not decide it, and the previous version pretended it
// did: it ranged over a map with a strict `>`, so on a tie the winner was
// whichever key Go's randomised iteration reached first. That is not a flaky
// test, it is a flaky agent — the same review dispatches the fixer to a
// different file on two runs, with nothing in the output to say why.
//
// Ties are common rather than exotic. Two findings in one file and two in
// another is an ordinary review, and it is exactly the fixture that caught
// this.
//
// So the order is: most citations, then the worst severity among them, then
// the file the reviewer mentioned first. The middle rule is the one worth
// having on its own merits — given equal attention, the fixer should go where
// the most severe finding is, which is what a person reading the review would
// do. The last is a pure determinism backstop.
func extractFileFromFindings(findings []map[string]any) string {
	type fileScore struct {
		count      int
		worstRank  int
		firstIndex int
	}
	scores := make(map[string]*fileScore)
	for i, f := range findings {
		file, ok := f["file"].(string)
		if !ok || file == "" {
			continue
		}
		severity, _ := f["severity"].(string)
		rank := findingSeverityRank(severity)
		s := scores[file]
		if s == nil {
			scores[file] = &fileScore{count: 1, worstRank: rank, firstIndex: i}
			continue
		}
		s.count++
		if rank > s.worstRank {
			s.worstRank = rank
		}
	}

	best := ""
	var bestScore *fileScore
	for file, s := range scores {
		if bestScore == nil ||
			s.count > bestScore.count ||
			(s.count == bestScore.count && s.worstRank > bestScore.worstRank) ||
			(s.count == bestScore.count && s.worstRank == bestScore.worstRank && s.firstIndex < bestScore.firstIndex) {
			best, bestScore = file, s
		}
	}
	return best
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

// flattenForTask makes a finding message fit on the one line a task-string
// entry is, without cutting it. It used to cut at 100 characters. Whole prior
// shard output goes through priorShardContext instead, which projects it and
// retains the rest behind a subagent_expand handle.
func flattenForTask(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// withTaskConstraint carries the user's own words into the shard task. The
// free-text path hands perception's Constraint here and the /fix case used to
// drop it (observed 2026-09-17: the coder was started with "fix issue in
// <file>" and guessed what to fix); every verb now appends it.
func withTaskConstraint(base, constraint string) string {
	c := strings.TrimSpace(constraint)
	if c == "" || strings.EqualFold(c, "none") {
		return base
	}
	return base + " with constraint: " + c
}

// splitSlashTarget separates a slash command's arguments into the target the
// shard should open and the words the user typed about it. The first token is
// the target when it looks like a path; the rest is the constraint. Without a
// path-like first token the whole text is the target, as before, and there is
// no constraint. The slash handlers used to join everything into the target,
// so "/fix auth.go login fails" reached the shard as one opaque string.
func splitSlashTarget(args []string) (target, constraint string) {
	if len(args) == 0 {
		return "", ""
	}
	first := strings.TrimSpace(args[0])
	if looksLikePath(first) {
		return first, strings.TrimSpace(strings.Join(args[1:], " "))
	}
	return strings.TrimSpace(strings.Join(args, " ")), ""
}

// looksLikePath reports whether a token names a file or directory: it has a
// path separator, or a short alphanumeric extension.
func looksLikePath(s string) bool {
	if strings.ContainsAny(s, `/\`) {
		return true
	}
	i := strings.LastIndexByte(s, '.')
	if i <= 0 || i == len(s)-1 {
		return false
	}
	ext := s[i+1:]
	if len(ext) > 7 {
		return false
	}
	for _, r := range ext {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
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
			return withTaskConstraint(fmt.Sprintf("review files:%s", fileList), constraint)
		}
		if target == "codebase" {
			return withTaskConstraint("review all", constraint)
		}
		return withTaskConstraint(fmt.Sprintf("review file:%s", target), constraint)

	case "/security":
		if fileList != "" {
			return withTaskConstraint(fmt.Sprintf("security_scan files:%s", fileList), constraint)
		}
		if target == "codebase" {
			return withTaskConstraint("security_scan all", constraint)
		}
		return withTaskConstraint(fmt.Sprintf("security_scan file:%s", target), constraint)

	case "/analyze":
		if fileList != "" {
			return withTaskConstraint(fmt.Sprintf("complexity files:%s", fileList), constraint)
		}
		if target == "codebase" {
			return withTaskConstraint("complexity all", constraint)
		}
		return withTaskConstraint(fmt.Sprintf("complexity file:%s", target), constraint)

	case "/fix":
		return withTaskConstraint(fmt.Sprintf("fix file:%s", target), constraint)

	case "/refactor":
		return withTaskConstraint(fmt.Sprintf("refactor file:%s", target), constraint)
	case "/create":
		return withTaskConstraint(fmt.Sprintf("create %s", target), constraint)

	case "/test":
		if strings.Contains(target, "run") || target == "codebase" {
			return withTaskConstraint("run_tests", constraint)
		}
		return withTaskConstraint(fmt.Sprintf("write_tests for %s", target), constraint)

	case "/debug":
		return withTaskConstraint(fmt.Sprintf("debug %s", target), constraint)

	case "/research":
		return withTaskConstraint(fmt.Sprintf("research %s", target), constraint)

	case "/explore":
		return withTaskConstraint(fmt.Sprintf("explore %s", target), constraint)

	case "/document":
		return withTaskConstraint(fmt.Sprintf("document %s", target), constraint)

	case "/diff":
		return withTaskConstraint(fmt.Sprintf("review diff:%s", target), constraint)

	default:
		// Generic task format
		if constraint != "none" && constraint != "" {
			return fmt.Sprintf("%s %s with constraint: %s", verb, target, constraint)
		}
		return fmt.Sprintf("%s %s", verb, target)
	}
}

// formatDelegatedResponse creates a user-friendly response from shard execution.
// Shards may return a piggyback envelope (JSON with surface_response +
// control_packet) rather than plain text — when that happens we extract
// surface_response so the user sees clean prose instead of raw JSON. The
// fallback path preserves the original result unchanged.
func formatDelegatedResponse(intent perception.Intent, shardType, task, result string) string {
	displayResult := strings.TrimSpace(result)
	if looksLikeEnvelope(displayResult) {
		if processed := articulation.ProcessLLMResponseAllowPlain(displayResult); processed != nil &&
			processed.ParseMethod != "fallback" && strings.TrimSpace(processed.Surface) != "" {
			displayResult = strings.TrimSpace(processed.Surface)
		}
	}

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
		header = fmt.Sprintf("## %s Results", titleWords(strings.TrimPrefix(intent.Verb, "/")))
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
%s`, header, surfaceNote, intent.Target, shardType, task, displayResult)
}

// sendObserverEvent sends an event to the background observer manager (if active).
func (m Model) sendObserverEvent(eventType shards.ObserverEventType, source, target string, details map[string]string) {
	if m.observerMgr == nil {
		return
	}
	m.observerMgr.SendEvent(shards.ObserverEvent{
		Type:    eventType,
		Source:  source,
		Target:  target,
		Details: details,
	})
}

// recordShardExecution records a shard execution for prompt evolution learning.
// This enables the System Prompt Learning (SPL) system to improve prompts over time.
// It captures LLM thinking metadata (ThoughtSummary, ThinkingTokens) when available
// for the LLM-as-Judge to evaluate reasoning quality.
func (m Model) recordShardExecution(shardType, task, result string, err error, duration time.Duration) {
	// The shard profile's enable_learning gates recording. It was collected
	// by the wizard and persisted for months while every run was recorded
	// regardless; reviewer ships with it off.
	if !m.shardLearningEnabled(shardType) {
		return
	}
	if m.promptEvolver == nil {
		return
	}

	// Create execution record
	exec := &prompt_evolution.ExecutionRecord{
		TaskID:      fmt.Sprintf("shard-%d", time.Now().UnixNano()),
		SessionID:   m.sessionID,
		Timestamp:   time.Now(),
		ShardID:     fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano()),
		ShardType:   shardType,
		TaskRequest: task,
		Duration:    duration,
		ExecutionResult: prompt_evolution.ExecutionResult{
			Success: err == nil,
			Output:  result,
		},
	}

	// Add error details if failed
	if err != nil {
		exec.ExecutionResult.BuildErrors = []string{err.Error()}
	}

	exec.PromptManifest, exec.AtomIDs = m.promptContextForExecution(shardType)

	// Record which LLM produced this execution. The evolution loop groups
	// failures by serving model and pins the atoms it generates to that model,
	// so without this the atom learned from one vendor's failure modes is
	// served to every other vendor. A client that cannot report its identity
	// leaves both empty, which groups the record under the unpinned bucket.
	if mi, ok := m.client.(types.ModelIdentifier); ok {
		exec.Provider, exec.Model = mi.ModelIdentity()
	}

	// Extract thinking metadata if client supports it (Gemini 3 with Thinking Mode)
	// This allows the LLM-as-Judge to evaluate the model's reasoning process
	if tp, ok := m.client.(types.ThinkingProvider); ok {
		exec.ThoughtSummary = tp.GetLastThoughtSummary()
		exec.ThinkingTokens = tp.GetLastThinkingTokens()
	}

	// Extract grounding sources if client supports it (Gemini with Google Search)
	// This provides transparency about which sources influenced the response
	if gp, ok := m.client.(types.GroundingProvider); ok {
		exec.GroundingSources = gp.GetLastGroundingSources()
	}

	// Record the execution asynchronously
	go func() {
		if recErr := m.promptEvolver.RecordExecution(exec); recErr != nil {
			logging.Get(logging.CategoryAutopoiesis).Debug("Failed to record shard execution: %v", recErr)
		}
	}()
}

func (m Model) promptContextForExecution(shardType string) (*promptpkg.PromptManifest, []string) {
	if m.jitCompiler == nil {
		return nil, nil
	}

	jitResult := m.jitCompiler.GetLastResult()
	if jitResult == nil {
		return nil, nil
	}

	trimmedShardType := strings.TrimSpace(shardType)
	if trimmedShardType != "" && jitResult.Stats != nil {
		statsShard := strings.TrimSpace(jitResult.Stats.ShardID)
		if statsShard != "" && statsShard != trimmedShardType && statsShard != strings.TrimPrefix(trimmedShardType, "/") {
			return nil, nil
		}
	}

	var manifest *promptpkg.PromptManifest
	if jitResult.Manifest != nil {
		manifest = clonePromptManifest(jitResult.Manifest)
	}

	atomIDs := collectExecutionAtomIDs(jitResult)
	return manifest, atomIDs
}

func collectExecutionAtomIDs(result *promptpkg.CompilationResult) []string {
	if result == nil {
		return nil
	}

	if result.Manifest != nil && len(result.Manifest.Selected) > 0 {
		ids := make([]string, 0, len(result.Manifest.Selected))
		for _, entry := range result.Manifest.Selected {
			if strings.TrimSpace(entry.ID) != "" {
				ids = append(ids, entry.ID)
			}
		}
		return ids
	}

	if len(result.IncludedAtoms) == 0 {
		return nil
	}

	ids := make([]string, 0, len(result.IncludedAtoms))
	for _, atom := range result.IncludedAtoms {
		if atom == nil || strings.TrimSpace(atom.ID) == "" {
			continue
		}
		ids = append(ids, atom.ID)
	}
	return ids
}

func clonePromptManifest(src *promptpkg.PromptManifest) *promptpkg.PromptManifest {
	if src == nil {
		return nil
	}

	cloned := *src
	if len(src.Selected) > 0 {
		cloned.Selected = append([]promptpkg.AtomManifestEntry(nil), src.Selected...)
	}
	if len(src.Dropped) > 0 {
		cloned.Dropped = append([]promptpkg.DroppedAtomEntry(nil), src.Dropped...)
	}
	return &cloned
}

// spawnShard spawns a shard agent for a task
func (m Model) spawnShard(shardType, task string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().ShardExecutionTimeout)
		defer cancel()

		startTime := time.Now()
		m.ReportStatus(fmt.Sprintf("Spawning %s...", shardType))

		// Notify observers about task start
		m.sendObserverEvent(shards.EventTaskStarted, shardType, task, map[string]string{
			"shard_type": shardType,
		})

		result, err := m.spawnTask(ctx, shardType, task)
		duration := time.Since(startTime)

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

		// Record execution for prompt evolution learning
		m.recordShardExecution(shardType, task, result, err, duration)

		if err != nil {
			// Notify observers about task failure
			m.sendObserverEvent(shards.EventTaskFailed, shardType, task, map[string]string{
				"shard_type": shardType,
				"error":      err.Error(),
			})
			return errorMsg(fmt.Errorf("shard spawn failed: %w", err))
		}

		// Notify observers about task completion
		m.sendObserverEvent(shards.EventTaskCompleted, shardType, task, map[string]string{
			"shard_type": shardType,
		})

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

		// 4. Tell the kernel what the matcher found, then let
		//    specialist_should_execute (policy/shards.mg) pick the
		//    high-confidence executor. The Go boolean is the nil-kernel fallback.
		complexity := shards.TaskComplexity(task, len(files))
		m.assertSpecialistMatches(task, complexity, specialists)
		if spec, ok := m.specialistThatShouldExecute(task, specialists); ok {
			return m.executeSpecialistDirectMode(ctx, verb, spec, task, target, complexity, startTime)
		}

		// 5. Route based on execution mode
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

// assertSpecialistMatches loads the specialist classification table and this
// task's matches into the kernel so the specialist_* rules in
// policy/shards.mg can fire. Per-task facts from the previous delegation are
// retracted first; the classification table is static and idempotent.
func (m Model) assertSpecialistMatches(task, complexity string, specialists []shards.SpecialistMatch) {
	if m.kernel == nil {
		return
	}
	_ = m.kernel.Retract("specialist_match")
	_ = m.kernel.Retract("task_complexity")
	facts := append(shards.ClassificationFacts(), shards.MatchFacts(task, complexity, specialists)...)
	if err := m.kernel.LoadFacts(facts); err != nil {
		logging.Get(logging.CategoryRouting).Warn("failed to load specialist facts: %v", err)
	}
}

// specialistThatShouldExecute returns the specialist the kernel derived
// specialist_should_execute for on this task, in match order. Without a
// kernel it applies the same test in Go (an executor matched above 80).
func (m Model) specialistThatShouldExecute(task string, specialists []shards.SpecialistMatch) (shards.SpecialistMatch, bool) {
	if m.kernel == nil {
		for _, spec := range specialists {
			if spec.ShouldExecute && spec.Classification != nil &&
				spec.Classification.ExecutionMode == shards.SpecialistModeExecutor {
				return spec, true
			}
		}
		return shards.SpecialistMatch{}, false
	}
	derived, err := m.kernel.Query("specialist_should_execute")
	if err != nil {
		logging.Get(logging.CategoryRouting).Warn("specialist_should_execute query failed: %v", err)
		return shards.SpecialistMatch{}, false
	}
	chosen := make(map[string]struct{}, len(derived))
	for _, f := range derived {
		if len(f.Args) >= 2 && types.ExtractString(f.Args[1]) == task {
			chosen[types.ExtractString(f.Args[0])] = struct{}{}
		}
	}
	for _, spec := range specialists {
		if _, ok := chosen[string(shards.SpecialistAtom(spec.AgentName))]; ok {
			return spec, true
		}
	}
	return shards.SpecialistMatch{}, false
}

// strategicAdvisorRequired reports whether the kernel derived
// strategic_advisor_required for this task (a /high task while a strategic
// advisor is classified). Without a kernel it applies the Go test.
func (m Model) strategicAdvisorRequired(task, complexity string, executor shards.SpecialistMatch) bool {
	if m.kernel == nil {
		return shards.ShouldConsultBeforeExecution(executor.AgentName, strings.TrimPrefix(complexity, "/"))
	}
	derived, err := m.kernel.Query("strategic_advisor_required")
	if err != nil {
		logging.Get(logging.CategoryRouting).Warn("strategic_advisor_required query failed: %v", err)
		return false
	}
	for _, f := range derived {
		if len(f.Args) >= 1 && types.ExtractString(f.Args[0]) == task {
			return true
		}
	}
	return false
}

// spawnSimpleShard handles the case where no specialists are matched
func (m Model) spawnSimpleShard(ctx context.Context, shardType, task string, startTime time.Time) tea.Msg {
	m.ReportStatus(fmt.Sprintf("Spawning %s...", shardType))
	result, err := m.spawnTask(ctx, shardType, task)
	duration := time.Since(startTime)
	shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)
	if m.kernel != nil && len(facts) > 0 {
		if err := m.kernel.LoadFacts(facts); err != nil {
			logging.Routing("[delegation] failed to load shard facts: %v", err)
		}
	}

	// Record execution for prompt evolution learning
	m.recordShardExecution(shardType, task, result, err, duration)

	if err != nil {
		return errorMsg(fmt.Errorf("shard spawn failed: %w", err))
	}
	response := fmt.Sprintf("## Shard Execution Complete\n\n**Agent**: %s\n**Task**: %s\n**Duration**: %s\n\n### Result\n%s",
		shardType, task, duration.Round(time.Second), result)
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

// findingLineNumber reads a finding's line number, whichever numeric shape it
// arrived in.
//
// Findings reach here two ways: parsed from a shard's text by extractFindings,
// which stores a Go int, and decoded from JSON, where every number is a
// float64. The consumer asserted float64 only, so an extractor-produced finding
// silently reported line 0 -- which reads as "no line known" and drops the
// citation the reviewer was required to provide.
func findingLineNumber(f map[string]any) int {
	switch v := f["line"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return 0
}
