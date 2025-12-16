package core

import (
	"time"
)

// =============================================================================
// SHARD RESULT TO FACTS CONVERSION (Cross-Turn Context Propagation)
// =============================================================================
// These methods convert shard execution results into Mangle facts that can be
// loaded into the kernel for persistence across conversation turns.

// ResultToFacts converts a shard execution result into kernel-loadable facts.
// This is the key bridge between shard execution and kernel state.
func (sm *ShardManager) ResultToFacts(shardID, shardType, task, result string, err error) []Fact {
	facts := make([]Fact, 0, 5)
	timestamp := time.Now().Unix()

	// Core execution fact
	facts = append(facts, Fact{
		Predicate: "shard_executed",
		Args:      []interface{}{shardID, shardType, task, timestamp},
	})

	// Track last execution for quick reference
	facts = append(facts, Fact{
		Predicate: "last_shard_execution",
		Args:      []interface{}{shardID, shardType, task},
	})

	if err != nil {
		// Record failure
		facts = append(facts, Fact{
			Predicate: "shard_error",
			Args:      []interface{}{shardID, err.Error()},
		})
	} else {
		// Record success
		facts = append(facts, Fact{
			Predicate: "shard_success",
			Args:      []interface{}{shardID},
		})

		// Store output (truncate if too long for kernel)
		output := result
		if len(output) > 4000 {
			output = output[:4000] + "... (truncated)"
		}
		facts = append(facts, Fact{
			Predicate: "shard_output",
			Args:      []interface{}{shardID, output},
		})

		// Create compressed context for LLM injection
		summary := sm.extractSummary(shardType, result)
		facts = append(facts, Fact{
			Predicate: "recent_shard_context",
			Args:      []interface{}{shardType, task, summary, timestamp},
		})

		// Parse shard-specific structured facts
		structuredFacts := sm.parseShardSpecificFacts(shardID, shardType, result)
		facts = append(facts, structuredFacts...)
	}

	return facts
}

// extractSummary creates a compressed summary of shard output for context injection.
func (sm *ShardManager) extractSummary(shardType, result string) string {
	// Extract key information based on shard type
	lines := splitLines(result)

	switch shardType {
	case "reviewer":
		// Look for summary lines in review output
		for _, line := range lines {
			if contains(line, "PASSED") || contains(line, "FAILED") ||
				contains(line, "critical") || contains(line, "error") ||
				contains(line, "warning") || contains(line, "info") {
				return truncateString(line, 200)
			}
		}
	case "tester":
		// Look for test summary
		for _, line := range lines {
			if contains(line, "PASS") || contains(line, "FAIL") ||
				contains(line, "ok") || contains(line, "---") {
				return truncateString(line, 200)
			}
		}
	case "coder":
		// Look for completion indicators
		for _, line := range lines {
			if contains(line, "created") || contains(line, "modified") ||
				contains(line, "wrote") || contains(line, "updated") {
				return truncateString(line, 200)
			}
		}
	}

	// Default: first meaningful line
	for _, line := range lines {
		trimmed := trimSpace(line)
		if len(trimmed) > 10 {
			return truncateString(trimmed, 200)
		}
	}

	return truncateString(result, 200)
}

// parseShardSpecificFacts extracts structured facts from shard-specific output formats.
func (sm *ShardManager) parseShardSpecificFacts(shardID, shardType, result string) []Fact {
	var facts []Fact

	switch shardType {
	case "reviewer":
		facts = append(facts, sm.parseReviewerOutput(shardID, result)...)
	case "tester":
		facts = append(facts, sm.parseTesterOutput(shardID, result)...)
	}

	return facts
}

// parseReviewerOutput extracts structured facts from reviewer shard output.
func (sm *ShardManager) parseReviewerOutput(shardID, result string) []Fact {
	var facts []Fact

	// Parse summary counts (e.g., "0 critical, 0 errors, 0 warnings, 5 info")
	critical, errors, warnings, info := 0, 0, 0, 0

	lines := splitLines(result)
	for _, line := range lines {
		lower := toLower(line)

		// Try to extract counts
		if contains(lower, "critical") {
			critical = extractCount(line, "critical")
		}
		if contains(lower, "error") && !contains(lower, "errors:") {
			errors = extractCount(line, "error")
		}
		if contains(lower, "warning") {
			warnings = extractCount(line, "warning")
		}
		if contains(lower, "info") && !contains(lower, "information") {
			info = extractCount(line, "info")
		}
	}

	// Add summary fact
	facts = append(facts, Fact{
		Predicate: "review_summary",
		Args:      []interface{}{shardID, critical, errors, warnings, info},
	})

	return facts
}

// parseTesterOutput extracts structured facts from tester shard output.
func (sm *ShardManager) parseTesterOutput(shardID, result string) []Fact {
	var facts []Fact

	// Parse test summary (e.g., "10 passed, 2 failed, 1 skipped")
	total, passed, failed, skipped := 0, 0, 0, 0

	lines := splitLines(result)
	for _, line := range lines {
		lower := toLower(line)

		if contains(lower, "pass") {
			passed = extractCount(line, "pass")
		}
		if contains(lower, "fail") {
			failed = extractCount(line, "fail")
		}
		if contains(lower, "skip") {
			skipped = extractCount(line, "skip")
		}
	}

	total = passed + failed + skipped
	if total > 0 {
		facts = append(facts, Fact{
			Predicate: "test_summary",
			Args:      []interface{}{shardID, total, passed, failed, skipped},
		})
	}

	return facts
}

// =============================================================================
// STRING MANIPULATION UTILITIES
// =============================================================================
// Helper functions for string manipulation (avoid importing strings in hot path)

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		} else {
			b[i] = c
		}
	}
	return string(b)
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractCount(line, keyword string) int {
	// Simple extraction: find number before keyword
	lower := toLower(line)
	idx := findSubstring(lower, toLower(keyword))
	if idx < 0 {
		return 0
	}

	// Look backward for digits
	numEnd := idx
	for numEnd > 0 && (line[numEnd-1] == ' ' || line[numEnd-1] == ':') {
		numEnd--
	}

	numStart := numEnd
	for numStart > 0 && line[numStart-1] >= '0' && line[numStart-1] <= '9' {
		numStart--
	}

	if numStart < numEnd {
		num := 0
		for i := numStart; i < numEnd; i++ {
			num = num*10 + int(line[i]-'0')
		}
		return num
	}

	return 0
}
