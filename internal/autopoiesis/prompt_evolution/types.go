// Package prompt_evolution implements System Prompt Learning (SPL) for codeNERD.
// This package enables automatic evolution of prompt atoms based on execution feedback,
// implementing Karpathy's "third paradigm" of LLM learning.
//
// The core loop is:
// Execute → Evaluate (LLM-as-Judge) → Evolve (Meta-Prompt) → Integrate (JIT Compiler)
//
// Key components:
// - TaskJudge: LLM-based evaluation with explanations
// - FeedbackCollector: Execution outcome recording
// - StrategyStore: Problem-type-specific strategy database
// - AtomGenerator: Automatic atom creation from failures
// - PromptEvolver: Main orchestrator
package prompt_evolution

import (
	"time"

	"codenerd/internal/prompt"
)

// =============================================================================
// ERROR CATEGORIES - Classification of task failures
// =============================================================================

// ErrorCategory represents the type of error that caused a task to fail.
type ErrorCategory string

const (
	// CategoryLogicError indicates wrong approach or algorithm.
	CategoryLogicError ErrorCategory = "LOGIC_ERROR"

	// CategorySyntaxError indicates code syntax issues.
	CategorySyntaxError ErrorCategory = "SYNTAX_ERROR"

	// CategoryAPIMisuse indicates wrong API or library usage.
	CategoryAPIMisuse ErrorCategory = "API_MISUSE"

	// CategoryEdgeCase indicates missing edge case handling.
	CategoryEdgeCase ErrorCategory = "EDGE_CASE"

	// CategoryContextMiss indicates missed relevant context.
	CategoryContextMiss ErrorCategory = "CONTEXT_MISS"

	// CategoryInstructionMiss indicates didn't follow instructions.
	CategoryInstructionMiss ErrorCategory = "INSTRUCTION_MISS"

	// CategoryHallucination indicates made up information.
	CategoryHallucination ErrorCategory = "HALLUCINATION"

	// CategoryCorrect indicates task completed correctly.
	CategoryCorrect ErrorCategory = "CORRECT"
)

// =============================================================================
// PROBLEM TYPES - Classification of tasks for strategy selection
// =============================================================================

// ProblemType represents the category of problem being solved.
type ProblemType string

const (
	ProblemDebugging       ProblemType = "debugging"
	ProblemFeatureCreation ProblemType = "feature_creation"
	ProblemRefactoring     ProblemType = "refactoring"
	ProblemTesting         ProblemType = "testing"
	ProblemDocumentation   ProblemType = "documentation"
	ProblemPerformance     ProblemType = "performance"
	ProblemSecurity        ProblemType = "security"
	ProblemAPIIntegration  ProblemType = "api_integration"
	ProblemDataMigration   ProblemType = "data_migration"
	ProblemConfigSetup     ProblemType = "config_setup"
	ProblemErrorHandling   ProblemType = "error_handling"
	ProblemConcurrency     ProblemType = "concurrency"
	ProblemTypeSystem      ProblemType = "type_system"
	ProblemDependencyMgmt  ProblemType = "dependency_mgmt"
	ProblemCodeReview      ProblemType = "code_review"
	ProblemResearch        ProblemType = "research"
)

// AllProblemTypes returns all defined problem types.
func AllProblemTypes() []ProblemType {
	return []ProblemType{
		ProblemDebugging,
		ProblemFeatureCreation,
		ProblemRefactoring,
		ProblemTesting,
		ProblemDocumentation,
		ProblemPerformance,
		ProblemSecurity,
		ProblemAPIIntegration,
		ProblemDataMigration,
		ProblemConfigSetup,
		ProblemErrorHandling,
		ProblemConcurrency,
		ProblemTypeSystem,
		ProblemDependencyMgmt,
		ProblemCodeReview,
		ProblemResearch,
	}
}

// =============================================================================
// EXECUTION RECORD - What happened during task execution
// =============================================================================

// AgentAction represents a single action taken by the agent.
type AgentAction struct {
	Type        string    `json:"type"`        // e.g., "edit", "read", "run", "search"
	Description string    `json:"description"` // Human-readable description
	Target      string    `json:"target"`      // File path, command, etc.
	Result      string    `json:"result"`      // Outcome of the action
	Success     bool      `json:"success"`
	Timestamp   time.Time `json:"timestamp"`
}

// ExecutionResult represents the outcome of task execution.
type ExecutionResult struct {
	Success     bool              `json:"success"`
	Output      string            `json:"output"`       // Final output or error message
	TestsPassed int               `json:"tests_passed"` // If tests were run
	TestsFailed int               `json:"tests_failed"`
	BuildErrors []string          `json:"build_errors,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ExecutionRecord captures everything about a shard task execution.
type ExecutionRecord struct {
	// Identity
	TaskID    string    `json:"task_id"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`

	// Shard Context
	ShardID   string `json:"shard_id"`
	ShardType string `json:"shard_type"` // e.g., "/coder", "/tester"

	// Task Details
	TaskRequest string `json:"task_request"` // Original user request
	ProblemType string `json:"problem_type"` // Classified problem type

	// Execution Details
	AgentActions    []AgentAction   `json:"agent_actions"`
	ExecutionResult ExecutionResult `json:"execution_result"`
	Duration        time.Duration   `json:"duration"`

	// Prompt Context (which atoms were used)
	PromptManifest *prompt.PromptManifest `json:"prompt_manifest,omitempty"`
	AtomIDs        []string               `json:"atom_ids,omitempty"` // Simplified list

	// Verdict (filled after evaluation)
	Verdict *JudgeVerdict `json:"verdict,omitempty"`
}

// =============================================================================
// JUDGE VERDICT - LLM-as-Judge evaluation result
// =============================================================================

// JudgeVerdict represents the LLM judge's evaluation of a task execution.
type JudgeVerdict struct {
	// Core Verdict
	Verdict     string        `json:"verdict"`     // "PASS" or "FAIL"
	Explanation string        `json:"explanation"` // WHY it passed or failed
	Category    ErrorCategory `json:"category"`    // Error classification
	Confidence  float64       `json:"confidence"`  // 0.0-1.0

	// Learning Signal
	ImprovementRule string `json:"improvement_rule,omitempty"` // Rule for next time

	// Context
	TaskID    string    `json:"task_id"`
	ShardType string    `json:"shard_type"`
	AtomIDs   []string  `json:"atom_ids,omitempty"` // Which atoms were active
	Timestamp time.Time `json:"timestamp"`

	// Tracking
	EvaluatedBy string `json:"evaluated_by"` // Model that did the evaluation
}

// IsFail returns true if the verdict is a failure.
func (v *JudgeVerdict) IsFail() bool {
	return v.Verdict == "FAIL"
}

// IsPass returns true if the verdict is a pass.
func (v *JudgeVerdict) IsPass() bool {
	return v.Verdict == "PASS"
}

// =============================================================================
// STRATEGY - Problem-solving strategy from SPL
// =============================================================================

// Strategy represents a learned problem-solving strategy.
type Strategy struct {
	ID          string      `json:"id"`
	ProblemType ProblemType `json:"problem_type"`
	ShardType   string      `json:"shard_type"`
	Content     string      `json:"content"` // The strategy text

	// Performance Tracking
	SuccessCount int       `json:"success_count"`
	FailureCount int       `json:"failure_count"`
	SuccessRate  float64   `json:"success_rate"`
	LastUsed     time.Time `json:"last_used"`
	LastRefined  time.Time `json:"last_refined"`

	// Metadata
	Version   int       `json:"version"`
	Source    string    `json:"source"` // "generated", "evolved", "manual"
	CreatedAt time.Time `json:"created_at"`
}

// TotalUses returns the total number of times this strategy was used.
func (s *Strategy) TotalUses() int {
	return s.SuccessCount + s.FailureCount
}

// =============================================================================
// GENERATED ATOM - Result of automatic atom generation
// =============================================================================

// GeneratedAtom represents an atom created by the evolution process.
type GeneratedAtom struct {
	Atom *prompt.PromptAtom `json:"atom"`

	// Origin
	Source     string   `json:"source"`      // "failure_analysis", "strategy_refinement"
	SourceIDs  []string `json:"source_ids"`  // Which failures/strategies triggered this
	ShardType  string   `json:"shard_type"`  // Which shard type it's for
	ProblemType string  `json:"problem_type"` // Which problem type it addresses

	// Confidence
	Confidence float64 `json:"confidence"` // 0.0-1.0

	// Tracking
	UsageCount   int       `json:"usage_count"`
	SuccessCount int       `json:"success_count"`
	CreatedAt    time.Time `json:"created_at"`
	PromotedAt   time.Time `json:"promoted_at,omitempty"`
}

// SuccessRate returns the success rate of this atom.
func (ga *GeneratedAtom) SuccessRate() float64 {
	if ga.UsageCount == 0 {
		return 0.5 // Neutral confidence when unused
	}
	return float64(ga.SuccessCount) / float64(ga.UsageCount)
}

// ShouldPromote returns true if the atom should be promoted based on confidence.
func (ga *GeneratedAtom) ShouldPromote(threshold float64) bool {
	return ga.UsageCount >= 3 && ga.SuccessRate() >= threshold
}

// =============================================================================
// EVOLUTION RESULT - Outcome of an evolution cycle
// =============================================================================

// EvolutionResult summarizes what happened during an evolution cycle.
type EvolutionResult struct {
	Timestamp time.Time `json:"timestamp"`
	Duration  time.Duration `json:"duration"`

	// Input
	FailuresAnalyzed int `json:"failures_analyzed"`
	GroupsProcessed  int `json:"groups_processed"`

	// Output
	AtomsGenerated     int      `json:"atoms_generated"`
	AtomIDs            []string `json:"atom_ids,omitempty"`
	StrategiesCreated  int      `json:"strategies_created"`
	StrategiesRefined  int      `json:"strategies_refined"`
	AtomsPromoted      int      `json:"atoms_promoted"`

	// Errors
	Errors []string `json:"errors,omitempty"`
}

// =============================================================================
// EVOLUTION STATS - Overall statistics
// =============================================================================

// EvolutionStats provides high-level statistics about the evolution system.
type EvolutionStats struct {
	// Cycle Counts
	TotalCycles      int `json:"total_cycles"`
	SuccessfulCycles int `json:"successful_cycles"`

	// Atom Counts
	TotalAtomsGenerated int `json:"total_atoms_generated"`
	AtomsPending        int `json:"atoms_pending"`
	AtomsPromoted       int `json:"atoms_promoted"`
	AtomsRejected       int `json:"atoms_rejected"`

	// Strategy Counts
	TotalStrategies   int     `json:"total_strategies"`
	AvgStrategySuccessRate float64 `json:"avg_strategy_success_rate"`

	// Feedback
	TotalExecutionsRecorded int     `json:"total_executions_recorded"`
	TotalFailuresAnalyzed   int     `json:"total_failures_analyzed"`
	OverallSuccessRate      float64 `json:"overall_success_rate"`

	// Timing
	LastEvolutionAt   time.Time `json:"last_evolution_at"`
	AvgCycleDuration  time.Duration `json:"avg_cycle_duration"`
}
