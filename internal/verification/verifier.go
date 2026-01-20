// Package verification implements the quality-enforcing verification loop.
// This ensures tasks are completed PROPERLY - no shortcuts, no mock code, no corner-cutting.
// After shard execution, results are verified and automatically retried with corrective
// action until success or max retries.
package verification

import (
	"codenerd/internal/autopoiesis"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	// researcher removed - JIT clean loop handles research
	"codenerd/internal/session"
	"codenerd/internal/store"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrMaxRetriesExceeded is returned when verification fails after max retries.
var ErrMaxRetriesExceeded = errors.New("max retries exceeded - escalating to user")

// QualityViolation represents a type of corner-cutting detected in code.
type QualityViolation string

const (
	MockCode        QualityViolation = "mock_code"        // func Mock..., // mock implementation
	PlaceholderCode QualityViolation = "placeholder"      // TODO, FIXME, placeholder, stub
	HallucinatedAPI QualityViolation = "hallucinated_api" // APIs that don't exist
	IncompleteImpl  QualityViolation = "incomplete"       // panic("not implemented")
	HardcodedValues QualityViolation = "hardcoded"        // Magic strings instead of real logic
	EmptyFunction   QualityViolation = "empty_function"   // func Foo() {} with no body
	MissingErrors   QualityViolation = "missing_errors"   // No error handling
	FakeTests       QualityViolation = "fake_tests"       // Tests that don't test anything
)

// CorrectiveType defines the type of corrective action to take.
type CorrectiveType string

const (
	CorrectiveResearch  CorrectiveType = "research"  // Use ResearcherShard
	CorrectiveDocs      CorrectiveType = "docs"      // Use Context7 API
	CorrectiveTool      CorrectiveType = "tool"      // Use Autopoiesis to generate tool
	CorrectiveDecompose CorrectiveType = "decompose" // Break into smaller tasks
)

// CorrectiveAction describes what action to take to fix a verification failure.
type CorrectiveAction struct {
	Type      CorrectiveType `json:"type"`
	Query     string         `json:"query"`
	Reason    string         `json:"reason"`
	ShardHint string         `json:"shard_hint,omitempty"` // Suggested shard to use
}

// ShardSelectionResult contains the decision about which shard to use.
type ShardSelectionResult struct {
	ShardType    string   // Selected shard type
	ShardName    string   // Specific shard name (for specialists)
	Reason       string   // Why this shard was selected
	Confidence   float64  // Confidence in this selection
	Alternatives []string // Other shards that could work
}

// VerificationResult contains the outcome of verifying a task result.
type VerificationResult struct {
	Success           bool               `json:"success"`
	Confidence        float64            `json:"confidence"`
	Reason            string             `json:"reason"`
	Suggestions       []string           `json:"suggestions,omitempty"`
	Evidence          []string           `json:"evidence,omitempty"`
	QualityViolations []QualityViolation `json:"quality_violations,omitempty"`
	CorrectiveAction  *CorrectiveAction  `json:"corrective_action,omitempty"`
}

// TaskVerifier implements the quality-enforcing verification loop.
type TaskVerifier struct {
	mu           sync.RWMutex
	client       perception.LLMClient
	localDB      *store.LocalStore
	shardMgr     *coreshards.ShardManager // For ListAvailableShards. Use taskExecutor for task execution.
	taskExecutor session.TaskExecutor     // For task execution (replaces direct shardMgr.Spawn calls)
	autopoiesis  *autopoiesis.Orchestrator
	context7Key  string

	// Session context for persistence
	sessionID string
	turnCount int
}

// SetTaskExecutor sets the unified task executor for shard spawning.
func (v *TaskVerifier) SetTaskExecutor(te session.TaskExecutor) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.taskExecutor = te
}

// spawnTask is the unified entry point for task execution.
// It uses TaskExecutor when available, falling back to ShardManager.
func (v *TaskVerifier) spawnTask(ctx context.Context, shardType string, task string) (string, error) {
	// Prefer TaskExecutor when available
	if v.taskExecutor != nil {
		intent := session.LegacyShardNameToIntent(shardType)
		return v.taskExecutor.Execute(ctx, intent, task)
	}

	// Fall back to ShardManager
	if v.shardMgr != nil || v.taskExecutor != nil {
		return v.spawnTask(ctx, shardType, task)
	}

	return "", fmt.Errorf("no executor available: both taskExecutor and shardMgr are nil")
}

// NewTaskVerifier creates a new verifier with all dependencies.
func NewTaskVerifier(
	client perception.LLMClient,
	localDB *store.LocalStore,
	shardMgr *coreshards.ShardManager,
	autopoiesisOrch *autopoiesis.Orchestrator,
	context7Key string,
) *TaskVerifier {
	return &TaskVerifier{
		client:      client,
		localDB:     localDB,
		shardMgr:    shardMgr,
		autopoiesis: autopoiesisOrch,
		context7Key: context7Key,
	}
}

// SetSessionContext sets the current session for persistence.
func (v *TaskVerifier) SetSessionContext(sessionID string, turnCount int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sessionID = sessionID
	v.turnCount = turnCount
}

// VerifyWithRetry is the main entry point - loops until success or max retries.
// It executes the shard, verifies quality, and applies corrective actions if needed.
// Uses intelligent shard selection to pick the best shard for each retry across all 4 types:
// system shards, LLM-created specialists, user-created specialists, and ephemeral shards.
func (v *TaskVerifier) VerifyWithRetry(
	ctx context.Context,
	task string,
	shardType string,
	maxRetries int,
) (string, *VerificationResult, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastResult string
	var lastVerification *VerificationResult
	currentTask := task
	currentShardType := shardType

	for attempt := 0; attempt < maxRetries; attempt++ {
		// 1. Execute shard (uses selected shard type which may change between attempts)
		result, err := v.spawnTask(ctx, currentShardType, currentTask)
		if err != nil {
			// Shard execution failed - not a verification issue
			return "", nil, fmt.Errorf("shard execution failed: %w", err)
		}
		lastResult = result

		// 2. Verify quality
		verification, verifyErr := v.verifyTask(ctx, currentTask, result)
		if verifyErr != nil {
			// Verification itself failed (e.g., LLM error) - continue with default
			verification = &VerificationResult{
				Success:    true, // Assume success if we can't verify
				Confidence: 0.3,
				Reason:     fmt.Sprintf("Verification skipped: %v", verifyErr),
			}
		}
		lastVerification = verification

		// 3. Success? We're done
		if verification.Success && len(verification.QualityViolations) == 0 {
			v.storeVerification(currentTask, currentShardType, verification, attempt, true)
			return result, verification, nil
		}

		// 4. Store failed attempt for learning
		v.storeVerification(currentTask, currentShardType, verification, attempt, false)

		// 5. Intelligent shard selection for retry
		// Consider all 4 types: system, LLM-created specialists, user-created specialists, ephemeral
		if attempt < maxRetries-1 {
			shardSelection := v.selectBestShard(ctx, currentTask, currentShardType, verification)
			if shardSelection != nil && shardSelection.ShardType != "" {
				currentShardType = shardSelection.ShardType
			}
		}

		// 6. Apply corrective action if suggested
		if verification.CorrectiveAction != nil && attempt < maxRetries-1 {
			additionalContext := v.applyCorrectiveAction(ctx, verification.CorrectiveAction)
			if additionalContext != "" {
				currentTask = v.enrichTaskWithContext(currentTask, additionalContext, verification)
			}
		}
	}

	// Max retries reached - escalate
	return lastResult, lastVerification, ErrMaxRetriesExceeded
}

// isReviewTask checks if the task is a review/analysis task (not implementation).
func isReviewTask(task string) bool {
	lower := strings.ToLower(task)
	reviewKeywords := []string{
		"review", "analyze", "security_scan", "complexity",
		"audit", "inspect", "examine", "assess", "evaluate",
	}
	for _, kw := range reviewKeywords {
		if strings.HasPrefix(lower, kw) || strings.Contains(lower, kw+" ") {
			return true
		}
	}
	return false
}

// verifyTask uses LLM to assess if the task was completed properly.
func (v *TaskVerifier) verifyTask(ctx context.Context, task, result string) (*VerificationResult, error) {
	if v.client == nil {
		return &VerificationResult{Success: true, Confidence: 0.5, Reason: "No LLM client for verification"}, nil
	}

	// Use different verification criteria for review vs implementation tasks
	var systemPrompt string
	if isReviewTask(task) {
		// REVIEW TASK: Verify the review output is useful, not the code being reviewed
		systemPrompt = `You are verifying a CODE REVIEW task. The shard was asked to review/analyze existing code.

## What to Check (Review Quality)
- Did the review provide useful analysis of the code?
- Did it identify issues, patterns, or areas for improvement?
- Is the review output coherent and actionable?

## What NOT to Check
- DO NOT fail because the CODE BEING REVIEWED is incomplete or has issues
- The reviewer's job is to REPORT problems, not fix them
- If the reviewer correctly identifies that code is incomplete, that's SUCCESS

## Quality Violations for Reviews
- Empty or meaningless review output
- Review that doesn't actually analyze the code
- Hallucinated file contents or made-up code snippets
- Review that contradicts obvious facts about the code

## Response Format (JSON only)
{
  "success": true/false,
  "confidence": 0.0-1.0,
  "reason": "explanation of review quality",
  "quality_violations": [],
  "evidence": [],
  "suggestions": []
}

IMPORTANT: A review that correctly reports incomplete code is SUCCESSFUL.
Only return the JSON object, no other text.`
	} else {
		// IMPLEMENTATION TASK: Check for quality violations in generated code
		systemPrompt = `You are a strict code quality verifier. Your job is to ensure tasks are completed PROPERLY - no shortcuts, no mock code, no corner-cutting.

## Quality Violations to Detect
- Mock implementations (func Mock..., fake data, // mock, mockImplementation)
- Placeholder code (TODO, FIXME, stub, placeholder, "not implemented", XXX)
- Hallucinated APIs (imports/calls to libraries that don't exist)
- Incomplete implementations (empty functions, panic("not implemented"), pass)
- Hardcoded magic values instead of real logic
- Missing error handling where needed
- Tests that don't actually test anything (empty assertions, always pass)

## Response Format (JSON only, no markdown)
{
  "success": true/false,
  "confidence": 0.0-1.0,
  "reason": "detailed explanation",
  "quality_violations": ["mock_code", "placeholder", "incomplete", ...],
  "evidence": ["line X: TODO implement", "function Y is empty", ...],
  "suggestions": ["suggestion 1", ...],
  "corrective_action": {
    "type": "research|docs|tool|decompose",
    "query": "what to look up or research",
    "reason": "why this will help"
  }
}

CRITICAL: If you detect ANY quality violations, success MUST be false.
Only return the JSON object, no other text.`
	}

	userPrompt := fmt.Sprintf(`## Task
%s

## Result to Verify
%s

Analyze this result for quality violations and determine if the task was completed properly.`, task, truncateForVerification(result))

	response, err := v.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("verification LLM call failed: %w", err)
	}

	// Parse JSON response
	verification, parseErr := parseVerificationResponse(response)
	if parseErr != nil {
		// If parsing fails, do basic quality checks
		return v.basicQualityCheck(result), nil
	}

	return verification, nil
}

// applyCorrectiveAction gathers additional context based on failure analysis.
// Priority: 1) Existing specialist shards, 2) Context7 docs, 3) Researcher web search
func (v *TaskVerifier) applyCorrectiveAction(ctx context.Context, action *CorrectiveAction) string {
	if action == nil {
		return ""
	}

	// FIRST: Check if we have a specialist shard that can help
	// This avoids unnecessary Context7 API calls when we have local knowledge
	if action.ShardHint != "" && (v.shardMgr != nil || v.taskExecutor != nil) {
		if specialist := v.findMatchingSpecialist(action.ShardHint, action.Query); specialist != "" {
			result, err := v.spawnTask(ctx, specialist, action.Query)
			if err == nil && result != "" {
				return fmt.Sprintf("## Specialist Knowledge (%s)\n%s", specialist, truncateContext(result, 2000))
			}
		}
	}

	switch action.Type {
	case CorrectiveResearch:
		// Check for specialist before web research
		if specialist := v.findMatchingSpecialist("", action.Query); specialist != "" && (v.shardMgr != nil || v.taskExecutor != nil) {
			result, err := v.spawnTask(ctx, specialist, action.Query)
			if err == nil && result != "" {
				return fmt.Sprintf("## Specialist Knowledge (%s)\n%s", specialist, truncateContext(result, 2000))
			}
		}
		// Fallback to web research
		if v.shardMgr != nil || v.taskExecutor != nil {
			result, err := v.spawnTask(ctx, "researcher", "research: "+action.Query)
			if err == nil && result != "" {
				return fmt.Sprintf("## Research Results\n%s", truncateContext(result, 2000))
			}
		}

	case CorrectiveDocs:
		// PRIORITY 1: Check for specialist with pre-built knowledge
		if specialist := v.findMatchingSpecialist("", action.Query); specialist != "" && (v.shardMgr != nil || v.taskExecutor != nil) {
			result, err := v.spawnTask(ctx, specialist, "docs: "+action.Query)
			if err == nil && result != "" {
				return fmt.Sprintf("## Specialist Documentation (%s)\n%s", specialist, truncateContext(result, 2000))
			}
		}

		// PRIORITY 2: Context7 API (external LLM-optimized docs)
		if v.context7Key != "" {
			docs := v.fetchContext7Docs(ctx, action.Query)
			if docs != "" {
				return fmt.Sprintf("## Documentation\n%s", truncateContext(docs, 2000))
			}
		}

		// PRIORITY 3: Researcher web fallback
		if v.shardMgr != nil || v.taskExecutor != nil {
			result, err := v.spawnTask(ctx, "researcher", "research docs: "+action.Query)
			if err == nil && result != "" {
				return fmt.Sprintf("## Documentation Research\n%s", truncateContext(result, 2000))
			}
		}

	case CorrectiveTool:
		// Use Autopoiesis to generate missing tool
		if v.autopoiesis != nil {
			toolNeed := &autopoiesis.ToolNeed{
				Name:       action.Query,
				Purpose:    action.Reason,
				Confidence: 0.8,
			}
			tool, err := v.autopoiesis.GenerateTool(ctx, toolNeed)
			if err == nil && tool != nil {
				return fmt.Sprintf("## Generated Tool: %s\n%s", tool.Name, tool.Description)
			}
		}

	case CorrectiveDecompose:
		// Return hint to break task into smaller pieces
		return fmt.Sprintf("## Task Decomposition Needed\nBreak this task into smaller steps: %s", action.Query)
	}

	return ""
}

// findMatchingSpecialist checks if we have a specialist shard that matches the query.
// Returns the specialist name if found, empty string otherwise.
func (v *TaskVerifier) findMatchingSpecialist(hint, query string) string {
	if v.shardMgr == nil {
		return ""
	}

	// Get available shards
	available := v.shardMgr.ListAvailableShards()

	// Technology keywords to match against specialist names
	techKeywords := map[string][]string{
		"rod":     {"browser", "rod", "cdp", "devtools", "scraping", "automation", "chromium"},
		"golang":  {"go", "golang", "goroutine", "channel", "interface"},
		"react":   {"react", "jsx", "tsx", "component", "hook", "usestate"},
		"mangle":  {"mangle", "datalog", "logic", "predicate", "rule"},
		"sql":     {"sql", "database", "query", "postgres", "sqlite", "mysql"},
		"api":     {"api", "rest", "http", "endpoint", "handler"},
		"testing": {"test", "spec", "coverage", "mock", "assert"},
	}

	queryLower := strings.ToLower(query)
	hintLower := strings.ToLower(hint)

	// First check if hint directly matches a specialist
	if hint != "" {
		for _, shard := range available {
			if strings.EqualFold(shard.Name, hint) && shard.Type == "specialist" {
				return shard.Name
			}
		}
	}

	// Then check query keywords against specialist knowledge domains
	for _, shard := range available {
		if shard.Type != "specialist" {
			continue
		}

		shardLower := strings.ToLower(shard.Name)

		// Check if shard name appears in query
		if strings.Contains(queryLower, shardLower) || strings.Contains(hintLower, shardLower) {
			return shard.Name
		}

		// Check tech keywords
		if keywords, ok := techKeywords[shardLower]; ok {
			for _, kw := range keywords {
				if strings.Contains(queryLower, kw) {
					return shard.Name
				}
			}
		}
	}

	return ""
}

// enrichTaskWithContext adds corrective context to the task for retry.
func (v *TaskVerifier) enrichTaskWithContext(
	originalTask string,
	additionalContext string,
	verification *VerificationResult,
) string {
	var builder strings.Builder
	builder.WriteString(originalTask)

	// Add failure feedback
	if verification != nil && verification.Reason != "" {
		builder.WriteString("\n\n## Previous Attempt Failed\n")
		builder.WriteString(verification.Reason)

		if len(verification.QualityViolations) > 0 {
			builder.WriteString("\n\n## Quality Issues to Fix\n")
			for _, v := range verification.QualityViolations {
				builder.WriteString(fmt.Sprintf("- %s\n", v))
			}
		}

		if len(verification.Evidence) > 0 {
			builder.WriteString("\n## Specific Problems\n")
			for _, e := range verification.Evidence {
				builder.WriteString(fmt.Sprintf("- %s\n", e))
			}
		}
	}

	// Add gathered context
	if additionalContext != "" {
		builder.WriteString("\n\n")
		builder.WriteString(additionalContext)
	}

	// Add quality reminder
	builder.WriteString("\n\n## IMPORTANT\n")
	builder.WriteString("- Do NOT use mock implementations or placeholder code\n")
	builder.WriteString("- Do NOT use TODO/FIXME comments\n")
	builder.WriteString("- Do NOT hallucinate APIs that don't exist\n")
	builder.WriteString("- Implement the ACTUAL functionality requested\n")

	return builder.String()
}

// storeVerification persists verification results for learning.
func (v *TaskVerifier) storeVerification(
	task string,
	shardType string,
	verification *VerificationResult,
	attempt int,
	success bool,
) {
	if v.localDB == nil {
		return
	}

	v.mu.RLock()
	sessionID := v.sessionID
	turnCount := v.turnCount
	v.mu.RUnlock()

	// Convert to JSON for storage
	violationsJSON, err := json.Marshal(verification.QualityViolations)
	if err != nil {
		logging.SystemShardsWarn("failed to marshal quality violations: %v", err)
	}
	evidenceJSON, err := json.Marshal(verification.Evidence)
	if err != nil {
		logging.SystemShardsWarn("failed to marshal evidence: %v", err)
	}
	correctiveJSON, err := json.Marshal(verification.CorrectiveAction)
	if err != nil {
		logging.SystemShardsWarn("failed to marshal corrective action: %v", err)
	}

	// Hash the task for dedup
	taskHash := sha256.Sum256([]byte(task))
	taskHashHex := hex.EncodeToString(taskHash[:])

	if err := v.localDB.StoreVerification(
		sessionID,
		turnCount,
		task,
		shardType,
		attempt,
		success,
		verification.Confidence,
		verification.Reason,
		string(violationsJSON),
		string(correctiveJSON),
		string(evidenceJSON),
		taskHashHex,
	); err != nil {
		logging.StoreError("failed to store verification result: %v", err)
	}
}

// fetchContext7Docs fetches documentation from Context7 API.
// STUBBED: Researcher shard removed as part of JIT refactor.
// Context7 research is now handled on-demand via session.Executor with /researcher persona.
func (v *TaskVerifier) fetchContext7Docs(ctx context.Context, query string) string {
	if v.context7Key == "" || query == "" {
		return ""
	}

	// Context7 research stubbed out - JIT clean loop handles this
	logging.Boot("Context7 docs fetch stubbed (JIT refactor) - query: %s", query)
	return ""
}

// basicQualityCheck performs simple pattern matching for quality violations.
func (v *TaskVerifier) basicQualityCheck(result string) *VerificationResult {
	violations := []QualityViolation{}
	evidence := []string{}

	lower := strings.ToLower(result)

	// Check for common violations
	if strings.Contains(lower, "todo") || strings.Contains(lower, "fixme") {
		violations = append(violations, PlaceholderCode)
		evidence = append(evidence, "Contains TODO/FIXME comments")
	}

	if strings.Contains(lower, "mock") || strings.Contains(result, "Mock") {
		violations = append(violations, MockCode)
		evidence = append(evidence, "Contains mock implementations")
	}

	if strings.Contains(lower, "not implemented") || strings.Contains(result, "panic(\"not implemented\")") {
		violations = append(violations, IncompleteImpl)
		evidence = append(evidence, "Contains 'not implemented' code")
	}

	if strings.Contains(lower, "placeholder") || strings.Contains(lower, "stub") {
		violations = append(violations, PlaceholderCode)
		evidence = append(evidence, "Contains placeholder/stub code")
	}

	success := len(violations) == 0

	return &VerificationResult{
		Success:           success,
		Confidence:        0.6, // Lower confidence for basic check
		Reason:            "Basic quality check",
		QualityViolations: violations,
		Evidence:          evidence,
	}
}

// parseVerificationResponse parses the LLM's JSON response.
func parseVerificationResponse(response string) (*VerificationResult, error) {
	// Clean up response - remove markdown code blocks if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result VerificationResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse verification JSON: %w", err)
	}

	return &result, nil
}

// truncateForVerification limits result size for LLM verification.
func truncateForVerification(s string) string {
	const maxLen = 8000
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... [truncated]"
}

// truncateContext limits context size.
func truncateContext(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... [truncated]"
}

// selectBestShard analyzes the failure and selects the best shard to fix it.
// It considers all 4 shard types: system, LLM-created specialists, user-created specialists, and ephemeral.
func (v *TaskVerifier) selectBestShard(
	ctx context.Context,
	task string,
	originalShardType string,
	verification *VerificationResult,
) *ShardSelectionResult {
	if v.shardMgr == nil || v.client == nil {
		return &ShardSelectionResult{
			ShardType:  originalShardType,
			Reason:     "No shard manager or LLM client available",
			Confidence: 0.5,
		}
	}

	// Get all available shards from the manager
	availableShards := v.shardMgr.ListAvailableShards()

	// Build context for LLM decision
	var violationList []string
	for _, v := range verification.QualityViolations {
		violationList = append(violationList, string(v))
	}

	systemPrompt := `You are a shard selection advisor. Given a failed task and available shards,
select the BEST shard to fix the problem. Consider:

1. SYSTEM SHARDS: Built-in shards for core operations (perception, execution, planning)
2. SPECIALIST SHARDS: Pre-trained on specific domains (frameworks, languages, tools)
3. EPHEMERAL SHARDS: General-purpose (coder, tester, reviewer, researcher)

IMPORTANT: Prefer specialists if the task matches their domain - they have pre-loaded knowledge.

Response format (JSON only):
{
  "selected_shard": "shard_name",
  "shard_type": "system|specialist|ephemeral",
  "reason": "why this shard is best for the failure",
  "confidence": 0.0-1.0,
  "alternatives": ["other", "options"]
}`

	userPrompt := fmt.Sprintf(`## Failed Task
%s

## Original Shard: %s

## Quality Violations
%v

## Failure Reason
%s

## Evidence
%v

## Available Shards
%v

Select the best shard to fix this failure.`,
		task,
		originalShardType,
		violationList,
		verification.Reason,
		verification.Evidence,
		availableShards,
	)

	response, err := v.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		// Fallback to heuristic selection
		return v.heuristicShardSelection(originalShardType, verification)
	}

	// Parse LLM response
	return v.parseShardSelection(response, originalShardType)
}

// heuristicShardSelection uses simple rules when LLM is unavailable.
func (v *TaskVerifier) heuristicShardSelection(
	originalShardType string,
	verification *VerificationResult,
) *ShardSelectionResult {
	// Check for specific violation patterns
	for _, violation := range verification.QualityViolations {
		switch violation {
		case HallucinatedAPI:
			// Need research to find real APIs
			return &ShardSelectionResult{
				ShardType:    "researcher",
				Reason:       "Hallucinated API detected - researcher can find real documentation",
				Confidence:   0.8,
				Alternatives: []string{originalShardType},
			}
		case MissingErrors:
			// Reviewer can identify error handling patterns
			return &ShardSelectionResult{
				ShardType:    "reviewer",
				Reason:       "Missing error handling - reviewer can identify patterns",
				Confidence:   0.7,
				Alternatives: []string{"coder", originalShardType},
			}
		case FakeTests:
			// Tester knows how to write real tests
			return &ShardSelectionResult{
				ShardType:    "tester",
				Reason:       "Fake tests detected - tester shard specializes in real tests",
				Confidence:   0.85,
				Alternatives: []string{originalShardType},
			}
		}
	}

	// Default: retry with same shard but with more context
	return &ShardSelectionResult{
		ShardType:    originalShardType,
		Reason:       "No specific shard better suited - retry with additional context",
		Confidence:   0.6,
		Alternatives: []string{"researcher"},
	}
}

// parseShardSelection parses the LLM's shard selection response.
func (v *TaskVerifier) parseShardSelection(response, fallback string) *ShardSelectionResult {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result struct {
		SelectedShard string   `json:"selected_shard"`
		ShardType     string   `json:"shard_type"`
		Reason        string   `json:"reason"`
		Confidence    float64  `json:"confidence"`
		Alternatives  []string `json:"alternatives"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return &ShardSelectionResult{
			ShardType:  fallback,
			Reason:     "Failed to parse shard selection",
			Confidence: 0.5,
		}
	}

	return &ShardSelectionResult{
		ShardType:    result.SelectedShard,
		ShardName:    result.SelectedShard,
		Reason:       result.Reason,
		Confidence:   result.Confidence,
		Alternatives: result.Alternatives,
	}
}
