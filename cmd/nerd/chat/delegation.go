// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains shard spawning and task delegation helpers.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/articulation"
	prompt_evolution "codenerd/internal/autopoiesis/prompt_evolution"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	promptpkg "codenerd/internal/prompt"
	"codenerd/internal/shards"
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

// spawnTaskWithContext spawns a task with additional session context and priority.
// This is used for dream mode, shadow mode, and other speculative execution scenarios.
func (m *Model) spawnTaskWithContext(ctx context.Context, shardType string, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	if m.taskExecutor == nil {
		return "", fmt.Errorf("taskExecutor not initialized")
	}
	ctx = m.withShardModelContext(ctx, shardType)
	return m.taskExecutor.ExecuteWithContext(ctx, shardTypeToTaskRequest(shardType, task), sessionCtx, priority)
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

		// 4. Check for high-confidence executor specialist that should handle directly
		//    This implements the specialist_should_execute rule from shards.mg
		for _, spec := range specialists {
			if spec.ShouldExecute && spec.Classification != nil &&
				spec.Classification.ExecutionMode == shards.SpecialistModeExecutor {
				// High-confidence executor specialist - route directly to them
				return m.executeSpecialistDirectMode(ctx, verb, spec, task, target, startTime)
			}
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
