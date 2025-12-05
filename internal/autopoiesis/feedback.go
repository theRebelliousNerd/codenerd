// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file implements the feedback and learning system for tool optimization.
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
	"regexp"
	"strings"
	"sync"
	"time"
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
	Duration    time.Duration `json:"duration"`
	MemoryUsed  int64         `json:"memory_used,omitempty"`
	RetryCount  int           `json:"retry_count"`

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
	Accepted    bool      `json:"accepted"`     // Did user accept the output?
	Modified    bool      `json:"modified"`     // Did user modify/correct it?
	Reran       bool      `json:"reran"`        // Did user ask to re-run?
	Complaint   string    `json:"complaint"`    // User's complaint if any
	Improvement string    `json:"improvement"`  // What user wanted instead
	Timestamp   time.Time `json:"timestamp"`
}

// =============================================================================
// QUALITY ASSESSMENT - HOW GOOD WAS THE OUTPUT?
// =============================================================================

// QualityAssessment evaluates tool output quality
type QualityAssessment struct {
	// Overall score (0.0 - 1.0)
	Score float64 `json:"score"`

	// Dimension scores
	Completeness float64 `json:"completeness"` // Did we get all available data?
	Accuracy     float64 `json:"accuracy"`     // Was the output correct?
	Efficiency   float64 `json:"efficiency"`   // Resource efficiency
	Relevance    float64 `json:"relevance"`    // Was output relevant to intent?

	// Specific issues detected
	Issues []QualityIssue `json:"issues"`

	// Improvement suggestions
	Suggestions []ImprovementSuggestion `json:"suggestions"`

	// Metadata
	EvaluatedAt time.Time `json:"evaluated_at"`
	EvaluatedBy string    `json:"evaluated_by"` // "heuristic", "llm", "user"
}

// QualityIssue describes a specific quality problem
type QualityIssue struct {
	Type        IssueType `json:"type"`
	Severity    float64   `json:"severity"`    // 0.0 - 1.0
	Description string    `json:"description"`
	Evidence    string    `json:"evidence"`    // What in the output shows this?
	Fixable     bool      `json:"fixable"`     // Can we auto-fix this?
}

// IssueType categorizes quality issues
type IssueType string

const (
	IssueIncomplete     IssueType = "incomplete"      // Missing data (like your Context7 example)
	IssueInaccurate     IssueType = "inaccurate"      // Wrong data
	IssueSlow           IssueType = "slow"            // Too slow
	IssueResourceHeavy  IssueType = "resource_heavy"  // Used too much memory/CPU
	IssuePoorFormat     IssueType = "poor_format"     // Output format issues
	IssuePartialFailure IssueType = "partial_failure" // Partially worked
	IssuePagination     IssueType = "pagination"      // Didn't handle pagination
	IssueRateLimit      IssueType = "rate_limit"      // Hit rate limits
	IssueAuth           IssueType = "auth"            // Authentication issues
)

// ImprovementSuggestion describes how to improve the tool
type ImprovementSuggestion struct {
	Type        SuggestionType `json:"type"`
	Priority    float64        `json:"priority"`
	Description string         `json:"description"`
	CodeHint    string         `json:"code_hint,omitempty"` // Specific code change
}

// SuggestionType categorizes improvement suggestions
type SuggestionType string

const (
	SuggestAddPagination   SuggestionType = "add_pagination"
	SuggestIncreaseLimit   SuggestionType = "increase_limit"
	SuggestAddRetry        SuggestionType = "add_retry"
	SuggestCaching         SuggestionType = "add_caching"
	SuggestBatching        SuggestionType = "add_batching"
	SuggestParallel        SuggestionType = "parallelize"
	SuggestBetterParsing   SuggestionType = "better_parsing"
	SuggestErrorHandling   SuggestionType = "better_errors"
	SuggestValidation      SuggestionType = "add_validation"
)

// =============================================================================
// TOOL QUALITY PROFILE - LLM-DEFINED EXPECTATIONS PER TOOL
// =============================================================================
// Each tool has different expectations. A background indexer that takes
// 5 minutes is fine; a calculator that takes 5 minutes is broken.
// The LLM defines these expectations during tool generation.

// ToolQualityProfile defines expected quality dimensions for a specific tool
type ToolQualityProfile struct {
	// Identity
	ToolName    string    `json:"tool_name"`
	ToolType    ToolType  `json:"tool_type"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`

	// Performance Expectations
	Performance PerformanceExpectations `json:"performance"`

	// Output Expectations
	Output OutputExpectations `json:"output"`

	// Usage Pattern
	UsagePattern UsagePattern `json:"usage_pattern"`

	// Caching Configuration
	Caching CachingConfig `json:"caching"`

	// Custom Dimensions (tool-specific metrics)
	CustomDimensions []CustomDimension `json:"custom_dimensions,omitempty"`
}

// ToolType classifies the tool for appropriate evaluation
type ToolType string

const (
	ToolTypeQuickCalculation   ToolType = "quick_calculation"   // < 1s, simple computation
	ToolTypeDataFetch          ToolType = "data_fetch"          // API call, may paginate
	ToolTypeBackgroundTask     ToolType = "background_task"     // Long-running, minutes OK
	ToolTypeRecursiveAnalysis  ToolType = "recursive_analysis"  // Codebase traversal, slow OK
	ToolTypeRealTimeQuery      ToolType = "realtime_query"      // Must be fast, frequent
	ToolTypeOneTimeSetup       ToolType = "one_time_setup"      // Run once, can be slow
	ToolTypeBatchProcessor     ToolType = "batch_processor"     // Processes many items
	ToolTypeMonitor            ToolType = "monitor"             // Called repeatedly for status
)

// PerformanceExpectations defines expected timing for this tool
type PerformanceExpectations struct {
	// Duration expectations
	ExpectedDurationMin time.Duration `json:"expected_duration_min"` // Faster than this is suspicious
	ExpectedDurationMax time.Duration `json:"expected_duration_max"` // Slower than this is a problem
	AcceptableDuration  time.Duration `json:"acceptable_duration"`   // Target duration
	TimeoutDuration     time.Duration `json:"timeout_duration"`      // When to give up

	// Resource expectations
	MaxMemoryMB       int64   `json:"max_memory_mb,omitempty"`
	ExpectedAPIcalls  int     `json:"expected_api_calls,omitempty"`   // Expected number of external calls
	MaxRetries        int     `json:"max_retries"`                    // How many retries are acceptable

	// Scaling behavior
	ScalesWithInputSize bool    `json:"scales_with_input_size"` // Duration scales with input?
	ScalingFactor       float64 `json:"scaling_factor,omitempty"` // ms per unit of input size
}

// OutputExpectations defines what output should look like
type OutputExpectations struct {
	// Size expectations
	ExpectedMinSize   int `json:"expected_min_size"`    // Smaller is suspicious
	ExpectedMaxSize   int `json:"expected_max_size"`    // Larger might indicate issue
	ExpectedTypicalSize int `json:"expected_typical_size"` // Normal output size

	// Content expectations
	ExpectedFormat    string   `json:"expected_format"`     // json, text, csv, etc.
	RequiredFields    []string `json:"required_fields,omitempty"` // Fields that must be present
	MustContain       []string `json:"must_contain,omitempty"`    // Strings that must appear
	MustNotContain    []string `json:"must_not_contain,omitempty"` // Strings that indicate failure

	// Pagination expectations
	ExpectsPagination bool `json:"expects_pagination"`   // Should we paginate?
	ExpectedPages     int  `json:"expected_pages,omitempty"` // How many pages expected

	// Completeness criteria
	CompletenessCheck string `json:"completeness_check,omitempty"` // How to verify completeness
}

// UsagePattern describes how this tool is typically used
type UsagePattern struct {
	Frequency        UsageFrequency `json:"frequency"`          // How often called
	CallsPerSession  int            `json:"calls_per_session"`  // Expected calls per session
	IsIdempotent     bool           `json:"is_idempotent"`      // Same input = same output?
	HasSideEffects   bool           `json:"has_side_effects"`   // Modifies external state?
	DependsOnExternal bool          `json:"depends_on_external"` // Needs external service?
}

// UsageFrequency describes how often a tool is called
type UsageFrequency string

const (
	FrequencyOnce       UsageFrequency = "once"        // Run once per task
	FrequencyOccasional UsageFrequency = "occasional"  // Few times per session
	FrequencyFrequent   UsageFrequency = "frequent"    // Many times per session
	FrequencyConstant   UsageFrequency = "constant"    // Called continuously
)

// CachingConfig defines caching behavior
type CachingConfig struct {
	Cacheable     bool          `json:"cacheable"`       // Can results be cached?
	CacheDuration time.Duration `json:"cache_duration"`  // How long to cache
	CacheKey      string        `json:"cache_key"`       // What makes cache key unique
	InvalidateOn  []string      `json:"invalidate_on,omitempty"` // Events that invalidate cache
}

// CustomDimension allows tool-specific quality metrics
type CustomDimension struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	ExpectedValue  float64 `json:"expected_value"`
	Tolerance      float64 `json:"tolerance"`       // +/- acceptable range
	Weight         float64 `json:"weight"`          // How much this affects overall score
	ExtractPattern string  `json:"extract_pattern"` // Regex to extract value from output
}

// ProfileStore manages tool quality profiles
type ProfileStore struct {
	mu       sync.RWMutex
	profiles map[string]*ToolQualityProfile // ToolName -> Profile
	storePath string
}

// NewProfileStore creates a new profile store
func NewProfileStore(storePath string) *ProfileStore {
	store := &ProfileStore{
		profiles: make(map[string]*ToolQualityProfile),
		storePath: storePath,
	}
	store.load()
	return store
}

// GetProfile retrieves a tool's quality profile
func (ps *ProfileStore) GetProfile(toolName string) *ToolQualityProfile {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.profiles[toolName]
}

// SetProfile stores a tool's quality profile
func (ps *ProfileStore) SetProfile(profile *ToolQualityProfile) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.profiles[profile.ToolName] = profile
	ps.save()
}

// GetDefaultProfile returns a default profile based on tool type
func GetDefaultProfile(toolName string, toolType ToolType) *ToolQualityProfile {
	profile := &ToolQualityProfile{
		ToolName:  toolName,
		ToolType:  toolType,
		CreatedAt: time.Now(),
	}

	// Set defaults based on tool type
	switch toolType {
	case ToolTypeQuickCalculation:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 1 * time.Millisecond,
			ExpectedDurationMax: 1 * time.Second,
			AcceptableDuration:  100 * time.Millisecond,
			TimeoutDuration:     5 * time.Second,
			MaxRetries:          0,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     1,
			ExpectedMaxSize:     1024,
			ExpectedTypicalSize: 100,
			ExpectsPagination:   false,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:       FrequencyFrequent,
			CallsPerSession: 50,
			IsIdempotent:    true,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 5 * time.Minute,
		}

	case ToolTypeDataFetch:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 100 * time.Millisecond,
			ExpectedDurationMax: 30 * time.Second,
			AcceptableDuration:  5 * time.Second,
			TimeoutDuration:     60 * time.Second,
			ExpectedAPIcalls:    1,
			MaxRetries:          3,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     100,
			ExpectedMaxSize:     1024 * 1024, // 1MB
			ExpectedTypicalSize: 10 * 1024,   // 10KB
			ExpectsPagination:   true,
			ExpectedPages:       5,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:         FrequencyOccasional,
			CallsPerSession:   5,
			DependsOnExternal: true,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 15 * time.Minute,
		}

	case ToolTypeBackgroundTask:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 1 * time.Second,
			ExpectedDurationMax: 10 * time.Minute,
			AcceptableDuration:  2 * time.Minute,
			TimeoutDuration:     30 * time.Minute,
			MaxRetries:          2,
			ScalesWithInputSize: true,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     10,
			ExpectedMaxSize:     10 * 1024 * 1024, // 10MB
			ExpectedTypicalSize: 100 * 1024,       // 100KB
		}
		profile.UsagePattern = UsagePattern{
			Frequency:       FrequencyOnce,
			CallsPerSession: 1,
			HasSideEffects:  true,
		}
		profile.Caching = CachingConfig{
			Cacheable: false,
		}

	case ToolTypeRecursiveAnalysis:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 5 * time.Second,
			ExpectedDurationMax: 15 * time.Minute,
			AcceptableDuration:  3 * time.Minute,
			TimeoutDuration:     30 * time.Minute,
			MaxRetries:          1,
			ScalesWithInputSize: true,
			ScalingFactor:       10, // 10ms per file
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     1024,
			ExpectedMaxSize:     50 * 1024 * 1024, // 50MB
			ExpectedTypicalSize: 500 * 1024,       // 500KB
			ExpectsPagination:   false,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:       FrequencyOnce,
			CallsPerSession: 1,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 1 * time.Hour,
		}

	case ToolTypeRealTimeQuery:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 1 * time.Millisecond,
			ExpectedDurationMax: 500 * time.Millisecond,
			AcceptableDuration:  100 * time.Millisecond,
			TimeoutDuration:     2 * time.Second,
			MaxRetries:          1,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     10,
			ExpectedMaxSize:     10 * 1024, // 10KB
			ExpectedTypicalSize: 1024,      // 1KB
		}
		profile.UsagePattern = UsagePattern{
			Frequency:         FrequencyConstant,
			CallsPerSession:   100,
			IsIdempotent:      true,
			DependsOnExternal: true,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 1 * time.Minute,
		}

	case ToolTypeMonitor:
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 10 * time.Millisecond,
			ExpectedDurationMax: 2 * time.Second,
			AcceptableDuration:  500 * time.Millisecond,
			TimeoutDuration:     5 * time.Second,
			MaxRetries:          2,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     50,
			ExpectedMaxSize:     5 * 1024, // 5KB
			ExpectedTypicalSize: 500,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:         FrequencyConstant,
			CallsPerSession:   200,
			IsIdempotent:      true,
			DependsOnExternal: true,
		}
		profile.Caching = CachingConfig{
			Cacheable:     true,
			CacheDuration: 30 * time.Second,
		}

	default:
		// Generic defaults
		profile.Performance = PerformanceExpectations{
			ExpectedDurationMin: 100 * time.Millisecond,
			ExpectedDurationMax: 30 * time.Second,
			AcceptableDuration:  5 * time.Second,
			TimeoutDuration:     60 * time.Second,
			MaxRetries:          2,
		}
		profile.Output = OutputExpectations{
			ExpectedMinSize:     10,
			ExpectedMaxSize:     1024 * 1024,
			ExpectedTypicalSize: 10 * 1024,
		}
		profile.UsagePattern = UsagePattern{
			Frequency:       FrequencyOccasional,
			CallsPerSession: 10,
		}
	}

	return profile
}

// persistence
func (ps *ProfileStore) load() {
	path := filepath.Join(ps.storePath, "quality_profiles.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &ps.profiles)
}

func (ps *ProfileStore) save() {
	if err := os.MkdirAll(ps.storePath, 0755); err != nil {
		return
	}
	path := filepath.Join(ps.storePath, "quality_profiles.json")
	data, _ := json.MarshalIndent(ps.profiles, "", "  ")
	os.WriteFile(path, data, 0644)
}

// =============================================================================
// QUALITY EVALUATOR - ASSESS TOOL OUTPUT
// =============================================================================

// QualityEvaluator assesses the quality of tool executions
type QualityEvaluator struct {
	client           LLMClient
	heuristicRules   []HeuristicRule
	completenessHints map[string]CompletenessHint
	profileStore     *ProfileStore // Tool-specific quality profiles
}

// HeuristicRule is a rule for detecting quality issues
type HeuristicRule struct {
	Name        string
	IssueType   IssueType
	Pattern     *regexp.Regexp
	Severity    float64
	Description string
}

// CompletenessHint provides context-specific completeness expectations
type CompletenessHint struct {
	ToolPattern       string  // Regex for tool name
	ExpectedMinSize   int     // Minimum expected output size
	ExpectedMaxPages  int     // Expected pagination depth
	PaginationPattern string  // How to detect pagination in output
	TotalPattern      string  // How to detect total available
}

// NewQualityEvaluator creates a new evaluator
func NewQualityEvaluator(client LLMClient, profileStore *ProfileStore) *QualityEvaluator {
	return &QualityEvaluator{
		client:           client,
		heuristicRules:   defaultHeuristicRules(),
		completenessHints: defaultCompletenessHints(),
		profileStore:     profileStore,
	}
}

// SetProfileStore sets the profile store (for delayed initialization)
func (qe *QualityEvaluator) SetProfileStore(store *ProfileStore) {
	qe.profileStore = store
}

// defaultHeuristicRules returns built-in quality detection rules
func defaultHeuristicRules() []HeuristicRule {
	return []HeuristicRule{
		{
			Name:        "pagination_truncated",
			IssueType:   IssuePagination,
			Pattern:     regexp.MustCompile(`(?i)(page\s*1\s*of\s*\d+|showing\s+\d+\s*-\s*\d+|has_more.*true|next_page|truncated)`),
			Severity:    0.7,
			Description: "Output appears to be paginated but only first page was fetched",
		},
		{
			Name:        "partial_results",
			IssueType:   IssueIncomplete,
			Pattern:     regexp.MustCompile(`(?i)(partial|incomplete|limited|max.*reached|rate.*limit)`),
			Severity:    0.6,
			Description: "Output indicates partial/limited results",
		},
		{
			Name:        "error_in_output",
			IssueType:   IssuePartialFailure,
			Pattern:     regexp.MustCompile(`(?i)(error|failed|exception|timeout|connection.*refused)`),
			Severity:    0.8,
			Description: "Output contains error indicators",
		},
		{
			Name:        "empty_or_minimal",
			IssueType:   IssueIncomplete,
			Pattern:     regexp.MustCompile(`^\s*(\[\s*\]|\{\s*\}|null|none|empty)\s*$`),
			Severity:    0.9,
			Description: "Output is empty or minimal",
		},
		{
			Name:        "rate_limited",
			IssueType:   IssueRateLimit,
			Pattern:     regexp.MustCompile(`(?i)(rate\s*limit|429|too\s*many\s*requests|throttl)`),
			Severity:    0.7,
			Description: "Output indicates rate limiting",
		},
	}
}

// defaultCompletenessHints returns hints for common tool types
func defaultCompletenessHints() map[string]CompletenessHint {
	return map[string]CompletenessHint{
		"api_docs": {
			ToolPattern:       "(?i)(docs|documentation|api|context7)",
			ExpectedMinSize:   10000, // Expect at least 10KB of docs
			ExpectedMaxPages:  10,
			PaginationPattern: `page.*of|has_more|next`,
			TotalPattern:      `total.*:\s*(\d+)`,
		},
		"search": {
			ToolPattern:       "(?i)(search|query|find)",
			ExpectedMinSize:   1000,
			ExpectedMaxPages:  5,
			PaginationPattern: `results\s+\d+\s*-\s*\d+`,
			TotalPattern:      `(\d+)\s*results`,
		},
		"list": {
			ToolPattern:       "(?i)(list|get_all|fetch)",
			ExpectedMinSize:   500,
			ExpectedMaxPages:  10,
			PaginationPattern: `showing.*of`,
			TotalPattern:      `total.*(\d+)`,
		},
	}
}

// Evaluate performs quality assessment on a tool execution
func (qe *QualityEvaluator) Evaluate(ctx context.Context, feedback *ExecutionFeedback) *QualityAssessment {
	assessment := &QualityAssessment{
		EvaluatedAt: time.Now(),
		EvaluatedBy: "heuristic",
		Issues:      []QualityIssue{},
		Suggestions: []ImprovementSuggestion{},
	}

	// If execution failed, that's the main issue
	if !feedback.Success {
		assessment.Score = 0.1
		assessment.Issues = append(assessment.Issues, QualityIssue{
			Type:        IssuePartialFailure,
			Severity:    1.0,
			Description: "Tool execution failed",
			Evidence:    feedback.ErrorMsg,
			Fixable:     true,
		})
		return assessment
	}

	// Apply heuristic rules
	for _, rule := range qe.heuristicRules {
		if rule.Pattern.MatchString(feedback.Output) {
			assessment.Issues = append(assessment.Issues, QualityIssue{
				Type:        rule.IssueType,
				Severity:    rule.Severity,
				Description: rule.Description,
				Evidence:    extractMatch(feedback.Output, rule.Pattern),
				Fixable:     true,
			})
		}
	}

	// Check completeness based on tool type hints
	completenessScore := qe.evaluateCompleteness(feedback)
	assessment.Completeness = completenessScore

	// Efficiency based on duration
	assessment.Efficiency = qe.evaluateEfficiency(feedback)

	// Calculate overall score
	issueImpact := 0.0
	for _, issue := range assessment.Issues {
		issueImpact += issue.Severity * 0.2 // Each issue can reduce score by up to 20%
	}

	assessment.Score = clamp(
		(assessment.Completeness*0.4 + assessment.Efficiency*0.2 + 0.4) - issueImpact,
		0.0, 1.0,
	)

	// Generate improvement suggestions based on issues
	assessment.Suggestions = qe.generateSuggestions(assessment.Issues, feedback)

	return assessment
}

// EvaluateWithProfile performs profile-aware quality assessment
func (qe *QualityEvaluator) EvaluateWithProfile(ctx context.Context, feedback *ExecutionFeedback, profile *ToolQualityProfile) *QualityAssessment {
	assessment := &QualityAssessment{
		EvaluatedAt: time.Now(),
		EvaluatedBy: "profile_heuristic",
		Issues:      []QualityIssue{},
		Suggestions: []ImprovementSuggestion{},
	}

	// If execution failed, that's the main issue
	if !feedback.Success {
		assessment.Score = 0.1
		assessment.Issues = append(assessment.Issues, QualityIssue{
			Type:        IssuePartialFailure,
			Severity:    1.0,
			Description: "Tool execution failed",
			Evidence:    feedback.ErrorMsg,
			Fixable:     true,
		})
		return assessment
	}

	// Profile-aware efficiency evaluation
	assessment.Efficiency = qe.evaluateEfficiencyWithProfile(feedback, profile)

	// Profile-aware completeness evaluation
	assessment.Completeness = qe.evaluateCompletenessWithProfile(feedback, profile)

	// Profile-aware output validation
	outputIssues := qe.validateOutputWithProfile(feedback, profile)
	assessment.Issues = append(assessment.Issues, outputIssues...)

	// Apply standard heuristic rules
	for _, rule := range qe.heuristicRules {
		if rule.Pattern.MatchString(feedback.Output) {
			assessment.Issues = append(assessment.Issues, QualityIssue{
				Type:        rule.IssueType,
				Severity:    rule.Severity,
				Description: rule.Description,
				Evidence:    extractMatch(feedback.Output, rule.Pattern),
				Fixable:     true,
			})
		}
	}

	// Evaluate custom dimensions
	customScore := qe.evaluateCustomDimensions(feedback, profile)

	// Calculate overall score using profile weights
	issueImpact := 0.0
	for _, issue := range assessment.Issues {
		issueImpact += issue.Severity * 0.15 // Each issue reduces score
	}

	// Weighted score based on tool type
	baseScore := assessment.Completeness*0.35 + assessment.Efficiency*0.25 +
		assessment.Accuracy*0.2 + customScore*0.2

	assessment.Score = clamp(baseScore-issueImpact, 0.0, 1.0)

	// Generate profile-aware suggestions
	assessment.Suggestions = qe.generateProfileAwareSuggestions(assessment.Issues, feedback, profile)

	return assessment
}

// evaluateEfficiencyWithProfile scores efficiency against profile expectations
func (qe *QualityEvaluator) evaluateEfficiencyWithProfile(feedback *ExecutionFeedback, profile *ToolQualityProfile) float64 {
	if profile == nil {
		return qe.evaluateEfficiency(feedback)
	}

	perf := profile.Performance
	duration := feedback.Duration

	// Too fast might indicate incomplete execution
	if perf.ExpectedDurationMin > 0 && duration < perf.ExpectedDurationMin {
		return 0.5 // Suspiciously fast
	}

	// Within acceptable range
	if duration <= perf.AcceptableDuration {
		return 1.0
	}

	// Within expected max
	if duration <= perf.ExpectedDurationMax {
		// Linear degradation from acceptable to max
		ratio := float64(duration-perf.AcceptableDuration) / float64(perf.ExpectedDurationMax-perf.AcceptableDuration)
		return 1.0 - (ratio * 0.3) // 1.0 to 0.7
	}

	// Exceeds expected max but within timeout
	if duration <= perf.TimeoutDuration {
		// Degradation from 0.7 to 0.3
		ratio := float64(duration-perf.ExpectedDurationMax) / float64(perf.TimeoutDuration-perf.ExpectedDurationMax)
		return 0.7 - (ratio * 0.4)
	}

	// Beyond timeout
	return 0.1
}

// evaluateCompletenessWithProfile checks if output meets profile expectations
func (qe *QualityEvaluator) evaluateCompletenessWithProfile(feedback *ExecutionFeedback, profile *ToolQualityProfile) float64 {
	if profile == nil {
		return qe.evaluateCompleteness(feedback)
	}

	output := profile.Output
	outputSize := feedback.OutputSize

	// Check minimum size
	if output.ExpectedMinSize > 0 && outputSize < output.ExpectedMinSize {
		ratio := float64(outputSize) / float64(output.ExpectedMinSize)
		return clamp(ratio, 0.1, 0.7) // Below minimum = 0.1 to 0.7
	}

	// Check if output is within expected range
	if outputSize >= output.ExpectedMinSize && outputSize <= output.ExpectedMaxSize {
		// Perfect if around typical size
		if output.ExpectedTypicalSize > 0 {
			deviation := float64(outputSize-output.ExpectedTypicalSize) / float64(output.ExpectedTypicalSize)
			if deviation < 0 {
				deviation = -deviation
			}
			if deviation < 0.5 {
				return 1.0 // Within 50% of typical
			}
			return 0.9 // Further from typical but in range
		}
		return 0.95
	}

	// Output too large - might indicate issues
	if outputSize > output.ExpectedMaxSize {
		return 0.7 // Suspicious but not necessarily bad
	}

	return 0.8
}

// validateOutputWithProfile checks output against profile expectations
func (qe *QualityEvaluator) validateOutputWithProfile(feedback *ExecutionFeedback, profile *ToolQualityProfile) []QualityIssue {
	issues := []QualityIssue{}
	if profile == nil {
		return issues
	}

	output := profile.Output

	// Check required fields (for JSON output)
	if output.ExpectedFormat == "json" && len(output.RequiredFields) > 0 {
		for _, field := range output.RequiredFields {
			if !strings.Contains(feedback.Output, `"`+field+`"`) {
				issues = append(issues, QualityIssue{
					Type:        IssueIncomplete,
					Severity:    0.6,
					Description: fmt.Sprintf("Missing required field: %s", field),
					Evidence:    fmt.Sprintf("Field '%s' not found in JSON output", field),
					Fixable:     true,
				})
			}
		}
	}

	// Check must-contain patterns
	for _, pattern := range output.MustContain {
		if !strings.Contains(feedback.Output, pattern) {
			issues = append(issues, QualityIssue{
				Type:        IssueIncomplete,
				Severity:    0.5,
				Description: fmt.Sprintf("Missing expected content: %s", pattern),
				Fixable:     true,
			})
		}
	}

	// Check must-not-contain patterns (error indicators)
	for _, pattern := range output.MustNotContain {
		if strings.Contains(feedback.Output, pattern) {
			issues = append(issues, QualityIssue{
				Type:        IssuePartialFailure,
				Severity:    0.7,
				Description: fmt.Sprintf("Output contains error indicator: %s", pattern),
				Evidence:    pattern,
				Fixable:     true,
			})
		}
	}

	// Check pagination expectations
	if output.ExpectsPagination {
		// Look for signs that pagination wasn't handled
		paginationIndicators := regexp.MustCompile(`(?i)(page\s*1\s*of|has_more.*true|next_page|offset.*0)`)
		if paginationIndicators.MatchString(feedback.Output) {
			issues = append(issues, QualityIssue{
				Type:        IssuePagination,
				Severity:    0.7,
				Description: "Pagination expected but only first page may have been fetched",
				Evidence:    extractMatch(feedback.Output, paginationIndicators),
				Fixable:     true,
			})
		}
	}

	return issues
}

// evaluateCustomDimensions evaluates tool-specific quality metrics
func (qe *QualityEvaluator) evaluateCustomDimensions(feedback *ExecutionFeedback, profile *ToolQualityProfile) float64 {
	if profile == nil || len(profile.CustomDimensions) == 0 {
		return 1.0 // No custom dimensions, assume good
	}

	totalWeight := 0.0
	weightedScore := 0.0

	for _, dim := range profile.CustomDimensions {
		totalWeight += dim.Weight

		// Try to extract value using pattern
		if dim.ExtractPattern != "" {
			pattern := regexp.MustCompile(dim.ExtractPattern)
			match := pattern.FindStringSubmatch(feedback.Output)
			if len(match) > 1 {
				// Try to parse as float
				var extractedValue float64
				if _, err := fmt.Sscanf(match[1], "%f", &extractedValue); err == nil {
					// Check if within tolerance
					deviation := extractedValue - dim.ExpectedValue
					if deviation < 0 {
						deviation = -deviation
					}
					if deviation <= dim.Tolerance {
						weightedScore += dim.Weight * 1.0
					} else {
						// Score based on how far outside tolerance
						outsideBy := deviation - dim.Tolerance
						penalty := outsideBy / dim.ExpectedValue
						score := clamp(1.0-penalty, 0.0, 1.0)
						weightedScore += dim.Weight * score
					}
					continue
				}
			}
		}

		// Pattern didn't match or couldn't extract - neutral score
		weightedScore += dim.Weight * 0.7
	}

	if totalWeight == 0 {
		return 1.0
	}
	return weightedScore / totalWeight
}

// generateProfileAwareSuggestions creates suggestions based on profile context
func (qe *QualityEvaluator) generateProfileAwareSuggestions(issues []QualityIssue, feedback *ExecutionFeedback, profile *ToolQualityProfile) []ImprovementSuggestion {
	suggestions := qe.generateSuggestions(issues, feedback)

	if profile == nil {
		return suggestions
	}

	// Add profile-specific suggestions
	usage := profile.UsagePattern

	// If frequently called and slow, suggest caching more aggressively
	if usage.Frequency == FrequencyFrequent || usage.Frequency == FrequencyConstant {
		if feedback.Duration > profile.Performance.AcceptableDuration {
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestCaching,
				Priority:    0.9,
				Description: fmt.Sprintf("Tool is frequently called but takes %.1fs - add aggressive caching", feedback.Duration.Seconds()),
				CodeHint:    fmt.Sprintf("Cache results for %v based on cache key: %s", profile.Caching.CacheDuration, profile.Caching.CacheKey),
			})
		}
	}

	// If idempotent but output varies, flag as issue
	if usage.IsIdempotent && profile.Caching.Cacheable {
		// Could compare with previous outputs for same input
		suggestions = append(suggestions, ImprovementSuggestion{
			Type:        SuggestValidation,
			Priority:    0.5,
			Description: "Tool is idempotent - consider adding output validation",
		})
	}

	// If depends on external and slow, suggest retry/timeout handling
	if usage.DependsOnExternal && profile.Performance.MaxRetries > 0 {
		if feedback.RetryCount == 0 && !feedback.Success {
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestAddRetry,
				Priority:    0.8,
				Description: fmt.Sprintf("External dependency failure - add retry logic (max %d retries)", profile.Performance.MaxRetries),
			})
		}
	}

	return suggestions
}

// evaluateCompleteness checks if we got all available data
func (qe *QualityEvaluator) evaluateCompleteness(feedback *ExecutionFeedback) float64 {
	// Find matching completeness hint
	var hint *CompletenessHint
	for _, h := range qe.completenessHints {
		if matched, _ := regexp.MatchString(h.ToolPattern, feedback.ToolName); matched {
			hint = &h
			break
		}
	}

	if hint == nil {
		// No specific hint, use generic assessment
		if feedback.OutputSize < 100 {
			return 0.3 // Suspiciously small
		}
		return 0.8 // Assume OK
	}

	// Check against expected minimum
	if feedback.OutputSize < hint.ExpectedMinSize {
		ratio := float64(feedback.OutputSize) / float64(hint.ExpectedMinSize)
		return clamp(ratio, 0.1, 0.9)
	}

	// Check for pagination indicators
	if hint.PaginationPattern != "" {
		if matched, _ := regexp.MatchString(hint.PaginationPattern, feedback.Output); matched {
			// Found pagination - check if we got all pages
			if totalPattern := regexp.MustCompile(hint.TotalPattern); totalPattern != nil {
				// Could extract total and compare, for now penalize
				return 0.5 // Pagination detected but may not be complete
			}
		}
	}

	return 0.9 // Looks complete
}

// evaluateEfficiency scores execution efficiency
func (qe *QualityEvaluator) evaluateEfficiency(feedback *ExecutionFeedback) float64 {
	// Simple duration-based efficiency
	// < 1s = excellent, 1-5s = good, 5-30s = acceptable, >30s = poor
	seconds := feedback.Duration.Seconds()
	switch {
	case seconds < 1:
		return 1.0
	case seconds < 5:
		return 0.8
	case seconds < 30:
		return 0.6
	default:
		return 0.3
	}
}

// generateSuggestions creates improvement suggestions from detected issues
func (qe *QualityEvaluator) generateSuggestions(issues []QualityIssue, feedback *ExecutionFeedback) []ImprovementSuggestion {
	suggestions := []ImprovementSuggestion{}

	for _, issue := range issues {
		switch issue.Type {
		case IssuePagination:
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestAddPagination,
				Priority:    0.9,
				Description: "Add pagination handling to fetch all pages",
				CodeHint:    "Add loop to follow next_page/has_more until exhausted",
			})
		case IssueIncomplete:
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestIncreaseLimit,
				Priority:    0.8,
				Description: "Increase result limit or add pagination",
				CodeHint:    "Set limit/max_results to higher value or maximum allowed",
			})
		case IssueRateLimit:
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestAddRetry,
				Priority:    0.7,
				Description: "Add exponential backoff retry logic",
				CodeHint:    "Implement retry with backoff when rate limited",
			})
		case IssueSlow:
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestCaching,
				Priority:    0.5,
				Description: "Add caching for repeated requests",
			})
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestParallel,
				Priority:    0.6,
				Description: "Parallelize independent requests",
			})
		}
	}

	return suggestions
}

// EvaluateWithLLM uses LLM for deeper quality assessment
func (qe *QualityEvaluator) EvaluateWithLLM(ctx context.Context, feedback *ExecutionFeedback) (*QualityAssessment, error) {
	// First get heuristic assessment
	assessment := qe.Evaluate(ctx, feedback)

	// Then enhance with LLM analysis
	prompt := fmt.Sprintf(`Analyze this tool execution and assess its quality.

Tool: %s
Input: %s
Output (truncated to 2000 chars): %s
Duration: %v
Success: %v

Existing issues detected: %v

Please provide:
1. Completeness score (0.0-1.0) - Did the tool fetch ALL available data?
2. Accuracy score (0.0-1.0) - Is the output correct and well-formed?
3. Additional issues not detected by heuristics
4. Specific improvement suggestions with code hints

Focus especially on:
- Pagination: Did it only get the first page when more were available?
- Limits: Did it use default limits instead of maximum?
- Error handling: Did it properly handle edge cases?

Return JSON:
{
  "completeness": 0.0-1.0,
  "accuracy": 0.0-1.0,
  "relevance": 0.0-1.0,
  "additional_issues": [{"type": "...", "description": "...", "severity": 0.0-1.0}],
  "suggestions": [{"type": "...", "description": "...", "code_hint": "..."}],
  "reasoning": "..."
}`,
		feedback.ToolName,
		truncate(feedback.Input, 500),
		truncate(feedback.Output, 2000),
		feedback.Duration,
		feedback.Success,
		assessment.Issues,
	)

	resp, err := qe.client.Complete(ctx, prompt)
	if err != nil {
		return assessment, nil // Return heuristic assessment on LLM failure
	}

	// Parse LLM response and merge with heuristic assessment
	var llmResult struct {
		Completeness     float64 `json:"completeness"`
		Accuracy         float64 `json:"accuracy"`
		Relevance        float64 `json:"relevance"`
		AdditionalIssues []struct {
			Type        string  `json:"type"`
			Description string  `json:"description"`
			Severity    float64 `json:"severity"`
		} `json:"additional_issues"`
		Suggestions []struct {
			Type        string `json:"type"`
			Description string `json:"description"`
			CodeHint    string `json:"code_hint"`
		} `json:"suggestions"`
	}

	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &llmResult); err == nil {
		// Merge LLM assessment
		assessment.Completeness = (assessment.Completeness + llmResult.Completeness) / 2
		assessment.Accuracy = llmResult.Accuracy
		assessment.Relevance = llmResult.Relevance
		assessment.EvaluatedBy = "llm"

		// Add LLM-detected issues
		for _, issue := range llmResult.AdditionalIssues {
			assessment.Issues = append(assessment.Issues, QualityIssue{
				Type:        IssueType(issue.Type),
				Severity:    issue.Severity,
				Description: issue.Description,
				Fixable:     true,
			})
		}

		// Add LLM suggestions
		for _, sug := range llmResult.Suggestions {
			assessment.Suggestions = append(assessment.Suggestions, ImprovementSuggestion{
				Type:        SuggestionType(sug.Type),
				Description: sug.Description,
				CodeHint:    sug.CodeHint,
				Priority:    0.7,
			})
		}

		// Recalculate score
		assessment.Score = (assessment.Completeness*0.3 + assessment.Accuracy*0.3 +
			assessment.Relevance*0.2 + assessment.Efficiency*0.2)
	}

	return assessment, nil
}

// =============================================================================
// PATTERN DETECTOR - FIND RECURRING ISSUES
// =============================================================================

// PatternDetector identifies recurring issues across tool executions
type PatternDetector struct {
	mu       sync.RWMutex
	history  []ExecutionFeedback
	patterns map[string]*DetectedPattern
}

// DetectedPattern represents a recurring issue pattern
type DetectedPattern struct {
	PatternID   string              `json:"pattern_id"`
	ToolName    string              `json:"tool_name"`
	IssueType   IssueType           `json:"issue_type"`
	Occurrences int                 `json:"occurrences"`
	FirstSeen   time.Time           `json:"first_seen"`
	LastSeen    time.Time           `json:"last_seen"`
	Confidence  float64             `json:"confidence"`
	Examples    []string            `json:"examples"`
	Suggestions []ImprovementSuggestion `json:"suggestions"`
}

// NewPatternDetector creates a new pattern detector
func NewPatternDetector() *PatternDetector {
	return &PatternDetector{
		history:  []ExecutionFeedback{},
		patterns: make(map[string]*DetectedPattern),
	}
}

// RecordExecution adds an execution to history and updates patterns
func (pd *PatternDetector) RecordExecution(feedback ExecutionFeedback) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.history = append(pd.history, feedback)

	// Limit history size
	if len(pd.history) > 1000 {
		pd.history = pd.history[100:] // Keep last 900
	}

	// Update patterns based on quality issues
	if feedback.Quality != nil {
		for _, issue := range feedback.Quality.Issues {
			patternKey := fmt.Sprintf("%s:%s", feedback.ToolName, issue.Type)

			pattern, exists := pd.patterns[patternKey]
			if !exists {
				pattern = &DetectedPattern{
					PatternID:   patternKey,
					ToolName:    feedback.ToolName,
					IssueType:   issue.Type,
					FirstSeen:   time.Now(),
					Examples:    []string{},
					Suggestions: []ImprovementSuggestion{},
				}
				pd.patterns[patternKey] = pattern
			}

			pattern.Occurrences++
			pattern.LastSeen = time.Now()
			pattern.Confidence = calculatePatternConfidence(pattern.Occurrences)

			// Add example (limit to 5)
			if len(pattern.Examples) < 5 {
				pattern.Examples = append(pattern.Examples, issue.Evidence)
			}

			// Merge suggestions
			if feedback.Quality != nil {
				for _, sug := range feedback.Quality.Suggestions {
					if !hasSuggestion(pattern.Suggestions, sug.Type) {
						pattern.Suggestions = append(pattern.Suggestions, sug)
					}
				}
			}
		}
	}
}

// GetPatterns returns detected patterns above confidence threshold
func (pd *PatternDetector) GetPatterns(minConfidence float64) []*DetectedPattern {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	patterns := []*DetectedPattern{}
	for _, p := range pd.patterns {
		if p.Confidence >= minConfidence {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// GetToolPatterns returns patterns for a specific tool
func (pd *PatternDetector) GetToolPatterns(toolName string) []*DetectedPattern {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	patterns := []*DetectedPattern{}
	for _, p := range pd.patterns {
		if p.ToolName == toolName {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// calculatePatternConfidence returns confidence based on occurrence count
func calculatePatternConfidence(occurrences int) float64 {
	// 1 occurrence = 0.3, 2 = 0.5, 3+ = 0.7+
	switch {
	case occurrences >= 5:
		return 0.9
	case occurrences >= 3:
		return 0.7
	case occurrences >= 2:
		return 0.5
	default:
		return 0.3
	}
}

// =============================================================================
// TOOL REFINER - IMPROVE TOOLS BASED ON FEEDBACK
// =============================================================================

// ToolRefiner generates improved tool versions based on feedback
type ToolRefiner struct {
	client  LLMClient
	toolGen *ToolGenerator
}

// RefinementRequest describes what needs to be improved
type RefinementRequest struct {
	ToolName      string
	OriginalCode  string
	Feedback      []ExecutionFeedback
	Patterns      []*DetectedPattern
	Suggestions   []ImprovementSuggestion
}

// RefinementResult contains the improved tool
type RefinementResult struct {
	Success       bool
	ImprovedCode  string
	Changes       []string  // Description of changes made
	ExpectedGain  float64   // Expected quality improvement
	TestCases     []string  // Test cases to verify improvement
}

// NewToolRefiner creates a new tool refiner
func NewToolRefiner(client LLMClient, toolGen *ToolGenerator) *ToolRefiner {
	return &ToolRefiner{
		client:  client,
		toolGen: toolGen,
	}
}

// Refine generates an improved version of a tool
func (tr *ToolRefiner) Refine(ctx context.Context, req RefinementRequest) (*RefinementResult, error) {
	result := &RefinementResult{
		Changes:   []string{},
		TestCases: []string{},
	}

	// Build improvement prompt
	prompt := tr.buildRefinementPrompt(req)

	resp, err := tr.client.CompleteWithSystem(ctx, refinementSystemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("refinement failed: %w", err)
	}

	// Parse response
	var refinement struct {
		ImprovedCode string   `json:"improved_code"`
		Changes      []string `json:"changes"`
		ExpectedGain float64  `json:"expected_gain"`
		TestCases    []string `json:"test_cases"`
	}

	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &refinement); err != nil {
		// Try to extract code block directly
		code := extractCodeBlock(resp, "go")
		if code != "" {
			result.ImprovedCode = code
			result.Success = true
			result.Changes = []string{"LLM-generated improvements"}
			return result, nil
		}
		return nil, fmt.Errorf("failed to parse refinement: %w", err)
	}

	result.Success = true
	result.ImprovedCode = refinement.ImprovedCode
	result.Changes = refinement.Changes
	result.ExpectedGain = refinement.ExpectedGain
	result.TestCases = refinement.TestCases

	return result, nil
}

// buildRefinementPrompt creates the prompt for tool improvement
func (tr *ToolRefiner) buildRefinementPrompt(req RefinementRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Improve this tool based on execution feedback:\n\n"))
	sb.WriteString(fmt.Sprintf("Tool Name: %s\n\n", req.ToolName))
	sb.WriteString(fmt.Sprintf("Original Code:\n```go\n%s\n```\n\n", req.OriginalCode))

	// Add feedback summary
	sb.WriteString("Execution Feedback:\n")
	for i, fb := range req.Feedback {
		if i >= 3 {
			sb.WriteString(fmt.Sprintf("... and %d more executions\n", len(req.Feedback)-3))
			break
		}
		sb.WriteString(fmt.Sprintf("- Execution %d: success=%v, quality=%.2f\n",
			i+1, fb.Success, fb.Quality.Score))
		for _, issue := range fb.Quality.Issues {
			sb.WriteString(fmt.Sprintf("  - Issue: %s (%s)\n", issue.Type, issue.Description))
		}
	}

	// Add detected patterns
	if len(req.Patterns) > 0 {
		sb.WriteString("\nRecurring Patterns:\n")
		for _, p := range req.Patterns {
			sb.WriteString(fmt.Sprintf("- %s: %d occurrences (%.0f%% confidence)\n",
				p.IssueType, p.Occurrences, p.Confidence*100))
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
	mu       sync.RWMutex
	storePath string
	learnings map[string]*ToolLearning
}

// ToolLearning contains all learnings about a tool
type ToolLearning struct {
	ToolName           string                 `json:"tool_name"`
	Version            int                    `json:"version"`
	TotalExecutions    int                    `json:"total_executions"`
	SuccessRate        float64                `json:"success_rate"`
	AverageQuality     float64                `json:"average_quality"`
	KnownIssues        []IssueType            `json:"known_issues"`
	AppliedFixes       []string               `json:"applied_fixes"`
	BestPractices      []string               `json:"best_practices"`
	AntiPatterns       []string               `json:"anti_patterns"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// NewLearningStore creates a new learning store
func NewLearningStore(storePath string) *LearningStore {
	store := &LearningStore{
		storePath: storePath,
		learnings: make(map[string]*ToolLearning),
	}
	store.load()
	return store
}

// RecordLearning updates learnings for a tool
func (ls *LearningStore) RecordLearning(toolName string, feedback *ExecutionFeedback, patterns []*DetectedPattern) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	learning, exists := ls.learnings[toolName]
	if !exists {
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
		for _, issue := range feedback.Quality.Issues {
			if !containsIssueType(learning.KnownIssues, issue.Type) {
				learning.KnownIssues = append(learning.KnownIssues, issue.Type)
			}
		}
	}

	// Extract anti-patterns from patterns
	for _, p := range patterns {
		if p.Confidence > 0.7 {
			antiPattern := fmt.Sprintf("%s: %s", p.IssueType, p.PatternID)
			if !contains(learning.AntiPatterns, antiPattern) {
				learning.AntiPatterns = append(learning.AntiPatterns, antiPattern)
			}
		}
	}

	learning.UpdatedAt = time.Now()
	ls.save()
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

func extractMatch(text string, pattern *regexp.Regexp) string {
	match := pattern.FindString(text)
	if len(match) > 100 {
		return match[:100] + "..."
	}
	return match
}

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
