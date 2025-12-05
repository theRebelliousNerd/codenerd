// Package autopoiesis implements self-modification capabilities for codeNERD.
// This includes complexity analysis, tool generation, and persistent agent creation.
package autopoiesis

import (
	"context"
	"regexp"
	"strings"
)

// =============================================================================
// COMPLEXITY ANALYZER
// =============================================================================
// Analyzes user requests to determine if they require campaigns, persistent
// agents, or new tool capabilities.

// ComplexityLevel represents how complex a task is
type ComplexityLevel int

const (
	ComplexitySimple   ComplexityLevel = iota // Single action, single file
	ComplexityModerate                        // Multiple files, one phase
	ComplexityComplex                         // Multiple phases, dependencies
	ComplexityEpic                            // Full feature, multiple components
)

// ComplexityResult contains the analysis of a task's complexity
type ComplexityResult struct {
	Level           ComplexityLevel
	Score           float64 // 0.0 - 1.0
	NeedsCampaign   bool
	NeedsPersistent bool
	Reasons         []string
	SuggestedPhases []string
	EstimatedFiles  int
}

// ComplexityAnalyzer determines task complexity from natural language
type ComplexityAnalyzer struct {
	client LLMClient
}

// LLMClient interface for LLM calls
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// NewComplexityAnalyzer creates a new analyzer
func NewComplexityAnalyzer(client LLMClient) *ComplexityAnalyzer {
	return &ComplexityAnalyzer{client: client}
}

// Complexity indicators - patterns that suggest different complexity levels
var (
	// Epic-level indicators (full features, systems)
	epicPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)implement\s+(a\s+)?(full|complete|entire|whole)\s+(feature|system|module|service)`),
		regexp.MustCompile(`(?i)build\s+(out\s+)?(a\s+)?(full|complete|entire)\s+`),
		regexp.MustCompile(`(?i)(create|add)\s+(a\s+)?(new\s+)?(authentication|authorization|payment|billing|notification|messaging)\s+(system|service|module)`),
		regexp.MustCompile(`(?i)migrate\s+(from|to)\s+`),
		regexp.MustCompile(`(?i)rewrite\s+(the\s+)?(entire|whole|complete)`),
		regexp.MustCompile(`(?i)(api|database|frontend|backend)\s+(redesign|overhaul|rewrite)`),
		regexp.MustCompile(`(?i)add\s+(support\s+for\s+)?(multiple|multi-tenant|internationalization|i18n|localization)`),
	}

	// Complex-level indicators (multi-phase work)
	complexPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(implement|add|create)\s+.+\s+(with|including|and)\s+.+\s+(and|with)\s+`),
		regexp.MustCompile(`(?i)(refactor|restructure)\s+(the\s+)?(entire|whole|all)`),
		regexp.MustCompile(`(?i)add\s+(a\s+)?(new\s+)?(crud|rest\s*api|graphql|grpc)\s+(endpoint|api|service)`),
		regexp.MustCompile(`(?i)(database|schema)\s+(migration|change|update)`),
		regexp.MustCompile(`(?i)integrate\s+(with\s+)?[a-zA-Z]+\s+(api|service|system)`),
		regexp.MustCompile(`(?i)(test|testing)\s+(coverage|suite)\s+(for|across)\s+(all|multiple|the\s+entire)`),
		regexp.MustCompile(`(?i)split\s+.+\s+into\s+(multiple|separate)\s+`),
	}

	// Moderate-level indicators (multi-file but single phase)
	moderatePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(update|change|modify)\s+(all|every|multiple)\s+(files?|instances?|occurrences?)`),
		regexp.MustCompile(`(?i)rename\s+.+\s+(across|throughout|in\s+all)`),
		regexp.MustCompile(`(?i)(add|implement)\s+(a\s+)?(new\s+)?(component|handler|controller|service|repository)`),
		regexp.MustCompile(`(?i)(create|add)\s+(unit\s+)?tests?\s+for\s+`),
		regexp.MustCompile(`(?i)extract\s+(method|function|class|interface|module)\s+`),
	}

	// Persistence indicators (long-running or monitoring needs)
	persistencePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(monitor|watch|track)\s+(for\s+)?(changes?|updates?|errors?|issues?)`),
		regexp.MustCompile(`(?i)(continuous|ongoing|regular)\s+(review|analysis|monitoring|checking)`),
		regexp.MustCompile(`(?i)keep\s+(an?\s+)?(eye|track|watch)\s+(on|of)`),
		regexp.MustCompile(`(?i)alert\s+(me\s+)?(when|if|on)`),
		regexp.MustCompile(`(?i)learn\s+(from|about)\s+(my|our|the)\s+(preferences?|style|patterns?)`),
		regexp.MustCompile(`(?i)(remember|recall|store)\s+(this|that|my)\s+(preference|setting|choice)`),
		regexp.MustCompile(`(?i)always\s+(use|prefer|apply|remember)`),
		regexp.MustCompile(`(?i)whenever\s+(i|we)\s+(commit|push|deploy|test)`),
	}

	// Multi-file indicators
	multiFileIndicators = []string{
		"all files", "every file", "multiple files", "across files",
		"throughout the codebase", "entire codebase", "whole project",
		"all components", "every component", "all modules",
		"refactor all", "update all", "change all",
	}

	// Phase indicators (suggest multi-phase work)
	phaseIndicators = map[string]string{
		"test":          "Testing Phase",
		"document":      "Documentation Phase",
		"implement":     "Implementation Phase",
		"design":        "Design Phase",
		"refactor":      "Refactoring Phase",
		"migrate":       "Migration Phase",
		"deploy":        "Deployment Phase",
		"review":        "Review Phase",
		"validate":      "Validation Phase",
		"integrate":     "Integration Phase",
		"authentication": "Auth Implementation",
		"authorization": "Auth Implementation",
		"database":      "Database Phase",
		"api":           "API Phase",
		"frontend":      "Frontend Phase",
		"backend":       "Backend Phase",
	}
)

// Analyze determines the complexity of a task from its description
func (ca *ComplexityAnalyzer) Analyze(ctx context.Context, input string, target string) ComplexityResult {
	result := ComplexityResult{
		Level:   ComplexitySimple,
		Score:   0.1,
		Reasons: []string{},
	}

	lower := strings.ToLower(input)

	// Check for epic patterns (highest priority)
	for _, pattern := range epicPatterns {
		if pattern.MatchString(lower) {
			result.Level = ComplexityEpic
			result.Score = 0.95
			result.NeedsCampaign = true
			result.Reasons = append(result.Reasons, "Matches epic-level pattern: large feature implementation")
			break
		}
	}

	// Check for complex patterns
	if result.Level < ComplexityComplex {
		for _, pattern := range complexPatterns {
			if pattern.MatchString(lower) {
				result.Level = ComplexityComplex
				result.Score = max(result.Score, 0.75)
				result.NeedsCampaign = true
				result.Reasons = append(result.Reasons, "Matches complex pattern: multi-phase work required")
				break
			}
		}
	}

	// Check for moderate patterns
	if result.Level < ComplexityModerate {
		for _, pattern := range moderatePatterns {
			if pattern.MatchString(lower) {
				result.Level = ComplexityModerate
				result.Score = max(result.Score, 0.5)
				result.Reasons = append(result.Reasons, "Matches moderate pattern: multi-file changes")
				break
			}
		}
	}

	// Check for persistence needs
	for _, pattern := range persistencePatterns {
		if pattern.MatchString(lower) {
			result.NeedsPersistent = true
			result.Score = max(result.Score, 0.4)
			result.Reasons = append(result.Reasons, "Requires persistent monitoring or learning")
			break
		}
	}

	// Check multi-file indicators
	for _, indicator := range multiFileIndicators {
		if strings.Contains(lower, indicator) {
			result.EstimatedFiles += 5
			if result.Level < ComplexityModerate {
				result.Level = ComplexityModerate
				result.Score = max(result.Score, 0.5)
			}
			result.Reasons = append(result.Reasons, "Multi-file operation indicated")
			break
		}
	}

	// Extract suggested phases
	for keyword, phase := range phaseIndicators {
		if strings.Contains(lower, keyword) {
			result.SuggestedPhases = appendUnique(result.SuggestedPhases, phase)
		}
	}

	// Boost complexity if multiple phases detected
	if len(result.SuggestedPhases) >= 3 {
		if result.Level < ComplexityComplex {
			result.Level = ComplexityComplex
			result.NeedsCampaign = true
			result.Score = max(result.Score, 0.7)
		}
		result.Reasons = append(result.Reasons, "Multiple phases detected")
	}

	// Check target for complexity hints
	if target != "" && target != "none" && target != "codebase" {
		// Specific file target reduces complexity
		if strings.Contains(target, ".") && !strings.Contains(target, "*") {
			result.EstimatedFiles = 1
		}
	} else if target == "codebase" || strings.Contains(lower, "codebase") {
		result.EstimatedFiles = 10 // Assume many files
		if result.Level < ComplexityModerate {
			result.Level = ComplexityModerate
			result.Score = max(result.Score, 0.5)
		}
	}

	// Campaign threshold
	if result.Score >= 0.7 || result.Level >= ComplexityComplex {
		result.NeedsCampaign = true
	}

	return result
}

// AnalyzeWithLLM uses the LLM for deeper complexity analysis
func (ca *ComplexityAnalyzer) AnalyzeWithLLM(ctx context.Context, input string) (ComplexityResult, error) {
	// First do heuristic analysis
	result := ca.Analyze(ctx, input, "")

	// If heuristics are confident, skip LLM
	if result.Score > 0.8 || result.Score < 0.3 {
		return result, nil
	}

	// Use LLM for ambiguous cases
	prompt := `Analyze this task for complexity. Return JSON only:
{
  "complexity": "simple|moderate|complex|epic",
  "needs_campaign": true/false,
  "needs_persistent": true/false,
  "estimated_files": number,
  "phases": ["phase1", "phase2"],
  "reasoning": "brief explanation"
}

Task: "` + input + `"

JSON only:`

	resp, err := ca.client.Complete(ctx, prompt)
	if err != nil {
		return result, nil // Fall back to heuristic result
	}

	// Parse LLM response and merge with heuristics
	// (simplified - in production you'd parse JSON properly)
	if strings.Contains(strings.ToLower(resp), `"epic"`) {
		result.Level = ComplexityEpic
		result.Score = 0.95
		result.NeedsCampaign = true
	} else if strings.Contains(strings.ToLower(resp), `"complex"`) {
		result.Level = ComplexityComplex
		result.Score = max(result.Score, 0.75)
		result.NeedsCampaign = true
	}

	if strings.Contains(resp, `"needs_campaign": true`) || strings.Contains(resp, `"needs_campaign":true`) {
		result.NeedsCampaign = true
	}

	if strings.Contains(resp, `"needs_persistent": true`) || strings.Contains(resp, `"needs_persistent":true`) {
		result.NeedsPersistent = true
	}

	return result, nil
}

// Helper functions

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
