// Package shards implements specialized ShardAgent types for the Cortex 1.5.0 architecture.
// This file implements custom rule loading and management for ReviewerShard.
package shards

import (
	"codenerd/internal/core"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// CUSTOM RULES MANAGEMENT
// =============================================================================

// LoadCustomRules loads custom review rules from a JSON file.
// Returns error if file cannot be read or parsed. Silently succeeds if file doesn't exist.
func (r *ReviewerShard) LoadCustomRules(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Printf("[ReviewerShard] Custom rules file not found: %s (using built-in rules only)\n", absPath)
		return nil
	}

	// Read file using virtualStore if available, otherwise use direct file read
	var content string

	if r.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", absPath},
		}
		content, err = r.virtualStore.RouteAction(context.Background(), action)
		if err != nil {
			return fmt.Errorf("failed to read custom rules file via VirtualStore: %w", err)
		}
	} else {
		// Direct file read fallback
		data, readErr := os.ReadFile(absPath)
		if readErr != nil {
			return fmt.Errorf("failed to read custom rules file: %w", readErr)
		}
		content = string(data)
	}

	// Parse JSON
	var rulesFile CustomRulesFile
	if err := json.Unmarshal([]byte(content), &rulesFile); err != nil {
		return fmt.Errorf("failed to parse custom rules JSON: %w", err)
	}

	// Validate and add rules
	loadedCount := 0
	for _, rule := range rulesFile.Rules {
		if err := r.validateCustomRule(rule); err != nil {
			fmt.Printf("[ReviewerShard] Skipping invalid rule %s: %v\n", rule.ID, err)
			continue
		}
		if rule.Enabled {
			r.customRules = append(r.customRules, rule)
			loadedCount++
		}
	}

	fmt.Printf("[ReviewerShard] Loaded %d custom rules from %s\n", loadedCount, absPath)
	return nil
}

// AddCustomRule adds a single custom rule to the reviewer.
func (r *ReviewerShard) AddCustomRule(rule CustomRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.validateCustomRule(rule); err != nil {
		return fmt.Errorf("invalid custom rule: %w", err)
	}

	// Check for duplicate IDs
	for _, existing := range r.customRules {
		if existing.ID == rule.ID {
			return fmt.Errorf("rule with ID %s already exists", rule.ID)
		}
	}

	r.customRules = append(r.customRules, rule)
	return nil
}

// validateCustomRule validates a custom rule's fields.
func (r *ReviewerShard) validateCustomRule(rule CustomRule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if rule.Category == "" {
		return fmt.Errorf("rule category is required")
	}
	if rule.Severity == "" {
		return fmt.Errorf("rule severity is required")
	}
	if rule.Pattern == "" {
		return fmt.Errorf("rule pattern is required")
	}
	if rule.Message == "" {
		return fmt.Errorf("rule message is required")
	}

	// Validate severity
	validSeverities := []string{"critical", "error", "warning", "info"}
	if !contains(validSeverities, rule.Severity) {
		return fmt.Errorf("invalid severity: %s (must be one of: %s)",
			rule.Severity, strings.Join(validSeverities, ", "))
	}

	// Validate category
	validCategories := []string{"security", "style", "bug", "performance", "maintainability"}
	if !contains(validCategories, rule.Category) {
		return fmt.Errorf("invalid category: %s (must be one of: %s)",
			rule.Category, strings.Join(validCategories, ", "))
	}

	// Validate regex pattern
	if _, err := regexp.Compile(rule.Pattern); err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	return nil
}

// GetCustomRules returns a copy of the current custom rules.
func (r *ReviewerShard) GetCustomRules() []CustomRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rules := make([]CustomRule, len(r.customRules))
	copy(rules, r.customRules)
	return rules
}

// ClearCustomRules removes all custom rules.
func (r *ReviewerShard) ClearCustomRules() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customRules = make([]CustomRule, 0)
}

// checkCustomRules applies custom rules to a file.
func (r *ReviewerShard) checkCustomRules(filePath, content string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)

	r.mu.RLock()
	customRules := r.customRules
	r.mu.RUnlock()

	if len(customRules) == 0 {
		return findings
	}

	lines := strings.Split(content, "\n")
	lang := r.detectLanguage(filePath)

	for _, rule := range customRules {
		// Check language filter
		if len(rule.Languages) > 0 && !contains(rule.Languages, lang) {
			continue
		}

		// Compile pattern
		pattern, err := regexp.Compile(rule.Pattern)
		if err != nil {
			// This shouldn't happen since we validate on load, but be safe
			continue
		}

		// Check each line
		for lineNum, line := range lines {
			if pattern.MatchString(line) {
				findings = append(findings, ReviewFinding{
					File:        filePath,
					Line:        lineNum + 1,
					Severity:    rule.Severity,
					Category:    rule.Category,
					RuleID:      rule.ID,
					Message:     rule.Message,
					Suggestion:  rule.Suggestion,
					CodeSnippet: strings.TrimSpace(line),
				})
			}
		}
	}

	return findings
}
