// Package autopoiesis implements self-modification capabilities for codeNERD.
// Tool-generation audit records retain requests, declared decisions, costs,
// generated artifacts, and execution outcomes. They do not reconstruct a model's
// private deliberation or retain response bodies for that purpose.
package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// GLOBAL REGEX PATTERNS (Performance Optimization)
// =============================================================================
var (
	importPattern       = regexp.MustCompile(`import\s*\(\s*\n`)
	singleImportPattern = regexp.MustCompile("import\\s+\"([^\"]+)\"")
	funcPattern         = regexp.MustCompile(`(func\s+\w+\s*\([^)]*\)\s*(?:\([^)]*\)\s*)?\{)\n`)
	entryPattern        = regexp.MustCompile(`(\[TOOL_ENTRY\][^\n]+\n)`)
	errorReturnPattern  = regexp.MustCompile(`return\s+([^,\n]+),\s*(fmt\.Errorf|errors\.New|err)\b`)
	httpPattern         = regexp.MustCompile(`(http\.(Get|Post|Do)\([^)]+\))`)
	forPattern          = regexp.MustCompile(`(for\s+[^{]+\{)\n`)
	hasErrorPattern     = regexp.MustCompile(`if\s+err\s*!=\s*nil.*log\.`)
)

// =============================================================================
// TOOL GENERATION AUDIT RECORDS
// =============================================================================

// ToolGenerationRecord captures declared inputs and observable tool-generation outcomes.
type ToolGenerationRecord struct {
	// Identity
	TraceID     string    `json:"trace_id"`
	ToolName    string    `json:"tool_name"`
	GeneratedAt time.Time `json:"generated_at"`

	// Input Context
	UserRequest     string            `json:"user_request"`     // What triggered the need
	DetectedNeed    *ToolNeed         `json:"detected_need"`    // Structured need
	ExistingTools   []string          `json:"existing_tools"`   // Tools already available
	ContextProvided map[string]string `json:"context_provided"` // Additional context

	// LLM Interaction
	SystemPrompt string `json:"system_prompt"` // System prompt used
	UserPrompt   string `json:"user_prompt"`   // Full prompt sent to LLM

	// Explicit metadata supplied by the caller, never inferred from model prose.
	DecisionSummary string     `json:"decision_summary,omitempty"`
	KeyDecisions    []Decision `json:"key_decisions,omitempty"`
	Assumptions     []string   `json:"assumptions,omitempty"`

	// Generation Metadata
	ModelUsed      string        `json:"model_used"`
	TokensUsed     int           `json:"tokens_used"`
	GenerationTime time.Duration `json:"generation_time"`
	RetryCount     int           `json:"retry_count"`

	// Outcome
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason,omitempty"`
	CodeGenerated string `json:"code_generated"`

	// Learnings (filled after execution feedback)
	PostExecutionNotes []string `json:"post_execution_notes,omitempty"`
	QualityScore       float64  `json:"quality_score,omitempty"`
	IssuesFound        []string `json:"issues_found,omitempty"`
}

// Decision represents a key decision made during generation
type Decision struct {
	Topic        string   `json:"topic"`                  // e.g., "pagination_strategy"
	Choice       string   `json:"choice"`                 // What was chosen
	Rationale    string   `json:"rationale"`              // Brief, externally assessable justification
	Alternatives []string `json:"alternatives,omitempty"` // Explicitly documented design options
}

// =============================================================================
// TRACE COLLECTOR - CAPTURE AND ANALYZE TRACES
// =============================================================================

// TraceCollector manages tool-generation audit records and outcome statistics.
type TraceCollector struct {
	mu        sync.RWMutex
	storePath string
	traces    map[string]*ToolGenerationRecord // TraceID -> Trace
	byTool    map[string][]string              // ToolName -> TraceIDs
}

// NewTraceCollector creates a tool-generation audit collector. The client
// parameter is retained for caller compatibility; collection makes no LLM calls.
func NewTraceCollector(storePath string, _ LLMClient) *TraceCollector {
	tc := &TraceCollector{
		storePath: storePath,
		traces:    make(map[string]*ToolGenerationRecord),
		byTool:    make(map[string][]string),
	}
	tc.load()
	return tc
}

// StartTrace begins a tool-generation audit record.
func (tc *TraceCollector) StartTrace(toolName string, need *ToolNeed, userRequest string) *ToolGenerationRecord {
	trace := &ToolGenerationRecord{
		TraceID:         fmt.Sprintf("%s-%d", toolName, time.Now().UnixNano()),
		ToolName:        toolName,
		GeneratedAt:     time.Now(),
		UserRequest:     userRequest,
		DetectedNeed:    need,
		ContextProvided: make(map[string]string),
		KeyDecisions:    []Decision{},
		Assumptions:     []string{},
	}

	tc.mu.Lock()
	tc.traces[trace.TraceID] = trace
	tc.byTool[toolName] = append(tc.byTool[toolName], trace.TraceID)
	tc.mu.Unlock()

	return trace
}

// RecordPrompt captures the prompts sent to the LLM
func (tc *TraceCollector) RecordPrompt(trace *ToolGenerationRecord, systemPrompt, userPrompt string) {
	trace.SystemPrompt = systemPrompt
	trace.UserPrompt = userPrompt
}

// RecordResponse records usage metadata only. Response text is intentionally
// ignored: audit collection must not infer or request intermediate deliberation.
// The parameters remain for compatibility with generation callers.
func (tc *TraceCollector) RecordResponse(_ context.Context, trace *ToolGenerationRecord, _ string, tokensUsed int, duration time.Duration) {
	trace.TokensUsed = tokensUsed
	trace.GenerationTime = duration
}

// FinalizeTrace marks a trace as complete
func (tc *TraceCollector) FinalizeTrace(trace *ToolGenerationRecord, success bool, code string, failureReason string) {
	trace.Success = success
	trace.CodeGenerated = code
	trace.FailureReason = failureReason
	if success {
		trace.DecisionSummary = "Tool generation completed successfully."
	} else {
		trace.DecisionSummary = "Tool generation failed. See failure_reason for the reported error."
	}
	tc.save()
}

// UpdateWithFeedback adds execution feedback to a trace
func (tc *TraceCollector) UpdateWithFeedback(toolName string, quality float64, issues []string, notes []string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Find most recent trace for this tool
	traceIDs := tc.byTool[toolName]
	if len(traceIDs) == 0 {
		return
	}

	latestID := traceIDs[len(traceIDs)-1]
	if trace, exists := tc.traces[latestID]; exists {
		trace.QualityScore = quality
		trace.IssuesFound = issues
		trace.PostExecutionNotes = notes
	}

	tc.save()
}

// GetTrace retrieves a specific trace
func (tc *TraceCollector) GetTrace(traceID string) *ToolGenerationRecord {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.traces[traceID]
}

// GetToolTraces retrieves all traces for a tool
func (tc *TraceCollector) GetToolTraces(toolName string) []*ToolGenerationRecord {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	traces := []*ToolGenerationRecord{}
	for _, id := range tc.byTool[toolName] {
		if trace, exists := tc.traces[id]; exists {
			traces = append(traces, trace)
		}
	}
	return traces
}

// GetAllTraces returns all traces
func (tc *TraceCollector) GetAllTraces() []*ToolGenerationRecord {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	traces := make([]*ToolGenerationRecord, 0, len(tc.traces))
	for _, trace := range tc.traces {
		traces = append(traces, trace)
	}
	return traces
}

// =============================================================================
// GENERATION AUDIT LOG - BROAD PATTERN ANALYSIS
// =============================================================================

// GenerationAudit contains aggregated analysis of tool generations
type GenerationAudit struct {
	// Summary Statistics
	TotalGenerations      int           `json:"total_generations"`
	SuccessfulGenerations int           `json:"successful_generations"`
	SuccessRate           float64       `json:"success_rate"`
	AverageQuality        float64       `json:"average_quality"`
	AverageTokens         int           `json:"average_tokens"`
	AverageTime           time.Duration `json:"average_time"`

	// Pattern Analysis
	CommonDecisions   []DecisionPattern   `json:"common_decisions"`
	CommonAssumptions []AssumptionPattern `json:"common_assumptions"`
	CommonIssues      []IssuePattern      `json:"common_issues"`

	// Optimization Opportunities
	Optimizations []OptimizationOpportunity `json:"optimizations"`

	// Tool Type Analysis
	ByToolType map[string]TypeAnalysis `json:"by_tool_type"`
}

// DecisionPattern represents a recurring decision pattern
type DecisionPattern struct {
	Topic            string  `json:"topic"`
	MostCommonChoice string  `json:"most_common_choice"`
	Occurrences      int     `json:"occurrences"`
	SuccessRate      float64 `json:"success_rate_with_choice"`
}

// AssumptionPattern represents a recurring assumption
type AssumptionPattern struct {
	Assumption   string  `json:"assumption"`
	Occurrences  int     `json:"occurrences"`
	AccuracyRate float64 `json:"accuracy_rate"` // How often it was correct
}

// IssuePattern represents a recurring issue
type IssuePattern struct {
	Issue          string   `json:"issue"`
	Occurrences    int      `json:"occurrences"`
	CommonCauses   []string `json:"common_causes"`
	EffectiveFixes []string `json:"effective_fixes"`
}

// OptimizationOpportunity represents a potential improvement
type OptimizationOpportunity struct {
	Area        string  `json:"area"`
	Description string  `json:"description"`
	Evidence    string  `json:"evidence"`
	Impact      float64 `json:"estimated_impact"` // 0.0 - 1.0
	Suggestion  string  `json:"suggestion"`
}

// TypeAnalysis contains analysis for a specific tool type
type TypeAnalysis struct {
	ToolType       string   `json:"tool_type"`
	Count          int      `json:"count"`
	SuccessRate    float64  `json:"success_rate"`
	AvgQuality     float64  `json:"avg_quality"`
	CommonPatterns []string `json:"common_patterns"`
	BestPractices  []string `json:"best_practices"`
}

// AnalyzeGenerations performs broad analysis across all tool generations
func (tc *TraceCollector) AnalyzeGenerations(ctx context.Context) (*GenerationAudit, error) {
	tc.mu.RLock()
	traces := make([]*ToolGenerationRecord, 0, len(tc.traces))
	for _, trace := range tc.traces {
		traces = append(traces, trace)
	}
	tc.mu.RUnlock()

	if len(traces) == 0 {
		return &GenerationAudit{}, nil
	}

	audit := &GenerationAudit{
		ByToolType: make(map[string]TypeAnalysis),
	}

	// Calculate summary statistics
	var totalQuality float64
	var totalTokens int
	var totalTime time.Duration
	successCount := 0

	decisionCounts := make(map[string]map[string]int) // topic -> choice -> count
	assumptionCounts := make(map[string]int)          // assumption -> count
	issueCounts := make(map[string]int)               // issue -> count

	for _, trace := range traces {
		audit.TotalGenerations++
		totalTokens += trace.TokensUsed
		totalTime += trace.GenerationTime

		if trace.Success {
			successCount++
		}
		if trace.QualityScore > 0 {
			totalQuality += trace.QualityScore
		}

		// Track decisions
		for _, decision := range trace.KeyDecisions {
			if decisionCounts[decision.Topic] == nil {
				decisionCounts[decision.Topic] = make(map[string]int)
			}
			decisionCounts[decision.Topic][decision.Choice]++
		}

		// Track assumptions
		for _, assumption := range trace.Assumptions {
			assumptionCounts[assumption]++
		}

		// Track issues
		for _, issue := range trace.IssuesFound {
			issueCounts[issue]++
		}

		// Track by tool type
		toolType := inferToolType(trace.ToolName)
		analysis := audit.ByToolType[toolType]
		analysis.ToolType = toolType
		analysis.Count++
		if trace.Success {
			analysis.SuccessRate = (analysis.SuccessRate*float64(analysis.Count-1) + 1.0) / float64(analysis.Count)
		}
		if trace.QualityScore > 0 {
			analysis.AvgQuality = (analysis.AvgQuality*float64(analysis.Count-1) + trace.QualityScore) / float64(analysis.Count)
		}
		audit.ByToolType[toolType] = analysis
	}

	// Compute averages
	audit.SuccessfulGenerations = successCount
	if audit.TotalGenerations > 0 {
		audit.SuccessRate = float64(successCount) / float64(audit.TotalGenerations)
		audit.AverageTokens = totalTokens / audit.TotalGenerations
		audit.AverageTime = totalTime / time.Duration(audit.TotalGenerations)
		if successCount > 0 {
			audit.AverageQuality = totalQuality / float64(successCount)
		}
	}

	// Find common patterns
	audit.CommonDecisions = findCommonDecisions(decisionCounts)
	audit.CommonAssumptions = findCommonAssumptions(assumptionCounts)
	audit.CommonIssues = findCommonIssues(issueCounts, traces)

	// Identify optimization opportunities
	audit.Optimizations = tc.identifyOptimizations(ctx, audit, traces)

	return audit, nil
}

// identifyOptimizations uses analysis to suggest improvements
func (tc *TraceCollector) identifyOptimizations(_ context.Context, audit *GenerationAudit, _ []*ToolGenerationRecord) []OptimizationOpportunity {
	opportunities := []OptimizationOpportunity{}

	// Check for common issues that could be prevented
	for _, issue := range audit.CommonIssues {
		if issue.Occurrences >= 3 {
			opportunities = append(opportunities, OptimizationOpportunity{
				Area:        "issue_prevention",
				Description: fmt.Sprintf("Issue '%s' occurred %d times", issue.Issue, issue.Occurrences),
				Evidence:    strings.Join(issue.CommonCauses, "; "),
				Impact:      float64(issue.Occurrences) / float64(audit.TotalGenerations),
				Suggestion:  fmt.Sprintf("Add '%s' to generation prompts by default", strings.Join(issue.EffectiveFixes, ", ")),
			})
		}
	}

	// Check for patterns where certain decisions lead to better outcomes
	for _, pattern := range audit.CommonDecisions {
		if pattern.SuccessRate > 0.8 && pattern.Occurrences >= 3 {
			opportunities = append(opportunities, OptimizationOpportunity{
				Area:        "decision_standardization",
				Description: fmt.Sprintf("Decision '%s: %s' has %.0f%% success rate", pattern.Topic, pattern.MostCommonChoice, pattern.SuccessRate*100),
				Impact:      0.3,
				Suggestion:  fmt.Sprintf("Make '%s' the default for '%s'", pattern.MostCommonChoice, pattern.Topic),
			})
		}
	}

	// Check for assumptions that are frequently wrong
	for _, assumption := range audit.CommonAssumptions {
		if assumption.AccuracyRate < 0.5 && assumption.Occurrences >= 3 {
			opportunities = append(opportunities, OptimizationOpportunity{
				Area:        "assumption_correction",
				Description: fmt.Sprintf("Assumption '%s' is only %.0f%% accurate", assumption.Assumption, assumption.AccuracyRate*100),
				Impact:      0.4,
				Suggestion:  "Add explicit checks or clarification for this assumption",
			})
		}
	}

	return opportunities
}

// =============================================================================
// MANDATORY LOGGING INJECTION
// =============================================================================

// LoggingRequirements defines what logging must be present in generated tools
type LoggingRequirements struct {
	RequireEntryLog     bool `json:"require_entry_log"`     // Log on function entry
	RequireExitLog      bool `json:"require_exit_log"`      // Log on function exit
	RequireErrorLog     bool `json:"require_error_log"`     // Log all errors
	RequireInputLog     bool `json:"require_input_log"`     // Log input parameters
	RequireOutputLog    bool `json:"require_output_log"`    // Log output/return values
	RequireTimingLog    bool `json:"require_timing_log"`    // Log execution duration
	RequireDecisionLog  bool `json:"require_decision_log"`  // Log key decisions
	RequireAPICallLog   bool `json:"require_api_call_log"`  // Log external API calls
	RequireIterationLog bool `json:"require_iteration_log"` // Log loop iterations
}

// DefaultLoggingRequirements returns the mandatory logging requirements
func DefaultLoggingRequirements() LoggingRequirements {
	return LoggingRequirements{
		RequireEntryLog:     true,
		RequireExitLog:      true,
		RequireErrorLog:     true,
		RequireInputLog:     true,
		RequireOutputLog:    true,
		RequireTimingLog:    true,
		RequireDecisionLog:  true,
		RequireAPICallLog:   true,
		RequireIterationLog: true,
	}
}

// LogInjector injects mandatory logging into generated tool code
type LogInjector struct {
	requirements LoggingRequirements
}

// NewLogInjector creates a new log injector
func NewLogInjector(requirements LoggingRequirements) *LogInjector {
	return &LogInjector{requirements: requirements}
}

// InjectLogging adds mandatory logging to tool code
func (li *LogInjector) InjectLogging(code string, toolName string) (string, error) {
	// Add logging import if not present
	code = li.ensureLoggingImport(code)

	// Add logging statements
	code = li.injectEntryLogging(code, toolName)
	code = li.injectExitLogging(code, toolName)
	code = li.injectErrorLogging(code, toolName)
	code = li.injectTimingLogging(code, toolName)
	code = li.injectAPICallLogging(code, toolName)
	code = li.injectIterationLogging(code, toolName)

	return code, nil
}

// ValidateLogging checks that required logging is present
func (li *LogInjector) ValidateLogging(code string) *LoggingValidation {
	validation := &LoggingValidation{
		Valid:   true,
		Missing: []string{},
	}

	if li.requirements.RequireEntryLog && !hasEntryLogging(code) {
		validation.Valid = false
		validation.Missing = append(validation.Missing, "entry_log")
	}

	if li.requirements.RequireExitLog && !hasExitLogging(code) {
		validation.Valid = false
		validation.Missing = append(validation.Missing, "exit_log")
	}

	if li.requirements.RequireErrorLog && !hasErrorLogging(code) {
		validation.Valid = false
		validation.Missing = append(validation.Missing, "error_log")
	}

	if li.requirements.RequireTimingLog && !hasTimingLogging(code) {
		validation.Valid = false
		validation.Missing = append(validation.Missing, "timing_log")
	}

	return validation
}

// LoggingValidation contains the result of logging validation
type LoggingValidation struct {
	Valid    bool     `json:"valid"`
	Missing  []string `json:"missing"`
	Warnings []string `json:"warnings"`
}

// ensureLoggingImport adds the logging import if not present
func (li *LogInjector) ensureLoggingImport(code string) string {
	// Check if we have structured logging
	if strings.Contains(code, `"log"`) || strings.Contains(code, `"log/slog"`) {
		return code
	}

	// Find import block and add logging
	// Find import block and add logging
	if importPattern.MatchString(code) {
		code = importPattern.ReplaceAllString(code, "import (\n\t\"log\"\n\t\"time\"\n")
	} else if strings.Contains(code, "import ") {
		// Single import, convert to block
		code = singleImportPattern.ReplaceAllString(code, "import (\n\t\"log\"\n\t\"time\"\n\t\"$1\"\n)")
	}

	return code
}

// injectEntryLogging adds entry logging to functions
func (li *LogInjector) injectEntryLogging(code string, toolName string) string {
	if !li.requirements.RequireEntryLog {
		return code
	}

	// Find main tool function and add entry log
	return funcPattern.ReplaceAllStringFunc(code, func(match string) string {
		// Don't add if already has entry log
		if strings.Contains(code[strings.Index(code, match):], "TOOL_ENTRY") {
			return match
		}
		indent := "\t"
		logStmt := fmt.Sprintf(`%slog.Printf("[TOOL_ENTRY] %s: starting execution, input=%%q", input)%s`, indent, toolName, "\n")
		return match + logStmt
	})
}

// injectExitLogging adds exit logging to functions
func (li *LogInjector) injectExitLogging(code string, toolName string) string {
	if !li.requirements.RequireExitLog {
		return code
	}

	// Add defer for exit logging after entry log
	return entryPattern.ReplaceAllStringFunc(code, func(match string) string {
		if strings.Contains(code, "TOOL_EXIT") {
			return match
		}
		logStmt := fmt.Sprintf("\tdefer log.Printf(\"[TOOL_EXIT] %s: execution complete\")\n", toolName)
		return match + logStmt
	})
}

// injectErrorLogging wraps error returns with logging
func (li *LogInjector) injectErrorLogging(code string, toolName string) string {
	if !li.requirements.RequireErrorLog {
		return code
	}

	// Find error returns and wrap with logging
	return errorReturnPattern.ReplaceAllStringFunc(code, func(match string) string {
		if strings.Contains(match, "TOOL_ERROR") {
			return match
		}
		// Extract the error part
		parts := errorReturnPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		// Add logging before return
		return fmt.Sprintf(`log.Printf("[TOOL_ERROR] %s: %%v", %s); %s`, toolName, parts[2], match)
	})
}

// injectTimingLogging adds timing measurement
func (li *LogInjector) injectTimingLogging(code string, toolName string) string {
	if !li.requirements.RequireTimingLog {
		return code
	}

	// Add timing after entry log
	return entryPattern.ReplaceAllStringFunc(code, func(match string) string {
		if strings.Contains(code, "_toolStartTime") {
			return match
		}
		timing := fmt.Sprintf("\t_toolStartTime := time.Now()\n\tdefer func() { log.Printf(\"[TOOL_TIMING] %s: duration=%%v\", time.Since(_toolStartTime)) }()\n", toolName)
		return match + timing
	})
}

// injectAPICallLogging wraps HTTP/API calls with logging
func (li *LogInjector) injectAPICallLogging(code string, toolName string) string {
	if !li.requirements.RequireAPICallLog {
		return code
	}

	// Find http.Get, http.Post, etc. and wrap with logging
	return httpPattern.ReplaceAllStringFunc(code, func(match string) string {
		idx := strings.Index(code, match)
		startIdx := idx - 50
		if startIdx < 0 {
			startIdx = 0
		}
		if strings.Contains(code[startIdx:], "TOOL_API_CALL") {
			return match
		}
		return fmt.Sprintf(`func() (r *http.Response, e error) { log.Printf("[TOOL_API_CALL] %s: making request"); r, e = %s; log.Printf("[TOOL_API_RESPONSE] %s: status=%%v", r.StatusCode); return }()`, toolName, match, toolName)
	})
}

// injectIterationLogging adds logging for loops
func (li *LogInjector) injectIterationLogging(code string, toolName string) string {
	if !li.requirements.RequireIterationLog {
		return code
	}

	// Find for loops and add iteration logging
	counter := 0
	return forPattern.ReplaceAllStringFunc(code, func(match string) string {
		counter++
		if strings.Contains(code, fmt.Sprintf("_loopIter%d", counter)) {
			return match
		}
		loopLog := fmt.Sprintf("\t\t_loopIter%d := 0; defer func() { log.Printf(\"[TOOL_ITERATION] %s: loop %d completed %%d iterations\", _loopIter%d) }()\n\t\t_loopIter%d++\n", counter, toolName, counter, counter, counter)
		return match + loopLog
	})
}

// =============================================================================
// LOGGING VALIDATION HELPERS
// =============================================================================

func hasEntryLogging(code string) bool {
	return strings.Contains(code, "TOOL_ENTRY") || strings.Contains(code, "entry") && strings.Contains(code, "log.")
}

func hasExitLogging(code string) bool {
	return strings.Contains(code, "TOOL_EXIT") || strings.Contains(code, "defer") && strings.Contains(code, "log.")
}

var errorLogPattern = regexp.MustCompile(`if\s+err\s*!=\s*nil.*log\.`)

func hasErrorLogging(code string) bool {
	if strings.Contains(code, "TOOL_ERROR") {
		return true
	}
	if !strings.Contains(code, "err") || !strings.Contains(code, "log.") {
		return false
	}
	return hasErrorPattern.MatchString(code)
}

func hasTimingLogging(code string) bool {
	return strings.Contains(code, "TOOL_TIMING") || (strings.Contains(code, "time.Since") && strings.Contains(code, "log."))
}

// =============================================================================
// ANALYSIS HELPERS
// =============================================================================

func findCommonDecisions(decisionCounts map[string]map[string]int) []DecisionPattern {
	patterns := []DecisionPattern{}

	for topic, choices := range decisionCounts {
		var maxChoice string
		var maxCount int
		var totalCount int

		// Sorted, because this is a LEARNED preference: whatever wins here is
		// recorded as what the agent decided about this topic. Ranging the map
		// with a strict `>` meant two equally-taken choices produced a
		// different lesson each time the traces were analysed, which is a
		// memory that disagrees with itself.
		options := make([]string, 0, len(choices))
		for choice := range choices {
			options = append(options, choice)
		}
		sort.Strings(options)

		for _, choice := range options {
			count := choices[choice]
			totalCount += count
			if count > maxCount {
				maxCount = count
				maxChoice = choice
			}
		}

		if maxCount >= 2 {
			patterns = append(patterns, DecisionPattern{
				Topic:            topic,
				MostCommonChoice: maxChoice,
				Occurrences:      maxCount,
				SuccessRate:      float64(maxCount) / float64(totalCount),
			})
		}
	}

	return patterns
}

func findCommonAssumptions(assumptionCounts map[string]int) []AssumptionPattern {
	patterns := []AssumptionPattern{}

	for assumption, count := range assumptionCounts {
		if count >= 2 {
			patterns = append(patterns, AssumptionPattern{
				Assumption:   assumption,
				Occurrences:  count,
				AccuracyRate: 0.5, // Default, updated when we have feedback
			})
		}
	}

	return patterns
}

func findCommonIssues(issueCounts map[string]int, traces []*ToolGenerationRecord) []IssuePattern {
	patterns := []IssuePattern{}

	for issue, count := range issueCounts {
		if count >= 2 {
			pattern := IssuePattern{
				Issue:       issue,
				Occurrences: count,
			}

			// Find common causes from traces that had this issue
			causes := make(map[string]int)
			for _, trace := range traces {
				for _, traceIssue := range trace.IssuesFound {
					if traceIssue == issue {
						// Look at decisions that might have caused this
						for _, decision := range trace.KeyDecisions {
							causes[decision.Choice]++
						}
					}
				}
			}

			for cause, causeCount := range causes {
				if causeCount >= 2 {
					pattern.CommonCauses = append(pattern.CommonCauses, cause)
				}
			}

			patterns = append(patterns, pattern)
		}
	}

	return patterns
}

func inferToolType(toolName string) string {
	lower := strings.ToLower(toolName)

	switch {
	case strings.Contains(lower, "api") || strings.Contains(lower, "fetch"):
		return "api_client"
	case strings.Contains(lower, "parse") || strings.Contains(lower, "extract"):
		return "parser"
	case strings.Contains(lower, "valid"):
		return "validator"
	case strings.Contains(lower, "convert") || strings.Contains(lower, "transform"):
		return "converter"
	case strings.Contains(lower, "format"):
		return "formatter"
	default:
		return "utility"
	}
}

// =============================================================================
// PERSISTENCE
// =============================================================================

func (tc *TraceCollector) load() {
	path := filepath.Join(tc.storePath, "tool_generation_records.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var stored struct {
		Traces map[string]*ToolGenerationRecord `json:"traces"`
		ByTool map[string][]string              `json:"by_tool"`
	}

	if err := json.Unmarshal(data, &stored); err == nil {
		tc.traces = stored.Traces
		tc.byTool = stored.ByTool
	}
}

func (tc *TraceCollector) save() {
	if err := os.MkdirAll(tc.storePath, 0755); err != nil {
		return
	}

	tc.mu.RLock()
	stored := struct {
		Traces map[string]*ToolGenerationRecord `json:"traces"`
		ByTool map[string][]string              `json:"by_tool"`
	}{
		Traces: tc.traces,
		ByTool: tc.byTool,
	}
	tc.mu.RUnlock()

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(tc.storePath, "tool_generation_records.json")
	// Surface persistence failures instead of silently dropping the traces.
	if err := os.WriteFile(path, data, 0644); err != nil {
		logging.AutopoiesisError("TraceCollector.save: write %s: %v", path, err)
	}
}
