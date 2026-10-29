package autopoiesis

import (
	"context"
	"fmt"

	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// OutputJudgments is Jev's reading of one execution's output: the probability
// that each learnjudge.Output* judgment is true, and judgment_yes_percent as a
// probability.
type OutputJudgments struct {
	Probabilities map[string]float64
	Yes           float64
}

// outputRuleJudgment pairs each pattern rule that reads language with the Jev
// judgment that replaces it. A rule not named here (empty_or_minimal) is an
// exact test of the output and always runs in code.
var outputRuleJudgment = map[string]string{
	"pagination_truncated": learnjudge.OutputReportsMorePages,
	"partial_results":      learnjudge.OutputReportsPartial,
	"error_in_output":      learnjudge.OutputReportsError,
	"rate_limited":         learnjudge.OutputReportsRateLimit,
}

// outputIssues names the issues an output shows. Truth table, per rule that
// reads language:
//
//	Jev answered, at or above yes   the issue, with Jev's probability as evidence
//	Jev answered, below yes         no issue (a no or an unsettled reading)
//	Jev off, failed or timed out    the rule's pattern decides, as before
//
// Exact rules run whatever Jev says.
func (qe *QualityEvaluator) outputIssues(ctx context.Context, feedback *ExecutionFeedback) []QualityIssue {
	var judged *OutputJudgments
	if qe.outputJudge != nil {
		j, err := qe.outputJudge(ctx, feedback)
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("jev: output of %s unjudged, the pattern rules decide: %v", feedback.ToolName, err)
		} else {
			judged = &j
		}
	}
	var issues []QualityIssue
	for _, rule := range qe.heuristicRules {
		name, readsLanguage := outputRuleJudgment[rule.Name]
		if readsLanguage && judged != nil {
			p, ok := judged.Probabilities[name]
			if !ok || p < judged.Yes {
				continue
			}
			issues = append(issues, QualityIssue{
				Type:        rule.IssueType,
				Severity:    rule.Severity,
				Description: rule.Description,
				Evidence:    fmt.Sprintf("Jev %d%%: %s", types.PercentFromRatio(p), name),
				Fixable:     true,
			})
			continue
		}
		if rule.Pattern.MatchString(feedback.Output) {
			issues = append(issues, QualityIssue{
				Type:        rule.IssueType,
				Severity:    rule.Severity,
				Description: rule.Description,
				Evidence:    extractMatch(feedback.Output, rule.Pattern),
				Fixable:     true,
			})
		}
	}
	return issues
}
