// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file implements the core feedback tracking and learning system for tool optimization.
//
// The Learning Loop:
// Execute → Evaluate → Detect Patterns → Refine → Re-Execute
//
// This closes the autopoiesis cycle - not just creating tools, but learning
// from their execution and continuously improving them.
package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/types"

	"codenerd/internal/logging"
)

// =============================================================================
// EXECUTION FEEDBACK - WHAT HAPPENED WHEN THE TOOL RAN?
// =============================================================================

// ExecutionFeedback captures everything about a tool execution
type ExecutionFeedback struct {
	// Identity
	ToolName           string    `json:"tool_name"`
	ExecutionID        string    `json:"execution_id"`
	RequestFingerprint string    `json:"request_fingerprint,omitempty"`
	Timestamp          time.Time `json:"timestamp"`

	// Input/Output
	Input               string `json:"input"`
	Output              string `json:"output"`
	OutputSize          int    `json:"output_size"`
	ProcessStarted      bool   `json:"process_started,omitempty"`
	ExitCode            int    `json:"exit_code,omitempty"`
	Stdout              string `json:"stdout,omitempty"`
	Stderr              string `json:"stderr,omitempty"`
	PartialOutput       bool   `json:"partial_output,omitempty"`
	OutputTruncated     bool   `json:"output_truncated,omitempty"`
	BackendError        string `json:"backend_error,omitempty"`
	ValidationError     string `json:"validation_error,omitempty"`
	ValidationCompleted bool   `json:"validation_completed,omitempty"`
	ValidationPassed    bool   `json:"validation_passed,omitempty"`

	// Performance
	Duration   time.Duration `json:"duration"`
	MemoryUsed int64         `json:"memory_used,omitempty"`
	RetryCount int           `json:"retry_count"`

	// Success/Failure
	Success   bool   `json:"success"`
	ErrorType string `json:"error_type,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`

	// Quality Signals (filled by evaluator)
	Quality *QualityAssessment `json:"quality,omitempty"`

	// User Feedback (filled by user interaction)
	UserFeedback *UserFeedback `json:"user_feedback,omitempty"`

	// Context
	IntentID    string            `json:"intent_id,omitempty"`
	TaskContext map[string]string `json:"task_context,omitempty"`
}

// UserFeedback captures explicit user reactions to tool output
type UserFeedback struct {
	Accepted    bool      `json:"accepted"`    // Did user accept the output?
	Modified    bool      `json:"modified"`    // Did user modify/correct it?
	Reran       bool      `json:"reran"`       // Did user ask to re-run?
	Complaint   string    `json:"complaint"`   // User's complaint if any
	Improvement string    `json:"improvement"` // What user wanted instead
	Timestamp   time.Time `json:"timestamp"`
}

// =============================================================================
// TOOL REFINER - IMPROVE TOOLS BASED ON FEEDBACK
// =============================================================================

// ToolRefiner generates improved tool versions based on feedback
type ToolRefiner struct {
	client          LLMClient
	toolGen         *ToolGenerator
	promptAssembler PromptAssembler
	jitEnabled      bool
}

// RefinementRequest describes what needs to be improved
type RefinementRequest struct {
	ToolName     string
	OriginalCode string
	Feedback     []ExecutionFeedback
	Patterns     []*DetectedPattern
	Suggestions  []ImprovementSuggestion
}

// RefinementResult contains the improved tool
type RefinementResult struct {
	Success      bool
	ImprovedCode string
	Changes      []string // Description of changes made
	ExpectedGain float64  // Expected quality improvement
	TestCases    []string // Test cases to verify improvement
}

// NewToolRefiner creates a new tool refiner
func NewToolRefiner(client LLMClient, toolGen *ToolGenerator) *ToolRefiner {
	logging.AutopoiesisDebug("Creating ToolRefiner")
	return &ToolRefiner{
		client:  client,
		toolGen: toolGen,
	}
}

// Refine generates an improved version of a tool
func (tr *ToolRefiner) Refine(ctx context.Context, req RefinementRequest) (*RefinementResult, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "ToolRefiner.Refine")
	defer timer.Stop()

	logging.Autopoiesis("Refining tool: %s (%d feedback items, %d patterns)",
		req.ToolName, len(req.Feedback), len(req.Patterns))

	// Try JIT compilation first if available
	if tr.jitEnabled && tr.promptAssembler != nil && tr.promptAssembler.JITReady() {
		return tr.refineWithJIT(ctx, req)
	}

	return tr.refineLegacy(ctx, req)
}

func (tr *ToolRefiner) refineLegacy(ctx context.Context, req RefinementRequest) (*RefinementResult, error) {
	// Fallback to legacy refinement
	result := &RefinementResult{
		Changes:   []string{},
		TestCases: []string{},
	}

	// Build improvement prompt
	prompt := tr.buildRefinementPrompt(req)
	logging.AutopoiesisDebug("Built refinement prompt: %d chars", len(prompt))

	logging.AutopoiesisDebug("Sending refinement request to LLM")
	llmTimer := logging.StartTimer(logging.CategoryAutopoiesis, "LLMRefinement")
	resp, err := tr.client.CompleteWithSystem(ctx, refinementSystemPrompt, prompt)
	llmTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Refinement LLM call failed: %v", err)
		return nil, fmt.Errorf("refinement failed: %w", err)
	}
	logging.AutopoiesisDebug("Received LLM response: %d chars", len(resp))

	// Parse response
	var refinement struct {
		ImprovedCode string   `json:"improved_code"`
		Changes      []string `json:"changes"`
		ExpectedGain float64  `json:"expected_gain"`
		TestCases    []string `json:"test_cases"`
	}

	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &refinement); err != nil {
		logging.AutopoiesisDebug("JSON parsing failed, trying code block extraction")
		// Try to extract code block directly
		code := extractCodeBlock(resp, "go")
		if code != "" {
			result.ImprovedCode = code
			result.Success = true
			result.Changes = []string{"LLM-generated improvements"}
			logging.Autopoiesis("Tool refined via code block extraction: %s", req.ToolName)
			return result, nil
		}
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to parse refinement response: %v", err)
		return nil, fmt.Errorf("failed to parse refinement: %w", err)
	}

	result.Success = true
	result.ImprovedCode = refinement.ImprovedCode
	result.Changes = refinement.Changes
	result.ExpectedGain = refinement.ExpectedGain
	result.TestCases = refinement.TestCases

	logging.Autopoiesis("Tool refined successfully: %s (expectedGain=%.2f, changes=%d)",
		req.ToolName, result.ExpectedGain, len(result.Changes))
	for i, change := range result.Changes {
		logging.AutopoiesisDebug("  Change %d: %s", i+1, change)
	}

	return result, nil
}

// refineWithJIT generates an improved version using JIT-compiled prompts
func (tr *ToolRefiner) refineWithJIT(ctx context.Context, req RefinementRequest) (*RefinementResult, error) {
	logging.AutopoiesisDebug("Refining tool with JIT: %s", req.ToolName)

	result := &RefinementResult{
		Changes:   []string{},
		TestCases: []string{},
	}

	// Build prompt context for refinement stage
	pc := map[string]any{
		"shard_id":        "tool_refiner_" + req.ToolName,
		"shard_type":      "tool_generator",
		"stage":           "/refinement",
		"ouroboros_stage": "/refinement",
		"tool_name":       req.ToolName,
	}

	// Assemble system prompt using JIT compiler
	systemPrompt, err := tr.promptAssembler.AssembleSystemPrompt(ctx, pc)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("JIT assembly failed for refinement, falling back: %v", err)
		// Fall back to legacy refinement without re-entering the JIT path.
		return tr.refineLegacy(ctx, req)
	}

	logging.AutopoiesisDebug("JIT-compiled refinement system prompt: %d bytes", len(systemPrompt))

	// Build user prompt with feedback
	userPrompt := tr.buildRefinementPrompt(req)

	logging.AutopoiesisDebug("Sending JIT refinement request to LLM")
	llmTimer := logging.StartTimer(logging.CategoryAutopoiesis, "LLMRefinementJIT")
	resp, err := tr.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	llmTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("JIT refinement LLM call failed: %v", err)
		return nil, fmt.Errorf("jit refinement failed: %w", err)
	}
	logging.AutopoiesisDebug("Received JIT LLM response: %d chars", len(resp))

	// Parse response
	var refinement struct {
		ImprovedCode string   `json:"improved_code"`
		Changes      []string `json:"changes"`
		ExpectedGain float64  `json:"expected_gain"`
		TestCases    []string `json:"test_cases"`
	}

	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &refinement); err != nil {
		logging.AutopoiesisDebug("JSON parsing failed, trying code block extraction")
		// Try to extract code block directly
		code := extractCodeBlock(resp, "go")
		if code != "" {
			result.ImprovedCode = code
			result.Success = true
			result.Changes = []string{"JIT-generated improvements"}
			logging.Autopoiesis("Tool refined via JIT code block extraction: %s", req.ToolName)
			return result, nil
		}
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to parse JIT refinement response: %v", err)
		return nil, fmt.Errorf("failed to parse jit refinement: %w", err)
	}

	result.Success = true
	result.ImprovedCode = refinement.ImprovedCode
	result.Changes = refinement.Changes
	result.ExpectedGain = refinement.ExpectedGain
	result.TestCases = refinement.TestCases

	logging.Autopoiesis("Tool refined successfully with JIT: %s (expectedGain=%.2f, changes=%d)",
		req.ToolName, result.ExpectedGain, len(result.Changes))
	for i, change := range result.Changes {
		logging.AutopoiesisDebug("  Change %d: %s", i+1, change)
	}

	return result, nil
}

// SetPromptAssembler attaches a JIT-aware prompt assembler to the tool refiner
func (tr *ToolRefiner) SetPromptAssembler(assembler PromptAssembler) {
	tr.promptAssembler = assembler
	tr.jitEnabled = assembler != nil && assembler.JITReady()
	if tr.jitEnabled {
		logging.Autopoiesis("JIT prompt compilation enabled for ToolRefiner")
	}
}

// buildRefinementPrompt creates the prompt for tool improvement
func (tr *ToolRefiner) buildRefinementPrompt(req RefinementRequest) string {
	var sb strings.Builder
	sb.WriteString("Improve this tool based on execution feedback:\n\n")
	sb.WriteString(fmt.Sprintf("Tool Name: %s\n\n", req.ToolName))
	sb.WriteString(fmt.Sprintf("Original Code:\n```go\n%s\n```\n\n", req.OriginalCode))

	// Add feedback summary
	sb.WriteString("Execution Feedback:\n")
	for i, fb := range req.Feedback {
		if i >= 3 {
			sb.WriteString(fmt.Sprintf("... and %d more executions\n", len(req.Feedback)-3))
			break
		}
		quality := 0.0
		var issues []QualityIssue
		if fb.Quality != nil {
			quality = fb.Quality.Score
			issues = fb.Quality.Issues
		}
		sb.WriteString(fmt.Sprintf("- Execution %d: success=%v, quality=%.2f\n", i+1, fb.Success, quality))
		for _, issue := range issues {
			sb.WriteString(fmt.Sprintf("  - Issue: %s (%s)\n", issue.Type, issue.Description))
		}
	}

	// Add detected patterns
	if len(req.Patterns) > 0 {
		sb.WriteString("\nRecurring Patterns:\n")
		for _, p := range req.Patterns {
			sb.WriteString(fmt.Sprintf("- %s: %d occurrences (%.0f%% confidence)\n", p.IssueType, p.Occurrences, p.Confidence*100))
		}
	}

	// Add suggestions
	if len(req.Suggestions) > 0 {
		sb.WriteString("\nSuggested Improvements:\n")
		for _, s := range req.Suggestions {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Type, s.Description))
			if s.CodeHint != "" {
				sb.WriteString(fmt.Sprintf("  Hint: %s\n", s.CodeHint))
			}
		}
	}

	sb.WriteString(`
Return JSON with:
{
  "improved_code": "full improved Go code",
  "changes": ["list of changes made"],
  "expected_gain": 0.0-1.0,
  "test_cases": ["test case descriptions to verify improvements"]
}
`)

	return sb.String()
}

var refinementSystemPrompt = `You are an elite Go code optimizer. Improve reliability, safety, and completeness.

Avoid domain assumptions. Do not add network retries/pagination unless the code clearly performs network I/O.

Prioritize:
1. NIL SAFETY - Check pointers/interfaces before dereference.
2. RESOURCE SAFETY - Close files/streams/connections with defer.
3. CONTEXT - Respect cancellation in loops/blocking operations.
4. ERROR WRAPPING - Return descriptive errors using %w when appropriate.
5. BOUNDS CHECKS - Prevent slice/map/index panics.

Fix root causes without inventing unrelated features. Return clean, idiomatic Go.`

// =============================================================================
// LEARNING STORE - PERSIST LEARNINGS
// =============================================================================

// LearningStore persists tool learnings for future reference
type LearningStore struct {
	mu         sync.RWMutex
	storePath  string
	learnings  map[string]*ToolLearning
	executions map[string]types.GeneratedLearningAck
	feedback   map[string]ExecutionFeedback
	loadErr    error
}

type durableLearningSnapshot struct {
	Version    int                                   `json:"version"`
	Learnings  map[string]*ToolLearning              `json:"learnings"`
	Executions map[string]types.GeneratedLearningAck `json:"executions"`
	Feedback   map[string]ExecutionFeedback          `json:"feedback"`
}

// ToolLearning contains all learnings about a tool
type ToolLearning struct {
	ToolName        string      `json:"tool_name"`
	Version         int         `json:"version"`
	TotalExecutions int         `json:"total_executions"`
	SuccessRate     float64     `json:"success_rate"`
	AverageQuality  float64     `json:"average_quality"`
	KnownIssues     []IssueType `json:"known_issues"`
	AppliedFixes    []string    `json:"applied_fixes"`
	BestPractices   []string    `json:"best_practices"`
	AntiPatterns    []string    `json:"anti_patterns"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// NewLearningStore creates a new learning store
func NewLearningStore(storePath string) *LearningStore {
	logging.AutopoiesisDebug("Creating LearningStore: path=%s", storePath)
	store := &LearningStore{
		storePath:  storePath,
		learnings:  make(map[string]*ToolLearning),
		executions: make(map[string]types.GeneratedLearningAck),
		feedback:   make(map[string]ExecutionFeedback),
	}
	store.load()
	logging.Autopoiesis("LearningStore initialized with %d existing learnings", len(store.learnings))
	return store
}

// maxLearningListLen caps the length of unbounded slices on ToolLearning so a
// single misbehaving tool can't grow them without limit. Once the cap is hit,
// new entries are silently dropped (a debug line records the drop).
const maxLearningListLen = 100

// RecordLearning updates learnings for a tool
func (ls *LearningStore) RecordLearning(toolName string, feedback *ExecutionFeedback, patterns []*DetectedPattern) {
	if feedback == nil || strings.TrimSpace(toolName) == "" {
		return
	}
	copyFeedback := *feedback
	if copyFeedback.ExecutionID == "" {
		copyFeedback.ExecutionID = fmt.Sprintf("legacy-%d", time.Now().UnixNano())
	}
	if copyFeedback.RequestFingerprint == "" {
		copyFeedback.RequestFingerprint = copyFeedback.ExecutionID
	}
	if _, err := ls.RecordLearningDurable(context.Background(), toolName, &copyFeedback, patterns); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Tool learning was not acknowledged: %v", err)
	}
}

// RecordLearningDurable serializes mutation and publication in one transaction.
// Its receipt survives reopen; an unsuccessful publication never updates memory.
func (ls *LearningStore) RecordLearningDurable(ctx context.Context, toolName string, feedback *ExecutionFeedback, patterns []*DetectedPattern) (ack types.GeneratedLearningAck, err error) {
	// Guard: nil feedback would panic on feedback.Success below.
	if feedback == nil {
		logging.AutopoiesisDebug("RecordLearning: nil feedback, skipping (tool=%q)", toolName)
		return ack, fmt.Errorf("nil execution feedback")
	}
	// Guard: empty toolName produces useless empty-key entries.
	if strings.TrimSpace(toolName) == "" {
		logging.AutopoiesisDebug("RecordLearning: empty toolName, skipping")
		return ack, fmt.Errorf("empty feedback tool name")
	}
	if feedback.ExecutionID == "" || feedback.RequestFingerprint == "" {
		return ack, fmt.Errorf("missing feedback identity")
	}
	if err := ctx.Err(); err != nil {
		return ack, err
	}

	logging.AutopoiesisDebug("Recording learning for tool: %s (success=%v)", toolName, feedback.Success)

	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.loadErr != nil {
		return ack, ls.loadErr
	}
	if prior, ok := ls.executions[feedback.ExecutionID]; ok {
		if prior.Fingerprint != feedback.RequestFingerprint {
			return ack, fmt.Errorf("conflicting feedback identity")
		}
		prior.Replayed = true
		return prior, nil
	}
	previous := ls.learnings
	encodedPrevious, err := json.Marshal(previous)
	if err != nil {
		return ack, err
	}
	staged := make(map[string]*ToolLearning)
	if err := json.Unmarshal(encodedPrevious, &staged); err != nil {
		return ack, err
	}
	learning, exists := staged[toolName]
	if !exists {
		logging.AutopoiesisDebug("Creating new learning record for tool: %s", toolName)
		learning = &ToolLearning{
			ToolName:      toolName,
			Version:       1,
			KnownIssues:   []IssueType{},
			AppliedFixes:  []string{},
			BestPractices: []string{},
			AntiPatterns:  []string{},
		}
		staged[toolName] = learning
	}

	// Update statistics
	learning.TotalExecutions++
	if feedback.Success {
		learning.SuccessRate = (learning.SuccessRate*float64(learning.TotalExecutions-1) + 1.0) /
			float64(learning.TotalExecutions)
	} else {
		learning.SuccessRate = learning.SuccessRate * float64(learning.TotalExecutions-1) /
			float64(learning.TotalExecutions)
	}

	if feedback.Quality != nil {
		// Reject NaN/Inf scores so the running average never becomes poisoned.
		if !math.IsNaN(feedback.Quality.Score) && !math.IsInf(feedback.Quality.Score, 0) {
			learning.AverageQuality = (learning.AverageQuality*float64(learning.TotalExecutions-1) +
				feedback.Quality.Score) / float64(learning.TotalExecutions)
		}

		// Track known issues (capped to prevent unbounded growth)
		newIssues := 0
		for _, issue := range feedback.Quality.Issues {
			if !containsIssueType(learning.KnownIssues, issue.Type) {
				if len(learning.KnownIssues) >= maxLearningListLen {
					logging.AutopoiesisDebug("KnownIssues cap reached for %s, dropping %v", toolName, issue.Type)
					continue
				}
				learning.KnownIssues = append(learning.KnownIssues, issue.Type)
				newIssues++
			}
		}
		if newIssues > 0 {
			logging.AutopoiesisDebug("Added %d new known issues for %s", newIssues, toolName)
		}
	}

	// Extract anti-patterns from patterns (capped to prevent unbounded growth)
	newPatterns := 0
	for _, p := range patterns {
		if p != nil && p.Confidence > 0.7 {
			antiPattern := fmt.Sprintf("%s: %s", p.IssueType, p.PatternID)
			if !contains(learning.AntiPatterns, antiPattern) {
				if len(learning.AntiPatterns) >= maxLearningListLen {
					logging.AutopoiesisDebug("AntiPatterns cap reached for %s, dropping %q", toolName, antiPattern)
					continue
				}
				learning.AntiPatterns = append(learning.AntiPatterns, antiPattern)
				newPatterns++
			}
		}
	}
	if newPatterns > 0 {
		logging.AutopoiesisDebug("Added %d new anti-patterns for %s", newPatterns, toolName)
	}

	learning.UpdatedAt = time.Now()

	ack = types.GeneratedLearningAck{ExecutionID: feedback.ExecutionID, Fingerprint: feedback.RequestFingerprint,
		Path: filepath.Join(ls.storePath, "tool_learnings.json"), CommittedAt: time.Now(), Durable: true}
	executions := make(map[string]types.GeneratedLearningAck, len(ls.executions)+1)
	for key, value := range ls.executions {
		executions[key] = value
	}
	executions[feedback.ExecutionID] = ack
	records := make(map[string]ExecutionFeedback, len(ls.feedback)+1)
	for key, value := range ls.feedback {
		records[key] = value
	}
	record := *feedback
	if record.Quality != nil && (math.IsNaN(record.Quality.Score) || math.IsInf(record.Quality.Score, 0)) {
		record.Quality = nil
	}
	recordBytes, err := json.Marshal(record)
	if err != nil {
		return types.GeneratedLearningAck{}, err
	}
	record = ExecutionFeedback{}
	if err = json.Unmarshal(recordBytes, &record); err != nil {
		return types.GeneratedLearningAck{}, err
	}
	records[feedback.ExecutionID] = record
	data, err := json.MarshalIndent(durableLearningSnapshot{Version: 2, Learnings: staged, Executions: executions, Feedback: records}, "", "  ")
	if err != nil {
		return types.GeneratedLearningAck{}, err
	}
	if err = ctx.Err(); err != nil {
		return types.GeneratedLearningAck{}, err
	}
	if err = ls.saveBytes(data); err != nil {
		return types.GeneratedLearningAck{}, err
	}
	ls.learnings, ls.executions, ls.feedback = staged, executions, records

	logging.Autopoiesis("Learning recorded for %s: executions=%d, successRate=%.2f, avgQuality=%.2f",
		toolName, learning.TotalExecutions, learning.SuccessRate, learning.AverageQuality)
	return ack, nil
}

// GetLearning retrieves learnings for a tool
func (ls *LearningStore) GetLearning(toolName string) *ToolLearning {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	learning := ls.learnings[toolName]
	if learning == nil {
		return nil
	}
	clone := *learning
	clone.KnownIssues = slices.Clone(learning.KnownIssues)
	clone.AppliedFixes = slices.Clone(learning.AppliedFixes)
	clone.BestPractices = slices.Clone(learning.BestPractices)
	clone.AntiPatterns = slices.Clone(learning.AntiPatterns)
	return &clone
}

func (ls *LearningStore) ExecutionAcknowledgment(id string) (types.GeneratedLearningAck, bool) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	ack, ok := ls.executions[id]
	return ack, ok
}

func (ls *LearningStore) DurabilityError() error {
	if ls == nil {
		return fmt.Errorf("learning store unavailable")
	}
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.loadErr
}

func (ls *LearningStore) ExecutionFeedback(id string) (ExecutionFeedback, bool) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	record, ok := ls.feedback[id]
	// Return a deep copy so recall cannot mutate the durable snapshot.
	data, err := json.Marshal(record)
	if err != nil {
		return ExecutionFeedback{}, false
	}
	var clone ExecutionFeedback
	if json.Unmarshal(data, &clone) != nil {
		return ExecutionFeedback{}, false
	}
	return clone, ok
}

// GetAllLearnings returns all tool learnings
func (ls *LearningStore) GetAllLearnings() []*ToolLearning {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	learnings := make([]*ToolLearning, 0, len(ls.learnings))
	for _, l := range ls.learnings {
		clone := *l
		clone.KnownIssues = append([]IssueType(nil), l.KnownIssues...)
		clone.AppliedFixes = append([]string(nil), l.AppliedFixes...)
		clone.BestPractices = append([]string(nil), l.BestPractices...)
		clone.AntiPatterns = append([]string(nil), l.AntiPatterns...)
		learnings = append(learnings, &clone)
	}
	return learnings
}

// GenerateMangleFacts creates Mangle facts from learnings
func (ls *LearningStore) GenerateMangleFacts() []string {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	facts := []string{}
	for _, l := range ls.learnings {
		// %d, not %.2f: schemas_tools.mg declares both metrics /number, and a
		// float literal here becomes an ast.Float64 that this Mangle fork's
		// comparison builtins reject — taking the entire fixpoint down, not
		// just the tool_quality_* rules. These facts are persisted to
		// learned.mg, so a float would poison every subsequent boot too.
		facts = append(facts, fmt.Sprintf(`tool_learning(%q, %d, %d, %d).`,
			l.ToolName, l.TotalExecutions, normalizePercent(l.SuccessRate), normalizePercent(l.AverageQuality)))

		for _, issue := range l.KnownIssues {
			facts = append(facts, fmt.Sprintf(`tool_known_issue(%q, %s).`,
				l.ToolName, normalizeCapabilityName(string(issue))))
		}
	}
	return facts
}

// load reads learnings from disk.
//
// The unmarshal used to be a bare call. A truncated or half-written
// tool_learnings.json therefore loaded as an empty store — every tool's success
// rate, known issues and anti-patterns gone — with no error, no log line and no
// difference from a first run. Worse, the next saveBytes then overwrote the
// damaged file with that empty map, so the corruption became permanent and the
// evidence was destroyed.
//
// Now a parse failure is logged at Error and the file is set aside with a
// .corrupt-<unix> suffix, which both preserves it for recovery and stops the
// next save from silently overwriting it. Decoding into a temporary map keeps a
// partial parse from merging half a file into a live store.
func (ls *LearningStore) load() {
	path := filepath.Join(ls.storePath, "tool_learnings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			ls.loadErr = err
		}
		return
	}

	loaded := make(map[string]*ToolLearning)
	var snapshot durableLearningSnapshot
	decodeErr := json.Unmarshal(data, &snapshot)
	if decodeErr == nil && snapshot.Version == 2 {
		loaded = snapshot.Learnings
		if loaded == nil || snapshot.Executions == nil || snapshot.Feedback == nil {
			decodeErr = fmt.Errorf("incomplete durable learning snapshot")
		}
	} else {
		decodeErr = json.Unmarshal(data, &loaded)
	}
	if decodeErr != nil {
		ls.loadErr = fmt.Errorf("durable learning requires recovery: %w", decodeErr)
		quarantine := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		logging.Get(logging.CategoryAutopoiesis).Error(
			"Tool learning store %s is corrupt (%d bytes); everything learned about every tool would be silently forgotten, so it is preserved at %s: %v",
			path, len(data), quarantine, decodeErr)
		if renameErr := os.Rename(path, quarantine); renameErr != nil {
			ls.loadErr = fmt.Errorf("durable learning recovery failed: %v: %w", decodeErr, renameErr)
			logging.Get(logging.CategoryAutopoiesis).Error(
				"Could not preserve the corrupt learning store %s; the next save will overwrite it: %v", path, renameErr)
		}
		return
	}

	for name, learning := range loaded {
		if learning == nil {
			ls.loadErr = fmt.Errorf("nil tool learning %q", name)
			return
		}
		ls.learnings[name] = learning
	}
	if snapshot.Version == 2 {
		ls.executions, ls.feedback = snapshot.Executions, snapshot.Feedback
	}
}

// saveBytes writes the pre-marshaled learnings to disk
func (ls *LearningStore) saveBytes(data []byte) error {
	if err := os.MkdirAll(ls.storePath, 0755); err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(ls.storePath, "tool_learnings.json"), data, 0600)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func containsIssueType(types []IssueType, t IssueType) bool {
	return slices.Contains(types, t)
}

func contains(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

func hasSuggestion(suggestions []ImprovementSuggestion, t SuggestionType) bool {
	for _, s := range suggestions {
		if s.Type == t {
			return true
		}
	}
	return false
}
