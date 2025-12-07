package tester

import (
	"codenerd/internal/core"
	"time"
)

// =============================================================================
// FACT GENERATION
// =============================================================================

// assertInitialFacts asserts initial facts to the kernel.
func (t *TesterShard) assertInitialFacts(task *TesterTask) {
	if t.kernel == nil {
		return
	}

	_ = t.kernel.Assert(core.Fact{
		Predicate: "tester_task",
		Args:      []interface{}{t.id, "/" + task.Action, task.Target, time.Now().Unix()},
	})

	_ = t.kernel.Assert(core.Fact{
		Predicate: "coverage_goal",
		Args:      []interface{}{t.testerConfig.CoverageGoal},
	})
}

// generateFacts generates facts from test results for propagation.
func (t *TesterShard) generateFacts(result *TestResult) []core.Fact {
	facts := make([]core.Fact, 0)

	// Test state
	stateAtom := "/passing"
	if !result.Passed {
		stateAtom = "/failing"
	}
	facts = append(facts, core.Fact{
		Predicate: "test_state",
		Args:      []interface{}{stateAtom},
	})

	// Test type
	if result.TestType != "" && result.TestType != "unknown" {
		facts = append(facts, core.Fact{
			Predicate: "test_type",
			Args:      []interface{}{"/" + result.TestType},
		})
	}

	// Test output
	facts = append(facts, core.Fact{
		Predicate: "test_output",
		Args:      []interface{}{truncateString(result.Output, 1000)},
	})

	// Coverage metric
	if result.Coverage > 0 {
		facts = append(facts, core.Fact{
			Predicate: "coverage_metric",
			Args:      []interface{}{result.Coverage},
		})

		// Check against goal
		if result.Coverage < t.testerConfig.CoverageGoal {
			facts = append(facts, core.Fact{
				Predicate: "coverage_below_goal",
				Args:      []interface{}{result.Coverage, t.testerConfig.CoverageGoal},
			})
		}
	}

	// Retry count
	facts = append(facts, core.Fact{
		Predicate: "retry_count",
		Args:      []interface{}{int64(result.Retries)},
	})

	// Failed tests
	for _, failed := range result.FailedTests {
		facts = append(facts, core.Fact{
			Predicate: "failed_test",
			Args:      []interface{}{failed.Name, failed.FilePath, failed.Message},
		})
	}

	// Diagnostics
	for _, diag := range result.Diagnostics {
		facts = append(facts, diag.ToFact())
	}

	// Autopoiesis facts
	t.mu.RLock()
	for pattern, count := range t.failurePatterns {
		if count >= 3 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"avoid_pattern", pattern},
			})
		}
	}
	for pattern, count := range t.successPatterns {
		if count >= 5 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"test_template", pattern},
			})
		}
	}
	t.mu.RUnlock()

	return facts
}
