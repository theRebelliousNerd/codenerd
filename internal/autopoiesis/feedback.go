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
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/logging"
)

// =============================================================================
// EXECUTION FEEDBACK - WHAT HAPPENED WHEN THE TOOL RAN?
// =============================================================================

// ExecutionFeedback captures everything about a tool execution
type ExecutionFeedback struct {
	// Identity
	ToolName    string    `json:"tool_name"`
	ExecutionID string    `json:"execution_id"`
	Timestamp   time.Time `json:"timestamp"`

	// Input/Output
	Input      string `json:"input"`
	Output     string `json:"output"`
	OutputSize int    `json:"output_size"`

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
	promptAssembler *articulation.PromptAssembler
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
	pc := &articulation.PromptContext{
		ShardID:   "tool_refiner_" + req.ToolName,
		ShardType: "tool_generator",
	}

	// Assemble system prompt using JIT compiler
	systemPrompt, err := tr.promptAssembler.AssembleSystemPrompt(ctx, pc)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("JIT assembly failed for refinement, falling back: %v", err)
		// Fall back to legacy refinement
		return tr.Refine(ctx, req)
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
func (tr *ToolRefiner) SetPromptAssembler(assembler *articulation.PromptAssembler) {
	tr.promptAssembler = assembler
	tr.jitEnabled = assembler != nil && assembler.JITReady()
	if tr.jitEnabled {
		logging.Autopoiesis("JIT prompt compilation enabled for ToolRefiner")
	}
}

// buildRefinementPrompt creates the prompt for tool improvement
func (tr *ToolRefiner) buildRefinementPrompt(req RefinementRequest) string {
	var sb fmt.Stringer
	var builder interface {
		WriteString(string) (int, error)
		String() string
	}

	// Use a simple string concatenation approach
	prompt := "Improve this tool based on execution feedback:\n\n"
	prompt += fmt.Sprintf("Tool Name: %s\n\n", req.ToolName)
	prompt += fmt.Sprintf("Original Code:\n```go\n%s\n```\n\n", req.OriginalCode)

	// Add feedback summary
	prompt += "Execution Feedback:\n"
	for i, fb := range req.Feedback {
		if i >= 3 {
			prompt += fmt.Sprintf("... and %d more executions\n", len(req.Feedback)-3)
			break
		}
		prompt += fmt.Sprintf("- Execution %d: success=%v, quality=%.2f\n",
			i+1, fb.Success, fb.Quality.Score)
		for _, issue := range fb.Quality.Issues {
			prompt += fmt.Sprintf("  - Issue: %s (%s)\n", issue.Type, issue.Description)
		}
	}

	// Add detected patterns
	if len(req.Patterns) > 0 {
		prompt += "\nRecurring Patterns:\n"
		for _, p := range req.Patterns {
			prompt += fmt.Sprintf("- %s: %d occurrences (%.0f%% confidence)\n",
				p.IssueType, p.Occurrences, p.Confidence*100)
		}
	}

	// Add suggestions
	if len(req.Suggestions) > 0 {
		prompt += "\nSuggested Improvements:\n"
		for _, s := range req.Suggestions {
			prompt += fmt.Sprintf("- %s: %s\n", s.Type, s.Description)
			if s.CodeHint != "" {
				prompt += fmt.Sprintf("  Hint: %s\n", s.CodeHint)
			}
		}
	}

	prompt += `
Return JSON with:
{
  "improved_code": "full improved Go code",
  "changes": ["list of changes made"],
  "expected_gain": 0.0-1.0,
  "test_cases": ["test case descriptions to verify improvements"]
}
`

	// Suppress unused variable warnings
	_ = sb
	_ = builder

	return prompt
}

var refinementSystemPrompt = `You are a Go code optimizer specializing in improving tool reliability and completeness.

When improving tools, focus on:
1. PAGINATION - Always fetch all pages, not just the first
2. LIMITS - Use maximum allowed limits, not defaults
3. RETRIES - Add exponential backoff for transient failures
4. ERROR HANDLING - Handle all error cases gracefully
5. VALIDATION - Validate inputs and outputs

Common anti-patterns to fix:
- Only fetching first page of paginated results
- Using default limit (10) instead of max (100+)
- No retry logic for rate limits or network errors
- Missing error handling for edge cases

Generate clean, idiomatic Go code with proper error handling.`

// =============================================================================
// LEARNING STORE - PERSIST LEARNINGS
// =============================================================================

// LearningStore persists tool learnings for future reference
type LearningStore struct {
	mu        sync.RWMutex
	storePath string
	learnings map[string]*ToolLearning
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
		storePath: storePath,
		learnings: make(map[string]*ToolLearning),
	}
	store.load()
	logging.Autopoiesis("LearningStore initialized with %d existing learnings", len(store.learnings))
	return store
}

// RecordLearning updates learnings for a tool
func (ls *LearningStore) RecordLearning(toolName string, feedback *ExecutionFeedback, patterns []*DetectedPattern) {
	logging.AutopoiesisDebug("Recording learning for tool: %s (success=%v)", toolName, feedback.Success)

	ls.mu.Lock()
	defer ls.mu.Unlock()

	learning, exists := ls.learnings[toolName]
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
		ls.learnings[toolName] = learning
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
		learning.AverageQuality = (learning.AverageQuality*float64(learning.TotalExecutions-1) +
			feedback.Quality.Score) / float64(learning.TotalExecutions)

		// Track known issues
		newIssues := 0
		for _, issue := range feedback.Quality.Issues {
			if !containsIssueType(learning.KnownIssues, issue.Type) {
				learning.KnownIssues = append(learning.KnownIssues, issue.Type)
				newIssues++
			}
		}
		if newIssues > 0 {
			logging.AutopoiesisDebug("Added %d new known issues for %s", newIssues, toolName)
		}
	}

	// Extract anti-patterns from patterns
	newPatterns := 0
	for _, p := range patterns {
		if p.Confidence > 0.7 {
			antiPattern := fmt.Sprintf("%s: %s", p.IssueType, p.PatternID)
			if !contains(learning.AntiPatterns, antiPattern) {
				learning.AntiPatterns = append(learning.AntiPatterns, antiPattern)
				newPatterns++
			}
		}
	}
	if newPatterns > 0 {
		logging.AutopoiesisDebug("Added %d new anti-patterns for %s", newPatterns, toolName)
	}

	learning.UpdatedAt = time.Now()
	ls.save()

	logging.Autopoiesis("Learning recorded for %s: executions=%d, successRate=%.2f, avgQuality=%.2f",
		toolName, learning.TotalExecutions, learning.SuccessRate, learning.AverageQuality)
}

// GetLearning retrieves learnings for a tool
func (ls *LearningStore) GetLearning(toolName string) *ToolLearning {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.learnings[toolName]
}

// GetAllLearnings returns all tool learnings
func (ls *LearningStore) GetAllLearnings() []*ToolLearning {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	learnings := make([]*ToolLearning, 0, len(ls.learnings))
	for _, l := range ls.learnings {
		learnings = append(learnings, l)
	}
	return learnings
}

// GenerateMangleFacts creates Mangle facts from learnings
func (ls *LearningStore) GenerateMangleFacts() []string {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	facts := []string{}
	for _, l := range ls.learnings {
		facts = append(facts, fmt.Sprintf(`tool_learning(%q, %d, %.2f, %.2f).`,
			l.ToolName, l.TotalExecutions, l.SuccessRate, l.AverageQuality))

		for _, issue := range l.KnownIssues {
			facts = append(facts, fmt.Sprintf(`tool_known_issue(%q, %q).`,
				l.ToolName, issue))
		}
	}
	return facts
}

// load reads learnings from disk
func (ls *LearningStore) load() {
	path := filepath.Join(ls.storePath, "tool_learnings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return // File doesn't exist yet
	}

	json.Unmarshal(data, &ls.learnings)
}

// save writes learnings to disk
func (ls *LearningStore) save() {
	if err := os.MkdirAll(ls.storePath, 0755); err != nil {
		return
	}

	path := filepath.Join(ls.storePath, "tool_learnings.json")
	data, err := json.MarshalIndent(ls.learnings, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(path, data, 0644)
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
	for _, it := range types {
		if it == t {
			return true
		}
	}
	return false
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func hasSuggestion(suggestions []ImprovementSuggestion, t SuggestionType) bool {
	for _, s := range suggestions {
		if s.Type == t {
			return true
		}
	}
	return false
}
