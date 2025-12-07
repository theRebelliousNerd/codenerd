package reviewer

import (
	"codenerd/internal/core"
	"strings"
	"time"
)

// =============================================================================
// FACT GENERATION
// =============================================================================

// assertInitialFacts asserts initial facts to the kernel.
func (r *ReviewerShard) assertInitialFacts(task *ReviewerTask) {
	if r.kernel == nil {
		return
	}

	_ = r.kernel.Assert(core.Fact{
		Predicate: "reviewer_task",
		Args:      []interface{}{r.id, "/" + task.Action, strings.Join(task.Files, ","), time.Now().Unix()},
	})
}

// generateFacts generates facts from review results for propagation.
func (r *ReviewerShard) generateFacts(result *ReviewResult) []core.Fact {
	facts := make([]core.Fact, 0)

	// Review status
	facts = append(facts, core.Fact{
		Predicate: "review_complete",
		Args:      []interface{}{strings.Join(result.Files, ","), "/" + string(result.Severity)},
	})

	// Block commit fact
	if result.BlockCommit {
		facts = append(facts, core.Fact{
			Predicate: "block_commit",
			Args:      []interface{}{"critical_review_findings"},
		})
	}

	// Individual findings
	for _, finding := range result.Findings {
		facts = append(facts, core.Fact{
			Predicate: "review_finding",
			Args: []interface{}{
				finding.File,
				int64(finding.Line),
				"/" + finding.Severity,
				finding.Category,
				finding.Message,
			},
		})

		// Security-specific facts
		if finding.Category == "security" && (finding.Severity == "critical" || finding.Severity == "error") {
			facts = append(facts, core.Fact{
				Predicate: "security_issue",
				Args:      []interface{}{finding.File, int64(finding.Line), finding.RuleID, finding.Message},
			})
		}
	}

	// Metrics facts
	if result.Metrics != nil {
		facts = append(facts, core.Fact{
			Predicate: "code_metrics",
			Args: []interface{}{
				int64(result.Metrics.TotalLines),
				int64(result.Metrics.CodeLines),
				result.Metrics.CyclomaticAvg,
				int64(result.Metrics.FunctionCount),
			},
		})
	}

	// Autopoiesis facts
	r.mu.RLock()
	for pattern, count := range r.flaggedPatterns {
		if count >= 3 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"anti_pattern", pattern},
			})
		}
	}
	for pattern, count := range r.approvedPatterns {
		if count >= 5 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"approved_style", pattern},
			})
		}
	}
	r.mu.RUnlock()

	// Specialist recommendation facts
	// These enable the main agent and kernel to suggest specialists for follow-up tasks
	for _, rec := range result.SpecialistRecommendations {
		// suggest_use_specialist(TaskType, SpecialistName)
		facts = append(facts, core.Fact{
			Predicate: "suggest_use_specialist",
			Args:      []interface{}{"/" + rec.ShardName, rec.Reason},
		})

		// shard_can_handle(ShardType, TaskType) for each task hint
		for _, hint := range rec.TaskHints {
			facts = append(facts, core.Fact{
				Predicate: "shard_can_handle",
				Args:      []interface{}{"/" + rec.ShardName, hint},
			})
		}

		// specialist_recommended(ShardName, FilePath, Confidence)
		for _, file := range rec.ForFiles {
			facts = append(facts, core.Fact{
				Predicate: "specialist_recommended",
				Args:      []interface{}{"/" + rec.ShardName, file, rec.Confidence},
			})
		}
	}

	return facts
}
