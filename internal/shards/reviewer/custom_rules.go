package reviewer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// CUSTOM RULES
// =============================================================================

// CustomRule represents a user-defined review rule that can be loaded from JSON/YAML.
type CustomRule struct {
	ID          string   `json:"id"`                    // Unique rule identifier
	Category    string   `json:"category"`              // security, style, bug, performance, maintainability
	Severity    string   `json:"severity"`              // critical, error, warning, info
	Pattern     string   `json:"pattern"`               // Regex pattern to match
	Message     string   `json:"message"`               // Error message to display
	Suggestion  string   `json:"suggestion,omitempty"`  // Optional suggestion for fix
	Languages   []string `json:"languages,omitempty"`   // Language filter (empty = all)
	Description string   `json:"description,omitempty"` // Human-readable description
	Enabled     bool     `json:"enabled"`               // Whether the rule is active
}

// CustomRulesFile represents the JSON structure for custom rules files.
type CustomRulesFile struct {
	Version string       `json:"version"` // Rules file format version
	Rules   []CustomRule `json:"rules"`   // List of custom rules
}

// LoadCustomRules loads custom review rules from a JSON file.
func (r *ReviewerShard) LoadCustomRules(path string) error {
	// Handle relative paths
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.reviewerConfig.WorkingDir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err // File doesn't exist or can't be read
	}

	var rulesFile CustomRulesFile
	if err := json.Unmarshal(data, &rulesFile); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Only load enabled rules with valid patterns
	for _, rule := range rulesFile.Rules {
		if !rule.Enabled {
			continue
		}
		// Validate regex
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			continue
		}
		r.customRules = append(r.customRules, rule)
	}

	return nil
}

// checkCustomRules checks file content against user-defined custom rules.
func (r *ReviewerShard) checkCustomRules(filePath, content string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")
	lang := r.detectLanguage(filePath)

	r.mu.RLock()
	rules := r.customRules
	r.mu.RUnlock()

	for _, rule := range rules {
		// Check language filter
		if len(rule.Languages) > 0 && !contains(rule.Languages, lang) {
			continue
		}

		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		for lineNum, line := range lines {
			if re.MatchString(line) {
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
