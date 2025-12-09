package coder

import (
	"codenerd/internal/logging"
	"fmt"
)

// =============================================================================
// AUTOPOIESIS (SELF-IMPROVEMENT)
// =============================================================================

// trackRejection tracks a rejection pattern for autopoiesis.
func (c *CoderShard) trackRejection(action, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%s:%s", action, reason)
	c.rejectionCount[key]++

	logging.Autopoiesis("Coder rejection tracked: action=%s, reason=%s, count=%d",
		action, reason, c.rejectionCount[key])

	// Persist to LearningStore if count exceeds threshold
	if c.learningStore != nil && c.rejectionCount[key] >= 2 {
		logging.Autopoiesis("Coder persisting avoid_pattern: %s:%s (threshold reached)", action, reason)
		_ = c.learningStore.Save("coder", "avoid_pattern", []any{action, reason}, "")
	}
}

// trackAcceptance tracks an acceptance pattern for autopoiesis.
func (c *CoderShard) trackAcceptance(action string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acceptanceCount[action]++

	logging.Autopoiesis("Coder acceptance tracked: action=%s, count=%d",
		action, c.acceptanceCount[action])

	// Persist to LearningStore if count exceeds threshold
	if c.learningStore != nil && c.acceptanceCount[action] >= 3 {
		logging.Autopoiesis("Coder persisting preferred_pattern: %s (threshold reached)", action)
		_ = c.learningStore.Save("coder", "preferred_pattern", []any{action}, "")
	}
}

// loadLearnedPatterns loads existing patterns from LearningStore on initialization.
// Must be called with lock held.
func (c *CoderShard) loadLearnedPatterns() {
	if c.learningStore == nil {
		logging.AutopoiesisDebug("Coder: no LearningStore configured, skipping pattern load")
		return
	}

	logging.Autopoiesis("Coder loading learned patterns from LearningStore")

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
		logging.Autopoiesis("Coder loaded %d rejection patterns", len(rejectionLearnings))
	} else {
		logging.AutopoiesisDebug("Coder: failed to load rejection patterns: %v", err)
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
		logging.Autopoiesis("Coder loaded %d acceptance patterns", len(acceptanceLearnings))
	} else {
		logging.AutopoiesisDebug("Coder: failed to load acceptance patterns: %v", err)
	}
}
