package coder

import (
	"codenerd/internal/core"
	"time"
)

// =============================================================================
// FACT GENERATION
// =============================================================================

// generateFacts creates Mangle facts from the result for propagation.
func (c *CoderShard) generateFacts(result *CoderResult) []core.Fact {
	facts := make([]core.Fact, 0)

	// File modifications
	for _, edit := range result.Edits {
		facts = append(facts, core.Fact{
			Predicate: "modified",
			Args:      []interface{}{edit.File},
		})

		// File topology
		hash := hashContent(edit.NewContent)
		facts = append(facts, core.Fact{
			Predicate: "file_topology",
			Args: []interface{}{
				edit.File,
				hash,
				core.MangleAtom("/" + edit.Language),
				time.Now().Unix(),
				isTestFile(edit.File),
			},
		})
	}

	// Build state
	if result.BuildPassed {
		facts = append(facts, core.Fact{
			Predicate: "build_state",
			Args:      []interface{}{core.MangleAtom("/passing")},
		})
	} else {
		facts = append(facts, core.Fact{
			Predicate: "build_state",
			Args:      []interface{}{core.MangleAtom("/failing")},
		})
	}

	// Diagnostics
	for _, diag := range result.Diagnostics {
		facts = append(facts, diag.ToFact())
	}

	// Autopoiesis: check for patterns to promote
	c.mu.RLock()
	for pattern, count := range c.rejectionCount {
		if count >= 2 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"style_preference", pattern},
			})
		}
	}
	for pattern, count := range c.acceptanceCount {
		if count >= 3 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"preferred_pattern", pattern},
			})
		}
	}
	c.mu.RUnlock()

	return facts
}

// assertTaskFacts asserts task-related facts to the kernel.
func (c *CoderShard) assertTaskFacts(task CoderTask) {
	if c.kernel == nil {
		return
	}

	// Assert the coding task
	_ = c.kernel.Assert(core.Fact{
		Predicate: "coder_task",
		Args:      []interface{}{task.Action, task.Target, task.Instruction},
	})

	// Assert target file
	if task.Target != "" {
		_ = c.kernel.Assert(core.Fact{
			Predicate: "coder_target",
			Args:      []interface{}{task.Target},
		})
	}
}
