package coder

import "fmt"

// =============================================================================
// AUTOPOIESIS (SELF-IMPROVEMENT)
// =============================================================================

// trackRejection tracks a rejection pattern for autopoiesis.
func (c *CoderShard) trackRejection(action, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%s:%s", action, reason)
	c.rejectionCount[key]++

	// Persist to LearningStore if count exceeds threshold
	if c.learningStore != nil && c.rejectionCount[key] >= 2 {
		_ = c.learningStore.Save("coder", "avoid_pattern", []any{action, reason}, "")
	}
}

// trackAcceptance tracks an acceptance pattern for autopoiesis.
func (c *CoderShard) trackAcceptance(action string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acceptanceCount[action]++

	// Persist to LearningStore if count exceeds threshold
	if c.learningStore != nil && c.acceptanceCount[action] >= 3 {
		_ = c.learningStore.Save("coder", "preferred_pattern", []any{action}, "")
	}
}

// loadLearnedPatterns loads existing patterns from LearningStore on initialization.
// Must be called with lock held.
func (c *CoderShard) loadLearnedPatterns() {
	if c.learningStore == nil {
		return
	}

	// Load rejection patterns
	rejectionLearnings, err := c.learningStore.LoadByPredicate("coder", "avoid_pattern")
	if err == nil {
		for _, learning := range rejectionLearnings {
			if len(learning.FactArgs) >= 2 {
				action, _ := learning.FactArgs[0].(string)
				reason, _ := learning.FactArgs[1].(string)
				key := fmt.Sprintf("%s:%s", action, reason)
				// Initialize with threshold count to avoid re-learning
				c.rejectionCount[key] = 2
			}
		}
	}

	// Load acceptance patterns
	acceptanceLearnings, err := c.learningStore.LoadByPredicate("coder", "preferred_pattern")
	if err == nil {
		for _, learning := range acceptanceLearnings {
			if len(learning.FactArgs) >= 1 {
				action, _ := learning.FactArgs[0].(string)
				// Initialize with threshold count
				c.acceptanceCount[action] = 3
			}
		}
	}
}
