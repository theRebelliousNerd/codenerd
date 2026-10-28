package northstar

import (
	"fmt"
	"strings"
)

// Alignment verdicts: turning a model response or an unavailable assessment
// into an AlignmentResult, and grading scores against the configured bands.

// markUnknown records that no assessment happened. Score is zero because there
// is no score; consumers must branch on Result, never on Score, for unknown.
func markUnknown(check *AlignmentCheck, explanation string) {
	check.Result = AlignmentUnknown
	check.Score = 0
	check.Explanation = explanation
}

// alignmentSeverity orders results so the stricter of two verdicts can win.
func alignmentSeverity(r AlignmentResult) int {
	switch r {
	case AlignmentPassed:
		return 0
	case AlignmentWarning:
		return 1
	case AlignmentFailed:
		return 2
	case AlignmentBlocked:
		return 3
	default:
		return -1
	}
}

// parseAlignmentResponse reads the SCORE/RESULT/EXPLANATION/SUGGESTIONS
// contract (atom northstar/guardian/output_contract).
//
// Rulings (NS-VISION-02):
//   - No usable SCORE and no usable RESULT (empty, prose, garbage) is unknown.
//     It used to keep a default 0.7 that the default warning threshold
//     classified as passed.
//   - SCORE without RESULT classifies the score.
//   - RESULT without SCORE is honoured only when it is non-passing: a bare
//     "passed" is an unsupported claim and is unknown, but a bare "blocked" is
//     never weakened. Its score is the bottom of its band.
//   - Both present and disagreeing: the stricter verdict wins, so a "passed"
//     label cannot launder a failing score and a "blocked" label is never
//     overridden by a high one.
func (g *Guardian) parseAlignmentResponse(response string, check *AlignmentCheck) {
	check.Score = 0
	check.Result = AlignmentUnknown
	check.Explanation = ""
	var explicit AlignmentResult
	scoreFound := false

	lines := strings.SplitSeq(response, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		line = strings.ReplaceAll(line, "\"", "")
		line = strings.TrimSuffix(line, ",")

		if after, ok := strings.CutPrefix(line, "SCORE:"); ok {
			var score float64
			if _, err := fmt.Sscanf(after, "%f", &score); err == nil && score >= 0 && score <= 1 {
				check.Score = score
				scoreFound = true
			}
		} else if after, ok := strings.CutPrefix(line, "RESULT:"); ok {
			switch r := AlignmentResult(strings.ToLower(strings.TrimSpace(after))); r {
			case AlignmentPassed, AlignmentWarning, AlignmentFailed, AlignmentBlocked:
				explicit = r
			}
		} else if after, ok := strings.CutPrefix(line, "EXPLANATION:"); ok {
			check.Explanation = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "SUGGESTIONS:"); ok {
			sugStr := strings.TrimSpace(after)
			if sugStr != "none" && sugStr != "" {
				for s := range strings.SplitSeq(sugStr, ",") {
					if s = strings.TrimSpace(s); s != "" {
						check.Suggestions = append(check.Suggestions, s)
					}
				}
			}
		}
	}

	switch {
	case scoreFound && explicit != "":
		check.Result = explicit
		if scored := g.classifyScore(check.Score); alignmentSeverity(scored) > alignmentSeverity(explicit) {
			check.Result = scored
		}
	case scoreFound:
		check.Result = g.classifyScore(check.Score)
	case explicit != "" && explicit != AlignmentPassed:
		check.Result = explicit
		check.Score = g.bandFloor(explicit)
	default:
		suggestions := check.Suggestions
		markUnknown(check, "Alignment response carried no usable SCORE/RESULT verdict")
		check.Suggestions = suggestions
		return
	}
	if check.Explanation == "" {
		check.Explanation = "No explanation given"
	}
}

// bandFloor is the lowest score that still classifies as r: the most
// pessimistic score consistent with a verdict that arrived without one.
func (g *Guardian) bandFloor(r AlignmentResult) float64 {
	switch r {
	case AlignmentWarning:
		return g.config.FailureThreshold
	case AlignmentFailed:
		return g.config.BlockThreshold
	default:
		return 0
	}
}

func (g *Guardian) scoreToSeverity(score float64) DriftSeverity {
	switch g.classifyScore(score) {
	case AlignmentPassed:
		return DriftMinor
	case AlignmentWarning:
		return DriftModerate
	case AlignmentFailed:
		return DriftMajor
	default:
		return DriftCritical
	}
}

func (g *Guardian) classifyScore(score float64) AlignmentResult {
	if score >= g.config.WarningThreshold {
		return AlignmentPassed
	}
	if score >= g.config.FailureThreshold {
		return AlignmentWarning
	}
	if score >= g.config.BlockThreshold {
		return AlignmentFailed
	}
	return AlignmentBlocked
}
