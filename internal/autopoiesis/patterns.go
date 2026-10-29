package autopoiesis

import (
	"fmt"
	"sync"
	"time"

	"codenerd/internal/logging"
)

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
	PatternID   string                  `json:"pattern_id"`
	ToolName    string                  `json:"tool_name"`
	IssueType   IssueType               `json:"issue_type"`
	Occurrences int                     `json:"occurrences"`
	FirstSeen   time.Time               `json:"first_seen"`
	LastSeen    time.Time               `json:"last_seen"`
	Confidence  float64                 `json:"confidence"`
	Examples    []string                `json:"examples"`
	Suggestions []ImprovementSuggestion `json:"suggestions"`
}

// NewPatternDetector creates a new pattern detector
func NewPatternDetector() *PatternDetector {
	logging.AutopoiesisDebug("Creating PatternDetector")
	return &PatternDetector{
		history:  []ExecutionFeedback{},
		patterns: make(map[string]*DetectedPattern),
	}
}

// RecordExecution adds an execution to history and updates patterns
func (pd *PatternDetector) RecordExecution(feedback ExecutionFeedback) {
	logging.AutopoiesisDebug("Recording execution for pattern detection: tool=%s, success=%v",
		feedback.ToolName, feedback.Success)

	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.history = append(pd.history, feedback)

	// Limit history size
	if len(pd.history) > 1000 {
		logging.AutopoiesisDebug("Trimming history from 1000 to 900 entries")
		pd.history = pd.history[100:] // Keep last 900
	}

	// Update patterns based on quality issues
	if feedback.Quality != nil {
		issueCount := len(feedback.Quality.Issues)
		if issueCount > 0 {
			logging.AutopoiesisDebug("Processing %d quality issues for pattern detection", issueCount)
		}

		for _, issue := range feedback.Quality.Issues {
			patternKey := fmt.Sprintf("%s:%s", feedback.ToolName, issue.Type)

			pattern, exists := pd.patterns[patternKey]
			if !exists {
				logging.Autopoiesis("New pattern detected: %s (issue=%s)", patternKey, issue.Type)
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

			prevConfidence := pattern.Confidence
			pattern.Occurrences++
			pattern.LastSeen = time.Now()
			pattern.Confidence = calculatePatternConfidence(pattern.Occurrences)

			// Log confidence increases
			if pattern.Confidence > prevConfidence {
				logging.AutopoiesisDebug("Pattern confidence increased: %s (%.2f -> %.2f, occurrences=%d)",
					patternKey, prevConfidence, pattern.Confidence, pattern.Occurrences)
			}

			// Log when pattern becomes high-confidence
			if prevConfidence < 0.7 && pattern.Confidence >= 0.7 {
				logging.Autopoiesis("Pattern reached high confidence: %s (confidence=%.2f, occurrences=%d)",
					patternKey, pattern.Confidence, pattern.Occurrences)
			}

			// Add example (limit to 5)
			if len(pattern.Examples) < 5 {
				pattern.Examples = append(pattern.Examples, issue.Evidence)
			}

			// Merge suggestions
			if feedback.Quality != nil {
				for _, sug := range feedback.Quality.Suggestions {
					if !hasSuggestion(pattern.Suggestions, sug.Type) {
						pattern.Suggestions = append(pattern.Suggestions, sug)
						logging.AutopoiesisDebug("Added suggestion to pattern %s: %s", patternKey, sug.Type)
					}
				}
			}
		}
	}

	logging.AutopoiesisDebug("Pattern detection complete: total patterns=%d, history size=%d",
		len(pd.patterns), len(pd.history))
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
