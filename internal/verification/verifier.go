// Package verification runs a chat delegation to its end: it spawns the
// persona's turn, reads the turn's kernel verdict, has an LLM judge look at an
// attempt the kernel called done, and asks the kernel what happens next
// (delegation_move, policy/delegation.mg): accept, retry carrying why, or
// escalate at the persona's cap. The kernel's verdict decides; the judge can
// only withhold (sweep finding F5).
package verification

import (
	"codenerd/internal/autopoiesis"
	"codenerd/internal/broker"
	"codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrMaxRetriesExceeded is returned when verification fails after max retries.
var ErrMaxRetriesExceeded = errors.New("max retries exceeded - escalating to user")

// ErrVerificationUnavailable is returned when verification itself could not
// run (LLM failure, timeout, malformed judgment, missing client). It is a
// fail-closed signal: the shard result is NOT verified and must not be
// treated as a success. Callers should use errors.Is to detect it.
var ErrVerificationUnavailable = errors.New("verification unavailable")

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

// Kernel is what a delegation needs of the kernel: to assert its facts, ask
// what they derive, and retract them when it ends.
type Kernel interface {
	Query(predicate string) ([]types.Fact, error)
	Assert(fact types.Fact) error
	RetractFact(fact types.Fact) error
}

// TaskVerifier runs chat delegations to their end.
type TaskVerifier struct {
	mu           sync.RWMutex
	client       perception.LLMClient
	localDB      *store.LocalStore
	shardMgr     *coreshards.ShardManager // For ListAvailableShards. Use taskExecutor for task execution.
	taskExecutor session.TaskExecutor     // For task execution (replaces direct shardMgr.Spawn calls)
	kernel       Kernel                   // Decides each delegation's attempts
	autopoiesis  *autopoiesis.Orchestrator

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

// SetKernel sets the kernel that decides each delegation's attempts.
func (v *TaskVerifier) SetKernel(k Kernel) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.kernel = k
}

// spawnTask runs a corrective action's side task (a specialist's knowledge),
// whose output is context for the next attempt and whose verdict nothing
// reads. It uses TaskExecutor when available, falling back to ShardManager.
// A delegation's own attempts go through ExecuteObserved (VerifyWithRetry).
// The persona or verb goes to the executor as it is: the executor maps a
// persona to its verb (the kernel's persona_verb table).
func (v *TaskVerifier) spawnTask(ctx context.Context, intent string, task string) (string, error) {

	// Prefer TaskExecutor when available
	if v.taskExecutor != nil {
		req := session.TaskRequest{
			IntentVerb: intent,
			Task:       task,
		}
		return v.taskExecutor.Execute(ctx, req)
	}

	// Fall back to ShardManager
	if v.shardMgr != nil {
		return v.shardMgr.Spawn(ctx, intent, task)
	}

	return "", fmt.Errorf("no executor available: both taskExecutor and shardMgr are nil")
}

// NewTaskVerifier creates a new verifier with all dependencies.
func NewTaskVerifier(
	client perception.LLMClient,
	localDB *store.LocalStore,
	shardMgr *coreshards.ShardManager,
	autopoiesisOrch *autopoiesis.Orchestrator,
) *TaskVerifier {
	return &TaskVerifier{
		client:      client,
		localDB:     localDB,
		shardMgr:    shardMgr,
		autopoiesis: autopoiesisOrch,
	}
}

// SetSessionContext sets the current session for persistence.
func (v *TaskVerifier) SetSessionContext(sessionID string, turnCount int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sessionID = sessionID
	v.turnCount = turnCount
}

// Delegation is one chat request handed to a persona.
type Delegation struct {
	// Task is what the persona is asked to do.
	Task string
	// Persona is the shard persona ("coder", "reviewer", a specialist's
	// name) or an intent verb.
	Persona string
	// MaxAttempts is the persona's attempt cap,
	// shard_profiles.<persona>.max_retries.
	MaxAttempts int
	// Params are the delegation section's thresholds
	// (config.DelegationConfig.Params).
	Params []config.Param
}

// VerifyWithRetry runs a delegation to its end. Each attempt spawns the
// persona's turn through the observed executor, so the turn's kernel verdict
// comes back typed; that verdict, and the LLM judge's view of an attempt the
// kernel called done, are asserted, and the kernel derives the move
// (delegation_move): accept, retry with why the attempt was not accepted, or
// escalate at the persona's cap (ErrMaxRetriesExceeded). The judge can only
// withhold. A judge that cannot run fails the delegation closed
// (ErrVerificationUnavailable): retrying cannot fix it.
//
// Until 2026-09-23 the attempts went through the string-only executor, which
// dropped the verdict, so a turn the kernel ended /unverified completed the
// request on the judge's word; an LLM advisor picked who retried, and a retry
// without a corrective action re-ran the task with no word of why.
func (v *TaskVerifier) VerifyWithRetry(ctx context.Context, d Delegation) (string, *VerificationResult, error) {
	v.mu.RLock()
	te, k := v.taskExecutor, v.kernel
	v.mu.RUnlock()
	observed, ok := te.(session.ObservedTaskExecutor)
	if !ok {
		return "", nil, fmt.Errorf("task executor %T returns no verdict; a delegation reads each attempt's", te)
	}
	if k == nil {
		return "", nil, errors.New("no kernel to decide the delegation's attempts")
	}
	if d.MaxAttempts < 1 {
		return "", nil, fmt.Errorf("no attempt cap for %s: shard_profiles.%s.max_retries must be at least 1", d.Persona, d.Persona)
	}
	if err := config.EnsureParams(k, d.Params); err != nil {
		return "", nil, fmt.Errorf("assert the delegation thresholds: %w", err)
	}

	// Verification retries multiply spend: a task verified three times costs
	// four inferences, not one. Attributing them here is what makes that
	// multiplier visible instead of hiding inside the caller's budget.
	ctx = broker.WithPurpose(ctx, broker.PurposeVerification)

	root := fmt.Sprintf("delegation-%d", time.Now().UnixNano())
	defer forgetDelegation(k, root)
	if err := k.Assert(types.Fact{Predicate: "delegation_request", Args: []any{root, personaAtom(d.Persona), int64(d.MaxAttempts)}}); err != nil {
		return "", nil, fmt.Errorf("assert the delegation: %w", err)
	}

	intent := strings.TrimSpace(d.Persona)
	task := d.Task
	for attempt := int64(1); ; attempt++ {
		ret, err := observed.ExecuteObserved(ctx, session.TaskRequest{IntentVerb: intent, Task: task})
		if err != nil {
			return ret.Output, nil, fmt.Errorf("shard execution failed: %w", err)
		}
		if err := k.Assert(types.Fact{Predicate: "delegation_attempt", Args: []any{root, attempt, outcomeAtom(ret.Outcome)}}); err != nil {
			return ret.Output, nil, fmt.Errorf("assert attempt %d: %w", attempt, err)
		}

		verdict := turnVerdict(ret.Outcome, ret.Missing)
		due, err := derivedFor(k, "delegation_judge_due", root, attempt)
		if err != nil {
			return ret.Output, verdict, err
		}
		if len(due) > 0 {
			rubric, err := derivedFor(k, "delegation_judge_rubric", root, -1)
			if err != nil || len(rubric) != 1 {
				return ret.Output, verdict, fmt.Errorf("the kernel derived %d judge rubrics for %s (%v)", len(rubric), root, err)
			}
			judged, jerr := v.verifyTask(ctx, d.Task, ret.Output, rubric[0])
			if jerr != nil {
				verdict = &VerificationResult{
					Success:    false,
					Confidence: 0,
					Reason:     fmt.Sprintf("verification unavailable: %v", jerr),
				}
				v.storeVerification(d.Task, d.Persona, verdict, int(attempt-1), false)
				return ret.Output, verdict, fmt.Errorf("shard result could not be verified (%v): %w", jerr, ErrVerificationUnavailable)
			}
			verdict = judged
			if err := k.Assert(types.Fact{Predicate: "judge_verdict", Args: []any{root, attempt, judgeAtom(judged), percent(judged.Confidence)}}); err != nil {
				return ret.Output, verdict, fmt.Errorf("assert the judgment of attempt %d: %w", attempt, err)
			}
		}

		moves, err := derivedFor(k, "delegation_move", root, attempt)
		if err != nil {
			return ret.Output, verdict, err
		}
		if len(moves) != 1 {
			return ret.Output, verdict, fmt.Errorf("the kernel derived %d moves for attempt %d of %s (%v); the policy must decide one", len(moves), attempt, root, moves)
		}
		switch moves[0] {
		case "/accept":
			if !verdict.Success {
				// The kernel accepted over a judge below
				// delegation.judge_reject_confidence.
				verdict.Reason = fmt.Sprintf("accepted: the turn ended /done; the judge's objection (%.0f%% confidence) is below delegation.judge_reject_confidence: %s", verdict.Confidence*100, verdict.Reason)
				verdict.Success = true
			}
			v.storeVerification(d.Task, d.Persona, verdict, int(attempt-1), true)
			return ret.Output, verdict, nil
		case "/escalate":
			v.storeVerification(d.Task, d.Persona, verdict, int(attempt-1), false)
			return ret.Output, verdict, ErrMaxRetriesExceeded
		case "/retry":
			v.storeVerification(d.Task, d.Persona, verdict, int(attempt-1), false)
			var gathered string
			if verdict.CorrectiveAction != nil {
				gathered = v.applyCorrectiveAction(ctx, verdict.CorrectiveAction)
			}
			task = v.enrichTaskWithContext(d.Task, gathered, verdict)
		default:
			return ret.Output, verdict, fmt.Errorf("the kernel derived an unknown move %s for %s", moves[0], root)
		}
	}
}

// turnVerdict is what the turn's kernel verdict says, before any judge.
func turnVerdict(outcome string, missing []string) *VerificationResult {
	if outcome == "/done" {
		return &VerificationResult{Success: true, Confidence: 1, Reason: "the turn ended /done"}
	}
	if outcome == "" {
		return &VerificationResult{Reason: "the turn returned no verdict"}
	}
	reason := "the turn ended " + outcome
	if why := session.DescribeMissingEvidence(missing); why != "" {
		reason += ": " + why
	}
	return &VerificationResult{Reason: reason}
}

func outcomeAtom(outcome string) types.MangleAtom {
	if strings.HasPrefix(outcome, "/") && len(outcome) > 1 {
		return types.MangleAtom(outcome)
	}
	return types.MangleAtom("/none")
}

func personaAtom(persona string) types.MangleAtom {
	return types.MangleAtom("/" + strings.TrimPrefix(strings.TrimSpace(persona), "/"))
}

// judgeAtom is the judge's verdict: a pass is a success with no quality
// violation.
func judgeAtom(judged *VerificationResult) types.MangleAtom {
	if judged.Success && len(judged.QualityViolations) == 0 {
		return types.MangleAtom("/pass")
	}
	return types.MangleAtom("/fail")
}

// percent is a 0-1 confidence as an integer percent: Mangle compares
// integers only.
func percent(confidence float64) int64 {
	return int64(min(max(confidence, 0), 1) * 100)
}

// derivedFor returns the last argument of every row of predicate whose first
// argument is root and, when attempt is not negative, whose second is
// attempt.
func derivedFor(k Kernel, predicate, root string, attempt int64) ([]string, error) {
	rows, err := k.Query(predicate)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", predicate, err)
	}
	var out []string
	for _, f := range rows {
		if len(f.Args) < 2 || types.ExtractString(f.Args[0]) != root {
			continue
		}
		if attempt >= 0 {
			if n, ok := f.Args[1].(int64); !ok || n != attempt {
				continue
			}
		}
		out = append(out, types.ExtractString(f.Args[len(f.Args)-1]))
	}
	return out, nil
}

// forgetDelegation retracts a finished delegation's facts: they describe
// attempts that are over.
func forgetDelegation(k Kernel, root string) {
	for _, p := range []string{"delegation_request", "delegation_attempt", "judge_verdict"} {
		if err := k.RetractFact(types.Fact{Predicate: p, Args: []any{root}}); err != nil {
			logging.SystemShardsWarn("retract %s for %s: %v", p, root, err)
		}
	}
}

// verifyTask asks the LLM judge whether an attempt did the task properly,
// with the rubric the kernel chose (delegation_judge_rubric: /review for an
// analysis persona, /implementation otherwise). A judgment that cannot be
// had -- no client, a failed call, a malformed answer -- is an error: the
// caller fails closed. Until 2026-09-23 a malformed answer fell back to a
// keyword scan of the output for "todo" and "mock", which could pass it,
// and the rubric was picked by keywords in the task text.
func (v *TaskVerifier) verifyTask(ctx context.Context, task, result, rubric string) (*VerificationResult, error) {
	if v.client == nil {
		return nil, ErrVerificationUnavailable
	}

	var systemPrompt string
	if rubric == "/review" {
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

Analyze this result for quality violations and determine if the task was completed properly.`, task, result)

	response, err := v.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("verification LLM call failed: %w", err)
	}

	verification, parseErr := parseVerificationResponse(response)
	if parseErr != nil {
		return nil, fmt.Errorf("the judgment was malformed: %w", parseErr)
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
				return fmt.Sprintf("## Specialist Knowledge (%s)\n%s", specialist, result)
			}
		}
	}

	switch action.Type {
	case CorrectiveResearch:
		// Check for specialist before web research
		if specialist := v.findMatchingSpecialist("", action.Query); specialist != "" && (v.shardMgr != nil || v.taskExecutor != nil) {
			result, err := v.spawnTask(ctx, specialist, action.Query)
			if err == nil && result != "" {
				return fmt.Sprintf("## Specialist Knowledge (%s)\n%s", specialist, result)
			}
		}

	case CorrectiveDocs:
		// Check for specialist with pre-built knowledge
		if specialist := v.findMatchingSpecialist("", action.Query); specialist != "" && (v.shardMgr != nil || v.taskExecutor != nil) {
			result, err := v.spawnTask(ctx, specialist, "docs: "+action.Query)
			if err == nil && result != "" {
				return fmt.Sprintf("## Specialist Documentation (%s)\n%s", specialist, result)
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
