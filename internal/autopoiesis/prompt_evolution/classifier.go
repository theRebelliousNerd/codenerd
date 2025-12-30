package prompt_evolution

import (
	"context"
	"regexp"
	"strings"

	"codenerd/internal/logging"
)

// ProblemClassifier classifies task requests into problem types.
// This enables strategy selection and failure grouping by problem category.
type ProblemClassifier struct {
	// Compiled patterns for each problem type
	patterns map[ProblemType][]*regexp.Regexp
}

// NewProblemClassifier creates a new problem classifier.
func NewProblemClassifier() *ProblemClassifier {
	pc := &ProblemClassifier{
		patterns: make(map[ProblemType][]*regexp.Regexp),
	}
	pc.compilePatterns()
	return pc
}

// Classify determines the problem type for a task request.
// Returns the problem type and confidence score (0.0-1.0).
func (pc *ProblemClassifier) Classify(taskRequest string) (ProblemType, float64) {
	normalized := strings.ToLower(taskRequest)

	// Score each problem type
	scores := make(map[ProblemType]float64)
	maxScore := 0.0
	var maxType ProblemType = ProblemDebugging // Default

	for problemType, patterns := range pc.patterns {
		score := 0.0
		for _, pattern := range patterns {
			if pattern.MatchString(normalized) {
				score += 1.0
			}
		}

		// Normalize by number of patterns
		if len(patterns) > 0 {
			score = score / float64(len(patterns))
		}

		scores[problemType] = score
		if score > maxScore {
			maxScore = score
			maxType = problemType
		}
	}

	// Apply keyword boosters
	maxType, maxScore = pc.applyKeywordBoosters(normalized, maxType, maxScore)

	// Minimum confidence threshold
	if maxScore < 0.1 {
		maxScore = 0.3 // Low confidence default
	}

	// Cap confidence
	if maxScore > 1.0 {
		maxScore = 1.0
	}

	logging.AutopoiesisDebug("Classified task: type=%s, confidence=%.2f", maxType, maxScore)
	return maxType, maxScore
}

// ClassifyWithContext uses additional context for classification.
func (pc *ProblemClassifier) ClassifyWithContext(ctx context.Context, taskRequest string, shardType string) (ProblemType, float64) {
	baseType, baseConf := pc.Classify(taskRequest)

	// Boost confidence based on shard type alignment
	switch shardType {
	case "/tester":
		if baseType == ProblemTesting {
			baseConf = min(baseConf+0.2, 1.0)
		}
	case "/reviewer":
		if baseType == ProblemCodeReview {
			baseConf = min(baseConf+0.2, 1.0)
		}
	case "/researcher":
		if baseType == ProblemResearch || baseType == ProblemDocumentation {
			baseConf = min(baseConf+0.2, 1.0)
		}
	}

	return baseType, baseConf
}

// compilePatterns compiles regex patterns for each problem type.
func (pc *ProblemClassifier) compilePatterns() {
	// Debugging patterns
	pc.patterns[ProblemDebugging] = compileAll(
		`\b(fix|debug|bug|error|issue|problem|broken|crash|fail)\b`,
		`\bnot working\b`,
		`\bwhy (is|does|doesn't|isn't)\b`,
		`\bstack trace\b`,
		`\bpanic\b`,
		`\bnil pointer\b`,
	)

	// Feature creation patterns
	pc.patterns[ProblemFeatureCreation] = compileAll(
		`\b(add|create|implement|build|new|introduce)\b`,
		`\b(feature|functionality|capability|support)\b`,
		`\bintegrate\b`,
		`\bwant\s+(to\s+)?(add|create|have)\b`,
		`\bneed\s+(a\s+)?new\b`,
	)

	// Refactoring patterns
	pc.patterns[ProblemRefactoring] = compileAll(
		`\b(refactor|restructure|reorganize|clean up|improve|optimize)\b`,
		`\b(simplify|consolidate|merge)\b`,
		`\bextract\s+(method|function|class|interface)\b`,
		`\brename\b`,
		`\bmove\s+(to|into)\b`,
	)

	// Testing patterns
	pc.patterns[ProblemTesting] = compileAll(
		`\b(test|tests|testing|unittest|unit test)\b`,
		`\bcoverage\b`,
		`\bmock\b`,
		`\btest\s+(case|suite|file)\b`,
		`\bwrite\s+tests?\b`,
		`\badd\s+tests?\b`,
	)

	// Documentation patterns
	pc.patterns[ProblemDocumentation] = compileAll(
		`\b(document|documentation|docs|readme|comment|comments)\b`,
		`\bexplain\b`,
		`\bdescribe\b`,
		`\badd\s+(comment|doc|documentation)\b`,
		`\bupdate\s+(comment|doc|documentation|readme)\b`,
	)

	// Performance patterns
	pc.patterns[ProblemPerformance] = compileAll(
		`\b(performance|perf|optimize|optimization|speed|slow|fast)\b`,
		`\b(memory|cpu|latency|throughput)\b`,
		`\bbenchmark\b`,
		`\btoo slow\b`,
		`\bprofil(e|ing)\b`,
	)

	// Security patterns
	pc.patterns[ProblemSecurity] = compileAll(
		`\b(security|secure|vulnerability|exploit|attack)\b`,
		`\b(injection|xss|csrf|auth|authentication|authorization)\b`,
		`\bsanitize\b`,
		`\bescape\b`,
		`\bvalidate\s+(input|user)\b`,
	)

	// API integration patterns
	pc.patterns[ProblemAPIIntegration] = compileAll(
		`\b(api|endpoint|rest|graphql|grpc)\b`,
		`\bintegrat(e|ion)\b`,
		`\b(http|request|response)\b`,
		`\bclient\b`,
		`\bwebhook\b`,
	)

	// Data migration patterns
	pc.patterns[ProblemDataMigration] = compileAll(
		`\b(migration|migrate|transform|convert)\b`,
		`\b(schema|database|db)\b`,
		`\bupgrade\b`,
		`\bdata\s+(format|structure)\b`,
	)

	// Config setup patterns
	pc.patterns[ProblemConfigSetup] = compileAll(
		`\b(config|configuration|configure|setup|settings)\b`,
		`\b(environment|env|variable)\b`,
		`\b(yaml|json|toml)\s+(file|config)\b`,
		`\binitialize\b`,
	)

	// Error handling patterns
	pc.patterns[ProblemErrorHandling] = compileAll(
		`\b(error\s+handling|handle\s+error|error\s+message)\b`,
		`\breturn\s+error\b`,
		`\bwrap\s+error\b`,
		`\berror\s+case\b`,
		`\bgraceful\b`,
	)

	// Concurrency patterns
	pc.patterns[ProblemConcurrency] = compileAll(
		`\b(concurrent|concurrency|parallel|async|goroutine)\b`,
		`\b(thread|mutex|lock|deadlock|race)\b`,
		`\bchannel\b`,
		`\bsync\b`,
		`\bwaitgroup\b`,
	)

	// Type system patterns
	pc.patterns[ProblemTypeSystem] = compileAll(
		`\b(type|types|interface|struct|generic)\b`,
		`\b(typedef|type\s+alias)\b`,
		`\btype\s+assertion\b`,
		`\btype\s+(safe|safety)\b`,
	)

	// Dependency management patterns
	pc.patterns[ProblemDependencyMgmt] = compileAll(
		`\b(dependency|dependencies|package|module|import)\b`,
		`\b(upgrade|update|version)\b`,
		`\bgo\s+mod\b`,
		`\bnpm|yarn|pip\b`,
		`\bvendor\b`,
	)

	// Code review patterns
	pc.patterns[ProblemCodeReview] = compileAll(
		`\b(review|check|audit|inspect)\b`,
		`\blook\s+(at|over)\b`,
		`\bfeedback\b`,
		`\bwhat\s+do\s+you\s+think\b`,
		`\bsuggestions?\b`,
	)

	// Research patterns
	pc.patterns[ProblemResearch] = compileAll(
		`\b(research|investigate|explore|find|search)\b`,
		`\bhow\s+(do|does|can|to)\b`,
		`\bwhat\s+is\b`,
		`\blearn\s+about\b`,
		`\bunderstand\b`,
	)
}

// applyKeywordBoosters applies additional keyword-based boosting.
func (pc *ProblemClassifier) applyKeywordBoosters(text string, currentType ProblemType, currentScore float64) (ProblemType, float64) {
	// Strong indicators that override pattern matching
	strongIndicators := map[string]ProblemType{
		"fix the bug":         ProblemDebugging,
		"add a new":           ProblemFeatureCreation,
		"write tests":         ProblemTesting,
		"add tests":           ProblemTesting,
		"refactor this":       ProblemRefactoring,
		"clean up":            ProblemRefactoring,
		"performance issue":   ProblemPerformance,
		"security issue":      ProblemSecurity,
		"security vulnerab":   ProblemSecurity,
		"error handling":      ProblemErrorHandling,
		"race condition":      ProblemConcurrency,
		"deadlock":            ProblemConcurrency,
		"update the docs":     ProblemDocumentation,
		"add documentation":   ProblemDocumentation,
		"review the code":     ProblemCodeReview,
		"research":            ProblemResearch,
		"how do i":            ProblemResearch,
		"upgrade dependency":  ProblemDependencyMgmt,
		"update dependencies": ProblemDependencyMgmt,
	}

	for indicator, problemType := range strongIndicators {
		if strings.Contains(text, indicator) {
			if problemType != currentType {
				return problemType, max(currentScore, 0.8)
			}
			return currentType, max(currentScore, 0.9)
		}
	}

	return currentType, currentScore
}

// compileAll compiles multiple regex patterns.
func compileAll(patterns ...string) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if r, err := regexp.Compile(p); err == nil {
			result = append(result, r)
		}
	}
	return result
}

// min returns the minimum of two float64 values.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two float64 values.
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// GetProblemTypeDescription returns a human-readable description.
func GetProblemTypeDescription(pt ProblemType) string {
	descriptions := map[ProblemType]string{
		ProblemDebugging:       "Fixing bugs and resolving errors",
		ProblemFeatureCreation: "Creating new features and functionality",
		ProblemRefactoring:     "Improving code structure and quality",
		ProblemTesting:         "Writing and improving tests",
		ProblemDocumentation:   "Writing documentation and comments",
		ProblemPerformance:     "Optimizing speed and resource usage",
		ProblemSecurity:        "Fixing security vulnerabilities",
		ProblemAPIIntegration:  "Integrating with external APIs",
		ProblemDataMigration:   "Migrating and transforming data",
		ProblemConfigSetup:     "Setting up configuration",
		ProblemErrorHandling:   "Improving error handling",
		ProblemConcurrency:     "Working with concurrent code",
		ProblemTypeSystem:      "Working with types and interfaces",
		ProblemDependencyMgmt:  "Managing dependencies",
		ProblemCodeReview:      "Reviewing and auditing code",
		ProblemResearch:        "Researching and exploring",
	}

	if desc, ok := descriptions[pt]; ok {
		return desc
	}
	return "General task"
}
